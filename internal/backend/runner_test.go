package backend

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRunnerPropagatesEnvironment(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "task")
	scriptBody := "#!/bin/sh\necho \"$TASKDATA\"\n"
	if err := os.WriteFile(script, []byte(scriptBody), 0755); err != nil {
		t.Fatal(err)
	}
	r := NewRunner(script, []string{"TASKDATA=/tmp/other-tasks"})
	out, err := r.Run([]string{"--version"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(string(out), "/tmp/other-tasks") {
		t.Errorf("environment not propagated: %q", string(out))
	}
}

func TestRunnerReportsNonZeroExit(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "task")
	scriptBody := "#!/bin/sh\necho 'boom' >&2\nexit 3\n"
	if err := os.WriteFile(script, []byte(scriptBody), 0755); err != nil {
		t.Fatal(err)
	}
	r := NewRunner(script, os.Environ())
	_, err := r.Run([]string{"export"})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("want stderr in error, got %v", err)
	}
}

func TestExportFilterComposition(t *testing.T) {
	runner := &stubRunner{out: []byte(`[]`)}
	q := NewQuery(runner)
	var f Filter
	f, _ = f.WithSegment("home")
	f, _ = f.WithSegment("+next")
	if _, err := q.Export(f); err != nil {
		t.Fatal(err)
	}
	want := []string{"project:home", "+next", "status:pending", "export"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Errorf("args = %v, want %v", runner.args, want)
	}
}

func TestExportQueryComposition(t *testing.T) {
	runner := &stubRunner{out: []byte(`[]`)}
	q := NewQuery(runner)
	if _, err := q.ExportQuery("status:pending sched.after:sonw sched.before:eonw"); err != nil {
		t.Fatal(err)
	}
	want := []string{"( status:pending sched.after:sonw sched.before:eonw )", "export"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Errorf("args = %v, want %v", runner.args, want)
	}
}

func TestExportQueryKeepsTrailingColonTerm(t *testing.T) {
	runner := &stubRunner{out: []byte(`[]`)}
	q := NewQuery(runner)
	if _, err := q.ExportQuery("status.any:"); err != nil {
		t.Fatal(err)
	}
	want := []string{"( status.any: )", "export"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Errorf("args = %v, want %v", runner.args, want)
	}
}

func TestExportQueryKeepsQuotedPhrase(t *testing.T) {
	runner := &stubRunner{out: []byte(`[]`)}
	q := NewQuery(runner)
	if _, err := q.ExportQuery(`description.contains:"Get proficient"`); err != nil {
		t.Fatal(err)
	}
	want := []string{`( description.contains:"Get proficient" )`, "export"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Errorf("args = %v, want %v", runner.args, want)
	}
}

func TestExportQueryZeroMatches(t *testing.T) {
	runner := &stubRunner{out: []byte(`[]`)}
	q := NewQuery(runner)
	tasks, err := q.ExportQuery("project:does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Errorf("tasks = %+v, want none", tasks)
	}
}

func TestExportQueryPropagatesError(t *testing.T) {
	runner := &stubRunner{err: errors.New("bad filter")}
	q := NewQuery(runner)
	if _, err := q.ExportQuery("("); err == nil {
		t.Fatal("expected error")
	}
}

func TestSweepQueryComposition(t *testing.T) {
	tests := []struct {
		name   string
		action RemoveAction
		want   []string
	}{
		{"delete", RemoveDelete, []string{"( +next )", "status.not:deleted", "export"}},
		{"complete", RemoveComplete, []string{"( +next )", "status.not:deleted", "status.not:completed", "export"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &stubRunner{out: []byte(`[]`)}
			q := NewQuery(runner)
			if _, err := q.SweepQuery("+next", tt.action); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(runner.args, tt.want) {
				t.Errorf("args = %v, want %v", runner.args, tt.want)
			}
		})
	}
}

func TestSweepTasksQueryComposition(t *testing.T) {
	runner := &stubRunner{out: []byte(`[{"uuid":"a1b2c3d4-0000-0000-0000-000000000000","description":"Do taxes","project":"home","status":"pending"}]`)}
	q := NewQuery(runner)
	var f Filter
	f, _ = f.WithSegment("home")
	f, _ = f.WithSegment("+next")
	tasks, err := q.SweepTasks(f, RemoveDelete)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"project:home", "+next", "status.not:deleted", "export"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Errorf("args = %v, want %v", runner.args, want)
	}
	if len(tasks) != 1 || tasks[0].UUID != "a1b2c3d4-0000-0000-0000-000000000000" {
		t.Errorf("sweep tasks = %+v", tasks)
	}
}

func TestRemoveDispatch(t *testing.T) {
	const uuid = "a1b2c3d4-0000-0000-0000-000000000000"
	tests := []struct {
		name   string
		action RemoveAction
		want   []string
	}{
		{"complete", RemoveComplete, []string{"rc.confirmation=off", uuid, "done"}},
		{"delete", RemoveDelete, []string{"rc.confirmation=off", uuid, "delete"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &stubRunner{}
			q := NewQuery(runner)
			if err := q.Remove(uuid, tt.action); err != nil {
				t.Fatalf("Remove(%q): %v", tt.action, err)
			}
			if !reflect.DeepEqual(runner.args, tt.want) {
				t.Errorf("args = %v, want %v", runner.args, tt.want)
			}
		})
	}
}

func TestRemoveRejectsUnknownAction(t *testing.T) {
	runner := &stubRunner{}
	q := NewQuery(runner)
	if err := q.Remove("a1b2c3d4-0000-0000-0000-000000000000", RemoveAction("archive")); err == nil {
		t.Fatal("expected error for unknown remove action")
	}
	if len(runner.args) != 0 {
		t.Errorf("unknown action ran a command: %v", runner.args)
	}
}

func TestSweepTasksCompleteExcludesCompleted(t *testing.T) {
	runner := &stubRunner{out: []byte(`[]`)}
	q := NewQuery(runner)
	var f Filter
	f, _ = f.WithSegment("home")
	if _, err := q.SweepTasks(f, RemoveComplete); err != nil {
		t.Fatal(err)
	}
	want := []string{"project:home", "status.not:deleted", "status.not:completed", "export"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Errorf("args = %v, want %v", runner.args, want)
	}
}
