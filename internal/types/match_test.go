package types_test

import (
	"testing"

	"github.com/joshmcarthur/trove/internal/types"
)

func TestMatchTypePattern(t *testing.T) {
	t.Parallel()

	if !types.MatchTypePattern("trove://type/note/*", "trove://type/note/created/1") {
		t.Fatal("MatchTypePattern(note/*, note/created/1) = false, want true")
	}
	if types.MatchTypePattern("trove://type/note/created/1", "trove://type/note/created/2") {
		t.Fatal("MatchTypePattern(exact v1, v2) = true, want false")
	}
	if !types.MatchTypePattern("trove://type/note/created/1", "trove://type/note/created/1") {
		t.Fatal("MatchTypePattern(exact) = false, want true")
	}
}

func TestMatchAnyPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		patterns []string
		typeURI  string
		want     bool
	}{
		{
			name:     "trove wildcard",
			patterns: []string{"trove://type/note/*"},
			typeURI:  "trove://type/note/created/1",
			want:     true,
		},
		{
			name:     "dotted wildcard",
			patterns: []string{"note.*"},
			typeURI:  "note.created",
			want:     true,
		},
		{
			name:     "exact dotted",
			patterns: []string{"http.ingest.received"},
			typeURI:  "http.ingest.received",
			want:     true,
		},
		{
			name:     "no match",
			patterns: []string{"note.*"},
			typeURI:  "mqtt.foo",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := types.MatchAnyPattern(tt.patterns, tt.typeURI); got != tt.want {
				t.Fatalf("MatchAnyPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}
