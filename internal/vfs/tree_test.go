package vfs

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"task-fs/internal/backend"
	"task-fs/internal/naming"
	"task-fs/internal/view"
)

type fakeRunner struct {
	log [][]string
	fn  func(args []string) ([]byte, error)
}

func (r *fakeRunner) Run(args []string) ([]byte, error) {
	r.log = append(r.log, args)
	return r.fn(args)
}

func taskJSON(tasks ...backend.Task) []byte {
	data, err := json.Marshal(tasks)
	if err != nil {
		panic(err)
	}
	return data
}

func task(uuid, description, project string, tags ...string) backend.Task {
	return backend.Task{
		UUID:        uuid,
		Description: description,
		Project:     project,
		Tags:        tags,
		Status:      "pending",
	}
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

// newTestFS wires a filesystem against a stubbed command runner that
// serves canned results depending on the rendered filter arguments.
func newTestFS(pending, completed []backend.Task, projects, tags []string, cfgs ...Config) (*FS, *fakeRunner) {
	runner := &fakeRunner{fn: func(args []string) ([]byte, error) {
		if hasArg(args, "_unique") {
			if hasArg(args, "project") {
				return []byte(strings.Join(projects, "\n")), nil
			}
			return []byte(strings.Join(tags, "\n")), nil
		}
		if hasArg(args, "export") {
			if hasArg(args, "status:completed") {
				return taskJSON(completed...), nil
			}
			return taskJSON(pending...), nil
		}
		return nil, nil
	}}
	cfg := Config{RemoveAction: backend.RemoveDelete}
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	fs := New(backend.NewQuery(runner), cfg)
	return fs, runner
}

var (
	uuidA = "a1b2c3d4-0000-0000-0000-000000000000"
	uuidB = "e5f6a7b8-1111-1111-1111-111111111111"
)

func readdirNames(t *testing.T, stream fs.DirStream) []string {
	t.Helper()
	var names []string
	for stream.HasNext() {
		e, errno := stream.Next()
		if errno != 0 {
			break
		}
		names = append(names, e.Name)
	}
	stream.Close()
	return names
}

func TestRootReaddirListsVocabulary(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{
			task(uuidA, "Clean the garage", "home.clean-garage", "next"),
			task(uuidB, "File taxes", "finances.2026-taxes"),
		}, nil, []string{"home.clean-garage", "finances.2026-taxes"}, []string{"next"})
	stream, errno := fs.Readdir(context.Background())
	if errno != 0 {
		t.Fatalf("Readdir: %v", errno)
	}
	names := readdirNames(t, stream)
	for _, want := range []string{"home", "finances", "+next", "+COMPLETED", "+DELETED"} {
		if !contains(names, want) {
			t.Errorf("root listing missing %q; got %v", want, names)
		}
	}
	for _, absent := range []string{".+COMPLETED", ".+DELETED", "+DUE"} {
		if contains(names, absent) {
			t.Errorf("root listing must not contain %q; got %v", absent, names)
		}
	}
	homeNode, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	cleanNode, errno := homeNode.(*dirNode).resolveChild("clean-garage")
	if errno != 0 {
		t.Fatalf("lookup clean-garage: %v", errno)
	}
	homeNames, errno := readdirOf(t, cleanNode.(*dirNode))
	if errno != 0 {
		t.Fatalf("Readdir clean-garage: %v", errno)
	}
	if !contains(homeNames, naming.FileName("Clean the garage", uuidA)) {
		t.Errorf("/home/clean-garage listing missing its task: %v", homeNames)
	}
}

func readdirOf(t *testing.T, n *dirNode) ([]string, syscall.Errno) {
	t.Helper()
	stream, errno := n.Readdir(context.Background())
	if errno != 0 {
		return nil, errno
	}
	return readdirNames(t, stream), 0
}

func TestStatusVirtualTagsResolveAndList(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{task(uuidA, "Chore", "home")},
		[]backend.Task{task(uuidA, "Old chore", "home")},
		[]string{"home"}, []string{})
	for _, name := range []string{"+COMPLETED", "+DELETED"} {
		if _, errno := fs.resolveChild(name); errno != 0 {
			t.Errorf("lookup %s: %v", name, errno)
		}
		if _, errno := fs.resolveChild("." + name); errno != syscall.ENOENT {
			t.Errorf("lookup .%s: errno = %v, want ENOENT", name, errno)
		}
	}
	names, errno := readdirOf(t, &fs.dirNode)
	if errno != 0 {
		t.Fatalf("Readdir root: %v", errno)
	}
	for _, want := range []string{"+COMPLETED", "+DELETED"} {
		if !contains(names, want) {
			t.Errorf("root listing missing %q: %v", want, names)
		}
	}
	for _, absent := range []string{".+COMPLETED", ".+DELETED"} {
		if contains(names, absent) {
			t.Errorf("root listing must not contain %q: %v", absent, names)
		}
	}
}

func TestNonStatusVirtualSegmentsAreNotVirtual(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{task(uuidA, "Chore", "home")}, nil, []string{"home"}, []string{})
	for _, name := range []string{"+DUE", ".+DUE", ".+COMPLETED"} {
		if _, errno := fs.resolveChild(name); errno != syscall.ENOENT {
			t.Errorf("lookup %s: errno = %v, want ENOENT", name, errno)
		}
	}

	fs, _ = newTestFS(
		[]backend.Task{task(uuidA, "Chore", "home", "DUE")}, nil, []string{"home"}, []string{"DUE"})
	child, errno := fs.resolveChild("+DUE")
	if errno != 0 {
		t.Fatalf("lookup tag +DUE: %v", errno)
	}
	f := child.(*dirNode).filter
	if !contains(f.Tags, "DUE") || len(f.VirtualTags) != 0 {
		t.Errorf("+DUE must resolve as a tag filter: %+v", f)
	}
}

func TestStatusVirtualTagShadowsUppercaseTag(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{task(uuidA, "Chore", "home", "COMPLETED")}, nil, []string{"home"}, []string{"COMPLETED"})
	child, errno := fs.resolveChild("+COMPLETED")
	if errno != 0 {
		t.Fatalf("lookup +COMPLETED: %v", errno)
	}
	f := child.(*dirNode).filter
	if !contains(f.VirtualTags, "COMPLETED") || contains(f.Tags, "COMPLETED") {
		t.Errorf("status virtual tag must win over a same-named tag: %+v", f)
	}
}

func TestLookupStrictENOENT(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{task(uuidA, "Clean the garage", "home.clean-garage", "next")}, nil, []string{"home.clean-garage"}, []string{"next"})
	if _, errno := fs.resolveChild("homi"); errno != syscall.ENOENT {
		t.Errorf("lookup typo'd segment: errno = %v, want ENOENT", errno)
	}
	if _, errno := fs.resolveChild("+DUE"); errno != syscall.ENOENT {
		t.Errorf("unselected virtual-tag spelling +DUE must not resolve: errno = %v", errno)
	}
	if _, errno := fs.resolveChild(".hidden"); errno != syscall.ENOENT {
		t.Errorf("dot-prefixed segment must not resolve: errno = %v", errno)
	}
	child, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	node := child.(*dirNode)
	if node.filter.Project != "home" {
		t.Errorf("filter project = %q", node.filter.Project)
	}
	grandchild, errno := node.resolveChild("clean-garage")
	if errno != 0 {
		t.Fatalf("lookup clean-garage: %v", errno)
	}
	if gc := grandchild.(*dirNode); gc.filter.Project != "home.clean-garage" {
		t.Errorf("subproject filter = %q", gc.filter.Project)
	}
}

func TestCompletedOnlyUnderCompletedSubtree(t *testing.T) {
	completed := backend.Task{
		UUID:        uuidA,
		Description: "Old chore",
		Project:     "home",
		Status:      "completed",
	}
	fs, _ := newTestFS(
		[]backend.Task{task(uuidB, "Chore", "home")}, []backend.Task{completed}, []string{"home"}, []string{})
	completedDir, errno := fs.resolveChild("+COMPLETED")
	if errno != 0 {
		t.Fatalf("lookup +COMPLETED: %v", errno)
	}
	completedNode := completedDir.(*dirNode)

	rootStream, errno := fs.Readdir(context.Background())
	if errno != 0 {
		t.Fatalf("Readdir root: %v", errno)
	}
	if contains(readdirNames(t, rootStream), naming.FileName("Old chore", uuidA)) {
		t.Error("completed task leaked into pending root listing")
	}

	stream, errno := completedNode.Readdir(context.Background())
	if errno != 0 {
		t.Fatalf("Readdir +COMPLETED: %v", errno)
	}
	if !contains(readdirNames(t, stream), naming.FileName("Old chore", uuidA)) {
		t.Error("completed task missing from +COMPLETED listing")
	}
}

func TestLookupResolvesTaskFileAndXattr(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{task(uuidA, "Do taxes", "finances")}, nil, []string{"finances"}, []string{})
	name := naming.FileName("Do taxes", uuidA)
	finances, errno := fs.resolveChild("finances")
	if errno != 0 {
		t.Fatalf("lookup finances: %v", errno)
	}
	child, errno := finances.(*dirNode).resolveChild(name)
	if errno != 0 {
		t.Fatalf("lookup task file: %v", errno)
	}
	fn := child.(*fileNode)
	buf := make([]byte, 64)
	res, errno := fn.Read(context.Background(), nil, buf, 0)
	if errno != 0 || res.Size() != 0 {
		t.Errorf("read = %d bytes, errno %v; want 0 bytes", res.Size(), errno)
	}
	var attr fuse.AttrOut
	if errno := fn.Getattr(context.Background(), nil, &attr); errno != 0 {
		t.Fatalf("Getattr: %v", errno)
	}
	if attr.Size != 0 {
		t.Errorf("size = %d, want 0", attr.Size)
	}
	got := make([]byte, len(uuidA))
	n, errno := fn.Getxattr(context.Background(), uuidXattr, got)
	if errno != 0 || n != uint32(len(uuidA)) || string(got) != uuidA {
		t.Errorf("Getxattr = %q, %v; want %q", string(got), errno, uuidA)
	}
	dest := make([]byte, 128)
	sz, errno := fn.Listxattr(context.Background(), dest)
	if errno != 0 || !strings.HasPrefix(string(dest), uuidXattr+"\x00") {
		t.Errorf("Listxattr = %q, %d, errno %v", string(dest), sz, errno)
	}
}

func TestCreateGestures(t *testing.T) {
	fs, runner := newTestFS(
		[]backend.Task{task(uuidB, "Old", "home")}, nil, []string{"home"}, []string{})
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	homeNode := home.(*dirNode)

	// After the gesture, the stub reports the new task as existing.
	runner.fn = func(args []string) ([]byte, error) {
		if hasArg(args, "add") {
			return nil, nil
		}
		if hasArg(args, "export") {
			return taskJSON(backend.Task{UUID: uuidA, Description: "Pick up Milk", Project: "home", Status: "pending"}), nil
		}
		return nil, nil
	}
	childNode, errno := homeNode.createChild("Pick up Milk")
	if errno != 0 || childNode == nil {
		t.Fatalf("create: errno %v", errno)
	}
	var addCmd []string
	for _, cmd := range runner.log {
		if hasArg(cmd, "add") {
			addCmd = cmd
		}
	}
	if !hasArg(addCmd, "Pick up Milk") || !hasArg(addCmd, "project:home") {
		t.Errorf("add command = %v", addCmd)
	}
	if _, errno := homeNode.createChild("X.a1b2c3d4.md"); errno != syscall.EPERM {
		t.Errorf("create minted name: errno = %v, want EPERM", errno)
	}
}

func TestCreateDropsVirtualTags(t *testing.T) {
	fs, runner := newTestFS(nil, nil, []string{"home"}, []string{})
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	completedNode, errno := home.(*dirNode).resolveChild("+COMPLETED")
	if errno != 0 {
		t.Fatalf("lookup +COMPLETED: %v", errno)
	}
	completedNode.(*dirNode).createChild("Pick up Milk")
	var addCmd []string
	for _, cmd := range runner.log {
		if hasArg(cmd, "add") {
			addCmd = cmd
		}
	}
	if addCmd == nil {
		t.Fatal("no add command issued")
	}
	if hasArg(addCmd, "+COMPLETED") || hasArg(addCmd, "status:completed") {
		t.Errorf("add command carries a virtual tag: %v", addCmd)
	}
	if !hasArg(addCmd, "project:home") {
		t.Errorf("add command missing write filter: %v", addCmd)
	}
}

func TestWriteRejection(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{task(uuidA, "Do taxes", "finances")}, nil, []string{"finances"}, []string{})
	name := naming.FileName("Do taxes", uuidA)
	finances, errno := fs.resolveChild("finances")
	if errno != 0 {
		t.Fatalf("lookup finances: %v", errno)
	}
	child, errno := finances.(*dirNode).resolveChild(name)
	if errno != 0 {
		t.Fatalf("lookup: %v", errno)
	}
	fn := child.(*fileNode)
	if _, _, errno := fn.Open(context.Background(), syscall.O_WRONLY); errno != syscall.EACCES {
		t.Errorf("open for write: errno = %v, want EACCES", errno)
	}
	if _, _, errno := fn.Open(context.Background(), syscall.O_RDONLY); errno != 0 {
		t.Errorf("open for read: %v", errno)
	}
}

