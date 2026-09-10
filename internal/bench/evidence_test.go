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

// The scorer reads a plan written by the agent under test. An escaped pipe belongs to its cell;
// splitting on it shifts the evidence column, so a finding could be read as unlinked when it is
// linked, or the other way round.
func TestSplitCellsHonoursEscapedPipes(t *testing.T) {
	got := splitCells(`| F1 | a \| b | M | yes | E1 | open | me | - | - |`)
	if len(got) != 9 {
		t.Fatalf("cells = %q", got)
	}
	if got[1] != "a | b" {
		t.Fatalf("cell 1 = %q, want %q", got[1], "a | b")
	}
	if got[4] != "E1" {
		t.Fatalf("the evidence column shifted: %q", got)
	}
}
