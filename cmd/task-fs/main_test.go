package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"task-fs/internal/backend"
	"task-fs/internal/vfs"
)

func TestParseMountFlags(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		mountpoint string
		selection  string
		hidden     bool
		action     backend.RemoveAction
		views      []vfs.View
		wantErr    bool
	}{
		{"mountpoint only", []string{"/mnt"}, "/mnt", "", false, backend.RemoveComplete, nil, false},
		{"long flags before mountpoint", []string{"--virtual-tags", "COMPLETED,DUE", "--virtual-hidden", "/mnt"}, "/mnt", "COMPLETED,DUE", true, backend.RemoveComplete, nil, false},
		{"short flags after mountpoint", []string{"/mnt", "-t", "ALL", "-H"}, "/mnt", "ALL", true, backend.RemoveComplete, nil, false},
		{"mixed positions", []string{"-H", "/mnt", "--virtual-tags=COMPLETED"}, "/mnt", "COMPLETED", true, backend.RemoveComplete, nil, false},
		{"rm delete after mountpoint", []string{"/mnt", "--rm", "delete"}, "/mnt", "", false, backend.RemoveDelete, nil, false},
		{"rm delete before mountpoint", []string{"--rm", "delete", "/mnt"}, "/mnt", "", false, backend.RemoveDelete, nil, false},
		{"rm complete with equals", []string{"--rm=complete", "/mnt"}, "/mnt", "", false, backend.RemoveComplete, nil, false},
		{"view after mountpoint", []string{"/mnt", "--view", "Due Next Week=status:pending sched.after:sonw sched.before:eonw"}, "/mnt", "", false, backend.RemoveComplete, []vfs.View{{Name: "Due Next Week", Filter: "status:pending sched.after:sonw sched.before:eonw"}}, false},
		{"view before mountpoint", []string{"--view", "@Due=due.before:now", "/mnt"}, "/mnt", "", false, backend.RemoveComplete, []vfs.View{{Name: "Due", Filter: "due.before:now"}}, false},
		{"repeated views", []string{"/mnt", "--view", "Due=a", "--view", "Status=status=pending"}, "/mnt", "", false, backend.RemoveComplete, []vfs.View{{Name: "Due", Filter: "a"}, {Name: "Status", Filter: "status=pending"}}, false},
		{"old query flag rejected", []string{"/mnt", "--query", "Due=a"}, "", "", false, "", nil, true},
		{"invalid rm value", []string{"/mnt", "--rm", "archive"}, "", "", false, "", nil, true},
		{"missing mountpoint", []string{"--virtual-hidden"}, "", "", false, "", nil, true},
		{"two mountpoints", []string{"/a", "/b"}, "", "", false, "", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp, selection, hidden, action, views, err := parseMountFlags(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseMountFlags(%v) = %q, want error", tt.args, mp)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseMountFlags(%v): %v", tt.args, err)
			}
			if mp != tt.mountpoint || selection != tt.selection || hidden != tt.hidden || action != tt.action || !reflect.DeepEqual(views.views, tt.views) {
				t.Errorf("parseMountFlags(%v) = (%q, %q, %v, %q, %v), want (%q, %q, %v, %q, %v)",
					tt.args, mp, selection, hidden, action, views.views, tt.mountpoint, tt.selection, tt.hidden, tt.action, tt.views)
			}
		})
	}
}

func TestParseViews(t *testing.T) {
	tests := []struct {
		name    string
		values  []string
		want    []vfs.View
		wantErr string
	}{
		{"name and filter with spaces", []string{"Due Next Week=status:pending sched.after:sonw sched.before:eonw"}, []vfs.View{{Name: "Due Next Week", Filter: "status:pending sched.after:sonw sched.before:eonw"}}, ""},
		{"leading at sign", []string{"@Due=due.before:now"}, []vfs.View{{Name: "Due", Filter: "due.before:now"}}, ""},
		{"equals inside filter", []string{"Status=status=pending"}, []vfs.View{{Name: "Status", Filter: "status=pending"}}, ""},
		{"dynamic filter", []string{"Payday=due.before:{{ date +%F }}"}, []vfs.View{{Name: "Payday", Filter: "due.before:{{ date +%F }}"}}, ""},
		{"escaped delimiter", []string{`Literal=status:pending \{{ not a command }}`}, []vfs.View{{Name: "Literal", Filter: `status:pending \{{ not a command }}`}}, ""},
		{"empty command block", []string{"Broken=due.before:{{ }}"}, nil, "empty command block"},
		{"unclosed command block", []string{"Broken=due.before:{{ date +%F"}, nil, "unclosed command block"},
		{"maximum name length", []string{strings.Repeat("a", 254) + "=status:pending"}, []vfs.View{{Name: strings.Repeat("a", 254), Filter: "status:pending"}}, ""},
		{"missing delimiter", []string{"Due"}, nil, "expected NAME=FILTER"},
		{"empty name", []string{"=due.before:now"}, nil, "invalid view"},
		{"empty filter", []string{"Due="}, nil, "empty filter"},
		{"whitespace filter", []string{"Due=   "}, nil, "empty filter"},
		{"slash in name", []string{"a/b=due.before:now"}, nil, "invalid view"},
		{"control in name", []string{"bad\tname=due.before:now"}, nil, "invalid view"},
		{"dot name", []string{"..=due.before:now"}, nil, "invalid view"},
		{"name too long", []string{strings.Repeat("a", 255) + "=status:pending"}, nil, "invalid view"},
		{"minted name", []string{"Report.a1b2c3d4.md=status:pending"}, nil, "invalid view"},
		{"duplicate name", []string{"Due=a", "Due=b"}, nil, "duplicate view name"},
		{"duplicate after leading at", []string{"Due=a", "@Due=b"}, nil, "duplicate view name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseViews(tt.values)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseViews(%v) error = %v, want containing %q", tt.values, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseViews(%v): %v", tt.values, err)
			}
			if !reflect.DeepEqual(got.views, tt.want) {
				t.Errorf("parseViews(%v) = %v, want %v", tt.values, got.views, tt.want)
			}
		})
	}
}

