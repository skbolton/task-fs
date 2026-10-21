package vfs

import (
	"context"
	"encoding/json"
	"strings"
	"syscall"
	"testing"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"task-fs/internal/backend"
	"task-fs/internal/naming"
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
func newTestFS(pending, completed []backend.Task, projects, tags, allTags []string) (*FS, *fakeRunner) {
	runner := &fakeRunner{fn: func(args []string) ([]byte, error) {
		if hasArg(args, "_tags") {
			return []byte(strings.Join(allTags, "\n")), nil
		}
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
	fs := New(backend.NewQuery(runner))
	fs.SetVirtualTags(backend.VirtualTagsFromDiscovery(allTags))
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
		},
		nil,
		[]string{"home.clean-garage", "finances.2026-taxes"},
		[]string{"next"},
		[]string{"ACTIVE", "COMPLETED", "DUE"},
	)
	stream, errno := fs.Readdir(context.Background())
	if errno != 0 {
		t.Fatalf("Readdir: %v", errno)
	}
	names := readdirNames(t, stream)
	for _, want := range []string{"home", "finances", "+next", ".+COMPLETED", ".+DUE"} {
		if !contains(names, want) {
			t.Errorf("root listing missing %q; got %v", want, names)
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

func TestLookupStrictENOENT(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{task(uuidA, "Clean the garage", "home.clean-garage", "next")},
		nil,
		[]string{"home.clean-garage"},
		[]string{"next"},
		[]string{"COMPLETED"},
	)
	if _, errno := fs.resolveChild("homi"); errno != syscall.ENOENT {
		t.Errorf("lookup typo'd segment: errno = %v, want ENOENT", errno)
	}
	if _, errno := fs.resolveChild("+COMPLETED"); errno != syscall.ENOENT {
		t.Errorf("bare +COMPLETED must not resolve as virtual tag: errno = %v", errno)
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
		[]backend.Task{task(uuidB, "Chore", "home")},
		[]backend.Task{completed},
		[]string{"home"},
		[]string{},
		[]string{"COMPLETED"},
	)
	completedDir, errno := fs.resolveChild(".+COMPLETED")
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
		[]backend.Task{task(uuidA, "Do taxes", "finances")},
		nil,
		[]string{"finances"},
		[]string{},
		[]string{"COMPLETED"},
	)
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
		[]backend.Task{task(uuidB, "Old", "home")},
		nil,
		[]string{"home"},
		[]string{},
		[]string{"COMPLETED"},
	)
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
	fs, runner := newTestFS(nil, nil, []string{"home"}, []string{}, []string{"COMPLETED", "DUE"})
	due, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	dueNode, errno := due.(*dirNode).resolveChild(".+DUE")
	if errno != 0 {
		t.Fatalf("lookup +DUE: %v", errno)
	}
	dueNode.(*dirNode).createChild("Pick up Milk")
	var addCmd []string
	for _, cmd := range runner.log {
		if hasArg(cmd, "add") {
			addCmd = cmd
		}
	}
	if addCmd == nil {
		t.Fatal("no add command issued")
	}
	if hasArg(addCmd, ".+DUE") {
		t.Errorf("add command carries a virtual tag: %v", addCmd)
	}
	if !hasArg(addCmd, "project:home") {
		t.Errorf("add command missing write filter: %v", addCmd)
	}
}

func TestWriteRejection(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{task(uuidA, "Do taxes", "finances")},
		nil,
		[]string{"finances"},
		[]string{},
		[]string{"COMPLETED"},
	)
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
		[]backend.Task{task(uuidA, "Do taxes", "")},
		nil,
		[]string{"finances"},
		[]string{},
		[]string{"COMPLETED"},
	)
	name := naming.FileName("Do taxes", uuidA)
	if errno := fs.Unlink(context.Background(), name); errno != 0 {
		t.Fatalf("unlink: %v", errno)
	}
	delCmd := runner.log[len(runner.log)-1]
	if len(delCmd) != 3 || !hasArg(delCmd, uuidA) || !hasArg(delCmd, "delete") {
		t.Errorf("delete command = %v", delCmd)
	}
}

