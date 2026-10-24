# Tasks

## 1. Backend removal action

- [x] 1.1 Add `RemoveAction` with `RemoveComplete`/`RemoveDelete` and `ParseRemoveAction` to `internal/backend`; verify a table test accepts `complete` and `delete` and rejects other values (including empty) with an error naming the input
- [x] 1.2 Make the sweep action-aware: `Filter.RenderSweep`/`Query.SweepTasks` take the action, `delete` emits `status.not:deleted`, and `complete` additionally emits `status.not:completed`; verify backend tests assert the rendered arguments for both actions and that no completed task remains in the complete sweep
- [x] 1.3 Add `Query.Remove(uuid, action)` dispatching to the completion (`done`) or deletion (`delete`) command; verify fake-runner tests assert the exact command for each action and that the UUID is passed unchanged

## 2. VFS removal behavior

- [x] 2.1 Add the removal action to `vfs.Config`, store it on `FS`, and update `New`; verify `go build ./...` and adapt test constructors that build `Config` values
- [x] 2.2 Extend the `sweepDB` fake in `internal/vfs/tree_test.go` to apply `complete` by marking tasks completed and to match `status.not:completed`; verify the existing `internal/vfs` suite still passes
- [x] 2.3 Update `dirNode.Unlink` to call the configured removal action, keeping the deleted-tombstone no-op and adding a completed-task no-op under `complete`; verify tests cover complete, delete, completed-file, and deleted-file removal scenarios
- [x] 2.4 Update destructive `dirNode.Rmdir` to sweep with the action-aware query and apply the configured action per UUID; verify tests cover completing a project subtree and a tag across projects under `complete`, deleting the same under `delete`, skipping deleted tasks, and ENOENT for an unknown name
- [x] 2.5 Use the action-aware sweep for virtual-tag view removal; verify under `complete` that `rmdir .+COMPLETED` succeeds with completed tasks present, `rmdir .+PENDING` still returns ENOTEMPTY with pending tasks, a `rm -rf this`-shaped walk with pre-existing completed tasks converges, and `delete` behavior is unchanged
- [x] 2.6 Branch the `Rename` status destination on the destination's status tag so `+DELETED`/`.+DELETED` deletes and `+COMPLETED`/`.+COMPLETED` completes; verify tests cover both DELETED spellings, moving out of DELETED reopening and re-locating the task, the contradictory-destination error, and that the existing COMPLETED tests still pass
- [x] 2.7 Update the README gesture section for the configured `rm` behavior, directory removal, and the move into `+DELETED`; verify each documented gesture matches an implemented code path

## 3. Mount flag and wiring

- [x] 3.1 Add `--rm` to `parseMountFlags` with default `complete`, validation through `ParseRemoveAction`, usage text, and the `reorderFlags` value-token set; verify `main_test.go` covers the default, explicit `delete`, `--rm=complete`, an invalid value, and flag-before/after-mountpoint positions
- [x] 3.2 Pass the parsed action into `vfs.Config` in `run`; verify `go test ./cmd/task-fs` and confirm an invalid action fails before any mount attempt
- [x] 3.3 Document `--rm complete|delete`, the `complete` default, and the breaking change in the README mount-flags section; verify the text matches the flag help and usage string

## 4. Integration verification

- [x] 4.1 Run `go vet ./...`, `go test ./...`, and `openspec validate configurable-rm-action`; confirm all pass
- [x] 4.2 Against a scratch `TASKDATA`/`TASKRC`, verify with a real mount: default `rm` completes a task; `--rm delete` deletes one; `mv` into `+DELETED` with `--virtual-tags ALL` deletes one and moving out reopens it; `rm -rf` on a project that has pending and completed tasks exits cleanly under the default action; confirm the resulting statuses with `task export`

## Workflow follow-up

- Archive the change after the project's review requirements are satisfied.
- Verify the archived result.
