package bench

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Rescore applies the current scoring and catch rules to a finished run: each case is re-read
// from the plan kept beside its result.json and, when the workspace was kept, its tests are
// re-checked. Cost, turns, and failure state come from the original run. The result is written
// under <results>/rescored so the original aggregate is never rewritten.
// lookup maps a case name to its case directory (fixture/, fix/, KEY.json).
func Rescore(results string, lookup func(caseName string) (string, error), suiteTimeout time.Duration) (Aggregate, error) {
	orig, err := readAggregate(results)
	if err != nil {
		return orig, err
	}
	agg := Aggregate{TS: time.Now().UTC().Format(time.RFC3339), Out: filepath.Join(results, "rescored"), Model: orig.Model, RescoredFrom: results}
	for _, old := range orig.Cases {
		res := old
		agg.CostUSD += old.CostUSD
		if old.Invalid || old.Failed {
			agg.Cases = append(agg.Cases, res)
			if old.Invalid {
				agg.Invalid++
			} else {
				agg.Failed++
			}
			continue
		}
		caseDir, err := lookup(old.Case)
		if err != nil {
			return agg, fmt.Errorf("%s: %w", old.Case, err)
		}
		key, err := LoadKey(caseDir)
		if err != nil {
			return agg, fmt.Errorf("%s: %w", old.Case, err)
		}
		dir := filepath.Join(results, old.Case, fmt.Sprint(max(old.Run, 1)))
		ws := filepath.Join(dir, "ws")
		if st, err := os.Stat(ws); err == nil && st.IsDir() {
			res = merge(old, ScoreWorkspace(ws, key))
			res.Catch = Discriminate(caseDir, ws, key, suiteTimeout)
		} else {
			res = merge(old, ScorePlanFile(filepath.Join(dir, "test-plan.md"), key))
			res.Catch = old.Catch // no workspace to re-check: the run's own verdict stands
		}
		res.Caught = res.Catch.Count()
		agg.Cases = append(agg.Cases, res)
		agg.Defects += res.Total
		agg.Found += res.Found
		agg.Caught += res.Caught
		agg.FalsePositives += res.FalsePositives
		if !res.PlanFound {
			agg.NoPlan++
		}
	}
	if agg.Defects > 0 {
		agg.Recall = float64(agg.Found) / float64(agg.Defects)
		agg.RecallCaught = float64(agg.Caught) / float64(agg.Defects)
	}
	if err := os.MkdirAll(agg.Out, 0o755); err != nil {
		return agg, err
	}
	writeJSON(filepath.Join(agg.Out, "aggregate.json"), agg)
	_ = os.WriteFile(filepath.Join(agg.Out, "summary.md"), []byte(Summary(agg)), 0o644)
	return agg, nil
}
