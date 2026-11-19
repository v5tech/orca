package cli

import "runtime/debug"

// version is set at build time with -ldflags for release binaries.
var version = "dev"

// VersionString returns the injected version or a build-derived fallback.
func VersionString() string {
	if version == "" {
		version = "dev"
	}
	if version != "dev" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}

	revision := ""
	modified := ""
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value
		}
	}
	if revision == "" {
		return version
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if modified == "true" {
		return "dev-" + revision + "-dirty"
	}
	return "dev-" + revision
}
