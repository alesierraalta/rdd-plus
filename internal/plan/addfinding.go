package plan

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrUsage marks a refusal of the invocation's own values (exit 2) rather than a refusal by the plan.
var ErrUsage = errors.New("usage error")

// usageError carries the sentinel without printing it, so errors.Is still finds ErrUsage in the chain.
type usageError struct{ err error }

func (u usageError) Error() string { return u.err.Error() }
func (u usageError) Unwrap() error { return ErrUsage }

func usagef(format string, args ...any) error { return usageError{err: fmt.Errorf(format, args...)} }

// Finding is one Findings row: this command never writes the ledger row its evidence cites.
type Finding struct {
	ID          string
	Location    string
	Severity    string
	DataSafe    string
	Evidence    string
	Test        string
	Status      string
	VerdictBy   string
	Reason      string
	Fingerprint string
}

// AddFinding writes one Findings row into the plan at path and returns the 1-based line it landed on. Every
// refusal leaves the file byte-identical: the candidate is held to CheckDocument — the rules `plan check`
// runs — before a single byte moves.
func AddFinding(path string, f Finding) (int, error) { return addFinding(path, f) }

// addFinding is the read, validate and insert transaction; it returns before the write on every refusal.
func addFinding(path string, f Finding) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(string(raw), "\n")

	heading, end := sectionRegion(lines, "Findings")
	if heading < 0 {
		return 0, fmt.Errorf("the plan has no ## Findings section, so there is nowhere to record the row")
	}
	table := scanTable(lines, heading+1, end)
	if table.header == nil {
		return 0, fmt.Errorf("the Findings section has no table, so the row has no columns to land in")
	}
	// A row in a cut table sits in the file and never in the plan; interruptedTable already names the cut.
	if _, message := interruptedTable("Findings", table); message != "" {
		return 0, fmt.Errorf("%s; writing into a cut table is how rows get lost", message)
	}

	if err := f.checkValues(); err != nil {
		return 0, err
	}
	status, err := f.checkStatus()
	if err != nil {
		return 0, err
	}
	if err := f.checkTest(status); err != nil {
		return 0, err
	}
	// An absent fingerprint has a spelling: the placeholder the template uses for a digest not computed.
	if strings.TrimSpace(f.Fingerprint) == "" {
		f.Fingerprint = "-"
	}
	row, err := f.row(table.header, status)
	if err != nil {
		return 0, err
	}
	for _, r := range table.rows {
		if strings.EqualFold(strings.TrimSpace(cell(r.cells, 0)), strings.TrimSpace(f.ID)) {
			return 0, fmt.Errorf("finding %s is already row %d: this command never overwrites a verdict", quote(f.ID), r.line)
		}
	}
	if !pathCiteRe.MatchString(f.Location) {
		return 0, usagef("--location %s is not path:line, so nothing can be located", quote(f.Location))
	}
	// The ids are read exactly as the checker reads them, so the writer validates what the checker will.
	ledger := scanSection(lines, "Evidence ledger")
	known := map[string]bool{}
	for _, r := range ledger.rows {
		known[cell(r.cells, 0)] = true
	}
	for _, part := range strings.FieldsFunc(f.Evidence, func(r rune) bool { return r == ',' || r == ';' || r == '/' || r == ' ' }) {
		id := strings.Trim(part, "`")
		if id == "" || known[id] {
			continue
		}
		return 0, fmt.Errorf("--evidence names %s, which is not a row in the Evidence ledger: add the ledger row first, this command writes findings only", quote(id))
	}

	after := findingsEnd(table)
	candidate := strings.Join(insertLine(lines, after, row), "\n")
	if problems := CheckDocument(candidate); len(problems) > 0 {
		return 0, fmt.Errorf("the row would not pass plan check: %s; the plan is unchanged", strings.Join(problems, "; "))
	}
	if err := os.WriteFile(path, []byte(candidate), 0o644); err != nil {
		return 0, err
	}
	return after + 1, nil
}

// field is one value the row carries: the flag that names it and whether the row needs it.
type field struct {
	flag     string
	value    string
	required bool
}

