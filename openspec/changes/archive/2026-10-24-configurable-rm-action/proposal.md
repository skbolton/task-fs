# Proposal

## Why

Today completing a task requires moving its file into `+COMPLETED`, while `rm` permanently deletes it. Completing is the far more common way to close a task, so the destructive gesture should not be the default. Users need a mount-time choice between completing and deleting when they remove a task file or remove a directory recursively.

## What Changes

- Add `--rm complete|delete` to `task-fs mount`, defaulting to `complete`, selecting what the removal gestures do to the matching tasks:
  - `rm <taskfile>` marks the task complete (`complete`) or deletes it (`delete`).
  - Removing a directory, including the recursive `rm -rf <dir>` walk, applies the same action to every matching task in that directory's filter scope.
- **BREAKING**: the default removal action changes from deletion to completion. A plain `task-fs mount` now completes tasks removed with `rm`/`rmdir`; `--rm delete` restores today's behavior.
- Tasks that are already completed or deleted are left untouched by the `complete` action, since Taskwarrior rejects those repeated transitions; deleted tombstones remain a no-op in both modes.
- Moving a task file into the DELETED status view (`+DELETED` / `.+DELETED`) deletes the task, symmetric with moving into COMPLETED completing it. Moving out of DELETED reopens the task. The DELETED view is available only when selected with `--virtual-tags`; the default virtual-tag selection is unchanged.
- The virtual-tag directory removal rule becomes action-aware so recursive removal still converges in complete mode: a view blocks its own removal only while it holds tasks the configured action would change. In delete mode this is the existing behavior; in complete mode the completed view no longer blocks removal because its tasks are already at the target status.
- README documents the flag, the changed default, and the DELETED move gesture.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `mount-lifecycle`: the mount command gains `--rm complete|delete`, its default, and validation that fails the mount on any other value.
- `task-mutations`: `rm` and directory removal become configured completion or deletion gestures; moving into DELETED deletes, and repeated completion/deletion is a no-op.
- `task-navigation`: the virtual-tag directory removal (emptiness) rule depends on the configured removal action.

## Impact

- `cmd/task-fs/main.go`: flag parsing (including value-token reordering), validation, usage text, and wiring the action into the vfs config.
- `internal/backend`: removal-action type and parsing, action-aware sweep rendering/query, and dispatch to completion (`done`) or deletion (`delete`).
- `internal/vfs/tree.go`: removal action in the mount config; `Unlink`, `Rmdir`, virtual-view removal, and the DELETED branch of `Rename`.
- Tests: `cmd/task-fs/main_test.go`, `internal/vfs/tree_test.go`, and backend tests for parsing and sweep rendering.
- `README.md`: mount flags and gesture documentation.
- No new dependencies, no database or on-disk format changes, and no changes to virtual-tag defaults.
