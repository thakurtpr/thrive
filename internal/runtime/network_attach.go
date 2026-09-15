//go:build linux
// +build linux

package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/thakurprasadrout/thrive/internal/network"
	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

func pendingNetworksPath(id string) string {
	return filepath.Join("/run/thrive/containers", id, "pending_networks.json")
}

// AddPendingNetwork records a network attachment for a stopped container,
// applied on the next Start.
func AddPendingNetwork(id, networkName string) error {
	var pending []string
	if data, err := os.ReadFile(pendingNetworksPath(id)); err == nil {
		json.Unmarshal(data, &pending) //nolint:errcheck
	}
	for _, p := range pending {
		if p == networkName {
			return nil
		}
	}
	pending = append(pending, networkName)
	data, err := json.Marshal(pending)
	if err != nil {
		return fmt.Errorf("runtime: marshal pending networks: %w", err)
	}
	if err := os.WriteFile(pendingNetworksPath(id), data, 0644); err != nil {
		return fmt.Errorf("runtime: write pending networks: %w", err)
	}
	return nil
}

// PendingNetworks returns networks awaiting attachment at next Start.
func PendingNetworks(id string) []string {
	data, err := os.ReadFile(pendingNetworksPath(id))
	if err != nil {
		return nil
	}
	var pending []string
	if err := json.Unmarshal(data, &pending); err != nil {
		return nil
	}
	return pending
}

func clearPendingNetworks(id string) {
	os.Remove(pendingNetworksPath(id)) //nolint:errcheck
}

// attachPendingNetworks attaches networks connected while the container was
// stopped, as eth1, eth2, ... Skips the primary network.
func attachPendingNetworks(id string, pid int, primary string, log *zap.Logger) {
	pending := PendingNetworks(id)
	if len(pending) == 0 {
		return
	}
	clearPendingNetworks(id)
	ifIdx := 1
	for _, name := range pending {
		if name == primary {
			continue
		}
		nw, err := network.InspectNetwork(name)
		if err != nil {
			log.Warn("runtime.Start: pending network not found",
				telemetry.FieldString("network", name), telemetry.FieldError(err))
			continue
		}
		if err := network.EnsureBridgeWith(nw.Bridge, nw.Gateway+"/16"); err != nil {
			log.Warn("runtime.Start: pending EnsureBridge failed",
				telemetry.FieldString("network", name), telemetry.FieldError(err))
			continue
		}
		ifName := fmt.Sprintf("eth%d", ifIdx)
		veth, err := network.SetupVethOn(nw, id, pid, ifName)
		if err != nil {
			log.Warn("runtime.Start: pending attach failed",
				telemetry.FieldString("network", name), telemetry.FieldError(err))
			continue
		}
		network.ConnectNetwork(id, name) //nolint:errcheck
		network.RecordAttachment(id, network.Attachment{
			Network: nw.Name, HostVeth: veth.Host,
			Interface: ifName, ContainerIP: veth.ContainerIP,
		})
		log.Info("runtime.Start: attached pending network",
			telemetry.FieldString("network", name),
			telemetry.FieldString("ip", veth.ContainerIP))
		ifIdx++
	}
}
