package main

import (
	"runtime/debug"
)

// productVersion is the user-facing build number. Each build script bumps it.
const productVersion = "0.1.4"

func (a *App) Version() string {
	return AppVersion()
}

func AppVersion() string {
	revision := vcsRevision()
	if revision == "" {
		return productVersion
	}
	return productVersion + " · " + revision
}

func appTitle() string {
	return "NekSSH " + AppVersion()
}

func vcsRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	revision := ""
	dirty := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if revision != "" && dirty {
		revision += "+"
	}
	return revision
}
