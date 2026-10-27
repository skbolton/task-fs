# Proposal

## Why

Advanced Taskwarrior users want saved filters available where they browse their tasks, without turning every ad-hoc filter into project or tag structure. A custom view such as `@Due Next Week` puts a frequently used filter at the root of the mounted tree and keeps it current through the same on-demand filter path as the rest of TaskFS.

## What Changes

- Add a repeatable mount flag `--view "Name=filter"` that defines a named, exact Taskwarrior filter. The filter is interpreted as a filter expression, never as a command, and no implicit `status:pending` is added.
- Validate view definitions at mount time: reject malformed definitions, duplicate names, invalid names, and filters the backend rejects with a parse error; name the offending custom view in the error and mount nothing.
- Present each configured custom view as a visible root-only directory named `@Name`.
- Treat a custom view directory as a lookup view, not a filter directory: it lists exactly the task files matching its filter and resolves only minted task-file names. It does not list project, tag, virtual-tag, or other custom view directories, and custom view directories do not nest.
- Reject task creation, `mkdir`, and `mv` in both directions inside a custom view directory. The custom view filter is opaque, so there is no known task modifications that make a task enter or leave it.
- Keep removal semantics: removing a task file in a custom view directory applies the configured `--rm` action. Removing the custom view directory itself follows the virtual-view rule: it fails with `ENOTEMPTY` while a task the removal action would change remains, and succeeds once none does, so recursive removal converges. The configured custom view directory remains present after a successful `rmdir`.
- Fix Home Manager mount-flag rendering so a single `mountFlags` item containing spaces, such as a `--view` value, reaches `task-fs mount` as one argument.
- Document custom views, the `@` namespace, the exact-filter semantics, and the absence of create/move gestures.

## Capabilities

### New Capabilities

None. Custom views extend the existing mount, navigation, mutation, and Home Manager capabilities.

### Modified Capabilities

- `mount-lifecycle`: Adds the repeatable `--view "Name=filter"` mount option, its naming and filter validation, exact-filter semantics, and mount-time rejection behavior.
- `task-navigation`: Adds root-only `@Name` custom view directories, their lookup-only contents, their non-nesting behavior, the root `@` lookup precedence, and custom view visibility when no task matches.
- `task-mutations`: Defines custom view directory gesture restrictions, task-file removal under the configured action, and the virtual-view-style directory removal rule.
- `home-manager-module`: Requires `mountFlags` items to be rendered as single command-line arguments so view values containing spaces survive systemd `ExecStart` parsing.

## Impact

- CLI: `cmd/task-fs/main.go` gains repeatable view parsing, validation, mount flag ordering, and usage text.
- VFS: `internal/vfs/tree.go` gains a root-only view node, view-aware root lookup/listing, and custom view gesture rejection; `internal/vfs/tree_test.go` gains coverage for lookup mode, root-only visibility, and removal.
- Backend: `internal/backend/query.go` gains exact-filter export through a single grouped filter expression; backend tests cover the argument shape and parsing.
- Nix: `nix/home-manager.nix` and `nix/hm-module-test.nix` gain proper argument quoting and a scenario for a view value containing spaces.
- Documentation: `README.md` documents custom views, naming, exact filters, and gesture limits.
- No changes to task file naming, task content, or existing project/tag/virtual-tag navigation behavior.
- A configured custom view shadows a root project whose first segment has the same `@` name; this is documented rather than treated as a migration.
- Out of scope: editing, creating, or deleting view definitions through the filesystem; composing custom view filters with project/tag path segments; an implicit pending default; a typed Home Manager views option.
