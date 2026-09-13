// Package plan writes and checks docs/testing/test-plan.md against the contract the
// test-strategy skill persists. Format compliance is deterministic work: a binary does it once
// instead of every session re-deriving the structure from prose.
package plan

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alesierraalta/rdd-plus/internal/assets"
)

// TemplatePath is where the skeleton lives inside the embedded skills.
const TemplatePath = "test-strategy/assets/test-plan-template.md"

// DefaultPath is where the skill persists the plan.
const DefaultPath = "docs/testing/test-plan.md"

// Template returns the shipped plan skeleton.
func Template() (string, error) {
	data, err := fs.ReadFile(assets.Skills(), TemplatePath)
	if err != nil {
		return "", fmt.Errorf("read embedded template: %w", err)
	}
	return string(data), nil
}

// Init writes the skeleton to path. An existing plan is never overwritten unless force is set:
// the plan never shrinks, so replacing one is a decision, not a default.
func Init(path string, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("%s already exists; pass --force to replace it", path)
	}
	body, err := Template()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

var (
	pathCiteRe  = regexp.MustCompile(`[A-Za-z0-9_./\\-]+\.[A-Za-z][A-Za-z0-9]*:\d+`)
	placeholder = regexp.MustCompile(`^(?i)(|-|—|n/?a|none|\(none\)|tbd)$`)
	// A finding whose verdict asserts the defect is real owes a test that holds it.
	settledStatus = regexp.MustCompile(`(?i)\b(confirmed|fixed)\b`)
	// A scoped run declares itself in one plan-header line, `Light: <blast radius> · touches
	// <classes>`. The declaration is the cheap half of the decision: a binary can read its shape and
	// whether the plan corroborates the target it names, never whether the change was really bounded.
	// The whitespace class stays inside the line: a declaration is one header line, and a match that can
	// start on the previous line would name the wrong line as its home.
	lightLineRe  = regexp.MustCompile(`^[ \t]*Light:[ \t]*(.*)$`)
	lightShapeRe = regexp.MustCompile(`^(.+?)[ \t]*·[ \t]*touches[ \t]*(.*)$`)
)

// findingsStatuses is the Findings status vocabulary the plan documents. It is the membership the
// checker enforces; FindingsStatusList spells the same list for every message that has to offer it.
var findingsStatuses = map[string]bool{
	"open": true, "confirmed": true, "fixed": true, "rejected": true, "wontfix": true,
}

// FindingsStatusList is the closed Findings vocabulary in one string, so the list a reader is shown is
// the list the checker enforces: a status the vocabulary does not carry (`resolved`) is answered rather
// than silently read as `open`.
const FindingsStatusList = "open, confirmed, fixed, rejected, wontfix"

// Check reads a plan and returns everything that breaks the contract, most structural first.
// Every breach that names a row or a cell carries its file line, so the reader opens the plan at the row
// instead of grepping for the text the message quotes.
// An empty result means the plan is well formed, not that the testing was good.
func Check(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return CheckDocument(string(raw)), nil
}

