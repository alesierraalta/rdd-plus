// Package state is what tpp actually installed: per host, which components and files, with the
// digest it wrote, plus the feature toggles that survive updates.
package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// AssetRecord is one file we wrote for a host.
type AssetRecord struct {
	SHA256      string `json:"sha256"`
	Mode        uint32 `json:"mode"`
	FromVersion string `json:"fromVersion"`
}

// HookState records the Stop hook we wired.
type HookState struct {
	Command string `json:"command"`
	Wired   bool   `json:"wired"`
}

// HostState is one host's installation.
type HostState struct {
	ConfigDir  string                 `json:"configDir"`
	Components []string               `json:"components,omitempty"`
	Hook       *HookState             `json:"hook,omitempty"`
	Assets     map[string]AssetRecord `json:"assets,omitempty"`
}

// FeatureState is one optional feature's toggle.
type FeatureState struct {
	Enabled   bool   `json:"enabled"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// State is the whole installation record.
type State struct {
	SchemaVersion    int                     `json:"schemaVersion"`
	InstalledVersion string                  `json:"installedVersion"`
	Channel          string                  `json:"channel,omitempty"`
	UpdatedAt        string                  `json:"updatedAt,omitempty"`
	Hosts            map[string]HostState    `json:"hosts,omitempty"`
	Features         map[string]FeatureState `json:"features,omitempty"`
}

// Path resolves <root>/state.json. The root is $TPP_HOME when set, then the legacy $RDD_PLUS_HOME, otherwise
// the XDG config directory's tpp folder. An installation made before the rename lives in the XDG config
// directory's rdd-plus folder: when only that one exists it is moved to the new place once, and when the move
// cannot happen the old folder stays the root, so the installation is never split across two places.
func Path() (string, error) {
	root, err := resolveRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "state.json"), nil
}

// resolveRoot answers the directory that holds state.json, its backups and the update cache; see Path.
func resolveRoot() (string, error) {
	for _, name := range []string{"TPP_HOME", "RDD_PLUS_HOME"} {
		if root := os.Getenv(name); root != "" {
			return root, nil
		}
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve XDG config directory: %w", err)
	}
	return adoptLegacyRoot(filepath.Join(configDir, "rdd-plus"), filepath.Join(configDir, "tpp")), nil
}

// adoptLegacyRoot answers the root to use: current once it exists, else legacy moved to current, else legacy
// itself when the move fails. Anything but a clean absence of current leaves legacy untouched.
func adoptLegacyRoot(legacy, current string) string {
	if _, err := os.Lstat(current); !errors.Is(err, os.ErrNotExist) {
		return current
	}
	if info, err := os.Stat(legacy); err != nil || !info.IsDir() {
		return current
	}
	if err := os.Rename(legacy, current); err != nil {
		return legacy
	}
	return current
}

// Load answers an empty state when the file is absent, and an error when it exists but cannot be
// trusted — a corrupt state must fail closed, never be silently replaced.
func Load() (*State, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &State{SchemaVersion: 1}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state %s: %w", path, err)
	}
	state, err := decode(data, path)
	if err != nil {
		return nil, err
	}
	return state, nil
}

// Save serializes exactly the state it is given and writes only when the bytes would differ, so a
// second Save with the same value writes nothing and leaves the file byte-identical. It reports
// whether it wrote. Save never touches UpdatedAt: the caller decides that, because a no-op run must
// not churn the state.
func (s *State) Save() (bool, error) {
	if s == nil {
		return false, errors.New("cannot save a nil state")
	}
	if s.SchemaVersion != 1 {
		return false, fmt.Errorf("unsupported state schema version %d", s.SchemaVersion)
	}
	path, err := Path()
	if err != nil {
		return false, err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return false, fmt.Errorf("marshal state: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, fmt.Errorf("create state directory %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return false, fmt.Errorf("protect state directory %s: %w", dir, err)
	}

	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if _, err := decode(existing, path); err != nil {
			return false, fmt.Errorf("refuse to replace untrusted state: %w", err)
		}
		if bytes.Equal(existing, data) {
			return false, nil
		}
	case !errors.Is(err, os.ErrNotExist):
		return false, fmt.Errorf("read existing state %s: %w", path, err)
	}
	if err := writeAtomic(path, data); err != nil {
		return false, err
	}
	return true, nil
}

func decode(data []byte, path string) (*State, error) {
	var state *State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("state %s is corrupt: %w", path, err)
	}
	if state == nil {
		return nil, fmt.Errorf("state %s is corrupt: JSON null is not a state", path)
	}
	if state.SchemaVersion != 1 {
		return nil, fmt.Errorf("state %s has unsupported schema version %d", path, state.SchemaVersion)
	}
	return state, nil
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary state file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("protect temporary state file: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary state file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary state file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary state file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace state %s: %w", path, err)
	}
	return nil
}