func TestUnlinkDeletesByFullUUID(t *testing.T) {
	fs, runner := newTestFS(
		[]backend.Task{task(uuidA, "Do taxes", "")}, nil, []string{"finances"}, []string{})
	name := naming.FileName("Do taxes", uuidA)
	if errno := fs.Unlink(context.Background(), name); errno != 0 {
		t.Fatalf("unlink: %v", errno)
	}
	delCmd := runner.log[len(runner.log)-1]
	if len(delCmd) != 3 || !hasArg(delCmd, uuidA) || !hasArg(delCmd, "delete") {
		t.Errorf("delete command = %v", delCmd)
	}
}

func TestUnlinkCompleteCompletesTask(t *testing.T) {
	fs, runner, db := newSweepFSWithAction(t, backend.RemoveComplete,
		backend.Task{UUID: uuidA, Description: "Do taxes", Status: "pending"},
	)
	name := naming.FileName("Do taxes", uuidA)
	if errno := fs.Unlink(context.Background(), name); errno != 0 {
		t.Fatalf("unlink: %v", errno)
	}
	if !db.completed[uuidA] {
		t.Error("task was not completed")
	}
	doneCmd := runner.log[len(runner.log)-1]
	if len(doneCmd) != 3 || !hasArg(doneCmd, uuidA) || !hasArg(doneCmd, "done") {
		t.Errorf("done command = %v", doneCmd)
	}
	if hasArg(doneCmd, "delete") {
		t.Errorf("done removal issued delete: %v", doneCmd)
	}
}

func TestUnlinkCompletedTaskIsNoopUnderComplete(t *testing.T) {
	fs, runner, _ := newSweepFSWithAction(t, backend.RemoveComplete,
		backend.Task{UUID: uuidA, Description: "Finished", Project: "home", Status: "completed"},
	)
	completed, errno := fs.resolveChild("+COMPLETED")
	if errno != 0 {
		t.Fatalf("lookup +COMPLETED: %v", errno)
	}
	name := naming.FileName("Finished", uuidA)
	if errno := completed.(*dirNode).Unlink(context.Background(), name); errno != 0 {
		t.Fatalf("unlink completed task: %v", errno)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("completed-task unlink issued %d mutations", n)
	}
}

func TestRenameIntoCompleted(t *testing.T) {
	fs, runner := newTestFS(
		[]backend.Task{task(uuidA, "Do taxes", "")}, nil, []string{"finances"}, []string{})
	name := naming.FileName("Do taxes", uuidA)
	completed, errno := fs.resolveChild("+COMPLETED")
	if errno != 0 {
		t.Fatalf("lookup +COMPLETED: %v", errno)
	}
	completedNode := completed.(*dirNode)
	if errno := fs.Rename(context.Background(), name, completedNode, name, 0); errno != 0 {
		t.Fatalf("rename into +COMPLETED: %v", errno)
	}
	doneCmd := runner.log[len(runner.log)-1]
	if len(doneCmd) != 3 || !hasArg(doneCmd, uuidA) || !hasArg(doneCmd, "done") {
		t.Errorf("done command = %v", doneCmd)
	}
}

func TestRenameIntoDeletedDeletes(t *testing.T) {
	fs, runner := newTestFS(
		[]backend.Task{task(uuidA, "Do taxes", "")}, nil, nil, nil)
	name := naming.FileName("Do taxes", uuidA)
	deleted, errno := fs.resolveChild("+DELETED")
	if errno != 0 {
		t.Fatalf("lookup +DELETED: %v", errno)
	}
	if errno := fs.Rename(context.Background(), name, deleted.(*dirNode), name, 0); errno != 0 {
		t.Fatalf("rename into +DELETED: %v", errno)
	}
	delCmd := runner.log[len(runner.log)-1]
	if len(delCmd) != 3 || !hasArg(delCmd, uuidA) || !hasArg(delCmd, "delete") || hasArg(delCmd, "done") {
		t.Errorf("delete command = %v", delCmd)
	}
}

func TestRenameOutOfDeletedReopens(t *testing.T) {
	deletedTask := backend.Task{UUID: uuidA, Description: "Do taxes", Project: "finances", Status: "deleted"}
	fs, runner := newTestFS(
		[]backend.Task{deletedTask}, nil, []string{"finances"}, nil)
	name := naming.FileName("Do taxes", uuidA)
	deleted, errno := fs.resolveChild("+DELETED")
	if errno != 0 {
		t.Fatalf("lookup +DELETED: %v", errno)
	}
	finances, errno := fs.resolveChild("finances")
	if errno != 0 {
		t.Fatalf("lookup finances: %v", errno)
	}
	if errno := deleted.(*dirNode).Rename(context.Background(), name, finances.(*dirNode), name, 0); errno != 0 {
		t.Fatalf("rename out of +DELETED: %v", errno)
	}
	var foundStatus, foundProject bool
	for _, cmd := range runner.log {
		if hasArg(cmd, "modify") {
			if hasArg(cmd, "status:pending") {
				foundStatus = true
			}
			if hasArg(cmd, "project:finances") {
				foundProject = true
			}
		}
	}
	if !foundStatus || !foundProject {
		t.Errorf("reopen commands missing: %v", runner.log)
	}
}

func TestRenameContradictoryStatusDestinationRejected(t *testing.T) {
	fs, runner := newTestFS(
		[]backend.Task{task(uuidA, "Chore", "home")}, nil, []string{"home"}, nil)
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	completed, errno := fs.resolveChild("+COMPLETED")
	if errno != 0 {
		t.Fatalf("lookup +COMPLETED: %v", errno)
	}
	both, errno := completed.(*dirNode).resolveChild("+DELETED")
	if errno != 0 {
		t.Fatalf("lookup +COMPLETED/+DELETED: %v", errno)
	}
	name := naming.FileName("Chore", uuidA)
	if errno := home.(*dirNode).Rename(context.Background(), name, both, name, 0); errno != syscall.ENOTSUP {
		t.Errorf("rename into contradictory status: errno = %v, want ENOTSUP", errno)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("rejected rename issued %d mutations", n)
	}
}

func TestRenameOutOfCompletedReopens(t *testing.T) {
	completedTask := backend.Task{UUID: uuidA, Description: "Do taxes", Project: "finances", Status: "completed"}
	fs, runner := newTestFS(
		nil, []backend.Task{completedTask}, []string{"finances"}, []string{})
	name := naming.FileName("Do taxes", uuidA)
	completed, errno := fs.resolveChild("+COMPLETED")
	if errno != 0 {
		t.Fatalf("lookup +COMPLETED: %v", errno)
	}
	finances, errno := fs.resolveChild("finances")
	if errno != 0 {
		t.Fatalf("lookup finances: %v", errno)
	}
	if errno := completed.(*dirNode).Rename(context.Background(), name, finances.(*dirNode), name, 0); errno != 0 {
		t.Fatalf("rename out of +COMPLETED: %v", errno)
	}
	var foundStatus, foundProject bool
	for _, cmd := range runner.log {
		if hasArg(cmd, "modify") {
			if hasArg(cmd, "status:pending") {
				foundStatus = true
			}
			if hasArg(cmd, "project:finances") {
				foundProject = true
			}
		}
	}
	if !foundStatus || !foundProject {
		t.Errorf("reopen commands missing: %v", runner.log)
	}
}

func TestRenameDotStatusDestinationRejected(t *testing.T) {
	fs, runner := newTestFS(
		[]backend.Task{task(uuidA, "Chore", "home")}, nil, []string{"home"}, []string{})
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	name := naming.FileName("Chore", uuidA)
	if errno := home.(*dirNode).Rename(context.Background(), name, home.(*dirNode), ".+COMPLETED", 0); errno != syscall.EINVAL {
		t.Errorf("rename to dot status destination: errno = %v, want EINVAL", errno)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("rejected rename issued %d mutations", n)
	}
}

func TestRenameDescriptionRejected(t *testing.T) {
	fs, runner := newTestFS(
		[]backend.Task{task(uuidA, "Old", "home")}, nil, []string{"home"}, []string{})
	if errno := fs.Rename(context.Background(),
		naming.FileName("Old", uuidA), fs, naming.FileName("New", uuidA), 0); errno != syscall.EOPNOTSUPP {
		t.Errorf("rename: errno = %v, want EOPNOTSUPP", errno)
	}
	if len(runner.log) != 0 {
		t.Errorf("rejected rename issued commands: %v", runner.log)
	}
}

func TestRenameDeltaMoves(t *testing.T) {
	tests := []struct {
		name        string
		srcFilter   []string
		taskProject string
		dstFilter   []string
		dstName     string
		want        []string
	}{
		{
			name:        "inbox to project",
			srcFilter:   []string{"inbox"},
			taskProject: "inbox",
			dstFilter:   []string{"finances", "2026-taxes"},
			want:        []string{"project:finances.2026-taxes"},
		},
		{
			name:        "tag delta both directions",
			srcFilter:   []string{"home", "+next"},
			taskProject: "home",
			dstFilter:   []string{"finances", "+urgent"},
			want:        []string{"project:finances", "+urgent", "-next"},
		},
		{
			name:        "parent project exactness",
			srcFilter:   []string{"finances", "2026-taxes"},
			taskProject: "finances.2026-taxes",
			dstFilter:   []string{"finances"},
			want:        []string{"project:finances"},
		},
		{
			name:        "minted destination",
			srcFilter:   []string{"finances"},
			taskProject: "finances",
			dstFilter:   []string{"finances"},
			dstName:     "2026-taxes",
			want:        []string{"project:finances.2026-taxes"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := backend.Filter{}
			for _, seg := range tt.srcFilter {
				src, _ = src.WithSegment(seg)
			}
			dst := backend.Filter{}
			for _, seg := range tt.dstFilter {
				dst, _ = dst.WithSegment(seg)
			}
			fs, runner := newTestFS(
				[]backend.Task{task(uuidA, "Fix leak", tt.taskProject)}, nil, []string{"inbox", "finances", "finances.2026-taxes"}, []string{"next", "urgent"})
			newName := naming.FileName("Fix leak", uuidA)
			if tt.dstName != "" {
				newName = tt.dstName
			}
			errno := (&dirNode{root: fs, filter: src}).Rename(
				context.Background(), naming.FileName("Fix leak", uuidA), &dirNode{root: fs, filter: dst}, newName, 0)
			if errno != 0 {
				t.Fatalf("rename: %v", errno)
			}
			var got []string
			for _, cmd := range runner.log {
				if hasArg(cmd, "modify") {
					for i, a := range cmd {
						if a == "modify" {
							got = cmd[i+1:]
						}
					}
				}
			}
			if len(got) != len(tt.want) {
				t.Errorf("modify args = %v, want %v", got, tt.want)
			}
			for _, w := range tt.want {
				if !hasArg(got, w) {
					t.Errorf("modify args %v missing %q", got, w)
				}
			}
		})
	}
}

func TestRenameUnresolvableParentChain(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{task(uuidA, "Fix leak", "inbox")}, nil, []string{"inbox"}, []string{})
	if _, errno := fs.resolveChild("brandnew"); errno != syscall.ENOENT {
		t.Errorf("lookup of undeclared project: errno = %v, want ENOENT", errno)
	}
}

func TestMkdirDeclarations(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{task(uuidA, "Chore", "home", "next")}, nil, []string{"home"}, []string{"next"})
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	homeNode := home.(*dirNode)

	inode, errno := homeNode.declareChild("+urgent")
	if errno != 0 || inode == nil {
		t.Fatalf("mkdir +urgent: %v", errno)
	}
	if _, errno := homeNode.declareChild("+next"); errno != syscall.EEXIST {
		t.Errorf("redeclare +next: errno = %v, want EEXIST", errno)
	}
	if _, errno := homeNode.declareChild("+urgent"); errno != syscall.EEXIST {
		t.Errorf("redeclare +urgent: errno = %v, want EEXIST", errno)
	}
	if _, errno := homeNode.declareChild("+DUE"); errno != syscall.EACCES {
		t.Errorf("mkdir +DUE: errno = %v, want EACCES", errno)
	}
	if _, errno := homeNode.declareChild("+COMPLETED"); errno != syscall.EACCES {
		t.Errorf("mkdir +COMPLETED: errno = %v, want EACCES", errno)
	}
	if _, errno := homeNode.declareChild(".+COMPLETED"); errno != syscall.EACCES {
		t.Errorf("mkdir .+COMPLETED: errno = %v, want EACCES", errno)
	}
	if _, errno := homeNode.declareChild(".hidden"); errno != syscall.EACCES {
		t.Errorf("mkdir .hidden: errno = %v, want EACCES", errno)
	}
	if _, errno := fs.declareChild("side-projects"); errno != 0 {
		t.Errorf("mkdir side-projects: %v", errno)
	}

	stream, errno := homeNode.Readdir(context.Background())
	if errno != 0 {
		t.Fatalf("Readdir: %v", errno)
	}
	if !contains(readdirNames(t, stream), "+urgent") {
		t.Error("declared tag missing from listing")
	}
}

