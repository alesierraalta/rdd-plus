package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ConfigName is the repository-local declaration read from the worktree root.
const ConfigName = ".rdd-plus.json"

// Reader reads a file's text; nil means os.ReadFile.
type Reader func(path string) (string, error)

type declaration struct {
	planPath string
	run      string
}

var runSlugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)

// readDeclaration reads the repository declaration once and resolves both keys. A declaration that
// cannot be read, does not parse, or carries an unusable value is an error: a typo must never read as
// "nothing declared".
func readDeclaration(root string, read Reader) (declaration, error) {
	if read == nil {
		read = func(path string) (string, error) {
			body, err := os.ReadFile(path)
			return string(body), err
		}
	}
	body, err := read(filepath.Join(root, ConfigName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return declaration{}, nil
		}
		return declaration{}, fmt.Errorf("%s: %w", ConfigName, err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &fields); err != nil {
		return declaration{}, fmt.Errorf("%s: %w", ConfigName, err)
	}
	if fields == nil {
		return declaration{}, fmt.Errorf("%s: declaration must be a JSON object", ConfigName)
	}
	// The key check must be a map check: Go's decoder matches struct field names case-insensitively,
	// so {"planpath": ...} would decode into the field and DisallowUnknownFields would not refuse it.
	// Checking the exact key is what makes a typo fail closed.
	for key := range fields {
		if key != "planPath" && key != "run" {
			return declaration{}, fmt.Errorf("%s: unknown field %q", ConfigName, key)
		}
	}

	var d declaration
	if declared, present := fields["planPath"]; present {
		if err := json.Unmarshal(declared, &d.planPath); err != nil {
			return declaration{}, fmt.Errorf("%s: planPath must be a string: %w", ConfigName, err)
		}
		if strings.TrimSpace(d.planPath) == "" {
			return declaration{}, fmt.Errorf("%s: planPath is empty", ConfigName)
		}
		if err := ValidatePlanPath(ConfigName, d.planPath); err != nil {
			return declaration{}, err
		}
	}
	if declared, present := fields["run"]; present {
		if err := json.Unmarshal(declared, &d.run); err != nil {
			return declaration{}, fmt.Errorf("%s: run must be a string: %w", ConfigName, err)
		}
		if err := ValidateRun(ConfigName, d.run); err != nil {
			return declaration{}, err
		}
	}
	return d, nil
}

// DeclaredPath returns the repository-relative plan the root declares, or "" when the root declares
// nothing.
func DeclaredPath(root string, read Reader) (string, error) {
	d, err := readDeclaration(root, read)
	return d.planPath, err
}

// DeclaredRun returns the active run the root declares, or "" when no run key is present.
func DeclaredRun(root string, read Reader) (string, error) {
	d, err := readDeclaration(root, read)
	return d.run, err
}

// Resolve returns the effective plan path and declared run for a worktree in one declaration read.
func Resolve(root string, read Reader) (string, string, error) {
	d, err := readDeclaration(root, read)
	if err != nil {
		return "", "", err
	}
	if d.planPath == "" {
		d.planPath = DefaultPath
	}
	return d.planPath, d.run, nil
}

// ResolvePath is the effective plan path for a worktree: the declaration when there is one, else
// DefaultPath.
func ResolvePath(root string, read Reader) (string, error) {
	path, _, err := Resolve(root, read)
	return path, err
}

// ResolveRun is the effective declared run for a worktree, or "" when no run key is present.
func ResolveRun(root string, read Reader) (string, error) {
	_, run, err := Resolve(root, read)
	return run, err
}

// DeclareRun writes the active run into the worktree declaration without replacing its plan path. When
// planPath is supplied for a new declaration, it becomes the repository-relative plan path; an existing
// planPath always wins. A declaration that cannot be read is refused before the atomic replacement, so a
// broken file is never repaired by overwriting the evidence of the refusal.
func DeclareRun(root, run string, planPath ...string) error {
	if err := ValidateRun("run", run); err != nil {
		return err
	}
	path := filepath.Join(root, ConfigName)
	d, err := readDeclaration(root, nil)
	if err != nil {
		return err
	}
	planPathChanged := false
	if d.planPath == "" && len(planPath) > 0 && planPath[0] != "" {
		if err := ValidatePlanPath("planPath", planPath[0]); err != nil {
			return err
		}
		d.planPath = filepath.Clean(planPath[0])
		planPathChanged = true
	}
	if d.run == run && !planPathChanged {
		return nil
	}
	return writeDeclaration(path, d, run)
}

// writeDeclaration serializes the two supported keys to a sibling temporary file and renames it over the
// declaration. The rename is the commit point: readers see either the previous JSON or the complete new JSON.
func writeDeclaration(path string, d declaration, run string) error {
	body, err := json.Marshal(struct {
		PlanPath string `json:"planPath,omitempty"`
		Run      string `json:"run,omitempty"`
	}{PlanPath: d.planPath, Run: run})
	if err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(body); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return nil
}

// StartRun seeds the six layer rows from the embedded template while holding the plan lock. It returns true
// when rows were inserted; an existing row carrying run is the idempotent marker for an already opened run.
func StartRun(path, run string) (bool, error) {
	if err := ValidateRun("run", run); err != nil {
		return false, err
	}
	target := canonicalPath(path)
	lock, err := lockPlan(target)
	if err != nil {
		return false, err
	}
	defer unlockPlan(lock)

	raw, err := os.ReadFile(target)
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(raw), "\n")
	layers := scanSection(lines, "Layer matrix")
	if layers.header == nil {
		return false, fmt.Errorf("the plan has no Layer matrix; run rdd-plus plan upgrade to add the table before opening a run")
	}
	if _, message := interruptedTable("Layer matrix", layers); message != "" {
		return false, fmt.Errorf("%s; run rdd-plus plan upgrade before opening a run", message)
	}
	iLayer := columnIndex(layers.header, "layer")
	iSkill := columnIndex(layers.header, "skill")
	iScope := columnIndex(layers.header, "scope")
	iStatus := columnIndex(layers.header, "status")
	iRun := columnIndex(layers.header, "run")
	if iRun < 0 {
		return false, fmt.Errorf("the Layer matrix has no Run column; run rdd-plus plan upgrade before opening a run")
	}
	for name, at := range map[string]int{"Layer": iLayer, "Skill": iSkill, "Scope": iScope, "Status": iStatus} {
		if at < 0 {
			return false, fmt.Errorf("the Layer matrix has no %s column, so run start cannot seed its rows", name)
		}
	}
	for _, r := range layers.rows {
		if cell(r.cells, iRun) == run {
			return false, nil
		}
	}

	templateBody, err := Template()
	if err != nil {
		return false, err
	}
	templateLayers := scanSection(strings.Split(templateBody, "\n"), "Layer matrix")
	if len(templateLayers.rows) != 6 {
		return false, fmt.Errorf("the embedded Layer matrix has %d rows, want six", len(templateLayers.rows))
	}
	templateIndexes := map[string]int{
		"Layer":  columnIndex(templateLayers.header, "layer"),
		"Skill":  columnIndex(templateLayers.header, "skill"),
		"Scope":  columnIndex(templateLayers.header, "scope"),
		"Status": columnIndex(templateLayers.header, "status"),
	}
	for name, at := range templateIndexes {
		if at < 0 {
			return false, fmt.Errorf("the embedded Layer matrix has no %s column", name)
		}
	}
	seeded := make([]string, 0, len(templateLayers.rows))
	for _, source := range templateLayers.rows {
		cells := make([]string, len(layers.header))
		cells[iLayer] = cellValue(cell(source.cells, templateIndexes["Layer"]))
		cells[iSkill] = cellValue(cell(source.cells, templateIndexes["Skill"]))
		cells[iScope] = cellValue(cell(source.cells, templateIndexes["Scope"]))
		cells[iStatus] = "pending"
		cells[iRun] = cellValue(run)
		seeded = append(seeded, "| "+strings.Join(cells, " | ")+" |")
	}
	at := layers.head
	if len(layers.rows) > 0 {
		at = layers.rows[len(layers.rows)-1].line
	}
	for i, row := range seeded {
		lines = insertLine(lines, at+i, row)
	}
	if err := writePlan(target, strings.Join(lines, "\n")); err != nil {
		return false, err
	}
	return true, nil
}

