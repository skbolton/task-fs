package view

import (
	"context"
	"strings"
	"testing"
)

type fakeRunner struct {
	calls []string
	fn    func(string) ([]byte, error)
}

func (r *fakeRunner) Run(_ context.Context, command string) ([]byte, error) {
	r.calls = append(r.calls, command)
	return r.fn(command)
}

func TestParseAndRender(t *testing.T) {
	tests := []struct {
		name    string
		filter  string
		outputs map[string]string
		want    string
		calls   []string
	}{
		{
			name:   "static filter",
			filter: "status:pending",
			want:   "status:pending",
		},
		{
			name:    "single block",
			filter:  "due.before:{{ date }}",
			outputs: map[string]string{"date": "2026-11-01\n"},
			want:    "due.before:2026-11-01",
			calls:   []string{"date"},
		},
		{
			name:    "multiple blocks",
			filter:  "after:{{ start }} before:{{ end }}",
			outputs: map[string]string{"start": "2026-10-01", "end": "2026-10-31"},
			want:    "after:2026-10-01 before:2026-10-31",
			calls:   []string{"start", "end"},
		},
		{
			name:   "escaped delimiter",
			filter: `status:pending \{{ not a command }}`,
			want:   "status:pending {{ not a command }}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := Parse(tt.filter)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.filter, err)
			}
			runner := &fakeRunner{fn: func(command string) ([]byte, error) {
				return []byte(tt.outputs[command]), nil
			}}
			got, err := tmpl.Render(context.Background(), runner)
			if err != nil {
				t.Fatalf("Render(): %v", err)
			}
			if got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
			if strings.Join(runner.calls, ",") != strings.Join(tt.calls, ",") {
				t.Errorf("commands = %v, want %v", runner.calls, tt.calls)
			}
		})
	}
}

func TestParseRejectsInvalidTemplates(t *testing.T) {
	for _, filter := range []string{
		"due.before:{{ }}",
		"due.before:{{ date +%F",
		`status:pending \`,
	} {
		t.Run(filter, func(t *testing.T) {
			if _, err := Parse(filter); err == nil {
				t.Errorf("Parse(%q) succeeded, want error", filter)
			}
		})
	}
}

func TestRenderValidatesOutput(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    string
		wantErr string
	}{
		{name: "trailing newline", output: "2026-11-01\n", want: "2026-11-01"},
		{name: "surrounding whitespace", output: "  2026-11-01 \n", want: "2026-11-01"},
		{name: "internal spaces", output: "project:home or project:finances", want: "project:home or project:finances"},
		{name: "not reparsed", output: "{{ other }}", want: "{{ other }}"},
		{name: "empty", output: "", wantErr: "empty output"},
		{name: "whitespace only", output: " \n\t", wantErr: "empty output"},
		{name: "NUL", output: "2026\x00-11-01", wantErr: "NUL"},
		{name: "embedded newline", output: "one\ntwo", wantErr: "embedded newline"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := Parse("{{ value }}")
			if err != nil {
				t.Fatalf("Parse(): %v", err)
			}
			runner := &fakeRunner{fn: func(string) ([]byte, error) {
				return []byte(tt.output), nil
			}}
			got, err := tmpl.Render(context.Background(), runner)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Render() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Render(): %v", err)
			}
			if got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
			if len(runner.calls) != 1 {
				t.Errorf("commands = %v, want one call", runner.calls)
			}
		})
	}
}

func TestRenderRequiresRunnerForCommands(t *testing.T) {
	tmpl, err := Parse("due:{{ value }}")
	if err != nil {
		t.Fatalf("Parse(): %v", err)
	}
	if _, err := tmpl.Render(context.Background(), nil); err == nil {
		t.Error("Render() succeeded without a runner")
	}
}
