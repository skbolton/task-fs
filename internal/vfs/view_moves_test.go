package vfs

import (
	"context"
	"errors"
	"strings"
	"syscall"
	"testing"

	"task-fs/internal/backend"
	"task-fs/internal/naming"
)

func TestWritableViewMoveAppliesModificationArgs(t *testing.T) {
	fs, runner, db := newSweepFSWithViews(t, backend.RemoveDelete,
		[]testView{{Name: "This Month", Filter: "status:pending", Modify: "sched:2026-10-26"}},
		task(uuidA, "Chore", "home"),
	)
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	name := naming.FileName("Chore", uuidA)
	vn := resolveViewNode(t, fs, "@This Month")

	if errno := home.(*dirNode).Rename(context.Background(), name, vn, name, 0); errno != 0 {
		t.Fatalf("move into writable view: %v", errno)
	}
	got := lastModifyArgs(runner)
	if len(got) != 1 || got[0] != "sched:2026-10-26" {
		t.Errorf("modify args = %v, want [sched:2026-10-26]", got)
	}
	if db.tasks[0].Project != "home" || len(db.tasks[0].Tags) != 0 {
		t.Errorf("move changed project or tags: %+v", db.tasks[0])
	}
	if n := mutationCount(runner); n != 1 {
		t.Errorf("move issued %d mutations, want 1", n)
	}
}