func TestRenameIntoCompleted(t *testing.T) {
	fs, runner := newTestFS(
		[]backend.Task{task(uuidA, "Do taxes", "")},
		nil,
		[]string{"finances"},
		[]string{},
		[]string{"COMPLETED", "DUE"},
	)
	name := naming.FileName("Do taxes", uuidA)
	completed, errno := fs.resolveChild(".+COMPLETED")
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

func TestRenameOutOfCompletedReopens(t *testing.T) {
	completedTask := backend.Task{UUID: uuidA, Description: "Do taxes", Project: "finances", Status: "completed"}
	fs, runner := newTestFS(
		nil,
		[]backend.Task{completedTask},
		[]string{"finances"},
		[]string{},
		[]string{"COMPLETED", "DUE"},
	)
	name := naming.FileName("Do taxes", uuidA)
	completed, errno := fs.resolveChild(".+COMPLETED")
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

func TestRenameVirtualOnlyDestinationRejected(t *testing.T) {
	fs, runner := newTestFS(
		[]backend.Task{task(uuidA, "Chore", "home")},
		nil,
		[]string{"home"},
		[]string{},
		[]string{"COMPLETED", "DUE", "OVERDUE"},
	)
	overdue, errno := fs.resolveChild(".+OVERDUE")
	if errno != 0 {
		t.Fatalf("lookup .+OVERDUE: %v", errno)
	}
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	name := naming.FileName("Chore", uuidA)
	if errno := home.(*dirNode).Rename(context.Background(), name, overdue.(*dirNode), name, 0); errno != syscall.ENOTSUP {
		t.Errorf("rename between virtual views: errno = %v, want ENOTSUP", errno)
	}
	for _, cmd := range runner.log {
		if hasArg(cmd, "modify") || hasArg(cmd, "done") {
			t.Errorf("rejected rename issued commands: %v", cmd)
		}
	}
}

func TestRenameDescriptionRejected(t *testing.T) {
	fs, runner := newTestFS(
		[]backend.Task{task(uuidA, "Old", "home")},
		nil,
		[]string{"home"},
		[]string{},
		[]string{"COMPLETED"},
	)
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
				[]backend.Task{task(uuidA, "Fix leak", tt.taskProject)},
				nil,
				[]string{"inbox", "finances", "finances.2026-taxes"},
				[]string{"next", "urgent"},
				[]string{"COMPLETED"},
			)
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
		[]backend.Task{task(uuidA, "Fix leak", "inbox")},
		nil,
		[]string{"inbox"},
		[]string{},
		[]string{"COMPLETED"},
	)
	if _, errno := fs.resolveChild("brandnew"); errno != syscall.ENOENT {
		t.Errorf("lookup of undeclared project: errno = %v, want ENOENT", errno)
	}
}

