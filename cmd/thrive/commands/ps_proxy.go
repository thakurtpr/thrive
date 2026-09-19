//go:build !linux

package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func PsCmd() *cobra.Command {
	var allFlag, quiet bool
	var filters []string
	var format string
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "ps", nil, nil)
			if err != nil {
				return err
			}

			var result map[string]any
			json.Unmarshal(data, &result) //nolint:errcheck

			containers, _ := result["containers"].([]any)
			type row struct {
				id, image, status string
				pid               int
			}
			var rows []row
			for _, c := range containers {
				cm, ok := c.(map[string]any)
				if !ok {
					continue
				}
				id, _ := cm["id"].(string)
				image, _ := cm["image"].(string)
				status, _ := cm["status"].(string)
				if cfg, ok := cm["config"].(map[string]any); ok && image == "" {
					image, _ = cfg["Image"].(string)
				}
				pid := 0
				if pf, ok := cm["pid"].(float64); ok {
					pid = int(pf)
				}
				rows = append(rows, row{id, image, status, pid})
			}

			var fStatus, fName, fImage string
			for _, f := range filters {
				k, v, ok := strings.Cut(f, "=")
				if !ok {
					continue
				}
				switch strings.TrimSpace(k) {
				case "status":
					fStatus = v
				case "name":
					fName = v
				case "image":
					fImage = v
				}
			}
			var kept []row
			for _, r := range rows {
				if !allFlag && r.status == "stopped" {
					continue
				}
				if fStatus != "" && r.status != fStatus {
					continue
				}
				if fName != "" && !strings.Contains(r.id, fName) {
					continue
				}
				if fImage != "" && !strings.Contains(r.image, fImage) {
					continue
				}
				kept = append(kept, r)
			}

			if quiet {
				for _, r := range kept {
					fmt.Println(r.id)
				}
				return nil
			}
			if format != "" {
				for _, r := range kept {
					out, err := renderFormat(format, map[string]any{
						"id": r.id, "image": r.image, "status": r.status, "pid": r.pid,
					})
					if err != nil {
						return err
					}
					fmt.Println(out)
				}
				return nil
			}
			if len(kept) == 0 {
				fmt.Println("no containers running")
				return nil
			}
			fmt.Printf("%-13s %-20s %-10s %s\n", "CONTAINER ID", "IMAGE", "STATUS", "PID")
			fmt.Println("────────────────────────────────────────────────────")
			for _, r := range kept {
				id := r.id
				if len(id) > 12 {
					id = id[:12]
				}
				fmt.Printf("%-13s %-20s %-10s %d\n", id, r.image, r.status, r.pid)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&allFlag, "all", "a", false, "Show all containers (default shows running)")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Only show container IDs")
	cmd.Flags().StringArrayVar(&filters, "filter", nil, "Filter output (status=, name=, image=)")
	cmd.Flags().StringVar(&format, "format", "", "Format output with a Go template")
	return cmd
}
