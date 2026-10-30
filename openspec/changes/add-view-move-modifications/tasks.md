# Tasks

## 1. View definition format and store

- [x] 1.1 Rename the view definition constant and store surface from `_filter` to `_view` (`ViewFileName`, `HasView`, `DeleteViewFile`, and the internal no-file marker), keeping one raw definition file per view; verify `go test ./internal/view` passes after updating store tests and that reopening a store preserves single-line and two-line definitions.
- [x] 1.2 Add definition splitting in `internal/view` (first line filter, optional non-empty second line, blank second line read-only, more than two non-empty lines invalid, CRLF and trailing-newline tolerance); verify table tests cover each form and error case with `go test ./internal/view`.
- [x] 1.3 Add quote-aware modification argument splitting in `internal/view` with single quotes, double quotes, backslash escapes, and rejection of unterminated quotes, without variable expansion or globbing; verify tests cover `description:"call mom"`, escaped spaces, empty quotes, and malformed input.
- [x] 1.4 Update `ImportViews` to read `@NAME/_view` and store its raw bytes, ignore legacy `_filter`, and keep the newest-definition rule; verify store tests cover new, empty, two-line, newer-source, newer-store, legacy-only, and invalid entries.

## 2. View tree and rendering

- [x] 2.1 Rename the VFS view file node and all `_filter` references to `_view`, including listing, lookup, creation, write/truncate, flush, unlink, and clear-definition behavior; verify the existing view file tests are renamed and pass with `go test ./internal/vfs`.
- [x] 2.2 Extend the render cache to carry the filter and the rendered modification arguments from one `_view` source, render both non-empty lines through the existing template callback, and surface parse, template, command, and extra-line failures as `EIO` with the view name in stderr; verify dynamic-view tests cover filter rendering, modification rendering, cache reuse, forced refresh, and failures.
- [x] 2.3 Keep listings and task-file lookup driven only by the first line while making writability depend on the second line; verify tests show that editing only the second line does not change listed files and that a blank second line remains read-only.
- [x] 2.4 Add tests for creating, truncating, and removing `_view` with a two-line definition, including write-then-move without remounting; verify `go test ./internal/vfs` covers the definition lifecycle.

## 3. Moving tasks into writable views

- [x] 3.1 Add the view destination branch to `dirNode.Rename`: for a writable view with the same minted task-file name, resolve the task in the source directory, render fresh modification arguments, and call `Query.Modify`; verify the fake runner observes exactly those arguments and no project/tag delta.
- [x] 3.2 Keep read-only views, renames during a move, source views, moves out of views, and moves between views rejected with `EACCES`; verify tests cover each gesture and confirm no task mutation occurs.
- [x] 3.3 Verify moves into a writable view from a status virtual-tag directory apply only the modification line and leave status unchanged, and that standard `mv file @NAME` and root rename onto `@NAME` reach the same branch.
- [x] 3.4 Verify a failed template render, failed command, unterminated quoting, or Taskwarrior rejection returns `EIO` with no task change, and that a successful modification which leaves the task outside the filter makes the file disappear on the next listing.

## 4. Mount, documentation, and module

- [x] 4.1 Update the mount renderer wiring and startup import for `_view`, including rendering the modification line on demand; verify `go test ./cmd/task-fs` passes and no `_filter` references remain in the CLI.
- [x] 4.2 Rewrite the README custom-view, path-grammar, gesture, and Home Manager sections for `_view`, the optional modification line, writable-view moves, the read-only rule, the task-disappears behavior, and the `_filter` migration; verify every documented command matches the implemented behavior and no stale `_filter` reference remains.
- [x] 4.3 Update `nix/home-manager.nix` descriptions and `nix/hm-module-test.nix` checks from `_filter` to `_view` and document the modification line; verify the module check builds with the project's Nix command and its assertions pass.

## 5. Integration verification

- [x] 5.1 Run `gofmt -l`, `go vet ./...`, and `go test ./...`; verify no formatting output, no vet findings, and all packages pass.
- [x] 5.2 Smoke-test with a temporary `TASKDATA` and `XDG_STATE_HOME`: create `@This Month` with `mkdir`, write a two-line `_view`, move a task in from a project directory, and confirm the modification applies and the file lists under the view; then remove the second line and confirm the move fails with EACCES.
- [x] 5.3 Confirm a mountpoint-staged `@NAME/_view` imports while `@NAME/_filter` is ignored, and that a stored single-line definition still opens as a read-only view after remount.
- [x] 5.4 Run `openspec validate add-view-move-modifications --strict`; verify the change is valid, noting that the informational archive-header issues disappear once `runtime-view-queries` is archived.

## Workflow follow-up

- Archive `runtime-view-queries` before this change so its runtime-view requirements reach the main specs.
- Archive this change after review and verification are complete.
