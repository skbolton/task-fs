# Proposal

## Why

Today the mount always exposes every discovered virtual tag as a hidden `.+NAME` directory. Users cannot opt out of the views they never use, cannot get them as visible directories, and the all-tags default clutters every listing. The mount command should let the user choose which virtual tags appear and whether they are hidden.

## What Changes

- Add a `--virtual-tags` (`-t`) flag to `task-fs mount` accepting a comma-separated list of virtual-tag names without the `+` sign (for example `COMPLETED,OVERDUE,DUE,PENDING,ACTIVE`), or the special value `ALL` for the previous behavior of exposing every discovered virtual tag.
- Default the flag to `COMPLETED,OVERDUE,DUE,ACTIVE,WAITING` when it is not given; the previous all-tags behavior is available with `--virtual-tags ALL`.
- Validate the flag at mount time: every entry must be `ALL` or a discovered, fully-uppercase virtual tag; otherwise the mount fails with an error naming the invalid entry.
- Add a `--virtual-hidden` (`-H`) flag that presents the enabled virtual-tag directories as hidden entries in the current `.+NAME` dot form. Without it, they are presented as regular directories in `+NAME` form. **BREAKING**: the default presentation changes from hidden `.+NAME` entries to regular `+NAME` entries.
- Both `+NAME` and `.+NAME` spellings continue to resolve a virtual tag on lookup regardless of presentation mode, so existing scripts and muscle memory keep working; the listing shows only the presentation form.
- A taskwarrior filter is unchanged for a given set of virtual tags: presentation affects only entry names and visibility in listings, not the rendered query.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `mount-lifecycle`: the mount command gains the `--virtual-tags` and `--hidden` flags, with validation of the virtual-tag list and a new default selection.
- `task-navigation`: which virtual-tag views are listed and how their entries are spelled changes; the bare-uppercase-`+`-is-not-a-virtual-tag rule is replaced by mode-dependent presentation, and listing rules become mode-dependent.
- `task-mutations`: status-change gestures are restated over the presentation-mode spelling of status virtual-tag directories (move into `+COMPLETED` or `.+COMPLETED` completes).

## Impact

- `cmd/task-fs/main.go`: flag parsing, validation, and wiring of the selected virtual tags and presentation mode into the vfs root.
- `internal/backend`: reuse `VirtualTagsFromDiscovery` and the `AllTags` discovery for validation; no filter-render changes expected.
- `internal/vfs/tree.go`: virtual-tag listing (Readdir), child resolution (`resolveChild`, `isVirtualTag`), mkdir rejection rules, and rmdir's virtual-tag branch must understand both spellings and the enabled set.
- `internal/backend/backend_test.go` and `internal/vfs/tree_test.go`: new coverage for validation and mode-dependent listing/resolution.
- `README.md`: document the new flags and the changed default.
