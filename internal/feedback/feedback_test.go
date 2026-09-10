package feedback

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validText is the shape --template prints, filled in: one key per line, the run's identity in
// place, and the operator's own words.
func validText() string {
	return "# a comment line is ignored\n" +
		"\n" +
		"ts: 2026-09-10T12:00:00Z\n" +
		"repo: /repo\n" +
		"plan: docs/testing/test-plan.md\n" +
		"skill: 0.3.6\n" +
		"build: 0.3.6 (abc1234)\n" +
		"paid: the plan made me write the row first\n" +
		"cost: two hours\n" +
		"reason: it earned its keep\n" +
		"verdict: partly\n" +
		"guess: I cannot tell a probe from a pin\n" +
		"freeform: ship it\n"
}

func TestParseReadsTheTemplateShape(t *testing.T) {
	r, err := Parse(validText())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.TS != "2026-09-10T12:00:00Z" || r.Repo != "/repo" || r.Plan != "docs/testing/test-plan.md" {
		t.Fatalf("identity mangled: %+v", r)
	}
	if r.Skill != "0.3.6" || r.Build != "0.3.6 (abc1234)" {
		t.Fatalf("build identity mangled: %+v", r)
	}
	if r.Paid != "the plan made me write the row first" || r.Cost != "two hours" {
		t.Fatalf("fields mangled: %+v", r)
	}
	if r.Reason != "it earned its keep" || r.Verdict != VerdictPartly {
		t.Fatalf("verdict mangled: %+v", r)
	}
	if r.Guess != "I cannot tell a probe from a pin" || r.Freeform != "ship it" {
		t.Fatalf("optional fields mangled: %+v", r)
	}
}

// A value runs to the next key: the operator writes prose, not a single line.
func TestParseContinuesAValueOnTheFollowingLines(t *testing.T) {
	text := strings.Replace(validText(), "paid: the plan made me write the row first\n",
		"paid: the plan made me write the row first\nand then the test, twice\n", 1)
	r, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.Paid != "the plan made me write the row first\nand then the test, twice" {
		t.Fatalf("continuation mangled: %q", r.Paid)
	}
}

func TestParseRefusesUnknownKeysNamingThem(t *testing.T) {
	text := validText() + "surprise: something\n"
	_, err := Parse(text)
	if err == nil || !strings.Contains(err.Error(), "surprise") {
		t.Fatalf("want a refusal naming the unknown key, got %v", err)
	}
}

func TestParseRefusesMissingRequiredFieldsNamingThem(t *testing.T) {
	for _, field := range []string{"ts", "repo", "plan", "skill", "build", "paid", "cost", "reason", "verdict"} {
		t.Run(field, func(t *testing.T) {
			var kept []string
			for _, line := range strings.Split(validText(), "\n") {
				if strings.HasPrefix(line, field+":") {
					continue
				}
				kept = append(kept, line)
			}
			_, err := Parse(strings.Join(kept, "\n"))
			if err == nil || !strings.Contains(err.Error(), field) {
				t.Fatalf("want a refusal naming %q, got %v", field, err)
			}
		})
	}
}

func TestParseRefusesAVerdictOutsideTheThree(t *testing.T) {
	text := strings.Replace(validText(), "verdict: partly", "verdict: maybe", 1)
	_, err := Parse(text)
	if err == nil {
		t.Fatal("want a refusal for an unknown verdict")
	}
	for _, want := range []string{VerdictPaid, VerdictPartly, VerdictCeremony} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal must name all three verdicts; missing %q in %v", want, err)
		}
	}
}

