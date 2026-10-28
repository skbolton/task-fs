# Tasks

## 1. View store and naming

- [x] 1.1 Add `view.Store` in `internal/view` with directory resolution (`$XDG_STATE_HOME/task-fs/views`, falling back to `$HOME/.local/state/task-fs/views`), `List`, `Get`, `Create`, `Put`, and `Delete`; verify `go test ./internal/view` covers creating a missing directory, absolute-versus-relative `XDG_STATE_HOME`, the `HOME` fallback, and an unresolvable location returning an error.
- [x] 1.2 Persist one file per view with user-only permissions and atomic temporary-file-plus-rename writes; verify store tests reopen the store and observe saved filters, and that a failed write leaves the previous definition intact.
- [x] 1.3 Add `view.ValidateName` for non-empty, 254-byte maximum, no `/` or control characters, `.`/`..` rejection, and minted task-file rejection, with distinguishable invalid/existing/missing errors; verify table tests cover every accepted and rejected form and that `Create` reports an existing name separately.
- [x] 1.4 Add store tests for list ordering and filter text with spaces, newlines, `@`, and `{{ ... }}` blocks; verify `go test ./internal/view` passes and no task process is invoked.

## 2. Store-backed view tree

- [x] 2.1 Replace `vfs.Config.Views` and `vfs.Config.RenderView` with a `ViewStore` interface and a `RenderFilter(ctx, raw)` callback; make a nil store behave as an empty read-only store and inject a renderer that reads and renders the stored source; verify existing VFS tests that do not use views still pass and `go build ./...` succeeds.
- [x] 2.2 Make root listing and lookup read view names from the store, list views first and sorted, deduplicate `@NAME` against project entries, and resolve `@NAME` as a view before a project of the same name; verify VFS tests cover a view shadowing an `@`-prefixed project and the project resolving again after the view is deleted.
- [x] 2.3 Implement root `mkdir @NAME` to create an empty persisted view and root `rmdir @NAME` to delete only the definition without querying or changing tasks; verify VFS tests cover empty-filter views, `EEXIST`, invalid names, removal with matching tasks, and project removal when no view exists.
- [x] 2.4 Make each view directory list `_filter` once defined plus matching task files and resolve only those names, reading the source per access and re-rendering on listing or after the one-second reuse window; verify VFS tests show an edited `_filter` changing the next listing, an empty filter yielding no task files, and an invalid filter yielding `EIO` with no task mutation.
- [x] 2.5 Add the `_filter` file node with read, write, truncate, flush, and release handling that buffers per open handle and persists through the store on flush or fsync; verify VFS tests cover editor-style creation, `echo`-style truncate-and-write, multi-chunk writes, `touch`, read-after-write, and a flush error leaving the prior filter.
- [x] 2.6 Make removing `_filter` clear the definition and hide it while keeping the view listed, and let task-file unlinks fall back to the last successful filter when the source became empty mid-traversal; verify VFS tests cover `rm _filter`, `rm -rf @NAME` removing listed tasks and the definition, and no task mutation from `rmdir @NAME`.
- [x] 2.7 Rewrite the existing custom-view tests and helpers (`newSweepFSWithViews`, `newDynamicViewFS`, `viewNode` resolvers) around a temporary store and remove `vfs.View` usages; verify `go test ./internal/vfs` passes and covers view inode identity per view, task-file xattrs, rejection of moves and creation inside views, and render-failure EIO.
- [x] 2.8 Update the README path-grammar, custom-view, and gesture sections for `@NAME/_filter`, empty views, per-access edits, `_filter` removal, and `rmdir` versus recursive view removal; verify each documented example matches the implemented behavior and no README example defines a view with `--view`.

## 3. Mount command

- [x] 3.1 Remove `--view`, `viewFlag`, `compiledViews`, `parseViews`, `validateViews`, and the view entry in `reorderFlags`; resolve and create the store in `run`, inject `ViewStore` and the renderer into `vfs.Config`, and update the usage string; verify `go test ./cmd/task-fs` and a manual `--view` invocation both fail with the usage message and mount nothing.
- [x] 3.2 Replace the mount-flag and view-parsing tests with tests for `--rm` positions, `--view` rejection in any position, missing mountpoint, and store-construction failure; verify `go test ./cmd/task-fs` passes.
- [x] 3.3 Update the README mount section and migration guidance to remove `--view`, document `@NAME/_filter`, the state directory, cross-database sharing, and how to recreate former flag-defined views; verify the documented commands match the CLI and the README contains no `--view` examples.
- [x] 3.4 Import pre-existing `@NAME` directories from the mountpoint at startup, using directory and `_filter` modification times so newer mountpoint definitions update the store without overwriting newer runtime definitions; verify store tests cover new, empty, newer-source, newer-store, and invalid entries and a smoke mount shows pre-created views.

## 4. Home Manager module

- [x] 4.1 Remove `--view` from `nix/home-manager.nix` examples and description, document that views are created through the mounted tree and persist under the service user's state directory, and keep the coreutils/shell baseline for `_filter` command blocks; verify the module text mentions no `--view` and the `--rm` example still describes removal semantics.
- [x] 4.2 Remove the `viewFlagged` and `dynamicViewFlagged` cases (or replace them with a `--view`-rejection expectation) and keep the `--rm` and quoting checks in `nix/hm-module-test.nix`; verify the module check builds and no generated `ExecStart` advertises `--view`, `--virtual-tags`, or `--virtual-hidden`.
- [x] 4.3 Confirm the automount environment exposes `HOME`/`XDG_STATE_HOME` as the user session does or document the fallback; verify the module check covers the documented state-directory resolution.

## 5. Integration verification

- [x] 5.1 Run `go test ./...` and `go vet ./...`; verify all packages pass with no leftover references to `vfs.View`, `--view`, or mount-time view validation.
- [x] 5.2 Run the Home Manager module check (`nix build .#checks.<system>.hm-module` or the current system equivalent); verify it passes.
- [x] 5.3 Smoke-test a temporary `TASKDATA` and `XDG_STATE_HOME`: create `@Due` with `mkdir`, write a filter with `echo`, list the matching tasks, edit the filter and list again, remount and confirm the view persists, `rmdir @Due` and confirm tasks are unchanged, and confirm `--view` fails the mount.
- [x] 5.4 Run `openspec validate "runtime-view-queries" --strict` and verify the change remains valid.

## Workflow follow-up

- Archive the change after review and verification are complete.
