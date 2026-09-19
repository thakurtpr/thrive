//go:build linux

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

// applyPsFilters filters by status=/name=/image= and drops stopped unless
// all. Delegates to the shared portable helper so Linux and the VM-daemon
// proxy filter identically.
func applyPsFilters(entries []psEntry, filters []string, all bool) []psEntry {
	rows := make([]psRow, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, psRow{ID: e.ID, Image: e.Image, Status: e.Status, PID: e.PID})
	}
	kept := filterPsRows(rows, filters, all)
	out := make([]psEntry, 0, len(kept))
	for _, r := range kept {
		out = append(out, psEntry{ID: r.ID, Image: r.Image, Status: r.Status, PID: r.PID})
	}
	return out
}

func psEntryMap(e psEntry) map[string]any {
	return psRowMap(psRow{ID: e.ID, Image: e.Image, Status: e.Status, PID: e.PID})
}
