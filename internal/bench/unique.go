package bench

import (
	"sort"
	"strconv"
	"strings"
)

// UniqueCounts are the aggregate's numbers in the unit the report names: distinct defects, not
// defect-runs. Two runs of one case cannot make one defect look like two.
type UniqueCounts struct {
	Defects      int      // distinct (case, defect id) over valid runs
	Found        int      // distinct defects found in at least one valid run (reported)
	Confirmed    int      // distinct defects confirmed by an adjudication decision
	Caught       int      // distinct defects some test distinguished in at least one valid run
	DefectRuns   int      // keyed defects over valid runs: the sample size behind the numbers above
	Controls     int      // clean-control runs measured
	Inconclusive int      // valid runs with at least one keyed defect the catch check left unresolved
	Unstable     []string // cases whose outcome differs between their own valid runs, sorted
}

// CountUnique counts valid results by their case-and-defect identity. A run is valid when neither
// Invalid nor Failed is set. For instability, each run's outcome signature lists every keyed defect
// in key order as DefectResult.Found and Catch.Caught[id], followed by that run's Found total; a
// case with one valid run has no comparison and is never unstable.
type uniqueDefectState struct {
	found, confirmed, caught bool
}

func CountUnique(cases []Result) UniqueCounts {
	var counts UniqueCounts
	unique := map[string]uniqueDefectState{}
	signatures := map[string][]string{}
	for _, r := range cases {
		addUniqueRun(&counts, unique, signatures, r)
	}
	counts.Defects = len(unique)
	addUniqueStates(&counts, unique)
	counts.Unstable = unstableCaseNames(signatures)
	sort.Strings(counts.Unstable)
	return counts
}

func addUniqueRun(counts *UniqueCounts, unique map[string]uniqueDefectState, signatures map[string][]string, r Result) {
	if r.Invalid || r.Failed {
		return
	}
	if r.Control {
		counts.Controls++
		return
	}
	counts.DefectRuns += r.Total
	byID, order := defectResults(r)
	if hasMissingCatchVerdict(r, order) {
		counts.Inconclusive++
	}
	signatures[r.Case] = append(signatures[r.Case], outcomeSignature(r, byID, order))
	for _, id := range order {
		d := byID[id]
		key := r.Case + "\x00" + id
		state := unique[key]
		state.found = state.found || d.Found
		state.confirmed = state.confirmed || d.Confirmed
		state.caught = state.caught || r.Catch.Caught[id]
		unique[key] = state
	}
}

func addUniqueStates(counts *UniqueCounts, unique map[string]uniqueDefectState) {
	for _, state := range unique {
		if state.found {
			counts.Found++
		}
		if state.confirmed {
			counts.Confirmed++
		}
		if state.caught {
			counts.Caught++
		}
	}
}

func unstableCaseNames(signatures map[string][]string) []string {
	var unstable []string
	for caseName, caseSignatures := range signatures {
		if len(caseSignatures) < 2 || allSignaturesEqual(caseSignatures) {
			continue
		}
		unstable = append(unstable, caseName)
	}
	return unstable
}

func defectResults(r Result) (map[string]DefectResult, []string) {
	byID := make(map[string]DefectResult, len(r.Defects))
	order := make([]string, 0, len(r.Defects))
	for _, d := range r.Defects {
		if _, seen := byID[d.ID]; seen {
			continue
		}
		byID[d.ID] = d
		order = append(order, d.ID)
	}
	return byID, order
}

func hasMissingCatchVerdict(r Result, order []string) bool {
	for _, id := range order {
		if _, ok := r.Catch.Caught[id]; !ok {
			return true
		}
	}
	return false
}

func outcomeSignature(r Result, byID map[string]DefectResult, order []string) string {
	var b strings.Builder
	for _, id := range order {
		d := byID[id]
		b.WriteString(strconv.FormatBool(d.Found))
		b.WriteByte('/')
		b.WriteString(strconv.FormatBool(r.Catch.Caught[id]))
		b.WriteByte(';')
	}
	b.WriteString(strconv.Itoa(r.Found))
	return b.String()
}

func allSignaturesEqual(signatures []string) bool {
	first := signatures[0]
	for _, signature := range signatures[1:] {
		if signature != first {
			return false
		}
	}
	return true
}
