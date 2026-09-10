package bench

import (
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
			{Case: "a", Total: 2, Found: 2, Caught: 2, PlanFound: true},
			{Case: "b", Total: 2, Found: 2, Caught: 2, PlanFound: true},
			{Case: "c", Total: 1, Found: 0, Caught: 0, PlanFound: false},
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
	for _, want := range []string{"| a | 1/2 | 2/2 | 0/2 | 2/2 |", "| c | FAILED | 0/1 | FAILED | 0/1 | before did not run to completion; NO PLAN after |", "reported 2 → 4", "caught 1 → 4"} {
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
