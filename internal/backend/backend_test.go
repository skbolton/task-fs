package backend

import (
	"os"
	"reflect"
	"testing"
)

type stubRunner struct {
	args []string
	out  []byte
	err  error
}

func (s *stubRunner) Run(args []string) ([]byte, error) {
	s.args = args
	return s.out, s.err
}

func TestParseExport(t *testing.T) {
	canned := `[
  {"uuid":"f88d7a5f-8d68-44fd-8cd3-3725592b673e","description":"Qt6 signals and slots","project":"dk","tags":["next"],"status":"pending"},
  {"uuid":"a1b2c3d4-0000-0000-0000-000000000000","description":"Do taxes","project":"finances.2026-taxes","tags":[],"status":"pending"}
]`
	tasks, err := ParseExport([]byte(canned))
	if err != nil {
		t.Fatalf("ParseExport: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("want 2 tasks, got %d", len(tasks))
	}
	want := Task{
		UUID:        "f88d7a5f-8d68-44fd-8cd3-3725592b673e",
		Description: "Qt6 signals and slots",
		Project:     "dk",
		Tags:        []string{"next"},
		Status:      "pending",
	}
	if !reflect.DeepEqual(tasks[0], want) {
		t.Errorf("task 0 = %+v, want %+v", tasks[0], want)
	}
}

func TestRunnerErrorWhenMissing(t *testing.T) {
	r := NewRunner("task-fs-nonexistent-binary-xyz", os.Environ())
	if _, err := r.Run([]string{"--version"}); err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestFilterRender(t *testing.T) {
	tests := []struct {
		name string
		path []string
		want []string
	}{
		{"root", nil, []string{"status:pending"}},
		{"project", []string{"home"}, []string{"project:home", "status:pending"}},
		{"subproject", []string{"home", "clean-garage"}, []string{"project:home.clean-garage", "status:pending"}},
		{"tag", []string{"+next"}, []string{"+next", "status:pending"}},
		{"virtual tag verbatim", []string{".+DUE"}, []string{"+DUE", "status:pending"}},
		{"completed", []string{".+COMPLETED", "home"}, []string{"project:home", "status:completed"}},
		{"order free", []string{"home", "+next"}, []string{"project:home", "+next", "status:pending"}},
		{"deep", []string{"finances", "2026-taxes", "+urgent", ".+DUE"}, []string{"project:finances.2026-taxes", "+urgent", "+DUE", "status:pending"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f Filter
			var err error
			for _, seg := range tt.path {
				f, err = f.WithSegment(seg)
				if err != nil {
					t.Fatalf("WithSegment(%q): %v", seg, err)
				}
			}
			got := f.Render()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("render = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterSegmentErrors(t *testing.T) {
	var f Filter
	f, _ = f.WithSegment("home")
	f, _ = f.WithSegment("+next")

	if _, err := f.WithSegment("+next"); err == nil {
		t.Error("expected error re-adding same tag")
	}
	if _, err := f.WithSegment("home"); err == nil {
		t.Error("expected error re-adding same project")
	}
	var err error
	f, err = f.WithSegment(".+DUE")
	if err != nil {
		t.Fatalf("add virtual tag: %v", err)
	}
	if _, err := f.WithSegment(".+DUE"); err == nil {
		t.Error("expected error re-adding same virtual tag")
	}
	if _, err := f.WithSegment(".hidden"); err == nil {
		t.Error("expected error for dot-prefixed segment")
	}
	if _, err := f.WithSegment(".+due"); err == nil {
		t.Error("expected error for lowercase virtual-tag segment")
	}
	if _, err := f.WithSegment(""); err == nil {
		t.Error("expected error for empty segment")
	}
}

func TestWriteFilterDropsVirtualTags(t *testing.T) {
	var f Filter
	f, _ = f.WithSegment("home")
	f, _ = f.WithSegment("+next")
	f, _ = f.WithSegment(".+DUE")
	w := f.WriteFilter()
	got := w.Render()
	want := []string{"project:home", "+next", "status:pending"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("write filter render = %v, want %v", got, want)
	}
}

func TestChildProjects(t *testing.T) {
	children := ChildProjects("finances", []string{
		"finances", "finances.2026-taxes", "finances.2026-taxes.federal", "home",
	})
	want := []string{"2026-taxes"}
	if !reflect.DeepEqual(children, want) {
		t.Errorf("children = %v, want %v", children, want)
	}
	root := ChildProjects("", []string{"home.clean-garage", "finances.2026-taxes"})
	if !reflect.DeepEqual(root, []string{"home", "finances"}) {
		t.Errorf("root children = %v", root)
	}
}

func TestChildTags(t *testing.T) {
	tags := ChildTags([]string{"next,shopping", "next", "", "urgent"})
	want := []string{"next", "shopping", "urgent"}
	if !reflect.DeepEqual(tags, want) {
		t.Errorf("tags = %v, want %v", tags, want)
	}
}

func TestVirtualTagsFromDiscovery(t *testing.T) {
	vts := VirtualTagsFromDiscovery([]string{"ACTIVE", "COMPLETED", "shopping", "OVERDUE", "", "ACTIVE"})
	want := []string{"ACTIVE", "COMPLETED", "OVERDUE"}
	if !reflect.DeepEqual(vts, want) {
		t.Errorf("virtual tags = %v, want %v", vts, want)
	}
}

func TestVirtualTagGestureMapping(t *testing.T) {
	if s, ok := StatusVirtualTag("COMPLETED"); !ok || s != "status:completed" {
		t.Errorf("COMPLETED = %q, %v", s, ok)
	}
	if s, ok := StatusVirtualTag("DELETED"); !ok || s != "status:deleted" {
		t.Errorf("DELETED = %q, %v", s, ok)
	}
	if _, ok := StatusVirtualTag("DUE"); ok {
		t.Error("DUE must not map to a status gesture")
	}
}