func TestTemplateFillsTheIdentityAndNamesTheSubmitCommand(t *testing.T) {
	got := Template(Report{TS: "t", Repo: "/repo", Plan: "/plan", Skill: "0.3.6", Build: "0.3.6 (abc)"})
	for _, want := range []string{"ts: t", "repo: /repo", "plan: /plan", "skill: 0.3.6", "build: 0.3.6 (abc)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("template missing %q:\n%s", want, got)
		}
	}
	for _, want := range []string{"paid", "cost", "reason", "verdict", "guess", "freeform"} {
		if !strings.Contains(got, want+":") {
			t.Fatalf("template missing the %q field:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "--file") {
		t.Fatalf("the template must name the command that submits it:\n%s", got)
	}
}

// Record is append-only: the header is written once and a second report never rewrites the first.
func TestRecordAppendsJSONAndMarkdownWithoutRewriting(t *testing.T) {
	dir := t.TempDir()
	first := Report{TS: "2026-09-10T12:00:00Z", Repo: "/repo", Plan: "p", Skill: "0.3.6",
		Build: "b", Paid: "paid words", Cost: "one hour", Reason: "reason one",
		Verdict: VerdictPaid, Guess: "first guess"}
	second := Report{TS: "2026-09-11T12:00:00Z", Repo: "/repo", Plan: NotGiven, Skill: "0.3.6",
		Build: "b", Paid: "ceremony words", Cost: "ten minutes", Reason: "reason two",
		Verdict: VerdictCeremony}
	if err := Record(dir, first); err != nil {
		t.Fatalf("Record first: %v", err)
	}
	if err := Record(dir, second); err != nil {
		t.Fatalf("Record second: %v", err)
	}

	raw, err := os.ReadFile(LedgerPath(dir))
	if err != nil {
		t.Fatalf("ledger: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("ledger has %d lines, want 2:\n%s", len(lines), raw)
	}
	for i, want := range []Report{first, second} {
		var got Report
		if err := json.Unmarshal([]byte(lines[i]), &got); err != nil {
			t.Fatalf("line %d not JSON: %v", i, err)
		}
		if got != want {
			t.Fatalf("line %d = %+v, want %+v", i, got, want)
		}
	}

	md, err := os.ReadFile(MarkdownPath(dir))
	if err != nil {
		t.Fatalf("markdown: %v", err)
	}
	if strings.Count(string(md), "# Run feedback") != 1 {
		t.Fatalf("the header must be written once:\n%s", md)
	}
	if n := strings.Count(string(md), "\n## "); n != 2 {
		t.Fatalf("want one section per report, got %d:\n%s", n, md)
	}
	if !strings.Contains(string(md), "ceremony words") || !strings.Contains(string(md), "paid words") {
		t.Fatalf("the rendering lost the operator's words:\n%s", md)
	}
}

func TestReadMissingLedgerIsEmptyNotAnError(t *testing.T) {
	got, err := Read(t.TempDir())
	if err != nil {
		t.Fatalf("a missing ledger is not an error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want no reports, got %d", len(got))
	}
}

func TestSummaryCountsVerdictsAndSkillVersions(t *testing.T) {
	dir := t.TempDir()
	records := []Report{
		{TS: "2026-09-08T00:00:00Z", Repo: "/r", Plan: "p", Skill: "0.3.5", Build: "b",
			Paid: "a", Cost: "b", Reason: "oldest", Verdict: VerdictCeremony, Guess: "oldest guess"},
		{TS: "2026-09-09T00:00:00Z", Repo: "/r", Plan: "p", Skill: "0.3.6", Build: "b",
			Paid: "a", Cost: "b", Reason: "middle", Verdict: VerdictPaid, Guess: "middle guess"},
		{TS: "2026-09-10T00:00:00Z", Repo: "/r", Plan: "p", Skill: "0.3.6", Build: "b",
			Paid: "a", Cost: "b", Reason: "newest", Verdict: VerdictPartly, Guess: "newest guess"},
	}
	for _, r := range records {
		if err := Record(dir, r); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}
	got, err := Summary(dir)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	for _, want := range []string{
		"3 report",
		"paid: 1", "partly: 1", "ceremony: 1",
		"0.3.6: 2 (paid 1, partly 1, ceremony 0)",
		"0.3.5: 1 (paid 0, partly 0, ceremony 1)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary missing %q:\n%s", want, got)
		}
	}
}

func TestSummaryListsRecentGuessesNewestFirst(t *testing.T) {
	dir := t.TempDir()
	for i, guess := range []string{"g1", "g2", "g3", "g4", "g5", "g6"} {
		r := Report{TS: "t", Repo: "/r", Plan: "p", Skill: "0.3.6", Build: "b",
			Paid: "a", Cost: "b", Reason: "x", Verdict: VerdictPaid, Guess: guess}
		if err := Record(dir, r); err != nil {
			t.Fatalf("Record %d: %v", i, err)
		}
	}
	got, err := Summary(dir)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if strings.Contains(got, "g1") {
		t.Fatalf("only the five most recent guesses belong in the summary:\n%s", got)
	}
	newest := strings.Index(got, "g6")
	older := strings.Index(got, "g2")
	if newest == -1 || older == -1 || newest > older {
		t.Fatalf("guesses must be newest first:\n%s", got)
	}
}

func TestSummaryOnAMissingLedgerSaysSo(t *testing.T) {
	got, err := Summary(t.TempDir())
	if err != nil {
		t.Fatalf("a missing ledger is not an error: %v", err)
	}
	if !strings.Contains(strings.ToLower(got), "no reports") {
		t.Fatalf("want a plain no-reports line, got:\n%s", got)
	}
}

// The ledger and the rendering live under the same telemetry directory as the gate's own log.
func TestPathsFollowTheGatePrecedent(t *testing.T) {
	if filepath.ToSlash(LedgerPath("/cfg")) != "/cfg/telemetry/run-feedback.jsonl" {
		t.Fatalf("ledger path = %s", LedgerPath("/cfg"))
	}
	if filepath.ToSlash(MarkdownPath("/cfg")) != "/cfg/telemetry/run-feedback.md" {
		t.Fatalf("markdown path = %s", MarkdownPath("/cfg"))
	}
}

// A report with no plan named says so; it never guesses a path.
func TestPlanMissingSaysSo(t *testing.T) {
	text := strings.Replace(validText(), "plan: docs/testing/test-plan.md", "plan: ", 1)
	_, err := Parse(text)
	if err == nil || !strings.Contains(err.Error(), "plan") {
		t.Fatalf("an empty plan is a missing required field naming plan, got %v", err)
	}
}
