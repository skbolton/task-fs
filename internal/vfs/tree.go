package vfs

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"task-fs/internal/backend"
	"task-fs/internal/naming"
	"task-fs/internal/view"
)

const uuidXattr = "user.taskwarrior.uuid"

// FS is the root of the mounted filesystem.
type FS struct {
	dirNode
	query        *backend.Query
	removeAction backend.RemoveAction
	viewStore    ViewStore
	renderFilter func(context.Context, string) (string, error)

	mu              sync.Mutex
	declared        map[string]map[string]declaredSegment
	viewRenders     map[string]*viewRender
	viewRenderMu    sync.Mutex
	lastFilters     map[string]string
	lastFilterMu    sync.Mutex
	filterHandles   map[string]map[*filterHandle]struct{}
	filterHandlesMu sync.Mutex
}

const viewRenderTTL = time.Second

const filterFileName = view.FilterFileName

type viewRender struct {
	mu     sync.Mutex
	source string
	filter string
	at     time.Time
	valid  bool
}

// ViewStore holds the persistent definitions of runtime views.
type ViewStore interface {
	List() ([]string, error)
	Get(name string) (filter string, ok bool, err error)
	HasFilter(name string) (bool, error)
	Create(name string) error
	Put(name, filter string) error
	DeleteFilter(name string) error
	Delete(name string) error
}

// Config fixes the mount-time removal action and runtime view store.
type Config struct {
	RemoveAction backend.RemoveAction
	ViewStore    ViewStore
	RenderFilter func(context.Context, string) (string, error)
}

// declaredSegment remembers the filter a mkdir declared, so removing a
// directory can purge the declarations made under it.
type declaredSegment struct {
	filter backend.Filter
}

func New(query *backend.Query, cfg Config) *FS {
	root := &FS{
		dirNode:       dirNode{root: nil},
		query:         query,
		removeAction:  cfg.RemoveAction,
		viewStore:     cfg.ViewStore,
		renderFilter:  cfg.RenderFilter,
		declared:      map[string]map[string]declaredSegment{},
		viewRenders:   map[string]*viewRender{},
		lastFilters:   map[string]string{},
		filterHandles: map[string]map[*filterHandle]struct{}{},
	}
	if root.renderFilter == nil {
		root.renderFilter = func(_ context.Context, raw string) (string, error) {
			return raw, nil
		}
	}
	root.dirNode.root = root
	return root
}

func viewEntryName(name string) string { return "@" + name }

// lookupView reports whether an entry name is a persisted view. Invalid
// view names are not views, so they fall through to project lookup.
func (f *FS) lookupView(entryName string) (string, bool, syscall.Errno) {
	if f.viewStore == nil || !strings.HasPrefix(entryName, "@") {
		return "", false, 0
	}
	name := strings.TrimPrefix(entryName, "@")
	if err := view.ValidateName(name); err != nil {
		return "", false, 0
	}
	_, ok, err := f.viewStore.Get(name)
	if err != nil {
		return "", false, syscall.EIO
	}
	return name, ok, 0
}

func (f *FS) viewNames() ([]string, error) {
	if f.viewStore == nil {
		return nil, nil
	}
	return f.viewStore.List()
}

func (f *FS) invalidateViewRender(name string) {
	f.viewRenderMu.Lock()
	delete(f.viewRenders, name)
	f.viewRenderMu.Unlock()
}

func (f *FS) setLastFilter(name, filter string) {
	if filter == "" {
		return
	}
	f.lastFilterMu.Lock()
	f.lastFilters[name] = filter
	f.lastFilterMu.Unlock()
}

func (f *FS) lastFilter(name string) string {
	f.lastFilterMu.Lock()
	defer f.lastFilterMu.Unlock()
	return f.lastFilters[name]
}

func (f *FS) forgetLastFilter(name string) {
	f.lastFilterMu.Lock()
	delete(f.lastFilters, name)
	f.lastFilterMu.Unlock()
}

func (f *FS) registerFilterHandle(view string, handle *filterHandle) {
	f.filterHandlesMu.Lock()
	set := f.filterHandles[view]
	if set == nil {
		set = map[*filterHandle]struct{}{}
		f.filterHandles[view] = set
	}
	set[handle] = struct{}{}
	f.filterHandlesMu.Unlock()
}

