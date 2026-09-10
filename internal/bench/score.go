package bench

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// PlanPath is where the skill persists its plan inside a workspace.
const PlanPath = "docs/testing/test-plan.md"

// LineTolerance is how far a cited line may sit from the planted line and still count.
const LineTolerance = 5

// DefectResult says whether one planted defect was reported, and by which rule.
type DefectResult struct {
	ID        string `json:"id"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Found     bool   `json:"found"`
	MatchedBy string `json:"matched_by,omitempty"` // "line" or "keyword"
	Row       string `json:"row,omitempty"`        // the finding row that matched, for audit
}

// Result is the score of one workspace against its key.
type Result struct {
	Case                 string         `json:"case"`
	Run                  int            `json:"run"`
	Defects              []DefectResult `json:"defects"`
	Total                int            `json:"total"`
	Found                int            `json:"found"`
	Recall               float64        `json:"recall"`
	FalsePositives       int            `json:"false_positives"`
	FindingRows          int            `json:"finding_rows"`
	FindingsWithEvidence int            `json:"findings_with_evidence"`
	LedgerRows           int            `json:"ledger_rows"`
	Notes                []string       `json:"notes,omitempty"`
	CostUSD              float64        `json:"cost_usd"`
	Turns                int            `json:"turns"`
	Seconds              float64        `json:"seconds"`
	Invalid              bool           `json:"invalid"`
	InvalidReason        string         `json:"invalid_reason,omitempty"`
	Failed               bool           `json:"failed"`
	FailReason           string         `json:"fail_reason,omitempty"`
	Suite                SuiteResult    `json:"suite"`
	Workspace            string         `json:"workspace,omitempty"`
	PlanFound            bool           `json:"plan_found"`
	Caught               int            `json:"caught"` // defects some agent test distinguishes (fixture vs fix)
	Catch                CatchResult    `json:"catch"`
}

var (
	// A cell that means "no rows yet" rather than data.
	placeholderRe = regexp.MustCompile(`^(?i)(|-|—|n/?a|none|\(none\)|tbd)$`)
	// A path with an extension, optionally followed by :line or :first-last.
	citationRe = regexp.MustCompile(`([A-Za-z0-9_./\\-]+\.[A-Za-z0-9]+)(?::(\d+)(?:-(\d+))?)?`)
)

type citation struct {
	path  string
	first int
	last  int
}

// ScoreWorkspace reads the plan from a workspace and scores it; a missing plan scores zero.
func ScoreWorkspace(ws string, key Key) Result {
	return ScorePlanFile(filepath.Join(ws, PlanPath), key)
}

// ScorePlanFile scores one plan file, such as the copy a run keeps next to its result.json.
func ScorePlanFile(path string, key Key) Result {
	raw, err := os.ReadFile(path)
	if err != nil {
		r := Score("", key)
		r.Notes = append(r.Notes, "no plan")
		return r
	}
	r := Score(string(raw), key)
	r.PlanFound = true
	return r
}

// Score applies the scoring rule to plan text. A defect is found when a finding row cites its
// file and either a line within LineTolerance or one of its keywords; a row matching no defect
// is a false positive.
func Score(plan string, key Key) Result {
	r := Result{Case: key.ID, Total: len(key.Defects)}
	rows := dataRows(sectionText(plan, "Findings"))
	r.FindingRows = len(rows)
	for _, row := range rows {
		if len(row) > 4 && !placeholderRe.MatchString(strings.TrimSpace(row[4])) {
			r.FindingsWithEvidence++
		}
	}
	ledgerRows := dataRows(sectionText(plan, "Evidence ledger"))
	r.LedgerRows = len(ledgerRows)
	ledgerByID := map[string]string{}
	for _, lr := range ledgerRows {
		if len(lr) > 0 {
			ledgerByID[strings.Trim(strings.TrimSpace(lr[0]), "`")] = strings.Join(lr, " | ")
		}
	}

	matchedRow := make([]bool, len(rows))
	for _, d := range key.Defects {
		dr := DefectResult{ID: d.ID, File: d.File, Line: d.Line}
		for i, row := range rows {
			text := strings.Join(row, " | ")
			// The evidence rows a finding cites are part of its claim: the file is often named
			// there while the finding itself names the symbol.
			linked := linkedLedgerText(row, ledgerByID)
			by := matches(text, strings.Join(linked, " | "), d)
			if by == "" {
				continue
			}
			matchedRow[i] = true
			// A row that names the defect but cites no ledger row is prose, not a catch; it is
			// recorded as "unlinked" and does not count.
			if len(linked) == 0 {
				if !dr.Found && dr.MatchedBy == "" {
					dr.MatchedBy, dr.Row = "unlinked:"+by, text
				}
				continue
			}
			if !dr.Found {
				dr.Found, dr.MatchedBy, dr.Row = true, by, text
			}
		}
		if dr.Found {
			r.Found++
		}
		r.Defects = append(r.Defects, dr)
	}
	for _, m := range matchedRow {
		if !m {
			r.FalsePositives++
		}
	}
	if r.Total > 0 {
		r.Recall = float64(r.Found) / float64(r.Total)
	}
	if plan == "" {
		r.Recall = 0
	}
	return r
}

