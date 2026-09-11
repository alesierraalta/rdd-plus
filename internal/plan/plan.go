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
	// <classes>`. The declaration is the cheap half of the decision: a binary can read that the run
	// said so, never that the change was really bounded.
	lightHeader = regexp.MustCompile(`(?m)^\s*Light:`)
)

// Check reads a plan and returns everything that breaks the contract, most structural first.
// An empty result means the plan is well formed, not that the testing was good.
func Check(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc := string(raw)
	var problems []string

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
	if lightHeader.MatchString(doc) {
		problems = append(problems, unscopedLayers(doc)...)
	}
	return problems, nil
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
