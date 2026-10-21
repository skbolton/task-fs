package main

import (
	"reflect"
	"testing"
)

func TestParseMountFlags(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		mountpoint string
		selection  string
		hidden     bool
		wantErr    bool
	}{
		{"mountpoint only", []string{"/mnt"}, "/mnt", "", false, false},
		{"long flags before mountpoint", []string{"--virtual-tags", "COMPLETED,DUE", "--virtual-hidden", "/mnt"}, "/mnt", "COMPLETED,DUE", true, false},
		{"short flags after mountpoint", []string{"/mnt", "-t", "ALL", "-H"}, "/mnt", "ALL", true, false},
		{"mixed positions", []string{"-H", "/mnt", "--virtual-tags=COMPLETED"}, "/mnt", "COMPLETED", true, false},
		{"missing mountpoint", []string{"--virtual-hidden"}, "", "", false, true},
		{"two mountpoints", []string{"/a", "/b"}, "", "", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp, selection, hidden, err := parseMountFlags(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseMountFlags(%v) = %q, want error", tt.args, mp)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseMountFlags(%v): %v", tt.args, err)
			}
			if mp != tt.mountpoint || selection != tt.selection || hidden != tt.hidden {
				t.Errorf("parseMountFlags(%v) = (%q, %q, %v), want (%q, %q, %v)",
					tt.args, mp, selection, hidden, tt.mountpoint, tt.selection, tt.hidden)
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
}
