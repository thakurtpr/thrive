package network

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/thakurprasadrout/thrive/internal/events"
)

// Network is a named container network (docker network parity).
type Network struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Subnet     string            `json:"subnet"`
	Gateway    string            `json:"gateway"`
	Bridge     string            `json:"bridge"`
	Created    time.Time         `json:"created"`
	Containers map[string]string `json:"containers,omitempty"`
}

// Built-in default network backed by thrive0.
var defaultNetwork = Network{
	Driver:  "bridge",
	Subnet:  Subnet,
	Gateway: "172.18.0.1",
	Bridge:  BridgeName,
}

// networkDirOverride redirects the network store in tests.
var networkDirOverride string

func networkDir() string {
	if networkDirOverride != "" {
		return networkDirOverride
	}
	return "/var/lib/thrive/networks"
}

func networkPath(name string) string {
	return filepath.Join(networkDir(), name+".json")
}

func validNetworkName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if !isNameChar(c) {
			return false
		}
	}
	return true
}

// isNameChar reports whether c is allowed in a resource name.
func isNameChar(c rune) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	switch c {
	case '_', '.', '-':
		return true
	}
	return false
}

// bridgeFor derives a stable Linux bridge name for a custom network.
func bridgeFor(name string) string {
	sum := sha256.Sum256([]byte(name))
	return "br-" + hex.EncodeToString(sum[:])[:8]
}

// CreateNetwork creates a named bridge network. An empty subnet auto-allocates
// from 172.19.0.0/16-172.30.0.0/16. Bridge/NAT setup runs on Linux only;
// on other platforms the metadata is recorded and realised by the daemon.
func CreateNetwork(name, driver, subnet string) (*Network, error) {
	if !validNetworkName(name) {
		return nil, fmt.Errorf("network: invalid name %q", name)
	}
	if driver == "" {
		driver = "bridge"
	}
	if driver != "bridge" {
		return nil, fmt.Errorf("network: driver %q not supported (only bridge)", driver)
	}
	if name == "bridge" || name == "host" || name == "none" {
		return nil, fmt.Errorf("network: %q is a reserved network name", name)
	}
	if _, err := os.Stat(networkPath(name)); err == nil {
		return nil, fmt.Errorf("network: %s already exists", name)
	}
	if subnet == "" {
		var err error
		subnet, err = allocateSubnet()
		if err != nil {
			return nil, err
		}
	}
	nw := &Network{
		Name: name, Driver: driver, Subnet: subnet,
		Gateway:    subnetBase(subnet) + ".1",
		Bridge:     bridgeFor(name),
		Created:    time.Now().UTC(),
		Containers: map[string]string{},
	}
	if runtime.GOOS == "linux" {
		if err := EnsureBridgeWith(nw.Bridge, nw.Gateway+"/16"); err != nil {
			return nil, err
		}
	}
	if err := writeNetwork(nw); err != nil {
		return nil, err
	}
	events.Log("network", "create", name, map[string]string{"subnet": subnet})
	return nw, nil
}

// ListNetworks returns the built-in bridge network plus all custom networks.
func ListNetworks() ([]*Network, error) {
	def := defaultNetwork
	def.Name = "bridge"
	out := []*Network{&def}
	entries, err := os.ReadDir(networkDir())
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("network: list: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(networkDir(), e.Name()))
		if err != nil {
			continue
		}
		var nw Network
		if err := json.Unmarshal(data, &nw); err != nil {
			continue
		}
		cp := nw
		out = append(out, &cp)
	}
	return out, nil
}

// InspectNetwork returns a network by name ("bridge" resolves to built-in).
func InspectNetwork(name string) (*Network, error) {
	if name == "" || name == "bridge" {
		def := defaultNetwork
		def.Name = "bridge"
		return &def, nil
	}
	data, err := os.ReadFile(networkPath(name))
	if err != nil {
		return nil, fmt.Errorf("network: %s not found: %w", name, err)
	}
	var nw Network
	if err := json.Unmarshal(data, &nw); err != nil {
		return nil, fmt.Errorf("network: parse %s: %w", name, err)
	}
	return &nw, nil
}

// RemoveNetwork deletes a custom network. Refuses while containers are
// attached unless force is set.
func RemoveNetwork(name string, force bool) error {
	if name == "bridge" || name == "host" || name == "none" {
		return fmt.Errorf("network: %q is a reserved network and cannot be removed", name)
	}
	nw, err := InspectNetwork(name)
	if err != nil {
		return err
	}
	if len(nw.Containers) > 0 && !force {
		return fmt.Errorf("network: %s has %d attached container(s) (use --force)", name, len(nw.Containers))
	}
	if runtime.GOOS == "linux" {
		DeleteBridgeWith(nw.Bridge) //nolint:errcheck
	}
	if err := os.Remove(networkPath(name)); err != nil {
		return fmt.Errorf("network: remove: %w", err)
	}
	events.Log("network", "destroy", name, nil)
	return nil
}

// ConnectNetwork records a container attachment. Live-attach of a second
// interface to running containers is performed by the caller via SetupVethOn.
func ConnectNetwork(containerID, networkName string) error {
	if containerID == "" {
		return fmt.Errorf("network: container ID required")
	}
	nw, err := InspectNetwork(networkName)
	if err != nil {
		return err
	}
	if nw.Name == "bridge" {
		return nil // default attachment is implicit
	}
	if nw.Containers == nil {
		nw.Containers = map[string]string{}
	}
	nw.Containers[containerID] = containerID
	if err := writeNetwork(nw); err != nil {
		return err
	}
	events.Log("network", "connect", containerID, map[string]string{"network": nw.Name})
	return nil
}

