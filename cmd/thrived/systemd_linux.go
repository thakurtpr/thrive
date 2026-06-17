//go:build linux

package main

import (
	"log"
	"net"
	"os"
	"strconv"
	"syscall"
)

// socketActivationListener returns the listener passed by systemd socket
// activation (LISTEN_FDS=1, fd=3), or nil if not running under systemd.
func socketActivationListener() net.Listener {
	listenFds := os.Getenv("LISTEN_FDS")
	if listenFds == "" {
		return nil
	}
	n, err := strconv.Atoi(listenFds)
	if err != nil || n < 1 {
		return nil
	}
	const sdListenFdsStart = 3
	syscall.CloseOnExec(sdListenFdsStart)
	f := os.NewFile(uintptr(sdListenFdsStart), "systemd-socket")
	if f == nil {
		return nil
	}
	ln, err := net.FileListener(f)
	f.Close()
	if err != nil {
		log.Printf("thrived: systemd socket activation: %v", err)
		return nil
	}
	log.Printf("thrived: adopted systemd socket")
	return ln
}

// notifyReady sends sd_notify READY=1 to systemd when NOTIFY_SOCKET is set.
// Uses a raw Unix datagram — no external dependency required.
func notifyReady() {
	notifySocket := os.Getenv("NOTIFY_SOCKET")
	if notifySocket == "" {
		return
	}
	conn, err := net.Dial("unixgram", notifySocket)
	if err != nil {
		log.Printf("thrived: sd_notify dial: %v", err)
		return
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("READY=1\n")); err != nil {
		log.Printf("thrived: sd_notify write: %v", err)
	}
}

// setupJournalLogging suppresses Go's timestamp prefix when running under
// systemd (journal adds its own timestamps). Detected via INVOCATION_ID or
// JOURNAL_STREAM env vars set by systemd.
func setupJournalLogging() {
	if os.Getenv("INVOCATION_ID") == "" && os.Getenv("JOURNAL_STREAM") == "" {
		return
	}
	log.SetFlags(0)
	log.SetPrefix("thrived: ")
}
