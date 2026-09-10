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
	raw, err := os.ReadFile(filepath.Join(ws, PlanPath))
	if err != nil {
		r := Score("", key)
		r.Notes = append(r.Notes, "no plan")
		return r
	}
	return Score(string(raw), key)
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
	r.LedgerRows = len(dataRows(sectionText(plan, "Evidence ledger")))

	matchedRow := make([]bool, len(rows))
	for _, d := range key.Defects {
		dr := DefectResult{ID: d.ID, File: d.File, Line: d.Line}
		for i, row := range rows {
			text := strings.Join(row, " | ")
			by := matches(text, d)
			if by == "" {
				continue
			}
			matchedRow[i] = true
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
func matches(rowText string, d Defect) string {
	cites := citations(rowText)
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
