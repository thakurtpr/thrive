//go:build linux
// +build linux

package p2p

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"
	"math/rand"
	"sync"

	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

const (
	bucketSize = 20
	alpha      = 3
	k           = 8
)

type NodeID [32]byte

func (n NodeID) String() string {
	return fmt.Sprintf("%x", n[:8])
}

func (n NodeID) Xor(other NodeID) NodeID {
	var r NodeID
	for i := 0; i < 32; i++ {
		r[i] = n[i] ^ other[i]
	}
	return r
}

type kbucket struct {
	nodes   map[NodeID]string
	lriTime map[NodeID]int64
	mu      sync.RWMutex
}

func newKBucket() *kbucket {
	return &kbucket{
		nodes:   make(map[NodeID]string),
		lriTime: make(map[NodeID]int64),
	}
}

type DHT struct {
	nodeID   NodeID
	kbuckets [256]*kbucket
	selfAddr string
	peers    map[NodeID]string
	mu       sync.RWMutex
}

func NewDHT(selfID NodeID, selfAddr string) *DHT {
	log := telemetry.Logger()
	log.Info("dht.NewDHT: initializing", telemetry.FieldString("nodeID", selfID.String()))

	dht := &DHT{
		nodeID:   selfID,
		selfAddr: selfAddr,
		kbuckets: [256]*kbucket{},
		peers:    make(map[NodeID]string),
	}
	for i := 0; i < 256; i++ {
		dht.kbuckets[i] = newKBucket()
	}
	return dht
}

func (d *DHT) bucketIndex(id NodeID) int {
	distance := id.Xor(d.nodeID)
	for i := 0; i < 32; i++ {
		if distance[i] != 0 {
			return 255 - i*8 - int(log2(uint64(distance[i])))
		}
	}
	return 0
}

func log2(x uint64) int {
	if x == 0 {
		return 0
	}
	n := 0
	if x >= 1<<16 {
		x >>= 16
		n += 16
	}
	if x >= 1<<8 {
		x >>= 8
		n += 8
	}
	if x >= 1<<4 {
		x >>= 4
		n += 4
	}
	if x >= 1<<2 {
		x >>= 2
		n += 2
	}
	if x >= 1<<1 {
		n += 1
	}
	return n
}

func (d *DHT) AddNode(peerID NodeID, addr string) error {
	log := telemetry.Logger()
	log.Debug("dht.AddNode: adding peer", telemetry.FieldString("peerID", peerID.String()), telemetry.FieldString("addr", addr))

	d.mu.Lock()
	defer d.mu.Unlock()

	idx := d.bucketIndex(peerID)
	kb := d.kbuckets[idx]
	kb.mu.Lock()
	defer kb.mu.Unlock()

	kb.nodes[peerID] = addr
	kb.lriTime[peerID] = rand.Int63()
	d.peers[peerID] = addr

	log.Debug("dht.AddNode: added to bucket", telemetry.FieldInt("bucket", idx))
	return nil
}

func (d *DHT) RemoveNode(peerID NodeID) {
	d.mu.Lock()
	defer d.mu.Unlock()

	idx := d.bucketIndex(peerID)
	kb := d.kbuckets[idx]
	kb.mu.Lock()
	defer kb.mu.Unlock()

	delete(kb.nodes, peerID)
	delete(kb.lriTime, peerID)
	delete(d.peers, peerID)
}

func (d *DHT) FindNode(target NodeID) []NodeInfo {
	log := telemetry.Logger()
	log.Debug("dht.FindNode: finding nodes closest to", telemetry.FieldString("target", target.String()))

	d.mu.RLock()
	defer d.mu.RUnlock()

	var candidates []NodeInfo
	seen := make(map[NodeID]bool)

	bucketIdx := d.bucketIndex(target)
	kb := d.kbuckets[bucketIdx]
	kb.mu.RLock()
	for id, addr := range kb.nodes {
		if !seen[id] {
			candidates = append(candidates, NodeInfo{ID: id, Addr: addr})
			seen[id] = true
		}
	}
	kb.mu.RUnlock()

	for i := 1; i < 256 && len(candidates) < k; i++ {
		lo := bucketIdx - i
		hi := bucketIdx + i
		if lo < 0 {
			lo = 0
		}
		if hi >= 256 {
			hi = 255
		}
		for j := lo; j <= hi; j++ {
			kb := d.kbuckets[j]
			kb.mu.RLock()
			for id, addr := range kb.nodes {
				if !seen[id] {
					candidates = append(candidates, NodeInfo{ID: id, Addr: addr})
					seen[id] = true
					if len(candidates) >= k {
						kb.mu.RUnlock()
						goto done
					}
				}
			}
			kb.mu.RUnlock()
		}
	}

done:
	return candidates
}

func (d *DHT) Announce(chunkDigest string, peerAddr string) error {
	log := telemetry.Logger()
	log.Debug("dht.Announce: announcing chunk", telemetry.FieldString("digest", chunkDigest[:12]))

	id := ChunkDigestToNodeID(chunkDigest)
	return d.AddNode(id, peerAddr)
}

func ChunkDigestToNodeID(digest string) NodeID {
	h := sha256.Sum256([]byte(digest))
	var id NodeID
	copy(id[:], h[:])
	return id
}

type NodeInfo struct {
	ID   NodeID
	Addr string
}

func XorDistance(a, b NodeID) *big.Int {
	r := a.Xor(b)
	var val uint64
	for i := 0; i < 8; i++ {
		val = (val << 8) | uint64(r[i])
	}
	return new(big.Int).SetUint64(val)
}

func GenerateNodeID() NodeID {
	var id NodeID
	binary.BigEndian.PutUint64(id[:], rand.Uint64())
	binary.BigEndian.PutUint64(id[8:], rand.Uint64())
	binary.BigEndian.PutUint64(id[16:], rand.Uint64())
	binary.BigEndian.PutUint64(id[24:], rand.Uint64())
	return id
}

func NodeIDFromString(s string) (NodeID, error) {
	var id NodeID
	if len(s) != 64 {
		return id, fmt.Errorf("NodeIDFromString: invalid length")
	}
	for i := 0; i < 32; i++ {
		b, err := parseHex(s[i*2 : i*2+2])
		if err != nil {
			return id, err
		}
		id[i] = b
	}
	return id, nil
}

func parseHex(s string) (byte, error) {
	var v byte
	for _, c := range s {
		var b byte
		switch {
		case c >= '0' && c <= '9':
			b = byte(c - '0')
		case c >= 'a' && c <= 'f':
			b = byte(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			b = byte(c - 'A' + 10)
		default:
			return 0, fmt.Errorf("parseHex: invalid char %c", c)
		}
		v = v<<4 | b
	}
	return v, nil
}