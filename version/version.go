// Package version provides build version information.
package version

import (
	"runtime/debug"
)

// shortCommitLen is the length of the abbreviated commit hash.
const shortCommitLen = 7

// Info holds version information.
type Info struct {
	Version  string
	Commit   string
	Modified bool
	GoVer    string
}

// Get returns the build version information.
func Get() Info {
	info := Info{
		Version: "dev",
	}

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}

	info.GoVer = bi.GoVersion

	for _, setting := range bi.Settings {
		switch setting.Key {
		case "vcs.revision":
			info.Commit = setting.Value
			if len(info.Commit) > shortCommitLen {
				info.Commit = info.Commit[:shortCommitLen]
			}
		case "vcs.modified":
			info.Modified = setting.Value == "true"
		}
	}

	return info
}

// String returns a formatted version string.
func (i Info) String() string {
	if i.Commit == "" {
		return i.Version
	}

	s := i.Commit
	if i.Modified {
		s += "-dirty"
	}

	return s
}
