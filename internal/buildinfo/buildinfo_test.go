package buildinfo

import (
	"strings"
	"testing"
)

// The benchmark records this string in every history row, and a row nobody can attribute to a build is a
// number nobody can reproduce. The reading moved here with the function it belongs to, so the assertion
// moved with it.
func TestRevisionIsNeverEmpty(t *testing.T) {
	if Revision() == "" {
		t.Fatal("a build must always name a revision, even outside a repository")
	}
}

// String is what `tpp version` prints and what this package exists to compose, so both halves have to
// survive in it: a version that lost its revision names a release nobody can check out. The assertion the
// review found here promised this contract and could not check it — `Version` is never empty, so the branch
// that guarded it was unreachable — and this one fails as soon as either half goes missing.
func TestStringCarriesBothTheVersionAndTheRevision(t *testing.T) {
	got := String()
	if !strings.Contains(got, Version) || !strings.Contains(got, Revision()) {
		t.Fatalf("String() = %q, want the version %q and the revision %q in it", got, Version, Revision())
	}
}