func (f *FS) unregisterFilterHandle(view string, handle *filterHandle) {
	f.filterHandlesMu.Lock()
	if set := f.filterHandles[view]; set != nil {
		delete(set, handle)
		if len(set) == 0 {
			delete(f.filterHandles, view)
		}
	}
	f.filterHandlesMu.Unlock()
}

func (f *FS) resizeFilterHandles(view string, size uint64) {
	f.filterHandlesMu.Lock()
	handles := make([]*filterHandle, 0, len(f.filterHandles[view]))
	for handle := range f.filterHandles[view] {
		handles = append(handles, handle)
	}
	f.filterHandlesMu.Unlock()
	for _, handle := range handles {
		handle.mu.Lock()
		handle.data = resizeFilter(handle.data, size)
		handle.dirty = true
		handle.mu.Unlock()
	}
}

func (f *FS) renderViewFilter(ctx context.Context, name string, refresh bool) (source, filter string, err error) {
	if f.viewStore == nil {
		return "", "", nil
	}
	source, ok, err := f.viewStore.Get(name)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", nil
	}
	if strings.TrimSpace(source) == "" {
		return source, "", nil
	}

	f.viewRenderMu.Lock()
	entry := f.viewRenders[name]
	if entry == nil {
		entry = &viewRender{}
		f.viewRenders[name] = entry
	}
	f.viewRenderMu.Unlock()

	entry.mu.Lock()
	defer entry.mu.Unlock()

	if !refresh && entry.valid && entry.source == source && time.Since(entry.at) < viewRenderTTL {
		return source, entry.filter, nil
	}
	filter, err = f.renderFilter(ctx, source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "task-fs: view %q: %v\n", name, err)
		return source, "", err
	}
	entry.source = source
	entry.filter = filter
	entry.at = time.Now()
	entry.valid = true
	return source, filter, nil
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
	return hashString(canonicalKey(f))
}

func hashString(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// setOwner reports the mount process owner so permission-aware tools
// such as editors see writable nodes as writable.
func setOwner(out *fuse.AttrOut) {
	out.Uid = uint32(os.Getuid())
	out.Gid = uint32(os.Getgid())
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
	entries, errno := n.readdirEntries()
	if errno != 0 {
		return nil, errno
	}
	return fs.NewListDirStream(entries), 0
}

func (n *dirNode) readdirEntries() ([]fuse.DirEntry, syscall.Errno) {
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
		for _, vt := range backend.StatusVirtualTags() {
			add("+"+vt, fuse.S_IFDIR)
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
	return entries, 0
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
	setOwner(out)
	return 0
}

var _ = (fs.NodeLookuper)((*FS)(nil))

func (f *FS) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	child, errno := f.resolveChild(name)
	if errno != 0 {
		return nil, errno
	}
	inode := f.NewInode(ctx, child, child.(stableNoder).stableAttr())
	return inode, 0
}

func (f *FS) resolveChild(name string) (fs.InodeEmbedder, syscall.Errno) {
	viewName, ok, errno := f.lookupView(name)
	if errno != 0 {
		return nil, errno
	}
	if ok {
		return &viewNode{root: f, name: viewName}, 0
	}
	return f.dirNode.resolveChild(name)
}

var _ = (fs.NodeReaddirer)((*FS)(nil))

func (f *FS) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	names, err := f.viewNames()
	if err != nil {
		return nil, syscall.EIO
	}
	entries := make([]fuse.DirEntry, 0, len(names))
	seen := map[string]bool{}
	for _, viewName := range names {
		name := viewEntryName(viewName)
		if seen[name] {
			continue
		}
		seen[name] = true
		entries = append(entries, fuse.DirEntry{Name: name, Mode: fuse.S_IFDIR})
	}
	normal, errno := f.dirNode.readdirEntries()
	if errno != 0 {
		return nil, errno
	}
	for _, entry := range normal {
		if !seen[entry.Name] {
			seen[entry.Name] = true
			entries = append(entries, entry)
		}
	}
	return fs.NewListDirStream(entries), 0
}

