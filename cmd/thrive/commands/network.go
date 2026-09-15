//go:build linux

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/network"
	"github.com/thakurprasadrout/thrive/internal/runtime"
)

// NetworkCmd manages container networks (docker network parity).
func NetworkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage container networks",
	}

	create := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			driver, _ := cmd.Flags().GetString("driver")
			subnet, _ := cmd.Flags().GetString("subnet")
			nw, err := network.CreateNetwork(args[0], driver, subnet)
			if err != nil {
				return err
			}
			fmt.Printf("%s (%s)\n", nw.Name, nw.Subnet)
			return nil
		},
	}
	create.Flags().String("driver", "bridge", "Network driver (only bridge)")
	create.Flags().String("subnet", "", "Subnet in CIDR format (auto-allocated if empty)")

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			nets, err := network.ListNetworks()
			if err != nil {
				return err
			}
			fmt.Printf("%-15s %-8s %-18s %s\n", "NAME", "DRIVER", "SUBNET", "CONTAINERS")
			for _, nw := range nets {
				fmt.Printf("%-15s %-8s %-18s %d\n", nw.Name, nw.Driver, nw.Subnet, len(nw.Containers))
			}
			return nil
		},
	}

	inspect := &cobra.Command{
		Use:   "inspect [network]",
		Short: "Display detailed information about a network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nw, err := network.InspectNetwork(args[0])
			if err != nil {
				return err
			}
			out, _ := json.MarshalIndent(nw, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	rm := &cobra.Command{
		Use:   "rm [network...]",
		Short: "Remove networks",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			failed := false
			for _, name := range args {
				if err := network.RemoveNetwork(name, force); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					failed = true
					continue
				}
				fmt.Printf("Removed network %s\n", name)
			}
			if failed {
				return fmt.Errorf("one or more networks could not be removed")
			}
			return nil
		},
	}
	rm.Flags().BoolP("force", "f", false, "Remove despite attached containers")

	connect := &cobra.Command{
		Use:   "connect [network] [container]",
		Short: "Connect a container to a network",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return connectContainer(args[0], args[1])
		},
	}

	disconnect := &cobra.Command{
		Use:   "disconnect [network] [container]",
		Short: "Disconnect a container from a network",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			return disconnectContainer(args[0], args[1], force)
		},
	}
	disconnect.Flags().BoolP("force", "f", false, "Force disconnect")

	prune := &cobra.Command{
		Use:   "prune",
		Short: "Remove all unused networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			removed, err := network.PruneNetworks()
			if err != nil {
				return err
			}
			for _, name := range removed {
				fmt.Printf("Removed network %s\n", name)
			}
			fmt.Printf("Pruned %d network(s)\n", len(removed))
			return nil
		},
	}

	cmd.AddCommand(create, ls, inspect, rm, connect, disconnect, prune)
	return cmd
}

// connectContainer attaches a container: live second interface when running,
// pending attachment for the next start otherwise.
func connectContainer(networkName, containerID string) error {
	ctx := context.Background()
	if _, err := network.InspectNetwork(networkName); err != nil {
		return err
	}
	state, err := runtime.State(ctx, containerID)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	if state.Status != "running" {
		if err := runtime.AddPendingNetwork(containerID, networkName); err != nil {
			return err
		}
		if err := network.ConnectNetwork(containerID, networkName); err != nil {
			return err
		}
		fmt.Printf("Connected %s to %s (applies on next start)\n", containerID, networkName)
		return nil
	}
	nw, _ := network.InspectNetwork(networkName)
	iface := fmt.Sprintf("eth%d", len(network.Attachments(containerID)))
	if err := network.EnsureBridgeWith(nw.Bridge, nw.Gateway+"/16"); err != nil {
		return err
	}
	veth, err := network.SetupVethOn(nw, containerID, state.PID, iface)
	if err != nil {
		return err
	}
	if err := network.ConnectNetwork(containerID, networkName); err != nil {
		return err
	}
	network.RecordAttachment(containerID, network.Attachment{
		Network: nw.Name, HostVeth: veth.Host,
		Interface: iface, ContainerIP: veth.ContainerIP,
	})
	fmt.Printf("Connected %s to %s (%s)\n", containerID, networkName, veth.ContainerIP)
	return nil
}

// disconnectContainer detaches a container from a network.
func disconnectContainer(networkName, containerID string, force bool) error {
	ctx := context.Background()
	state, err := runtime.State(ctx, containerID)
	if err != nil {
		return fmt.Errorf("disconnect: %w", err)
	}
	if state.Status == "running" && !force {
		for _, a := range network.Attachments(containerID) {
			if a.Network == networkName && a.Interface == "eth0" {
				return fmt.Errorf("disconnect: %s is the primary network (use --force)", networkName)
			}
		}
	}
	nw, err := network.InspectNetwork(networkName)
	if err != nil {
		return err
	}
	for _, a := range network.Attachments(containerID) {
		if a.Network == networkName {
			network.TeardownVethOn(nw, containerID, a.HostVeth)
		}
	}
	// Drop the attachment record.
	remaining := []network.Attachment{}
	for _, a := range network.Attachments(containerID) {
		if a.Network != networkName {
			remaining = append(remaining, a)
		}
	}
	network.ClearAttachments(containerID)
	for _, a := range remaining {
		network.RecordAttachment(containerID, a)
	}
	if err := network.DisconnectNetwork(containerID, networkName); err != nil {
		return err
	}
	fmt.Printf("Disconnected %s from %s\n", containerID, networkName)
	return nil
}
