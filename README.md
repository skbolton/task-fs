# TaskFS

>[!CAUTION]
> Currently iterating with an agent to prototype this out.
> I am planning to circle back and harden things as I learn more
> This is alpha software

TaskFS mounts your Taskwarrior database as a FUSE filesystem. Your project
hierarchy and tags become directories, tasks become files, and the ordinary
file gestures — `touch`, `rm`, `mv`, `mkdir` — talk to the `task` CLI. Use
your file manager, `ls`, `cd`, and shell completions on your task list.

## Requirements

- A Taskwarrior 3.x `task` binary on `$PATH`
- FUSE on the host (Linux; macOS should work with macFUSE installed but is
  untested)

## Mounting

```sh
task-fs mount ~/c/TaskFS
```

The mountpoint directory must already exist. The mount runs in the
foreground; Ctrl-C unmounts cleanly. Backgrounding is left to your session
manager. The database is whatever the mount process resolves, so alternate
databases work through the environment:

```sh
TASKDATA=$HOME/other-tasks task-fs mount ~/c/TaskFS
```

TaskFS never reads Taskwarrior data files directly — every listing is an
on-demand `task` invocation, so changes made with the CLI in another
terminal are visible immediately.

## Home Manager

Add this repository as a flake input, import the module, and enable the
service:

```nix
{
  inputs.task-fs.url = "github:<owner>/task-fs";

  imports = [ inputs.task-fs.homeModules.task-fs ];

  services.task-fs = {
    enable = true;
    automount = {
      enable = true;
      path = "~/some/path";
    };
  };
}
```

`services.task-fs.package` overrides the mounted binary; it defaults to the
package built by this flake. The automount runs as a systemd user service:
it creates the mountpoint if needed, mounts on session start, and unmounts
when the session ends. Linux only.

The mount resolves its Taskwarrior database from the systemd user manager's
environment, not your interactive shell. For an alternate database, set
`TASKDATA` in the session environment:

```nix
systemd.user.sessionEnvironment.TASKDATA = "/home/me/other-tasks";
```

## The path grammar

A path is a task filter. Segments are order-free; these two paths are the
same view:

```
/home/+next
/+next/home
```

- **Project segments** (`home`, `finances`, ...) filter to a project and all
  of its dotted subprojects: `/finances` includes `finances.2026-taxes`.
  Nesting goes one level per path segment. A task file appears in the
  directory of its **exact** project only — `/finances` shows the
  `2026-taxes` directory but not that subproject's tasks; those live in
  `/finances/2026-taxes`. The root is the inbox: it lists only tasks with
  no project.
- **Tag segments** (`+next`, `+shopping`, ...) filter to tasks carrying the
  tag. Tag views without a project segment (e.g. `/+next`) list matching
  tasks of every project depth.
- **Uppercase `+` segments prefixed with a dot are virtual tags**: hidden
  entries like `.+COMPLETED`, `.+DUE`, `.+OVERDUE` — Taskwarrior's derived
  filters. The dot prefix makes them hidden files, so plain `ls` and file
  managers hide them; use `ls -a` or the "show hidden files" toggle to see
  them. The dot form is the only spelling: `+COMPLETED` does not resolve,
  and a bare uppercase `+` segment is treated as an ordinary
  (case-sensitive) tag.
- **Pending by default**: without a status segment the tree shows pending
  tasks. Under `.+COMPLETED` the tree shows completed tasks.

Only segments that exist — derivable from your tasks, or declared with
`mkdir` (below) — resolve. A typo fails with ENOENT instead of showing an
empty directory:

```
$ cd /homi
ls: cannot access '/homi': No such file or directory
```

Dot-prefixed names are reserved for the filesystem: `mkdir` rejects them.

## Task files

Each task matching a view appears as one file:

```
Pick up Milk.96b7b9b1.md
```

The suffix is the first 8 hex characters of the task's UUID. Names are
stable as the database changes, and descriptions are sanitized to a
filename-legal character set. Task files are empty placeholders and
read-only in this version — the file name is the preview. To script against
a task, resolve its full UUID from the extended attribute:

```sh
getfattr -n user.taskwarrior.uuid --only-values "Pick up Milk.96b7b9b1.md"
```

Writes to task files are rejected; a future note system will map file edits
to task annotations.

## Gestures

| Gesture | Effect |
| --- | --- |
| `touch "Description"` | Adds a task with the current directory's project and tags. |
| `rm <taskfile>` | Deletes the task. |
| `mv <taskfile> .+COMPLETED/` | Completes the task (rest of the destination path is ignored). |
| `mv <taskfile> out of .+COMPLETED` | Reopens the task and applies the destination's project/tags. |
| `mv <taskfile> <other-dir>` | The task's project and tags match the destination: projects set exactly (moving to a parent drops the subproject), tags added or removed per the filter delta. |
| `mv <taskfile> /new/deep/dir` (no trailing slash) | The final component is minted as a new project segment, provided the parent path exists. |
| `mkdir +tag` or `mkdir project-segment` | Declares a namespace that no task matches yet, so a later move can land there. |

Notes:

- Creating a task inside a virtual-tag view works: virtual tags are dropped
  from the write filter (`touch x` in `/home/.+DUE` creates a task in
  `home`), so the new file appears one level up.
- `mv` is a filter delta: moving from `/home/+next` to `/finances/+urgent`
  sets project `finances`, removes `next`, adds `urgent`. Renaming a task
  file (changing its description) is not supported in this version.
- Moving into a read-only virtual-tag view (`.+DUE`, `.+OVERDUE`, ...) fails,
  since those statuses are derived, not assignable.
- `mkdir` declarations last for the mount session only; once a task lands
  there the directory exists from the task data itself.
- `rmdir` of a declared segment fails with ENOTEMPTY while any task carries
  it.

## Unmounting

```sh
fusermount3 -u ~/c/TaskFS
```

Or interrupt the foreground process (Ctrl-C).

## Development

With Nix: `nix develop` provides `go`, `fuse3`, and `taskwarrior`.

```sh
go build ./...
go test ./...
go run ./cmd/task-fs mount <mountpoint>
```