func TestRmdirDeclaredInUseDeletesTasks(t *testing.T) {
	fs, runner, db := newSweepFS(t,
		backend.Task{UUID: uuidA, Description: "Chore", Project: "home", Tags: []string{"next"}, Status: "pending"},
	)
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	homeNode := home.(*dirNode)
	if _, errno := homeNode.declareChild("+urgent"); errno != 0 {
		t.Fatalf("mkdir +urgent: %v", errno)
	}
	// The declared tag becomes in use: a task lands under it afterwards.
	db.tasks = append(db.tasks, backend.Task{UUID: uuidB, Description: "Urgent thing", Project: "home", Tags: []string{"urgent"}, Status: "pending"})
	if errno := homeNode.Rmdir(context.Background(), "+urgent"); errno != 0 {
		t.Fatalf("rmdir in-use tag: %v", errno)
	}
	if got := rmdirDeletedUUIDs(runner); len(got) != 1 || got[0] != uuidB {
		t.Errorf("deleted uuids = %v, want [%s]", got, uuidB)
	}
	if db.deleted[uuidA] {
		t.Error("untagged task must survive")
	}
	names, errno := readdirOf(t, homeNode)
	if errno != 0 {
		t.Fatalf("Readdir: %v", errno)
	}
	if contains(names, "+urgent") {
		t.Error("declaration survived rmdir")
	}
}

func TestRmdirUnusedDeclaration(t *testing.T) {
	runner := &fakeRunner{fn: func(args []string) ([]byte, error) {
		if hasArg(args, "export") {
			if hasArg(args, "+urgent") {
				return taskJSON(), nil
			}
			return taskJSON(task(uuidA, "Chore", "home", "next")), nil
		}
		if hasArg(args, "_unique") {
			if hasArg(args, "project") {
				return []byte("home"), nil
			}
			return []byte("next"), nil
		}
		return nil, nil
	}}
	fs := New(backend.NewQuery(runner), Config{RemoveAction: backend.RemoveDelete})
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	homeNode := home.(*dirNode)
	if _, errno := homeNode.declareChild("+urgent"); errno != 0 {
		t.Fatalf("mkdir +urgent: %v", errno)
	}
	if errno := homeNode.Rmdir(context.Background(), "+urgent"); errno != 0 {
		t.Errorf("rmdir unused: %v", errno)
	}
	stream, errno := homeNode.Readdir(context.Background())
	if errno != 0 {
		t.Fatalf("Readdir: %v", errno)
	}
	if contains(readdirNames(t, stream), "+urgent") {
		t.Error("removed declaration still listed")
	}
}

func TestMutationArgumentsUseFullUUID(t *testing.T) {
	fs, runner := newTestFS(
		[]backend.Task{task(uuidA, "Do taxes", "")}, nil, []string{"finances"}, []string{})
	name := naming.FileName("Do taxes", uuidA)
	if errno := fs.Unlink(context.Background(), name); errno != 0 {
		t.Fatalf("unlink: %v", errno)
	}
	for _, cmd := range runner.log {
		isMutation := hasArg(cmd, "delete") || hasArg(cmd, "modify") || hasArg(cmd, "done") || hasArg(cmd, "add")
		if !isMutation {
			continue
		}
		found := false
		for _, a := range cmd {
			if a == uuidA {
				found = true
			}
		}
		if !found {
			t.Errorf("mutation command %v does not reference the full uuid", cmd)
		}
	}
	for _, cmd := range runner.log {
		isMutation := hasArg(cmd, "delete") || hasArg(cmd, "modify") || hasArg(cmd, "done") || hasArg(cmd, "add")
		if !isMutation {
			continue
		}
		for _, a := range cmd {
			if a == uuidA[:8] || a == "1" || a == "Do taxes" {
				t.Errorf("command %v uses a non-full-uuid identifier %q", cmd, a)
			}
		}
	}
}

func TestRenameWithVirtualTagsInSourceIgnoredInDelta(t *testing.T) {
	completedTask := backend.Task{UUID: uuidA, Description: "Chore", Project: "home", Status: "completed"}
	fs, runner := newTestFS(nil, []backend.Task{completedTask}, []string{"home"}, []string{})
	completed, errno := fs.resolveChild("+COMPLETED")
	if errno != 0 {
		t.Fatalf("lookup +COMPLETED: %v", errno)
	}
	name := naming.FileName("Chore", uuidA)
	if errno := completed.(*dirNode).Rename(context.Background(), name, fs, name, 0); errno != 0 {
		t.Fatalf("rename out of +COMPLETED: %v", errno)
	}
	for _, cmd := range runner.log {
		if hasArg(cmd, "modify") && (hasArg(cmd, "+COMPLETED") || hasArg(cmd, "status:completed")) {
			t.Errorf("modify command carries a status virtual tag: %v", cmd)
		}
	}
}

func TestExactProjectPlacement(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{
			task(uuidA, "Less nested project", "this.is"),
			task(uuidB, "Test out deep projects", "this.is.a.test"),
		}, nil, []string{"this.is", "this.is.a.test"}, []string{})
	fileA := naming.FileName("Less nested project", uuidA)
	fileB := naming.FileName("Test out deep projects", uuidB)

	thisNode, errno := fs.resolveChild("this")
	if errno != 0 {
		t.Fatalf("lookup this: %v", errno)
	}
	stream, errno := thisNode.(*dirNode).Readdir(context.Background())
	if errno != 0 {
		t.Fatalf("Readdir /this: %v", errno)
	}
	names := readdirNames(t, stream)
	if !contains(names, "is") {
		t.Errorf("/this missing subproject dir: %v", names)
	}
	if contains(names, fileA) || contains(names, fileB) {
		t.Errorf("subproject tasks leaked into /this: %v", names)
	}

	isNode, errno := thisNode.(*dirNode).resolveChild("is")
	if errno != 0 {
		t.Fatalf("lookup is: %v", errno)
	}
	stream, errno = isNode.(*dirNode).Readdir(context.Background())
	if errno != 0 {
		t.Fatalf("Readdir /this/is: %v", errno)
	}
	names = readdirNames(t, stream)
	if !contains(names, fileA) {
		t.Errorf("/this/is missing its own task: %v", names)
	}
	if contains(names, fileB) {
		t.Errorf("deeper task leaked into /this/is: %v", names)
	}

	// Lookup follows the same rule: the subproject task is not found in
	// the ancestor directory.
	if _, errno := thisNode.(*dirNode).resolveChild(fileA); errno != syscall.ENOENT {
		t.Errorf("ancestor lookup of subproject task: errno = %v, want ENOENT", errno)
	}
	if _, errno := isNode.(*dirNode).resolveChild(fileA); errno != 0 {
		t.Errorf("exact-dir lookup of own task: %v", errno)
	}
}

func TestSelfTagNotListed(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{
			task(uuidA, "Deep task", "home", "next"),
			task(uuidB, "Other task", "work", "next"),
		}, nil, []string{"home", "work"}, []string{"next"})
	nextNode, errno := fs.resolveChild("+next")
	if errno != 0 {
		t.Fatalf("lookup +next: %v", errno)
	}
	names, errno := readdirOf(t, nextNode.(*dirNode))
	if errno != 0 {
		t.Fatalf("Readdir +next: %v", errno)
	}
	if contains(names, "+next") {
		t.Errorf("tag view lists itself: %v", names)
	}
	for _, want := range []string{"home", "work",
		naming.FileName("Deep task", uuidA), naming.FileName("Other task", uuidB)} {
		if !contains(names, want) {
			t.Errorf("tag view missing %q: %v", want, names)
		}
	}
}

func TestTaskFileInodePerView(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{task(uuidA, "Chore", "home", "next")}, nil, []string{"home"}, []string{"next"})
	name := naming.FileName("Chore", uuidA)
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	inHome, errno := home.(*dirNode).resolveChild(name)
	if errno != 0 {
		t.Fatalf("lookup in /home: %v", errno)
	}
	tagged, errno := home.(*dirNode).resolveChild("+next")
	if errno != 0 {
		t.Fatalf("lookup +next: %v", errno)
	}
	inTagged, errno := tagged.(*dirNode).resolveChild(name)
	if errno != 0 {
		t.Fatalf("lookup in /home/+next: %v", errno)
	}
	a := inHome.(*fileNode).stableAttr()
	b := inTagged.(*fileNode).stableAttr()
	if a.Ino == b.Ino {
		t.Error("same task must carry distinct inode numbers in distinct views, or mv sees it as the same file")
	}
	again, errno := home.(*dirNode).resolveChild(name)
	if errno != 0 {
		t.Fatalf("re-lookup: %v", errno)
	}
	if again.(*fileNode).stableAttr().Ino != a.Ino {
		t.Error("inode must be stable within a view")
	}
}

func TestStatusVirtualTagsAlwaysListed(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{task(uuidA, "Chore", "home")}, nil, []string{"home"}, []string{})
	names, errno := readdirOf(t, &fs.dirNode)
	if errno != 0 {
		t.Fatalf("Readdir root: %v", errno)
	}
	for _, want := range []string{"+COMPLETED", "+DELETED"} {
		if !contains(names, want) {
			t.Errorf("root listing missing %q: %v", want, names)
		}
	}
	for _, absent := range []string{".+COMPLETED", ".+DELETED", "+DUE", "+ACTIVE"} {
		if contains(names, absent) {
			t.Errorf("root listing must not contain %q: %v", absent, names)
		}
	}

	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	homeNames, errno := readdirOf(t, home.(*dirNode))
	if errno != 0 {
		t.Fatalf("Readdir home: %v", errno)
	}
	for _, want := range []string{"+COMPLETED", "+DELETED"} {
		if !contains(homeNames, want) {
			t.Errorf("project listing missing %q: %v", want, homeNames)
		}
	}
}

func TestVirtualTagViewDoesNotNest(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{task(uuidA, "Chore", "home")},
		[]backend.Task{task(uuidA, "Old chore", "home")},
		[]string{"home"}, []string{})
	completed, errno := fs.resolveChild("+COMPLETED")
	if errno != 0 {
		t.Fatalf("lookup +COMPLETED: %v", errno)
	}
	names, errno := readdirOf(t, completed.(*dirNode))
	if errno != 0 {
		t.Fatalf("Readdir +COMPLETED: %v", errno)
	}
	for _, absent := range []string{"+COMPLETED", "+DELETED", ".+COMPLETED", ".+DELETED"} {
		if contains(names, absent) {
			t.Errorf("virtual-tag view lists %q: %v", absent, names)
		}
	}
}

func TestSelfVirtualTagNotListed(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{task(uuidA, "Old chore", "home")}, []backend.Task{task(uuidA, "Old chore", "home")}, []string{"home"}, []string{})
	completed, errno := fs.resolveChild("+COMPLETED")
	if errno != 0 {
		t.Fatalf("lookup +COMPLETED: %v", errno)
	}
	names, errno := readdirOf(t, completed.(*dirNode))
	if errno != 0 {
		t.Fatalf("Readdir +COMPLETED: %v", errno)
	}
	if contains(names, "+COMPLETED") {
		t.Errorf("virtual-tag view lists itself: %v", names)
	}
	if contains(names, "+DELETED") {
		t.Errorf("virtual-tag views must not nest, or recursive walks explode: %v", names)
	}
}

func TestDeclaredTagNotListedInsideItself(t *testing.T) {
	runner := &fakeRunner{fn: func(args []string) ([]byte, error) {
		if hasArg(args, "export") {
			return taskJSON(), nil
		}
		return nil, nil
	}}
	fs := New(backend.NewQuery(runner), Config{RemoveAction: backend.RemoveDelete})
	if _, errno := fs.declareChild("+next"); errno != 0 {
		t.Fatalf("declare +next: %v", errno)
	}
	nextNode, errno := fs.resolveChild("+next")
	if errno != 0 {
		t.Fatalf("lookup +next: %v", errno)
	}
	names, errno := readdirOf(t, nextNode.(*dirNode))
	if errno != 0 {
		t.Fatalf("Readdir +next: %v", errno)
	}
	if contains(names, "+next") {
		t.Errorf("declared tag view lists itself: %v", names)
	}
}

func TestRootListsOnlyProjectlessTasks(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{
			task(uuidA, "Less nested project", "this.is"),
			task(uuidB, "Inbox item", ""),
		}, nil, []string{"this.is"}, []string{})
	stream, errno := fs.Readdir(context.Background())
	if errno != 0 {
		t.Fatalf("Readdir root: %v", errno)
	}
	names := readdirNames(t, stream)
	if !contains(names, naming.FileName("Inbox item", uuidB)) {
		t.Errorf("root missing project-less task: %v", names)
	}
	if contains(names, naming.FileName("Less nested project", uuidA)) {
		t.Errorf("root leaked project task: %v", names)
	}
}

