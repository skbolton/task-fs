package vfs

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"task-fs/internal/backend"
	"task-fs/internal/naming"
)

const uuidXattr = "user.taskwarrior.uuid"

// FS is the root of the mounted filesystem.
type FS struct {
	dirNode
	query        *backend.Query
	virtualTags  []string
	hidden       bool
	removeAction backend.RemoveAction

	mu       sync.Mutex
	declared map[string]map[string]declaredSegment
}

// Config fixes the mount-time virtual-tag selection, presentation,
// and removal action.
type Config struct {
	VirtualTags  []string
	Hidden       bool
	RemoveAction backend.RemoveAction
}

// declaredSegment remembers the filter a mkdir declared, so removing a
// directory can purge the declarations made under it.
type declaredSegment struct {
	filter backend.Filter
}

func New(query *backend.Query, cfg Config) *FS {
	root := &FS{
		dirNode:      dirNode{root: nil},
		query:        query,
		virtualTags:  cfg.VirtualTags,
		hidden:       cfg.Hidden,
		removeAction: cfg.RemoveAction,
		declared:     map[string]map[string]declaredSegment{},
	}
	root.dirNode.root = root
	return root
}

func (f *FS) isVirtualTag(name string) bool {
	for _, vt := range f.virtualTags {
		if vt == name {
			return true
		}
	}
	return false
}

// virtualEntryName is the listed form of a virtual tag: dot-prefixed
// and hidden, or plus-prefixed and regular.
func (f *FS) virtualEntryName(vt string) string {
	if f.hidden {
		return ".+" + vt
	}
	return "+" + vt
}

func (f *FS) declare(parent backend.Filter, segment string, child backend.Filter) {
	key := canonicalKey(parent)
	f.mu.Lock()
	defer f.mu.Unlock()
	set, ok := f.declared[key]
	if !ok {
		set = map[string]declaredSegment{}
		f.declared[key] = set
	}
	set[segment] = declaredSegment{filter: child}
}

