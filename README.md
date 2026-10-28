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

Every mount exposes two virtual-tag views: `+COMPLETED` and `+DELETED`.
They are always listed, even on an empty database, because moving a task
into either one is how you complete or delete it.

Other Taskwarrior derived filters, such as `DUE` or `OVERDUE`, are custom
views. A custom view is a root directory whose name starts with `@` and whose
`_filter` file holds the Taskwarrior filter. Create one while the filesystem is
mounted:

```sh
task-fs mount ~/c/TaskFS
cd ~/c/TaskFS
mkdir @Due
vim @Due/_filter          # create it and write: status:pending +DUE
# or: echo 'status:pending +DUE' > @Due/_filter
ls @Due
```

- `@NAME` exists only at the root. Its `_filter` runs exactly as written, so
  include `status:pending` yourself when you want pending tasks only.
- A view with no `_filter` is a no-op: it is listed at the root but its
  directory is empty. This is the state right after `mkdir`; create `_filter`
  with your editor, `touch`, or a shell redirection, then list the view again.
- An empty `_filter` is a real file and is listed, but the view shows no task
  files until the filter is written. Removing `_filter` returns the view to the
  no-op state; truncating it leaves an empty `_filter` visible.
- Editing `_filter` changes the next listing without remounting. A malformed
  filter, invalid template, or failing command makes that view fail with
  `EIO` and writes the view name and error to the mount process's standard
  error.
- View definitions persist across mounts in
  `$XDG_STATE_HOME/task-fs/views`, falling back to
  `$HOME/.local/state/task-fs/views`. One file per view holds the `_filter`
  text. Views are user-scoped and shared by every Taskwarrior database you
  mount.
- `@NAME` directories already present under the mountpoint are imported when
  the mount starts, so a view staged while unmounted appears too. If both a
  stored definition and a mountpoint copy exist, the newer `_filter` (or
  directory) wins. Runtime edits made through the mount stay in the state
  store and are not written back into the mountpoint.

You can stage a view before mounting. The mountpoint directory is hidden once
the filesystem mounts, so task-fs imports these directories during startup:

```sh
mkdir -p ~/c/TaskFS/@Due
echo 'status:pending +DUE' > ~/c/TaskFS/@Due/_filter
task-fs mount ~/c/TaskFS
```
- The former `--view` flag is removed; passing it aborts the mount with a
  usage error. Recreate a former `--view NAME=FILTER` view once with
  `mkdir "@NAME"` and write `FILTER` into `"@NAME/_filter"`.

A `_filter` may contain one or more `{{ command }}` blocks. Each command runs
through `/bin/sh -c` in the mount environment, and its trimmed standard output
replaces the block as filter text. Use a backslash to keep literal braces:
`\{{ not a command }}`.

```sh
mkdir @Payday
echo 'status:pending due.before:{{ payday-date }}' > @Payday/_filter
mkdir '@Next Sprint'
echo 'status:pending due.before:{{ date -d "+14 days" +%F }}' > '@Next Sprint/_filter'
```

Single-quote the whole filter when writing it from an interactive shell so your
shell does not expand the command itself.

- Command output is trimmed and spliced as a filter expression. Empty output,
  NUL bytes, and embedded newlines are rejected. Each command has a
  five-second timeout.
- A long-lived mount re-renders the block when the view is browsed. A listing
  updates the value immediately; lookups within the same traversal reuse the
  rendered value for up to one second. Removal gestures render fresh.
- A render failure makes the view operation fail with `EIO` and writes the
  view name, command, and stderr to the mount process's standard error. Under
  Home Manager that is the user service journal.
- A view directory lists `_filter` once it exists, plus the matching task files
  flat across all projects. It does not expose project, tag, virtual-tag, or
  nested view directories. The view path does not compose with other segments:
  `@NAME/home` is ENOENT.
- Custom view directories are for reading and removing tasks. `rm` on a task
  file applies the configured `--rm` action. `touch`, `mkdir`, and `mv` of
  task files are rejected: an arbitrary filter has no known set of task
  changes that would move a task into or out of it. `_filter` is writable.
- `rmdir @NAME` deletes the saved view without changing any task, even when
  tasks match it. Use this to discard a view. `rm -rf @NAME` unlinks the listed
  task files under the configured action first, then deletes the view.
- Removing `@NAME/_filter` removes the filter; `@NAME` remains listed as a
  no-op until `_filter` is created again.
- A runtime view shadows a real project whose first root segment has the same
  `@NAME`; the project becomes reachable again when the view is deleted.

A mount flag selects what removing a task does:

```sh
task-fs mount ~/c/TaskFS --rm delete
```

- `--rm complete|delete` selects the action for `rm` and for removing
  directories: `complete` (the default) completes the matching tasks, while
  `delete` permanently deletes them. **This changes the previous default**:
  a flagless mount now completes tasks removed with `rm` or folder deletion;
  pass `--rm delete` to keep the old destructive behavior.

The former `--virtual-tags`/`-t` and `--virtual-hidden`/`-H` flags are also
removed; passing either aborts the mount with a usage error.

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
`services.task-fs.automount.mountFlags`, a list of strings where each item is
one command-line token:

```nix
services.task-fs.automount.mountFlags = [
  "--rm"
  "delete"
];
```

