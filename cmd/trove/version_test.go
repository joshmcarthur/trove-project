package main

import "testing"

func TestVersionString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		commit  string
		date    string
		want    string
	}{
		{
			name:    "dev default",
			version: "dev",
			commit:  "none",
			date:    "",
			want:    "dev",
		},
		{
			name:    "release",
			version: "0.1.0",
			commit:  "abc1234",
			date:    "2026-07-12T07:00:00Z",
			want:    "0.1.0 (abc1234, 2026-07-12)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			version = tt.version
			commit = tt.commit
			date = tt.date
			t.Cleanup(func() {
				version = "dev"
				commit = "none"
				date = ""
			})

			if got := versionString(); got != tt.want {
				t.Errorf("versionString() = %q, want %q", got, tt.want)
			}
		})
	}
}