// purgeDeclarations drops declarations made at or under a removed
// directory: those whose project sits in the removed project subtree
// and whose tags include the removed filter's. Without this, Readdir
// would keep listing ghost segments of the removed subtree for the
// rest of the mount session.
func (f *FS) purgeDeclarations(removed backend.Filter) {
	if removed.Project == "" {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for key, set := range f.declared {
		for seg, d := range set {
			if extendsFilter(d.filter, removed) {
				delete(set, seg)
			}
		}
		if len(set) == 0 {
			delete(f.declared, key)
		}
	}
}

func extendsFilter(d, base backend.Filter) bool {
	if d.Project != base.Project && !strings.HasPrefix(d.Project, base.Project+".") {
		return false
	}
	for _, t := range base.Tags {
		if !contains(d.Tags, t) {
			return false
		}
	}
	return true
}

func (f *FS) undeclare(filter backend.Filter, segment string) {
	key := canonicalKey(filter)
	f.mu.Lock()
	defer f.mu.Unlock()
	if set, ok := f.declared[key]; ok {
		delete(set, segment)
	}
}

func (f *FS) isDeclared(filter backend.Filter, segment string) bool {
	key := canonicalKey(filter)
	f.mu.Lock()
	defer f.mu.Unlock()
	set, ok := f.declared[key]
	_, exists := set[segment]
	return ok && exists
}

func (f *FS) declaredSegments(filter backend.Filter) []string {
	key := canonicalKey(filter)
	f.mu.Lock()
	defer f.mu.Unlock()
	var segments []string
	for seg := range f.declared[key] {
		segments = append(segments, seg)
	}
	sort.Strings(segments)
	return segments
}

// canonicalKey identifies a filter state so order-free paths that
// resolve to the same filter share declarations.
func canonicalKey(f backend.Filter) string {
	tags := append([]string(nil), f.Tags...)
	sort.Strings(tags)
	vts := append([]string(nil), f.VirtualTags...)
	sort.Strings(vts)
	return fmt.Sprintf("%s|%s|%s", f.Project, strings.Join(tags, ","), strings.Join(vts, ","))
}

// dirNode is a directory holding an accumulated filter.
type dirNode struct {
	fs.Inode
	root   *FS
	filter backend.Filter
}

func (n *dirNode) stableAttr() fs.StableAttr {
	return fs.StableAttr{
		Mode: fuse.S_IFDIR,
		Ino:  hashFilter(n.filter),
	}
}

func hashFilter(f backend.Filter) uint64 {
	h := fnv.New64a()
	h.Write([]byte(canonicalKey(f)))
	return h.Sum64()
}

var _ = (fs.NodeLookuper)((*dirNode)(nil))

func (n *dirNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	child, errno := n.resolveChild(name)
	if errno != 0 {
		return nil, errno
	}
	inode := n.NewInode(ctx, child, child.(stableNoder).stableAttr())
	return inode, 0
}

var _ = (fs.NodeReaddirer)((*dirNode)(nil))

func (n *dirNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	seen := map[string]bool{}
	var entries []fuse.DirEntry
	add := func(name string, mode uint32) {
		if !seen[name] {
			seen[name] = true
			entries = append(entries, fuse.DirEntry{Name: name, Mode: mode})
		}
	}
	projects, err := n.root.query.UniqueProjects(n.filter)
	if err != nil {
		return nil, syscall.EIO
	}
	for _, seg := range backend.ChildProjects(n.filter.Project, projects) {
		add(seg, fuse.S_IFDIR)
	}
	tagCombos, err := n.root.query.UniqueTags(n.filter)
	if err != nil {
		return nil, syscall.EIO
	}
	for _, tag := range backend.ChildTags(tagCombos) {
		if contains(n.filter.Tags, tag) {
			continue
		}
		add("+"+tag, fuse.S_IFDIR)
	}
	// Virtual-tag views do not nest: stacking derived filters makes
	// the tree combinatorial, and a recursive walk of a project would
	// never finish. One derived filter per path is enough for the
	// read-only views.
	if len(n.filter.VirtualTags) == 0 {
		for _, vt := range n.root.virtualTags {
			add(n.root.virtualEntryName(vt), fuse.S_IFDIR)
		}
	}
	tasks, err := n.listedTasks(n.filter)
	if err != nil {
		return nil, syscall.EIO
	}
	for _, t := range tasks {
		add(naming.FileName(t.Description, t.UUID), fuse.S_IFREG)
	}
	for _, seg := range n.root.declaredSegments(n.filter) {
		if strings.HasPrefix(seg, "+") && contains(n.filter.Tags, seg[1:]) {
			continue
		}
		add(seg, fuse.S_IFDIR)
	}
	return fs.NewListDirStream(entries), 0
}

// listedTasks applies the exact-placement rule: directories with a
// project segment list only tasks whose project equals it exactly. The
// bare root lists only project-less tasks (the inbox). Tag and
// virtual-tag views without a project segment list every matching task
// regardless of project depth.
func (n *dirNode) listedTasks(f backend.Filter) ([]backend.Task, error) {
	tasks, err := n.root.query.Export(f)
	if err != nil {
		return nil, err
	}
	if f.Project != "" {
		var exact []backend.Task
		for _, t := range tasks {
			if t.Project == f.Project {
				exact = append(exact, t)
			}
		}
		return exact, nil
	}
	if len(f.Tags) == 0 && len(f.VirtualTags) == 0 {
		var inbox []backend.Task
		for _, t := range tasks {
			if t.Project == "" {
				inbox = append(inbox, t)
			}
		}
		return inbox, nil
	}
	return tasks, nil
}

var _ = (fs.NodeGetattrer)((*dirNode)(nil))

func (n *dirNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = fuse.S_IFDIR | 0755
	out.Size = 0
	out.Nlink = 1
	return 0
}

// resolveChild maps a directory entry name to a node, applying the
// strict known-segments rule: anything not derivable from query
// results or mkdir declarations is ENOENT.
func (n *dirNode) resolveChild(name string) (fs.InodeEmbedder, syscall.Errno) {
	if naming.IsMintedName(name) {
		tasks, err := n.listedTasks(n.filter)
		if err != nil {
			return nil, syscall.EIO
		}
		for _, t := range tasks {
			if naming.FileName(t.Description, t.UUID) == name {
				fn := &fileNode{root: n.root, uuid: t.UUID, viewKey: canonicalKey(n.filter)}
				return fn, 0
			}
		}
		return nil, syscall.ENOENT
	}

	var child backend.Filter
	var err error
	switch {
	case strings.HasPrefix(name, ".+"):
		seg := name[2:]
		if !n.root.isVirtualTag(seg) {
			return nil, syscall.ENOENT
		}
		child, err = n.filter.WithVirtualSegment(name)
	case strings.HasPrefix(name, "."):
		return nil, syscall.ENOENT
	case strings.HasPrefix(name, "+"):
		seg := name[1:]
		if n.root.isVirtualTag(seg) {
			child, err = n.filter.WithVirtualSegment(name)
			break
		}
		if !n.root.isDeclared(n.filter, name) {
			tags, err := n.root.query.UniqueTags(n.filter)
			if err != nil {
				return nil, syscall.EIO
			}
			if !contains(backend.ChildTags(tags), seg) {
				return nil, syscall.ENOENT
			}
		}
		child, err = n.filter.WithSegment(name)
	default:
		if !n.root.isDeclared(n.filter, name) {
			projects, err := n.root.query.UniqueProjects(n.filter)
			if err != nil {
				return nil, syscall.EIO
			}
			if !contains(backend.ChildProjects(n.filter.Project, projects), name) {
				return nil, syscall.ENOENT
			}
		}
		child, err = n.filter.WithSegment(name)
	}
	if err != nil {
		return nil, syscall.ENOENT
	}
	return &dirNode{root: n.root, filter: child}, 0
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// fileNode is one task presented as an empty read-only placeholder.
// viewKey records which filter view minted this inode: the same task
// appears in overlapping views and must carry distinct inode numbers
// per view, or mv refuses the move as "same file" before the
// filesystem can apply the tag delta.
type fileNode struct {
	fs.Inode
	root    *FS
	uuid    string
	viewKey string
}

func (n *fileNode) stableAttr() fs.StableAttr {
	h := fnv.New64a()
	h.Write([]byte(n.viewKey + "|" + n.uuid))
	return fs.StableAttr{Mode: fuse.S_IFREG, Ino: h.Sum64()}
}

var _ = (fs.NodeGetattrer)((*fileNode)(nil))

func (n *fileNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = fuse.S_IFREG | 0444
	out.Size = 0
	out.Nlink = 1
	return 0
}

var _ = (fs.NodeOpener)((*fileNode)(nil))

func (n *fileNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	if flags&syscall.O_ACCMODE != syscall.O_RDONLY {
		return nil, 0, syscall.EACCES
	}
	return nil, 0, 0
}

var _ = (fs.NodeReader)((*fileNode)(nil))

func (n *fileNode) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	return fuse.ReadResultData(dest[:0]), 0
}

var _ = (fs.NodeSetattrer)((*fileNode)(nil))

// Setattr accepts time updates (touch) but rejects size changes: task
// file content is owned by Taskwarrior in v1.
func (n *fileNode) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if sz, ok := in.GetSize(); ok && sz != 0 {
		return syscall.EACCES
	}
	if mtime, ok := in.GetMTime(); ok {
		out.SetTimes(nil, &mtime, nil)
	}
	out.Mode = fuse.S_IFREG | 0444
	out.Size = 0
	return 0
}

