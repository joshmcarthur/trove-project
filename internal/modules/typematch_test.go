package modules

import (
	"testing"
)

func TestMatchType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		patterns []string
		event    string
		want     bool
	}{
		{name: "exact match", patterns: []string{"a"}, event: "a", want: true},
		{name: "exact mismatch", patterns: []string{"a"}, event: "b", want: false},
		{name: "note wildcard", patterns: []string{"trove://type/note/*"}, event: "trove://type/note/created/1", want: true},
		{name: "note wildcard miss", patterns: []string{"trove://type/note/*"}, event: "trove://type/notes/created/1", want: false},
		{name: "mqtt exact", patterns: []string{"trove://type/mqtt/message/received/1"}, event: "trove://type/mqtt/message/received/1", want: true},
		{name: "mqtt exact miss", patterns: []string{"trove://type/mqtt/message/received/1"}, event: "trove://type/mqtt/foo/1", want: false},
		{name: "multiple patterns", patterns: []string{"trove://type/http/ingest/received/1", "trove://type/note/*"}, event: "trove://type/note/updated/1", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := MatchType(tt.patterns, tt.event); got != tt.want {
				t.Errorf("MatchType() = %v, want %v", got, tt.want)
			}
		})
	}
}
