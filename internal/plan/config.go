package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ConfigName is the repository-local declaration read from the worktree root.
const ConfigName = ".rdd-plus.json"

// Config is the declaration's shape.
type Config struct {
	PlanPath string `json:"planPath"`
}

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

	var cfg Config
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return "", fmt.Errorf("%s: %w", ConfigName, err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return "", fmt.Errorf("%s: multiple JSON values", ConfigName)
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
	for key := range fields {
		if key != "planPath" {
			return "", fmt.Errorf("%s: unknown field %q", ConfigName, key)
		}
	}
	if _, present := fields["planPath"]; !present {
		return "", nil
	}
	if strings.TrimSpace(cfg.PlanPath) == "" {
		return "", fmt.Errorf("%s: planPath is empty", ConfigName)
	}
	if err := ValidatePlanPath(ConfigName, cfg.PlanPath); err != nil {
		return "", err
	}
	return cfg.PlanPath, nil
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
