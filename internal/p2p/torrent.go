//go:build linux
// +build linux

package p2p

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

type TorrentEngine struct {
	peers       map[NodeID]*PeerConn
	pieceCount  map[NodeID]int
	pendingReqs map[string]chan []byte
	mu          sync.RWMutex
	muPending   sync.Mutex
}

func NewTorrentEngine() *TorrentEngine {
	log := telemetry.Logger()
	log.Debug("torrent.NewTorrentEngine: creating engine")

	return &TorrentEngine{
		peers:       make(map[NodeID]*PeerConn),
		pieceCount:  make(map[NodeID]int),
		pendingReqs: make(map[string]chan []byte),
	}
}

func (te *TorrentEngine) AddPeer(nodeID NodeID, conn *PeerConn) {
	log := telemetry.Logger()
	log.Info("torrent.AddPeer: adding peer", telemetry.FieldString("nodeID", nodeID.String()))

	te.mu.Lock()
	defer te.mu.Unlock()

	te.peers[nodeID] = conn
	te.pieceCount[nodeID] = 0

	go te.handlePeerMessages(nodeID, conn)
}

func (te *TorrentEngine) RemovePeer(nodeID NodeID) {
	log := telemetry.Logger()
	log.Info("torrent.RemovePeer: removing peer", telemetry.FieldString("nodeID", nodeID.String()))

	te.mu.Lock()
	defer te.mu.Unlock()

	delete(te.peers, nodeID)
	delete(te.pieceCount, nodeID)
}

func (te *TorrentEngine) handlePeerMessages(nodeID NodeID, conn *PeerConn) {
	for msg := range conn.ReadCh() {
		msgType, _ := ParseMessage(msg)
		if msgType == MsgChunkResponse {
			te.muPending.Lock()
			var req ChunkRequest
			copy(req.Digest[:], msg[1:33])
			key := string(req.Digest[:])
			if ch, ok := te.pendingReqs[key]; ok {
				var resp ChunkResponse
				copy(resp.Digest[:], msg[1:33])
				if len(msg) > 37 {
					size := int(uint32(msg[33])<<24 | uint32(msg[34])<<16 | uint32(msg[35])<<8 | uint32(msg[36]))
					if size <= len(msg)-37 {
						resp.Data = msg[37 : 37+size]
					}
				}
				ch <- resp.Data
				delete(te.pendingReqs, key)
			}
			te.muPending.Unlock()

			te.mu.Lock()
			te.pieceCount[nodeID]++
			te.mu.Unlock()
		}
	}
}

func (te *TorrentEngine) SelectPeer(digest string) (NodeID, bool) {
	te.mu.RLock()
	defer te.mu.RUnlock()

	if len(te.peers) == 0 {
		return NodeID{}, false
	}

	type scoredPeer struct {
		nodeID NodeID
		pieces int
	}

	var scored []scoredPeer
	for id, count := range te.pieceCount {
		scored = append(scored, scoredPeer{id, count})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].pieces < scored[j].pieces
	})

	return scored[0].nodeID, true
}

func (te *TorrentEngine) RequestChunk(digest string, peerID NodeID) ([]byte, error) {
	key := digest

	te.muPending.Lock()
	if _, exists := te.pendingReqs[key]; exists {
		te.muPending.Unlock()
		return nil, fmt.Errorf("RequestChunk: request already in-flight for %s", digest[:12])
	}
	ch := make(chan []byte, 1)
	te.pendingReqs[key] = ch
	te.muPending.Unlock()

	defer func() {
		te.muPending.Lock()
		delete(te.pendingReqs, key)
		te.muPending.Unlock()
	}()

	te.mu.RLock()
	peer, ok := te.peers[peerID]
	te.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("RequestChunk: peer %s not connected", peerID.String())
	}

	var d [32]byte
	copy(d[:], digest)
	if err := peer.SendChunkRequest(d); err != nil {
		return nil, fmt.Errorf("RequestChunk: SendChunkRequest: %w", err)
	}

	// Block until peer responds or 30s timeout.
	select {
	case data := <-ch:
		return data, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("RequestChunk: timeout waiting for chunk %s", digest[:12])
	}
}

func (te *TorrentEngine) PeerCount() int {
	te.mu.RLock()
	defer te.mu.RUnlock()
	return len(te.peers)
}

func (te *TorrentEngine) HasPeer(nodeID NodeID) bool {
	te.mu.RLock()
	defer te.mu.RUnlock()
	_, ok := te.peers[nodeID]
	return ok
}