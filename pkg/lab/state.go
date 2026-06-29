package lab

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
)

const stateFileName = "cluster-state.json"

// ClusterState holds backups for restore after lab apply.
type ClusterState struct {
	Infrastructure *configv1.Infrastructure       `json:"infrastructure,omitempty"`
	ConfigMaps     map[string]corev1.ConfigMap    `json:"configMaps,omitempty"`
	Secrets        map[string]corev1.Secret       `json:"secrets,omitempty"`
}

func statePath(stateDir string) string {
	return filepath.Join(stateDir, stateFileName)
}

// SaveState writes cluster state to disk.
func SaveState(stateDir string, state *ClusterState) error {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	if err := os.WriteFile(statePath(stateDir), data, 0o600); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

// LoadState reads cluster state from disk.
func LoadState(stateDir string) (*ClusterState, error) {
	data, err := os.ReadFile(statePath(stateDir))
	if err != nil {
		return nil, fmt.Errorf("read state %q: %w", statePath(stateDir), err)
	}
	state := &ClusterState{}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return state, nil
}

// HasState reports whether a saved state file exists.
func HasState(stateDir string) bool {
	_, err := os.Stat(statePath(stateDir))
	return err == nil
}
