package main

import (
	"reflect"
	"testing"

	"task-fs/internal/backend"
)

func TestParseMountFlags(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		mountpoint string
		selection  string
		hidden     bool
		action     backend.RemoveAction
		wantErr    bool
	}{
		{"mountpoint only", []string{"/mnt"}, "/mnt", "", false, backend.RemoveComplete, false},
		{"long flags before mountpoint", []string{"--virtual-tags", "COMPLETED,DUE", "--virtual-hidden", "/mnt"}, "/mnt", "COMPLETED,DUE", true, backend.RemoveComplete, false},
		{"short flags after mountpoint", []string{"/mnt", "-t", "ALL", "-H"}, "/mnt", "ALL", true, backend.RemoveComplete, false},
		{"mixed positions", []string{"-H", "/mnt", "--virtual-tags=COMPLETED"}, "/mnt", "COMPLETED", true, backend.RemoveComplete, false},
		{"rm delete after mountpoint", []string{"/mnt", "--rm", "delete"}, "/mnt", "", false, backend.RemoveDelete, false},
		{"rm delete before mountpoint", []string{"--rm", "delete", "/mnt"}, "/mnt", "", false, backend.RemoveDelete, false},
		{"rm complete with equals", []string{"--rm=complete", "/mnt"}, "/mnt", "", false, backend.RemoveComplete, false},
		{"invalid rm value", []string{"/mnt", "--rm", "archive"}, "", "", false, "", true},
		{"missing mountpoint", []string{"--virtual-hidden"}, "", "", false, "", true},
		{"two mountpoints", []string{"/a", "/b"}, "", "", false, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp, selection, hidden, action, err := parseMountFlags(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseMountFlags(%v) = %q, want error", tt.args, mp)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseMountFlags(%v): %v", tt.args, err)
			}
			if mp != tt.mountpoint || selection != tt.selection || hidden != tt.hidden || action != tt.action {
				t.Errorf("parseMountFlags(%v) = (%q, %q, %v, %q), want (%q, %q, %v, %q)",
					tt.args, mp, selection, hidden, action, tt.mountpoint, tt.selection, tt.hidden, tt.action)
			}
		})
	}
}

func TestReorderFlagsKeepsFlagValuesTogether(t *testing.T) {
	got := reorderFlags([]string{"/mnt", "-t", "COMPLETED,DUE", "--virtual-hidden", "x"})
	want := []string{"-t", "COMPLETED,DUE", "--virtual-hidden", "/mnt", "x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("reorderFlags = %v, want %v", got, want)
	}

	got = reorderFlags([]string{"/mnt", "--rm", "delete", "--virtual-hidden"})
	want = []string{"--rm", "delete", "--virtual-hidden", "/mnt"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("reorderFlags with --rm = %v, want %v", got, want)
	}
}
