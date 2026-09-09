package assets

import (
	"io/fs"
	"regexp"
	"testing"
)

var expected = []string{
	"appsec-adversarial-auditor", "clean-architecture-audit", "database-persistence-testing",
	"dependency-legitimacy", "docker-test-containers", "exploit-testing", "implementation-theater",
	"no-excess-tests", "real-run-validation", "runtime-reliability-testing", "silent-degradation",
	"test-strategy",
}

func TestEverySkillIsEmbeddedWithAMatchingName(t *testing.T) {
	names := SkillNames()
	if len(names) != len(expected) {
		t.Fatalf("embedded %d skills, want %d: %v", len(names), len(expected), names)
	}
	nameLine := regexp.MustCompile(`(?m)^name:\s*"?([a-z0-9-]+)"?\s*$`)
	for i, name := range expected {
		t.Run(name, func(t *testing.T) {
			if names[i] != name {
				t.Fatalf("position %d is %q, want %q", i, names[i], name)
			}
			data, err := fs.ReadFile(Skills(), name+"/SKILL.md")
			if err != nil {
				t.Fatalf("SKILL.md missing: %v", err)
			}
			m := nameLine.FindSubmatch(data)
			if m == nil || string(m[1]) != name {
				t.Fatalf("frontmatter name does not match directory: %q", m)
			}
		})
	}
}

func TestNoRunArtifactsAreEmbedded(t *testing.T) {
	banned := regexp.MustCompile(`(^|/)(__pycache__|\.runs|results)(/|$)`)
	err := fs.WalkDir(Skills(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if banned.MatchString(p) {
			t.Errorf("run artifact embedded: %s", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
