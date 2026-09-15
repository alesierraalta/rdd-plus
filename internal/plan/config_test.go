package plan

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeclaredPathReadsTheRepositoryDeclaration(t *testing.T) {
	root := t.TempDir()
	var readPath string
	got, err := DeclaredPath(root, func(path string) (string, error) {
		readPath = path
		return `{"planPath":"docs/testing/custom-plan.md"}`, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "docs/testing/custom-plan.md" {
		t.Fatalf("path = %q", got)
	}
	if readPath != filepath.Join(root, ConfigName) {
		t.Fatalf("read path = %q, want %q", readPath, filepath.Join(root, ConfigName))
	}
}

func TestDeclaredPathIsAbsentWhenTheRepositoryDeclaresNothing(t *testing.T) {
	got, err := DeclaredPath(t.TempDir(), func(string) (string, error) {
		return `{}`, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("path = %q, want empty", got)
	}
}

func TestResolvePathFallsBackToTheDefault(t *testing.T) {
	got, err := ResolvePath(t.TempDir(), func(string) (string, error) {
		return "", fs.ErrNotExist
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != DefaultPath {
		t.Fatalf("path = %q, want %q", got, DefaultPath)
	}
}

func TestResolveFromRootTakesAnAbsolutePathAsGiven(t *testing.T) {
	const absolute = "/tmp/absolute-plan.md"
	got, err := ResolveFromRoot("/repository", "--path", absolute)
	if err != nil {
		t.Fatal(err)
	}
	if got != absolute {
		t.Fatalf("path = %q, want %q", got, absolute)
	}
}

func TestResolveFromRootResolvesARelativePathAgainstTheRoot(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveFromRoot(root, "--path", "docs/testing/plan.md")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "docs/testing/plan.md")
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestResolveFromRootRefusesARelativeEscape(t *testing.T) {
	_, err := ResolveFromRoot("/repository", "--path", "../../outside.md")
	if err == nil || !strings.Contains(err.Error(), "--path") {
		t.Fatalf("error = %v, want an error naming --path", err)
	}
}

func TestDeclaredPathRefusesAnUnknownKey(t *testing.T) {
	_, err := DeclaredPath(t.TempDir(), func(string) (string, error) {
		return `{"planpath":"docs/testing/custom-plan.md"}`, nil
	})
	if err == nil || !strings.Contains(err.Error(), ConfigName) {
		t.Fatalf("error = %v, want an error naming %s", err, ConfigName)
	}
}

func TestDeclaredPathRefusesAnEmptyPlanPath(t *testing.T) {
	for _, value := range []string{`{"planPath":""}`, `{"planPath":"   "}`} {
		t.Run(value, func(t *testing.T) {
			_, err := DeclaredPath(t.TempDir(), func(string) (string, error) {
				return value, nil
			})
			if err == nil || !strings.Contains(err.Error(), ConfigName) {
				t.Fatalf("error = %v, want an error naming %s", err, ConfigName)
			}
		})
	}
}

func TestDeclaredPathRefusesAPathThatEscapesTheWorktree(t *testing.T) {
	_, err := DeclaredPath(t.TempDir(), func(string) (string, error) {
		return `{"planPath":"../outside.md"}`, nil
	})
	if err == nil || !strings.Contains(err.Error(), ConfigName) {
		t.Fatalf("error = %v, want an error naming %s", err, ConfigName)
	}
}

func TestResolvePathRefusesAMalformedDeclaration(t *testing.T) {
	_, err := ResolvePath(t.TempDir(), func(string) (string, error) {
		return `{`, nil
	})
	if err == nil || !strings.Contains(err.Error(), ConfigName) {
		t.Fatalf("error = %v, want an error naming %s", err, ConfigName)
	}
}

func TestDeclaredPathRefusesAnUnreadableDeclaration(t *testing.T) {
	_, err := DeclaredPath(t.TempDir(), func(string) (string, error) {
		return "", errors.New("permission denied")
	})
	if err == nil || !strings.Contains(err.Error(), ConfigName) || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("error = %v, want the config name and read error", err)
	}
}
