//go:build !linux

package commands

import (
	"bytes"
	"context"
	encb64 "encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/registry"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func CpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cp [CONTAINER:]SRC [CONTAINER:]DEST",
		Short: "Copy files or directories between a container and the local filesystem",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			src, dst := args[0], args[1]

			containerID, srcPath, dstPath, toContainer := parseCpArgs(src, dst)
			if containerID == "" {
				return fmt.Errorf("cp: one argument must be in CONTAINER:PATH form")
			}

			if toContainer {
				return cpProxyToContainer(ctx, containerID, srcPath, dstPath)
			}
			return cpProxyFromContainer(ctx, containerID, srcPath, dstPath)
		},
	}
}

func cpProxyToContainer(ctx context.Context, containerID, srcPath, dstPath string) error {
	fi, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("cp: read %s: %w", srcPath, err)
	}
	opts := map[string]any{"direction": "to", "dst_path": dstPath}
	if fi.IsDir() {
		var buf bytes.Buffer
		if err := registry.TarDirectory(srcPath, &buf); err != nil {
			return fmt.Errorf("cp: tar %s: %w", srcPath, err)
		}
		opts["data"] = encb64.StdEncoding.EncodeToString(buf.Bytes())
		opts["tar"] = true
	} else {
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("cp: read %s: %w", srcPath, err)
		}
		opts["data"] = encb64.StdEncoding.EncodeToString(data)
	}
	if _, err := vm.DialControl(ctx, "cp", []string{containerID}, opts); err != nil {
		return fmt.Errorf("cp: %w", err)
	}
	fmt.Printf("Copied %s → %s:%s\n", srcPath, containerID, dstPath)
	return nil
}

func cpProxyFromContainer(ctx context.Context, containerID, srcPath, dstPath string) error {
	resp, err := vm.DialControl(ctx, "cp", []string{containerID}, map[string]any{
		"direction": "from",
		"src_path":  srcPath,
	})
	if err != nil {
		return fmt.Errorf("cp: %w", err)
	}
	var result map[string]any
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("cp: parse response: %w", err)
	}
	encoded, _ := result["data"].(string)
	raw, err := encb64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("cp: decode: %w", err)
	}
	if isTar, _ := result["tar"].(bool); isTar {
		target := dstPath
		if st, err := os.Stat(dstPath); err == nil && st.IsDir() {
			base := srcPath
			if i := strings.LastIndex(strings.TrimSuffix(srcPath, "/"), "/"); i >= 0 {
				base = srcPath[i+1:]
			}
			target = dstPath + "/" + base
		}
		if err := os.MkdirAll(target, 0755); err != nil {
			return fmt.Errorf("cp: mkdir %s: %w", target, err)
		}
		if err := registry.ExtractArchive(bytes.NewReader(raw), target); err != nil {
			return fmt.Errorf("cp: extract: %w", err)
		}
		fmt.Printf("Copied %s:%s → %s\n", containerID, srcPath, target)
		return nil
	}
	if err := os.WriteFile(dstPath, raw, 0644); err != nil {
		return fmt.Errorf("cp: write %s: %w", dstPath, err)
	}
	fmt.Printf("Copied %s:%s → %s\n", containerID, srcPath, dstPath)
	return nil
}

func parseCpArgs(src, dst string) (containerID, srcPath, dstPath string, toContainer bool) {
	if i := strings.Index(src, ":"); i > 0 {
		return src[:i], src[i+1:], dst, false
	}
	if i := strings.Index(dst, ":"); i > 0 {
		return dst[:i], src, dst[i+1:], true
	}
	return "", src, dst, false
}
