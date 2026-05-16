//go:build linux
// +build linux

package p2p

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

type PeerConn struct {
	conn    net.Conn
	nodeID  NodeID
	readCh  chan []byte
	writeCh chan []byte
	mu      sync.RWMutex
	closed  bool
	onClose func(NodeID)
}

type MessageType byte

const (
	MsgChunkRequest  MessageType = 0x01
	MsgChunkResponse MessageType = 0x02
	MsgPing          MessageType = 0x03
	MsgPong          MessageType = 0x04
	MsgFindNode      MessageType = 0x05
	MsgFindNodeResp  MessageType = 0x06
	MsgAnnounce      MessageType = 0x07
)

type ChunkRequest struct {
	Digest [32]byte
}

type ChunkResponse struct {
	Digest [32]byte
	Data   []byte
}

type FindNodeRequest struct {
	Target [32]byte
}

func NewPeerConn(conn net.Conn, nodeID NodeID, onClose func(NodeID)) *PeerConn {
	log := telemetry.Logger()
	log.Info("peer.NewPeerConn: new connection", telemetry.FieldString("nodeID", nodeID.String()), telemetry.FieldString("addr", conn.RemoteAddr().String()))

	pc := &PeerConn{
		conn:    conn,
		nodeID:  nodeID,
		readCh:  make(chan []byte, 64),
		writeCh: make(chan []byte, 64),
		onClose: onClose,
	}

	go pc.readLoop()
	go pc.writeLoop()

	return pc
}

func (p *PeerConn) readLoop() {
	log := telemetry.Logger()
	for {
		p.mu.RLock()
		if p.closed {
			p.mu.RUnlock()
			return
		}
		p.mu.RUnlock()

		msg, err := p.readMessage()
		if err != nil {
			log.Error("peer.readLoop: readMessage failed", telemetry.FieldError(err))
			p.Close()
			return
		}
		p.readCh <- msg
	}
}

func (p *PeerConn) writeLoop() {
	log := telemetry.Logger()
	for {
		select {
		case msg := <-p.writeCh:
			if err := p.writeMessage(msg); err != nil {
				log.Error("peer.writeLoop: writeMessage failed", telemetry.FieldError(err))
				p.Close()
				return
			}
		default:
			time.Sleep(10 * time.Millisecond)
		}

		p.mu.RLock()
		if p.closed {
			p.mu.RUnlock()
			return
		}
		p.mu.RUnlock()
	}
}

func (p *PeerConn) readMessage() ([]byte, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(p.conn, lenBuf[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(lenBuf[:])

	msg := make([]byte, size)
	if _, err := io.ReadFull(p.conn, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func (p *PeerConn) writeMessage(msg []byte) error {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(msg)))
	if _, err := p.conn.Write(lenBuf[:]); err != nil {
		return err
	}
	_, err := p.conn.Write(msg)
	return err
}

func (p *PeerConn) SendChunkRequest(digest [32]byte) error {
	log := telemetry.Logger()
	log.Debug("peer.SendChunkRequest: sending", telemetry.FieldString("digest", fmt.Sprintf("%x", digest[:12])))

	msg := []byte{byte(MsgChunkRequest)}
	msg = append(msg, digest[:]...)
	return p.send(msg)
}

func (p *PeerConn) SendChunkResponse(digest [32]byte, data []byte) error {
	msg := []byte{byte(MsgChunkResponse)}
	msg = append(msg, digest[:]...)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(data)))
	msg = append(msg, lenBuf...)
	msg = append(msg, data...)
	return p.send(msg)
}

func (p *PeerConn) SendFindNode(target [32]byte) error {
	msg := []byte{byte(MsgFindNode)}
	msg = append(msg, target[:]...)
	return p.send(msg)
}

func (p *PeerConn) send(msg []byte) error {
	select {
	case p.writeCh <- msg:
		return nil
	default:
		return fmt.Errorf("send: write channel full")
	}
}

func (p *PeerConn) ReadCh() <-chan []byte {
	return p.readCh
}

func (p *PeerConn) Close() error {
	log := telemetry.Logger()
	log.Info("peer.Close: closing", telemetry.FieldString("nodeID", p.nodeID.String()))

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true
	p.conn.Close()
	if p.onClose != nil {
		p.onClose(p.nodeID)
	}
	return nil
}

func (p *PeerConn) RemoteAddr() string {
	return p.conn.RemoteAddr().String()
}

func Dial(addr string, nodeID NodeID, onClose func(NodeID)) (*PeerConn, error) {
	log := telemetry.Logger()
	log.Info("peer.Dial: connecting to", telemetry.FieldString("addr", addr))

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("Dial: %w", err)
	}

	return NewPeerConn(conn, nodeID, onClose), nil
}

type Listener struct {
	ln     net.Listener
	onConn func(*PeerConn)
	mu     sync.RWMutex
	closed bool
}

func NewListener(addr string, onConn func(*PeerConn)) (*Listener, error) {
	log := telemetry.Logger()
	log.Info("peer.NewListener: listening on", telemetry.FieldString("addr", addr))

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("NewListener: %w", err)
	}

	l := &Listener{
		ln:     ln,
		onConn: onConn,
	}

	go l.acceptLoop()

	return l, nil
}

func (l *Listener) acceptLoop() {
	log := telemetry.Logger()
	for {
		_, err := l.ln.Accept()
		if err != nil {
			l.mu.RLock()
			if l.closed {
				l.mu.RUnlock()
				return
			}
			l.mu.RUnlock()
			log.Error("peer.acceptLoop: Accept failed", telemetry.FieldError(err))
			continue
		}

		go func() {
			conn, err := l.ln.Accept()
			if err != nil {
				return
			}
			pc := NewPeerConn(conn, NodeID{}, func(id NodeID) {})
			l.mu.RLock()
			if l.onConn != nil {
				l.onConn(pc)
			}
			l.mu.RUnlock()
		}()
	}
}

func (l *Listener) Close() error {
	l.mu.Lock()
	l.closed = true
	l.mu.Unlock()
	return l.ln.Close()
}

func ParseMessage(msg []byte) (MessageType, interface{}) {
	if len(msg) < 1 {
		return 0, nil
	}
	switch MessageType(msg[0]) {
	case MsgChunkRequest:
		var req ChunkRequest
		copy(req.Digest[:], msg[1:33])
		return MsgChunkRequest, req
	case MsgChunkResponse:
		var resp ChunkResponse
		copy(resp.Digest[:], msg[1:33])
		if len(msg) > 37 {
			size := binary.BigEndian.Uint32(msg[33:37])
			if int(size) <= len(msg)-37 {
				resp.Data = msg[37 : 37+size]
			}
		}
		return MsgChunkResponse, resp
	case MsgFindNode:
		var req FindNodeRequest
		copy(req.Target[:], msg[1:33])
		return MsgFindNode, req
	}
	return 0, nil
}