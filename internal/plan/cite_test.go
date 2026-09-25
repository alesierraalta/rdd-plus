package plan

import "testing"

// A finding location is path:line. Extensionless build files and dotfiles are real locations; a host
// port or a clock time is not.
func TestPathCiteAcceptsExtensionlessFilesAndRejectsNonPaths(t *testing.T) {
	for cite, want := range map[string]bool{
		"src/a.js:5":          true,
		"Makefile:3":          true,
		"build/Dockerfile:10": true,
		".gitignore:1":        true,
		"dir/.gitignore:1":    true,
		"src/a.js":            false,
		"localhost:8080":      false,
		"10:30":               false,
		"NotMakefile:3":       false,
	} {
		if got := pathCiteRe.MatchString(cite); got != want {
			t.Errorf("pathCiteRe.MatchString(%q) = %v, want %v", cite, got, want)
		}
	}
}
