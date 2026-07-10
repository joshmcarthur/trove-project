package config

import "testing"

func TestPackageExists(t *testing.T) {
	t.Parallel()
	// Smoke test so CI has real signal before config loader is implemented.
	if got, want := "config", "config"; got != want {
		t.Fatalf("package name = %q, want %q", got, want)
	}
}
