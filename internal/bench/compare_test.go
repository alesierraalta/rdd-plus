package bench

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeAggregate(t *testing.T, dir string, agg Aggregate) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(filepath.Join(dir, "aggregate.json"), agg)
	return dir
}

func TestCompareRuns(t *testing.T) {
	before := writeAggregate(t, filepath.Join(t.TempDir(), "before"), Aggregate{
		TS: "t1", Defects: 5, Found: 2, Caught: 1,
		Cases: []Result{
			{Case: "a", Total: 2, Found: 1, Caught: 0, PlanFound: true},
			{Case: "b", Total: 2, Found: 1, Caught: 1, PlanFound: true},
			{Case: "c", Total: 1, Failed: true, FailReason: "exit status 1"},
		},
	})
	after := writeAggregate(t, filepath.Join(t.TempDir(), "after"), Aggregate{
		TS: "t2", Defects: 5, Found: 4, Caught: 4,
		Cases: []Result{
			{Case: "a", Total: 2, Found: 2, Caught: 2, ClaimedPinned: 2, PlanFound: true, Catch: CatchResult{Checked: true, Notes: []string{"3 test(s) pin the defective behaviour"}}},
			{Case: "b", Total: 2, Found: 2, Caught: 2, ClaimedPinned: 2, PlanFound: true},
			{Case: "c", Total: 1, Found: 0, Caught: 0, ClaimedPinned: 0, PlanFound: false},
		},
	})
	cmp, err := Compare(before, after)
	if err != nil {
		t.Fatal(err)
	}
	if cmp.Before.Found != 2 || cmp.After.Found != 4 || cmp.Before.Caught != 1 || cmp.After.Caught != 4 {
		t.Fatalf("totals: %+v", cmp)
	}
	if len(cmp.Rows) != 3 {
		t.Fatalf("rows = %d", len(cmp.Rows))
	}
	c := cmp.Rows[2]
	if !c.OnlyAfterValid || c.Before.Failed != true {
		t.Fatalf("a case failed before and valid after must be flagged: %+v", c)
	}
	md := cmp.Markdown()
	for _, want := range []string{"| a | 1/2 | 2/2 | 0/2 | 2/2 | 0/2 | 2/2 |", "| c | FAILED | 0/1 | FAILED | 0/1 | FAILED | 0/1 | before did not run to completion; NO PLAN after |", "reported 2 → 4", "caught 1 → 4", "3 test(s) pin the defect"} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}
}

func TestCompareRejectsRunsOverDifferentCases(t *testing.T) {
	before := writeAggregate(t, filepath.Join(t.TempDir(), "b"), Aggregate{Cases: []Result{{Case: "a", Total: 1}}})
	after := writeAggregate(t, filepath.Join(t.TempDir(), "a"), Aggregate{Cases: []Result{{Case: "z", Total: 1}}})
	if _, err := Compare(before, after); err == nil {
		t.Fatal("different case sets compared silently")
	}
}

// Two runs of different corpora are not comparable even when their case sets and counts match.
func TestCompareRejectsDifferentCorpora(t *testing.T) {
	before := writeAggregate(t, filepath.Join(t.TempDir(), "before"), Aggregate{
		Corpus: "sha256:aaaaaaaaaaaaaaaa",
		Cases:  []Result{{Case: "a", Total: 2, PlanFound: true}},
	})
	after := writeAggregate(t, filepath.Join(t.TempDir(), "after"), Aggregate{
		Corpus: "sha256:bbbbbbbbbbbbbbbb",
		Cases:  []Result{{Case: "a", Total: 2, PlanFound: true}},
	})
	_, err := Compare(before, after)
	if err == nil {
		t.Fatal("two different corpora compared silently")
	}
	for _, want := range []string{"sha256:aaaaaaaaaaaaaaaa", "sha256:bbbbbbbbbbbbbbbb"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q must name %s", err, want)
		}
	}
}

// A case's defect count is part of the corpus even when it failed: Total comes from the key,
// not from scoring, so a changed count is a changed corpus.
func TestCompareRejectsChangedDefectCount(t *testing.T) {
	cases := []struct {
		name          string
		before, after []Result
	}{
		{
			name:   "scored case",
			before: []Result{{Case: "a", Total: 2, PlanFound: true}},
			after:  []Result{{Case: "a", Total: 3, PlanFound: true}},
		},
		{
			name:   "failed case",
			before: []Result{{Case: "c", Total: 1, Failed: true, FailReason: "exit status 1"}},
			after:  []Result{{Case: "c", Total: 2, Failed: true, FailReason: "exit status 1"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := writeAggregate(t, filepath.Join(t.TempDir(), "before"), Aggregate{Cases: tc.before})
			after := writeAggregate(t, filepath.Join(t.TempDir(), "after"), Aggregate{Cases: tc.after})
			_, err := Compare(before, after)
			if err == nil {
				t.Fatal("a changed defect count compared silently")
			}
			got := err.Error()
			if !strings.Contains(got, tc.before[0].Case) {
				t.Fatalf("error %q must name case %s", got, tc.before[0].Case)
			}
			for _, n := range []int{tc.before[0].Total, tc.after[0].Total} {
				if !strings.Contains(got, fmt.Sprint(n)) {
					t.Fatalf("error %q must name count %d", got, n)
				}
			}
		})
	}
}

// Runs recorded before the corpus field existed still compare when their counts agree.
func TestCompareAllowsLegacyAggregatesWithEqualCounts(t *testing.T) {
	before := writeAggregate(t, filepath.Join(t.TempDir(), "before"), Aggregate{Cases: []Result{{Case: "a", Total: 2, Found: 1, PlanFound: true}}})
	after := writeAggregate(t, filepath.Join(t.TempDir(), "after"), Aggregate{Cases: []Result{{Case: "a", Total: 2, Found: 2, PlanFound: true}}})
	if _, err := Compare(before, after); err != nil {
		t.Fatalf("legacy aggregates with equal counts must compare: %v", err)
	}
}
