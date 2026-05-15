package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/runtime"
)

func LogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs [container]",
		Short: "View container logs",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			containerID := args[0]

			state, err := runtime.State(ctx, containerID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting container state: %v\n", err)
				os.Exit(1)
			}

			data, _ := json.MarshalIndent(state, "", "  ")
			fmt.Println(string(data))
		},
	}
}