func TestTagViewListsAllDepths(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{
			task(uuidA, "Deep task", "home.clean-garage", "next"),
			task(uuidB, "Flat task", "", "next"),
		}, nil, []string{"home.clean-garage"}, []string{"next"})
	nextNode, errno := fs.resolveChild("+next")
	if errno != 0 {
		t.Fatalf("lookup +next: %v", errno)
	}
	stream, errno := nextNode.(*dirNode).Readdir(context.Background())
	if errno != 0 {
		t.Fatalf("Readdir +next: %v", errno)
	}
	names := readdirNames(t, stream)
	if !contains(names, naming.FileName("Deep task", uuidA)) {
		t.Errorf("tag view missing deep task: %v", names)
	}
	if !contains(names, naming.FileName("Flat task", uuidB)) {
		t.Errorf("tag view missing project-less task: %v", names)
	}
	if !contains(names, "home") {
		t.Errorf("tag view missing project dir: %v", names)
	}
}

func TestCanonicalKeyOrderFree(t *testing.T) {
	a := backend.Filter{}
	a, _ = a.WithSegment("home")
	a, _ = a.WithSegment("+next")
	b := backend.Filter{}
	b, _ = b.WithSegment("+next")
	b, _ = b.WithSegment("home")
	if canonicalKey(a) != canonicalKey(b) {
		t.Error("order-free paths must share declaration keys")
	}
}

// sweepDB is a stateful taskwarrior fake: deletes and completions
// really change tasks in the set, so sweeps, listings, and rm
// -rf-shaped sequences see the effects of earlier gestures.
type sweepDB struct {
	tasks     []backend.Task
	deleted   map[string]bool
	completed map[string]bool
}

func newSweepFS(t *testing.T, tasks ...backend.Task) (*FS, *fakeRunner, *sweepDB) {
	t.Helper()
	return newSweepFSWithAction(t, backend.RemoveDelete, tasks...)
}

func newSweepFSWithAction(t *testing.T, action backend.RemoveAction, tasks ...backend.Task) (*FS, *fakeRunner, *sweepDB) {
	t.Helper()
	return newSweepFSWithViews(t, action, nil, tasks...)
}

type testView struct {
	Name   string
	Filter string
	Modify string
}

func newViewStore(t *testing.T) *view.Store {
	t.Helper()
	store, err := view.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store
}

func newSweepFSWithViews(t *testing.T, action backend.RemoveAction, views []testView, tasks ...backend.Task) (*FS, *fakeRunner, *sweepDB) {
	t.Helper()
	store := newViewStore(t)
	for _, v := range views {
		definition := v.Filter
		if v.Modify != "" {
			definition += "\n" + v.Modify
		}
		if definition == "" {
			if err := store.Create(v.Name); err != nil {
				t.Fatalf("seed view %q: %v", v.Name, err)
			}
			continue
		}
		if err := store.Put(v.Name, definition); err != nil {
			t.Fatalf("seed view %q: %v", v.Name, err)
		}
	}
	db := &sweepDB{tasks: tasks, deleted: map[string]bool{}, completed: map[string]bool{}}
	runner := &fakeRunner{fn: db.serve}
	fs := New(backend.NewQuery(runner), Config{RemoveAction: action, ViewStore: store})
	return fs, runner, db
}

func newSweepFSWithDefinitions(t *testing.T, action backend.RemoveAction, definitions map[string]string, render func(context.Context, string) (string, error), tasks ...backend.Task) (*FS, *fakeRunner, *sweepDB) {
	t.Helper()
	store := newViewStore(t)
	for name, definition := range definitions {
		if err := store.Put(name, definition); err != nil {
			t.Fatalf("seed view %q: %v", name, err)
		}
	}
	db := &sweepDB{tasks: tasks, deleted: map[string]bool{}, completed: map[string]bool{}}
	runner := &fakeRunner{fn: db.serve}
	fs := New(backend.NewQuery(runner), Config{
		RemoveAction:   action,
		ViewStore:      store,
		RenderTemplate: render,
	})
	return fs, runner, db
}

func (db *sweepDB) serve(args []string) ([]byte, error) {
	switch {
	case hasArg(args, "_tags"):
		return []byte("COMPLETED\nPENDING\nDUE\nDELETED"), nil
	case hasArg(args, "_unique"):
		if hasArg(args, "project") {
			seen := map[string]bool{}
			var projects []string
			for _, t := range db.match(args) {
				if t.Project != "" && !seen[t.Project] {
					seen[t.Project] = true
					projects = append(projects, t.Project)
				}
			}
			return []byte(strings.Join(projects, "\n")), nil
		}
		seen := map[string]bool{}
		var tags []string
		for _, t := range db.match(args) {
			for _, tag := range t.Tags {
				if !seen[tag] {
					seen[tag] = true
					tags = append(tags, tag)
				}
			}
		}
		return []byte(strings.Join(tags, "\n")), nil
	case hasArg(args, "export"):
		return taskJSON(db.match(args)...), nil
	case hasArg(args, "delete"):
		for _, a := range args {
			if a != "rc.confirmation=off" && a != "delete" {
				db.deleted[a] = true
				for i, t := range db.tasks {
					if t.UUID == a {
						db.tasks[i].Status = "deleted"
					}
				}
			}
		}
		return nil, nil
	case hasArg(args, "done"):
		for _, a := range args {
			if a != "rc.confirmation=off" && a != "done" {
				db.completed[a] = true
				for i, t := range db.tasks {
					if t.UUID == a {
						db.tasks[i].Status = "completed"
					}
				}
			}
		}
		return nil, nil
	case hasArg(args, "modify"):
		for i, a := range args {
			if a != "modify" {
				continue
			}
			db.modify(args[i-1], args[i+1:])
			break
		}
		return nil, nil
	}
	return nil, nil
}

func (db *sweepDB) modify(uuid string, args []string) {
	for i := range db.tasks {
		if db.tasks[i].UUID != uuid {
			continue
		}
		for _, arg := range args {
			switch {
			case strings.HasPrefix(arg, "project:"):
				db.tasks[i].Project = strings.TrimPrefix(arg, "project:")
			case strings.HasPrefix(arg, "description:"):
				db.tasks[i].Description = strings.TrimPrefix(arg, "description:")
			case strings.HasPrefix(arg, "+"):
				tag := strings.TrimPrefix(arg, "+")
				if !contains(db.tasks[i].Tags, tag) {
					db.tasks[i].Tags = append(db.tasks[i].Tags, tag)
				}
			case strings.HasPrefix(arg, "-"):
				tag := strings.TrimPrefix(arg, "-")
				var tags []string
				for _, existing := range db.tasks[i].Tags {
					if existing != tag {
						tags = append(tags, existing)
					}
				}
				db.tasks[i].Tags = tags
			}
		}
	}
}

// match emulates taskwarrior's filter semantics for the argument shapes
// the filesystem issues: project prefixes match subprojects, uppercase
// +NAME segments constrain status (PENDING) or are derived (DUE), and a
// bare export with no status argument admits every status.
func (db *sweepDB) match(args []string) []backend.Task {
	args = expandGroupedFilter(args)
	sweep := hasArg(args, "status.not:deleted")
	notCompleted := hasArg(args, "status.not:completed")
	completedView := hasArg(args, "status:completed") || hasArg(args, "+COMPLETED")
	pendingView := hasArg(args, "status:pending") || hasArg(args, "+PENDING")
	deletedView := hasArg(args, "status:deleted") || hasArg(args, "+DELETED")
	allStatuses := !sweep && !completedView && !pendingView && !deletedView
	var out []backend.Task
	for _, t := range db.tasks {
		if t.Status == "deleted" && (sweep || (!deletedView && !allStatuses)) {
			continue
		}
		if t.Status == "completed" && notCompleted {
			continue
		}
		if !filterMatches(args, t, sweep, completedView, pendingView, deletedView, allStatuses) {
			continue
		}
		out = append(out, t)
	}
	return out
}

func expandGroupedFilter(args []string) []string {
	var out []string
	for _, a := range args {
		if strings.HasPrefix(a, "( ") && strings.HasSuffix(a, " )") {
			inner := strings.TrimSuffix(strings.TrimPrefix(a, "( "), " )")
			out = append(out, strings.Fields(inner)...)
			continue
		}
		out = append(out, a)
	}
	return out
}

func filterMatches(args []string, t backend.Task, sweep, completedView, pendingView, deletedView, allStatuses bool) bool {
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "project:"):
			p := strings.TrimPrefix(a, "project:")
			if t.Project != p && !strings.HasPrefix(t.Project, p+".") {
				return false
			}
		case strings.HasPrefix(a, "+"):
			tag := a[1:]
			switch tag {
			case "PENDING":
				if t.Status != "pending" {
					return false
				}
			case "COMPLETED":
				if t.Status != "completed" {
					return false
				}
			case "DELETED":
				if t.Status != "deleted" {
					return false
				}
			case "DUE", "ACTIVE", "OVERDUE", "WAITING":
			default:
				if !contains(t.Tags, tag) {
					return false
				}
			}
		}
	}
	switch {
	case deletedView:
		return t.Status == "deleted"
	case completedView:
		return t.Status == "completed"
	case pendingView:
		return t.Status == "pending"
	case allStatuses:
		return true
	case sweep:
		return t.Status != "deleted"
	default:
		return t.Status == "pending"
	}
}

func rmdirDeletedUUIDs(runner *fakeRunner) []string {
	var out []string
	for _, cmd := range runner.log {
		if hasArg(cmd, "delete") {
			out = append(out, cmd[len(cmd)-2])
		}
	}
	return out
}

func rmdirCompletedUUIDs(runner *fakeRunner) []string {
	var out []string
	for _, cmd := range runner.log {
		if hasArg(cmd, "done") {
			out = append(out, cmd[len(cmd)-2])
		}
	}
	return out
}

func lastModifyArgs(runner *fakeRunner) []string {
	var got []string
	for _, cmd := range runner.log {
		for i, arg := range cmd {
			if arg == "modify" {
				got = cmd[i+1:]
			}
		}
	}
	return got
}

func mutationCount(runner *fakeRunner) int {
	n := 0
	for _, cmd := range runner.log {
		if hasArg(cmd, "delete") || hasArg(cmd, "modify") || hasArg(cmd, "done") || hasArg(cmd, "add") {
			n++
		}
	}
	return n
}

func TestRmdirProjectDeletesSubtree(t *testing.T) {
	fs, runner, db := newSweepFS(t,
		backend.Task{UUID: uuidA, Description: "Top task", Project: "this", Status: "pending"},
		backend.Task{UUID: uuidB, Description: "Nested task", Project: "this.sub", Status: "pending"},
		backend.Task{UUID: "c9d0e1f2-2222-2222-2222-222222222222", Description: "Old nested", Project: "this.sub", Status: "completed"},
	)
	if errno := fs.Rmdir(context.Background(), "this"); errno != 0 {
		t.Fatalf("rmdir this: %v", errno)
	}
	got := rmdirDeletedUUIDs(runner)
	if len(got) != 3 {
		t.Fatalf("deleted uuids = %v, want all three", got)
	}
	for _, uuid := range []string{uuidA, uuidB, "c9d0e1f2-2222-2222-2222-222222222222"} {
		if !db.deleted[uuid] {
			t.Errorf("task %s not deleted", uuid)
		}
	}
	names, errno := readdirOf(t, &fs.dirNode)
	if errno != 0 {
		t.Fatalf("Readdir root: %v", errno)
	}
	if contains(names, "this") {
		t.Errorf("removed project still listed: %v", names)
	}
}

func TestRmdirProjectCompletesSubtreeUnderComplete(t *testing.T) {
	fs, runner, db := newSweepFSWithAction(t, backend.RemoveComplete,
		backend.Task{UUID: uuidA, Description: "Top task", Project: "this", Status: "pending"},
		backend.Task{UUID: uuidB, Description: "Nested task", Project: "this.sub", Status: "pending"},
		backend.Task{UUID: "c9d0e1f2-2222-2222-2222-222222222222", Description: "Old nested", Project: "this.sub", Status: "completed"},
	)
	if errno := fs.Rmdir(context.Background(), "this"); errno != 0 {
		t.Fatalf("rmdir this: %v", errno)
	}
	got := rmdirCompletedUUIDs(runner)
	if len(got) != 2 || !db.completed[uuidA] || !db.completed[uuidB] {
		t.Errorf("completed uuids = %v, want the two pending tasks", got)
	}
	if db.completed["c9d0e1f2-2222-2222-2222-222222222222"] {
		t.Error("already-completed task must not be completed again")
	}
	if len(rmdirDeletedUUIDs(runner)) != 0 {
		t.Errorf("complete removal issued deletes: %v", rmdirDeletedUUIDs(runner))
	}
	names, errno := readdirOf(t, &fs.dirNode)
	if errno != 0 {
		t.Fatalf("Readdir root: %v", errno)
	}
	if contains(names, "this") {
		t.Errorf("removed project still listed: %v", names)
	}
}

