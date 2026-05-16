//go:build linux
// +build linux

package p2p

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

type Node struct {
	ID       NodeID
	Addr     string
	chunks   *image.ChunkStore
	peers    map[NodeID]*PeerConn
	dht      *DHT
	torrent  *TorrentEngine
	listener *Listener
	mu       sync.RWMutex
}

func NewNode(listenAddr string) (*Node, error) {
	log := telemetry.Logger()
	log.Info("p2p.NewNode: starting", telemetry.FieldString("addr", listenAddr))

	nodeID := GenerateNodeID()
	cs := image.NewChunkStore("/var/lib/thrive/chunks")

	n := &Node{
		ID:      nodeID,
		Addr:    listenAddr,
		chunks:  cs,
		peers:   make(map[NodeID]*PeerConn),
		dht:     NewDHT(nodeID, listenAddr),
		torrent: NewTorrentEngine(),
	}

	ln, err := NewListener(listenAddr, func(conn *PeerConn) {
		if conn != nil {
			n.mu.Lock()
			n.peers[conn.nodeID] = conn
			n.mu.Unlock()
			n.torrent.AddPeer(conn.nodeID, conn)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("NewNode: Listener: %w", err)
	}
	n.listener = ln

	log.Info("p2p.NewNode: started", telemetry.FieldString("nodeID", nodeID.String()))
	return n, nil
}

func (n *Node) FetchChunk(ctx context.Context, digest string) ([]byte, error) {
	log := telemetry.Logger()
	log.Info("p2p.FetchChunk: fetching", telemetry.FieldString("digest", digest[:12]))

	if n.chunks.Has(ctx, digest) {
		data, err := n.chunks.Get(ctx, digest)
		if err == nil && len(data) > 0 {
			log.Debug("p2p.FetchChunk: cache hit", telemetry.FieldString("digest", digest[:12]))
			return data, nil
		}
	}

	peerID, ok := n.torrent.SelectPeer(digest)
	if !ok {
		log.Warn("p2p.FetchChunk: no peers available", telemetry.FieldString("digest", digest[:12]))
		return nil, fmt.Errorf("p2p.FetchChunk: no peers")
	}

	data, err := n.torrent.RequestChunk(digest, peerID)
	if err != nil {
		log.Error("p2p.FetchChunk: request failed", telemetry.FieldString("digest", digest[:12]), telemetry.FieldError(err))
		return nil, err
	}
	if len(data) > 0 {
		if err := n.validateAndStore(digest, data); err != nil {
			log.Error("p2p.FetchChunk: validateAndStore failed", telemetry.FieldError(err))
		}
		return data, nil
	}

	log.Warn("p2p.FetchChunk: P2P miss, falling back", telemetry.FieldString("digest", digest[:12]))
	return nil, fmt.Errorf("p2p.FetchChunk: not found in P2P network")
}

func (n *Node) validateAndStore(digest string, data []byte) error {
	h := sha256.Sum256(data)
	if fmt.Sprintf("%x", h[:]) != digest {
		return fmt.Errorf("validateAndStore: checksum mismatch")
	}
	return n.chunks.Put(context.Background(), digest, data)
}

func (n *Node) AnnounceChunk(digest string) error {
	log := telemetry.Logger()
	log.Debug("p2p.AnnounceChunk: announcing", telemetry.FieldString("digest", digest[:12]))

	return n.dht.Announce(digest, n.Addr)
}

func (n *Node) ConnectPeer(addr string, nodeID NodeID) error {
	log := telemetry.Logger()
	log.Info("p2p.ConnectPeer: connecting", telemetry.FieldString("addr", addr), telemetry.FieldString("nodeID", nodeID.String()))

	conn, err := Dial(addr, n.ID, func(id NodeID) {
		n.mu.Lock()
		delete(n.peers, id)
		n.mu.Unlock()
		n.torrent.RemovePeer(id)
	})
	if err != nil {
		return fmt.Errorf("ConnectPeer: %w", err)
	}

	n.mu.Lock()
	n.peers[nodeID] = conn
	n.mu.Unlock()

	n.dht.AddNode(nodeID, addr)
	n.torrent.AddPeer(nodeID, conn)

	log.Info("p2p.ConnectPeer: connected", telemetry.FieldString("nodeID", nodeID.String()))
	return nil
}

func (n *Node) Shutdown() error {
	log := telemetry.Logger()
	log.Info("p2p.Node.Shutdown: initiating")

	n.mu.Lock()
	for id, conn := range n.peers {
		conn.Close()
		delete(n.peers, id)
	}
	n.mu.Unlock()

	if n.listener != nil {
		n.listener.Close()
	}

	log.Info("p2p.Node.Shutdown: complete")
	return nil
}

func (n *Node) Bootstrap(ctx context.Context, peers []string) error {
	log := telemetry.Logger()
	log.Info("p2p.Bootstrap: starting", telemetry.FieldInt("peers", len(peers)))

	for _, addr := range peers {
		conn, err := Dial(addr, n.ID, nil)
		if err != nil {
			log.Warn("p2p.Bootstrap: failed to connect", telemetry.FieldString("addr", addr), telemetry.FieldError(err))
			continue
		}

		var remoteID NodeID
		copy(remoteID[:], addr)

		n.mu.Lock()
		n.peers[remoteID] = conn
		n.mu.Unlock()

		n.dht.AddNode(remoteID, addr)
		n.torrent.AddPeer(remoteID, conn)
	}

	return nil
}

func (n *Node) Serve(ctx context.Context) error {
	log := telemetry.Logger()
	log.Info("p2p.Node.Serve: starting P2P server", telemetry.FieldString("addr", n.Addr))

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			n.refreshBucket()
		}
	}
}

func (n *Node) refreshBucket() {
	log := telemetry.Logger()
	log.Debug("p2p.refreshBucket: refreshing")

	for id := range n.peers {
		nodes := n.dht.FindNode(id)
		for _, ni := range nodes {
			if _, ok := n.peers[ni.ID]; !ok {
				n.ConnectPeer(ni.Addr, ni.ID)
			}
		}
	}
}