// values lists the row's values in flag order; --status, --test and --fingerprint are conditional.
func (f Finding) values() []field {
	return []field{
		{"id", f.ID, true}, {"location", f.Location, true}, {"severity", f.Severity, true},
		{"data-safe", f.DataSafe, true}, {"evidence", f.Evidence, true}, {"test", f.Test, false},
		{"status", f.Status, false}, {"verdict-by", f.VerdictBy, true}, {"reason", f.Reason, true},
		{"fingerprint", f.Fingerprint, false},
	}
}

// checkValues refuses a pasted newline (one row into two) or an empty required value. Newlines first.
func (f Finding) checkValues() error {
	for _, v := range f.values() {
		if strings.ContainsAny(v.value, "\n\r") {
			return usagef("--%s carries a newline: a Findings row is one line", v.flag)
		}
	}
	for _, v := range f.values() {
		if v.required && strings.TrimSpace(v.value) == "" {
			return usagef("--%s is empty and the row needs it", v.flag)
		}
	}
	return nil
}

// checkStatus reads --status against the closed vocabulary the checker enforces, so writer and checker
// cannot drift: an unknown value is refused with the vocabulary rather than silently read as `open`.
func (f Finding) checkStatus() (string, error) {
	status, breach := findingStatus(f.Status)
	if breach != "" {
		return "", usagef("--status %s is not one of: %s", quote(f.Status), FindingsStatusList)
	}
	return status, nil
}

// checkTest is the writer's half of the checker's rule: a settled finding owes its pinning test.
func (f Finding) checkTest(status string) error {
	if settledStatus.MatchString(status) && placeholder.MatchString(strings.TrimSpace(f.Test)) {
		return usagef("--status %s is settled, so --test must name the pinning test that holds the verdict", status)
	}
	return nil
}

// row renders the row in header order and reads it back through the checker's own split for pipe safety.
func (f Finding) row(header []string, status string) (string, error) {
	cells := make([]string, len(header))

	id := idColumn(header)
	if id < 0 {
		return "", fmt.Errorf("the Findings header has no Id column, so the row cannot carry its identifier")
	}
	if placeholder.MatchString(strings.TrimSpace(f.ID)) {
		return "", usagef("--id %s is a placeholder: the checker skips a row whose Id cell is one, so the finding would never be read", quote(f.ID))
	}
	cells[id] = cellValue(f.ID)

	columns := []struct {
		name   string
		value  string
		needed bool
	}{
		{"Finding", f.Location, true},
		{"Severity", f.Severity, true},
		{"Data safe", f.DataSafe, true},
		{"Evidence", f.Evidence, true},
		// A settled finding owes the test; an unsettled one only carries the column when it has a test to
		// write, so a table that never had the column is not asked to grow one.
		{"Pinning test", f.Test, settledStatus.MatchString(status) || !placeholder.MatchString(strings.TrimSpace(f.Test))},
		{"Status", f.Status, true},
		{"Verdict by", f.VerdictBy, true},
		{"Reason", f.Reason, true},
		{"Fingerprint", f.Fingerprint, !placeholder.MatchString(strings.TrimSpace(f.Fingerprint))},
	}
	for _, c := range columns {
		at := columnIndex(header, c.name)
		switch {
		case at >= 0:
			cells[at] = cellValue(c.value)
		case c.needed:
			return "", fmt.Errorf("the Findings header has no %s column, so the row cannot carry it", c.name)
		}
	}

	line := "| " + strings.Join(cells, " | ") + " |"
	if got := len(split(line)); got != len(header) {
		return "", usagef("a value in the row shifts its columns: the row reads as %d cells where the header has %d", got, len(header))
	}
	return line, nil
}

// cellValue escapes a pipe, which would otherwise split the row and move every column to its right.
func cellValue(v string) string { return strings.ReplaceAll(strings.TrimSpace(v), "|", `\|`) }

// idColumn finds the Id column by exact name, so `Evidence id` is never mistaken for it.
func idColumn(header []string) int {
	for i, h := range header {
		if strings.EqualFold(strings.TrimSpace(h), "id") {
			return i
		}
	}
	return -1
}

// findingsEnd is the 1-based line the new row goes after: the last data row, else the table's heading.
func findingsEnd(scan tableScan) int {
	if len(scan.rows) > 0 {
		return scan.rows[len(scan.rows)-1].line
	}
	return scan.head
}

// insertLine puts row after the 1-based line at and leaves every other line where it was.
func insertLine(lines []string, at int, row string) []string {
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:at]...)
	out = append(out, row)
	return append(out, lines[at:]...)
}
