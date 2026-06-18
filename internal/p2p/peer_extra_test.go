//go:build linux
// +build linux

package p2p

import (
	"encoding/binary"
	"math/big"
	"testing"
)

// --- NodeID tests ---

func TestNodeIDString(t *testing.T) {
	var id NodeID
	id[0] = 0xAB
	id[1] = 0xCD
	s := id.String()
	if len(s) < 2 {
		t.Fatalf("NodeID.String() too short: %q", s)
	}
	// First 8 bytes encoded as hex = 16 chars
	if len(s) != 16 {
		t.Errorf("NodeID.String() length: got %d, want 16", len(s))
	}
}

func TestNodeIDXor(t *testing.T) {
	var a, b NodeID
	a[0] = 0xFF
	b[0] = 0x0F
	result := a.Xor(b)
	if result[0] != 0xF0 {
		t.Errorf("Xor[0]: got %02x, want f0", result[0])
	}
	// rest should be zero XOR zero = zero
	for i := 1; i < 32; i++ {
		if result[i] != 0 {
			t.Errorf("Xor[%d]: got %02x, want 00", i, result[i])
		}
	}
}

func TestGenerateNodeID_Unique(t *testing.T) {
	a := GenerateNodeID()
	b := GenerateNodeID()
	if a == b {
		t.Error("GenerateNodeID: two calls returned identical IDs")
	}
}

func TestNodeIDFromString_Valid(t *testing.T) {
	// 64 hex chars = 32 bytes
	hexStr := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	id, err := NodeIDFromString(hexStr)
	if err != nil {
		t.Fatalf("NodeIDFromString: %v", err)
	}
	if id[0] != 0x01 {
		t.Errorf("id[0]: got %02x, want 01", id[0])
	}
	if id[31] != 0x20 {
		t.Errorf("id[31]: got %02x, want 20", id[31])
	}
}

func TestNodeIDFromString_TooShort(t *testing.T) {
	_, err := NodeIDFromString("abc")
	if err == nil {
		t.Fatal("NodeIDFromString with short string: expected error")
	}
}

func TestNodeIDFromString_InvalidHex(t *testing.T) {
	// 64 chars but contains non-hex character
	bad := "zz02030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	_, err := NodeIDFromString(bad)
	if err == nil {
		t.Fatal("NodeIDFromString with invalid hex: expected error")
	}
}

// --- XorDistance tests ---

func TestXorDistance_SameNode(t *testing.T) {
	var id NodeID
	id[0] = 0x42
	dist := XorDistance(id, id)
	if dist.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("XorDistance(x, x): got %v, want 0", dist)
	}
}

func TestXorDistance_DifferentNodes(t *testing.T) {
	var a, b NodeID
	a[0] = 0x00
	b[0] = 0xFF
	dist := XorDistance(a, b)
	if dist.Sign() == 0 {
		t.Error("XorDistance: expected non-zero distance for different nodes")
	}
}

// --- ChunkDigestToNodeID tests ---

func TestChunkDigestToNodeID_Deterministic(t *testing.T) {
	digest := "sha256:abc123def456"
	id1 := ChunkDigestToNodeID(digest)
	id2 := ChunkDigestToNodeID(digest)
	if id1 != id2 {
		t.Error("ChunkDigestToNodeID: not deterministic for same input")
	}
}

func TestChunkDigestToNodeID_UniqueForDifferentDigests(t *testing.T) {
	id1 := ChunkDigestToNodeID("digest-a")
	id2 := ChunkDigestToNodeID("digest-b")
	if id1 == id2 {
		t.Error("ChunkDigestToNodeID: collision for different digests")
	}
}

// --- DHT tests ---

func TestNewDHT_Empty(t *testing.T) {
	selfID := GenerateNodeID()
	dht := NewDHT(selfID, "127.0.0.1:9000")
	if dht == nil {
		t.Fatal("NewDHT: returned nil")
	}
}

func TestDHT_AddAndFindNode(t *testing.T) {
	selfID := GenerateNodeID()
	dht := NewDHT(selfID, "127.0.0.1:9000")

	peerID := GenerateNodeID()
	if err := dht.AddNode(peerID, "127.0.0.1:9001"); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	// FindNode should return the added peer among candidates
	found := dht.FindNode(peerID)
	if len(found) == 0 {
		t.Error("FindNode: expected at least 1 result after AddNode")
	}

	foundPeer := false
	for _, ni := range found {
		if ni.ID == peerID {
			foundPeer = true
			if ni.Addr != "127.0.0.1:9001" {
				t.Errorf("FindNode: wrong addr %q, want 127.0.0.1:9001", ni.Addr)
			}
		}
	}
	if !foundPeer {
		t.Error("FindNode: added peer not in results")
	}
}

func TestDHT_RemoveNode(t *testing.T) {
	selfID := GenerateNodeID()
	dht := NewDHT(selfID, "127.0.0.1:9000")

	peerID := GenerateNodeID()
	dht.AddNode(peerID, "127.0.0.1:9001")
	dht.RemoveNode(peerID)

	// After removal, FindNode should not return the peer
	found := dht.FindNode(peerID)
	for _, ni := range found {
		if ni.ID == peerID {
			t.Error("FindNode: removed peer still present in results")
		}
	}
}

func TestDHT_Announce(t *testing.T) {
	selfID := GenerateNodeID()
	dht := NewDHT(selfID, "127.0.0.1:9000")

	digest := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	if err := dht.Announce(digest, "127.0.0.1:9002"); err != nil {
		t.Fatalf("Announce: %v", err)
	}

	// The chunk's NodeID should be discoverable
	chunkID := ChunkDigestToNodeID(digest)
	found := dht.FindNode(chunkID)
	if len(found) == 0 {
		t.Error("FindNode after Announce: expected at least 1 result")
	}
}