// CheckDocument is Check on the document itself, so the rules can be read against a document that has
// never been written to disk. The rules live in one place because a second copy would drift.
func CheckDocument(doc string) []string {
	lines := strings.Split(doc, "\n")

	heading, end := sectionRegion(lines, "Findings")
	if heading < 0 {
		// Nothing to point at: a section that does not exist never gets an invented location.
		return []string{"no Findings section"}
	}
	findings := scanTable(lines, heading+1, end)
	if findings.header == nil {
		prose := strings.Join(lines[heading+1:end], "\n")
		if strings.Contains(prose, "###") || strings.Contains(prose, "- ") {
			return []string{fmt.Sprintf("line %d: the Findings section is not a table: prose cannot be located, honoured, or re-scored", heading+1)}
		}
		return []string{fmt.Sprintf("line %d: the Findings section has no table", heading+1)}
	}
	// The ledger is the one other table the check reads back. A plan without one simply has no rows to
	// corroborate against, exactly as before: sectionRegion reports it missing and the scan returns empty.
	ledgerHeading, ledgerEnd := sectionRegion(lines, "Evidence ledger")
	ledger := scanTable(lines, ledgerHeading+1, ledgerEnd)

	var problems []string
	ledgerIDs := map[string]bool{}
	for _, r := range ledger.rows {
		id := cell(r.cells, 0)
		ledgerIDs[id] = true
		if label := cell(r.cells, len(r.cells)-1); strings.EqualFold(label, "razonado") {
			problems = append(problems, fmt.Sprintf("line %d: evidence %s is labelled razonado: a hypothesis belongs under Hypotheses, never in the ledger", r.line, quote(id)))
		}
	}

	iFind := columnIndex(findings.header, "finding")
	iEvidence := columnIndex(findings.header, "evidence")
	iPin := columnIndex(findings.header, "pinning test")
	iStatus := columnIndex(findings.header, "status")
	for _, r := range findings.rows {
		id := quote(cell(r.cells, 0))
		if iFind >= 0 && !pathCiteRe.MatchString(cell(r.cells, iFind)) {
			problems = append(problems, fmt.Sprintf("line %d: finding %s cites no path:line, so nothing can be located", r.line, id))
		}
		ev := cell(r.cells, iEvidence)
		switch {
		case iEvidence < 0 || placeholder.MatchString(ev):
			problems = append(problems, fmt.Sprintf("line %d: finding %s cites no evidence row", r.line, id))
		default:
			for _, part := range strings.FieldsFunc(ev, func(r rune) bool { return r == ',' || r == ';' || r == '/' || r == ' ' }) {
				if part = strings.Trim(part, "`"); part != "" && !ledgerIDs[part] {
					problems = append(problems, fmt.Sprintf("line %d: finding %s cites evidence %s, which is not a row in the Evidence ledger", r.line, id, quote(part)))
				}
			}
		}
		status, breach := findingStatus(cell(r.cells, iStatus))
		if breach != "" {
			problems = append(problems, fmt.Sprintf("line %d: finding %s %s", r.line, id, breach))
		}
		if settledStatus.MatchString(status) && (iPin < 0 || placeholder.MatchString(cell(r.cells, iPin))) {
			problems = append(problems, fmt.Sprintf("line %d: finding %s is settled but names no pinning test", r.line, id))
		}
	}
	if lightProblems, declared := lightReport(lines); declared {
		problems = append(problems, lightProblems...)
	}
	return problems
}

// findingStatus reads a Findings status cell against the closed vocabulary the plan documents. It is the
// breach the check used to hide: the old `confirmed|fixed` regex read `resolved` as unsettled, so the row
// silently stopped owing the test that holds its verdict and the plan still passed. The returned status
// is trimmed and lowercased for the settled rule; the breach, when there is one, is ready to follow the
// finding id.
func findingStatus(raw string) (string, string) {
	status := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case status == "":
		return "", "has no status: one of " + FindingsStatusList
	case !findingsStatuses[status]:
		return status, fmt.Sprintf("status \"%s\" is not one of: %s", quote(raw), FindingsStatusList)
	}
	return status, ""
}

// LightActivated reports whether a plan declares a scoped run that passes every Light-specific rule.
// The bench reads activation from here, so an ordinary plan, a detected defect, or a line that merely
// looks like a declaration is never a scoped run.
func LightActivated(doc string) bool {
	problems, declared := lightReport(strings.Split(doc, "\n"))
	return declared && len(problems) == 0
}

// lightDecl is a parsed `Light:` declaration: the target it names, the line it sits on, and the breaches
// its own shape owes. Whether the change was really bounded stays with the operator and the plan's
// reader, never with this check.
type lightDecl struct {
	target   string
	line     int
	problems []string
	found    bool
}

