package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const upgradePlan = "## Ranked targets\n\n" +
	"| Target | Status |\n" +
	"|---|---|\n" +
	"| target one | pending |\n" +
	"\n## Layer matrix\n\n" +
	"| Layer | Skill | Scope | Status |\n" +
	"|---|---|---|---|\n" +
	"| Security | `appsec` | input | pending |\n" +
	"| Runtime | `runtime` | faults | done |\n"

func TestUpgradeAddsTheRunColumnToBothTables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")
	if err := os.WriteFile(path, []byte(upgradePlan), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := Upgrade(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if changed != 2 {
		t.Fatalf("changed tables = %d, want 2", changed)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"| Target | Status | Run |",
		"|---|---|---|",
		"| target one | pending |  |",
		"| Layer | Skill | Scope | Status | Run |",
		"| Security | `appsec` | input | pending |  |",
	} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("upgraded plan missing %q:\n%s", want, got)
		}
	}
}

func TestUpgradeIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")
	if err := os.WriteFile(path, []byte(upgradePlan), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Upgrade(path, ""); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := Upgrade(path, "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if changed != 0 || string(second) != string(first) {
		t.Fatalf("second upgrade changed=%d or rewrote bytes:\nfirst %q\nsecond %q", changed, first, second)
	}
}

const recordUpgradePlan = "## Findings\n\n" +
	"| Id | Finding | Run |\n" +
	"|---|---|---|\n" +
	"| F1 | `src/a.go:1` |  |\n" +
	"\n## Execution log\n\n" +
	"| Date | Notes | Run |\n" +
	"|---|---|---|\n" +
	"| today | executed |  |\n" +
	"\n## Evidence ledger\n\n" +
	"| Id | Label | Run |\n" +
	"|---|---|---|\n" +
	"| E1 | observado |  |\n"

func TestUpgradeAddsTheRunColumnToTheRecordTables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")
	legacy := strings.ReplaceAll(recordUpgradePlan, " | Run |", " | Scope |")
	if strings.Contains(legacy, "| Run |") {
		t.Fatal("the legacy fixture still carries the Run column, so the upgrade proves nothing")
	}
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := Upgrade(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if changed != 3 {
		t.Fatalf("changed tables = %d, want 3", changed)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"| Id | Finding | Scope | Run |",
		"| Date | Notes | Scope | Run |",
		"| Id | Label | Scope | Run |",
		"| F1 | `src/a.go:1` |  |",
		"| today | executed |  |",
		"| E1 | observado |  |",
	} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("upgraded plan missing %q:\n%s", want, got)
		}
	}
}

func TestUpgradeLeavesAlreadyMigratedRecordTablesAlone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")
	legacy := strings.ReplaceAll(recordUpgradePlan, " | Run |", " | Scope |")
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, err := Upgrade(path, "alpha"); err != nil || changed != 3 {
		t.Fatalf("first upgrade changed=%d err=%v, want three tables", changed, err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := Upgrade(path, "beta")
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if changed != 0 || string(second) != string(first) {
		t.Fatalf("second upgrade changed=%d or rewrote bytes:\nfirst %q\nsecond %q", changed, first, second)
	}
}

func TestUpgradeStampsTheGivenSlug(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")
	if err := os.WriteFile(path, []byte(upgradePlan), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Upgrade(path, "redis-stream-pool"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(got), "| redis-stream-pool |") != 3 {
		t.Fatalf("slug was not stamped on every data row:\n%s", got)
	}
}

func TestUpgradeLeavesEveryOtherByteAlone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")
	before := "prefix  \n" + upgradePlan + "suffix\n"
	if err := os.WriteFile(path, []byte(before), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := Upgrade(path, ""); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "prefix  \n## Ranked targets\n\n" +
		"| Target | Status | Run |\n" +
		"|---|---|---|\n" +
		"| target one | pending |  |\n" +
		"\n## Layer matrix\n\n" +
		"| Layer | Skill | Scope | Status | Run |\n" +
		"|---|---|---|---|---|\n" +
		"| Security | `appsec` | input | pending |  |\n" +
		"| Runtime | `runtime` | faults | done |  |\n" +
		"suffix\n"
	if string(got) != want {
		t.Fatalf("upgrade changed bytes outside appended cells:\ngot  %q\nwant %q", got, want)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("upgrade must preserve mode: %v %v", info.Mode(), err)
	}
}

func TestUpgradeRefusesAnInterruptedTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")
	before := "## Layer matrix\n\n" +
		"| Layer | Skill | Scope | Status |\n" +
		"|---|---|---|---|\n" +
		"| Security | `appsec` | input | pending |\n" +
		"### Notes\n" +
		"| hidden row | hidden | hidden | pending |\n" +
		"prose cuts the table\n" +
		"|---|---|---|---|\n" +
		"\n## Ranked targets\n\n" +
		"| Target | Status |\n|---|---|\n| target | pending |\n"
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Upgrade(path, ""); err == nil || !strings.Contains(err.Error(), "writing into a cut table is how rows get lost") {
		t.Fatalf("upgrade must refuse an interrupted table with the AddFinding message, got %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != before {
		t.Fatalf("a refused upgrade changed the plan:\ngot  %q\nwant %q", got, before)
	}
}
