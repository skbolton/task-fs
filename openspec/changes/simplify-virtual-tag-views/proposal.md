# Proposal

## Why

Mounting now requires choosing a virtual-tag set with `--virtual-tags` and a hidden/visible presentation with `--virtual-hidden`, even though custom views cover every derived filter a user actually browses. The two flags duplicate a choice the path grammar already makes unnecessary, and the selection machinery makes the mount interface harder to explain than the tree it configures.

## What Changes

- **BREAKING** Remove `--virtual-tags`/`-t` and `--virtual-hidden`/`-H`. Passing either flag now fails the mount with a usage error and mounts nothing.
- Always expose exactly two virtual-tag views: `+COMPLETED` and `+DELETED`, listed as regular, non-hidden root directories whether or not any task currently matches them. These two remain because they double as the complete and delete gestures.
- Remove dot-prefixed virtual-tag spelling: `.+COMPLETED` and `.+DELETED` no longer list or resolve, and the hidden-presentation behavior is gone.
- Stop discovering virtual tags from Taskwarrior at mount and stop validating a selected set. Other derived filters (`OVERDUE`, `DUE`, `ACTIVE`, `WAITING`, ...) are obtained by defining them as custom views, for example `--view 'Due=+DUE'`.
- Keep custom views (`--view NAME=FILTER`, `@Name` directories) unchanged.
- Document the fixed `+COMPLETED`/`+DELETED` vocabulary, the custom-view replacement for other derived filters, and the flag removal in the README and Home Manager module text and checks.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `mount-lifecycle`: Removes the virtual-tag selection and presentation mount options and requires the mount to expose the fixed `COMPLETED`/`DELETED` set without backend discovery.
- `task-navigation`: Restricts virtual-tag segments to `+COMPLETED` and `+DELETED`, always lists both at the root, and removes the dot-prefixed spelling and hidden presentation.
- `task-mutations`: Requires complete and delete destinations to resolve unconditionally as `+COMPLETED` and `+DELETED` and removes the dot-prefixed destination spelling.
- `home-manager-module`: Replaces the documented virtual-tag flags in `mountFlags` guidance and checks with custom views and the fixed status views.

## Impact

- CLI: `cmd/task-fs/main.go` loses the two flags, the discovery query, and the selection validation; usage text and flag reordering change.
- Backend: `internal/backend/query.go` loses the discovery, default-selection, and selection-validation helpers; `AllTags` is no longer used by the mount.
- VFS: `internal/vfs/tree.go` loses `Config.VirtualTags` and `Config.Hidden`, the presentation switch, dot-prefixed resolution, and the now-unreachable non-status virtual-tag rejection paths.
- Tests: CLI, backend, and VFS tests that configure virtual-tag sets, hidden presentation, or other derived virtual tags are rewritten around the fixed status pair or custom views.
- Nix: `nix/home-manager.nix` and `nix/hm-module-test.nix` update `mountFlags` examples and checks to stop passing the removed flags.
- Documentation: `README.md` mount, path-grammar, and gesture sections describe the fixed status views and custom-view replacement.
- Task data is untouched; existing mounts must be remounted with the new binary and any `--virtual-tags`/`--virtual-hidden` flags removed.
