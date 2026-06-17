//go:build linux

package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	setupJournalLogging()
	log.Printf("thrived: starting...")

	// Configure DNS and network before anything else so image pulls work.
	configureNetwork()

	// Use the NAT gateway (192.168.64.1) as DNS via UDP — vfkit's Apple VF NAT
	// gateway acts as a DNS resolver. External DNS (8.8.8.8) may be filtered.
	// Fall back to 1.1.1.1 if the gateway doesn't respond.
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			// Try NAT gateway DNS first (always available in Apple VF NAT)
			conn, err := d.DialContext(ctx, "udp", "192.168.64.1:53")
			if err == nil {
				return conn, nil
			}
			// Fall back to Cloudflare DNS
			return d.DialContext(ctx, "udp", "1.1.1.1:53")
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigCh
		log.Printf("thrived: received signal, shutting down...")
		cancel()
	}()

	// vsock is the primary transport for desktop VM use.
	// Start it FIRST so boot detection works even if Unix socket setup fails.
	go serveVsock(ctx, vsockPort())

	// Unix socket for local in-VM use. FROM scratch containers may not have /var/run.
	// We log and continue in vsock-only mode rather than crashing.
	// Prefer systemd socket activation (LISTEN_FDS=1); fall back to self-bind.
	socketPath := "/var/run/thrive-daemon.sock"
	if ln := socketActivationListener(); ln != nil {
		log.Printf("thrived: listening on systemd socket")
		go serveUnix(ctx, ln)
		notifyReady()
	} else if err := os.MkdirAll("/var/run", 0755); err != nil {
		log.Printf("thrived: cannot create /var/run: %v (vsock-only mode)", err)
	} else {
		os.RemoveAll(socketPath)
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			log.Printf("thrived: cannot listen on %s: %v (vsock-only mode)", socketPath, err)
		} else {
			os.Chmod(socketPath, 0660)
			log.Printf("thrived: listening on %s", socketPath)
			go serveUnix(ctx, listener)
			notifyReady()
		}
	}

	<-ctx.Done()
	log.Printf("thrived: shut down")
}

func configureNetwork() {
	// Fix resolv.conf — LinuxKit's default may point to [::1]:53 (no server).
	// The kernel sets up IP/gateway via ip=dhcp; we only need to fix DNS.
	os.WriteFile("/etc/resolv.conf", []byte("nameserver 8.8.8.8\nnameserver 8.8.4.4\n"), 0644)
	log.Printf("thrived: DNS configured")

	// Mount the virtiofs share provided by vfkit (--device virtio-fs,mountTag=thrive-images)
	// This makes the macOS host's ~/.thrive/images/ available at /var/lib/thrive/images/
	// so images pulled on macOS are instantly available to thrived without VM internet access.
	imageDir := "/var/lib/thrive/images"
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		log.Printf("thrived: mkdir %s: %v", imageDir, err)
		return
	}
	if err := syscall.Mount("thrive-images", imageDir, "virtiofs", 0, ""); err != nil {
		log.Printf("thrived: virtiofs mount %s: %v (images may not be available)", imageDir, err)
	} else {
		log.Printf("thrived: virtiofs mounted at %s", imageDir)
	}
}

func serveUnix(ctx context.Context, listener net.Listener) {
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Printf("thrived: unix accept error: %v", err)
				continue
			}
		}
		go handleConn(conn, ctx)
	}
}