func TestDHT_FindNode_EmptyDHT(t *testing.T) {
	selfID := GenerateNodeID()
	dht := NewDHT(selfID, "127.0.0.1:9000")
	// Should not panic on empty DHT
	found := dht.FindNode(GenerateNodeID())
	_ = found
}

func TestDHT_AddMultipleNodes(t *testing.T) {
	selfID := GenerateNodeID()
	dht := NewDHT(selfID, "127.0.0.1:9000")

	const n = 5
	peers := make([]NodeID, n)
	for i := 0; i < n; i++ {
		peers[i] = GenerateNodeID()
		addr := "127.0.0.1:910" + string(rune('0'+i))
		if err := dht.AddNode(peers[i], addr); err != nil {
			t.Fatalf("AddNode[%d]: %v", i, err)
		}
	}

	// FindNode should find at least one of them
	found := dht.FindNode(peers[0])
	if len(found) == 0 {
		t.Error("FindNode: expected results after adding multiple nodes")
	}
}

// --- ParseMessage additional cases ---

func TestParseMessage_ChunkResponse(t *testing.T) {
	// Build a MsgChunkResponse: 1 type + 32 digest + 4 length + N data
	data := []byte("hello world")
	msg := make([]byte, 1+32+4+len(data))
	msg[0] = byte(MsgChunkResponse)
	for i := 1; i <= 32; i++ {
		msg[i] = byte(i)
	}
	binary.BigEndian.PutUint32(msg[33:37], uint32(len(data)))
	copy(msg[37:], data)

	msgType, payload := ParseMessage(msg)
	if msgType != MsgChunkResponse {
		t.Errorf("ParseMessage: got type %v, want MsgChunkResponse", msgType)
	}
	resp, ok := payload.(ChunkResponse)
	if !ok {
		t.Fatalf("ParseMessage: payload is %T, want ChunkResponse", payload)
	}
	if string(resp.Data) != "hello world" {
		t.Errorf("ParseMessage ChunkResponse data: got %q, want %q", resp.Data, "hello world")
	}
}

func TestParseMessage_FindNode(t *testing.T) {
	// Build a MsgFindNode: 1 type + 32 target bytes
	msg := make([]byte, 33)
	msg[0] = byte(MsgFindNode)
	for i := 1; i <= 32; i++ {
		msg[i] = byte(i)
	}

	msgType, payload := ParseMessage(msg)
	if msgType != MsgFindNode {
		t.Errorf("ParseMessage: got type %v, want MsgFindNode", msgType)
	}
	req, ok := payload.(FindNodeRequest)
	if !ok {
		t.Fatalf("ParseMessage: payload is %T, want FindNodeRequest", payload)
	}
	if req.Target[0] != 1 {
		t.Errorf("ParseMessage FindNode target[0]: got %d, want 1", req.Target[0])
	}
}

func TestParseMessage_UnknownType(t *testing.T) {
	msg := []byte{0xFF, 0x01, 0x02}
	msgType, payload := ParseMessage(msg)
	if msgType != 0 {
		t.Errorf("ParseMessage unknown type: got %v, want 0", msgType)
	}
	if payload != nil {
		t.Errorf("ParseMessage unknown type: got non-nil payload: %v", payload)
	}
}

// --- PeerConn send/receive tests using net.Pipe ---

func TestPeerConn_SendChunkResponse(t *testing.T) {
	var nodeID NodeID
	nodeID[0] = 0x11
	pc, serverConn := newTestPeerConn(nodeID)
	t.Cleanup(func() {
		pc.Close()
		serverConn.Close()
	})

	var digest [32]byte
	digest[0] = 0xAA
	data := []byte("chunk data")
	if err := pc.SendChunkResponse(digest, data); err != nil {
		t.Fatalf("SendChunkResponse: %v", err)
	}
}

func TestPeerConn_SendFindNode(t *testing.T) {
	var nodeID NodeID
	nodeID[0] = 0x22
	pc, serverConn := newTestPeerConn(nodeID)
	t.Cleanup(func() {
		pc.Close()
		serverConn.Close()
	})

	var target [32]byte
	target[0] = 0xBB
	if err := pc.SendFindNode(target); err != nil {
		t.Fatalf("SendFindNode: %v", err)
	}
}

func TestPeerConn_RemoteAddr(t *testing.T) {
	var nodeID NodeID
	pc, serverConn := newTestPeerConn(nodeID)
	t.Cleanup(func() {
		pc.Close()
		serverConn.Close()
	})

	addr := pc.RemoteAddr()
	if addr == "" {
		t.Error("RemoteAddr: expected non-empty string")
	}
}

func TestPeerConn_ReadCh(t *testing.T) {
	var nodeID NodeID
	pc, serverConn := newTestPeerConn(nodeID)
	t.Cleanup(func() {
		pc.Close()
		serverConn.Close()
	})

	ch := pc.ReadCh()
	if ch == nil {
		t.Error("ReadCh: returned nil channel")
	}
}

// --- TorrentEngine SelectPeer with one peer ---

func TestSelectPeer_WithOnePeer(t *testing.T) {
	engine := NewTorrentEngine()
	var nodeID NodeID
	nodeID[0] = 0x55

	pc, other := newTestPeerConn(nodeID)
	t.Cleanup(func() {
		pc.Close()
		other.Close()
	})
	engine.AddPeer(nodeID, pc)

	selected, ok := engine.SelectPeer("some-digest")
	if !ok {
		t.Fatal("SelectPeer: expected ok==true with one peer")
	}
	if selected != nodeID {
		t.Errorf("SelectPeer: got %v, want %v", selected, nodeID)
	}
}
