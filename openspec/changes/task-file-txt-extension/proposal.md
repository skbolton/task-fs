# Proposal

## Why

Task files currently end in `.md`, which suggests they are Markdown notes that can be edited in place. They are not: task files are read-only placeholders whose content is owned by Taskwarrior, and the planned note-taking workflow is better handled by whatever client or file manager the user chooses. The misleading extension should become a neutral one.

## What Changes

- Rename every minted task file from `<description>.<short-uuid>.md` to `<description>.<short-uuid>.txt`.
- Update minted-name detection so task files, create gestures, lookups, renames, and custom-view name validation use the `.txt` suffix.
- Keep the uniform short-UUID suffix and deterministic sanitization; `.txt` is the only naming change.
- Remove documentation that presents the extension as groundwork for an in-app note system, and state that note editing is left to user-chosen tools.
- Update user-facing examples, tests, and spec scenarios that show task file names.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `task-files`: Task files are named with a `.txt` extension instead of `.md`, and the write-rejection rationale no longer promises an in-app note system.
- `task-mutations`: Example task file names in mutation scenarios use the `.txt` extension.
- `mount-lifecycle`: The minted-name example used to validate custom view names uses the `.txt` extension.

## Impact

- `internal/naming/naming.go` and `internal/naming/naming_test.go`: suffix format, suffix length bookkeeping, minted-name detection, and expectations.
- `internal/vfs/tree.go` and `internal/vfs/tree_test.go`: behavior follows `naming.IsMintedName`; test fixtures that use minted names need the new extension.
- `internal/view/store_test.go`: view-store fixtures that distinguish task files from view declarations.
- `README.md`: task-file examples and the note-system sentence.
- No data migration: task files are generated from Taskwarrior on every listing, so no stored files change. Existing shell history or scripts that name `.md` files will need to use the new names.
