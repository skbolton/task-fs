# Proposal

## Why

A real project directory can never be removed: `Rmdir` only removes `mkdir`-declared segments and returns `ENOTEMPTY` for anything derived from query results. Running `rm -rf <project>` therefore fails with an error per hidden virtual-tag directory, and the failed walk has already deleted the tasks in the views it descended into — partial, order-dependent destruction with no way to complete the gesture.

## What Changes

- `rmdir` on any non-virtual-tag directory (project segment, tag segment, or `mkdir`-declared segment) deletes all tasks matching the child filter across statuses, then removes the declaration if one exists. The directory disappears on subsequent listings because it is derived from tasks.
- **BREAKING**: `rmdir +urgent` while tasks carry the tag now deletes those tasks instead of failing with `ENOTEMPTY` and leaving data unchanged.
- Virtual-tag directories (`.+NAME`) are never destructively removable: `rmdir` on one succeeds only when no non-deleted task matches the view; it deletes nothing. This keeps `rm -rf <project>` working, since the recursive walk unlinks the task files inside each view before rmdir-ing it.
- The backend gains the ability to export tasks matching a filter without the default `status:pending` injection, so a directory delete covers pending and completed tasks in the whole project subtree; already-deleted tasks are left alone.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `task-mutations`: new requirement — removing a directory is a deletion gesture over all tasks matching that directory's filter, resolved per task by full UUID.
- `task-navigation`: the declared-segment requirement changes — `rmdir` of a declared, in-use segment now deletes the matching tasks and the declaration; and virtual-tag directories gain a non-destructive removal rule (empty view only).

## Impact

- `internal/vfs/tree.go`: `dirNode.Rmdir` rewritten to resolve the child filter and delete matching tasks; unchanged behavior for nonexistent names (`ENOENT`).
- `internal/backend/filter.go` / `query.go`: a render variant without the implicit `status:pending` (deletion sweep excludes `status:deleted`), plus a query method driving per-UUID deletes from that export.
- Tests: `internal/vfs/tree_test.go` and `internal/backend` tests for the new sweep and the `rm -rf`-shaped sequence.
- No CLI, mount, or dependency changes.
