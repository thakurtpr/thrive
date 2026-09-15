//go:build linux

package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/swarm"
	"github.com/thakurprasadrout/thrive/pkg/compose"
)

// StackCmd deploys compose applications as swarm stacks.
func StackCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage stacks (compose applications on swarm)",
	}

	deploy := &cobra.Command{
		Use:   "deploy [stack]",
		Short: "Deploy a stack from a compose file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cf, err := compose.Load(file)
			if err != nil {
				return err
			}
			st, err := swarm.DeployStack(context.Background(), args[0], cf)
			if err != nil {
				return err
			}
			fmt.Printf("Deployed stack %s (%d service(s))\n", st.Name, len(st.Services))
			return nil
		},
	}
	deploy.Flags().StringVarP(&file, "file", "f", "docker-compose.yml", "Compose file path")

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List stacks",
		RunE: func(cmd *cobra.Command, args []string) error {
			stacks, err := swarm.ListStacks()
			if err != nil {
				return err
			}
			fmt.Printf("%-20s %-9s %s\n", "NAME", "SERVICES", "DEPLOYED")
			for _, st := range stacks {
				fmt.Printf("%-20s %-9d %s\n", st.Name, len(st.Services), st.Deployed.Format("2006-01-02 15:04"))
			}
			return nil
		},
	}

	psSt := &cobra.Command{
		Use:   "ps [stack]",
		Short: "List stack tasks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := swarm.InspectStack(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("%-30s %-10s\n", "SERVICE", "SPEC")
			for _, svc := range st.Services {
				s, err := swarm.InspectService(svc)
				if err != nil {
					continue
				}
				fmt.Printf("%-30s %s x%d\n", svc, s.Spec.Image, s.Spec.Replicas)
			}
			return nil
		},
	}

	services := &cobra.Command{
		Use:   "services [stack]",
		Short: "List stack services",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := swarm.InspectStack(args[0])
			if err != nil {
				return err
			}
			for _, svc := range st.Services {
				fmt.Println(svc)
			}
			return nil
		},
	}

	rmSt := &cobra.Command{
		Use:   "rm [stack]",
		Short: "Remove a stack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := swarm.RemoveStack(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Printf("Removed stack %s\n", args[0])
			return nil
		},
	}

	cmd.AddCommand(deploy, ls, psSt, services, rmSt)
	return cmd
}
