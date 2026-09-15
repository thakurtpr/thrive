package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/contextstore"
)

// ContextCmd manages daemon contexts (portable: local records everywhere).
func ContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Manage daemon contexts",
	}

	create := &cobra.Command{
		Use: "create [name]", Short: "Create a context", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			desc, _ := cmd.Flags().GetString("description")
			endpoint, _ := cmd.Flags().GetString("docker-host")
			c, err := contextstore.Create(args[0], desc, endpoint)
			if err != nil {
				return err
			}
			fmt.Printf("Created context %s (%s)\n", c.Name, c.Endpoint)
			return nil
		},
	}
	create.Flags().String("description", "", "Context description")
	create.Flags().String("docker-host", "", "Daemon endpoint (default unix:///var/run/thrive-daemon.sock)")

	ls := &cobra.Command{
		Use: "ls", Short: "List contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			contexts, err := contextstore.List()
			if err != nil {
				return err
			}
			fmt.Printf("%-20s %-40s %s\n", "NAME", "ENDPOINT", "CURRENT")
			for _, c := range contexts {
				cur := ""
				if c.Current {
					cur = "*"
				}
				fmt.Printf("%-20s %-40s %s\n", c.Name, c.Endpoint, cur)
			}
			return nil
		},
	}

	inspectCtx := &cobra.Command{
		Use: "inspect [name]", Short: "Inspect a context", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := contextstore.Inspect(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Name: %s\nEndpoint: %s\n", c.Name, c.Endpoint)
			if c.Description != "" {
				fmt.Printf("Description: %s\n", c.Description)
			}
			return nil
		},
	}

	rm := &cobra.Command{
		Use: "rm [name]", Short: "Remove a context", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return contextstore.Remove(args[0])
		},
	}

	use := &cobra.Command{
		Use: "use [name]", Short: "Set the current context", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := contextstore.Use(args[0]); err != nil {
				return err
			}
			fmt.Printf("Current context is now %s\n", args[0])
			return nil
		},
	}

	cmd.AddCommand(create, ls, inspectCtx, rm, use)
	return cmd
}
