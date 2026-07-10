package journal

import "testing"

func TestFormatFTSQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only",
			input: "   ",
			want:  "",
		},
		{
			name:  "single token",
			input: "hello",
			want:  `"hello"`,
		},
		{
			name:  "multiple tokens",
			input: "hello world",
			want:  `"hello" "world"`,
		},
		{
			name:  "token with embedded quote",
			input: `foo"bar`,
			want:  `"foo""bar"`,
		},
		{
			name:  "fts operators treated as literals",
			input: "OR NOT temp*",
			want:  `"OR" "NOT" "temp*"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := formatFTSQuery(tt.input); got != tt.want {
				t.Errorf("formatFTSQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