func TestRmdirTagCompletesAcrossProjectsUnderComplete(t *testing.T) {
	fs, runner, db := newSweepFSWithAction(t, backend.RemoveComplete,
		backend.Task{UUID: uuidA, Description: "Deep task", Project: "home.clean-garage", Tags: []string{"next"}, Status: "pending"},
		backend.Task{UUID: uuidB, Description: "Flat task", Project: "finances", Tags: []string{"next"}, Status: "completed"},
	)
	if errno := fs.Rmdir(context.Background(), "+next"); errno != 0 {
		t.Fatalf("rmdir +next: %v", errno)
	}
	if !db.completed[uuidA] || db.completed[uuidB] {
		t.Errorf("complete tag sweep completed the wrong tasks: %v", rmdirCompletedUUIDs(runner))
	}
	if len(rmdirDeletedUUIDs(runner)) != 0 {
		t.Errorf("complete removal issued deletes: %v", rmdirDeletedUUIDs(runner))
	}
	names, errno := readdirOf(t, &fs.dirNode)
	if errno != 0 {
		t.Fatalf("Readdir root: %v", errno)
	}
	if contains(names, "+next") {
		t.Errorf("removed tag still listed: %v", names)
	}
}

func TestRmdirTagDeletesAcrossProjects(t *testing.T) {
	fs, runner, db := newSweepFS(t,
		backend.Task{UUID: uuidA, Description: "Deep task", Project: "home.clean-garage", Tags: []string{"next"}, Status: "pending"},
		backend.Task{UUID: uuidB, Description: "Flat task", Project: "finances", Tags: []string{"next"}, Status: "completed"},
	)
	if errno := fs.Rmdir(context.Background(), "+next"); errno != 0 {
		t.Fatalf("rmdir +next: %v", errno)
	}
	if len(rmdirDeletedUUIDs(runner)) != 2 || !db.deleted[uuidA] || !db.deleted[uuidB] {
		t.Errorf("tagged tasks not all deleted: %v", runner.log)
	}
	names, errno := readdirOf(t, &fs.dirNode)
	if errno != 0 {
		t.Fatalf("Readdir root: %v", errno)
	}
	if contains(names, "+next") {
		t.Errorf("removed tag still listed: %v", names)
	}
}

func TestRmdirSkipsDeletedTasks(t *testing.T) {
	deletedTask := backend.Task{UUID: uuidB, Description: "Gone", Project: "this", Status: "deleted"}
	fs, runner, db := newSweepFS(t,
		backend.Task{UUID: uuidA, Description: "Alive", Project: "this", Status: "pending"},
		deletedTask,
	)
	db.tasks[1].Status = "deleted"
	if errno := fs.Rmdir(context.Background(), "this"); errno != 0 {
		t.Fatalf("rmdir this: %v", errno)
	}
	got := rmdirDeletedUUIDs(runner)
	if len(got) != 1 || got[0] != uuidA {
		t.Errorf("deleted uuids = %v, want [%s]", got, uuidA)
	}
}

func TestRmdirUnknownNameChangesNothing(t *testing.T) {
	fs, runner, _ := newSweepFS(t,
		backend.Task{UUID: uuidA, Description: "Chore", Project: "home", Status: "pending"},
	)
	if errno := fs.Rmdir(context.Background(), "homi"); errno != syscall.ENOENT {
		t.Fatalf("rmdir typo: errno = %v, want ENOENT", errno)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("ENOENT rmdir issued %d mutations", n)
	}
}

func TestRmdirTombstonedProjectSucceeds(t *testing.T) {
	fs, runner, db := newSweepFS(t,
		backend.Task{UUID: uuidA, Description: "Alive", Project: "this", Status: "pending"},
	)
	db.tasks[0].Status = "deleted"
	db.tasks = append(db.tasks, backend.Task{UUID: uuidB, Description: "Gone", Project: "this", Status: "deleted"})
	if errno := fs.Rmdir(context.Background(), "this"); errno != 0 {
		t.Fatalf("rmdir tombstoned project: %v", errno)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("tombstoned rmdir issued %d mutations", n)
	}
}

func TestRmdirVirtualTagViewNeedsEmptyView(t *testing.T) {
	fs, runner, db := newSweepFS(t,
		backend.Task{UUID: uuidA, Description: "Chore", Project: "this", Status: "pending"},
		backend.Task{UUID: uuidB, Description: "Old chore", Project: "this", Status: "completed"},
	)
	thisNode, errno := fs.resolveChild("this")
	if errno != 0 {
		t.Fatalf("lookup this: %v", errno)
	}
	if errno := thisNode.(*dirNode).Rmdir(context.Background(), "+COMPLETED"); errno != syscall.ENOTEMPTY {
		t.Fatalf("rmdir view with tasks: errno = %v, want ENOTEMPTY", errno)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("non-destructive view rmdir issued %d mutations", n)
	}
	db.tasks[1].Status = "deleted"
	if errno := thisNode.(*dirNode).Rmdir(context.Background(), "+COMPLETED"); errno != 0 {
		t.Errorf("rmdir emptied view: %v", errno)
	}
}

func TestRmdirDotVirtualTagViewIsENOENT(t *testing.T) {
	fs, runner, _ := newSweepFS(t,
		backend.Task{UUID: uuidA, Description: "Chore", Project: "this", Status: "pending"},
		backend.Task{UUID: uuidB, Description: "Old chore", Project: "this", Status: "completed"},
	)
	thisNode, errno := fs.resolveChild("this")
	if errno != 0 {
		t.Fatalf("lookup this: %v", errno)
	}
	if errno := thisNode.(*dirNode).Rmdir(context.Background(), ".+COMPLETED"); errno != syscall.ENOENT {
		t.Errorf("rmdir .+COMPLETED: errno = %v, want ENOENT", errno)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("rejected view rmdir issued %d mutations", n)
	}
}

func TestRmRfWalkThroughStatusViews(t *testing.T) {
	fs, runner, db := newSweepFS(t,
		backend.Task{UUID: uuidA, Description: "Do taxes", Project: "this", Status: "pending"},
		backend.Task{UUID: uuidB, Description: "Old taxes", Project: "this", Status: "completed"},
	)
	thisNode, errno := fs.resolveChild("this")
	if errno != 0 {
		t.Fatalf("lookup this: %v", errno)
	}
	this := thisNode.(*dirNode)

	pendingName := naming.FileName("Do taxes", uuidA)
	if errno := this.Unlink(context.Background(), pendingName); errno != 0 {
		t.Fatalf("unlink pending file: %v", errno)
	}
	if errno := this.Rmdir(context.Background(), "+COMPLETED"); errno != syscall.ENOTEMPTY {
		t.Fatalf("rmdir completed view before its file is unlinked: errno = %v, want ENOTEMPTY", errno)
	}

	completedName := naming.FileName("Old taxes", uuidB)
	completedView, errno := this.resolveChild("+COMPLETED")
	if errno != 0 {
		t.Fatalf("lookup +COMPLETED: %v", errno)
	}
	if errno := completedView.(*dirNode).Unlink(context.Background(), completedName); errno != 0 {
		t.Fatalf("unlink completed file: %v", errno)
	}
	if errno := this.Rmdir(context.Background(), "+COMPLETED"); errno != 0 {
		t.Fatalf("rmdir +COMPLETED: %v", errno)
	}
	if errno := this.Rmdir(context.Background(), "+DELETED"); errno != 0 {
		t.Fatalf("rmdir +DELETED: %v", errno)
	}

	if errno := fs.Rmdir(context.Background(), "this"); errno != 0 {
		t.Fatalf("rmdir this: %v", errno)
	}
	got := rmdirDeletedUUIDs(runner)
	if len(got) != 2 || !db.deleted[uuidA] || !db.deleted[uuidB] {
		t.Errorf("deleted uuids = %v, want both", got)
	}
	names, errno := readdirOf(t, &fs.dirNode)
	if errno != 0 {
		t.Fatalf("Readdir root: %v", errno)
	}
	if contains(names, "this") {
		t.Errorf("removed project still listed: %v", names)
	}
}

func TestRmdirVirtualTagViewUnderComplete(t *testing.T) {
	fs, runner, _ := newSweepFSWithAction(t, backend.RemoveComplete,
		backend.Task{UUID: uuidA, Description: "Chore", Project: "this", Status: "pending"},
		backend.Task{UUID: uuidB, Description: "Old chore", Project: "this", Status: "completed"},
	)
	thisNode, errno := fs.resolveChild("this")
	if errno != 0 {
		t.Fatalf("lookup this: %v", errno)
	}
	this := thisNode.(*dirNode)

	if errno := this.Rmdir(context.Background(), "+COMPLETED"); errno != 0 {
		t.Fatalf("rmdir completed view: %v", errno)
	}
	if errno := this.Rmdir(context.Background(), "+DELETED"); errno != 0 {
		t.Fatalf("rmdir deleted view: %v", errno)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("view rmdir issued %d mutations", n)
	}
}

func TestRmRfWalkThroughStatusViewsUnderComplete(t *testing.T) {
	fs, runner, db := newSweepFSWithAction(t, backend.RemoveComplete,
		backend.Task{UUID: uuidA, Description: "Do taxes", Project: "this", Status: "pending"},
		backend.Task{UUID: uuidB, Description: "Old taxes", Project: "this", Status: "completed"},
	)
	thisNode, errno := fs.resolveChild("this")
	if errno != 0 {
		t.Fatalf("lookup this: %v", errno)
	}
	this := thisNode.(*dirNode)

	pendingName := naming.FileName("Do taxes", uuidA)
	if errno := this.Unlink(context.Background(), pendingName); errno != 0 {
		t.Fatalf("unlink pending file: %v", errno)
	}

	completedView, errno := this.resolveChild("+COMPLETED")
	if errno != 0 {
		t.Fatalf("lookup +COMPLETED: %v", errno)
	}
	for _, name := range []string{naming.FileName("Do taxes", uuidA), naming.FileName("Old taxes", uuidB)} {
		if errno := completedView.(*dirNode).Unlink(context.Background(), name); errno != 0 {
			t.Fatalf("unlink completed file %q: %v", name, errno)
		}
	}
	if errno := this.Rmdir(context.Background(), "+COMPLETED"); errno != 0 {
		t.Fatalf("rmdir +COMPLETED: %v", errno)
	}
	if errno := this.Rmdir(context.Background(), "+DELETED"); errno != 0 {
		t.Fatalf("rmdir +DELETED: %v", errno)
	}

	if errno := fs.Rmdir(context.Background(), "this"); errno != 0 {
		t.Fatalf("rmdir this: %v", errno)
	}
	if got := rmdirCompletedUUIDs(runner); len(got) != 1 || got[0] != uuidA {
		t.Errorf("completed uuids = %v, want only the pending task", got)
	}
	if len(rmdirDeletedUUIDs(runner)) != 0 {
		t.Errorf("complete walk issued deletes: %v", rmdirDeletedUUIDs(runner))
	}
	if !db.completed[uuidA] || db.completed[uuidB] {
		t.Errorf("completed flags = %v", db.completed)
	}
	names, errno := readdirOf(t, &fs.dirNode)
	if errno != 0 {
		t.Fatalf("Readdir root: %v", errno)
	}
	if contains(names, "this") {
		t.Errorf("removed project still listed: %v", names)
	}
}

func TestRmdirPurgesUnderlyingDeclarations(t *testing.T) {
	fs, _, _ := newSweepFS(t,
		backend.Task{UUID: uuidA, Description: "Chore", Project: "other", Status: "pending"},
	)
	if _, errno := fs.declareChild("this"); errno != 0 {
		t.Fatalf("mkdir this: %v", errno)
	}
	thisNode, errno := fs.resolveChild("this")
	if errno != 0 {
		t.Fatalf("lookup this: %v", errno)
	}
	if _, errno := thisNode.(*dirNode).declareChild("ghost"); errno != 0 {
		t.Fatalf("mkdir ghost: %v", errno)
	}
	if errno := fs.Rmdir(context.Background(), "this"); errno != 0 {
		t.Fatalf("rmdir this: %v", errno)
	}
	names, errno := readdirOf(t, &fs.dirNode)
	if errno != 0 {
		t.Fatalf("Readdir root: %v", errno)
	}
	if contains(names, "this") {
		t.Errorf("ghost declaration kept removed project listed: %v", names)
	}
	if _, errno := fs.resolveChild("this"); errno != syscall.ENOENT {
		t.Errorf("resolve after purge: errno = %v, want ENOENT", errno)
	}
}

func TestUnlinkDeletedTaskIsNoop(t *testing.T) {
	for _, action := range []backend.RemoveAction{backend.RemoveDelete, backend.RemoveComplete} {
		t.Run(string(action), func(t *testing.T) {
			fs, runner, db := newSweepFSWithAction(t, action,
				backend.Task{UUID: uuidA, Description: "Gone", Project: "home", Status: "deleted"},
				backend.Task{UUID: uuidB, Description: "Alive", Project: "home", Status: "pending"},
			)
			db.tasks[0].Status = "deleted"
			home, errno := fs.resolveChild("home")
			if errno != 0 {
				t.Fatalf("lookup home: %v", errno)
			}
			homeNode := home.(*dirNode)
			deletedView, errno := homeNode.resolveChild("+DELETED")
			if errno != 0 {
				t.Fatalf("lookup +DELETED: %v", errno)
			}
			name := naming.FileName("Gone", uuidA)
			if errno := deletedView.(*dirNode).Unlink(context.Background(), name); errno != 0 {
				t.Fatalf("unlink tombstone: %v", errno)
			}
			if n := mutationCount(runner); n != 0 {
				t.Errorf("tombstone unlink issued %d mutations", n)
			}
		})
	}
}

