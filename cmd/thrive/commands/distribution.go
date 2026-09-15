//go:build linux || darwin

package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/registry"
)

// SaveCmd saves images to a tar archive (docker save parity).
func SaveCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "save [image...]",
		Short: "Save images to a tar archive",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var w *os.File
			if output == "" || output == "-" {
				w = os.Stdout
			} else {
				f, err := os.Create(output)
				if err != nil {
					return fmt.Errorf("save: %w", err)
				}
				defer f.Close()
				w = f
			}
			if err := registry.SaveTo(context.Background(), image.StoreDir(), args, w); err != nil {
				return err
			}
			if output != "" && output != "-" {
				fmt.Printf("Saved %d image(s) to %s\n", len(args), output)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write to file instead of stdout")
	return cmd
}

// LoadCmd loads images from a tar archive (docker load parity).
func LoadCmd() *cobra.Command {
	var input string
	cmd := &cobra.Command{
		Use:   "load",
		Short: "Load images from a tar archive",
		RunE: func(cmd *cobra.Command, args []string) error {
			var r *os.File
			if input == "" || input == "-" {
				r = os.Stdin
			} else {
				f, err := os.Open(input)
				if err != nil {
					return fmt.Errorf("load: %w", err)
				}
				defer f.Close()
				r = f
			}
			loaded, err := registry.LoadFrom(context.Background(), image.StoreDir(), r)
			if err != nil {
				return err
			}
			for _, ref := range loaded {
				fmt.Printf("Loaded image: %s\n", ref)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&input, "input", "i", "", "Read from file instead of stdin")
	return cmd
}

// ImportCmd imports a container filesystem tarball as an image.
func ImportCmd() *cobra.Command {
	var message string
	cmd := &cobra.Command{
		Use:   "import [file|-] [new-image]",
		Short: "Import a filesystem tarball as an image",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var r *os.File
			if args[0] == "-" {
				r = os.Stdin
			} else {
				f, err := os.Open(args[0])
				if err != nil {
					return fmt.Errorf("import: %w", err)
				}
				defer f.Close()
				r = f
			}
			if err := registry.ImportTo(context.Background(), image.StoreDir(), r, args[1]); err != nil {
				return err
			}
			fmt.Printf("Imported %s\n", args[1])
			_ = message
			return nil
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "Commit message (recorded for compatibility)")
	return cmd
}

// HistoryCmd shows the layer history of an image.
func HistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history [image]",
		Short: "Show the layer history of an image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			layers, err := registry.HistoryTo(context.Background(), image.StoreDir(), args[0])
			if err != nil {
				return err
			}
			fmt.Printf("%-15s %-10s %s\n", "LAYER", "SIZE", "DIGEST")
			for _, l := range layers {
				digest := l.Digest
				if len(digest) > 19 {
					digest = digest[:19]
				}
				fmt.Printf("%-15s %-10d %s\n", fmt.Sprintf("layer-%d", l.Index), l.Size, digest)
			}
			return nil
		},
	}
}
