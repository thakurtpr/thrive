//go:build windows

package commands

import (
	encb64 "encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

// On Windows there is no host-side image store: images live inside the
// Thrive VM, so store-backed commands proxy via the control socket.
// Login/logout/search/manifest are registry-local ops and stay native
// (see distribution_shared.go).

// SaveCmd proxies image export to the VM daemon (base64 tar transport).
func SaveCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "save [image...]",
		Short: "Save images to a tar archive",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "save", args, nil)
			if err != nil {
				return fmt.Errorf("save failed: %w", err)
			}
			var result map[string]any
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("save: parse response: %w", err)
			}
			raw, err := encb64.StdEncoding.DecodeString(result["data"].(string))
			if err != nil {
				return fmt.Errorf("save: decode: %w", err)
			}
			if output == "" || output == "-" {
				_, err = os.Stdout.Write(raw)
				return err
			}
			if err := os.WriteFile(output, raw, 0644); err != nil {
				return fmt.Errorf("save: write: %w", err)
			}
			fmt.Printf("Saved %d image(s) to %s\n", len(args), output)
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write to file instead of stdout")
	return cmd
}

// LoadCmd proxies image import to the VM daemon.
func LoadCmd() *cobra.Command {
	var input string
	cmd := &cobra.Command{
		Use:   "load",
		Short: "Load images from a tar archive",
		RunE: func(cmd *cobra.Command, args []string) error {
			var raw []byte
			var err error
			if input == "" || input == "-" {
				raw, err = readAllStdin()
			} else {
				raw, err = os.ReadFile(input)
			}
			if err != nil {
				return fmt.Errorf("load: %w", err)
			}
			data, err := vm.DialControl(cmd.Context(), "load", nil, map[string]any{
				"data": encb64.StdEncoding.EncodeToString(raw),
			})
			if err != nil {
				return fmt.Errorf("load failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			if refs, ok := result["images"].([]any); ok {
				for _, r := range refs {
					fmt.Printf("Loaded image: %v\n", r)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&input, "input", "i", "", "Read from file instead of stdin")
	return cmd
}

// ImportCmd proxies tarball import to the VM daemon.
func ImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import [file|-] [new-image]",
		Short: "Import a filesystem tarball as an image",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var raw []byte
			var err error
			if args[0] == "-" {
				raw, err = readAllStdin()
			} else {
				raw, err = os.ReadFile(args[0])
			}
			if err != nil {
				return fmt.Errorf("import: %w", err)
			}
			_, err = vm.DialControl(cmd.Context(), "import", []string{args[1]}, map[string]any{
				"data": encb64.StdEncoding.EncodeToString(raw),
			})
			if err != nil {
				return fmt.Errorf("import failed: %w", err)
			}
			fmt.Printf("Imported %s\n", args[1])
			return nil
		},
	}
	cmd.Flags().StringP("message", "m", "", "Commit message (recorded for compatibility)")
	return cmd
}

// HistoryCmd proxies history to the VM daemon.
func HistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history [image]",
		Short: "Show the layer history of an image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "history", args, nil)
			if err != nil {
				return fmt.Errorf("history failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			if layers, ok := result["layers"].([]any); ok {
				fmt.Printf("%-15s %-10s %s\n", "LAYER", "SIZE", "DIGEST")
				for i, l := range layers {
					lm, _ := l.(map[string]any)
					fmt.Printf("%-15s %-10v %v\n", fmt.Sprintf("layer-%d", i), lm["size"], lm["digest"])
				}
			}
			return nil
		},
	}
}
