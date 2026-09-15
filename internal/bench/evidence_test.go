package bench

import "testing"

func TestFoundRequiresLinkedEvidence(t *testing.T) {
	key := Key{ID: "k", Defects: []Defect{{ID: "D1", File: "src/x.js", Line: 10, Keywords: []string{"quote"}}}}
	findings := "## Findings\n\n| Id | Finding | Sev | Safe | Evidence id | Status | By | Reason | FP |\n|---|---|---|---|---|---|---|---|---|\n| F1 | src/x.js:11 quote lost | M | yes | E1 | open | t | r | f |\n\n"
	cases := []struct {
		name   string
		ledger string
		found  bool
	}{
		{"linked evidence counts", "## Evidence ledger\n\n| Id | Claim |\n|---|---|\n| E1 | c |\n", true},
		{"empty ledger: citation is dangling", "## Evidence ledger\n\n| Id | Claim |\n|---|---|\n", false},
		{"ledger without that id: dangling", "## Evidence ledger\n\n| Id | Claim |\n|---|---|\n| E9 | c |\n", false},
		{"no ledger section at all", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Score(findings+c.ledger, key)
			if r.Defects[0].Found != c.found {
				t.Fatalf("found=%v want %v (matched_by=%q)", r.Defects[0].Found, c.found, r.Defects[0].MatchedBy)
			}
		})
	}
}