type mountRunner struct {
	err    error
	failOn string
	log    [][]string
}

func (r *mountRunner) Run(args []string) ([]byte, error) {
	r.log = append(r.log, args)
	if r.err != nil {
		return nil, r.err
	}
	if r.failOn != "" && strings.Contains(strings.Join(args, " "), r.failOn) {
		return nil, errors.New("bad filter")
	}
	return []byte("[]"), nil
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

func TestValidateViews(t *testing.T) {
	goodViews, err := parseViews([]string{"Nothing=project:does-not-exist"})
	if err != nil {
		t.Fatalf("parseViews: %v", err)
	}
	if err := validateViews(backend.NewQuery(&mountRunner{}), goodViews, nil); err != nil {
		t.Fatalf("zero-match view rejected: %v", err)
	}

	badViews, err := parseViews([]string{"Broken=("})
	if err != nil {
		t.Fatalf("parseViews: %v", err)
	}
	err = validateViews(backend.NewQuery(&mountRunner{err: errors.New("bad filter")}), badViews, nil)
	if err == nil || !strings.Contains(err.Error(), "Broken") {
		t.Errorf("error = %v, want it to name Broken", err)
	}
}

func TestValidateViewsRendersDynamicFilters(t *testing.T) {
	views, err := parseViews([]string{"Due=due.before:{{ date +%F }}"})
	if err != nil {
		t.Fatalf("parseViews: %v", err)
	}
	queryRunner := &mountRunner{}
	commandRunner := &fakeViewRunner{out: "2026-11-01\n"}
	if err := validateViews(backend.NewQuery(queryRunner), views, views.renderer(commandRunner)); err != nil {
		t.Fatalf("validateViews: %v", err)
	}
	if len(queryRunner.log) != 1 || queryRunner.log[0][0] != "( due.before:2026-11-01 )" {
		t.Errorf("query args = %v, want rendered filter", queryRunner.log)
	}
	if len(commandRunner.calls) != 1 || commandRunner.calls[0] != "date +%F" {
		t.Errorf("command calls = %v, want one date call", commandRunner.calls)
	}
}

func TestValidateViewsReportsDynamicFailures(t *testing.T) {
	tests := []struct {
		name    string
		runner  *fakeViewRunner
		query   *mountRunner
		wantErr string
	}{
		{
			name:    "command failure",
			runner:  &fakeViewRunner{err: errors.New("boom")},
			query:   &mountRunner{},
			wantErr: "boom",
		},
		{
			name:    "empty output",
			runner:  &fakeViewRunner{out: "  \n"},
			query:   &mountRunner{},
			wantErr: "empty output",
		},
		{
			name:    "malformed rendered filter",
			runner:  &fakeViewRunner{out: "malformed"},
			query:   &mountRunner{failOn: "malformed"},
			wantErr: "bad filter",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			views, err := parseViews([]string{"Broken={{ value }}"})
			if err != nil {
				t.Fatalf("parseViews: %v", err)
			}
			err = validateViews(backend.NewQuery(tt.query), views, views.renderer(tt.runner))
			if err == nil || !strings.Contains(err.Error(), "Broken") || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want Broken and %q", err, tt.wantErr)
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

	got = reorderFlags([]string{"/mnt", "--view", "Due Next Week=status:pending", "--virtual-hidden"})
	want = []string{"--view", "Due Next Week=status:pending", "--virtual-hidden", "/mnt"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("reorderFlags with --view = %v, want %v", got, want)
	}
}
