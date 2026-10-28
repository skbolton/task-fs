# Proposal

## Why

Custom views are currently fixed at mount time with repeatable `--view NAME=FILTER` flags, so creating or changing a saved filter means editing the mount command (or a Home Manager configuration) and remounting. Users want saved lookups to be ordinary things they can create, edit, and delete while the filesystem is mounted, the same way projects and tags appear and disappear through file gestures.

## What Changes

- Define custom views through the mounted tree itself: a root directory named `@NAME` is a custom view, and its `_filter` file holds the Taskwarrior filter for that view.
- Allow runtime lifecycle: `mkdir @NAME` creates an empty view, writing `_filter` sets or replaces its filter, and removing the view definition removes it from the root without changing tasks.
- Persist view definitions on the host so they survive unmount/remount without any mount flag. The store is `$XDG_STATE_HOME/task-fs/views`, falling back to `$HOME/.local/state/task-fs/views`, with one file per view containing its filter text.
- Read the filter from `_filter` on each view access, so edits take effect without remounting. `_filter` supports the existing `{{ command }}` template syntax, rendered per access.
- Import `@NAME` directories already present under the mountpoint when the mount starts, so views staged outside the mount appear; newer mountpoint definitions update stored ones.
- A view with no `_filter` is a no-op: `mkdir @NAME` leaves it listed but empty. Creating `_filter` (even empty, with `touch` or an editor) makes it visible, and writing a filter populates the view.
- **BREAKING** Remove the `--view` mount option. Passing it SHALL fail the mount with a usage error; runtime `_filter` views are the only way to define a custom view.
- Keep custom views root-only: `@NAME` directories are not views inside project, tag, virtual-tag, or view directories, and a view path does not compose with filter segments.
- Keep view contents read-only as lookups: a view lists `_filter` when defined plus one file per matching task across projects, and does not expose project, tag, virtual-tag, or nested view directories. Creating or moving task files into or out of a view remains rejected; removing a task file still applies the configured removal action.
- Update the README and Home Manager module so `--view` examples and checks are replaced with runtime view guidance and the state directory location is documented.

Out of scope: per-database view stores, composing a view with filter segments, writing task notes through files, and changing project/tag navigation or the removal action.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `mount-lifecycle`: Removes the `--view` mount option and its mount-time name/filter/template validation, and adds resolution of the persistent runtime view store.
- `task-navigation`: Replaces mount-configured `@NAME` views with runtime `@NAME/_filter` views, including creation, listing, lookup, name precedence, and empty/persisted definitions.
- `task-mutations`: Defines `_filter` reads and writes and view-directory removal as configuration gestures that do not change tasks, while keeping task-file removal and the rejection of task creation/moves inside views.
- `home-manager-module`: Removes `--view` from `mountFlags` guidance and checks, and documents where runtime views are stored for the automount service.

## Impact

- CLI: `cmd/task-fs/main.go` loses view parsing, mount-time validation, and the `--view` flag; it gains a view store rooted in the XDG state directory.
- Backend: no change to task queries or mutations; views continue to use the existing opaque `ExportQuery` and `SweepQuery` paths.
- VFS: `internal/vfs/tree.go` replaces the in-memory `View` list with a view store, adds `_filter` file nodes on view directories, and adds root `mkdir`/`rmdir` handling for `@NAME`.
- New package surface: view store persistence and name validation live under `internal/view` alongside the existing template parser and shell runner.
- Tests: CLI view-flag tests are removed or rewritten around `--view` rejection; VFS custom-view tests construct a temporary store and cover creation, editing, listing, lookup, and removal.
- Nix: `nix/home-manager.nix` and `nix/hm-module-test.nix` stop advertising `--view` and keep only removal-flag and quoting checks.
- Documentation: `README.md` mount, path-grammar, gesture, and Home Manager sections describe runtime views and the state directory.
- Task data is untouched by the view lifecycle; existing mounts must be remounted with the new binary, and any `--view` flags removed. Existing mount-time views must be recreated once as `@NAME` directories with a `_filter` file.