func TestRmdirDeeperProjectAndTagConjunction(t *testing.T) {
	fs, runner, db := newSweepFS(t,
		backend.Task{UUID: uuidA, Description: "Match", Project: "this.is.a.test", Tags: []string{"next"}, Status: "pending"},
		backend.Task{UUID: uuidB, Description: "Untagged same project", Project: "this.is.a.test", Status: "pending"},
		backend.Task{UUID: "c9d0e1f2-2222-2222-2222-222222222222", Description: "Tagged other project", Project: "home", Tags: []string{"next"}, Status: "pending"},
		backend.Task{UUID: "d1e2f3a4-3333-3333-3333-333333333333", Description: "Tagged deeper", Project: "this.is.a.test.deep", Tags: []string{"next"}, Status: "completed"},
	)
	// Walk the exact path shape: this/is/a/+next, then rmdir test.
	node := &fs.dirNode
	for _, seg := range []string{"this", "is", "a", "+next"} {
		child, errno := node.resolveChild(seg)
		if errno != 0 {
			t.Fatalf("lookup %s: %v", seg, errno)
		}
		node = child.(*dirNode)
	}
	if errno := node.Rmdir(context.Background(), "test"); errno != 0 {
		t.Fatalf("rmdir test: %v", errno)
	}
	if got := rmdirDeletedUUIDs(runner); len(got) != 2 || !db.deleted[uuidA] || !db.deleted["d1e2f3a4-3333-3333-3333-333333333333"] {
		t.Errorf("deleted uuids = %v, want exactly the tagged this.is.a.test subtree", got)
	}
	if db.deleted[uuidB] {
		t.Error("untagged task in the same project must survive")
	}
	if db.deleted["c9d0e1f2-2222-2222-2222-222222222222"] {
		t.Error("tagged task in another project must survive")
	}
}

func resolveViewNode(t *testing.T, fs *FS, entryName string) *viewNode {
	t.Helper()
	child, errno := fs.resolveChild(entryName)
	if errno != 0 {
		t.Fatalf("lookup %s: %v", entryName, errno)
	}
	vn, ok := child.(*viewNode)
	if !ok {
		t.Fatalf("lookup %s returned %T, want *viewNode", entryName, child)
	}
	return vn
}

func readdirViewOf(t *testing.T, n *viewNode) []string {
	t.Helper()
	stream, errno := n.Readdir(context.Background())
	if errno != 0 {
		t.Fatalf("Readdir custom view: %v", errno)
	}
	return readdirNames(t, stream)
}

func TestRootListsConfiguredViews(t *testing.T) {
	fs, _, _ := newSweepFSWithViews(t, backend.RemoveDelete,
		[]testView{{Name: "Zulu", Filter: "status:any:"}, {Name: "Next", Filter: "+next"}},
		task(uuidA, "Chore", "home", "next"),
	)

	names := readdirOfRoot(t, fs)
	for _, want := range []string{"@Next", "@Zulu"} {
		if !contains(names, want) {
			t.Errorf("root listing missing %q: %v", want, names)
		}
	}
	if names[0] != "@Next" || names[1] != "@Zulu" {
		t.Errorf("custom view directories not listed first and sorted: %v", names)
	}

	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	homeNames, errno := readdirOf(t, home.(*dirNode))
	if errno != 0 {
		t.Fatalf("Readdir home: %v", errno)
	}
	for _, absent := range []string{"@Next", "@Zulu"} {
		if contains(homeNames, absent) {
			t.Errorf("custom view directory %q listed below the root: %v", absent, homeNames)
		}
	}

	emptyFS, _, _ := newSweepFSWithViews(t, backend.RemoveDelete, []testView{{Name: "Empty", Filter: "project:missing"}})
	if names := readdirOfRoot(t, emptyFS); !contains(names, "@Empty") {
		t.Errorf("empty custom view missing from root: %v", names)
	}
	emptyNode := resolveViewNode(t, emptyFS, "@Empty")
	if names := readdirViewOf(t, emptyNode); len(names) != 1 || names[0] != viewFileName {
		t.Errorf("empty custom view = %v, want only %q", names, viewFileName)
	}
}

func readdirOfRoot(t *testing.T, fs *FS) []string {
	t.Helper()
	stream, errno := fs.Readdir(context.Background())
	if errno != 0 {
		t.Fatalf("Readdir root: %v", errno)
	}
	return readdirNames(t, stream)
}

func newDynamicViewFS(t *testing.T, action backend.RemoveAction, render func(context.Context, string) (string, error), tasks ...backend.Task) (*FS, *fakeRunner, *sweepDB) {
	t.Helper()
	store := newViewStore(t)
	if err := store.Put("Dynamic", "unused"); err != nil {
		t.Fatalf("seed dynamic view: %v", err)
	}
	db := &sweepDB{tasks: tasks, deleted: map[string]bool{}, completed: map[string]bool{}}
	runner := &fakeRunner{fn: db.serve}
	f := New(backend.NewQuery(runner), Config{
		RemoveAction:   action,
		ViewStore:      store,
		RenderTemplate: render,
	})
	return f, runner, db
}

func TestDynamicViewUsesRenderedFilter(t *testing.T) {
	filter := "project:home"
	home := task(uuidA, "Home task", "home")
	finances := task(uuidB, "Finance task", "finances")
	fs, _, _ := newDynamicViewFS(t, backend.RemoveDelete, func(context.Context, string) (string, error) {
		return filter, nil
	}, home, finances)

	vn := resolveViewNode(t, fs, "@Dynamic")
	homeName := naming.FileName(home.Description, home.UUID)
	financeName := naming.FileName(finances.Description, finances.UUID)

	names := readdirViewOf(t, vn)
	if !contains(names, homeName) || contains(names, financeName) {
		t.Fatalf("first listing = %v, want only %q", names, homeName)
	}

	filter = "project:finances"
	names = readdirViewOf(t, vn)
	if contains(names, homeName) || !contains(names, financeName) {
		t.Fatalf("second listing = %v, want only %q", names, financeName)
	}

	if _, errno := vn.resolveChild(context.Background(), homeName, false); errno != syscall.ENOENT {
		t.Errorf("lookup of no-longer-matching file: errno = %v, want ENOENT", errno)
	}
}

func TestDynamicViewReusesRenderWithinTraversal(t *testing.T) {
	calls := 0
	home := task(uuidA, "Home task", "home")
	fs, _, _ := newDynamicViewFS(t, backend.RemoveDelete, func(context.Context, string) (string, error) {
		calls++
		return "project:home", nil
	}, home)
	vn := resolveViewNode(t, fs, "@Dynamic")
	name := naming.FileName(home.Description, home.UUID)

	readdirViewOf(t, vn)
	if calls != 1 {
		t.Fatalf("render calls after listing = %d, want 1", calls)
	}
	for i := 0; i < 3; i++ {
		if _, errno := vn.resolveChild(context.Background(), name, false); errno != 0 {
			t.Fatalf("lookup %d: %v", i, errno)
		}
	}
	if calls != 1 {
		t.Errorf("render calls after lookups = %d, want 1", calls)
	}
	readdirViewOf(t, vn)
	if calls != 2 {
		t.Errorf("render calls after second listing = %d, want 2", calls)
	}
}

func TestDynamicViewForcesFreshRenderForRemoval(t *testing.T) {
	filter := "project:home"
	home := task(uuidA, "Home task", "home")
	fs, runner, _ := newDynamicViewFS(t, backend.RemoveDelete, func(context.Context, string) (string, error) {
		return filter, nil
	}, home)
	vn := resolveViewNode(t, fs, "@Dynamic")
	name := naming.FileName(home.Description, home.UUID)

	if _, errno := vn.resolveChild(context.Background(), name, false); errno != 0 {
		t.Fatalf("initial lookup: %v", errno)
	}
	filter = "project:none"
	if errno := vn.Unlink(context.Background(), name); errno != syscall.ENOENT {
		t.Errorf("unlink after filter change: errno = %v, want ENOENT", errno)
	}

	filter = "project:home"
	if errno := fs.Rmdir(context.Background(), "@Dynamic"); errno != 0 {
		t.Errorf("rmdir custom view: %v", errno)
	}
	if names := readdirOfRoot(t, fs); contains(names, "@Dynamic") {
		t.Errorf("custom view remained after rmdir: %v", names)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("render-only changes issued %d mutations", n)
	}
}

func TestDynamicViewRenderFailureReturnsEIO(t *testing.T) {
	home := task(uuidA, "Home task", "home")
	fs, runner, _ := newDynamicViewFS(t, backend.RemoveDelete, func(context.Context, string) (string, error) {
		return "", errors.New("boom")
	}, home)
	vn := resolveViewNode(t, fs, "@Dynamic")
	name := naming.FileName(home.Description, home.UUID)

	if _, errno := vn.Readdir(context.Background()); errno != syscall.EIO {
		t.Errorf("Readdir: errno = %v, want EIO", errno)
	}
	if _, errno := vn.Lookup(context.Background(), name, &fuse.EntryOut{}); errno != syscall.EIO {
		t.Errorf("Lookup: errno = %v, want EIO", errno)
	}
	if errno := vn.Unlink(context.Background(), name); errno != syscall.EIO {
		t.Errorf("Unlink: errno = %v, want EIO", errno)
	}
	if errno := fs.Rmdir(context.Background(), "@Dynamic"); errno != 0 {
		t.Errorf("Rmdir should delete the definition without rendering: %v", errno)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("render failures issued %d mutations", n)
	}
}

func TestCustomViewDirectoryIsLookupOnly(t *testing.T) {
	fs, _, _ := newSweepFSWithViews(t, backend.RemoveDelete, []testView{{Name: "Next", Filter: "+next"}},
		task(uuidA, "Deep task", "home.clean-garage", "next"),
		task(uuidB, "Flat task", "", "next"),
		task("c9d0e1f2-2222-2222-2222-222222222222", "Other task", "home"),
	)
	vn := resolveViewNode(t, fs, "@Next")
	names := readdirViewOf(t, vn)
	for _, want := range []string{
		naming.FileName("Deep task", uuidA),
		naming.FileName("Flat task", uuidB),
	} {
		if !contains(names, want) {
			t.Errorf("custom view listing missing %q: %v", want, names)
		}
	}
	for _, absent := range []string{
		"home",
		"+next",
		"+COMPLETED",
		naming.FileName("Other task", "c9d0e1f2-2222-2222-2222-222222222222"),
	} {
		if contains(names, absent) {
			t.Errorf("custom view listing contains %q: %v", absent, names)
		}
	}
	for _, name := range []string{"home", "+next", "+COMPLETED"} {
		if _, errno := vn.resolveChild(context.Background(), name, false); errno != syscall.ENOENT {
			t.Errorf("custom view lookup %q: errno = %v, want ENOENT", name, errno)
		}
	}
}

func TestCustomViewKeepsExactStatusSemantics(t *testing.T) {
	fs, _, _ := newSweepFSWithViews(t, backend.RemoveDelete, []testView{{Name: "All", Filter: "status.any:"}},
		task(uuidA, "Pending task", "home"),
		backend.Task{UUID: uuidB, Description: "Done task", Project: "home", Status: "completed"},
	)
	vn := resolveViewNode(t, fs, "@All")
	names := readdirViewOf(t, vn)
	for _, want := range []string{
		naming.FileName("Pending task", uuidA),
		naming.FileName("Done task", uuidB),
	} {
		if !contains(names, want) {
			t.Errorf("custom view listing missing %q: %v", want, names)
		}
	}
}

func TestCustomViewNamesTakePrecedenceAtRoot(t *testing.T) {
	atProject := task(uuidA, "At project", "@Due")

	withView, _, _ := newSweepFSWithViews(t, backend.RemoveDelete, []testView{{Name: "Due", Filter: "status:any:"}}, atProject)
	child, errno := withView.resolveChild("@Due")
	if errno != 0 {
		t.Fatalf("lookup @Due with custom view: %v", errno)
	}
	if _, ok := child.(*viewNode); !ok {
		t.Fatalf("lookup @Due with custom view returned %T, want *viewNode", child)
	}

	withoutView, _, _ := newSweepFSWithViews(t, backend.RemoveDelete, nil, atProject)
	child, errno = withoutView.resolveChild("@Due")
	if errno != 0 {
		t.Fatalf("lookup @Due without custom view: %v", errno)
	}
	dir, ok := child.(*dirNode)
	if !ok || dir.filter.Project != "@Due" {
		t.Fatalf("lookup @Due without custom view returned %T / %+v, want @Due project", child, child)
	}
}

func TestCustomViewTaskFileInodePerView(t *testing.T) {
	fs, _, _ := newSweepFSWithViews(t, backend.RemoveDelete, []testView{{Name: "Next", Filter: "+next"}},
		task(uuidA, "Chore", "home", "next"),
	)
	name := naming.FileName("Chore", uuidA)
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	inHome, errno := home.(*dirNode).resolveChild(name)
	if errno != 0 {
		t.Fatalf("lookup in /home: %v", errno)
	}
	vn := resolveViewNode(t, fs, "@Next")
	inView, errno := vn.resolveChild(context.Background(), name, false)
	if errno != 0 {
		t.Fatalf("lookup in @Next: %v", errno)
	}
	if a, b := inHome.(*fileNode).stableAttr(), inView.(*fileNode).stableAttr(); a.Ino == b.Ino {
		t.Error("same task must carry distinct inode numbers in a custom view and a filter view")
	}
}

