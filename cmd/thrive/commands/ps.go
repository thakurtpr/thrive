//go:build linux

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type psEntry struct {
	ID     string `json:"id"`
	Image  string `json:"image"`
	Status string `json:"status"`
	PID    int    `json:"pid"`
}

func PsCmd() *cobra.Command {
	var allFlag, quiet bool
	var filters []string
	var format string
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List containers",
		Run: func(cmd *cobra.Command, args []string) {
			_ = context.Background() // reserved for future use
			entries := listPsEntries()
			entries = applyPsFilters(entries, filters, allFlag)
			if quiet {
				for _, e := range entries {
					fmt.Println(e.ID)
				}
				return
			}
			if format != "" {
				for _, e := range entries {
					out, err := renderFormat(format, psEntryMap(e))
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: %v\n", err)
						os.Exit(1)
					}
					fmt.Println(out)
				}
				return
			}
			fmt.Printf("%-13s %-20s %-10s %s\n", "CONTAINER ID", "IMAGE", "STATUS", "PID")
			for _, e := range entries {
				id := e.ID
				if len(id) > 12 {
					id = id[:12]
				}
				fmt.Printf("%-13s %-20s %-10s %d\n", id, e.Image, e.Status, e.PID)
			}
		},
	}
	cmd.Flags().BoolVarP(&allFlag, "all", "a", false, "Show all containers (default shows running)")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Only show container IDs")
	cmd.Flags().StringArrayVar(&filters, "filter", nil, "Filter output (status=, name=, image=)")
	cmd.Flags().StringVar(&format, "format", "", "Format output with a Go template")
	return cmd
}

func listPsEntries() []psEntry {
	var out []psEntry
	entries, err := os.ReadDir("/run/thrive/containers")
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		stateData, err := os.ReadFile(filepath.Join("/run/thrive/containers", entry.Name(), "state.json"))
		if err != nil {
			continue
		}
		var state struct {
			ID     string
			Status string
			PID    int
		}
		if err := json.Unmarshal(stateData, &state); err != nil {
			continue
		}
		var cfg struct {
			Image string `json:"Image"`
		}
		if configData, err := os.ReadFile(filepath.Join("/run/thrive/containers", entry.Name(), "config.json")); err == nil {
			json.Unmarshal(configData, &cfg) //nolint:errcheck
		}
		out = append(out, psEntry{ID: state.ID, Image: cfg.Image, Status: state.Status, PID: state.PID})
	}
	return out
}

// applyPsFilters filters by status=/name=/image= and drops stopped unless all.
func applyPsFilters(entries []psEntry, filters []string, all bool) []psEntry {
	var status, name, image string
	for _, f := range filters {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "status":
			status = v
		case "name":
			name = v
		case "image":
			image = v
		}
	}
	var out []psEntry
	for _, e := range entries {
		if !all && e.Status == "stopped" {
			continue
		}
		if status != "" && e.Status != status {
			continue
		}
		if name != "" && !strings.Contains(e.ID, name) {
			continue
		}
		if image != "" && !strings.Contains(e.Image, image) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func psEntryMap(e psEntry) map[string]any {
	return map[string]any{"id": e.ID, "image": e.Image, "status": e.Status, "pid": e.PID}
}
