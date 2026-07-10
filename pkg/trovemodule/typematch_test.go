package trovemodule

import "testing"

func TestMatchType(t *testing.T) {
	t.Parallel()

	if !MatchType([]string{"note.*"}, "note.created") {
		t.Fatal("MatchType(note.*, note.created) = false, want true")
	}
	if MatchType([]string{"note.*"}, "mqtt.foo") {
		t.Fatal("MatchType(note.*, mqtt.foo) = true, want false")
	}
}
