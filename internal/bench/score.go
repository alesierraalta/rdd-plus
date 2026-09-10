package bench

import (
	"fmt"
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
	// ClaimedPinned reports that the finding names a test pinning it; whether that test
	// distinguishes anything is measured separately, by the catch check.
	ClaimedPinned bool   `json:"claimed_pinned,omitempty"`
	Row           string `json:"row,omitempty"` // the finding row that matched, for audit
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
	PlanFormat           string         `json:"plan_format"`       // FormatTable, FormatProse or FormatEmpty
	RowsWithoutPath      int            `json:"rows_without_path"` // finding rows that name no file, so nothing can be located
	ClaimedPinned        int            `json:"claimed_pinned"`    // defects whose finding names a pinning test
	Caught               int            `json:"caught"`            // defects some agent test distinguishes (fixture vs fix)
	Catch                CatchResult    `json:"catch"`
}

var (
	// A cell that means "no rows yet" rather than data.
	placeholderRe = regexp.MustCompile(`^(?i)(|-|—|n/?a|none|\(none\)|tbd)$`)
	// A path with an extension, optionally followed by :line or :first-last.
	citationRe = regexp.MustCompile(`([A-Za-z0-9_./\\-]+\.[A-Za-z][A-Za-z0-9]*)(?::(\d+)(?:-(\d+))?)?`)
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
	findings := sectionText(plan, "Findings")
	rows := dataRows(findings)
	r.FindingRows = len(rows)
	// The pinning column exists only in plans written under rule 13; find it by its header so
	// its position can move.
	pinCol := columnIndex(headerCells(findings), "pinning test")
	r.PlanFormat = planFormat(findings, rows)
	if r.PlanFormat == FormatProse {
		r.Notes = append(r.Notes, "findings are not in the template table; nothing in this plan can be located or re-scored")
	}
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

	// Attribution is per row, not per defect: one finding row is one claim. A row that names a
	// defect by keyword is credited to that defect (to several only if it names several); a row
	// that matches by line alone goes to the nearest defect, so two defects a few lines apart
	// cannot both be credited to a run that noticed one of them.
	linkedText := make([]string, len(rows))
	credited := make([]map[string]bool, len(rows))
	matchedRow := make([]bool, len(rows))
	byID := map[string]*DefectResult{}
	for i := range key.Defects {
		d := key.Defects[i]
		byID[d.ID] = &DefectResult{ID: d.ID, File: d.File, Line: d.Line}
	}
	for i, row := range rows {
		text := strings.Join(row, " | ")
		// The evidence rows a finding cites are part of its claim: the file is often named
		// there while the finding itself names the symbol.
		linked := linkedLedgerText(row, ledgerByID)
		linkedText[i] = strings.Join(linked, " | ")
		credited[i] = map[string]bool{}
		if len(citations(text)) == 0 && len(citations(linkedText[i])) == 0 {
			r.RowsWithoutPath++
		}
		var specific []Defect
		var nearest *Defect
		nearestDist := 0
		for j := range key.Defects {
			d := key.Defects[j]
			m := matchDefect(text, linkedText[i], d)
			if m.kind == "" {
				continue
			}
			matchedRow[i] = true
			if m.keyword {
				specific = append(specific, d)
			} else if nearest == nil || m.dist < nearestDist {
				nearest, nearestDist = &key.Defects[j], m.dist
			}
		}
		if len(specific) == 0 && nearest != nil {
			specific = append(specific, *nearest)
		}
		for _, d := range specific {
			credited[i][d.ID] = true
		}
	}
	for i, row := range rows {
		text := strings.Join(row, " | ")
		for id := range credited[i] {
			dr := byID[id]
			m := matchDefect(text, linkedText[i], defectByID(key, id))
			// A row that names the defect but cites no ledger row is prose, not a catch; it is
			// recorded as "unlinked" and does not count.
			if linkedText[i] == "" {
				if !dr.Found && dr.MatchedBy == "" {
					dr.MatchedBy, dr.Row = "unlinked:"+m.kind, text
				}
				continue
			}
			if !dr.Found {
				dr.Found, dr.MatchedBy, dr.Row = true, m.kind, text
				dr.ClaimedPinned = cellFilled(row, pinCol)
			}
		}
	}
	for i := range key.Defects {
		dr := *byID[key.Defects[i].ID]
		if dr.Found {
			r.Found++
			if dr.ClaimedPinned {
				r.ClaimedPinned++
			}
		}
		r.Defects = append(r.Defects, dr)
	}
	for _, m := range matchedRow {
		if !m {
			r.FalsePositives++
		}
	}
	if r.RowsWithoutPath > 0 {
		r.Notes = append(r.Notes, fmt.Sprintf("%d finding row(s) cite no file, in the plan or in the evidence they link", r.RowsWithoutPath))
	}
	if r.Total > 0 {
		r.Recall = float64(r.Found) / float64(r.Total)
	}
	if plan == "" {
		r.Recall = 0
	}
	return r
}

