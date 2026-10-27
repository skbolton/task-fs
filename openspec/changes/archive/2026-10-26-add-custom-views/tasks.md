# Tasks

## 1. Backend exact-filter execution

- [x] 1.1 Add a backend helper that exports tasks for one opaque filter string by invoking `task` with `( <filter> )` and `export` as separate arguments, and verify a fake-runner unit test asserts that exact argument shape and parses the JSON result
- [x] 1.2 Add backend tests for trailing-colon filters such as `status.any:`, quoted phrases, zero-match results, and backend error propagation, and verify `go test ./internal/backend` passes

## 2. Mount flag parsing and view definitions

- [x] 2.1 Add repeatable `--view "Name=filter"` parsing that splits at the first `=`, accepts a leading `@` in the name, and carries definitions into `vfs.Config`, and verify `cmd/task-fs` parser tests cover the split, spaces in the name and filter, leading `@`, and an `=` inside the filter
- [x] 2.2 Validate custom view names at mount for empty values, length over 254 bytes, `/`, control characters, `.`/`..`, minted task-file patterns, and duplicate names, and verify table-driven `cmd/task-fs` tests reject each case with an error naming the custom view
- [x] 2.3 Validate each custom view filter against the backend before mounting, accepting zero matches and failing the mount with the custom view name when the backend rejects the filter, and verify tests cover both outcomes
- [x] 2.4 Add `--view` to `reorderFlags` value handling and update the mount usage string, and verify flag-position tests cover `--view` before and after the mountpoint

## 3. Root custom view namespace and lookup node

- [x] 3.1 Implement a `viewNode` that lists exactly the matching custom view task files and resolves only minted task-file names, and verify VFS tests cover flat results across projects, a completed task when the filter does not constrain status, an empty custom view, and ENOENT for project, tag, and virtual-tag lookups
- [x] 3.2 Add root `Lookup` and `Readdir` handling for configured `@Name` directories, including custom-view-first precedence over same-named projects, and verify VFS tests cover root listing, absence below the root, and an `@Due` project resolving only when no custom view shadows it
- [x] 3.3 Give custom view directories and their task files a distinct view identity, and verify a VFS test shows the same task resolving as different inodes in a custom view and a normal project view
- [x] 3.4 Preserve task-file naming, read-only content, and the UUID xattr inside custom views, and verify a VFS test reads the xattr and confirms file content is still empty

## 4. Custom view gesture semantics

- [x] 4.1 Reject `Create`, `Mkdir`, and `Rename` in custom view directories and implement `Unlink` through the existing full-UUID removal action, and verify tests cover rejected touch/mkdir/mv-in/mv-out/mv-to-COMPLETED plus successful `rm` under `complete` and `delete`
- [x] 4.2 Handle `Rmdir` of a custom view directory with the sweep-based view-emptiness rule while keeping the configured directory present, and verify tests cover ENOTEMPTY under both actions, success when no actionable task remains, and recursive `rm -rf` convergence

## 5. Home Manager argument quoting

- [x] 5.1 Quote each generated mount flag and the mount path in `nix/home-manager.nix`, and verify `nix build .#checks.$(nix eval --raw --impure --expr builtins.currentSystem).hm-module` passes
- [x] 5.2 Add a Home Manager module check whose `mountFlags` contain a `--view` value with spaces, and verify the generated `ExecStart` keeps that value as one argument while existing flag checks still pass

## 6. Documentation

- [x] 6.1 Document `--view`, exact filter semantics, the root-only `@Name` namespace, the `@`/project shadowing rule, and the read-plus-`rm` gesture surface in `README.md`, and verify the examples match the implemented CLI and path grammar
- [x] 6.2 Add a Home Manager `mountFlags` example containing a view value with spaces and verify it matches the quoted module behavior

## 7. Integration verification

- [x] 7.1 Run `go test ./...` and verify all packages pass
- [x] 7.2 Run the Home Manager module check for the current system and verify it passes
- [x] 7.3 Smoke-test a mount with a temporary `TASKDATA` and a configured custom view: list `@Name`, confirm the exact result set, remove one matching task, and verify the configured action changed only that task

## Workflow follow-up

- Archive the change after review and verification are complete.
