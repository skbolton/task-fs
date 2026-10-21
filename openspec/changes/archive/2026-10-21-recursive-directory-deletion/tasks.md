# Tasks

## 1. Backend sweep primitives

- [x] 1.1 Add `Filter.RenderSweep()` in `internal/backend/filter.go` emitting project/tag args plus `status.not:deleted` and never the implicit `status:pending`; verify with a table test in `internal/backend/filter_test.go` (or the existing filter test file) covering pending-only filters, tagged filters, and filters that already carry a status virtual tag
- [x] 1.2 Add `Query.SweepTasks(f Filter) ([]Task, error)` in `internal/backend/query.go` exporting with the sweep render; verify with a runner-fake test asserting the rendered args include `status.not:deleted` and the export JSON is parsed

## 2. Rmdir semantics in the VFS

- [x] 2.1 Store the `backend.Filter` with each declared segment in `FS` (declared map value becomes a struct) and adapt `declare`/`undeclare`/`isDeclared`/`declaredSegments`; verify `go build ./...` and the existing `internal/vfs` tests still pass
- [x] 2.2 Rewrite `dirNode.Rmdir`: virtual-tag names return `ENOTEMPTY` when `SweepTasks` of the child filter is non-empty and succeed otherwise without deleting; all other resolvable or declared names sweep tasks by UUID via the existing `query.Delete`, then undeclare; unresolvable unknown names keep `ENOENT`; `WithSegment` errors map to `EINVAL` and export errors to `EIO`; verify with unit tests in `internal/vfs/tree_test.go` for each branch
- [x] 2.3 Purge declarations under a removed directory: on a destructive rmdir, drop declared entries whose project equals or is a dotted descendant of the removed child's project and whose tags include the child's; verify with a test that a ghost declared subproject no longer appears in listings after `rmdir` of its parent

## 3. Deletion gesture tests (spec scenarios)

- [x] 3.1 Cover the `task-mutations` delta scenarios in `internal/vfs/tree_test.go`: rmdir of a project deletes pending tasks at exact and subproject depth plus a completed task and removes the directory from listings; rmdir of a root tag directory deletes tagged tasks across projects; a project whose tasks are all already deleted is removed without re-deleting them; unknown name returns `ENOENT` with no backend mutations
- [x] 3.2 Cover the `task-navigation` delta scenarios: a `rm -rf`-shaped sequence (unlink every task file inside `. +PENDING` and `.+COMPLETED` views, then rmdir those views, then rmdir the project) completes with no errors and removes the project; rmdir of a virtual-tag view that still has matching tasks fails `ENOTEMPTY` and changes no data; rmdir of a declared in-use segment deletes its tasks and removes the declaration

## 4. Integration verification

- [x] 4.1 Run `go vet ./...` and `go test ./...` and confirm the full suite passes
- [x] 4.2 Mount against a scratch `task` data directory (`TASKDATA`/`TASKRC`), create a project with pending and completed tasks plus subprojects, run `rm -rf <project>` and `rmdir <project>` from the mount, and verify both complete without errors, all non-deleted tasks in the subtree are gone from `task export`, and the mount shows no ghost directories

## Workflow follow-up

- Archive the change after the project's review requirements are satisfied.
- Verify the archived result.