// defectMatch is how one finding row matches one defect.
type defectMatch struct {
	kind    string // "line", "keyword", or "" for no match
	dist    int    // lines between the cited location and the planted one
	keyword bool   // the row names the defect, not just its neighbourhood
}

// matchDefect reports how a finding row matches a defect. The evidence rows it cites may supply
// the file and line; only the row itself may supply a keyword, so a shared ledger row cannot
// hand one defect's name to another.
func matchDefect(rowText, evidenceText string, d Defect) defectMatch {
	cites := append(citations(rowText), citations(evidenceText)...)
	fileCited, lineHit, best := false, false, 0
	for _, c := range cites {
		if !samePath(c.path, d.File) {
			continue
		}
		fileCited = true
		if c.first <= 0 || d.Line < c.first-LineTolerance || d.Line > c.last+LineTolerance {
			continue
		}
		dist := 0
		if d.Line < c.first {
			dist = c.first - d.Line
		} else if d.Line > c.last {
			dist = d.Line - c.last
		}
		if !lineHit || dist < best {
			lineHit, best = true, dist
		}
	}
	if !fileCited {
		return defectMatch{}
	}
	lower := strings.ToLower(rowText)
	named := false
	for _, kw := range d.Keywords {
		if kw != "" && strings.Contains(lower, strings.ToLower(kw)) {
			named = true
			break
		}
	}
	switch {
	case lineHit:
		return defectMatch{kind: "line", dist: best, keyword: named}
	case named:
		return defectMatch{kind: "keyword", keyword: true}
	}
	return defectMatch{}
}

// defectByID returns the key's defect with that id.
func defectByID(key Key, id string) Defect {
	for _, d := range key.Defects {
		if d.ID == id {
			return d
		}
	}
	return Defect{}
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

// headerCells returns the header row of the first markdown table in a section.
func headerCells(section string) []string {
	for _, l := range strings.Split(section, "\n") {
		if t := strings.TrimSpace(l); strings.HasPrefix(t, "|") {
			return splitCells(t)
		}
	}
	return nil
}

// columnIndex finds the column whose header contains name, case-insensitively.
func columnIndex(header []string, name string) int {
	for i, h := range header {
		if strings.Contains(strings.ToLower(strings.TrimSpace(h)), strings.ToLower(name)) {
			return i
		}
	}
	return -1
}

// cellFilled reports whether a row carries real content in the given column.
func cellFilled(row []string, col int) bool {
	if col < 0 || col >= len(row) {
		return false
	}
	return !placeholderRe.MatchString(strings.TrimSpace(row[col]))
}

// How a plan states its findings. Prose scores zero exactly like an empty plan, so the run has
// to say which of the two happened.
const (
	FormatTable = "table"
	FormatProse = "prose"
	FormatEmpty = "empty"
)

// planFormat classifies the Findings section: template rows, prose, or nothing yet.
func planFormat(section string, rows [][]string) string {
	if len(rows) > 0 {
		return FormatTable
	}
	for _, l := range strings.Split(section, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "###") || strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
			return FormatProse
		}
	}
	return FormatEmpty
}
