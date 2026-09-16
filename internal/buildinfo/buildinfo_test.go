package buildinfo

import "testing"

// The benchmark records this string in every history row, and a row nobody can attribute to a build is a
// number nobody can reproduce. The reading moved here with the function it belongs to, so the assertion
// moved with it.
func TestRevisionIsNeverEmpty(t *testing.T) {
	if Revision() == "" {
		t.Fatal("a build must always name a revision, even outside a repository")
	}
	// Outside a repository Go embeds nothing, and "unknown" is the honest answer rather than an empty
	// cell a reader would take for a missing field.
	if Revision() == "unknown" && Version == "" {
		t.Fatal("a build with no revision still has to name its version")
	}
}