// UnscopedRows counts breadth rows whose Run cell is blank, treating every row in a legacy table without
// that column as unscoped. It is a disclosure count for status, not the document-wide owed count.
func UnscopedRows(doc string) int {
	lines := strings.Split(doc, "\n")
	count := 0
	for _, name := range []string{"Layer matrix", "Ranked targets"} {
		scan := scanSection(lines, name)
		if scan.header == nil {
			continue
		}
		iRun := columnIndex(scan.header, "run")
		for _, r := range scan.rows {
			if iRun < 0 || cell(r.cells, iRun) == "" {
				count++
			}
		}
	}
	return count
}

// ValidateRun refuses an empty, malformed, or reserved run slug.
func ValidateRun(source, s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("%s must name a run", source)
	}
	if !runSlugRe.MatchString(s) {
		return fmt.Errorf("%s must be a valid run slug [a-z0-9][a-z0-9-]{1,63}: %q", source, s)
	}
	if s == "all" || s == "none" {
		return fmt.Errorf("%s cannot use reserved run %q", source, s)
	}
	return nil
}

// ResolveFromRoot turns a --path value into the file to read: an absolute value is taken as given, a
// relative one is resolved against root under the same containment rule the declaration obeys.
func ResolveFromRoot(root, source, p string) (string, error) {
	if filepath.IsAbs(p) {
		return p, nil
	}
	if err := ValidatePlanPath(source, p); err != nil {
		return "", err
	}
	return filepath.Join(root, filepath.Clean(p)), nil
}

// ValidatePlanPath rejects a declaration path that is not repository-relative or that escapes the
// worktree. source names where the value came from (the config file, or the flag) for the message.
func ValidatePlanPath(source, p string) error {
	if strings.TrimSpace(p) == "" {
		return fmt.Errorf("%s must name a plan path", source)
	}
	if filepath.IsAbs(p) {
		return fmt.Errorf("%s must be repository-relative: %q is absolute", source, p)
	}
	clean := filepath.Clean(p)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s escapes the worktree: %q", source, p)
	}
	return nil
}
