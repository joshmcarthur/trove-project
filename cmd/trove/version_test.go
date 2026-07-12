package main

import "testing"

func TestFormatVersion(t *testing.T) {
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

			if got := formatVersion(tt.version, tt.commit, tt.date); got != tt.want {
				t.Errorf("formatVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
