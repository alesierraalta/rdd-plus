// Package assets carries the skill files that tpp installs; go:embed only reaches
// files below the package directory, so the skills live here rather than under internal/.
package assets

import "embed"

// Root holds every file under assets/skills, dotfiles and underscore names included.
//
//go:embed all:skills
var Root embed.FS
