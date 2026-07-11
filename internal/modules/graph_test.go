package modules

import "testing"

func TestPatternOverlaps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		provide string
		consume string
		want    bool
	}{
		{"note.created", "note.created", true},
		{"note.created", "note.updated", false},
		{"note.*", "note.created", true},
		{"note.created", "note.*", true},
		{"mqtt.*.received", "mqtt.foo.received", true},
		{"note.*", "mqtt.*", true},
	}
	for _, tt := range tests {
		if got := patternOverlaps(tt.provide, tt.consume); got != tt.want {
			t.Errorf("patternOverlaps(%q, %q) = %v, want %v", tt.provide, tt.consume, got, tt.want)
		}
	}
}

func TestFindCycles(t *testing.T) {
	t.Parallel()

	adj := map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"a"},
	}
	cycles := findCycles([]string{"a", "b", "c"}, adj)
	if len(cycles) == 0 {
		t.Fatal("findCycles() = none, want cycle")
	}
}
