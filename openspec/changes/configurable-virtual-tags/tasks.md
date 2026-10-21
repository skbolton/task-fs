# Tasks

## 1. Backend selection resolution

- [x] 1.1 Add `ResolveVirtualTags(selection string, discovered []string) ([]string, error)` in `internal/backend` (split on `,`, trim whitespace, reject empty entries, `ALL` alone expands to the discovered set, any other entry must be discovered, error names the invalid entry) and verify with a table test covering defaults-accepted, `ALL`, explicit list, empty entries, and invalid names
- [x] 1.2 Add the default-intersection helper or behavior (default selection intersected with the discovered set, never failing) and verify with a test where the backend does not discover a default entry such as `WAITING`

## 2. VFS selection and presentation

- [x] 2.1 Replace `SetVirtualTags` with a `vfs.Config{VirtualTags, Hidden}` parameter on `vfs.New`, update `cmd/task-fs/main.go` and test call sites, and verify with `go build ./...` and the existing test suite passing
- [x] 2.2 Make `resolveChild` accept both `+NAME` and `.+NAME` spellings of a selected virtual tag, with the virtual-tag check taking precedence in the `+` branch, and verify with `internal/vfs/tree_test.go` cases: both spellings resolve in both modes, non-selected `+DUE`/`.+DUE` returns ENOENT, a selected virtual tag shadows a same-named uppercase tag
- [x] 2.3 Make Readdir list each selected virtual tag once in the presentation form (`+NAME` regular, `.+NAME` hidden) while keeping the no-nesting guard, and verify with listing tests for both modes over a root filter and the no-nesting case inside a virtual-tag view
- [x] 2.4 Extend the rmdir virtual-tag branch to trigger on a selected name under either prefix, keeping the destructive branch for all other names, and verify with tree tests: `rmdir +COMPLETED`/`.+COMPLETED` on a non-empty view fails ENOTEMPTY, succeeds when empty, and `rmdir +urgent` on a declared tag still deletes as before

## 3. Mount command wiring

- [x] 3.1 Parse `mount` with a `flag.FlagSet` providing `-t/--virtual-tags` (default `COMPLETED,OVERDUE,DUE,ACTIVE,WAITING`) and `-H/--virtual-hidden`, run the selection through `ResolveVirtualTags`, and fail the mount with an error naming the invalid entry; verify with tests or a manual run for missing mountpoint (usage, exit non-zero), an invalid entry (`--virtual-tags "COMPLETED,bogus"`), and `ALL`
- [x] 3.2 Update the usage message to document both flags and verify the documented command lines run as written

## 4. Documentation and integration

- [x] 4.1 Update `README.md`: the new flags, the changed default selection and presentation, and the `--virtual-hidden --virtual-tags ALL` rollback pairing; verify the documented commands match the implementation
- [x] 4.2 Run `go test ./...` and `go vet ./...` and fix any fallout; run a live mount (with and without `-H`, with `-t ALL`) and confirm listing presentation, both-spelling lookup, and the `+COMPLETED` completion gesture behave per the specs