var _ = (fs.NodeSetattrer)((*dirNode)(nil))

func (n *dirNode) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if mtime, ok := in.GetMTime(); ok {
		out.SetTimes(nil, &mtime, nil)
	}
	out.Mode = fuse.S_IFDIR | 0755
	return 0
}

var _ = (fs.NodeGetxattrer)((*fileNode)(nil))

func (n *fileNode) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	if attr != uuidXattr {
		return 0, syscall.ENODATA
	}
	if len(dest) < len(n.uuid) {
		return uint32(len(n.uuid)), syscall.ERANGE
	}
	copy(dest, n.uuid)
	return uint32(len(n.uuid)), 0
}

var _ = (fs.NodeListxattrer)((*fileNode)(nil))

func (n *fileNode) Listxattr(ctx context.Context, dest []byte) (uint32, syscall.Errno) {
	full := uuidXattr + "\x00"
	if len(dest) < len(full) {
		return uint32(len(full)), syscall.ERANGE
	}
	copy(dest, full)
	return uint32(len(full)), 0
}

var _ = (fs.NodeSetxattrer)((*fileNode)(nil))

func (n *fileNode) Setxattr(ctx context.Context, attr string, data []byte, flags uint32) syscall.Errno {
	return syscall.EACCES
}

var _ = (fs.NodeCreater)((*dirNode)(nil))