func TestCustomViewTaskFileIdentityAndContent(t *testing.T) {
	fs, _, _ := newSweepFSWithViews(t, backend.RemoveDelete, []testView{{Name: "All", Filter: "status:any:"}},
		task(uuidA, "Do taxes", "finances"),
	)
	name := naming.FileName("Do taxes", uuidA)
	vn := resolveViewNode(t, fs, "@All")
	child, errno := vn.resolveChild(context.Background(), name, false)
	if errno != 0 {
		t.Fatalf("lookup custom view task file: %v", errno)
	}
	fn := child.(*fileNode)
	buf := make([]byte, 16)
	res, errno := fn.Read(context.Background(), nil, buf, 0)
	if errno != 0 || res.Size() != 0 {
		t.Errorf("read = %d bytes, errno %v; want 0 bytes", res.Size(), errno)
	}
	var attr fuse.AttrOut
	if errno := fn.Getattr(context.Background(), nil, &attr); errno != 0 || attr.Size != 0 {
		t.Errorf("Getattr size = %d, errno %v; want size 0", attr.Size, errno)
	}
	got := make([]byte, len(uuidA))
	n, errno := fn.Getxattr(context.Background(), uuidXattr, got)
	if errno != 0 || n != uint32(len(uuidA)) || string(got) != uuidA {
		t.Errorf("Getxattr = %q, %v; want %q", string(got), errno, uuidA)
	}
}

func TestCustomViewDirectoriesRejectCreationAndMoves(t *testing.T) {
	fs, runner, _ := newSweepFSWithViews(t, backend.RemoveDelete, []testView{{Name: "Next", Filter: "+next"}},
		task(uuidA, "Chore", "home", "next"),
	)
	vn := resolveViewNode(t, fs, "@Next")
	name := naming.FileName("Chore", uuidA)

	if _, _, _, errno := vn.Create(context.Background(), "New task", 0, 0, &fuse.EntryOut{}); errno != syscall.EACCES {
		t.Errorf("custom view create: errno = %v, want EACCES", errno)
	}
	if _, errno := vn.Mkdir(context.Background(), "+urgent", 0, &fuse.EntryOut{}); errno != syscall.EACCES {
		t.Errorf("custom view mkdir: errno = %v, want EACCES", errno)
	}
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	if errno := home.(*dirNode).Rename(context.Background(), name, vn, name, 0); errno != syscall.EACCES {
		t.Errorf("rename into custom view: errno = %v, want EACCES", errno)
	}
	if errno := vn.Rename(context.Background(), name, home.(*dirNode), name, 0); errno != syscall.EACCES {
		t.Errorf("rename out of custom view: errno = %v, want EACCES", errno)
	}
	completed, errno := fs.resolveChild("+COMPLETED")
	if errno != 0 {
		t.Fatalf("lookup +COMPLETED: %v", errno)
	}
	if errno := vn.Rename(context.Background(), name, completed.(*dirNode), name, 0); errno != syscall.EACCES {
		t.Errorf("rename custom view result to status view: errno = %v, want EACCES", errno)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("rejected custom view gestures issued %d mutations", n)
	}
}

func TestCustomViewTaskRemovalAppliesConfiguredAction(t *testing.T) {
	for _, tt := range []struct {
		action backend.RemoveAction
		done   func(*sweepDB) bool
	}{
		{backend.RemoveComplete, func(db *sweepDB) bool { return db.completed[uuidA] }},
		{backend.RemoveDelete, func(db *sweepDB) bool { return db.deleted[uuidA] }},
	} {
		t.Run(string(tt.action), func(t *testing.T) {
			fs, _, db := newSweepFSWithViews(t, tt.action, []testView{{Name: "Due", Filter: "status:pending"}},
				task(uuidA, "Do taxes", "home"),
			)
			vn := resolveViewNode(t, fs, "@Due")
			name := naming.FileName("Do taxes", uuidA)
			if errno := vn.Unlink(context.Background(), name); errno != 0 {
				t.Fatalf("unlink custom view task: %v", errno)
			}
			if !tt.done(db) {
				t.Errorf("task was not changed by %s", tt.action)
			}
			if names := readdirViewOf(t, vn); contains(names, name) {
				t.Errorf("removed task still listed in custom view: %v", names)
			}
		})
	}
}

func TestCompletedCustomViewTaskRemovalIsNoopUnderComplete(t *testing.T) {
	fs, runner, _ := newSweepFSWithViews(t, backend.RemoveComplete, []testView{{Name: "Done", Filter: "status:completed"}},
		backend.Task{UUID: uuidA, Description: "Finished", Project: "home", Status: "completed"},
	)
	vn := resolveViewNode(t, fs, "@Done")
	name := naming.FileName("Finished", uuidA)
	if errno := vn.Unlink(context.Background(), name); errno != 0 {
		t.Fatalf("unlink completed custom view task: %v", errno)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("completed custom view task unlink issued %d mutations", n)
	}
}

func TestRmdirCustomViewDeletesDefinitionWithoutChangingTasks(t *testing.T) {
	for _, action := range []backend.RemoveAction{backend.RemoveComplete, backend.RemoveDelete} {
		t.Run(string(action), func(t *testing.T) {
			fs, runner, _ := newSweepFSWithViews(t, action, []testView{{Name: "Due", Filter: "status:pending"}},
				task(uuidA, "Do taxes", "home"),
			)
			if errno := fs.Rmdir(context.Background(), "@Due"); errno != 0 {
				t.Fatalf("rmdir custom view: %v", errno)
			}
			if n := mutationCount(runner); n != 0 {
				t.Errorf("custom view rmdir issued %d mutations", n)
			}
			if names := readdirOfRoot(t, fs); contains(names, "@Due") {
				t.Errorf("custom view remained after rmdir: %v", names)
			}
			if _, ok, err := fs.viewStore.Get("Due"); err != nil || ok {
				t.Errorf("view definition remained after rmdir: ok=%v err=%v", ok, err)
			}
		})
	}
}

func TestRecursiveCustomViewRemovalConverges(t *testing.T) {
	for _, action := range []backend.RemoveAction{backend.RemoveComplete, backend.RemoveDelete} {
		t.Run(string(action), func(t *testing.T) {
			fs, _, db := newSweepFSWithViews(t, action, []testView{{Name: "Due", Filter: "status:pending"}},
				task(uuidA, "Do taxes", "home"),
			)
			vn := resolveViewNode(t, fs, "@Due")
			name := naming.FileName("Do taxes", uuidA)
			if names := readdirViewOf(t, vn); !contains(names, name) {
				t.Fatalf("initial view listing missing task: %v", names)
			}
			if errno := vn.Unlink(context.Background(), viewFileName); errno != 0 {
				t.Fatalf("unlink _view: %v", errno)
			}
			if errno := vn.Unlink(context.Background(), name); errno != 0 {
				t.Fatalf("unlink task after clearing _view: %v", errno)
			}
			if errno := fs.Rmdir(context.Background(), "@Due"); errno != 0 {
				t.Fatalf("rmdir custom view: %v", errno)
			}
			if names := readdirOfRoot(t, fs); contains(names, "@Due") {
				t.Errorf("custom view remained after recursive removal: %v", names)
			}
			if action == backend.RemoveComplete && !db.completed[uuidA] {
				t.Error("recursive complete did not complete the task")
			}
			if action == backend.RemoveDelete && !db.deleted[uuidA] {
				t.Error("recursive delete did not delete the task")
			}
		})
	}
}

func TestRootMkdirCreatesEmptyView(t *testing.T) {
	fs, runner, _ := newSweepFS(t)
	if _, errno := fs.createView("@Due"); errno != 0 {
		t.Fatalf("create @Due: %v", errno)
	}
	if names := readdirOfRoot(t, fs); !contains(names, "@Due") {
		t.Fatalf("root listing missing @Due: %v", names)
	}
	vn := resolveViewNode(t, fs, "@Due")
	if names := readdirViewOf(t, vn); len(names) != 0 {
		t.Errorf("new view listing = %v, want no entries", names)
	}
	if _, errno := vn.resolveChild(context.Background(), viewFileName, false); errno != syscall.ENOENT {
		t.Errorf("lookup _view before creation: errno = %v, want ENOENT", errno)
	}
	if query, ok, err := fs.viewStore.Get("Due"); err != nil || !ok || query != "" {
		t.Errorf("new view definition = %q, %v, %v; want empty", query, ok, err)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("mkdir view issued %d task mutations", n)
	}
}

func TestRootMkdirDuplicateViewFails(t *testing.T) {
	fs, _, _ := newSweepFSWithViews(t, backend.RemoveDelete, []testView{{Name: "Due", Filter: "status:any:"}})
	if _, errno := fs.Mkdir(context.Background(), "@Due", 0o755, &fuse.EntryOut{}); errno != syscall.EEXIST {
		t.Errorf("mkdir duplicate view: errno = %v, want EEXIST", errno)
	}
}

func TestRuntimeViewNameValidation(t *testing.T) {
	fs, _, _ := newSweepFS(t)
	for _, name := range []string{
		"@",
		"@.",
		"@..",
		"@Report.a1b2c3d4.md",
		"@" + strings.Repeat("a", 255),
		"@bad\tname",
	} {
		if _, errno := fs.Mkdir(context.Background(), name, 0o755, &fuse.EntryOut{}); errno != syscall.EINVAL {
			t.Errorf("mkdir %q: errno = %v, want EINVAL", name, errno)
		}
	}
	if names := readdirOfRoot(t, fs); len(names) != 2 {
		t.Errorf("invalid view names created entries: %v", names)
	}
}

func TestViewDefinitionFileReadWrite(t *testing.T) {
	fs, _, _ := newSweepFSWithViews(t, backend.RemoveDelete,
		[]testView{{Name: "Due", Filter: ""}},
		task(uuidA, "Do taxes", "home"),
	)
	vn := resolveViewNode(t, fs, "@Due")
	qn, handle := createViewFileNode(t, vn, syscall.O_WRONLY|syscall.O_TRUNC)

	if size := definitionSize(t, qn); size != 0 {
		t.Errorf("new filter size = %d, want 0", size)
	}
	if got := readViewFile(t, qn); got != "" {
		t.Errorf("new filter = %q, want empty", got)
	}
	if errno := qn.Release(context.Background(), handle); errno != 0 {
		t.Fatalf("release empty filter: %v", errno)
	}

	writeViewFile(t, qn, "status:pending")
	if _, errno := vn.resolveChild(context.Background(), viewFileName, false); errno != 0 {
		t.Errorf("lookup _view after write: %v", errno)
	}
	if got := readViewFile(t, qn); got != "status:pending" {
		t.Errorf("read after write = %q", got)
	}
	if size := definitionSize(t, qn); size != len("status:pending") {
		t.Errorf("query size = %d, want %d", size, len("status:pending"))
	}
	if query, ok, err := fs.viewStore.Get("Due"); err != nil || !ok || query != "status:pending" {
		t.Errorf("stored query = %q, %v, %v", query, ok, err)
	}
	if names := readdirViewOf(t, vn); !contains(names, naming.FileName("Do taxes", uuidA)) {
		t.Errorf("view did not use written query: %v", names)
	}
}

func TestNodeAttributesReportMountOwner(t *testing.T) {
	fs, _, _ := newSweepFSWithViews(t, backend.RemoveDelete,
		[]testView{{Name: "Due", Filter: "status:pending"}},
		task(uuidA, "Do taxes", "home"),
	)
	vn := resolveViewNode(t, fs, "@Due")
	var dirOut fuse.AttrOut
	if errno := vn.Getattr(context.Background(), nil, &dirOut); errno != 0 {
		t.Fatalf("view Getattr: %v", errno)
	}
	qn := resolveViewFileNode(t, vn)
	var fileOut fuse.AttrOut
	if errno := qn.Getattr(context.Background(), nil, &fileOut); errno != 0 {
		t.Fatalf("filter Getattr: %v", errno)
	}
	for name, out := range map[string]fuse.AttrOut{"view": dirOut, "filter": fileOut} {
		if out.Uid != uint32(os.Getuid()) || out.Gid != uint32(os.Getgid()) {
			t.Errorf("%s owner = %d:%d, want %d:%d", name, out.Uid, out.Gid, os.Getuid(), os.Getgid())
		}
	}
	if perm := fileOut.Mode & 0o777; perm&0o200 == 0 {
		t.Errorf("filter mode = %o, want owner-writable", perm)
	}
}

