package plan

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// declares answers body for the current declaration and nothing for any other file, the way a repository
// that carries only .tpp.json reads.
func declares(body string) Reader {
	return declaresAs(ConfigName, body)
}

func declaresAs(name, body string) Reader {
	return func(path string) (string, error) {
		if filepath.Base(path) == name {
			return body, nil
		}
		return "", fs.ErrNotExist
	}
}

func TestDeclaredPathReadsTheRepositoryDeclaration(t *testing.T) {
	root := t.TempDir()
	read := declares(`{"planPath":"docs/testing/custom-plan.md"}`)
	var answered string
	got, err := DeclaredPath(root, func(path string) (string, error) {
		body, err := read(path)
		if err == nil {
			answered = path
		}
		return body, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "docs/testing/custom-plan.md" {
		t.Fatalf("path = %q", got)
	}
	if answered != filepath.Join(root, ConfigName) {
		t.Fatalf("declaration read from %q, want %q", answered, filepath.Join(root, ConfigName))
	}
}

func TestDeclaredPathIsAbsentWhenTheRepositoryDeclaresNothing(t *testing.T) {
	got, err := DeclaredPath(t.TempDir(), declares(`{}`))
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
	_, err := DeclaredPath(t.TempDir(), declares(`{"planpath":"docs/testing/custom-plan.md"}`))
	if err == nil || !strings.Contains(err.Error(), ConfigName) {
		t.Fatalf("error = %v, want an error naming %s", err, ConfigName)
	}
}

func TestDeclaredPathRefusesAnEmptyPlanPath(t *testing.T) {
	for _, value := range []string{`{"planPath":""}`, `{"planPath":"   "}`} {
		t.Run(value, func(t *testing.T) {
			_, err := DeclaredPath(t.TempDir(), declares(value))
			if err == nil || !strings.Contains(err.Error(), ConfigName) {
				t.Fatalf("error = %v, want an error naming %s", err, ConfigName)
			}
		})
	}
}

func TestDeclaredPathRefusesAPathThatEscapesTheWorktree(t *testing.T) {
	_, err := DeclaredPath(t.TempDir(), declares(`{"planPath":"../outside.md"}`))
	if err == nil || !strings.Contains(err.Error(), ConfigName) {
		t.Fatalf("error = %v, want an error naming %s", err, ConfigName)
	}
}

func TestResolvePathRefusesAMalformedDeclaration(t *testing.T) {
	_, err := ResolvePath(t.TempDir(), declares(`{`))
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

func TestDeclaredRunReadsTheDeclaration(t *testing.T) {
	got, err := DeclaredRun(t.TempDir(), declares(`{"planPath":"docs/testing/custom-plan.md","run":"redis-stream-pool"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "redis-stream-pool" {
		t.Fatalf("run = %q, want redis-stream-pool", got)
	}
}

func TestDeclaredRunIsAbsentWhenTheKeyIsAbsent(t *testing.T) {
	got, err := DeclaredRun(t.TempDir(), declares(`{"planPath":"docs/testing/custom-plan.md"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("run = %q, want empty", got)
	}
}

func TestDeclaredRunRefusesABadSlug(t *testing.T) {
	_, err := DeclaredRun(t.TempDir(), declares(`{"run":"Bad_Slug"}`))
	if err == nil || !strings.Contains(err.Error(), ConfigName) {
		t.Fatalf("error = %v, want a declaration error", err)
	}
}

func TestDeclaredRunRefusesAllAndNone(t *testing.T) {
	for _, run := range []string{"all", "none"} {
		t.Run(run, func(t *testing.T) {
			_, err := DeclaredRun(t.TempDir(), declares(`{"run":"`+run+`"}`))
			if err == nil || !strings.Contains(err.Error(), ConfigName) {
				t.Fatalf("error = %v, want a declaration error", err)
			}
		})
	}
}

func TestDeclaredRunRefusesAnEmptyValue(t *testing.T) {
	for _, value := range []string{`{"run":""}`, `{"run":"   "}`} {
		t.Run(value, func(t *testing.T) {
			_, err := DeclaredRun(t.TempDir(), declares(value))
			if err == nil || !strings.Contains(err.Error(), ConfigName) {
				t.Fatalf("error = %v, want a declaration error", err)
			}
		})
	}
}

func TestResolveReadsTheDeclarationOnceForPathAndRun(t *testing.T) {
	calls := 0
	read := declares(`{"planPath":"docs/testing/custom-plan.md","run":"redis-stream-pool"}`)
	path, run, err := Resolve(t.TempDir(), func(path string) (string, error) {
		if filepath.Base(path) == ConfigName {
			calls++
		}
		return read(path)
	})
	if err != nil {
		t.Fatal(err)
	}
	if path != "docs/testing/custom-plan.md" || run != "redis-stream-pool" {
		t.Fatalf("resolved %q, %q", path, run)
	}
	if calls != 1 {
		t.Fatalf("declaration read %d times, want once", calls)
	}
}

func TestTheCurrentDeclarationIsNamedTpp(t *testing.T) {
	if ConfigName != ".tpp.json" {
		t.Fatalf("ConfigName = %q, want .tpp.json", ConfigName)
	}
}

// A repository declared before the rename keeps working without anyone renaming its file.
func TestTheLegacyDeclarationIsReadWhenTheCurrentOneIsAbsent(t *testing.T) {
	got, err := DeclaredPath(t.TempDir(), declaresAs(LegacyConfigName, `{"planPath":"docs/testing/legacy-plan.md"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "docs/testing/legacy-plan.md" {
		t.Fatalf("path = %q, want the legacy declaration's", got)
	}
}

// Two declarations can disagree, and picking one silently would audit a plan the operator did not mean.
func TestBothDeclarationsPresentIsRefusedNamingBoth(t *testing.T) {
	_, err := DeclaredPath(t.TempDir(), func(string) (string, error) {
		return `{"planPath":"docs/testing/custom-plan.md"}`, nil
	})
	if err == nil || !strings.Contains(err.Error(), ".tpp.json") || !strings.Contains(err.Error(), ".rdd-plus.json") {
		t.Fatalf("error = %v, want a refusal naming both declarations", err)
	}
}

func TestAMalformedLegacyDeclarationIsNamedInTheError(t *testing.T) {
	_, err := DeclaredPath(t.TempDir(), declaresAs(LegacyConfigName, `{`))
	if err == nil || !strings.Contains(err.Error(), LegacyConfigName) {
		t.Fatalf("error = %v, want an error naming %s", err, LegacyConfigName)
	}
}