// DisconnectNetwork removes a container attachment record.
func DisconnectNetwork(containerID, networkName string) error {
	nw, err := InspectNetwork(networkName)
	if err != nil {
		return err
	}
	if nw.Name == "bridge" {
		return nil
	}
	delete(nw.Containers, containerID)
	if err := writeNetwork(nw); err != nil {
		return err
	}
	events.Log("network", "disconnect", containerID, map[string]string{"network": nw.Name})
	return nil
}

// DetachContainer removes a container from every custom network record
// (called on container deletion).
func DetachContainer(containerID string) {
	nets, err := ListNetworks()
	if err != nil {
		return
	}
	for _, nw := range nets {
		if nw.Name == "bridge" {
			continue
		}
		if _, ok := nw.Containers[containerID]; ok {
			delete(nw.Containers, containerID)
			writeNetwork(nw) //nolint:errcheck
		}
	}
}

// PruneNetworks removes custom networks with no attached containers,
// returning the removed names.
func PruneNetworks() ([]string, error) {
	nets, err := ListNetworks()
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, nw := range nets {
		if nw.Name == "bridge" || len(nw.Containers) > 0 {
			continue
		}
		if err := RemoveNetwork(nw.Name, false); err == nil {
			removed = append(removed, nw.Name)
		}
	}
	return removed, nil
}

// AllocateOn returns the next free IP in a named network's subnet.
func AllocateOn(nw *Network, containerID string) (string, error) {
	dir := filepath.Join(networkDir(), "ipam-"+nw.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("network: ipam mkdir: %w", err)
	}
	counterPath := filepath.Join(dir, "counter")
	var counter int
	if data, err := os.ReadFile(counterPath); err == nil {
		counter, _ = strconv.Atoi(strings.TrimSpace(string(data)))
	}
	base := subnetBase(nw.Subnet)
	for i := 0; i < 65000; i++ {
		counter++
		ip := fmt.Sprintf("%s.%d.%d", base, (counter/254)%254, counter%254+2)
		if ip == nw.Gateway {
			continue
		}
		os.WriteFile(counterPath, []byte(strconv.Itoa(counter)), 0644)        //nolint:errcheck
		os.WriteFile(filepath.Join(dir, "ip-"+containerID), []byte(ip), 0644) //nolint:errcheck
		return ip, nil
	}
	return "", fmt.Errorf("network: subnet %s exhausted", nw.Subnet)
}

// ReleaseOn frees a container's IP in a named network.
func ReleaseOn(nw *Network, containerID string) {
	os.Remove(filepath.Join(networkDir(), "ipam-"+nw.Name, "ip-"+containerID)) //nolint:errcheck
}

func writeNetwork(nw *Network) error {
	if err := os.MkdirAll(networkDir(), 0755); err != nil {
		return fmt.Errorf("network: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(nw, "", "  ")
	if err != nil {
		return fmt.Errorf("network: marshal: %w", err)
	}
	if err := os.WriteFile(networkPath(nw.Name), data, 0644); err != nil {
		return fmt.Errorf("network: write: %w", err)
	}
	return nil
}

// subnetBase turns "172.19.0.0/16" into "172.19".
func subnetBase(subnet string) string {
	ip := strings.SplitN(subnet, "/", 2)[0]
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return "172.19"
	}
	return parts[0] + "." + parts[1]
}

// Attachment records one container↔network link for teardown and inspect.
type Attachment struct {
	Network     string `json:"network"`
	HostVeth    string `json:"hostVeth"`
	Interface   string `json:"interface"`
	ContainerIP string `json:"containerIP"`
}

func attachmentsPath(containerID string) string {
	return filepath.Join("/run/thrive/containers", containerID, "networks.json")
}

// RecordAttachment persists one container network attachment.
func RecordAttachment(containerID string, a Attachment) {
	var list []Attachment
	if data, err := os.ReadFile(attachmentsPath(containerID)); err == nil {
		json.Unmarshal(data, &list) //nolint:errcheck
	}
	for _, existing := range list {
		if existing.Network == a.Network {
			return
		}
	}
	list = append(list, a)
	data, err := json.Marshal(list)
	if err != nil {
		return
	}
	os.WriteFile(attachmentsPath(containerID), data, 0644) //nolint:errcheck
}

// Attachments returns recorded network attachments for a container.
func Attachments(containerID string) []Attachment {
	data, err := os.ReadFile(attachmentsPath(containerID))
	if err != nil {
		return nil
	}
	var list []Attachment
	if err := json.Unmarshal(data, &list); err != nil {
		return nil
	}
	return list
}

// ClearAttachments removes the attachment record file.
func ClearAttachments(containerID string) {
	os.Remove(attachmentsPath(containerID)) //nolint:errcheck
}

// TeardownAttachments removes every recorded veth/IP for a container.
func TeardownAttachments(containerID string) {
	for _, a := range Attachments(containerID) {
		nw, err := InspectNetwork(a.Network)
		if err != nil {
			nw = nil
		}
		TeardownVethOn(nw, containerID, a.HostVeth)
		if nw != nil && nw.Name != "bridge" {
			DisconnectNetwork(containerID, nw.Name) //nolint:errcheck
		}
	}
	ClearAttachments(containerID)
}

// allocateSubnet finds a free /16 in 172.19.0.0-172.30.0.0.
func allocateSubnet() (string, error) {
	used := map[string]bool{Subnet: true}
	nets, _ := ListNetworks()
	for _, nw := range nets {
		used[nw.Subnet] = true
	}
	for i := 19; i <= 30; i++ {
		candidate := fmt.Sprintf("172.%d.0.0/16", i)
		if !used[candidate] {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("network: no free subnet available")
}
