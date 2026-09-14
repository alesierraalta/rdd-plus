// Package plan writes and checks docs/testing/test-plan.md against the contract the
// test-strategy skill persists. Format compliance is deterministic work: a binary does it once
// instead of every session re-deriving the structure from prose.
package plan

import (
	"errors"
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
	lightLineRe  = regexp.MustCompile(`(?m)^\s*Light:[ \t]*(.*)$`)
	lightShapeRe = regexp.MustCompile(`^(.+?)[ \t]*·[ \t]*touches[ \t]*(.*)$`)
)

// Check reads a plan and returns everything that breaks the contract, most structural first.
// An empty result means the plan is well formed, not that the testing was good.
func Check(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc := string(raw)
	problems := tableProblems(doc)

	findings := section(doc, "Findings")
	if findings == "" {
		return append(problems, "no Findings section"), nil
	}
	rows, header := table(findings)
	if header == nil {
		if strings.Contains(findings, "###") || strings.Contains(findings, "- ") {
			return append(problems, "the Findings section is not a table: prose cannot be located, honoured, or re-scored"), nil
		}
		return append(problems, "the Findings section has no table"), nil
	}

	ledgerRows, _ := table(section(doc, "Evidence ledger"))
	ledgerIDs := map[string]bool{}
	for _, lr := range ledgerRows {
		if len(lr) == 0 {
			continue
		}
		id := cell(lr, 0)
		ledgerIDs[id] = true
		if label := cell(lr, len(lr)-1); strings.EqualFold(label, "razonado") {
			problems = append(problems, fmt.Sprintf("evidence %s is labelled razonado: a hypothesis belongs under Hypotheses, never in the ledger", id))
		}
	}

	iFind := columnIndex(header, "finding")
	iEvidence := columnIndex(header, "evidence")
	iPin := columnIndex(header, "pinning test")
	iStatus := columnIndex(header, "status")
	for _, row := range rows {
		id := cell(row, 0)
		if iFind >= 0 && !pathCiteRe.MatchString(cell(row, iFind)) {
			problems = append(problems, fmt.Sprintf("finding %s cites no path:line, so nothing can be located", id))
		}
		ev := cell(row, iEvidence)
		switch {
		case iEvidence < 0 || placeholder.MatchString(ev):
			problems = append(problems, fmt.Sprintf("finding %s cites no evidence row", id))
		default:
			for _, part := range strings.FieldsFunc(ev, func(r rune) bool { return r == ',' || r == ';' || r == '/' || r == ' ' }) {
				if part = strings.Trim(part, "`"); part != "" && !ledgerIDs[part] {
					problems = append(problems, fmt.Sprintf("finding %s cites evidence %s, which is not a row in the Evidence ledger", id, part))
				}
			}
		}
		if settledStatus.MatchString(cell(row, iStatus)) && (iPin < 0 || placeholder.MatchString(cell(row, iPin))) {
			problems = append(problems, fmt.Sprintf("finding %s is settled but names no pinning test", id))
		}
	}
	if lightProblems, declared := lightReport(doc); declared {
		problems = append(problems, lightProblems...)
	}
	return problems, nil
}

// LedgerRow is one row of the Evidence ledger, with the cells a machine reads resolved by column name
// rather than by position. `Admit` is the single command the row declares and `Digest` the output that
// command was observed to produce; both are empty on a plan written before those columns existed.
//
// Cells and HeaderCells are the number of cells the row's line and its table's header hold as the
// table splitter reads them. The two differ exactly when an unescaped `|` cut a cell, which shifts
// every column to its right: a reader can then refuse the row instead of reading the wrong cell as the
// command. Both are zero for a row a caller assembled by hand rather than read from a document.
type LedgerRow struct {
	ID           string
	Claim        string
	Executed     string
	Admit        string
	Inputs       string
	Observed     string
	Digest       string
	Normalize    string
	Mode         string
	Mutation     string
	Reproduction string
	Label        string
	Cells        int
	HeaderCells  int
}

