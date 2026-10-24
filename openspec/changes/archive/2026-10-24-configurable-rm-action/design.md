# Design

## Context

`cmd/task-fs/main.go` parses mount flags with a `flag.FlagSet` plus `reorderFlags`, then builds `vfs.Config{VirtualTags, Hidden}`. Removal is hard-wired in `internal/vfs/tree.go`: `dirNode.Unlink` calls `query.Delete`, and `dirNode.Rmdir` sweeps with `Filter.RenderSweep` (`status.not:deleted`) and deletes each UUID. Virtual-tag views are removed only when `SweepTasks` of the view is empty. `dirNode.Rename` treats any destination with a status virtual tag as completion because `dstHasStatus && !srcHasStatus` calls `query.Done`; the backend already maps both `COMPLETED` and `DELETED` to status filters. See proposal.md for motivation and the delta specs for required behavior.

## Goals / Non-Goals

**Goals:**

- One mount-time action (`complete` or `delete`) shared by file unlink, directory removal, and the recursive walk.
- Prove the action-aware sweep is enough to keep `rm -rf` convergent in both modes.
- Add a symmetric delete-by-move gesture without disturbing existing completion behavior.

**Non-Goals:**

- Changing the default virtual-tag selection; `DELETED` stays opt-in through `--virtual-tags`.
- Special-casing moves between two status views (for example `+COMPLETED` to `+DELETED`); the existing "only act when the source has no status segment" gate stays.
- Bulk `task <filter> done/delete` commands; per-UUID mutation is preserved.
- Trash, undo, or purging tombstones.

## Decisions

### Removal action is a backend type parsed at mount time

Add `backend.RemoveAction` with `RemoveComplete` and `RemoveDelete`, and `ParseRemoveAction(string) (RemoveAction, error)`. `main.go` parses the `--rm` value before backend availability checks or mounting, so an invalid value fails the mount with an error naming it and the accepted values. `vfs.Config` carries the parsed action.

The backend owns Taskwarrior status semantics already (`statusVirtualTags`, `Render`, `RenderSweep`), and both the query layer and the VFS need the same notion, so a shared type prevents parallel string switches. Alternative: pass strings or booleans through the VFS — rejected because validation would spread across packages and boolean parameters hide the meaning at call sites.

`--rm` takes no shorthand: the user asked for the long flag, and adding a letter would collide with future gesture flags without a clear convention.

### One action-aware sweep defines "tasks this gesture would change"

`Query.SweepTasks(f, action)` renders:

- `delete`: the existing `status.not:deleted`.
- `complete`: `status.not:deleted status.not:completed`.

The complete sweep deliberately excludes completed tasks because Taskwarrior's `done` exits non-zero for a task that is "neither pending nor waiting". Waiting tasks are included and completable. The same sweep drives both destructive directory removal and the virtual-view emptiness check, so a view is removable exactly when it holds no task the configured action would change.

Alternative: change only `Unlink`/`Rmdir` and leave the emptiness check on `status.not:deleted`. Rejected: in complete mode an already-completed task in `this/.+COMPLETED` is no longer changed by unlink, so the view never empties and `rm -rf this` fails with `ENOTEMPTY`. With the action-aware sweep, `. +COMPLETED` is empty under `complete` (its `status:completed` plus `status.not:completed` is unsatisfiable) and the recursive walk converges, while `.+PENDING`, `.+DUE`, and similar actionable views still guard removal.

### A single removal dispatch guards no-op transitions

Add `Query.Remove(uuid, action)`, dispatching to the existing `delete` or `done` command according to the action. `Unlink` keeps the existing deleted-tombstone success and adds a completed-task success under `complete`; otherwise it calls `Remove`. `Rmdir` sweeps with the configured action and calls `Remove` per task. `Rename` uses `Remove` with `RemoveDelete` for the DELETED destination.

Taskwarrior rejects `delete` on a deleted task and `done` on a completed task, so the guards are required for recursive walks that revisit files after earlier steps changed their status. They also make retries safe. Keeping `Delete`/`Done` as thin wrappers over the commands is fine; only the removal gesture needs the action-aware entry point.

### DELETED uses the destination's status tag, not a hard-coded completion

In `Rename`, when the destination has a status virtual tag and the source view does not, resolve which one: `COMPLETED` calls complete, `DELETED` calls delete. A destination carrying both status tags is contradictory and unreachable through listings (virtual views do not nest); return `ENOTSUP` instead of picking one. Moving out of either status view keeps the existing `status:pending` modify, which Taskwarrior accepts for deleted tasks and therefore already provides reopen symmetry.

The early-return structure is unchanged: the move-in branch runs before the non-status-virtual rejection and the project/tag delta, which is why a move into DELETED needs no delta handling of its own. Status-to-status moves remain no-ops as today; making them transitions would require an undelete-then-complete sequence outside this change's scope.

### CLI flag reordering learns one more value token

Add `--rm` to `reorderFlags`'s value-taking set so `mount <mountpoint> --rm delete` parses, and add the flag to the usage string. Go's `flag` package handles `--rm=delete` without extra work. The default flag value is `complete`, so an omitted flag and an explicit `--rm complete` are identical.

### README presents the new default as a migration note

Document `--rm` beside the virtual-tag flags, state that `complete` is the default and changes prior `rm` behavior, and add the DELETED move to the gestures table. The existing recursive-removal behavior is not currently documented in the README; mention the directory-removal gesture and its action only where the flag explanation needs it, without expanding the README into a full rmdir reference.

## Risks / Trade-offs

- [Scripts or habits expecting `rm` to delete now complete tasks] → **BREAKING** default is called out in the proposal, README, and flag help; `--rm delete` (or `mountFlags` for Home Manager users) restores the old behavior exactly.
- [Complete-mode `rm -rf <project>` leaves tasks completed rather than removing them] → This is the requested semantics; completed tasks remain browsable in the completed view. Users who want removal must select `--rm delete`.
- [A task transitions statuses between sweep and per-UUID command] → Taskwarrior rejects the no-op transition, the command fails with `EIO`, and a re-run converges; no locking is possible over the CLI.
- [Partial directory sweep on a mid-loop failure] → Same idempotent-retry posture as the existing deletion gesture; first failure surfaces instead of being hidden.
- [Completed-task unlink becomes a silent success under complete] → `rm` of a file in the completed view cannot complete it again; succeeding is consistent with the tombstone rule and keeps recursive walks convergent.
- [`DELETED` is not exposed by default] → Users who want the move gesture must add it to `--virtual-tags` (for example `--virtual-tags COMPLETED,DELETED` or `ALL`); keeping tombstones out of default listings avoids a second behavioral change.

## Migration Plan

The default removal action changes. `--rm delete` reproduces every prior removal behavior, including the virtual-view guard, and Home Manager automount users can add the two tokens to `services.task-fs.automount.mountFlags`. No data or on-disk changes; rollback is removing the flag, which intentionally switches back to completion.

## Open Questions

None.
