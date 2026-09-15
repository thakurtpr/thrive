//go:build linux

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/runtime"
	"github.com/thakurprasadrout/thrive/internal/swarm"
)

// ServiceCmd manages replicated services.
func ServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage replicated services",
	}

	create := &cobra.Command{
		Use:   "create [image] [command...]",
		Short: "Create a replicated service",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			if name == "" {
				return fmt.Errorf("service create: --name is required")
			}
			replicas, _ := cmd.Flags().GetInt("replicas")
			envVars, _ := cmd.Flags().GetStringArray("env")
			portSpecs, _ := cmd.Flags().GetStringArray("publish")
			volumeSpecs, _ := cmd.Flags().GetStringArray("volume")
			netMode, _ := cmd.Flags().GetString("network")
			parallelism, _ := cmd.Flags().GetInt("update-parallelism")
			delay, _ := cmd.Flags().GetInt("update-delay")

			ports, err := parsePortSpecs(portSpecs)
			if err != nil {
				return err
			}
			mounts, err := parseVolumeSpecs(volumeSpecs)
			if err != nil {
				return err
			}
			svc, err := swarm.CreateService(context.Background(), name, swarm.ServiceSpec{
				Image: args[0], Command: args[1:], Env: envVars,
				Replicas: replicas, Ports: ports, Mounts: mounts,
				NetworkMode: netMode, Parallelism: parallelism, DelaySecs: delay,
			})
			if err != nil {
				return err
			}
			fmt.Printf("Created service %s (%d replica(s))\n", svc.Name, svc.Spec.Replicas)
			return nil
		},
	}
	create.Flags().String("name", "", "Service name (required)")
	create.Flags().IntP("replicas", "r", 1, "Replica count")
	create.Flags().StringArrayP("env", "e", nil, "Environment variables")
	create.Flags().StringArrayP("publish", "p", nil, "Publish ports host:container[/proto]")
	create.Flags().StringArrayP("volume", "v", nil, "Volumes name:/container or /host:/container")
	create.Flags().String("network", "", "Network mode or network name")
	create.Flags().Int("update-parallelism", 1, "Rolling update batch size")
	create.Flags().Int("update-delay", 0, "Seconds between update batches")
	create.Flags().SetInterspersed(false)

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List services",
		RunE: func(cmd *cobra.Command, args []string) error {
			services, err := swarm.ListServices()
			if err != nil {
				return err
			}
			fmt.Printf("%-20s %-30s %-9s %s\n", "NAME", "IMAGE", "REPLICAS", "UPDATED")
			for _, svc := range services {
				fmt.Printf("%-20s %-30s %-9d %s\n",
					svc.Name, svc.Spec.Image, svc.Spec.Replicas, svc.Updated.Format("2006-01-02 15:04"))
			}
			return nil
		},
	}

	inspectSvc := &cobra.Command{
		Use:   "inspect [service]",
		Short: "Display service details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := swarm.InspectService(args[0])
			if err != nil {
				return err
			}
			out, _ := json.MarshalIndent(svc, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	psSvc := &cobra.Command{
		Use:   "ps [service]",
		Short: "List service tasks (containers)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := swarm.InspectService(args[0]); err != nil {
				return err
			}
			entries, err := os.ReadDir("/run/thrive/containers")
			if err != nil {
				return err
			}
			prefix := "swarm-" + args[0] + "-"
			fmt.Printf("%-30s %-10s %s\n", "ID", "STATUS", "PID")
			for _, e := range entries {
				if !e.IsDir() || len(e.Name()) <= len(prefix) || e.Name()[:len(prefix)] != prefix {
					continue
				}
				state, err := runtime.State(context.Background(), e.Name())
				if err != nil {
					continue
				}
				fmt.Printf("%-30s %-10s %d\n", e.Name(), state.Status, state.PID)
			}
			return nil
		},
	}

	rmSvc := &cobra.Command{
		Use:   "rm [service]",
		Short: "Remove a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := swarm.RemoveService(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Printf("Removed service %s\n", args[0])
			return nil
		},
	}

	scale := &cobra.Command{
		Use:   "scale [service=num...]",
		Short: "Scale services",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, s := range args {
				name, n, ok := splitScale(s)
				if !ok {
					return fmt.Errorf("invalid scale %q (want service=num)", s)
				}
				if err := swarm.ScaleService(context.Background(), name, n); err != nil {
					return err
				}
				fmt.Printf("Scaled %s to %d\n", name, n)
			}
			return nil
		},
	}

	logsSvc := &cobra.Command{
		Use:   "logs [service]",
		Short: "Show service task logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := os.ReadDir("/run/thrive/containers")
			if err != nil {
				return err
			}
			prefix := "swarm-" + args[0] + "-"
			for _, e := range entries {
				if !e.IsDir() || len(e.Name()) <= len(prefix) || e.Name()[:len(prefix)] != prefix {
					continue
				}
				data, err := os.ReadFile("/run/thrive/containers/" + e.Name() + "/logs")
				if err == nil {
					fmt.Printf("==> %s <==\n%s\n", e.Name(), string(data))
				}
			}
			return nil
		},
	}

	update := &cobra.Command{
		Use:   "update [service]",
		Short: "Rolling-update a service image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			newImage, _ := cmd.Flags().GetString("image")
			if err := swarm.UpdateService(context.Background(), args[0], newImage); err != nil {
				return err
			}
			fmt.Printf("Updated service %s\n", args[0])
			return nil
		},
	}
	update.Flags().String("image", "", "New image (required)")

	rollback := &cobra.Command{
		Use:   "rollback [service]",
		Short: "Roll back a service to its previous spec",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := swarm.RollbackService(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Printf("Rolled back service %s\n", args[0])
			return nil
		},
	}

	cmd.AddCommand(create, ls, inspectSvc, psSvc, rmSvc, scale, logsSvc, update, rollback)
	return cmd
}