Commands in `{{ ... }}` blocks run with the automount service `PATH`. Add
packages whose executables you invoke by name:

```nix
services.task-fs.automount.extraPackages = [ pkgs.delta.cycle ];
```

That lets a `_filter` use `{{cycle last week --end}}` without an absolute Nix
store path. Commands outside the configured `PATH` still need an absolute path
or a wrapper script.

Runtime views are created through the mounted tree rather than in the mount
flags. Their definitions live under the automount user's state directory
(`$XDG_STATE_HOME/task-fs/views`, or `$HOME/.local/state/task-fs/views`), so
they survive service restarts and are shared with views created from an
interactive mount.

The automount service provides a POSIX shell and coreutils, so `{{ ... }}`
blocks in a `_filter` can use commands such as `date`. Commands outside that
baseline must use an absolute Nix store path or an executable wrapper script,
because the service does not inherit your interactive shell's `PATH`. The
mount command's shell environment does include the session's Taskwarrior
configuration and `TASKDATA`.

## The path grammar

A path in the filter tree is a task filter; custom view directories are saved
filters and do not accumulate path segments. Filter segments are order-free;
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
- **`+COMPLETED` and `+DELETED` are virtual tags**: they map to
  Taskwarrior's completed and deleted status filters and are always present
  in every filter directory. They are the only built-in virtual-tag views.
  Other derived filters (`+DUE`, `+OVERDUE`, ...) are not listed by the tree;
  define them as runtime views with a `_filter` file. The dot-prefixed
  spellings `.+COMPLETED` and `.+DELETED` no longer resolve.
- **`@` segments are custom views**: `@Due Next Week`, `@Overdue`, ...
  exist only at the root, when a saved view definition with that name exists.
  They are lookup views, not filter directories: their entries are `_filter`
  once defined plus the matching task files flat across all projects, and they
  do not list or resolve project, tag, virtual-tag, or nested view segments.
- **Filter directories are pending by default**: without a status segment
  a filter directory shows pending tasks, and under `+COMPLETED` it shows
  completed tasks. Custom view directories are the exception: their filter is
  used exactly as written, with no implicit status.

Only segments that exist — derivable from your tasks, declared with `mkdir`
(below), or named by a saved runtime view — resolve. A typo fails with ENOENT
instead of showing an empty directory:

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
| `mv <taskfile> +COMPLETED/` | Completes the task (rest of the destination path is ignored). |
| `mv <taskfile> +DELETED/` | Deletes the task (rest of the destination path is ignored). |
| `mv <taskfile> out of +COMPLETED or +DELETED` | Reopens the task and applies the destination's project/tags. |
| `mv <taskfile> <other-dir>` | The task's project and tags match the destination: projects set exactly (moving to a parent drops the subproject), tags added or removed per the filter delta. |
| `mv <taskfile> /new/deep/dir` (no trailing slash) | The final component is minted as a new project segment, provided the parent path exists. |
| `mkdir +tag` or `mkdir project-segment` | Declares a namespace that no task matches yet, so a later move can land there. |
| `mkdir @Name` | Creates a saved runtime view with no `_filter`; the view is a no-op until `_filter` is created. |
| `touch @Name/_filter` | Creates an empty `_filter`; the view lists it but has no tasks until a filter is written. |
| `vim @Name/_filter` or `echo 'FILTER' > @Name/_filter` | Creates or replaces the view's filter; it takes effect on the next listing. |
| `rm @Name/_filter` | Removes the filter; the view remains listed as a no-op. |
| `touch`, `mkdir`, or `mv` involving task files in `@Name` | Rejected; an arbitrary filter has no writable inverse to apply to a task. |
| `rmdir @Name` | Deletes the saved view without changing any task. |
| `rm -rf @Name` | Unlinks the listed task files under the configured action, then deletes the view. |

Notes:

- Creating a task inside a status view works: virtual tags are dropped from
  the write filter (`touch x` in `/home/+COMPLETED` creates a task in
  `home`), so the new file appears one level up.
- `mv` is a filter delta: moving from `/home/+next` to `/finances/+urgent`
  sets project `finances`, removes `next`, adds `urgent`. Renaming a task
  file (changing its description) is not supported in this version.
- Only `+COMPLETED` and `+DELETED` are virtual-tag destinations. Any other
  uppercase `+` name is an ordinary tag. Runtime view task files are read-only
  and cannot be moved; the only writable entry in a view is `_filter`, which
  you may create when it does not exist.
- `mkdir` declarations last for the mount session only; once a task lands
  there the directory exists from the task data itself.
- Removing a project or tag directory applies the configured action to every
  matching task in its subtree. Removing `+COMPLETED` or `+DELETED` never
  changes tasks; it succeeds once the view has nothing left for the action to
  change.
- Custom view directories support reading tasks and removing them. `rm` on a
  task file applies the configured action, but `touch`, `mkdir`, and `mv` of
  task files are rejected. Use `rmdir "@Due Next Week"` to discard the view
  without changing tasks; `rm -rf` also unlinks the matching task files under
  the configured action first.
- Runtime views persist across mounts and are shared across Taskwarrior
  databases. A malformed `_filter` is accepted by the write but makes the view
  fail with `EIO` until it is corrected.

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
