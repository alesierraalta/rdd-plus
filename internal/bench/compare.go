package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Comparison sets two runs of the same corpus side by side, per case and in total.
type Comparison struct {
	Before, After Aggregate
	Rows          []CompareRow
}

// CompareRow is one case in both runs.
type CompareRow struct {
	Case           string
	Before, After  Result
	OnlyAfterValid bool // before failed or was invalid, after is valid: not a like-for-like delta
}

// Compare reads aggregate.json from two result directories of the same corpus.
func Compare(beforeDir, afterDir string) (Comparison, error) {
	var cmp Comparison
	var err error
	if cmp.Before, err = readAggregate(beforeDir); err != nil {
		return cmp, err
	}
	if cmp.After, err = readAggregate(afterDir); err != nil {
		return cmp, err
	}
	byCase := func(a Aggregate) map[string]Result {
		m := map[string]Result{}
		for _, r := range a.Cases {
			m[r.Case] = r // the last run of a case stands for it
		}
		return m
	}
	b, a := byCase(cmp.Before), byCase(cmp.After)
	if len(b) != len(a) {
		return cmp, fmt.Errorf("different case sets: %d before, %d after", len(b), len(a))
	}
	for name := range b {
		if _, ok := a[name]; !ok {
			return cmp, fmt.Errorf("case %s is only in the before run", name)
		}
	}
	names := make([]string, 0, len(b))
	for name := range b {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		row := CompareRow{Case: name, Before: b[name], After: a[name]}
		row.OnlyAfterValid = (row.Before.Failed || row.Before.Invalid) && !(row.After.Failed || row.After.Invalid)
		cmp.Rows = append(cmp.Rows, row)
	}
	return cmp, nil
}

func readAggregate(dir string) (Aggregate, error) {
	var agg Aggregate
	data, err := os.ReadFile(filepath.Join(dir, "aggregate.json"))
	if err != nil {
		return agg, err
	}
	return agg, json.Unmarshal(data, &agg)
}

// Markdown renders the comparison as a table plus the totals line.
func (c Comparison) Markdown() string {
	var w strings.Builder
	fmt.Fprintf(&w, "# Compare %s → %s\n\n", c.Before.TS, c.After.TS)
	fmt.Fprintf(&w, "Defects %d: reported %d → %d · caught %d → %d · false positives %d → %d · cost $%.3f → $%.3f\n\n",
		c.After.Defects, c.Before.Found, c.After.Found, c.Before.Caught, c.After.Caught,
		c.Before.FalsePositives, c.After.FalsePositives, c.Before.CostUSD, c.After.CostUSD)
	w.WriteString("| case | reported before | reported after | caught before | caught after | note |\n|---|---|---|---|---|---|\n")
	for _, r := range c.Rows {
		var notes []string
		if r.OnlyAfterValid {
			notes = append(notes, "before did not run to completion")
		}
		if !r.After.Failed && !r.After.Invalid && !r.After.PlanFound {
			notes = append(notes, "NO PLAN after")
		}
		fmt.Fprintf(&w, "| %s | %s | %s | %s | %s | %s |\n", r.Case,
			cell(r.Before, r.Before.Found), cell(r.After, r.After.Found),
			cell(r.Before, r.Before.Caught), cell(r.After, r.After.Caught), strings.Join(notes, "; "))
	}
	return w.String()
}

func cell(r Result, n int) string {
	if r.Failed {
		return "FAILED"
	}
	if r.Invalid {
		return "INVALID"
	}
	return fmt.Sprintf("%d/%d", n, r.Total)
}
