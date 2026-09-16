package skilltree

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// installed writes the files of one tree under a fresh directory and returns it.
func installed(t *testing.T, tree map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range tree {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestIdenticalReadsTheSameTreeAsTheSameAndNothingElse(t *testing.T) {
	skills := fstest.MapFS{
		"demo/SKILL.md": {Data: []byte("skill\n")},
		"demo/notes.md": {Data: []byte("notes\n")},
	}
	cases := []struct {
		name   string
		target map[string]string
		want   bool
	}{
		{"a copy that matches", map[string]string{"SKILL.md": "skill\n", "notes.md": "notes\n"}, true},
		{"one changed byte", map[string]string{"SKILL.md": "skill!\n", "notes.md": "notes\n"}, false},
		{"a file the user added beside them", map[string]string{"SKILL.md": "skill\n", "notes.md": "notes\n", "run.log": "noise\n"}, true},
		{"a file that is missing", map[string]string{"SKILL.md": "skill\n"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			same, err := Identical(skills, "demo", installed(t, tc.target))
			if err != nil {
				t.Fatalf("err = %v, want none", err)
			}
			if same != tc.want {
				t.Fatalf("same = %v, want %v", same, tc.want)
			}
		})
	}

	t.Run("a target that is not there", func(t *testing.T) {
		same, err := Identical(skills, "demo", filepath.Join(t.TempDir(), "absent"))
		if err != nil {
			t.Fatalf("err = %v, want none: an absent copy is a state, not a failure", err)
		}
		if same {
			t.Fatal("an absent copy is not identical")
		}
	})
}

// The tree nobody can walk is exactly where the two earlier copies disagreed: one returned this error and the
// other dropped it and answered "identical" for a skill nothing had read. It is an error here, and each caller
// decides what a tree it could not read means for it.
func TestIdenticalReportsATreeItCannotWalk(t *testing.T) {
	target := installed(t, map[string]string{"SKILL.md": "skill\n"})
	same, err := Identical(os.DirFS(t.TempDir()), "demo", target)
	if err == nil {
		t.Fatalf("same = %v with no error: a tree that is not there was read as an answer", same)
	}
	if same {
		t.Fatal("a tree nobody could read must not come back as identical")
	}
}
