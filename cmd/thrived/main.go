//go:build linux

package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.Printf("thrived: starting...")

	// Set up signal handler for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigCh
		log.Printf("thrived: received signal, shutting down...")
		cancel()
	}()

	// Start Unix socket server
	socketPath := "/var/run/thrive-daemon.sock"
	if err := os.RemoveAll(socketPath); err != nil {
		log.Fatalf("thrived: failed to remove old socket: %v", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("thrived: failed to listen on %s: %v", socketPath, err)
	}

	// Chmod socket so CLI can connect (restrict to owner and group only)
	if err := os.Chmod(socketPath, 0660); err != nil {
		log.Fatalf("thrived: failed to chmod socket: %v", err)
	}

	log.Printf("thrived: listening on %s", socketPath)

	go serveVsock(ctx, vsockPort())

	// Accept connections and serve
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				log.Printf("thrived: shutting down")
				return
			default:
				log.Printf("thrived: accept error: %v", err)
				continue
			}
		}

		go handleConn(conn, ctx)
	}
}
