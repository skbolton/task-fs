# Proposal

## Why

Runtime views are read-only: a task file inside `@NAME` cannot be moved because the filesystem has no way to know what change the user intends. That leaves the view workflow split between the mounted tree and manual `task ... modify` commands. A view should be able to declare how a task enters it so that moving the file is enough.

## What Changes

- Rename the view definition file from `_filter` to `_view`. The first line remains the Taskwarrior filter; a view without `_view` stays a listed no-op.
- Accept an optional second line in `_view` containing the Taskwarrior modify arguments to apply when a task is moved into that view, exactly as if they followed `task <uuid> modify`.
- Render `{{ command }}` blocks in both the filter line and the modification line, reusing the existing template and shell-runner behavior.
- Allow moving a task file from a filter directory into a view whose `_view` has a non-empty second line, applying only those modification arguments. Do not derive a project/tag delta from the source because a view has no destination filter path.
- Keep a view with no second line read-only: moving into it continues to fail with the current permission error.
- Keep moves out of a view rejected, including moves from a view into a filter directory or a status view.
- **BREAKING** Existing `@NAME/_filter` files are no longer recognized. Stored view definitions remain readable because their first line is already the filter, but mountpoint-staged `_filter` files must be renamed to `_view` by the user.
- Update the README, Home Manager descriptions, and module checks to describe `_view`, the optional modification line, and the writable-view move gesture.

Out of scope: per-database view stores, moving tasks out of views, composing views with filter segments, changing removal semantics, and automatic migration from `_filter`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `mount-lifecycle`: The mount imports `@NAME/_view` definitions and renders both lines of a view definition; legacy `_filter` import is removed.
- `task-navigation`: A view directory exposes `_view` instead of `_filter` and resolves it as the editable definition file.
- `task-mutations`: `_view` creation, writing, and removal replace `_filter` gestures, and moves into a view with a modification line apply that line's arguments while moves into a view without one remain rejected.

## Impact

- `internal/view`: replace the `_filter` file-name constant, add definition splitting and quote-aware argument parsing, extend template rendering and store/import tests.
- `internal/vfs`: rename the view file node and its create/write/unlink paths, render filter and modification lines, and teach `Rename` about writable view destinations while keeping view sources non-writable.
- `cmd/task-fs`: mount import and renderer behavior follow the renamed definition and second line; no new CLI flags.
- `README.md`, `nix/home-manager.nix`, and `nix/hm-module-test.nix`: rename examples and document the move gesture.
- View data: the state store stays one file per view; its bytes now represent the full `_view` definition. No task data changes.
- This change assumes the completed `runtime-view-queries` change; both must be archived in order for the main specs to reflect runtime views.
