//go:build linux
// +build linux

package p2p

import (
	"net"
	"testing"
)

// newTestPeerConn creates a PeerConn backed by one end of an in-process
// net.Pipe(), which satisfies the net.Conn interface without a real socket.
// The other end of the pipe is returned so the caller can close it cleanly.
func newTestPeerConn(nodeID NodeID) (*PeerConn, net.Conn) {
	clientConn, serverConn := net.Pipe()
	pc := NewPeerConn(clientConn, nodeID, nil)
	return pc, serverConn
}

// TestNewTorrentEngine verifies that a freshly constructed TorrentEngine has
// no peers and reports a PeerCount of zero.
func TestNewTorrentEngine(t *testing.T) {
	// Act
	engine := NewTorrentEngine()

	// Assert
	if engine == nil {
		t.Fatal("NewTorrentEngine: returned nil")
	}
	if got := engine.PeerCount(); got != 0 {
		t.Errorf("PeerCount: got %d, want 0", got)
	}
}

// TestTorrentEngine_AddRemovePeer verifies that AddPeer increments PeerCount
// and RemovePeer decrements it back to zero.
func TestTorrentEngine_AddRemovePeer(t *testing.T) {
	// Arrange
	engine := NewTorrentEngine()
	var nodeID NodeID
	nodeID[0] = 0x01

	pc, other := newTestPeerConn(nodeID)
	t.Cleanup(func() {
		pc.Close()
		other.Close()
	})

	// Act — add
	engine.AddPeer(nodeID, pc)

	// Assert after add
	if got := engine.PeerCount(); got != 1 {
		t.Errorf("PeerCount after AddPeer: got %d, want 1", got)
	}

	// Act — remove
	engine.RemovePeer(nodeID)

	// Assert after remove
	if got := engine.PeerCount(); got != 0 {
		t.Errorf("PeerCount after RemovePeer: got %d, want 0", got)
	}
}

// TestSelectPeer_NoPeers verifies that SelectPeer returns ok==false when the
// engine has no registered peers.
func TestSelectPeer_NoPeers(t *testing.T) {
	// Arrange
	engine := NewTorrentEngine()

	// Act
	_, ok := engine.SelectPeer("")

	// Assert
	if ok {
		t.Error("SelectPeer on empty engine: expected ok==false, got true")
	}
}
