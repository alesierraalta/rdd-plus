package bench

import "testing"

func TestCountUniqueRepeatedDefectRuns(t *testing.T) {
	got := CountUnique([]Result{
		{Case: "case-a", Run: 1, Total: 1, Found: 1, Defects: []DefectResult{{ID: "D1", Found: true, Confirmed: true}}, Catch: CatchResult{Caught: map[string]bool{"D1": true}}},
		{Case: "case-a", Run: 2, Total: 1, Found: 1, Defects: []DefectResult{{ID: "D1", Found: true, Confirmed: true}}, Catch: CatchResult{Caught: map[string]bool{"D1": true}}},
	})
	if got.DefectRuns != 2 || got.Defects != 1 || got.Found != 1 || got.Confirmed != 1 || got.Caught != 1 {
		t.Fatalf("unique counts = %+v, want two runs of one found, confirmed and caught defect", got)
	}
	if len(got.Unstable) != 0 {
		t.Fatalf("identical runs are unstable: %v", got.Unstable)
	}
}

func TestCountUniqueMarksOnlyPartiallyFoundCaseUnstable(t *testing.T) {
	got := CountUnique([]Result{
		{Case: "case-a", Run: 1, Total: 1, Found: 0, Defects: []DefectResult{{ID: "D1"}}, Catch: CatchResult{Caught: map[string]bool{"D1": false}}},
		{Case: "case-a", Run: 2, Total: 1, Found: 1, Defects: []DefectResult{{ID: "D1", Found: true}}, Catch: CatchResult{Caught: map[string]bool{"D1": true}}},
	})
	if got.Found != 1 || got.Caught != 1 {
		t.Fatalf("unique counts = %+v, want the defect counted once when only one run found and caught it", got)
	}
	if len(got.Unstable) != 1 || got.Unstable[0] != "case-a" {
		t.Fatalf("unstable cases = %v, want [case-a]", got.Unstable)
	}
}

func TestCountUniqueNeverMarksSingleRunUnstable(t *testing.T) {
	got := CountUnique([]Result{{Case: "case-a", Run: 1, Total: 1, Found: 1, Defects: []DefectResult{{ID: "D1", Found: true}}}})
	if len(got.Unstable) != 0 {
		t.Fatalf("a single run is unstable: %v", got.Unstable)
	}
}

func TestCountUniqueControlsAndInvalidRuns(t *testing.T) {
	got := CountUnique([]Result{
		{Case: "control", Run: 1, Control: true, Total: 0},
		{Case: "invalid", Run: 1, Invalid: true, Total: 1, Defects: []DefectResult{{ID: "D1", Found: true}}},
		{Case: "failed", Run: 1, Failed: true, Total: 1, Defects: []DefectResult{{ID: "D2", Found: true}}},
	})
	if got.Controls != 1 {
		t.Fatalf("controls = %d, want 1", got.Controls)
	}
	if got.Defects != 0 || got.DefectRuns != 0 || got.Found != 0 || got.Confirmed != 0 || got.Caught != 0 {
		t.Fatalf("control and incomplete runs entered defect counts: %+v", got)
	}
}

func TestCountUniqueDistinguishesMissingCatchVerdictFromFalse(t *testing.T) {
	got := CountUnique([]Result{
		{Case: "missing", Run: 1, Total: 1, Defects: []DefectResult{{ID: "D1"}}, Catch: CatchResult{Caught: map[string]bool{}}},
		{Case: "false", Run: 1, Total: 1, Defects: []DefectResult{{ID: "D1"}}, Catch: CatchResult{Caught: map[string]bool{"D1": false}}},
	})
	if got.Inconclusive != 1 {
		t.Fatalf("inconclusive = %d, want only the run with no D1 verdict", got.Inconclusive)
	}
}