// Ledger reads the Evidence ledger exactly where Check reads it and resolves every machine column by
// name. Order is the document's; ID stays the first cell and Label the last, the same reading Check
// does, so a row with extra or truncated cells is reported rather than silently dropped. Rows the table
// already skips, a separator or a placeholder row, stay skipped.
func Ledger(doc string) []LedgerRow {
	rows, header := table(section(doc, "Evidence ledger"))
	if header == nil {
		return nil
	}
	// Column names are matched by substring, so each name below is the whole word the header cell
	// carries and shares it with no other column: `admit` never resolves to `Executed`, `digest` never
	// resolves to `Observed` or to the mutation column, and `normalize` resolves to nothing else.
	index := map[string]int{}
	for _, name := range []string{"claim", "executed", "admit", "inputs", "observed", "digest", "normalize", "mode", "mutation", "reproduction"} {
		index[name] = columnIndex(header, name)
	}
	ledger := make([]LedgerRow, 0, len(rows))
	for _, row := range rows {
		r := LedgerRow{ID: cell(row, 0), Label: cell(row, len(row)-1), Cells: len(row), HeaderCells: len(header)}
		for _, f := range []struct {
			name  string
			field *string
		}{
			{"claim", &r.Claim},
			{"executed", &r.Executed},
			{"admit", &r.Admit},
			{"inputs", &r.Inputs},
			{"observed", &r.Observed},
			{"digest", &r.Digest},
			{"normalize", &r.Normalize},
			{"mode", &r.Mode},
			{"mutation", &r.Mutation},
			{"reproduction", &r.Reproduction},
		} {
			*f.field = cell(row, index[f.name])
		}
		ledger = append(ledger, r)
	}
	return ledger
}

// digestRe is the only shape a recorded digest may take: it is exactly what Digest returns, so a
// recorded cell is always comparable to a fresh observation and a typo can never be pinned.
var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// modeRe is the only shape a recorded Mode may take. The mode is part of what a pin means because the same
// command digests differently in a container than on the host, so a row that pins a digest without pinning
// the mode it was taken in would fail its next check with a digest mismatch that says nothing about why.
var modeRe = regexp.MustCompile(`^(host|sandbox)$`)

// ErrNoColumn reports that the ledger's header does not name the column a writer was asked to fill. A caller
// branches on it instead of reading the message when it knows the column is optional: an empty Mode cell
// already means the host, so a host recording into a plan written before that column existed is still true.
var ErrNoColumn = errors.New("the Evidence ledger header names no such column")

// RecordDigest returns doc with the named ledger row's Digest cell replaced by digest, every other byte of
// the document unchanged. It is the one writer in this package: Check, Ledger and Gaps only read, so this is
// where a wrong splice could corrupt a plan.
func RecordDigest(doc, id, digest string) (string, error) {
	if !digestRe.MatchString(digest) {
		return "", fmt.Errorf("digest %q is not a sha256 digest: a Digest cell carries sha256:<64 lowercase hex>, the form a fresh observation takes", digest)
	}
	return recordCell(doc, id, "digest", "Digest", digest)
}

// RecordMode returns doc with the named ledger row's Mode cell replaced by mode, every other byte of the
// document unchanged. Recording a pin records both cells: the digest says what was observed and the mode
// says where, and a row that carries one without the other cannot be checked honestly in either mode.
func RecordMode(doc, id, mode string) (string, error) {
	if !modeRe.MatchString(mode) {
		return "", fmt.Errorf("mode %q is not an execution mode: a Mode cell carries host or sandbox, the two places a row's command can be observed", mode)
	}
	return recordCell(doc, id, "mode", "Mode", mode)
}

