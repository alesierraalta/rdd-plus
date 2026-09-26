package state

import (
	"os"
	"path/filepath"
	"testing"
)

// useConfigDir clears both home overrides and points the XDG config directory at a throwaway one, so
// Path resolves the default root the way it does on a machine with no override set.
func useConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TPP_HOME", "")
	t.Setenv("RDD_PLUS_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTppHomeWinsOverTheLegacyHome(t *testing.T) {
	current, legacy := t.TempDir(), t.TempDir()
	t.Setenv("TPP_HOME", current)
	t.Setenv("RDD_PLUS_HOME", legacy)
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(current, "state.json"); got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

// An installation made before the rename lives under the old root; the first resolution moves it whole
// (state, backups, update cache), so nothing is left behind and nothing is read from two places.
func TestTheLegacyConfigRootMovesOnce(t *testing.T) {
	dir := useConfigDir(t)
	writeFile(t, filepath.Join(dir, "rdd-plus", "state.json"), `{"schemaVersion":1}`)
	writeFile(t, filepath.Join(dir, "rdd-plus", "backups", "b1", "settings.json"), `{}`)

	for i := 0; i < 2; i++ {
		got, err := Path()
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(dir, "tpp", "state.json"); got != want {
			t.Fatalf("Path() call %d = %q, want %q", i, got, want)
		}
	}
	if body, err := os.ReadFile(filepath.Join(dir, "tpp", "state.json")); err != nil || string(body) != `{"schemaVersion":1}` {
		t.Fatalf("moved state = %q, %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tpp", "backups", "b1", "settings.json")); err != nil {
		t.Fatalf("backups did not move with the root: %v", err)
	}
	// The old path becomes a link to the new root, so an rdd-plus binary still on the machine reads and writes
	// the same state instead of starting a second, empty one.
	legacy := filepath.Join(dir, "rdd-plus")
	if info, err := os.Lstat(legacy); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("legacy root is not a link to the new root after the move: %v %v", info, err)
	}
	if body, err := os.ReadFile(filepath.Join(legacy, "state.json")); err != nil || string(body) != `{"schemaVersion":1}` {
		t.Fatalf("state read through the legacy path = %q, %v; want the moved state", body, err)
	}
}

// Once the new root exists it is the only one used: a legacy root left beside it is never merged in.
func TestAnExistingNewRootWinsOverTheLegacyRoot(t *testing.T) {
	dir := useConfigDir(t)
	writeFile(t, filepath.Join(dir, "rdd-plus", "state.json"), `{"schemaVersion":1,"installedVersion":"old"}`)
	writeFile(t, filepath.Join(dir, "tpp", "state.json"), `{"schemaVersion":1,"installedVersion":"new"}`)

	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "tpp", "state.json"); got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "rdd-plus", "state.json")); err != nil {
		t.Fatalf("legacy root was touched: %v", err)
	}
}

// A move that cannot happen must not strand the installation: the old root stays the one read and written.
func TestAFailedMoveKeepsUsingTheLegacyRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := useConfigDir(t)
	writeFile(t, filepath.Join(dir, "rdd-plus", "state.json"), `{"schemaVersion":1}`)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "rdd-plus", "state.json"); got != want {
		t.Fatalf("Path() = %q, want the legacy %q", got, want)
	}
	if body, err := os.ReadFile(got); err != nil || string(body) != `{"schemaVersion":1}` {
		t.Fatalf("legacy state = %q, %v", body, err)
	}
}

func TestAFreshMachineResolvesTheNewRoot(t *testing.T) {
	dir := useConfigDir(t)
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "tpp", "state.json"); got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}
