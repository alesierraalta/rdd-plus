package bench

import "testing"

const pinnedHeader = "| Id | Finding | Severity | Data safe? | Evidence id | Pinning test (suite path :: test name) | Status | Verdict by / date | Reason | Fingerprint |\n|---|---|---|---|---|---|---|---|---|---|\n"

// planWithHeader builds a plan whose Findings table uses the given header.
func planWithHeader(header, findings, ledger string) string {
	return "# Test plan\n\n## Findings\n\n" + header + findings +
		"\nStatuses: open · fixed.\n\n## Evidence ledger\n\n" + ledgerHeader + ledger +
		"\n### Hypotheses (razonado)\n\n| Id | H |\n|---|---|\n"
}

// A finding may claim a pinning test; whether that test distinguishes anything is measured
// separately. The gap between the two is the theater.
func TestClaimedPinning(t *testing.T) {
	key := Key{ID: "c", Defects: []Defect{{ID: "D1", File: "src/a.js", Line: 5, Keywords: []string{"x"}}}}
	ledger := "| E1 | c | cmd | i | o | m | r | observado |\n"
	cases := []struct {
		name        string
		header      string
		findings    string
		wantClaimed int
	}{
		{
			name:        "a named test in the pinning column is a claim",
			header:      pinnedHeader,
			findings:    "| F1 | src/a.js:5 x | M | yes | E1 | tests/a.test.js :: rejects the boundary | fixed | me | - | - |\n",
			wantClaimed: 1,
		},
		{
			name:        "an empty pinning cell claims nothing",
			header:      pinnedHeader,
			findings:    "| F1 | src/a.js:5 x | M | yes | E1 |  | open | me | - | - |\n",
			wantClaimed: 0,
		},
		{
			name:        "a placeholder in the pinning cell claims nothing",
			header:      pinnedHeader,
			findings:    "| F1 | src/a.js:5 x | M | yes | E1 | n/a | open | me | - | - |\n",
			wantClaimed: 0,
		},
		{
			name:        "a table without the column claims nothing",
			header:      "| Id | Finding | Severity | Data safe? | Evidence id | Status | Verdict by / date | Reason | Fingerprint |\n|---|---|---|---|---|---|---|---|---|\n",
			findings:    "| F1 | src/a.js:5 x | M | yes | E1 | fixed | me | - | - |\n",
			wantClaimed: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Score(planWithHeader(tc.header, tc.findings, ledger), key)
			if r.Found != 1 {
				t.Fatalf("the finding must still be found: %+v", r.Defects)
			}
			if r.ClaimedPinned != tc.wantClaimed {
				t.Fatalf("claimed = %d, want %d", r.ClaimedPinned, tc.wantClaimed)
			}
			if r.Defects[0].ClaimedPinned != (tc.wantClaimed == 1) {
				t.Fatalf("defect claim = %v", r.Defects[0].ClaimedPinned)
			}
		})
	}
}

// The pinning column is found by its header, wherever it sits.
func TestColumnIndexByHeader(t *testing.T) {
	header := []string{"Id", "Finding", "Evidence id", " Pinning test (suite path :: test name) ", "Status"}
	if got := columnIndex(header, "pinning test"); got != 3 {
		t.Fatalf("index = %d", got)
	}
	if got := columnIndex(header, "nothing here"); got != -1 {
		t.Fatalf("missing column = %d, want -1", got)
	}
}
