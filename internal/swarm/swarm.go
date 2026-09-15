//go:build linux
// +build linux

// Package swarm implements single-node orchestration: swarm membership,
// replicated services with rolling updates, and stack deploys.
// Multi-node Raft clustering is out of scope: join against a remote
// manager returns an explicit error (see Join).
package swarm

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const swarmDir = "/var/lib/thrive/swarm"

// swarmDirOverride redirects state in tests.
var swarmDirOverride string

func rootDir() string {
	if swarmDirOverride != "" {
		return swarmDirOverride
	}
	return swarmDir
}

func statePath() string { return filepath.Join(rootDir(), "state.json") }

// State is the local swarm membership record.
type State struct {
	Initialized  bool      `json:"initialized"`
	NodeID       string    `json:"nodeID"`
	Manager      bool      `json:"manager"`
	ManagerAddr  string    `json:"managerAddr"`
	Created      time.Time `json:"created"`
	WorkerToken  string    `json:"workerToken"`
	ManagerToken string    `json:"managerToken"`
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}

// Init initialises single-node swarm mode.
func Init(advertiseAddr string) (*State, error) {
	if st, err := Inspect(); err == nil && st.Initialized {
		return nil, fmt.Errorf("swarm: already initialised on node %s (leave first)", st.NodeID)
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "thrive-node"
	}
	st := &State{
		Initialized:  true,
		NodeID:       host + "-" + randomHex(4),
		Manager:      true,
		ManagerAddr:  advertiseAddr,
		Created:      time.Now().UTC(),
		WorkerToken:  "THRIVE-worker-" + randomHex(12),
		ManagerToken: "THRIVE-manager-" + randomHex(12),
	}
	if err := writeState(st); err != nil {
		return nil, err
	}
	return st, nil
}

// Leave exits swarm mode. Refuses while services exist unless force is set.
func Leave(force bool) error {
	st, err := Inspect()
	if err != nil || !st.Initialized {
		return fmt.Errorf("swarm: not initialised")
	}
	if !force {
		services, _ := ListServices()
		if len(services) > 0 {
			return fmt.Errorf("swarm: %d service(s) still running (use --force)", len(services))
		}
	}
	if err := os.Remove(statePath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("swarm: leave: %w", err)
	}
	return nil
}

// Inspect returns the swarm state.
func Inspect() (*State, error) {
	data, err := os.ReadFile(statePath())
	if err != nil {
		return nil, fmt.Errorf("swarm: not initialised: %w", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("swarm: parse state: %w", err)
	}
	return &st, nil
}

// RequireInitialised errors unless swarm mode is active.
func RequireInitialised() error {
	st, err := Inspect()
	if err != nil || !st.Initialized {
		return fmt.Errorf("swarm: not initialised (run `thrive swarm init`)")
	}
	return nil
}

// JoinToken returns the join token for a role, rotating it first when asked.
func JoinToken(role string, rotate bool) (string, error) {
	st, err := Inspect()
	if err != nil {
		return "", err
	}
	if role != "worker" && role != "manager" {
		return "", fmt.Errorf("swarm: role must be worker or manager")
	}
	if rotate {
		if role == "worker" {
			st.WorkerToken = "THRIVE-worker-" + randomHex(12)
		} else {
			st.ManagerToken = "THRIVE-manager-" + randomHex(12)
		}
		if err := writeState(st); err != nil {
			return "", err
		}
	}
	if role == "worker" {
		return st.WorkerToken, nil
	}
	return st.ManagerToken, nil
}

// Join refuses remote clustering: thrive runs single-node swarm mode.
func Join(addr, token string) error {
	return fmt.Errorf("swarm: multi-node clustering is not supported — this node runs single-node swarm mode")
}

func writeState(st *State) error {
	if err := os.MkdirAll(rootDir(), 0755); err != nil {
		return fmt.Errorf("swarm: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("swarm: marshal: %w", err)
	}
	if err := os.WriteFile(statePath(), data, 0644); err != nil {
		return fmt.Errorf("swarm: write: %w", err)
	}
	return nil
}