// recordCell returns doc with one named column of one ledger row replaced by value, every other byte of the
// document unchanged.
//
// The ledger is located exactly where Check and Ledger locate it, and the column is resolved by the same
// substring match Ledger resolves it with, so the writer and the readers can never disagree about which
// column is which. The row is found by its first cell and the cell is spliced by byte offset: a row may
// carry a backslash-escaped pipe in any cell, so the cell is cut where split cuts it rather than by
// re-rendering the row, which would rewrite escapes and spacing nobody asked to touch. Writing the value a
// cell already holds is a no-op, so a rerun rewrites nothing.
func recordCell(doc, id, column, label, value string) (string, error) {
	start, end, ok := sectionBounds(doc, "Evidence ledger")
	if !ok {
		return "", fmt.Errorf("the document has no Evidence ledger section, so there is no row %s to record", id)
	}
	body := doc[start:end]
	_, header := table(body)
	if header == nil {
		return "", fmt.Errorf("the Evidence ledger section holds no table, so there is no row %s to record", id)
	}
	iColumn := columnIndex(header, column)
	if iColumn < 0 {
		return "", fmt.Errorf("the Evidence ledger header names no %s column, so row %s has nowhere to record %s: %w", label, id, value, ErrNoColumn)
	}

	// The section's own lines are walked the way table walks them, so the row a reader sees is the row
	// this writes, and the offset in doc is carried alongside so the splice never re-joins cells.
	seenHeader := false
	lineStart := start
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "|") {
			if seenHeader {
				break
			}
			lineStart += len(line) + 1
			continue
		}
		cells := split(t)
		if !seenHeader {
			seenHeader = true
			lineStart += len(line) + 1
			continue
		}
		if isSeparator(cells) || len(cells) == 0 || placeholder.MatchString(cell(cells, 0)) {
			lineStart += len(line) + 1
			continue
		}
		if cell(cells, 0) != id {
			lineStart += len(line) + 1
			continue
		}
		cs, ce, ok := cellSpan(line, iColumn)
		if !ok {
			// cellSpan cuts the cells split cuts, so the column has no cell exactly when the row is shorter
			// than it: one refusal, not two spellings of the same fact.
			return "", fmt.Errorf("evidence %s has %d cells, so the %s column (%d) has no cell in that row; restore it before recording", id, len(cells), label, iColumn)
		}
		// Replace the cell's own bytes and nothing else: a cell that holds a value keeps the spacing its
		// author wrote around it, so a rerun with the same value is byte-identical. A cell that holds no
		// value has no spacing around a value to keep, so the value is framed by one space and the row still
		// reads as a table row instead of colliding with the next pipe.
		raw := line[cs:ce]
		content := strings.TrimSpace(raw)
		if content == "" {
			if raw == "" {
				return doc[:lineStart+cs] + value + doc[lineStart+ce:], nil
			}
			return doc[:lineStart+cs] + " " + value + " " + doc[lineStart+ce:], nil
		}
		first := lineStart + cs + strings.Index(raw, content)
		return doc[:first] + value + doc[first+len(content):], nil
	}
	return "", fmt.Errorf("evidence %s is not a row in the Evidence ledger, so the plan has nothing to record against", id)
}

// cellSpan returns the byte span one cell occupies in a markdown table row, cut exactly where split
// cuts it, so RecordDigest can splice a single cell instead of re-rendering the row. A backslash-
// escaped pipe is part of its cell, never a delimiter.
func cellSpan(line string, i int) (start, end int, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return 0, 0, false
	}
	base := strings.Index(line, trimmed)
	inner := trimmed
	if strings.HasSuffix(inner, "|") {
		inner = inner[:len(inner)-1]
	}
	if strings.HasPrefix(inner, "|") {
		inner = inner[1:]
		base++
	}
	cellStart, n := 0, 0
	escaped := false
	for j := 0; j < len(inner); j++ {
		switch c := inner[j]; {
		case escaped:
			escaped = false
		case c == '\\':
			escaped = true
		case c == '|':
			if n == i {
				return base + cellStart, base + j, true
			}
			n++
			cellStart = j + 1
		}
	}
	if n == i {
		return base + cellStart, base + len(inner), true
	}
	return 0, 0, false
}

// LightActivated reports whether a plan declares a scoped run that passes every Light-specific rule.
// The bench reads activation from here, so an ordinary plan, a detected defect, or a line that merely
// looks like a declaration is never a scoped run.
func LightActivated(doc string) bool {
	problems, declared := lightReport(doc)
	return declared && len(problems) == 0
}

// lightReport returns every breach a declared scoped run owes, and whether the plan declares one at
// all. A plan that never claims to be scoped owes none of these rules.
func lightReport(doc string) ([]string, bool) {
	target, problems, declared := lightDeclaration(doc)
	if !declared {
		return nil, false
	}
	if target != "" && !targetIsInEvidence(doc, target) {
		problems = append(problems, fmt.Sprintf("the Light declaration names target %s, which no Ranked-target row and no path:line citation corroborates", target))
	}
	return append(problems, unscopedLayers(doc)...), true
}

// lightDeclaration reads the `Light:` header: the declared shape, a non-empty blast radius and
// non-empty touched classes. The classes are the declaration's own words; a binary can read that they
// are there, never that they are true.
func lightDeclaration(doc string) (target string, problems []string, declared bool) {
	m := lightLineRe.FindStringSubmatch(doc)
	if m == nil {
		return "", nil, false
	}
	shape := lightShapeRe.FindStringSubmatch(strings.TrimSpace(m[1]))
	if shape == nil {
		return "", []string{"the Light declaration must read `Light: <blast radius> · touches <classes>`"}, true
	}
	target, classes := strings.TrimSpace(shape[1]), strings.TrimSpace(shape[2])
	if target == "" || placeholder.MatchString(target) {
		problems = append(problems, "the Light declaration names no blast radius")
		// Nothing to corroborate: the breach is reported once, not again as a target the plan
		// cannot vouch for.
		target = ""
	}
	if classes == "" || placeholder.MatchString(classes) {
		problems = append(problems, "the Light declaration names no touched classes")
	}
	return target, problems, true
}