func TestMkdirDeclarations(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{task(uuidA, "Chore", "home", "next")},
		nil,
		[]string{"home"},
		[]string{"next"},
		[]string{"COMPLETED"},
	)
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
	if _, errno := homeNode.declareChild(".+DUE"); errno != syscall.EACCES {
		t.Errorf("mkdir .+DUE: errno = %v, want EACCES", errno)
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

func TestRmdirInUse(t *testing.T) {
	runner := &fakeRunner{fn: func(args []string) ([]byte, error) {
		if hasArg(args, "_unique") {
			if hasArg(args, "project") {
				return []byte("home"), nil
			}
			return []byte("next"), nil
		}
		if hasArg(args, "export") && hasArg(args, "+urgent") {
			return taskJSON(task(uuidB, "Urgent thing", "home", "urgent")), nil
		}
		if hasArg(args, "export") {
			return taskJSON(task(uuidA, "Chore", "home", "next")), nil
		}
		return nil, nil
	}}
	fs := New(backend.NewQuery(runner))
	fs.SetVirtualTags([]string{"COMPLETED"})
	home, errno := fs.resolveChild("home")
	if errno != 0 {
		t.Fatalf("lookup home: %v", errno)
	}
	homeNode := home.(*dirNode)
	if _, errno := homeNode.declareChild("+urgent"); errno != 0 {
		t.Fatalf("mkdir +urgent: %v", errno)
	}
	if errno := homeNode.Rmdir(context.Background(), "+urgent"); errno != syscall.ENOTEMPTY {
		t.Errorf("rmdir in-use tag: errno = %v, want ENOTEMPTY", errno)
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
	fs := New(backend.NewQuery(runner))
	fs.SetVirtualTags([]string{"COMPLETED"})
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
		[]backend.Task{task(uuidA, "Do taxes", "")},
		nil,
		[]string{"finances"},
		[]string{},
		[]string{"COMPLETED"},
	)
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
	fs, runner := newTestFS(
		[]backend.Task{task(uuidA, "Chore", "home")},
		nil,
		[]string{"home"},
		[]string{},
		[]string{"COMPLETED", "DUE"},
	)
	due, errno := fs.resolveChild(".+DUE")
	if errno != 0 {
		t.Fatalf("lookup +DUE: %v", errno)
	}
	name := naming.FileName("Chore", uuidA)
	if errno := due.(*dirNode).Rename(context.Background(), name, fs, name, 0); errno != 0 {
		t.Fatalf("rename out of +DUE: %v", errno)
	}
	for _, cmd := range runner.log {
		if hasArg(cmd, "modify") && hasArg(cmd, "+DUE") {
			t.Errorf("modify command carries a virtual tag: %v", cmd)
		}
	}
}

func TestExactProjectPlacement(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{
			task(uuidA, "Less nested project", "this.is"),
			task(uuidB, "Test out deep projects", "this.is.a.test"),
		},
		nil,
		[]string{"this.is", "this.is.a.test"},
		[]string{},
		[]string{"COMPLETED"},
	)
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
		},
		nil,
		[]string{"home", "work"},
		[]string{"next"},
		[]string{"COMPLETED"},
	)
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
		[]backend.Task{task(uuidA, "Chore", "home", "next")},
		nil,
		[]string{"home"},
		[]string{"next"},
		[]string{"COMPLETED"},
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

func TestSelfVirtualTagNotListed(t *testing.T) {
	fs, _ := newTestFS(
		[]backend.Task{task(uuidA, "Old chore", "home")},
		[]backend.Task{task(uuidA, "Old chore", "home")},
		[]string{"home"},
		[]string{},
		[]string{"COMPLETED", "DUE"},
	)
	completed, errno := fs.resolveChild(".+COMPLETED")
	if errno != 0 {
		t.Fatalf("lookup .+COMPLETED: %v", errno)
	}
	names, errno := readdirOf(t, completed.(*dirNode))
	if errno != 0 {
		t.Fatalf("Readdir .+COMPLETED: %v", errno)
	}
	if contains(names, ".+COMPLETED") {
		t.Errorf("virtual-tag view lists itself: %v", names)
	}
	if !contains(names, ".+DUE") {
		t.Errorf("other virtual tags still listed: %v", names)
	}
}

func TestDeclaredTagNotListedInsideItself(t *testing.T) {
	runner := &fakeRunner{fn: func(args []string) ([]byte, error) {
		if hasArg(args, "export") {
			return taskJSON(), nil
		}
		return nil, nil
	}}
	fs := New(backend.NewQuery(runner))
	fs.SetVirtualTags([]string{"COMPLETED"})
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
		},
		nil,
		[]string{"this.is"},
		[]string{},
		[]string{"COMPLETED"},
	)
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
		},
		nil,
		[]string{"home.clean-garage"},
		[]string{"next"},
		[]string{"COMPLETED"},
	)
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
