# Tasks

## 1. Template parser and renderer

- [x] 1.1 Add an `internal/view` package with a `Parse` function that turns a raw filter into literal and command segments, handles multiple `{{ command }}` blocks, and turns `\{{` into a literal opening delimiter; verify `go test ./internal/view` covers a static filter, one block, multiple blocks, an escaped delimiter, an empty block, an unclosed block, and a trailing escape.
- [x] 1.2 Add template rendering that trims one trailing newline and surrounding whitespace, rejects empty output, NUL, and embedded newlines, and splices command output as literal filter text; verify a fake runner test covers every accepted and rejected output shape and confirms output is not re-parsed or shell-quoted.
- [x] 1.3 Add a shell runner that invokes `/bin/sh -c` with the mount environment, captures stdout and stderr separately, and enforces a five-second timeout; verify `go test ./internal/view` covers successful output, non-zero exit with stderr, environment inheritance, and timeout cancellation.

## 2. Mount parsing and validation

- [x] 2.1 Compile each `--view` filter during `cmd/task-fs` parsing and fail with an error naming the view when template syntax is invalid; verify `cmd/task-fs` parser tests cover a valid dynamic view, an empty block, an unclosed block, and an escaped delimiter.
- [x] 2.2 Render each compiled view through the shell runner before the existing backend validation and fold command, output, and rendered-filter errors into the existing view-named mount error; verify `cmd/task-fs` tests cover a successful dynamic render, a command failure, empty output, and a malformed rendered filter.
- [x] 2.3 Wire the compiled renderer and shell runner into `vfs.Config` when constructing the root, while leaving static view behavior unchanged; verify `go test ./cmd/task-fs` passes with the existing static view cases and the new dynamic cases.

## 3. VFS rendering and freshness

- [x] 3.1 Add an optional renderer to `vfs.Config` and an `FS` render cache with per-view locking, a one-second reuse window, and a forced-refresh path; verify `internal/vfs` tests show static views return their stored filter and dynamic views use the injected renderer.
- [x] 3.2 Render before custom view listings and task-file lookups, then derive names and task resolution from the rendered filter; verify VFS tests show a dynamic view listing and lookup change when the fake renderer's output changes.
- [x] 3.3 Force a fresh render for `Readdir`, `Unlink`, and custom-view `Rmdir`, while allowing lookups within one traversal to reuse the render; verify a counting fake renderer shows one execution for a listing plus several lookups and a new execution for the next listing.
- [x] 3.4 Map render failures to `EIO` for listings, lookups, task removal, and custom-view directory removal, and write the view name, command, stderr, and exit status to standard error; verify VFS tests cover each operation and assert no task mutation occurs on failure.

## 4. Home Manager runtime

- [x] 4.1 Add coreutils to the automount service `PATH` while keeping the absolute `/bin/sh` runner independent of `PATH`; verify the Home Manager module check's expected environment contains coreutils and still contains taskwarrior3 and fuse3.
- [x] 4.2 Add a module check with a dynamic `--view` value containing spaces and `{{ date +%F }}`; verify the generated `ExecStart` keeps the whole value as one argument with braces unchanged and existing flag checks still pass.
- [x] 4.3 Update the `mountFlags` option description with a dynamic view example and the absolute-path rule for commands outside coreutils; verify the module check evaluating the documented example produces a valid service command.

## 5. Documentation

- [x] 5.1 Document `{{ ... }}` syntax, multiple and escaped blocks, output trimming and rejection rules, mount-time validation, per-access re-rendering, timeout behavior, and failure reporting in `README.md`; verify each example matches the implemented CLI and view grammar.
- [x] 5.2 Document interactive-shell versus Nix quoting, the Home Manager coreutils baseline, and the absolute-path or wrapper requirement for other commands; verify the documented Home Manager example appears in `mountFlags` form and the CLI example uses quoting that preserves braces.

## 6. Integration verification

- [x] 6.1 Run `go test ./...` and verify all packages pass.
- [x] 6.2 Run the Home Manager module check for the current system with `nix build` and verify it passes.
- [x] 6.3 Smoke-test a temporary `TASKDATA` mount with a dynamic view whose command reads a file holding a date; list the view, change the file, list again, and verify the result set changes without remounting, then remove a matching task and verify the configured action applies.
- [x] 6.4 Run `openspec validate "add-dynamic-view-filters" --strict` and verify the change remains valid.

## Workflow follow-up

- Archive the change after review and verification are complete.
