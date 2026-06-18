//go:build !linux

package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func PsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ps",
		Short: "List running containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "ps", nil, nil)
			if err != nil {
				return err
			}

			var result map[string]any
			json.Unmarshal(data, &result)

			containers, _ := result["containers"].([]any)
			if len(containers) == 0 {
				fmt.Println("no containers running")
				return nil
			}

			fmt.Printf("%-13s %-20s %-10s %s\n", "CONTAINER ID", "IMAGE", "STATUS", "PID")
			fmt.Println("────────────────────────────────────────────────────")
			for _, c := range containers {
				cm, ok := c.(map[string]any)
				if !ok {
					continue
				}
				id, _ := cm["id"].(string)
				if len(id) > 12 {
					id = id[:12]
				}
				image, _ := cm["image"].(string)
				status, _ := cm["status"].(string)
				pid := 0
				if pf, ok := cm["pid"].(float64); ok {
					pid = int(pf)
				}
				fmt.Printf("%-13s %-20s %-10s %d\n", id, image, status, pid)
			}
			return nil
		},
	}
}
