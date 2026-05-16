//go:build linux

package main

import (
	"context"
	"log"
	"net"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

func vsockPort() uint32 {
	if p := os.Getenv("THRIVE_VSOCK_PORT"); p != "" {
		v, _ := strconv.ParseUint(p, 10, 32)
		return uint32(v)
	}
	return 1024
}

func serveVsock(ctx context.Context, port uint32) {
	fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM, 0)
	if err != nil {
		log.Printf("thrived: vsock socket: %v", err)
		return
	}
	defer unix.Close(fd)

	sa := &unix.SockaddrVM{CID: unix.VMADDR_CID_ANY, Port: port}
	if err := unix.Bind(fd, sa); err != nil {
		log.Printf("thrived: vsock bind: %v", err)
		return
	}
	if err := unix.Listen(fd, 128); err != nil {
		log.Printf("thrived: vsock listen: %v", err)
		return
	}

	log.Printf("thrived: listening on vsock port %d", port)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		nfd, _, err := unix.Accept(fd)
		if err != nil {
			log.Printf("thrived: vsock accept: %v", err)
			continue
		}
		f := os.NewFile(uintptr(nfd), "vsock")
		conn, err := net.FileConn(f)
		f.Close()
		if err != nil {
			log.Printf("thrived: vsock FileConn: %v", err)
			continue
		}
		go handleConn(conn, ctx)
	}
}
