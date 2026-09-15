//go:build linux

package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/swarm"
)

// SwarmCmd manages single-node swarm mode.
func SwarmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "swarm",
		Short: "Manage single-node swarm mode",
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialise swarm mode on this node",
		RunE: func(cmd *cobra.Command, args []string) error {
			addr, _ := cmd.Flags().GetString("advertise-addr")
			st, err := swarm.Init(addr)
			if err != nil {
				return err
			}
			fmt.Printf("Swarm initialised on %s (node %s)\n", st.ManagerAddr, st.NodeID)
			fmt.Printf("Worker join token: %s\n", st.WorkerToken)
			return nil
		},
	}
	initCmd.Flags().String("advertise-addr", "", "Advertised manager address")

	join := &cobra.Command{
		Use:   "join [address]",
		Short: "Join a swarm (single-node only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			token, _ := cmd.Flags().GetString("token")
			return swarm.Join(args[0], token)
		},
	}
	join.Flags().String("token", "", "Join token")

	leave := &cobra.Command{
		Use:   "leave",
		Short: "Leave swarm mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			if err := swarm.Leave(force); err != nil {
				return err
			}
			fmt.Println("Node left swarm mode")
			return nil
		},
	}
	leave.Flags().BoolP("force", "f", false, "Leave despite running services")

	inspect := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect swarm state",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := swarm.Inspect()
			if err != nil {
				return err
			}
			fmt.Printf("Node: %s (manager: %v)\n", st.NodeID, st.Manager)
			fmt.Printf("Address: %s\n", st.ManagerAddr)
			return nil
		},
	}

	token := &cobra.Command{
		Use:   "join-token [worker|manager]",
		Short: "Show or rotate a join token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rotate, _ := cmd.Flags().GetBool("rotate")
			tok, err := swarm.JoinToken(args[0], rotate)
			if err != nil {
				return err
			}
			fmt.Println(tok)
			return nil
		},
	}
	token.Flags().Bool("rotate", false, "Rotate the token")

	cmd.AddCommand(initCmd, join, leave, inspect, token)
	return cmd
}
