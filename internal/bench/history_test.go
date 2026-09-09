package bench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendHistoryIsAdditive(t *testing.T) {
	dir := t.TempDir()
	for i, r := range []float64{0.5, 0.8} {
		if err := AppendHistory(dir, HistoryEntry{TS: "t" + string(rune('0'+i)), Out: "o", Model: "m", Cases: 1, Defects: 4, Found: 2, Recall: r, SkillVersion: "3.0"}); err != nil {
			t.Fatal(err)
		}
	}
	jsonl, _ := os.ReadFile(filepath.Join(dir, "history.jsonl"))
	if n := strings.Count(strings.TrimSpace(string(jsonl)), "\n") + 1; n != 2 {
		t.Fatalf("jsonl lines = %d, want 2: %s", n, jsonl)
	}
	md, _ := os.ReadFile(filepath.Join(dir, "history.md"))
	if strings.Count(string(md), "# Benchmark history") != 1 {
		t.Fatalf("header must be written once: %s", md)
	}
	if strings.Count(string(md), "| t0 |") != 1 || strings.Count(string(md), "| t1 |") != 1 {
		t.Fatalf("both rows expected once: %s", md)
	}
	if !strings.Contains(string(md), "| 0.80 |") {
		t.Fatalf("recall formatting: %s", md)
	}
}

func TestSkillVersion(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(f, []byte("---\nname: test-strategy\nmetadata:\n  author: x\n  version: \"3.0\"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SkillVersion(f); got != "3.0" {
		t.Fatalf("version = %q", got)
	}
	if got := SkillVersion(filepath.Join(dir, "missing.md")); got != "unknown" {
		t.Fatalf("missing file version = %q", got)
	}
	if err := os.WriteFile(f, []byte("---\nname: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SkillVersion(f); got != "unknown" {
		t.Fatalf("no version field = %q", got)
	}
}
