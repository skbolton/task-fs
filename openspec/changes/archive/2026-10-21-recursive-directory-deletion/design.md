# Design

## Context

`dirNode.Rmdir` (internal/vfs/tree.go) today only removes `mkdir`-declared segments, and only when `query.Export` of the child filter is empty; every query-derived directory gets an unconditional `ENOTEMPTY`. Virtual-tag directories are listed at every level and can never be removed, which is what breaks `rm -rf <project>`. Deletion currently exists only per task file: `Unlink` resolves a minted file name to a UUID and calls `query.Delete(uuid)`. See proposal.md for motivation.

## Goals / Non-Goals

**Goals:**

- `rmdir` on any non-virtual-tag directory deletes all matching tasks (pending and completed, whole project subtree) and removes the directory from listings.
- `rm -rf <project>` completes without errors: the hidden virtual-tag view directories become removable once the walk has emptied them.
- Preserve the "mutations identify tasks by full UUID" guarantee.

**Non-Goals:**

- No change to `Unlink`, `Rename`, `Create`, or task-file semantics.
- No purge of `status:deleted` tasks; no trash/undo facility.
- No new CLI or mount options.

## Decisions

### Per-UUID sweep instead of one bulk `task <filter> delete`

The sweep exports the tasks matching the child filter (across statuses) and calls the existing `query.Delete(uuid)` per task. A single `task rc.confirmation=off project:this delete` would be one round-trip but would identify tasks by filter, weakening the spec guarantee that mutations name full UUIDs, and would make per-task failure attribution impossible. Cost is N invocations, acceptable at task-list scale.

### Status sweep render in the backend

`Filter.Render` injects `status:pending` when the filter has no status virtual tag, so it cannot express "everything not already deleted". Add a render variant (e.g. `Filter.RenderSweep()`) that emits the project/tag args plus `status.not:deleted`, and a `Query` method (e.g. `SweepTasks(f Filter) ([]Task, error)`) that exports with it. Reusing it for the virtual-tag emptiness check keeps one definition of "removable task": pending or completed. Excluding `status:deleted` avoids re-deleting tasks taskwarrior may reject or no-op on, and deleted tasks never keep a directory visible anyway (visibility is derived from the pending view).

### Rmdir branches on segment kind, not on declared-vs-derived

Current logic branches on `isDeclared`; the new logic branches on whether the name is a virtual-tag segment:

- Non-virtual-tag name (project, tag, declared): build the child filter with `WithSegment`; on error `EINVAL`. If the name is neither declared nor resolvable, `HasAnyTask` of the child's write filter (any status, including deleted tombstones) decides: a name that never held a task is `ENOENT`, a tombstoned one proceeds so the directory can be removed and the `rm -rf` walk converges after its own deletions emptied it. Export the sweep set; delete each by UUID (first failure returns `EIO`, leaving partial state — see risks); then purge declarations under the child and `undeclare` the segment. Success even when the set is empty.
- Virtual-tag name (`.+NAME`): never deletes anything. Export the sweep set; non-empty returns `ENOTEMPTY` (a bare `rmdir .+PENDING` with tasks still in the view fails, same errno as today), empty succeeds as a stateless no-op.

This is what makes `rm -rf this` converge: the walk unlinks every visible task file inside `this/.+PENDING` / `this/.+COMPLETED` first (each unlink already deletes by UUID), so those views are empty when their `rmdir` arrives, and the final `rmdir this` sweeps whatever the walk could not see (e.g. completed tasks if no `. +COMPLETED` entry is present, since the virtual-tag list comes from `_tags` discovery).

### Purge stale declarations under a removed directory

`Readdir` lists declared segments unconditionally, so a lingering declaration for a removed subtree (e.g. `mkdir this/sub` earlier in the session) would keep ghost directories in listings after the tasks are gone. Store the `backend.Filter` alongside each declared segment (declared map value becomes a small struct). When a directory is removed, drop declarations whose filter extends the removed child filter: project equal to or a dotted descendant of the child's project, with the child's tags included. Declarations unrelated to the removed subtree are untouched.

### Virtual-tag views do not nest

During live verification, `rm -rf <project>` never finished: every virtual-tag view listed all sibling views, so the tree below a project grew combinatorially (~33 views deep per level). `Readdir` now lists virtual-tag entries only when the accumulated filter carries none. One derived filter per path covers the read-only use cases; the task-navigation delta records the rule.

### Unlink of an already-deleted task is a no-op

The `.+DELETED` view lists tombstone files, and `task <uuid> delete` errors on an already-deleted task, so the recursive walk failed with EIO on them. `Unlink` now checks the resolved task's status and returns success without a delete call, keeping "deleted tasks are left in the deleted status" intact.

### Alternatives considered

- Bulk filter delete — rejected above.
- Making virtual-tag dirs destructively removable like everything else — rejected: users cannot assign virtual tags directly, so a destructive `. +PENDING` rmdir has no coherent gesture meaning, and `rmdir .+PENDING` at the root would wipe the whole database.
- Keeping `ENOTEMPTY` for declared in-use dirs and adding a separate recursive-delete gesture — rejected by product decision: `rmdir` is the delete gesture for all non-virtual-tag directories.

## Risks / Trade-offs

- [Root-level `rmdir +work` deletes every tagged task across all projects] → Accepted product decision; it is the same gesture shape as project deletion. Mitigation is convention, plus the `.+` carve-out keeps the most dangerous accidental case (`.+PENDING` at root) non-destructive.
- [Partial sweep on mid-loop failure leaves some tasks deleted and the directory present] → The gesture is idempotent: re-running `rmdir` deletes the remainder. `EIO` surfaces the failure instead of masking it.
- [Tasks added between the export and the deletes are missed] → Window is small and re-running converges; no lock is available over the taskwarrior CLI.
- [Declaring directories inside a virtual-tag view creates declaration entries whose filter carries virtual tags] → Pre-existing quirk, unchanged; the purge predicate ignores such entries (child filter of a destructive rmdir never carries virtual tags).
