package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ConfigName is the repository-local declaration read from the worktree root.
const ConfigName = ".rdd-plus.json"

// Reader reads a file's text; nil means os.ReadFile.
type Reader func(path string) (string, error)

// DeclaredPath returns the repository-relative plan the root declares, or "" when the root declares
// nothing. A declaration that cannot be read, does not parse, or carries an unusable path is an
// error: a typo must never read as "nothing declared".
func DeclaredPath(root string, read Reader) (string, error) {
	if read == nil {
		read = func(path string) (string, error) {
			body, err := os.ReadFile(path)
			return string(body), err
		}
	}
	body, err := read(filepath.Join(root, ConfigName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("%s: %w", ConfigName, err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &fields); err != nil {
		return "", fmt.Errorf("%s: %w", ConfigName, err)
	}
	if fields == nil {
		return "", fmt.Errorf("%s: declaration must be a JSON object", ConfigName)
	}
	// The key check must be a map check: Go's decoder matches struct field names case-insensitively,
	// so {"planpath": ...} would decode into the field and DisallowUnknownFields would not refuse it.
	// Checking the exact key is what makes a typo fail closed.
	for key := range fields {
		if key != "planPath" {
			return "", fmt.Errorf("%s: unknown field %q", ConfigName, key)
		}
	}
	declared, present := fields["planPath"]
	if !present {
		return "", nil
	}
	var path string
	if err := json.Unmarshal(declared, &path); err != nil {
		return "", fmt.Errorf("%s: planPath must be a string: %w", ConfigName, err)
	}
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("%s: planPath is empty", ConfigName)
	}
	if err := ValidatePlanPath(ConfigName, path); err != nil {
		return "", err
	}
	return path, nil
}

// ResolvePath is the effective plan path for a worktree: the declaration when there is one, else
// DefaultPath.
func ResolvePath(root string, read Reader) (string, error) {
	declared, err := DeclaredPath(root, read)
	if err != nil {
		return "", err
	}
	if declared == "" {
		return DefaultPath, nil
	}
	return declared, nil
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
