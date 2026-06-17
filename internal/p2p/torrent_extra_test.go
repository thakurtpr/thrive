//go:build linux
// +build linux

package p2p

import (
	"net"
	"strings"
	"testing"
	"time"
)

// TestRequestChunk_NoPeers verifies that RequestChunk returns an error when
// the requested peer is not registered in the engine.  This exercises the
// early-exit branch that fires before any network I/O.
func TestRequestChunk_NoPeers(t *testing.T) {
	// Arrange — empty engine, unknown peer ID.
	engine := NewTorrentEngine()
	var unknownPeer NodeID
	unknownPeer[0] = 0xFF

	digest := strings.Repeat("a", 32) // 32-character digest

	// Act
	_, err := engine.RequestChunk(digest, unknownPeer)

	// Assert
	if err == nil {
		t.Fatal("RequestChunk with unregistered peer: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("RequestChunk error: got %q, want it to contain \"not connected\"", err.Error())
	}
}

// TestRequestChunk_PeerDisappearsBeforeResponse verifies that RequestChunk
// errors when the peer is removed from the engine between registration and the
// call.  This is the deterministic analogue of the 30-second timeout path.
func TestRequestChunk_PeerDisappearsBeforeResponse(t *testing.T) {
	// Arrange
	engine := NewTorrentEngine()
	var nodeID NodeID
	nodeID[0] = 0x42

	clientConn, serverConn := net.Pipe()
	pc := NewPeerConn(clientConn, nodeID, nil)

	// Register the peer, then immediately remove it before calling RequestChunk.
	engine.AddPeer(nodeID, pc)
	serverConn.Close()
	pc.Close()
	engine.RemovePeer(nodeID)

	digest := strings.Repeat("b", 32)

	// Act
	_, err := engine.RequestChunk(digest, nodeID)

	// Assert
	if err == nil {
		t.Fatal("RequestChunk after peer removal: expected error, got nil")
	}
}