// Create treats a created file as a task-creation gesture. Names
// matching the filesystem's minted-name pattern are rejected to keep
// minted names reserved.
func (n *dirNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	child, errno := n.createChild(name)
	if errno != 0 {
		return nil, nil, 0, errno
	}
	return n.NewInode(ctx, child, child.(stableNoder).stableAttr()), nil, 0, 0
}

func (n *dirNode) createChild(name string) (fs.InodeEmbedder, syscall.Errno) {
	if naming.IsMintedName(name) {
		return nil, syscall.EPERM
	}
	if err := n.root.query.Add(n.filter, name); err != nil {
		return nil, syscall.EIO
	}
	// The new task may not match this directory's read filter (virtual
	// tags are dropped from the write filter), in which case the file
	// appears one level up or in a sibling view; the gesture still
	// succeeds and the inode resolves under the write filter.
	for _, f := range []backend.Filter{n.filter, n.filter.WriteFilter()} {
		tasks, err := n.listedTasks(f)
		if err != nil {
			return nil, syscall.EIO
		}
		for _, t := range tasks {
			if t.Description == name {
				return &fileNode{root: n.root, uuid: t.UUID, viewKey: canonicalKey(n.filter)}, 0
			}
		}
	}
	return nil, syscall.EIO
}

var _ = (fs.NodeUnlinker)((*dirNode)(nil))

func (n *dirNode) Unlink(ctx context.Context, name string) syscall.Errno {
	if !naming.IsMintedName(name) {
		return syscall.ENOENT
	}
	t, errno := n.resolveFileTask(name)
	if errno != 0 {
		return errno
	}
	// A deleted task's file only appears in the .+DELETED view; it is a
	// tombstone, and removing it again must not re-invoke delete. A
	// completed task is likewise already done, and Taskwarrior refuses
	// to complete it again.
	if t.Status == "deleted" {
		return 0
	}
	if n.root.removeAction == backend.RemoveComplete && t.Status == "completed" {
		return 0
	}
	if err := n.root.query.Remove(t.UUID, n.root.removeAction); err != nil {
		return syscall.EIO
	}
	return 0
}

func (n *dirNode) resolveFileTask(name string) (backend.Task, syscall.Errno) {
	tasks, err := n.listedTasks(n.filter)
	if err != nil {
		return backend.Task{}, syscall.EIO
	}
	for _, t := range tasks {
		if naming.FileName(t.Description, t.UUID) == name {
			return t, 0
		}
	}
	return backend.Task{}, syscall.ENOENT
}

var _ = (fs.NodeMkdirer)((*dirNode)(nil))

