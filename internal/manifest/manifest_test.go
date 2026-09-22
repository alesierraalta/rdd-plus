package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"strings"
	"testing"

	"github.com/alesierraalta/rdd-plus/internal/assets"
)

func TestEveryEmbeddedSkillIsAComponent(t *testing.T) {
	entries, err := fs.ReadDir(assets.Skills(), ".")
	if err != nil {
		t.Fatalf("read embedded skills: %v", err)
	}
	want := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() {
			want[entry.Name()] = true
		}
	}

	got := map[string]bool{}
	for _, component := range Components() {
		if component.Kind == KindSkill {
			got[component.ID] = true
			if component.Source != "skills/"+component.ID {
				t.Fatalf("component %q source = %q, want %q", component.ID, component.Source, "skills/"+component.ID)
			}
		}
	}
	if len(got) != len(want) {
		t.Fatalf("component skills = %d, want %d: got %v", len(got), len(want), got)
	}
	for name := range want {
		if !got[name] {
			t.Fatalf("embedded skill %q has no component; components = %v", name, got)
		}
	}
	for name := range got {
		if !want[name] {
			t.Fatalf("component %q has no embedded skill; embedded = %v", name, want)
		}
	}
}

func TestEveryFileIsHashedFromTheEmbedFS(t *testing.T) {
	got, err := Files()
	if err != nil {
		t.Fatalf("manifest files: %v", err)
	}
	for _, component := range Components() {
		if component.Kind != KindSkill {
			continue
		}
		files, ok := got[component.ID]
		if !ok {
			t.Fatalf("component %q has no manifest payload", component.ID)
		}
		bySource := map[string]File{}
		for _, file := range files {
			bySource[file.Source] = file
		}
		seen := map[string]bool{}
		err := fs.WalkDir(assets.Skills(), component.ID, func(rel string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			data, err := fs.ReadFile(assets.Skills(), rel)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(data)
			source := "skills/" + rel
			file, ok := bySource[source]
			if !ok {
				t.Fatalf("embedded file %q has no manifest entry", source)
			}
			seen[source] = true
			wantDigest := hex.EncodeToString(digest[:])
			if file.SHA256 != wantDigest {
				t.Fatalf("file %q digest = %q, want %q", source, file.SHA256, wantDigest)
			}
			if file.Size != int64(len(data)) {
				t.Fatalf("file %q size = %d, want %d", source, file.Size, len(data))
			}
			if file.Rel != strings.TrimPrefix(rel, component.ID+"/") {
				t.Fatalf("file %q relative path = %q, want %q", source, file.Rel, strings.TrimPrefix(rel, component.ID+"/"))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk embedded component %q: %v", component.ID, err)
		}
		if len(seen) != len(files) {
			t.Fatalf("component %q manifest files = %d, embedded files = %d", component.ID, len(files), len(seen))
		}
	}
}

func TestAppliesToHandlesWildcardsAndExplicitHosts(t *testing.T) {
	cases := []struct {
		name string
		host string
		c    Component
		want bool
	}{
		{name: "wildcard host", host: "claude", c: Component{Hosts: []string{"*"}}, want: true},
		{name: "wildcard other host", host: "opencode", c: Component{Hosts: []string{"*"}}, want: true},
		{name: "explicit match", host: "claude", c: Component{Hosts: []string{"claude"}}, want: true},
		{name: "explicit mismatch", host: "opencode", c: Component{Hosts: []string{"claude"}}, want: false},
		{name: "empty wildcard host", host: "", c: Component{Hosts: []string{"*"}}, want: false},
		{name: "empty explicit host", host: "", c: Component{Hosts: []string{"claude"}}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AppliesTo(tc.host, tc.c); got != tc.want {
				t.Fatalf("AppliesTo(%q, %+v) = %v, want %v", tc.host, tc.c, got, tc.want)
			}
		})
	}
}

func TestComponentFilesRefusesAComponentWithNoPayload(t *testing.T) {
	files, err := ComponentFiles("stop-gate")
	if err == nil {
		t.Fatalf("ComponentFiles(%q) returned files %v without an error", "stop-gate", files)
	}
}
