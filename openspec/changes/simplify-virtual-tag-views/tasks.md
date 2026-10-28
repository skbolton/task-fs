# Tasks

## 1. Fixed backend vocabulary

- [x] 1.1 Add `backend.StatusVirtualTags()` returning `COMPLETED` then `DELETED` from the existing status map, and verify a backend unit test asserts the exact names and order and that `IsVirtualTag` reports both.
- [x] 1.2 Restrict `Filter.WithVirtualSegment` to the `+NAME` spelling of a status tag and reject dot-prefixed or non-status names; verify backend tests accept `+COMPLETED`/`+DELETED`, reject `.+COMPLETED` and `+DUE`, and still treat `+DUE` as a tag segment through `WithSegment`.
- [x] 1.3 Update `Filter` tests that used `DUE`, `PENDING`, or `OVERDUE` as virtual tags so they exercise the status pair or plain tags; verify `go test ./internal/backend` passes.

## 2. Tree simplification

- [x] 2.1 Remove `Config.VirtualTags` and `Config.Hidden` from `internal/vfs`, make `FS` use `backend.StatusVirtualTags()` and `backend.IsVirtualTag`, and list `+COMPLETED` and `+DELETED` directly in every filter directory without nesting; verify VFS tests show both entries in the root and in a project directory even when nothing matches, and no `+` entries inside a status view.
- [x] 2.2 Remove dot-prefixed resolution and the hidden-presentation helpers so `.+COMPLETED`, `.+DELETED`, and other dot names fail with ENOENT while `createChild`/`declareChild` continue to reject dot names; verify VFS tests cover both status spellings and that `mkdir`/`touch` with a dot name fails.
- [x] 2.3 Remove `hasNonStatusVirtual` and the non-status virtual destination rejection, leaving `statusRemoveAction` as the only status-to-action mapping; verify VFS rename tests still complete into `+COMPLETED`, delete into `+DELETED`, and reject contradictory status destinations.
- [x] 2.4 Rewrite VFS presentation, spelling, nesting, self-view, and removal-guard tests around the fixed pair (for example delete-action guard via `+COMPLETED` and recursive walks through `+COMPLETED`), and rewrite virtual-tag rename tests that used `DUE`/`OVERDUE`; verify `go test ./internal/vfs` passes and no test constructs a virtual tag outside `COMPLETED`/`DELETED`.

## 3. Mount interface

- [x] 3.1 Remove `--virtual-tags`/`-t` and `--virtual-hidden`/`-H` from the usage string, `parseMountFlags`, and the `reorderFlags` value-flag map, returning only mountpoint, removal action, and compiled views; verify CLI tests show each removed flag exits with the usage error and mounts nothing, including mixed positions after the mountpoint.
- [x] 3.2 Remove the `AllTags` query and selection/default resolution from `run`, and pass a `vfs.Config` with only the removal action, views, and renderer; verify `go test ./cmd/task-fs` passes and a flagless parse produces no virtual-tag or hidden inputs.
- [x] 3.3 Remove the now-unused `Query.AllTags`, `VirtualTagsFromDiscovery`, `DefaultVirtualTagSelection`, `DefaultVirtualTags`, and `ResolveVirtualTags` plus their tests; verify `rg` finds no references and `go test ./...` passes.

## 4. Documentation and Home Manager

- [x] 4.1 Rewrite the README mount section, path grammar, gestures table, and Home Manager `mountFlags` example so they describe the fixed `+COMPLETED`/`+DELETED` views, custom views such as `--view 'Due=+DUE'` for other derived filters, and the removed flags; verify every documented command matches the implemented CLI and no README example passes `--virtual-tags`, `-t`, `--virtual-hidden`, or `-H`.
- [x] 4.2 Update `nix/home-manager.nix`'s `mountFlags` example and description to drop the removed flags and use a custom view, and update `nix/hm-module-test.nix`'s flagged case and assertion to check a `--view` value; verify no module text advertises the removed flags.
- [x] 4.3 Run the Home Manager module check (`nix build .#checks.x86_64-linux.hm-module` or the current system equivalent) and verify it passes with the updated expectations.

## 5. Integration verification

- [x] 5.1 Run `go test ./...` and `go vet ./...` and verify all packages pass.
- [x] 5.2 Smoke-test a temporary `TASKDATA` mount: confirm `+COMPLETED` and `+DELETED` are listed and empty on a fresh database, completing and deleting tasks through their `+` destinations works, a custom view using `+DUE` renders, `.+COMPLETED` fails with ENOENT, and `--virtual-tags` or `--virtual-hidden` fails the mount.
- [x] 5.3 Run `openspec validate "simplify-virtual-tag-views" --strict` and verify the change remains valid.

## Workflow follow-up

- Archive the change after review and verification are complete.
