//go:build !linux

package commands

import (
	encb64 "encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func CpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cp [CONTAINER:]SRC [CONTAINER:]DEST",
		Short: "Copy files between a container and the local filesystem",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			src, dst := args[0], args[1]

			containerID, srcPath, dstPath, toContainer := parseCpArgs(src, dst)
			if containerID == "" {
				return fmt.Errorf("cp: one argument must be in CONTAINER:PATH form")
			}

			if toContainer {
				data, err := os.ReadFile(srcPath)
				if err != nil {
					return fmt.Errorf("cp: read %s: %w", srcPath, err)
				}
				_, err = vm.DialControl(ctx, "cp", []string{containerID}, map[string]any{
					"direction": "to",
					"dst_path":  dstPath,
					"data":      encb64.StdEncoding.EncodeToString(data),
				})
				if err != nil {
					return fmt.Errorf("cp: %w", err)
				}
				fmt.Printf("Copied %s → %s:%s\n", srcPath, containerID, dstPath)
			} else {
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
				fileData, err := encb64.StdEncoding.DecodeString(encoded)
				if err != nil {
					return fmt.Errorf("cp: decode: %w", err)
				}
				if err := os.WriteFile(dstPath, fileData, 0644); err != nil {
					return fmt.Errorf("cp: write %s: %w", dstPath, err)
				}
				fmt.Printf("Copied %s:%s → %s\n", containerID, srcPath, dstPath)
			}
			return nil
		},
	}
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
