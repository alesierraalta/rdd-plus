// Package testlocale pins the contribution boundary: the no-excess-tests routing destination is
// gitignored, and pure E2E/process suites live under testLocales/, never under internal/.
package testlocale

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot walks up from the package directory until it finds go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above package directory")
		}
		dir = parent
	}
}

// TestGitignoreListsTestLocales holds F82: without this entry, a CUT/E2E candidate has no
// gitignored destination and stays committed instead of being routed local-only.
func TestGitignoreListsTestLocales(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "testLocales/") {
		t.Fatalf(".gitignore must list testLocales/ so no-excess-tests can route local-only tests; got:\n%s", raw)
	}
}

// TestProcessE2ETestsAreNotUnderInternal holds F83: the pure process/E2E suites cited by the finding
// must not ship under internal/. Their home is testLocales/ (gitignored, absent from CI checkout).
func TestProcessE2ETestsAreNotUnderInternal(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"internal/gate/integration_test.go",
		"internal/bench/failure_test.go",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			t.Fatalf("%s must live under testLocales/, not the committed internal/ tree", rel)
		}
	}
}