var _ = (fs.NodeMkdirer)((*FS)(nil))

func (f *FS) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if strings.HasPrefix(name, "@") {
		child, errno := f.createView(name)
		if errno != 0 {
			return nil, errno
		}
		return f.NewInode(ctx, child, child.stableAttr()), 0
	}
	return f.dirNode.Mkdir(ctx, name, mode, out)
}

func (f *FS) createView(name string) (*viewNode, syscall.Errno) {
	if f.viewStore == nil {
		return nil, syscall.EIO
	}
	viewName := strings.TrimPrefix(name, "@")
	if err := f.viewStore.Create(viewName); err != nil {
		switch {
		case errors.Is(err, view.ErrExists):
			return nil, syscall.EEXIST
		case errors.Is(err, view.ErrInvalidName):
			return nil, syscall.EINVAL
		default:
			return nil, syscall.EIO
		}
	}
	return &viewNode{root: f, name: viewName}, 0
}

var _ = (fs.NodeRmdirer)((*FS)(nil))

func (f *FS) Rmdir(ctx context.Context, name string) syscall.Errno {
	viewName, ok, errno := f.lookupView(name)
	if errno != 0 {
		return errno
	}
	if ok {
		if f.viewStore == nil {
			return syscall.EIO
		}
		if err := f.viewStore.Delete(viewName); err != nil {
			if errors.Is(err, view.ErrNotExist) {
				return syscall.ENOENT
			}
			return syscall.EIO
		}
		f.invalidateViewRender(viewName)
		f.forgetLastFilter(viewName)
		return 0
	}
	return f.dirNode.Rmdir(ctx, name)
}