// TestRequestChunk_DuplicateInFlight verifies that a second concurrent
// RequestChunk call for the same digest is rejected immediately with an
// "already in-flight" error while the first call is still pending.
func TestRequestChunk_DuplicateInFlight(t *testing.T) {
	// Arrange — peer whose server side silently swallows data but never replies.
	engine := NewTorrentEngine()
	var nodeID NodeID
	nodeID[0] = 0x07

	clientConn, serverConn := net.Pipe()
	pc := NewPeerConn(clientConn, nodeID, nil)
	engine.AddPeer(nodeID, pc)

	digest := strings.Repeat("c", 32)

	// Launch the first RequestChunk in the background; it will block on the
	// pending channel waiting for a ChunkResponse that never arrives.
	firstDone := make(chan error, 1)
	go func() {
		_, err := engine.RequestChunk(digest, nodeID)
		firstDone <- err
	}()

	// Give the first goroutine enough time to register its pending entry.
	time.Sleep(30 * time.Millisecond)

	// Act — second call with the same digest should be rejected immediately.
	_, err := engine.RequestChunk(digest, nodeID)

	// Assert
	if err == nil {
		t.Fatal("duplicate RequestChunk: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "already in-flight") {
		t.Errorf("duplicate RequestChunk error: got %q, want \"already in-flight\"", err.Error())
	}

	// Cleanup — close both ends of the pipe so the first goroutine unblocks.
	pc.Close()
	serverConn.Close()

	// Wait for the first goroutine to finish; do not assert its result.
	select {
	case <-firstDone:
	case <-time.After(35 * time.Second):
		t.Error("first RequestChunk goroutine did not finish within 35 s")
	}
}

// TestTorrentEngine_HasPeer verifies that HasPeer returns true after AddPeer
// and false after RemovePeer.
func TestTorrentEngine_HasPeer(t *testing.T) {
	engine := NewTorrentEngine()
	var nodeID NodeID
	nodeID[0] = 0x99

	clientConn, serverConn := net.Pipe()
	pc := NewPeerConn(clientConn, nodeID, nil)
	t.Cleanup(func() {
		pc.Close()
		serverConn.Close()
	})

	// Before add: HasPeer must return false.
	if engine.HasPeer(nodeID) {
		t.Error("HasPeer before AddPeer: expected false, got true")
	}

	engine.AddPeer(nodeID, pc)

	// After add: HasPeer must return true.
	if !engine.HasPeer(nodeID) {
		t.Error("HasPeer after AddPeer: expected true, got false")
	}

	engine.RemovePeer(nodeID)

	// After remove: HasPeer must return false.
	if engine.HasPeer(nodeID) {
		t.Error("HasPeer after RemovePeer: expected false, got true")
	}
}

// TestPeerConn_Close_Idempotent verifies that calling Close twice on a
// PeerConn does not panic or return an error.
func TestPeerConn_Close_Idempotent(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	var nodeID NodeID
	pc := NewPeerConn(clientConn, nodeID, nil)

	if err := pc.Close(); err != nil {
		t.Errorf("first Close: unexpected error: %v", err)
	}
	if err := pc.Close(); err != nil {
		t.Errorf("second Close: unexpected error: %v", err)
	}
}

// TestParseMessage_Empty verifies that ParseMessage on a zero-length slice
// returns message type 0 and a nil payload without panicking.
func TestParseMessage_Empty(t *testing.T) {
	msgType, payload := ParseMessage([]byte{})
	if msgType != 0 {
		t.Errorf("ParseMessage(empty): got type %d, want 0", msgType)
	}
	if payload != nil {
		t.Errorf("ParseMessage(empty): got non-nil payload: %v", payload)
	}
}

// TestParseMessage_ChunkRequest verifies that a well-formed MsgChunkRequest
// message is decoded into a ChunkRequest with the correct digest bytes.
func TestParseMessage_ChunkRequest(t *testing.T) {
	// Arrange — 1 type byte + 32 digest bytes.
	msg := make([]byte, 33)
	msg[0] = byte(MsgChunkRequest)
	for i := 1; i <= 32; i++ {
		msg[i] = byte(i)
	}

	// Act
	msgType, payload := ParseMessage(msg)

	// Assert
	if msgType != MsgChunkRequest {
		t.Errorf("ParseMessage: got type %v, want MsgChunkRequest", msgType)
	}
	req, ok := payload.(ChunkRequest)
	if !ok {
		t.Fatalf("ParseMessage: payload is %T, want ChunkRequest", payload)
	}
	if req.Digest[0] != 1 || req.Digest[31] != 32 {
		t.Errorf("ParseMessage digest: [0]=%d [31]=%d, want 1 and 32",
			req.Digest[0], req.Digest[31])
	}
}

// TestPeerCount_AddRemoveMultiple verifies PeerCount correctly tracks additions
// and removals with more than one peer.
func TestPeerCount_AddRemoveMultiple(t *testing.T) {
	engine := NewTorrentEngine()

	const n = 3
	pcs := make([]*PeerConn, n)
	others := make([]net.Conn, n)

	for i := 0; i < n; i++ {
		var id NodeID
		id[0] = byte(i + 1)
		c, o := net.Pipe()
		pcs[i] = NewPeerConn(c, id, nil)
		others[i] = o
		engine.AddPeer(id, pcs[i])
	}

	t.Cleanup(func() {
		for i := 0; i < n; i++ {
			pcs[i].Close()
			others[i].Close()
		}
	})

	if got := engine.PeerCount(); got != n {
		t.Errorf("PeerCount after %d adds: got %d, want %d", n, got, n)
	}

	// Remove one peer and verify count decrements.
	var removeID NodeID
	removeID[0] = 0x01
	engine.RemovePeer(removeID)

	if got := engine.PeerCount(); got != n-1 {
		t.Errorf("PeerCount after 1 remove: got %d, want %d", got, n-1)
	}
}