// lightReport returns every breach a declared scoped run owes, and whether the plan declares one at
// all. A plan that never claims to be scoped owes none of these rules.
func lightReport(lines []string) ([]string, bool) {
	d := lightDeclaration(lines)
	if !d.found {
		return nil, false
	}
	problems := d.problems
	if d.target != "" && !targetIsInEvidence(lines, d.target) {
		problems = append(problems, fmt.Sprintf("line %d: the Light declaration names target %s, which no Ranked-target row and no path:line citation corroborates", d.line, quote(d.target)))
	}
	layersHeading, layersEnd := sectionRegion(lines, "Layer matrix")
	return append(problems, unscopedLayers(scanTable(lines, layersHeading+1, layersEnd))...), true
}

// lightDeclaration reads the `Light:` header: the declared shape, a non-empty blast radius and
// non-empty touched classes. The classes are the declaration's own words; a binary can read that they
// are there, never that they are true. It reads the declaration line by line so a breach can name the
// line the declaration sits on.
func lightDeclaration(lines []string) lightDecl {
	for i, l := range lines {
		m := lightLineRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		line := i + 1
		shape := lightShapeRe.FindStringSubmatch(strings.TrimSpace(m[1]))
		if shape == nil {
			return lightDecl{line: line, found: true, problems: []string{
				fmt.Sprintf("line %d: the Light declaration must read `Light: <blast radius> · touches <classes>`", line)}}
		}
		target, classes := strings.TrimSpace(shape[1]), strings.TrimSpace(shape[2])
		var problems []string
		if target == "" || placeholder.MatchString(target) {
			problems = append(problems, fmt.Sprintf("line %d: the Light declaration names no blast radius", line))
			// Nothing to corroborate: the breach is reported once, not again as a target the plan
			// cannot vouch for.
			target = ""
		}
		if classes == "" || placeholder.MatchString(classes) {
			problems = append(problems, fmt.Sprintf("line %d: the Light declaration names no touched classes", line))
		}
		return lightDecl{target: target, line: line, problems: problems, found: true}
	}
	return lightDecl{}
}

// targetIsInEvidence corroborates a declared blast radius against what the plan already carries: an
// exact `Target` cell in the ranked targets, or a path the plan cites as path:line. Reading the claim
// back against the plan is all a binary can do; whether the target is bounded stays with the reader.
func targetIsInEvidence(lines []string, target string) bool {
	rankedHeading, rankedEnd := sectionRegion(lines, "Ranked targets")
	ranked := scanTable(lines, rankedHeading+1, rankedEnd)
	iTarget := columnIndex(ranked.header, "target")
	for _, r := range ranked.rows {
		if iTarget >= 0 && cell(r.cells, iTarget) == target {
			return true
		}
	}
	for _, cite := range pathCiteRe.FindAllString(strings.Join(lines, "\n"), -1) {
		if path, _, ok := strings.Cut(cite, ":"); ok && path == target {
			return true
		}
	}
	return false
}

// unscopedLayers enforces the one rule a `Light:` plan owes beyond the ordinary contract: a layer
// the run left out states why in its `Scope` cell, in a sentence rather than a keyword. Without it
// `Light:` is a label anyone can type, and the layer sweep's blind spot comes back unrecorded.
func unscopedLayers(layers tableScan) []string {
	iStatus := columnIndex(layers.header, "status")
	iScope := columnIndex(layers.header, "scope")
	var problems []string
	for _, r := range layers.rows {
		if !naStatus.MatchString(cell(r.cells, iStatus)) {
			continue
		}
		if cell(r.cells, iScope) == "" {
			problems = append(problems, fmt.Sprintf("line %d: layer %s is out of scope in a Light plan and its Scope cell is empty, so nothing records why it was left out", r.line, quote(cell(r.cells, 0))))
		}
	}
	return problems
}