// Mkdir declares a tag or project namespace in daemon memory for the
// mount session.
func (n *dirNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	child, errno := n.declareChild(name)
	if errno != 0 {
		return nil, errno
	}
	return n.NewInode(ctx, child, child.stableAttr()), 0
}

func (n *dirNode) declareChild(name string) (*dirNode, syscall.Errno) {
	if strings.HasPrefix(name, ".") {
		return nil, syscall.EACCES
	}
	if strings.HasPrefix(name, "+") && len(name) > 1 && isAllUppercase(name[1:]) {
		return nil, syscall.EACCES
	}
	if naming.IsMintedName(name) {
		return nil, syscall.EPERM
	}
	if _, errno := n.resolveChild(name); errno == 0 {
		return nil, syscall.EEXIST
	}
	child, err := n.filter.WithSegment(name)
	if err != nil {
		return nil, syscall.EINVAL
	}
	n.root.declare(n.filter, name, child)
	return &dirNode{root: n.root, filter: child}, 0
}

var _ = (fs.NodeRmdirer)((*dirNode)(nil))

// Rmdir interprets removal as a deletion gesture. A virtual-tag view
// is never destructive: it is removable only once no non-deleted task
// matches it, which lets a recursive rm finish after unlinking the
// task files inside. Any other resolvable or declared directory
// deletes every matching task in its filter scope, then removes the
// directory together with any declarations made under it.
func (n *dirNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	if strings.HasPrefix(name, ".") {
		if !strings.HasPrefix(name, ".+") || !n.root.isVirtualTag(name[2:]) {
			return syscall.ENOENT
		}
		return n.rmdirVirtualView(name)
	}
	if strings.HasPrefix(name, "+") && n.root.isVirtualTag(name[1:]) {
		return n.rmdirVirtualView(name)
	}
	child, err := n.filter.WithSegment(name)
	if err != nil {
		return syscall.EINVAL
	}
	if !n.root.isDeclared(n.filter, name) {
		if _, errno := n.resolveChild(name); errno != 0 {
			exists, err := n.root.query.HasAnyTask(child.WriteFilter())
			if err != nil {
				return syscall.EIO
			}
			if !exists {
				return syscall.ENOENT
			}
		}
	}
	tasks, err := n.root.query.SweepTasks(child.WriteFilter(), n.root.removeAction)
	if err != nil {
		return syscall.EIO
	}
	for _, t := range tasks {
		if err := n.root.query.Remove(t.UUID, n.root.removeAction); err != nil {
			return syscall.EIO
		}
	}
	n.root.purgeDeclarations(child)
	n.root.undeclare(n.filter, name)
	return 0
}

// rmdirVirtualView removes a virtual-tag view without changing tasks:
// it succeeds only once the view holds no task the configured removal
// action would change.
func (n *dirNode) rmdirVirtualView(name string) syscall.Errno {
	child, err := n.filter.WithVirtualSegment(name)
	if err != nil {
		return syscall.EINVAL
	}
	tasks, err := n.root.query.SweepTasks(child, n.root.removeAction)
	if err != nil {
		return syscall.EIO
	}
	if len(tasks) > 0 {
		return syscall.ENOTEMPTY
	}
	return 0
}

var _ = (fs.NodeRenamer)((*dirNode)(nil))

