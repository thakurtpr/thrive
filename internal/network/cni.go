//go:build linux

package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const cniPluginDir = "/opt/cni/bin"

// CNIPortMapping is the wire format used in the CNI port-mappings capability.
type CNIPortMapping struct {
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"hostIP,omitempty"`
}

type cniNetConf struct {
	CNIVersion   string          `json:"cniVersion"`
	Name         string          `json:"name"`
	Type         string          `json:"type"`
	Bridge       string          `json:"bridge"`
	IsGateway    bool            `json:"isGateway"`
	IPMasq       bool            `json:"ipMasq"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`
	IPAM         cniIPAMConf     `json:"ipam"`
	RuntimeConf  *cniRuntimeConf `json:"runtimeConfig,omitempty"`
}

type cniIPAMConf struct {
	Type   string       `json:"type"`
	Ranges [][]cniRange `json:"ranges"`
}

type cniRange struct {
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
}

type cniRuntimeConf struct {
	PortMappings []CNIPortMapping `json:"portMappings,omitempty"`
}

// CNISetup configures networking via standard CNI plugins when available under
// /opt/cni/bin, or returns an error directing the caller to use SetupContainer.
func CNISetup(containerID, netnsPath string, portMappings []CNIPortMapping) (string, error) {
	bridgeBin := filepath.Join(cniPluginDir, "bridge")
	hostLocalBin := filepath.Join(cniPluginDir, "host-local")
	if !fileExists(bridgeBin) || !fileExists(hostLocalBin) {
		return "", fmt.Errorf("network.CNISetup: CNI plugins not found in %s; use SetupContainer instead", cniPluginDir)
	}

	conf := cniNetConf{
		CNIVersion: "0.4.0",
		Name:       "thrive",
		Type:       "bridge",
		Bridge:     BridgeName,
		IsGateway:  true,
		IPMasq:     true,
		IPAM: cniIPAMConf{
			Type:   "host-local",
			Ranges: [][]cniRange{{{Subnet: Subnet, Gateway: "172.18.0.1"}}},
		},
	}
	if len(portMappings) > 0 {
		conf.Capabilities = map[string]bool{"portMappings": true}
		conf.RuntimeConf = &cniRuntimeConf{PortMappings: portMappings}
	}

	confJSON, err := json.Marshal(conf)
	if err != nil {
		return "", fmt.Errorf("network.CNISetup: marshal conf: %w", err)
	}

	cmd := exec.Command(bridgeBin)
	cmd.Env = append(os.Environ(),
		"CNI_COMMAND=ADD",
		"CNI_CONTAINERID="+containerID,
		"CNI_NETNS="+netnsPath,
		"CNI_IFNAME=eth0",
		"CNI_PATH="+cniPluginDir,
	)
	cmd.Stdin = bytes.NewReader(confJSON)

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("network.CNISetup: bridge plugin: %w", err)
	}

	var result struct {
		IPs []struct {
			Address string `json:"address"`
		} `json:"ips"`
	}
	if jsonErr := json.Unmarshal(out, &result); jsonErr == nil && len(result.IPs) > 0 {
		return result.IPs[0].Address, nil
	}
	return "", nil
}

// CNITeardown removes CNI-managed networking for a container.
func CNITeardown(containerID, netnsPath string) error {
	bridgeBin := filepath.Join(cniPluginDir, "bridge")
	if !fileExists(bridgeBin) {
		return nil
	}

	conf := cniNetConf{
		CNIVersion: "0.4.0",
		Name:       "thrive",
		Type:       "bridge",
		Bridge:     BridgeName,
		IPAM:       cniIPAMConf{Type: "host-local"},
	}
	confJSON, err := json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("network.CNITeardown: marshal conf: %w", err)
	}

	cmd := exec.Command(bridgeBin)
	cmd.Env = append(os.Environ(),
		"CNI_COMMAND=DEL",
		"CNI_CONTAINERID="+containerID,
		"CNI_NETNS="+netnsPath,
		"CNI_IFNAME=eth0",
		"CNI_PATH="+cniPluginDir,
	)
	cmd.Stdin = bytes.NewReader(confJSON)

	if out, err := cmd.Output(); err != nil {
		return fmt.Errorf("network.CNITeardown: bridge plugin: %w (output: %s)", err, out)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
