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
		{name: "note wildcard", patterns: []string{"note.*"}, event: "note.created", want: true},
		{name: "note wildcard miss", patterns: []string{"note.*"}, event: "note", want: false},
		{name: "mqtt wildcard", patterns: []string{"mqtt.*.received"}, event: "mqtt.sensor.temp.received", want: true},
		{name: "mqtt wildcard miss", patterns: []string{"mqtt.*.received"}, event: "mqtt.received", want: false},
		{name: "multiple patterns", patterns: []string{"http.ingest.received", "note.*"}, event: "note.updated", want: true},
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

func TestResolveSchemaPattern(t *testing.T) {
	t.Parallel()

	keys := []string{"note.*", "note.created", "http.ingest.received"}

	if got, ok := ResolveSchemaPattern(keys, "note.created"); !ok || got != "note.created" {
		t.Fatalf("ResolveSchemaPattern(note.created) = %q, %v; want note.created, true", got, ok)
	}
	if got, ok := ResolveSchemaPattern(keys, "note.updated"); !ok || got != "note.*" {
		t.Fatalf("ResolveSchemaPattern(note.updated) = %q, %v; want note.*, true", got, ok)
	}
	if got, ok := ResolveSchemaPattern(keys, "other.event"); ok {
		t.Fatalf("ResolveSchemaPattern(other.event) = %q, true; want false", got)
	}
}
