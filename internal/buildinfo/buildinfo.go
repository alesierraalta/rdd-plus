// Package buildinfo names the build: the release it was cut from and the commit behind it.
package buildinfo

import "runtime/debug"

// Version is the release this build was cut from. It is overridable at build time:
// -ldflags "-X github.com/alesierraalta/rdd-plus/internal/buildinfo.Version=v1.2.3".
var Version = "0.3.17"

// String renders the version and the revision the build came from, such as "0.3.5 (fcb4ce6)"
// or "0.3.5 (unknown)" when the build carries no revision.
func String() string {
	return Version + " (" + Revision() + ")"
}

// Revision identifies the commit behind this build. Go embeds it when the binary is built inside
// a repository; a build without that information still names itself, as "unknown", so a version
// is never confused with an attributable one.
func Revision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	rev, dirty := "", false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return "unknown"
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if dirty {
		return rev + "+dirty"
	}
	return rev
}