// Rename interprets gestures: into +COMPLETED completes, out of it
// reopens, and any other move applies the filter delta so the task
// matches the destination.
func (n *dirNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	dst, ok := dirNodeOf(newParent)
	if !ok {
		return syscall.ENOTSUP
	}
	if naming.IsMintedName(newName) && newName != name {
		return syscall.EOPNOTSUPP
	}

	t, errno := n.resolveFileTask(name)
	if errno != 0 {
		return errno
	}
	uuid := t.UUID

	destFilter, errno := n.destinationFilter(dst, newName)
	if errno != 0 {
		return errno
	}

	srcHasStatus := n.filter.HasStatus()
	dstHasStatus := destFilter.HasStatus()
	if dstHasStatus && !srcHasStatus {
		action, errno := statusRemoveAction(destFilter)
		if errno != 0 {
			return errno
		}
		if err := n.root.query.Remove(uuid, action); err != nil {
			return syscall.EIO
		}
		return 0
	}

	if hasNonStatusVirtual(destFilter) {
		return syscall.ENOTSUP
	}
	var args []string
	if srcHasStatus && !dstHasStatus {
		args = append(args, "status:pending")
	}
	args = append(args, deltaModifyArgs(n.filter, destFilter)...)
	if len(args) > 0 {
		if err := n.root.query.Modify(uuid, args); err != nil {
			return syscall.EIO
		}
	}
	return 0
}

// destinationFilter resolves the destination path's filter: the final
// component is either the moved file's own name (into-directory move)
// or a project, tag, or selected virtual-tag segment minted on demand.
func (n *dirNode) destinationFilter(dst *dirNode, newName string) (backend.Filter, syscall.Errno) {
	if naming.IsMintedName(newName) {
		return dst.filter, 0
	}
	if _, ok := virtualTagSegName(dst, newName); ok {
		child, err := dst.filter.WithVirtualSegment(newName)
		if err != nil {
			return backend.Filter{}, syscall.EINVAL
		}
		return child, 0
	}
	child, err := dst.filter.WithSegment(newName)
	if err != nil {
		return backend.Filter{}, syscall.EINVAL
	}
	return child, 0
}

// virtualTagSegName reports whether a destination name spells a
// virtual tag selected on this mount, under either prefix.
func virtualTagSegName(dst *dirNode, name string) (string, bool) {
	if strings.HasPrefix(name, ".+") {
		return name[2:], dst.root.isVirtualTag(name[2:])
	}
	if strings.HasPrefix(name, "+") {
		return name[1:], dst.root.isVirtualTag(name[1:])
	}
	return "", false
}

// deltaModifyArgs renders the taskwarrior modify arguments that make a
// task match the destination filter: exact project and the tag delta.
func deltaModifyArgs(src, dst backend.Filter) []string {
	var args []string
	if dst.Project != src.Project {
		args = append(args, "project:"+dst.Project)
	}
	for _, t := range dst.Tags {
		if !contains(src.Tags, t) {
			args = append(args, "+"+t)
		}
	}
	for _, t := range src.Tags {
		if !contains(dst.Tags, t) {
			args = append(args, "-"+t)
		}
	}
	return args
}

// statusRemoveAction maps a destination's status virtual tag to the
// gesture that enters it. A filter carrying both status tags has no
// single meaning.
func statusRemoveAction(f backend.Filter) (backend.RemoveAction, syscall.Errno) {
	var action backend.RemoveAction
	for _, vt := range f.VirtualTags {
		var next backend.RemoveAction
		switch vt {
		case "COMPLETED":
			next = backend.RemoveComplete
		case "DELETED":
			next = backend.RemoveDelete
		default:
			continue
		}
		if action != "" && action != next {
			return "", syscall.ENOTSUP
		}
		action = next
	}
	if action == "" {
		return "", syscall.ENOTSUP
	}
	return action, 0
}

func hasNonStatusVirtual(f backend.Filter) bool {
	for _, vt := range f.VirtualTags {
		if _, ok := backend.StatusVirtualTag(vt); !ok {
			return true
		}
	}
	return false
}

func dirNodeOf(node fs.InodeEmbedder) (*dirNode, bool) {
	switch n := node.(type) {
	case *dirNode:
		return n, true
	case *FS:
		return &n.dirNode, true
	}
	return nil, false
}

func isAllUppercase(s string) bool {
	return s == strings.ToUpper(s)
}

type stableNoder interface {
	stableAttr() fs.StableAttr
}
