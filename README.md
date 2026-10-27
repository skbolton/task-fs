# TaskFS

>[!CAUTION]
> Currently iterating with an agent to prototype this out. Consider this alpha software that is sure to have issues and
> breaking changes as development evolves.

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

Two flags shape the virtual-tag views (the uppercase Taskwarrior-derived
filters such as `COMPLETED` and `DUE`):

```sh
task-fs mount ~/c/TaskFS --virtual-tags "COMPLETED,OVERDUE,DUE,PENDING,ACTIVE"
task-fs mount ~/c/TaskFS --virtual-hidden
```

- `--virtual-tags` (`-t`) takes a comma-separated list of virtual tags
  without the `+` sign, or `ALL` for every derived filter your Taskwarrior
  knows. The default is `COMPLETED,OVERDUE,DUE,ACTIVE,WAITING`. Every entry
  is validated against what Taskwarrior discovers, and an unknown name
  aborts the mount.
- `--virtual-hidden` (`-H`) presents the virtual-tag views as hidden
  `.+NAME` directories; without it they are ordinary `+NAME` directories.
  Both spellings resolve on lookup no matter how they are listed. The old
  behavior — every discovered virtual tag, hidden — is
  `--virtual-hidden --virtual-tags ALL`.

A third flag selects what removing a task does:

```sh
task-fs mount ~/c/TaskFS --rm delete
```

- `--rm complete|delete` selects the action for `rm` and for removing
  directories: `complete` (the default) completes the matching tasks, while
  `delete` permanently deletes them. **This changes the previous default**:
  a flagless mount now completes tasks removed with `rm` or folder deletion;
  pass `--rm delete` to keep the old destructive behavior.

A repeatable `--view` flag adds custom views at the root. Custom views are the `@Name`
views defined with `--view`, distinct from virtual-tag views (`+COMPLETED`) and tag
views (`+next`):

```sh
task-fs mount ~/c/TaskFS \
  --view 'Due Next Week=status:pending sched.after:sonw sched.before:eonw' \
  --view 'Overdue=status:pending due.before:now'
```

- `--view NAME=FILTER` splits at the first `=` into the display name and a
  Taskwarrior filter. The name may contain spaces; a leading `@` is accepted
  and normalized. The filter runs exactly as written, so include
  `status:pending` yourself when you want pending tasks only. Filters are
  validated against Taskwarrior at mount time; a malformed filter aborts the
  mount, while a valid filter that matches no tasks yields an empty
  `@NAME` directory.
- A filter may contain one or more `{{ command }}` blocks. Each command runs
  through `/bin/sh -c` in the mount environment, and its trimmed standard
  output replaces the block as filter text. Use a backslash to keep literal
  braces: `\{{ not a command }}`.

Dynamic views cover dates and other values that should not be frozen at mount
time:

```sh
task-fs mount ~/c/TaskFS \
  --view 'Payday=status:pending due.before:{{ payday-date }}' \
  --view 'Next Sprint=status:pending due.before:{{ date -d "+14 days" +%F }}'
```

Single-quote the whole `--view` value in an interactive shell so your shell
does not expand the command itself. Nix strings are literal, so the braces
reach `task-fs` unchanged there as well.

- Command output is trimmed and spliced as a filter expression. Empty output,
  NUL bytes, and embedded newlines are rejected. Each command has a
  five-second timeout.
- Templates are parsed and rendered once during mount validation, so a
  missing program, failing command, empty output, or malformed rendered filter
  aborts the mount with the view name in the error.
- A long-lived mount re-renders the block when the view is browsed. A listing
  updates the value immediately; lookups within the same traversal reuse the
  rendered value for up to one second. Removal gestures render fresh.
- A render failure makes the view operation fail with `EIO` and writes the
  view name, command, and stderr to the mount process's standard error. Under
  Home Manager that is the user service journal.
- Each definition appears as a visible root-only directory named `@NAME`.
  It lists the matching task files flat across all projects and does not
  expose project, tag, virtual-tag, or nested custom view directories. The custom view
  path does not compose with other segments: `@NAME/home` is ENOENT.
- Custom view directories are for reading and removing tasks. `rm` applies the
  configured `--rm` action. `touch`, `mkdir`, and `mv` are rejected: an
  arbitrary filter has no known set of task changes that would move a task
  into or out of it.
- `rmdir @NAME` does not change tasks. It fails with `ENOTEMPTY` while the
  configured action would change a matching task and succeeds once none
  remains, so recursive removal converges. The custom view stays configured
  and remains in the root listing.
- A configured custom view shadows a real project whose first root segment has the
  same `@NAME`; the project is reachable again when the custom view is not
  configured.

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