func TestViewDefinitionMultiChunkWrite(t *testing.T) {
	fs, _, _ := newSweepFSWithViews(t, backend.RemoveDelete, []testView{{Name: "Due", Filter: ""}})
	qn, fh := createViewFileNode(t, resolveViewNode(t, fs, "@Due"), syscall.O_WRONLY|syscall.O_TRUNC)
	if _, errno := qn.Write(context.Background(), fh, []byte("status:"), 0); errno != 0 {
		t.Fatalf("Write first chunk: %v", errno)
	}
	if _, errno := qn.Write(context.Background(), fh, []byte("pending"), 7); errno != 0 {
		t.Fatalf("Write second chunk: %v", errno)
	}
	if errno := qn.Flush(context.Background(), fh); errno != 0 {
		t.Fatalf("Flush: %v", errno)
	}
	if got := readViewFile(t, qn); got != "status:pending" {
		t.Errorf("multi-chunk filter = %q", got)
	}
}

func TestViewDefinitionTruncateAndTouch(t *testing.T) {
	fs, _, _ := newSweepFSWithViews(t, backend.RemoveDelete, []testView{{Name: "Due", Filter: "status:pending"}})
	vn := resolveViewNode(t, fs, "@Due")
	qn := resolveViewFileNode(t, vn)

	writeViewFile(t, qn, "")
	if got := readViewFile(t, qn); got != "" {
		t.Errorf("truncated filter = %q, want empty", got)
	}
	if names := readdirViewOf(t, vn); !contains(names, viewFileName) {
		t.Errorf("empty filter is not listed: %v", names)
	}

	if errno := vn.Unlink(context.Background(), viewFileName); errno != 0 {
		t.Fatalf("unlink _view: %v", errno)
	}
	if names := readdirViewOf(t, vn); len(names) != 0 {
		t.Errorf("view after _view removal = %v, want no entries", names)
	}

	child, handle, _, errno := vn.createViewFile(context.Background(), viewFileName, syscall.O_WRONLY)
	if errno != 0 {
		t.Fatalf("create _view: %v", errno)
	}
	if errno := child.(*viewFileNode).Release(context.Background(), handle); errno != 0 {
		t.Fatalf("release recreated _view: %v", errno)
	}
	if names := readdirViewOf(t, vn); !contains(names, viewFileName) {
		t.Errorf("touched _view is not listed: %v", names)
	}
}

func TestViewDefinitionSetattrTruncatesOpenHandle(t *testing.T) {
	fs, _, _ := newSweepFSWithViews(t, backend.RemoveDelete, []testView{{Name: "Due", Filter: "status:pending +next"}})
	qn := resolveViewFileNode(t, resolveViewNode(t, fs, "@Due"))
	handle, _, errno := qn.Open(context.Background(), syscall.O_WRONLY)
	if errno != 0 {
		t.Fatalf("Open: %v", errno)
	}
	in := &fuse.SetAttrIn{SetAttrInCommon: fuse.SetAttrInCommon{Valid: fuse.FATTR_SIZE}}
	var out fuse.AttrOut
	if errno := qn.Setattr(context.Background(), handle, in, &out); errno != 0 {
		t.Fatalf("Setattr truncate: %v", errno)
	}
	if _, errno := qn.Write(context.Background(), handle, []byte("status:pending\n"), 0); errno != 0 {
		t.Fatalf("Write: %v", errno)
	}
	if errno := qn.Flush(context.Background(), handle); errno != 0 {
		t.Fatalf("Flush: %v", errno)
	}
	if query, ok, err := fs.viewStore.Get("Due"); err != nil || !ok || query != "status:pending\n" {
		t.Errorf("stored query = %q, %v, %v; want truncated replacement", query, ok, err)
	}
}

func TestViewDefinitionFsyncPersists(t *testing.T) {
	fs, _, _ := newSweepFSWithViews(t, backend.RemoveDelete, []testView{{Name: "Due", Filter: ""}})
	qn, handle := createViewFileNode(t, resolveViewNode(t, fs, "@Due"), syscall.O_WRONLY|syscall.O_TRUNC)
	if _, errno := qn.Write(context.Background(), handle, []byte("status:pending"), 0); errno != 0 {
		t.Fatalf("Write: %v", errno)
	}
	if errno := qn.Fsync(context.Background(), handle, 0); errno != 0 {
		t.Fatalf("Fsync: %v", errno)
	}
	if filter, ok, err := fs.viewStore.Get("Due"); err != nil || !ok || filter != "status:pending" {
		t.Errorf("stored filter after fsync = %q, %v, %v", filter, ok, err)
	}
}

func TestViewDefinitionFlushFailureKeepsPrevious(t *testing.T) {
	store := newViewStore(t)
	if err := store.Put("Due", "status:pending"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	failing := failPutStore{ViewStore: store}
	f := New(backend.NewQuery(&fakeRunner{fn: func([]string) ([]byte, error) { return []byte("[]"), nil }}),
		Config{ViewStore: failing})
	vn := resolveViewNode(t, f, "@Due")
	qn := resolveViewFileNode(t, vn)
	handle, _, errno := qn.Open(context.Background(), syscall.O_WRONLY|syscall.O_TRUNC)
	if errno != 0 {
		t.Fatalf("Open: %v", errno)
	}
	if _, errno := qn.Write(context.Background(), handle, []byte("status:completed"), 0); errno != 0 {
		t.Fatalf("Write: %v", errno)
	}
	if errno := qn.Flush(context.Background(), handle); errno != syscall.EIO {
		t.Fatalf("Flush = %v, want EIO", errno)
	}
	if query, ok, err := store.Get("Due"); err != nil || !ok || query != "status:pending" {
		t.Errorf("stored query after failed flush = %q, %v, %v", query, ok, err)
	}
}

func TestViewRemoveDefinitionClearsButKeepsView(t *testing.T) {
	fs, runner, _ := newSweepFSWithViews(t, backend.RemoveDelete,
		[]testView{{Name: "Due", Filter: "status:pending"}},
		task(uuidA, "Do taxes", "home"),
	)
	vn := resolveViewNode(t, fs, "@Due")
	if errno := vn.Unlink(context.Background(), viewFileName); errno != 0 {
		t.Fatalf("unlink _view: %v", errno)
	}
	if names := readdirOfRoot(t, fs); !contains(names, "@Due") {
		t.Errorf("view disappeared after _view removal: %v", names)
	}
	if names := readdirViewOf(t, vn); len(names) != 0 {
		t.Errorf("view after _view removal = %v, want no entries", names)
	}
	if _, errno := vn.resolveChild(context.Background(), viewFileName, false); errno != syscall.ENOENT {
		t.Errorf("lookup _view after removal: errno = %v, want ENOENT", errno)
	}
	if query, ok, err := fs.viewStore.Get("Due"); err != nil || !ok || query != "" {
		t.Errorf("stored query = %q, %v, %v; want empty definition", query, ok, err)
	}
	if n := mutationCount(runner); n != 0 {
		t.Errorf("_view removal issued %d task mutations", n)
	}
}

func TestViewInvalidDefinitionReturnsEIO(t *testing.T) {
	store := newViewStore(t)
	if err := store.Put("Broken", "("); err != nil {
		t.Fatalf("seed: %v", err)
	}
	runner := &fakeRunner{fn: func([]string) ([]byte, error) { return nil, errors.New("bad filter") }}
	fs := New(backend.NewQuery(runner), Config{ViewStore: store})
	vn := resolveViewNode(t, fs, "@Broken")
	if _, errno := vn.Readdir(context.Background()); errno != syscall.EIO {
		t.Errorf("Readdir: errno = %v, want EIO", errno)
	}
	if _, errno := vn.Lookup(context.Background(), naming.FileName("Do taxes", uuidA), &fuse.EntryOut{}); errno != syscall.EIO {
		t.Errorf("Lookup: errno = %v, want EIO", errno)
	}
	if errno := vn.Unlink(context.Background(), naming.FileName("Do taxes", uuidA)); errno != syscall.EIO {
		t.Errorf("Unlink: errno = %v, want EIO", errno)
	}
	if errno := fs.Rmdir(context.Background(), "@Broken"); errno != 0 {
		t.Errorf("Rmdir should delete the definition without querying: %v", errno)
	}
}

func TestViewAllowsOnlyDefinitionCreation(t *testing.T) {
	fs, _, _ := newSweepFSWithViews(t, backend.RemoveDelete, []testView{{Name: "Due", Filter: ""}})
	vn := resolveViewNode(t, fs, "@Due")
	if _, _, _, errno := vn.createViewFile(context.Background(), "New task", syscall.O_WRONLY); errno != syscall.EACCES {
		t.Errorf("create non-query: errno = %v, want EACCES", errno)
	}
	if _, _, _, errno := vn.createViewFile(context.Background(), viewFileName, syscall.O_WRONLY); errno != 0 {
		t.Errorf("create _view: errno = %v", errno)
	}
	if _, errno := vn.Mkdir(context.Background(), "sub", 0o755, &fuse.EntryOut{}); errno != syscall.EACCES {
		t.Errorf("Mkdir: errno = %v, want EACCES", errno)
	}
}

func TestRmdirAtPrefixedProjectWithoutViewRemovesTasks(t *testing.T) {
	fs, _, db := newSweepFS(t, task(uuidA, "At project", "@Due"))
	if errno := fs.Rmdir(context.Background(), "@Due"); errno != 0 {
		t.Fatalf("rmdir @Due project: %v", errno)
	}
	if !db.deleted[uuidA] {
		t.Error("removing an @-prefixed project did not apply the removal action")
	}
}

func TestViewShadowsAndRevealsProject(t *testing.T) {
	fs, _, _ := newSweepFSWithViews(t, backend.RemoveDelete,
		[]testView{{Name: "Due", Filter: "status:pending"}},
		task(uuidA, "At project", "@Due"),
	)
	child, errno := fs.resolveChild("@Due")
	if errno != 0 {
		t.Fatalf("lookup @Due: %v", errno)
	}
	if _, ok := child.(*viewNode); !ok {
		t.Fatalf("lookup @Due = %T, want view", child)
	}
	if names := readdirOfRoot(t, fs); countName(names, "@Due") != 1 {
		t.Errorf("root listing must contain one @Due: %v", names)
	}

	if errno := fs.Rmdir(context.Background(), "@Due"); errno != 0 {
		t.Fatalf("rmdir @Due: %v", errno)
	}
	child, errno = fs.resolveChild("@Due")
	if errno != 0 {
		t.Fatalf("lookup @Due after view removal: %v", errno)
	}
	dir, ok := child.(*dirNode)
	if !ok || dir.filter.Project != "@Due" {
		t.Fatalf("lookup @Due after view removal = %T / %+v, want project", child, child)
	}
}

func resolveViewFileNode(t *testing.T, vn *viewNode) *viewFileNode {
	t.Helper()
	child, errno := vn.resolveChild(context.Background(), viewFileName, false)
	if errno != 0 {
		t.Fatalf("lookup _view: %v", errno)
	}
	qn, ok := child.(*viewFileNode)
	if !ok {
		t.Fatalf("lookup _view returned %T", child)
	}
	return qn
}

func createViewFileNode(t *testing.T, vn *viewNode, flags uint32) (*viewFileNode, fs.FileHandle) {
	t.Helper()
	child, handle, _, errno := vn.createViewFile(context.Background(), viewFileName, flags)
	if errno != 0 {
		t.Fatalf("create _view: %v", errno)
	}
	qn, ok := child.(*viewFileNode)
	if !ok {
		t.Fatalf("create _view returned %T", child)
	}
	return qn, handle
}

func definitionSize(t *testing.T, qn *viewFileNode) int {
	t.Helper()
	var out fuse.AttrOut
	if errno := qn.Getattr(context.Background(), nil, &out); errno != 0 {
		t.Fatalf("query Getattr: %v", errno)
	}
	return int(out.Size)
}

func readViewFile(t *testing.T, qn *viewFileNode) string {
	t.Helper()
	handle, _, errno := qn.Open(context.Background(), syscall.O_RDONLY)
	if errno != 0 {
		t.Fatalf("query Open: %v", errno)
	}
	buf := make([]byte, 256)
	result, errno := qn.Read(context.Background(), handle, buf, 0)
	if errno != 0 {
		t.Fatalf("query Read: %v", errno)
	}
	data, status := result.Bytes(buf)
	if status != fuse.OK {
		t.Fatalf("query Read status: %v", status)
	}
	return string(data)
}

func writeViewFile(t *testing.T, qn *viewFileNode, query string) {
	t.Helper()
	handle, _, errno := qn.Open(context.Background(), syscall.O_WRONLY|syscall.O_TRUNC)
	if errno != 0 {
		t.Fatalf("query Open: %v", errno)
	}
	if _, errno := qn.Write(context.Background(), handle, []byte(query), 0); errno != 0 {
		t.Fatalf("query Write: %v", errno)
	}
	if errno := qn.Flush(context.Background(), handle); errno != 0 {
		t.Fatalf("query Flush: %v", errno)
	}
	if errno := qn.Release(context.Background(), handle); errno != 0 {
		t.Fatalf("query Release: %v", errno)
	}
}

func countName(names []string, want string) int {
	count := 0
	for _, name := range names {
		if name == want {
			count++
		}
	}
	return count
}

type failPutStore struct {
	ViewStore
}

func (s failPutStore) Put(name, query string) error {
	return errors.New("put failed")
}
