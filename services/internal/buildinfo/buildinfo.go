// Package buildinfo reports which build of a service is running.
package buildinfo

import "runtime/debug"

// Version returns the VCS revision the binary was built from, shortened to
// 12 characters and suffixed with "-dirty" for uncommitted changes, or "dev"
// when the build has no VCS stamp (go test, go run).
func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	return fromSettings(info.Settings)
}

func fromSettings(settings []debug.BuildSetting) string {
	var revision string
	var dirty bool
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if revision == "" {
		return "dev"
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if dirty {
		revision += "-dirty"
	}
	return revision
}
