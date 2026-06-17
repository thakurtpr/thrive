//go:build linux

package network

import (
	"fmt"
	"os"
	"path/filepath"
)

const resolvConf = "nameserver 8.8.8.8\nnameserver 8.8.4.4\nsearch .\n"

// WriteResolvConf writes resolv.conf into the container's merged rootfs so
// DNS works inside containers without a dedicated container DNS server.
func WriteResolvConf(rootfsPath string) error {
	if rootfsPath == "" {
		return nil
	}
	etcDir := filepath.Join(rootfsPath, "etc")
	if err := os.MkdirAll(etcDir, 0755); err != nil {
		return fmt.Errorf("network.WriteResolvConf: mkdir %s: %w", etcDir, err)
	}
	resolvPath := filepath.Join(etcDir, "resolv.conf")
	if err := os.WriteFile(resolvPath, []byte(resolvConf), 0644); err != nil {
		return fmt.Errorf("network.WriteResolvConf: write %s: %w", resolvPath, err)
	}
	return nil
}