// matches reports how a finding row matches a defect: "line", "keyword", or "" for no match.
// evidenceText may cite the file and line; only rowText may supply a keyword.
func matches(rowText, evidenceText string, d Defect) string {
	cites := append(citations(rowText), citations(evidenceText)...)
	fileCited := false
	for _, c := range cites {
		if !samePath(c.path, d.File) {
			continue
		}
		fileCited = true
		if c.first > 0 && d.Line >= c.first-LineTolerance && d.Line <= c.last+LineTolerance {
			return "line"
		}
	}
	if !fileCited {
		return ""
	}
	lower := strings.ToLower(rowText)
	for _, kw := range d.Keywords {
		if kw != "" && strings.Contains(lower, strings.ToLower(kw)) {
			return "keyword"
		}
	}
	return ""
}

// samePath matches by suffix: "render.js" or "fixture/src/render.js" both name "src/render.js".
func samePath(cited, key string) bool {
	c := strings.TrimPrefix(filepath.ToSlash(cited), "./")
	k := strings.TrimPrefix(filepath.ToSlash(key), "./")
	if c == k {
		return true
	}
	return strings.HasSuffix(c, "/"+k) || strings.HasSuffix(k, "/"+c)
}

func citations(text string) []citation {
	var out []citation
	for _, m := range citationRe.FindAllStringSubmatch(text, -1) {
		c := citation{path: m[1]}
		if m[2] != "" {
			c.first, _ = strconv.Atoi(m[2])
			c.last = c.first
			if m[3] != "" {
				c.last, _ = strconv.Atoi(m[3])
			}
		}
		out = append(out, c)
	}
	return out
}

// sectionText returns the body under "## <name>" up to the next level-2 heading.
func sectionText(doc, name string) string {
	lines := strings.Split(doc, "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "## ") && strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "## ")), name) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// dataRows parses the first markdown table of a section: cells of every row after the header
// and separator, skipping placeholder rows whose id cell is empty or a "no rows" marker.
func dataRows(section string) [][]string {
	var rows [][]string
	inTable, headerSeen := false, false
	for _, l := range strings.Split(section, "\n") {
		t := strings.TrimSpace(l)
		if !strings.HasPrefix(t, "|") {
			if inTable {
				break
			}
			continue
		}
		inTable = true
		cells := splitCells(t)
		if !headerSeen {
			headerSeen = true
			continue
		}
		if isSeparator(cells) {
			continue
		}
		if len(cells) == 0 || placeholderRe.MatchString(strings.TrimSpace(cells[0])) {
			continue
		}
		rows = append(rows, cells)
	}
	return rows
}

func splitCells(row string) []string {
	row = strings.TrimSpace(row)
	row = strings.TrimPrefix(row, "|")
	row = strings.TrimSuffix(row, "|")
	parts := strings.Split(row, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isSeparator(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		if strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}

// citedEvidenceIDs reads the ids in a finding row's evidence cell ("E1, E2", "E1/E3").
func citedEvidenceIDs(row []string) []string {
	if len(row) <= 4 {
		return nil
	}
	cell := strings.TrimSpace(row[4])
	if cell == "" || placeholderRe.MatchString(cell) {
		return nil
	}
	var ids []string
	for _, part := range strings.FieldsFunc(cell, func(c rune) bool { return c == ',' || c == ';' || c == '/' || c == ' ' }) {
		ids = append(ids, strings.Trim(part, "`"))
	}
	return ids
}

// linkedLedgerText returns the text of the ledger rows a finding row cites and that exist.
func linkedLedgerText(row []string, ledgerByID map[string]string) []string {
	var out []string
	for _, id := range citedEvidenceIDs(row) {
		if text, ok := ledgerByID[id]; ok {
			out = append(out, text)
		}
	}
	return out
}
