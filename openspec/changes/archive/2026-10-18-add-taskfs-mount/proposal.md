# Proposal

## Why

Taskwarrior has the concept of contexts which are user-defined filter that can be activated so that it is applied to any
commands. This allows you to work in a subset of tasks without typing the filter every time. When defining them you
specify a write filter to specify data that should go on new tasks in that filter as well as a read filter for listing tasks.

The issue in this approach is the tedium in creating contexts as well as having to manage them after the fact. If you
create an ad-hoc context for something you need to work on for an afternoon it still gets persisted in your
configuration file. So not only do you have the burden of creating the context, you have a burden of maintaining it into
the future.

A new approach to contexts could instead be a filepath on a FUSE filesystem. If the taskwarrior database counld be
mounted in user-space than the project hierarchy as well as tag states could be shown as folders to the user. This
enables the user to use whatever file manager tooling they want to be able to quickly navigate their projects and get
many of the same perks that the context system gives.

## What Changes

- Add `task-fs`, a Go program that mounts a Taskwarrior database as a FUSE filesystem at a user-supplied mountpoint.
- Directories form a filter tree: project segments, `+tag` segments, and hidden dot-prefixed virtual-tag segments (`.+COMPLETED`, `.+DUE`, ...), all order-free, with `status:pending` implied when no status segment is present. Virtual tags are spelled only in the dot-prefixed hidden form so file managers can toggle their visibility.
- Each task appears as one file named `<description>.<short-uuid>.md` with empty placeholder content, and the task's full UUID exposed as an extended attribute; task files are read-only in v1, with the write path reserved for a future note-taking layer where editing the file edits the task's notes.
- Filesystem gestures mutate tasks: `touch` a new file to add a task (cwd filter injected as defaults), `rm` to delete, `mv` into `+COMPLETED` to complete, `mv` out of `+COMPLETED` to reopen, and `mv` to any project or tag path to reassign the task's project and tags so they match the destination (a move is the filter delta between source and destination). `mkdir` declares tag or project namespaces that no task yet matches.
- The backend is the `task` CLI itself: queries run on demand via `task export` with a filter built from the path; no database is read directly and no results are cached in v1.

Out of scope for v1: 
- Renaming tasks via `mv` (description changes)
- Note editing (task files are read-only in v1; a future out-of-tree note system will make file edits map to task annotations)
- Browsing `+DELETED`, a "show all statuses" view
- Any caching layer
- Customizing the base filter in the mountpoint (status:pending the hard coded filter for now)

## Capabilities

### New Capabilities

- `mount-lifecycle`: Mounting and unmounting the filesystem — the `mount` command, mountpoint handling, backend resolution delegated to `task` (including `TASKDATA`/`rc.data.location`), and on-demand query behavior.
- `task-navigation`: The directory tree and its filter semantics — path grammar, project/tag/virtual-tag segments, order-free parsing, pending-by-default status, and which entries are visible at each level.
- `task-files`: How tasks appear as files — naming (`<description>.<short-uuid>.md`), empty placeholder content, UUID extended attribute, and behavior on duplicates, long names, and renames.
- `task-mutations`: Filesystem gestures that modify tasks — add via `touch`, delete via `rm`, complete/reopen via `mv` to and from `+COMPLETED`, and project/tag reassignment via `mv` to a destination whose filter the task then matches.

### Modified Capabilities

None — this is a new system with no existing specs.

## Impact

- New Go module and binary (`task-fs`) in this repository, depending on `github.com/hanwen/go-fuse/v2`.
- Runtime dependency on a Taskwarrior 3.x CLI (`task`) on `$PATH`; the filesystem never opens `taskchampion.sqlite3` itself.
- Requires FUSE availability on the host. Linux is the supported v1 platform; macOS is expected to work with macFUSE installed (go-fuse has darwin support) but is untested in v1 — the specs are platform-neutral, so macOS support requires no spec changes, only the macFUSE system extension on the host.
- No changes to existing Taskwarrior data or configuration; the mount is read/write through `task` commands only, so all mutations go through Taskwarrior's own command surface.