// row is one markdown table data row: its cells and the line it sits on in the plan file. Carrying them
// together is the whole point — `path: problem` left the reader grepping for the row the message was
// about.
type row struct {
	line  int
	cells []string
}

// tableScan is what a region scan found. header is nil when the region carries no table at all. A blank
// line is a separator that lost its pipes, not the end of the block: skipping it is what stops a blank
// inserted inside a table from dropping every row under it while the plan still reports `well formed`.
type tableScan struct {
	header []string
	rows   []row
}

// scanTable reads the first markdown table of lines[start:end] (0-based, end exclusive), carrying the line
// each row sits on. Before the header any line is the section's own description; once the header is read,
// a non-blank line that is not a row ends the table — the shipped template puts prose under every table —
// and the rows below it are not read. A separator or placeholder row is skipped, as before.
func scanTable(lines []string, start, end int) tableScan {
	var s tableScan
	for i := start; i < end && i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			continue
		}
		if !strings.HasPrefix(t, "|") {
			if s.header != nil {
				break
			}
			continue
		}
		cells := split(t)
		if s.header == nil {
			s.header = cells
			continue
		}
		if isSeparator(cells) || placeholder.MatchString(cell(cells, 0)) {
			continue
		}
		s.rows = append(s.rows, row{line: i + 1, cells: cells})
	}
	return s
}

// sectionRegion locates a `## <name>` section: its 0-based heading line and the exclusive end of its body,
// or (-1, -1) when the section is missing. A table may sit under a `###`, so the region spans to the next
// level-2 heading and the scan decides which part of it is a table.
func sectionRegion(lines []string, name string) (heading, end int) {
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if !strings.HasPrefix(t, "## ") || !strings.EqualFold(strings.TrimSpace(t[3:]), name) {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			if strings.HasPrefix(strings.TrimSpace(lines[j]), "## ") {
				return i, j
			}
		}
		return i, len(lines)
	}
	return -1, -1
}

// section and table are the readers gaps.go still uses. The Light rules read their tables through
// sectionRegion and scanTable instead, so a Light breach can name the row it blames; when the gaps sweep
// moves onto the same scanner these two go with it.

// section returns the body under "## <name>" up to the next level-2 heading.
func section(doc, name string) string {
	lines := strings.Split(doc, "\n")
	start := -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "## ") && strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(t, "## ")), name) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			return strings.Join(lines[start:i], "\n")
		}
	}
	return strings.Join(lines[start:], "\n")
}

// table returns the data rows and the header of the first markdown table in a section.
func table(sec string) (rows [][]string, header []string) {
	for _, l := range strings.Split(sec, "\n") {
		t := strings.TrimSpace(l)
		if !strings.HasPrefix(t, "|") {
			if header != nil {
				break
			}
			continue
		}
		cells := split(t)
		if header == nil {
			header = cells
			continue
		}
		if isSeparator(cells) || len(cells) == 0 || placeholder.MatchString(cell(cells, 0)) {
			continue
		}
		rows = append(rows, cells)
	}
	return rows, header
}

// split cuts a markdown row into cells. A backslash-escaped pipe belongs to the cell it sits in;
// splitting on it shifts every column to its right, so a crafted cell could move a layer's owner
// out of the column the report reads.
func split(line string) []string {
	line = strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(line), "|"), "|")
	var parts []string
	var cur strings.Builder
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			if r != '|' {
				cur.WriteRune('\\')
			}
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '|':
			parts = append(parts, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if escaped {
		cur.WriteRune('\\')
	}
	return append(parts, strings.TrimSpace(cur.String()))
}

func isSeparator(cells []string) bool {
	for _, c := range cells {
		if c != "" && strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}

func cell(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func columnIndex(header []string, name string) int {
	for i, h := range header {
		if strings.Contains(strings.ToLower(h), strings.ToLower(name)) {
			return i
		}
	}
	return -1
}
