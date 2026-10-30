package view

import (
	"strings"
	"testing"
)

func TestSplitDefinition(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		filter   string
		modify   string
		writable bool
		wantErr  string
	}{
		{name: "empty", raw: ""},
		{name: "filter only", raw: "status:pending +next", filter: "status:pending +next"},
		{name: "trailing newline", raw: "status:pending +next\n", filter: "status:pending +next"},
		{name: "filter and modifications", raw: "status:pending +next\nsched:2026-10-26", filter: "status:pending +next", modify: "sched:2026-10-26", writable: true},
		{name: "crlf", raw: "status:pending\r\nsched:2026-10-26\r\n", filter: "status:pending", modify: "sched:2026-10-26", writable: true},
		{name: "blank second line", raw: "status:pending\n", filter: "status:pending"},
		{name: "whitespace second line", raw: "status:pending\n \t ", filter: "status:pending"},
		{name: "blank trailing lines", raw: "status:pending\n\n\n", filter: "status:pending"},
		{name: "blank first line", raw: "\nmodify", filter: "", modify: "modify", writable: true},
		{name: "third non-empty line", raw: "status:pending\nsched:2026-10-26\nextra", wantErr: "more than two non-empty lines"},
		{name: "blank line before third content", raw: "status:pending\n\nextra", wantErr: "more than two non-empty lines"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := SplitDefinition(tt.raw)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("SplitDefinition() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SplitDefinition(): %v", err)
			}
			if def.Filter != tt.filter || def.Modify != tt.modify || def.Writable() != tt.writable {
				t.Errorf("definition = %+v writable=%v, want filter=%q modify=%q writable=%v", def, def.Writable(), tt.filter, tt.modify, tt.writable)
			}
		})
	}
}

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		line    string
		want    []string
		wantErr string
	}{
		{line: "", want: nil},
		{line: "   \t", want: nil},
		{line: "sched:2026-10-26", want: []string{"sched:2026-10-26"}},
		{line: "description:\"call mom\" +family", want: []string{"description:call mom", "+family"}},
		{line: "foo\\ bar", want: []string{"foo bar"}},
		{line: "'single quoted'", want: []string{"single quoted"}},
		{line: "\"\"", want: []string{""}},
		{line: "a \"\" b", want: []string{"a", "", "b"}},
		{line: "a\"b c\"d", want: []string{"ab cd"}},
		{line: "$HOME *", want: []string{"$HOME", "*"}},
		{line: "back\\slash", want: []string{"backslash"}},
		{line: "a \"b", wantErr: "unterminated double quote"},
		{line: "a 'b", wantErr: "unterminated single quote"},
		{line: "a\\", wantErr: "trailing escape"},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got, err := SplitArgs(tt.line)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("SplitArgs(%q) error = %v, want containing %q", tt.line, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SplitArgs(%q): %v", tt.line, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("SplitArgs(%q) = %#v, want %#v", tt.line, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("SplitArgs(%q)[%d] = %q, want %q", tt.line, i, got[i], tt.want[i])
				}
			}
		})
	}
}
