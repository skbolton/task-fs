package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"task-fs/internal/backend"
)

func TestParseMountFlags(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		mountpoint string
		action     backend.RemoveAction
		wantErr    bool
	}{
		{"mountpoint only", []string{"/mnt"}, "/mnt", backend.RemoveComplete, false},
		{"rm delete after mountpoint", []string{"/mnt", "--rm", "delete"}, "/mnt", backend.RemoveDelete, false},
		{"rm delete before mountpoint", []string{"--rm", "delete", "/mnt"}, "/mnt", backend.RemoveDelete, false},
		{"rm complete with equals", []string{"--rm=complete", "/mnt"}, "/mnt", backend.RemoveComplete, false},
		{"view after mountpoint rejected", []string{"/mnt", "--view", "Due Next Week=status:pending"}, "", "", true},
		{"view before mountpoint rejected", []string{"--view", "@Due=due.before:now", "/mnt"}, "", "", true},
		{"view equals rejected", []string{"/mnt", "--view=Due=due.before:now"}, "", "", true},
		{"old query flag rejected", []string{"/mnt", "--query", "Due=a"}, "", "", true},
		{"removed virtual-tags flag rejected", []string{"/mnt", "--virtual-tags", "COMPLETED,DUE"}, "", "", true},
		{"removed shorthand t flag rejected", []string{"/mnt", "-t", "ALL"}, "", "", true},
		{"removed virtual-hidden flag rejected", []string{"/mnt", "--virtual-hidden"}, "", "", true},
		{"removed shorthand H flag rejected", []string{"/mnt", "-H"}, "", "", true},
		{"invalid rm value", []string{"/mnt", "--rm", "archive"}, "", "", true},
		{"missing mountpoint", []string{"--rm", "delete"}, "", "", true},
		{"two mountpoints", []string{"/a", "/b"}, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp, action, err := parseMountFlags(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseMountFlags(%v) = %q, want error", tt.args, mp)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseMountFlags(%v): %v", tt.args, err)
			}
			if mp != tt.mountpoint || action != tt.action {
				t.Errorf("parseMountFlags(%v) = (%q, %q), want (%q, %q)",
					tt.args, mp, action, tt.mountpoint, tt.action)
			}
		})
	}
}

func TestReorderFlagsKeepsRemovalValueTogether(t *testing.T) {
	got := reorderFlags([]string{"/mnt", "--rm", "delete", "x"})
	want := []string{"--rm", "delete", "/mnt", "x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("reorderFlags = %v, want %v", got, want)
	}

	got = reorderFlags([]string{"/mnt", "--rm=delete", "x"})
	want = []string{"--rm=delete", "/mnt", "x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("reorderFlags with equals = %v, want %v", got, want)
	}
}

type fakeViewRunner struct {
	out   string
	err   error
	calls []string
}

func (r *fakeViewRunner) Run(_ context.Context, command string) ([]byte, error) {
	r.calls = append(r.calls, command)
	if r.err != nil {
		return nil, r.err
	}
	return []byte(r.out), nil
}

func TestNewQueryRendererRendersBlocks(t *testing.T) {
	runner := &fakeViewRunner{out: "2026-11-01\n"}
	render := newTemplateRenderer(runner)
	got, err := render(context.Background(), "status:pending due.before:{{ date +%F }}")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if want := "status:pending due.before:2026-11-01"; got != want {
		t.Errorf("render = %q, want %q", got, want)
	}
	if len(runner.calls) != 1 || runner.calls[0] != "date +%F" {
		t.Errorf("command calls = %v, want one date call", runner.calls)
	}
}

func TestNewQueryRendererReportsTemplateErrors(t *testing.T) {
	runner := &fakeViewRunner{out: "2026-11-01"}
	render := newTemplateRenderer(runner)
	if _, err := render(context.Background(), "due.before:{{ }}"); err == nil || !strings.Contains(err.Error(), "empty command block") {
		t.Errorf("empty block error = %v", err)
	}
	if _, err := render(context.Background(), "due.before:{{ date +%F"); err == nil || !strings.Contains(err.Error(), "unclosed command block") {
		t.Errorf("unclosed block error = %v", err)
	}
	if _, err := render(context.Background(), `literal \{{ not a command }}`); err != nil {
		t.Errorf("escaped delimiter: %v", err)
	}
	if len(runner.calls) != 0 {
		t.Errorf("template errors ran commands: %v", runner.calls)
	}
}

func TestOpenViewStoreFailure(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	t.Setenv("XDG_STATE_HOME", blocked)
	if _, err := openViewStore(); err == nil {
		t.Fatal("openViewStore succeeded with an unusable state root")
	}
}
