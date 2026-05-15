package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func PsCmd() *cobra.Command {
	var allFlag bool
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List containers",
		Run: func(cmd *cobra.Command, args []string) {
			_ = context.Background() // reserved for future use
			containersDir := "/run/thrive/containers"

			entries, err := os.ReadDir(containersDir)
			if err != nil {
				if os.IsNotExist(err) {
					return
				}
				fmt.Fprintf(os.Stderr, "Error reading containers: %v\n", err)
				return
			}

			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				statePath := filepath.Join(containersDir, entry.Name(), "state.json")
				data, err := os.ReadFile(statePath)
				if err != nil {
					continue
				}
				var state struct {
					ID     string
					Status string
					PID    int
				}
				json.Unmarshal(data, &state)

				if !allFlag && state.Status == "stopped" {
					continue
				}
				fmt.Printf("%s\t%s\t%d\n", state.ID, state.Status, state.PID)
			}
		},
	}
	cmd.Flags().BoolVarP(&allFlag, "all", "a", false, "Show all containers")
	return cmd
}
