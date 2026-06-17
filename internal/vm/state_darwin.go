//go:build darwin

package vm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// PersistedContainer is a lightweight record of a container run stored on the
// macOS host. In-VM container state is ephemeral; this survives VM restarts.
type PersistedContainer struct {
	ID        string    `json:"id"`
	Image     string    `json:"image"`
	Command   []string  `json:"command"`
	StartedAt time.Time `json:"started_at"`
	Status    string    `json:"status"`
}

func persistedStateDir() string {
	return filepath.Join(ThriveDir(), "state")
}

// SaveContainerRecord writes a container record to ~/.thrive/state/<id>.json.
func SaveContainerRecord(c PersistedContainer) error {
	dir := persistedStateDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, c.ID+".json"), data, 0644)
}

// LoadContainerRecords returns all persisted container records from the host.
// Returns nil (not error) when the state directory does not exist.
func LoadContainerRecords() ([]PersistedContainer, error) {
	dir := persistedStateDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var records []PersistedContainer
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var c PersistedContainer
		if err := json.Unmarshal(data, &c); err != nil {
			continue
		}
		records = append(records, c)
	}
	return records, nil
}
