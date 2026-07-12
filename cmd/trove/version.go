package main

import "fmt"

// version, commit, and date are set at build time via -ldflags "-X main.version=...".
var (
	version = "dev"
	commit  = "none"
	date    = ""
)

func versionString() string {
	if commit == "none" && date == "" {
		return version
	}
	d := date
	if len(d) >= 10 {
		d = d[:10]
	}
	return fmt.Sprintf("%s (%s, %s)", version, commit, d)
}
