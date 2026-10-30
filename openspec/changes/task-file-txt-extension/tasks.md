# Tasks

## 1. Mint `.txt` task file names

- [x] 1.1 Change `internal/naming/naming.go` so `suffixFmt` mints `<description>.<short-uuid>.txt`, `IsMintedName` accepts only the `.txt` form, and the `SuffixLenBase` comment matches the 13-byte `.txt` suffix; verify `go test ./internal/naming` passes
- [x] 1.2 Update `internal/naming/naming_test.go` expectations to `.txt`, add a negative case proving an old `.md` name is not minted, and keep the truncation, duplicate-description, stability, and sanitization cases passing; verify with `go test ./internal/naming`

## 2. Update filesystem and view fixtures

- [x] 2.1 Update minted-name fixtures in `internal/vfs/tree_test.go` (create rejection, custom-view and lookup cases) to `.txt`; verify `go test ./internal/vfs` passes and that non-minted names still create tasks
- [x] 2.2 Update minted-name fixtures in `internal/view/store_test.go` to `.txt`; verify `go test ./internal/view` passes and view declarations are still distinguished from task files

## 3. Update user documentation

- [x] 3.1 Update every task-file example in `README.md` to `.txt` and replace the future note-system sentence with a statement that note editing belongs to the user's chosen client or file manager; verify the documented `getfattr` and `mv` examples match the minted names and run with a live or test mount

## 4. Integration verification

- [x] 4.1 Run `go test ./...`, `go vet ./...`, and `go build ./...`; verify all packages pass with the new extension
- [x] 4.2 Search the active repository (excluding `openspec/changes/archive/`, historical change artifacts, and the stale `TASKFS/` mount) for remaining `.md` task-file references; verify only intentional prose or unrelated Markdown filenames remain
- [x] 4.3 Run `openspec validate task-file-txt-extension --strict`; verify the change has no validation errors

## Workflow follow-up

- After review, archive the change so the delta specs update the main capabilities.
- Verify the archived result and the final task-files specification after archiving.