// targetIsInEvidence corroborates a declared blast radius against what the plan already carries: an
// exact `Target` cell in the ranked targets, or a path the plan cites as path:line. Reading the claim
// back against the plan is all a binary can do; whether the target is bounded stays with the reader.
func targetIsInEvidence(doc, target string) bool {
	rows, header := table(section(doc, "Ranked targets"))
	iTarget := columnIndex(header, "target")
	for _, row := range rows {
		if iTarget >= 0 && cell(row, iTarget) == target {
			return true
		}
	}
	for _, cite := range pathCiteRe.FindAllString(doc, -1) {
		if path, _, ok := strings.Cut(cite, ":"); ok && path == target {
			return true
		}
	}
	return false
}

// unscopedLayers enforces the one rule a `Light:` plan owes beyond the ordinary contract: a layer
// the run left out states why in its `Scope` cell, in a sentence rather than a keyword. Without it
// `Light:` is a label anyone can type, and the layer sweep's blind spot comes back unrecorded.
func unscopedLayers(doc string) []string {
	rows, header := table(section(doc, "Layer matrix"))
	iStatus := columnIndex(header, "status")
	iScope := columnIndex(header, "scope")
	var problems []string
	for _, row := range rows {
		if !naStatus.MatchString(cell(row, iStatus)) {
			continue
		}
		if cell(row, iScope) == "" {
			problems = append(problems, fmt.Sprintf("layer %s is out of scope in a Light plan and its Scope cell is empty, so nothing records why it was left out", cell(row, 0)))
		}
	}
	return problems
}

// section returns the body under "## <name>" up to the next level-2 heading.
func section(doc, name string) string {
	start, end, ok := sectionBounds(doc, name)
	if !ok {
		return ""
	}
	return doc[start:end]
}

// sectionBounds returns the byte range the body under "## <name>" occupies in doc, up to the next
// level-2 heading. The heading is matched exactly as section matched it, so the one locator serves both
// the readers and the writer: RecordDigest needs the offset, and Check and Ledger need only the text.
func sectionBounds(doc, name string) (start, end int, ok bool) {
	lines := strings.Split(doc, "\n")
	heading := -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "## ") && strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(t, "## ")), name) {
			heading = i
			break
		}
	}
	if heading < 0 {
		return 0, 0, false
	}
	start = len(strings.Join(lines[:heading+1], "\n")) + 1
	if start > len(doc) {
		start = len(doc)
	}
	end = len(doc)
	for i := heading + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = len(strings.Join(lines[:i], "\n"))
			break
		}
	}
	if end < start {
		end = start
	}
	return start, end, true
}

// tableProblems reports every data row in the document whose cell count disagrees with its own table's
// header. A cell holding an unescaped `|` splits into several, so the row declares one thing and carries
// another, and every column to the right of the cut is read from the wrong cell. Each malformed row earns
// one breach, not one per extra cell. The rows table already skips, a separator or a placeholder, are
// skipped here too, so nothing is said about a line that is not a conclusion.
func tableProblems(doc string) []string {
	var problems []string
	name := ""
	var header []string
	for _, line := range strings.Split(doc, "\n") {
		t := strings.TrimSpace(line)
		if h, ok := headingName(t); ok {
			name, header = h, nil
			continue
		}
		if !strings.HasPrefix(t, "|") {
			header = nil
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
		if len(cells) != len(header) {
			problems = append(problems, cellCountBreach(name, cell(cells, 0), len(cells), len(header)))
		}
	}
	return problems
}

// headingName returns the text of a markdown heading line and whether the line is a heading at all. A `#`
// that is not followed by a space is a tag, not a heading, so the two never read as each other.
func headingName(t string) (string, bool) {
	level := 0
	for level < len(t) && t[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level == len(t) || t[level] != ' ' {
		return "", false
	}
	return strings.TrimSpace(t[level:]), true
}

// cellCountBreach is the one breach a malformed row earns: it names the table, the row's first cell so the
// reader can find it, and both counts, then says plainly what the mismatch means. A row with more cells
// was cut by an unescaped `|`; a row with fewer is simply missing one.
func cellCountBreach(table, first string, cells, header int) string {
	where := "an unnamed table"
	if table != "" {
		where = "the " + table + " table"
	}
	if cells > header {
		return fmt.Sprintf("%s row %q has %d cells against the header's %d: an unescaped `|` splits a cell, so the row carries more than it declares", where, first, cells, header)
	}
	return fmt.Sprintf("%s row %q has %d cells against the header's %d: a cell is missing, so the row carries less than it declares", where, first, cells, header)
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
