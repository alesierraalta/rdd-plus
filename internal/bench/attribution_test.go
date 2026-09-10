package bench

import "testing"

// One finding row is one claim. Two defects a few lines apart must not both be credited to it,
// or a run that noticed one of them reads as having noticed both.
func TestARowIsCreditedToOneDefect(t *testing.T) {
	// n11's shape: both defects on the same line of the same file.
	sameLine := Key{ID: "n11", Defects: []Defect{
		{ID: "D1", File: "src/escape.js", Line: 5, Keywords: []string{"single quote", "attribute"}},
		{ID: "D2", File: "src/escape.js", Line: 5, Keywords: []string{"idempotence", "double-escape"}},
	}}
	ledger := "| E1 | c | cmd | i | o | m | r | observado |\n| E2 | c | cmd | i | o | m | r | observado |\n"
	cases := []struct {
		name      string
		key       Key
		findings  string
		wantFound int
		wantIDs   []string
	}{
		{
			name:      "one row naming one defect counts once",
			key:       sameLine,
			findings:  "| F1 | `src/escape.js:5` leaves the single quote raw inside an attribute | M | yes | E1 | open | me | - | - |\n",
			wantFound: 1, wantIDs: []string{"D1"},
		},
		{
			name: "two rows, one per defect, count twice",
			key:  sameLine,
			findings: "| F1 | `src/escape.js:5` leaves the single quote raw inside an attribute | M | yes | E1 | open | me | - | - |\n" +
				"| F2 | `src/escape.js:5` re-escapes an entity, breaking idempotence | M | yes | E2 | open | me | - | - |\n",
			wantFound: 2, wantIDs: []string{"D1", "D2"},
		},
		{
			name:      "one row that names both defects counts for both",
			key:       sameLine,
			findings:  "| F1 | `src/escape.js:5` the single quote is left raw in an attribute and an entity is double-escaped, breaking idempotence | M | yes | E1 | open | me | - | - |\n",
			wantFound: 2, wantIDs: []string{"D1", "D2"},
		},
		{
			// n04's shape: three lines apart, so the tolerance alone would cover both.
			name: "a row matching only by line is credited to the nearer defect",
			key: Key{ID: "n04", Defects: []Defect{
				{ID: "D1", File: "src/slug.js", Line: 4, Keywords: []string{"leading dash"}},
				{ID: "D2", File: "src/slug.js", Line: 7, Keywords: []string{"unicode"}},
			}},
			findings:  "| F1 | `src/slug.js:4` strips only one leading separator | M | yes | E1 | open | me | - | - |\n",
			wantFound: 1, wantIDs: []string{"D1"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Score(plan(tc.findings, ledger), tc.key)
			if r.Found != tc.wantFound {
				t.Fatalf("found = %d, want %d: %+v", r.Found, tc.wantFound, r.Defects)
			}
			got := map[string]bool{}
			for _, d := range r.Defects {
				if d.Found {
					got[d.ID] = true
				}
			}
			for _, id := range tc.wantIDs {
				if !got[id] {
					t.Fatalf("%s not credited: %+v", id, r.Defects)
				}
			}
		})
	}
}

// Crediting one row to one defect must not turn a second, unrelated row into a false positive.
func TestOneRowPerDefectDoesNotInventFalsePositives(t *testing.T) {
	key := Key{ID: "n11", Defects: []Defect{
		{ID: "D1", File: "src/escape.js", Line: 5, Keywords: []string{"single quote"}},
		{ID: "D2", File: "src/escape.js", Line: 5, Keywords: []string{"idempotence"}},
	}}
	ledger := "| E1 | c | cmd | i | o | m | r | observado |\n"
	r := Score(plan("| F1 | `src/escape.js:5` single quote left raw | M | yes | E1 | open | me | - | - |\n", ledger), key)
	if r.FalsePositives != 0 {
		t.Fatalf("false positives = %d, want 0: the row matched a defect", r.FalsePositives)
	}
}
