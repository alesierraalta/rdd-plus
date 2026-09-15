package assets

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/alesierraalta/rdd-plus/internal/buildinfo"
)

var expected = []string{
	"appsec-adversarial-auditor", "ask-or-research", "clean-architecture-audit", "database-persistence-testing",
	"dependency-legitimacy", "docker-test-containers", "exploit-testing", "implementation-theater",
	"no-excess-tests", "purpose-first", "real-run-validation", "runtime-reliability-testing", "silent-degradation",
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

// The skill tells a session which binary it was written for; the two versions must move together,
// or a session cannot tell whether the tool it has is the tool the skill expects.
func TestSkillNamesTheRddPlusVersionItRequires(t *testing.T) {
	data, err := fs.ReadFile(Skills(), "test-strategy/SKILL.md")
	if err != nil {
		t.Fatalf("SKILL.md missing: %v", err)
	}
	field := regexp.MustCompile(`(?m)^\s*requires_rdd_plus:\s*"?([0-9]+\.[0-9]+\.[0-9]+)"?\s*$`).FindSubmatch(data)
	if field == nil {
		t.Fatalf("test-strategy frontmatter has no requires_rdd_plus; this build is %s", buildinfo.Version)
	}
	if got := string(field[1]); got != buildinfo.Version {
		t.Fatalf("test-strategy requires rdd-plus %s, but this build is %s", got, buildinfo.Version)
	}
	if !strings.Contains(string(data), "make build") ||
		!strings.Contains(string(data), "go install github.com/alesierraalta/rdd-plus/cmd/rdd-plus@latest") {
		t.Fatalf("test-strategy requires rdd-plus %s but names no install path", buildinfo.Version)
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