func (f *FS) removeTask(t backend.Task) syscall.Errno {
	// A deleted task's file only appears in the +DELETED view; it is a
	// tombstone, and removing it again must not re-invoke delete. A
	// completed task is likewise already done, and Taskwarrior refuses
	// to complete it again.
	if t.Status == "deleted" {
		return 0
	}
	if f.removeAction == backend.RemoveComplete && t.Status == "completed" {
		return 0
	}
	if err := f.query.Remove(t.UUID, f.removeAction); err != nil {
		return syscall.EIO
	}
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
	case strings.HasPrefix(name, "."):
		return nil, syscall.ENOENT
	case strings.HasPrefix(name, "+"):
		seg := name[1:]
		if backend.IsVirtualTag(seg) {
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
	setOwner(out)
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
	setOwner(out)
	return 0
}

var _ = (fs.NodeSetattrer)((*dirNode)(nil))

func (n *dirNode) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if mtime, ok := in.GetMTime(); ok {
		out.SetTimes(nil, &mtime, nil)
	}
	out.Mode = fuse.S_IFDIR | 0755
	setOwner(out)
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
	return n.root.removeTask(t)
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
		return syscall.ENOENT
	}
	if strings.HasPrefix(name, "+") && backend.IsVirtualTag(name[1:]) {
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
	if _, ok := newParent.(*viewNode); ok {
		return syscall.EACCES
	}
	if root, ok := newParent.(*FS); ok {
		if _, isView, errno := root.lookupView(newName); errno != 0 {
			return errno
		} else if isView {
			return syscall.EACCES
		}
	}
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
	if _, ok := virtualTagSegName(newName); ok {
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
// status virtual tag.
func virtualTagSegName(name string) (string, bool) {
	if strings.HasPrefix(name, "+") {
		return name[1:], backend.IsVirtualTag(name[1:])
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

// viewNode is one runtime saved filter. It is a lookup view, not a
// filter node: its entries are its filter file and task files.
type viewNode struct {
	fs.Inode
	root *FS
	name string
}

func (n *viewNode) viewKey() string { return "view:" + n.name }

func (n *viewNode) stableAttr() fs.StableAttr {
	return fs.StableAttr{Mode: fuse.S_IFDIR, Ino: hashString(n.viewKey())}
}

func (n *viewNode) tasks(ctx context.Context, refresh bool) (source string, tasks []backend.Task, err error) {
	source, filter, err := n.root.renderViewFilter(ctx, n.name, refresh)
	if err != nil {
		return source, nil, err
	}
	if strings.TrimSpace(source) == "" {
		return source, nil, nil
	}
	n.root.setLastFilter(n.name, filter)
	tasks, err = n.root.query.ExportQuery(filter)
	return source, tasks, err
}

func (n *viewNode) filterExists() (bool, syscall.Errno) {
	if n.root.viewStore == nil {
		return false, syscall.EIO
	}
	hasFilter, err := n.root.viewStore.HasFilter(n.name)
	if err != nil {
		if errors.Is(err, view.ErrInvalidName) {
			return false, syscall.EINVAL
		}
		return false, syscall.EIO
	}
	return hasFilter, 0
}

// tasksForRemoval keeps a recursive removal working when _filter is
// visited before the task files it already enumerated.
func (n *viewNode) tasksForRemoval(ctx context.Context) ([]backend.Task, error) {
	source, filter, err := n.root.renderViewFilter(ctx, n.name, true)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(source) == "" {
		filter = n.root.lastFilter(n.name)
		if filter == "" {
			return nil, nil
		}
	} else {
		n.root.setLastFilter(n.name, filter)
	}
	return n.root.query.ExportQuery(filter)
}

func (n *viewNode) resolveFileTask(ctx context.Context, name string, refresh bool) (backend.Task, syscall.Errno) {
	_, tasks, err := n.tasks(ctx, refresh)
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

func (n *viewNode) resolveChild(ctx context.Context, name string, refresh bool) (fs.InodeEmbedder, syscall.Errno) {
	if name == filterFileName {
		exists, errno := n.filterExists()
		if errno != 0 {
			return nil, errno
		}
		if !exists {
			return nil, syscall.ENOENT
		}
		return &filterFileNode{root: n.root, view: n.name}, 0
	}
	if !naming.IsMintedName(name) {
		return nil, syscall.ENOENT
	}
	t, errno := n.resolveFileTask(ctx, name, refresh)
	if errno != 0 {
		return nil, errno
	}
	return &fileNode{root: n.root, uuid: t.UUID, viewKey: n.viewKey()}, 0
}

var _ = (fs.NodeLookuper)((*viewNode)(nil))

func (n *viewNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	child, errno := n.resolveChild(ctx, name, false)
	if errno != 0 {
		return nil, errno
	}
	return n.NewInode(ctx, child, child.(stableNoder).stableAttr()), 0
}

var _ = (fs.NodeReaddirer)((*viewNode)(nil))

func (n *viewNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	hasFilter, errno := n.filterExists()
	if errno != 0 {
		return nil, errno
	}
	_, tasks, err := n.tasks(ctx, true)
	if err != nil {
		return nil, syscall.EIO
	}
	seen := map[string]bool{}
	entries := make([]fuse.DirEntry, 0, len(tasks)+1)
	if hasFilter {
		seen[filterFileName] = true
		entries = append(entries, fuse.DirEntry{Name: filterFileName, Mode: fuse.S_IFREG})
	}
	for _, t := range tasks {
		name := naming.FileName(t.Description, t.UUID)
		if seen[name] {
			continue
		}
		seen[name] = true
		entries = append(entries, fuse.DirEntry{Name: name, Mode: fuse.S_IFREG})
	}
	return fs.NewListDirStream(entries), 0
}

var _ = (fs.NodeGetattrer)((*viewNode)(nil))

func (n *viewNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = fuse.S_IFDIR | 0755
	out.Size = 0
	out.Nlink = 1
	setOwner(out)
	return 0
}

var _ = (fs.NodeSetattrer)((*viewNode)(nil))

func (n *viewNode) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if mtime, ok := in.GetMTime(); ok {
		out.SetTimes(nil, &mtime, nil)
	}
	out.Mode = fuse.S_IFDIR | 0755
	setOwner(out)
	return 0
}

var _ = (fs.NodeUnlinker)((*viewNode)(nil))

func (n *viewNode) Unlink(ctx context.Context, name string) syscall.Errno {
	if name == filterFileName {
		return n.clearFilter()
	}
	if !naming.IsMintedName(name) {
		return syscall.ENOENT
	}
	tasks, err := n.tasksForRemoval(ctx)
	if err != nil {
		return syscall.EIO
	}
	for _, t := range tasks {
		if naming.FileName(t.Description, t.UUID) == name {
			return n.root.removeTask(t)
		}
	}
	return syscall.ENOENT
}

func (n *viewNode) clearFilter() syscall.Errno {
	if n.root.viewStore == nil {
		return syscall.EIO
	}
	if err := n.root.viewStore.DeleteFilter(n.name); err != nil {
		return syscall.EIO
	}
	n.root.invalidateViewRender(n.name)
	return 0
}

var _ = (fs.NodeCreater)((*viewNode)(nil))

func (n *viewNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	child, handle, fuseFlags, errno := n.createFilter(ctx, name, flags)
	if errno != 0 {
		return nil, nil, 0, errno
	}
	return n.NewInode(ctx, child, child.(stableNoder).stableAttr()), handle, fuseFlags, 0
}

func (n *viewNode) createFilter(ctx context.Context, name string, flags uint32) (fs.InodeEmbedder, fs.FileHandle, uint32, syscall.Errno) {
	if name != filterFileName {
		return nil, nil, 0, syscall.EACCES
	}
	if n.root.viewStore == nil {
		return nil, nil, 0, syscall.EIO
	}
	hasFilter, err := n.root.viewStore.HasFilter(n.name)
	if err != nil {
		return nil, nil, 0, syscall.EIO
	}
	if hasFilter {
		return nil, nil, 0, syscall.EEXIST
	}
	if err := n.root.viewStore.Put(n.name, ""); err != nil {
		return nil, nil, 0, syscall.EIO
	}
	n.root.invalidateViewRender(n.name)
	child := &filterFileNode{root: n.root, view: n.name}
	handle, fuseFlags, errno := child.Open(ctx, flags)
	if errno != 0 {
		return nil, nil, 0, errno
	}
	return child, handle, fuseFlags, 0
}

var _ = (fs.NodeMkdirer)((*viewNode)(nil))

func (n *viewNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	return nil, syscall.EACCES
}

var _ = (fs.NodeRmdirer)((*viewNode)(nil))

func (n *viewNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	return syscall.ENOENT
}

var _ = (fs.NodeRenamer)((*viewNode)(nil))

func (n *viewNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	return syscall.EACCES
}

// filterFileNode is the editable _filter definition inside a view.
type filterFileNode struct {
	fs.Inode
	root *FS
	view string
}

func (n *filterFileNode) stableAttr() fs.StableAttr {
	return fs.StableAttr{Mode: fuse.S_IFREG, Ino: hashString("filter:" + n.view)}
}

func (n *filterFileNode) current() (string, bool, syscall.Errno) {
	if n.root.viewStore == nil {
		return "", false, syscall.EIO
	}
	filter, ok, err := n.root.viewStore.Get(n.view)
	if err != nil {
		return "", false, syscall.EIO
	}
	if !ok {
		return "", false, syscall.ENOENT
	}
	return filter, true, 0
}

var _ = (fs.NodeGetattrer)((*filterFileNode)(nil))

func (n *filterFileNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	size := 0
	if handle, ok := fh.(*filterHandle); ok {
		handle.mu.Lock()
		size = len(handle.data)
		handle.mu.Unlock()
	} else {
		filter, _, errno := n.current()
		if errno != 0 {
			return errno
		}
		size = len(filter)
	}
	out.Mode = fuse.S_IFREG | 0644
	out.Size = uint64(size)
	out.Nlink = 1
	setOwner(out)
	return 0
}

var _ = (fs.NodeOpener)((*filterFileNode)(nil))

func (n *filterFileNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	filter, _, errno := n.current()
	if errno != 0 {
		return nil, 0, errno
	}
	handle := &filterHandle{data: []byte(filter), appendMode: flags&syscall.O_APPEND != 0}
	if flags&syscall.O_TRUNC != 0 {
		handle.data = nil
		handle.dirty = true
	}
	n.root.registerFilterHandle(n.view, handle)
	return handle, fuse.FOPEN_KEEP_CACHE, 0
}

var _ = (fs.NodeReader)((*filterFileNode)(nil))

func (n *filterFileNode) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	var data []byte
	if handle, ok := fh.(*filterHandle); ok {
		handle.mu.Lock()
		data = append([]byte(nil), handle.data...)
		handle.mu.Unlock()
	} else {
		filter, _, errno := n.current()
		if errno != 0 {
			return nil, errno
		}
		data = []byte(filter)
	}
	if off < 0 {
		off = 0
	}
	if off >= int64(len(data)) {
		return fuse.ReadResultData(dest[:0]), 0
	}
	end := int(off) + len(dest)
	if end > len(data) {
		end = len(data)
	}
	return fuse.ReadResultData(data[off:end]), 0
}

var _ = (fs.NodeWriter)((*filterFileNode)(nil))

func (n *filterFileNode) Write(ctx context.Context, fh fs.FileHandle, data []byte, off int64) (uint32, syscall.Errno) {
	handle, ok := fh.(*filterHandle)
	if !ok {
		return 0, syscall.EIO
	}
	handle.mu.Lock()
	defer handle.mu.Unlock()
	if handle.appendMode {
		off = int64(len(handle.data))
	}
	if off < 0 {
		return 0, syscall.EINVAL
	}
	end := off + int64(len(data))
	if end > int64(len(handle.data)) {
		grown := make([]byte, end)
		copy(grown, handle.data)
		handle.data = grown
	}
	copy(handle.data[off:end], data)
	handle.dirty = true
	return uint32(len(data)), 0
}

var _ = (fs.NodeFlusher)((*filterFileNode)(nil))

func (n *filterFileNode) Flush(ctx context.Context, fh fs.FileHandle) syscall.Errno {
	return n.persist(fh)
}

var _ = (fs.NodeFsyncer)((*filterFileNode)(nil))

func (n *filterFileNode) Fsync(ctx context.Context, fh fs.FileHandle, flags uint32) syscall.Errno {
	return n.persist(fh)
}

var _ = (fs.NodeReleaser)((*filterFileNode)(nil))

func (n *filterFileNode) Release(ctx context.Context, fh fs.FileHandle) syscall.Errno {
	errno := n.persist(fh)
	if handle, ok := fh.(*filterHandle); ok {
		n.root.unregisterFilterHandle(n.view, handle)
	}
	return errno
}

func (n *filterFileNode) persist(fh fs.FileHandle) syscall.Errno {
	handle, ok := fh.(*filterHandle)
	if !ok || !handle.dirty {
		return 0
	}
	handle.mu.Lock()
	defer handle.mu.Unlock()
	if n.root.viewStore == nil {
		return syscall.EIO
	}
	if err := n.root.viewStore.Put(n.view, string(handle.data)); err != nil {
		return syscall.EIO
	}
	handle.dirty = false
	n.root.invalidateViewRender(n.view)
	return 0
}

var _ = (fs.NodeSetattrer)((*filterFileNode)(nil))

func (n *filterFileNode) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if size, ok := in.GetSize(); ok {
		filter, _, errno := n.current()
		if errno != 0 {
			return errno
		}
		if err := n.root.viewStore.Put(n.view, string(resizeFilter([]byte(filter), size))); err != nil {
			return syscall.EIO
		}
		n.root.resizeFilterHandles(n.view, size)
		n.root.invalidateViewRender(n.view)
	}
	if mtime, ok := in.GetMTime(); ok {
		out.SetTimes(nil, &mtime, nil)
	}
	out.Mode = fuse.S_IFREG | 0644
	if handle, ok := fh.(*filterHandle); ok {
		handle.mu.Lock()
		out.Size = uint64(len(handle.data))
		handle.mu.Unlock()
	}
	setOwner(out)
	return 0
}

func resizeFilter(data []byte, size uint64) []byte {
	if size <= uint64(len(data)) {
		return data[:size]
	}
	grown := make([]byte, size)
	copy(grown, data)
	return grown
}

type filterHandle struct {
	mu         sync.Mutex
	data       []byte
	dirty      bool
	appendMode bool
}

type stableNoder interface {
	stableAttr() fs.StableAttr
}