The automount accepts mount flags through
`services.task-fs.automount.mountFlags`, a list of strings where each item
is one command-line token. Values containing spaces are quoted in the
generated service command, so a `--view` value can be written naturally:

```nix
services.task-fs.automount.mountFlags = [
  "--virtual-tags"
  "COMPLETED,OVERDUE,DUE,ACTIVE,WAITING"
  "--virtual-hidden"
  "--view"
  "Due Next Week=status:pending sched.after:sonw sched.before:eonw"
];
```

Dynamic view blocks work in the same list:

```nix
services.task-fs.automount.mountFlags = [
  "--view"
  "Next Week=status:pending due.before:{{ date -d \"+7 days\" +%F }}"
];
```

The automount service provides a POSIX shell and coreutils, so commands such
as `date` are available. Commands outside that baseline must use an absolute
Nix store path or an executable wrapper script, because the service does not
inherit your interactive shell's `PATH`. The mount command's shell environment
does include the session's Taskwarrior configuration and `TASKDATA`.

## The path grammar

A path in the filter tree is a task filter; custom view directories are saved
lookups and do not accumulate filters. Filter segments are order-free;
these two paths are the same view:

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
- **Uppercase `+` segments are virtual tags**: `+COMPLETED`, `+DUE`,
  `+OVERDUE`, ... — Taskwarrior's derived filters. Which ones exist is
  chosen at mount time with `--virtual-tags` (above). Each selected view
  can also be spelled with a dot prefix (`.+COMPLETED`); the dot form is
  how they are listed under `--virtual-hidden`, where they are hidden
  files so plain `ls` and file managers hide them — use `ls -a` or the
  "show hidden files" toggle to see them.
- **`@` segments are custom views**: `@Due Next Week`, `@Overdue`, ...
  exist only at the root, for custom views configured with `--view` (above).
  They are lookup views, not filter directories: their entries are the
  matching task files flat across all projects, and they do not list or
  resolve project, tag, virtual-tag, or nested custom view segments.
- **Filter directories are pending by default**: without a status segment
  a filter directory shows pending tasks, and under `+COMPLETED` it shows
  completed tasks. Custom view directories are the exception: their filter is
  used exactly as written, with no implicit status.

Only segments that exist — derivable from your tasks, declared with
`mkdir` (below), or named by a configured custom view — resolve. A typo fails
with ENOENT instead of showing an empty directory:

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
| `rm <taskfile>` | Applies the configured `--rm` action: completes the task by default, deletes it with `--rm delete`. |
| `rmdir <dir>` / `rm -rf <dir>` | Applies the same configured action to every task matching the directory's filter. |
| `mv <taskfile> +COMPLETED/` | Completes the task (rest of the destination path is ignored; `.+COMPLETED/` works too). |
| `mv <taskfile> +DELETED/` | Deletes the task (rest of the destination path is ignored; `.+DELETED/` works too; the view must be selected with `--virtual-tags`). |
| `mv <taskfile> out of +COMPLETED or +DELETED` | Reopens the task and applies the destination's project/tags. |
| `mv <taskfile> <other-dir>` | The task's project and tags match the destination: projects set exactly (moving to a parent drops the subproject), tags added or removed per the filter delta. |
| `mv <taskfile> /new/deep/dir` (no trailing slash) | The final component is minted as a new project segment, provided the parent path exists. |
| `mkdir +tag` or `mkdir project-segment` | Declares a namespace that no task matches yet, so a later move can land there. |
| `touch`, `mkdir`, or `mv` involving `@Name` | Rejected; an arbitrary filter has no writable inverse to apply to a task. |
| `rmdir @Name` / `rm -rf @Name` | Does not change tasks directly; succeeds once the configured action would change no matching task. The custom view remains configured. |

Notes:

- Creating a task inside a virtual-tag view works: virtual tags are dropped
  from the write filter (`touch x` in `/home/+DUE` creates a task in
  `home`), so the new file appears one level up.
- `mv` is a filter delta: moving from `/home/+next` to `/finances/+urgent`
  sets project `finances`, removes `next`, adds `urgent`. Renaming a task
  file (changing its description) is not supported in this version.
- Moving into a read-only virtual-tag view (`+DUE`, `+OVERDUE`, ...) fails,
  since those statuses are derived, not assignable.
- `mkdir` declarations last for the mount session only; once a task lands
  there the directory exists from the task data itself.
- Removing a project or tag directory applies the configured action to every
  matching task in its subtree. Removing a virtual-tag directory never changes
  tasks; it succeeds once the view has nothing left for the action to change.
- Custom view directories support reading tasks and removing them. `rm` on a task
  file applies the configured action, but `touch`, `mkdir`, and `mv` are rejected. `rm -rf
  "@Due Next Week"` unlinks the matching files under that action and then
  succeeds without removing the custom view, which stays in the root listing.

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
