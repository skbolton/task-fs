package backend

import (
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

func TestExportQueryComposition(t *testing.T) {
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
