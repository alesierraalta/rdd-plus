// Package assets exposes the embedded skills as a filesystem rooted at the skills directory.
package assets

import (
	"io/fs"
	"sort"

	root "github.com/alesierraalta/rdd-plus/assets"
)

// Skills returns the embedded skill tree: one directory per skill, each with a SKILL.md.
func Skills() fs.FS {
	sub, err := fs.Sub(root.Root, "skills")
	if err != nil {
		panic("embedded skills missing: " + err.Error())
	}
	return sub
}

// SkillNames lists the embedded skills in a stable order.
func SkillNames() []string {
	entries, err := fs.ReadDir(Skills(), ".")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}
