package trovemodule

import "testing"

func TestMatchType(t *testing.T) {
	t.Parallel()

	if !MatchType([]string{"trove://type/note/*"}, "trove://type/note/created/1") {
		t.Fatal("MatchType(trove://type/note/*, trove://type/note/created/1) = false, want true")
	}
	if MatchType([]string{"trove://type/note/*"}, "trove://type/mqtt/foo/1") {
		t.Fatal("MatchType(trove://type/note/*, trove://type/mqtt/foo/1) = true, want false")
	}
}
