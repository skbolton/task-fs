# Design

## Context

See `proposal.md` for motivation. Today the mount tool chooses a virtual-tag set in `main`: it queries `task _tags` through `backend.AllTags`, keeps uppercase entries, intersects or validates a selection, and passes the result plus a hidden-presentation boolean into `vfs.Config`. `vfs.FS` stores both, lists `+NAME` or `.+NAME` depending on the boolean, resolves either spelling, and rejects moves into virtual tags that are not status tags.

The two status tags are already modeled separately in `backend.Filter`: `statusVirtualTags` maps `COMPLETED` to `status:completed` and `DELETED` to `status:deleted`, and `Filter.Render`, `RenderSweep`, and `WriteFilter` already treat them as status criteria rather than literal tags. The proposal fixes the exposed set to exactly those two.

## Goals / Non-Goals

**Goals:**

- Make the exposed virtual-tag vocabulary a backend constant rather than mount input.
- Remove presentation, discovery, and selection state from the mount and the filesystem tree.
- Keep `+COMPLETED` and `+DELETED` available for complete/delete moves on any database.
- Delete the now-unreachable non-status virtual-tag rejection paths.
- Leave custom view behavior and all task-data interactions unchanged.

**Non-Goals:**

- Emulating `DUE`, `OVERDUE`, `ACTIVE`, or `WAITING` as built-in views; custom views cover them.
- Accepting the removed flags as deprecated aliases.
- Changing custom view semantics, the removal action, the path grammar for projects and tags, or the task file format.

## Decisions

**Derive the exposed set from the status map.** Add a small `backend.StatusVirtualTags()` accessor returning `COMPLETED` then `DELETED` from the existing status map, and keep `IsVirtualTag`/`StatusVirtualTag` as the single source of truth. `vfs.FS` calls these directly instead of storing a configured slice.

Alternatives considered:

- Keep `vfs.Config.VirtualTags` and pass a constant from `main`. Rejected: it preserves a second source of truth and lets tests construct trees that production can never create.
- Keep discovery and hard-code a default selection. Rejected: discovery is only needed to validate user-chosen names, and it made the status views disappear on an empty database, which breaks the "always available" gesture.

**Remove presentation and dot resolution from the tree.** Delete `Config.Hidden`, the `FS.hidden` field, `virtualEntryName`, and the `.+NAME` branch in `resolveChild`. `readdirEntries` appends `+COMPLETED` and `+DELETED` directly. A dot-prefixed name now falls into the existing reserved-dot branch and returns ENOENT. `Filter.WithSegment` loses its `.+` case and `WithVirtualSegment` accepts only the `+NAME` spelling and only names in the status map.

Alternatives considered:

- Keep `.+` lookup as a compatibility alias while listing only `+`. Rejected: it leaves the hidden-presentation semantics half-alive, and the status views are the only virtual tags left, so scripts can migrate to one spelling.
- Leave the unused presentation helper in place. Rejected: a dead option implies a mode users can still request.

**Drop non-status virtual handling.** With the fixed set, every virtual tag in a filter is a status tag. Remove `hasNonStatusVirtual` and the `ENOTSUP` branch for destinations carrying a non-status virtual tag; `statusRemoveAction` remains the only mapping from a virtual-tag destination to a complete/delete gesture. `Filter.criteriaArgs` keeps its generic rendering, including the `+NAME` fallback, because `Filter` remains a reusable value and the fallback costs nothing.

Alternatives considered:

- Keep the non-status branch for hypothetical future tags. Rejected: it would be untested and would require re-introducing a way to select non-status tags.

**Remove discovery from the mount path.** `main` stops calling `Query.AllTags`, and `backend.VirtualTagsFromDiscovery`, `DefaultVirtualTagSelection`, `DefaultVirtualTags`, and `ResolveVirtualTags` are deleted with their tests. `parseMountFlags` returns only mountpoint, removal action, and views; `reorderFlags` keeps `--rm` and `--view` as value-taking flags. The standard `flag` package already rejects unknown options, so dropping the registrations makes `--virtual-tags`, `-t`, `--virtual-hidden`, and `-H` fail with the usage message.

Alternatives considered:

- Register the old flags and emit a deprecation warning. Rejected by the accepted behavior: removed flags should fail the mount, and silently mounting with a different view set is worse than an explicit error.
- Keep `AllTags` for future use. Rejected: unused code on an interface with a test double; adding it back is trivial if a future feature needs it.

**Exercise the fixed vocabulary in VFS tests.** Test helpers stop accepting a virtual-tag list or a hidden mode. Tests that only asserted presentation or a non-status virtual tag (`DUE`, `PENDING`, `OVERDUE`) are removed or rewritten against `COMPLETED`/`DELETED`: for example, the delete-action guard uses `+COMPLETED` because completed tasks are non-deleted, while `+DELETED` always sweeps empty. Recursive-removal tests walk `+COMPLETED` instead of `.+PENDING`. Custom-view tests are untouched because they never depended on the virtual-tag selection.

**Document custom views as the replacement.** No code supports `+DUE` as a path segment anymore, but `--view 'Due=+DUE'` already reaches Taskwarrior as an opaque filter expression, so Taskwarrior expands the virtual tag. The README and module description point other derived filters at custom views and remove the two flags from examples. The Home Manager module keeps passing `mountFlags` through unchanged; only the examples and the module check's expected command change.

## Risks / Trade-offs

- [Existing mounts pass the removed flags and fail] → Documented as a breaking CLI change in the proposal, README, and removal migration; the usage message names the mount command and shows the supported flags.
- [Users lose built-in `DUE`/`OVERDUE` browsing] → Custom views cover them with a filter the user already understands, and the migration example names `--view 'Due=+DUE'`.
- [Scripts use `.+COMPLETED` or `.+DELETED`] → The migration note maps both to the `+` spelling; the status views remain reachable with a stable name.
- [Tests that encoded hidden presentation lose coverage] → Rewrite them to assert the single listing form and the fixed set rather than deleting the behaviors they protect (nesting bound, self-view exclusion, removal guards).
- [A literal uppercase tag named `COMPLETED` is hidden by the virtual tag] → This is already true for Taskwarrior virtual tags and remains the documented precedence rule; no new ambiguity.
- [Custom view filters using a virtual tag bypass the tree's status logic] → That is the existing exact-filter rule for custom views; they are read-only for moves and apply the configured action only to file removal.

## Migration Plan

No task-data migration. Remove `--virtual-tags`/`-t` and `--virtual-hidden`/`-H` from mount commands and Home Manager `mountFlags`, then remount with the new binary. Replace other derived-filter browsing with `--view` definitions and any `.+COMPLETED`/`.+DELETED` path with `+COMPLETED`/`+DELETED`. Rolling back means restoring the previous binary and flags; the change never writes task data during mount or navigation.