func TestWritableViewMoveUsesShellStyleArguments(t *testing.T) {
	fs, runner, db := newSweepFSWithViews(t, backend.RemoveDelete,
		[]testView{{Name: "Family", Filter: "status:pending", Modify: `description:"call mom" +family`}},
		task(uuidA, "Chore", "home"),
	)
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	name := naming.FileName("Chore", uuidA)
	vn := resolveViewNode(t, fs, "@Family")

	if errno := home.(*dirNode).Rename(context.Background(), name, vn, name, 0); errno != 0 {
		t.Fatalf("move into writable view: %v", errno)
	}
	got := lastModifyArgs(runner)
	want := []string{"description:call mom", "+family"}
	if len(got) != len(want) {
		t.Fatalf("modify args = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("modify args[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if db.tasks[0].Description != "call mom" || !contains(db.tasks[0].Tags, "family") {
		t.Errorf("task after move = %+v, want updated description and tag", db.tasks[0])
	}
}

func TestViewBlankModificationLineRemainsReadOnly(t *testing.T) {
	fs, runner, _ := newSweepFSWithDefinitions(t, backend.RemoveDelete,
		map[string]string{"Due": "status:pending\n"}, nil,
		task(uuidA, "Chore", "home"),
	)
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	name := naming.FileName("Chore", uuidA)
	vn := resolveViewNode(t, fs, "@Due")

	if errno := home.(*dirNode).Rename(context.Background(), name, vn, name, 0); errno != syscall.EACCES {
		t.Errorf("move into blank modification view: errno = %v, want EACCES", errno)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("read-only move issued %d mutations", n)
	}
}

func TestViewExtraDefinitionLineIsUnreadable(t *testing.T) {
	fs, runner, _ := newSweepFSWithDefinitions(t, backend.RemoveDelete,
		map[string]string{"Due": "status:pending\n+next\nextra"}, nil,
		task(uuidA, "Chore", "home"),
	)
	vn := resolveViewNode(t, fs, "@Due")
	if _, errno := vn.Readdir(context.Background()); errno != syscall.EIO {
		t.Errorf("Readdir: errno = %v, want EIO", errno)
	}
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	name := naming.FileName("Chore", uuidA)
	if errno := home.(*dirNode).Rename(context.Background(), name, vn, name, 0); errno != syscall.EIO {
		t.Errorf("move into invalid definition: errno = %v, want EIO", errno)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("invalid definition move issued %d mutations", n)
	}
}

func TestViewListingUsesOnlyFilterLine(t *testing.T) {
	fs, _, db := newSweepFSWithViews(t, backend.RemoveDelete,
		[]testView{{Name: "Next", Filter: "status:pending +next", Modify: "-next"}},
		task(uuidA, "Chore", "home", "next"),
	)
	vn := resolveViewNode(t, fs, "@Next")
	name := naming.FileName("Chore", uuidA)

	if names := readdirViewOf(t, vn); !contains(names, name) {
		t.Fatalf("listing before move = %v, want %q", names, name)
	}

	qn := resolveViewFileNode(t, vn)
	writeViewFile(t, qn, "status:pending +next\n-next")
	if names := readdirViewOf(t, vn); !contains(names, name) {
		t.Errorf("listing after editing only the modification line = %v, want %q", names, name)
	}

	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	if errno := home.(*dirNode).Rename(context.Background(), name, vn, name, 0); errno != 0 {
		t.Fatalf("move into writable view: %v", errno)
	}
	if contains(db.tasks[0].Tags, "next") {
		t.Error("move did not apply the modification line")
	}
	if names := readdirViewOf(t, vn); contains(names, name) {
		t.Errorf("task no longer matching the filter is still listed: %v", names)
	}
}

func TestWritableViewMoveFromCompletedKeepsStatus(t *testing.T) {
	fs, runner, db := newSweepFSWithViews(t, backend.RemoveDelete,
		[]testView{{Name: "Archive", Filter: "status:any:", Modify: "+archived"}},
		backend.Task{UUID: uuidA, Description: "Finished", Project: "home", Status: "completed"},
	)
	completed, errno := fs.resolveChild("+COMPLETED")
	if errno != 0 {
		t.Fatalf("lookup +COMPLETED: %v", errno)
	}
	name := naming.FileName("Finished", uuidA)
	vn := resolveViewNode(t, fs, "@Archive")

	if errno := completed.(*dirNode).Rename(context.Background(), name, vn, name, 0); errno != 0 {
		t.Fatalf("move from +COMPLETED into writable view: %v", errno)
	}
	if got := lastModifyArgs(runner); len(got) != 1 || got[0] != "+archived" {
		t.Errorf("modify args = %v, want [+archived]", got)
	}
	if db.tasks[0].Status != "completed" {
		t.Errorf("status after move = %q, want completed", db.tasks[0].Status)
	}
	if !contains(db.tasks[0].Tags, "archived") {
		t.Errorf("tags after move = %v, want archived", db.tasks[0].Tags)
	}
}

func TestWritableViewMoveRendersFreshModificationArgs(t *testing.T) {
	week := "2026-10-26"
	calls := 0
	render := func(_ context.Context, raw string) (string, error) {
		calls++
		if strings.Contains(raw, "{{week}}") {
			return "sched:" + week, nil
		}
		return raw, nil
	}
	fs, runner, _ := newSweepFSWithDefinitions(t, backend.RemoveDelete,
		map[string]string{"Due": "status:pending\nsched:{{week}}"}, render,
		task(uuidA, "Chore", "home"),
	)
	vn := resolveViewNode(t, fs, "@Due")
	readdirViewOf(t, vn)
	afterListing := calls

	week = "2026-11-02"
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	name := naming.FileName("Chore", uuidA)
	if errno := home.(*dirNode).Rename(context.Background(), name, vn, name, 0); errno != 0 {
		t.Fatalf("move into writable view: %v", errno)
	}
	if got := lastModifyArgs(runner); len(got) != 1 || got[0] != "sched:2026-11-02" {
		t.Errorf("modify args = %v, want [sched:2026-11-02]", got)
	}
	if calls <= afterListing {
		t.Errorf("move reused cached render: calls %d before, %d after", afterListing, calls)
	}
}

func TestWritableViewMoveFailuresChangeNothing(t *testing.T) {
	t.Run("render failure", func(t *testing.T) {
		fs, runner, db := newSweepFSWithDefinitions(t, backend.RemoveDelete,
			map[string]string{"Due": "status:pending\n{{boom}}"},
			func(_ context.Context, raw string) (string, error) {
				if strings.Contains(raw, "boom") {
					return "", errors.New("boom")
				}
				return raw, nil
			},
			task(uuidA, "Chore", "home"),
		)
		moveHomeTaskToView(t, fs, "@Due", "Chore", uuidA, syscall.EIO)
		if n := mutationCount(runner); n != 0 {
			t.Errorf("render failure issued %d mutations", n)
		}
		if len(db.tasks[0].Tags) != 0 {
			t.Errorf("render failure changed tags: %v", db.tasks[0].Tags)
		}
	})

	t.Run("unterminated quote", func(t *testing.T) {
		fs, runner, _ := newSweepFSWithDefinitions(t, backend.RemoveDelete,
			map[string]string{"Due": "status:pending\nsched:\"oops"}, nil,
			task(uuidA, "Chore", "home"),
		)
		moveHomeTaskToView(t, fs, "@Due", "Chore", uuidA, syscall.EIO)
		if n := mutationCount(runner); n != 0 {
			t.Errorf("unterminated quote issued %d mutations", n)
		}
	})

	t.Run("taskwarrior rejection", func(t *testing.T) {
		fs, runner, db := newSweepFSWithViews(t, backend.RemoveDelete,
			[]testView{{Name: "Due", Filter: "status:pending", Modify: "+archived"}},
			task(uuidA, "Chore", "home"),
		)
		runner.fn = func(args []string) ([]byte, error) {
			if hasArg(args, "modify") {
				return nil, errors.New("rejected")
			}
			return db.serve(args)
		}
		moveHomeTaskToView(t, fs, "@Due", "Chore", uuidA, syscall.EIO)
		if contains(db.tasks[0].Tags, "archived") {
			t.Errorf("rejected modification changed tags: %v", db.tasks[0].Tags)
		}
	})
}

func TestWritableViewMoveRejectsRenameAndMovesBetweenViews(t *testing.T) {
	fs, runner, _ := newSweepFSWithViews(t, backend.RemoveDelete,
		[]testView{
			{Name: "First", Filter: "status:pending", Modify: "+first"},
			{Name: "Second", Filter: "status:pending", Modify: "+second"},
		},
		task(uuidA, "Chore", "home"),
	)
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	name := naming.FileName("Chore", uuidA)
	first := resolveViewNode(t, fs, "@First")
	second := resolveViewNode(t, fs, "@Second")

	if errno := home.(*dirNode).Rename(context.Background(), name, first, naming.FileName("Renamed", uuidA), 0); errno != syscall.EACCES {
		t.Errorf("rename into writable view: errno = %v, want EACCES", errno)
	}
	if errno := first.Rename(context.Background(), name, second, name, 0); errno != syscall.EACCES {
		t.Errorf("move between views: errno = %v, want EACCES", errno)
	}
	if errno := first.Rename(context.Background(), name, home.(*dirNode), name, 0); errno != syscall.EACCES {
		t.Errorf("move out of view: errno = %v, want EACCES", errno)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("rejected moves issued %d mutations", n)
	}
}

func TestWritableViewRootRenameReachesView(t *testing.T) {
	fs, runner, _ := newSweepFSWithViews(t, backend.RemoveDelete,
		[]testView{{Name: "Due", Filter: "status:pending", Modify: "+archived"}},
		task(uuidA, "Chore", "home"),
	)
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	name := naming.FileName("Chore", uuidA)

	if errno := home.(*dirNode).Rename(context.Background(), name, fs, "@Due", 0); errno != 0 {
		t.Fatalf("root rename onto writable view: %v", errno)
	}
	if got := lastModifyArgs(runner); len(got) != 1 || got[0] != "+archived" {
		t.Errorf("modify args = %v, want [+archived]", got)
	}
}

func TestViewDefinitionLifecycleSupportsMovesWithoutRemount(t *testing.T) {
	fs, runner, db := newSweepFSWithViews(t, backend.RemoveDelete,
		[]testView{{Name: "Due"}},
		task(uuidA, "First", "home"),
		task(uuidB, "Second", "home"),
	)
	vn := resolveViewNode(t, fs, "@Due")
	qn, handle := createViewFileNode(t, vn, syscall.O_WRONLY|syscall.O_TRUNC)
	if _, errno := qn.Write(context.Background(), handle, []byte("status:pending\n+archived"), 0); errno != 0 {
		t.Fatalf("Write definition: %v", errno)
	}
	if errno := qn.Flush(context.Background(), handle); errno != 0 {
		t.Fatalf("Flush definition: %v", errno)
	}

	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	firstName := naming.FileName("First", uuidA)
	if errno := home.(*dirNode).Rename(context.Background(), firstName, vn, firstName, 0); errno != 0 {
		t.Fatalf("move after writing definition: %v", errno)
	}
	if !contains(db.tasks[0].Tags, "archived") {
		t.Errorf("written modification line was not applied: %v", db.tasks[0].Tags)
	}

	writeViewFile(t, qn, "status:pending")
	secondName := naming.FileName("Second", uuidB)
	if errno := home.(*dirNode).Rename(context.Background(), secondName, vn, secondName, 0); errno != syscall.EACCES {
		t.Errorf("move after truncating modification line: errno = %v, want EACCES", errno)
	}

	if errno := vn.Unlink(context.Background(), viewFileName); errno != 0 {
		t.Fatalf("unlink _view: %v", errno)
	}
	if errno := home.(*dirNode).Rename(context.Background(), secondName, vn, secondName, 0); errno != syscall.EACCES {
		t.Errorf("move after removing _view: errno = %v, want EACCES", errno)
	}
	if names := readdirViewOf(t, vn); len(names) != 0 {
		t.Errorf("view after removing _view = %v, want empty", names)
	}
	if n := mutationCount(runner); n != 1 {
		t.Errorf("definition lifecycle issued %d mutations, want only the first move", n)
	}
}

func moveHomeTaskToView(t *testing.T, fs *FS, viewEntry, description, uuid string, want syscall.Errno) {
	t.Helper()
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	name := naming.FileName(description, uuid)
	vn := resolveViewNode(t, fs, viewEntry)
	if errno := home.(*dirNode).Rename(context.Background(), name, vn, name, 0); errno != want {
		t.Errorf("move into %s: errno = %v, want %v", viewEntry, errno, want)
	}
}
