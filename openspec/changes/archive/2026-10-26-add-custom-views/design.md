# Design

## Context

The mounted tree is currently a filter accumulator. Each `dirNode` carries a `backend.Filter` of project, tags, and virtual tags; `Lookup` extends that filter by one path segment, and `Readdir` derives projects, tags, virtual tags, and task files from it. The root is the zero-filter `dirNode`, whose special case is the inbox listing.

Two constraints shape this change:

- A raw Taskwarrior filter has no writable inverse. The existing move delta is computed from the source and destination filters, so an opaque filter cannot participate in `mv`.
- Taskwarrior parses command words from argv without respecting filter boundaries. A filter must reach the backend in a shape that cannot be interpreted as a command sequence.

See `proposal.md` for motivation and `specs/` for the required behavior.

## Goals / Non-Goals

**Goals:**

- Add custom view directories without changing the existing path-filter grammar for normal directories.
- Keep filter execution on demand and on the same `task` CLI boundary as every other listing.
- Make custom view directories unambiguous: one lookup behavior, one gesture surface.

**Non-Goals:**

- Creating, editing, or deleting view definitions through the filesystem.
- Composing a custom view filter with project or tag segments such as `/@Due/home`.
- Inferring a pending-status default for custom view filters.
- Adding a typed Home Manager view option; the existing `mountFlags` surface is kept.

## Decisions

**Custom view directories are a separate node type, not a `dirNode` with a special filter.** A `viewNode` holds the root pointer, the custom view name, and the raw filter. It implements `Readdir`, `Lookup`, `Getattr`, `Setattr`, `Unlink`, `Create`, `Mkdir`, and the source side of `Rename`; it does not embed `dirNode`. Embedding would promote the normal filter methods, and a zero filter would silently mean "inbox", which is exactly the wrong behavior. The root `*FS` overrides its promoted `Lookup` and `Readdir` to insert custom view entries, and overrides `Rmdir` because removal of `@Name` arrives at the parent.

**`@Name` is a root-only namespace checked before project lookup.** Root `Lookup` resolves a configured custom view name to its `viewNode`; if no custom view matches, it falls through to the existing project lookup. This keeps real projects named `@foo` reachable when no custom view shadows them, and avoids a global reservation that would hide existing data. A configured custom view wins over a same-named project. Root `Readdir` adds custom view directories first, sorted by display name, then the existing vocabulary.

**View definitions are parsed into `vfs.Config.Views` at mount time.** The mount parser accepts a repeatable `--view` value, splits at the first `=`, strips one leading `@` from the name, and validates the name before constructing the config. Definition errors fail the mount rather than producing a directory that errors on first access. The `reorderFlags` helper learns that `--view` takes a value so the flag works on either side of the mountpoint.

**Filters are executed exactly as supplied, as one parenthesized expression.** The backend gets one filter argument of the form `( <filter> )` followed by `export`. This preserves spaces, quotes, implicit AND, `or`, tags, and bare description terms without TaskFS tokenizing the filter itself. It also prevents Taskwarrior from scanning a filter term as a command, which is a parsing property rather than a security policy. Spaces inside the parentheses are required because a trailing-colon term such as `status.any:` otherwise consumes the closing parenthesis.

Alternatives considered:

- Passing the filter as separate argv tokens requires TaskFS to implement shell-like quoting and to understand Taskwarrior's command-word scanning. Rejected as more complexity for less fidelity.
- A throwaway report filter (`rc.report...filter`) avoids command scanning, but it rejected bare-word description terms in testing and depends on the undocumented `export <report>` behavior. Rejected.
- A command denylist is brittle against aliases, abbreviations, and future commands. Rejected.

**Custom view names and filters are validated at mount.** Name validation is structural: non-empty, at most 254 bytes, no `/` or control characters, not `.` or `..`, not a minted task-file pattern, and unique. Filter validation runs the same grouped export once per custom view. It catches malformed expressions and invalid dates, but not unknown attribute names, which Taskwarrior accepts and matches as empty. A zero-match custom view filter is valid.

**Custom view directories are read-and-remove only.** `Create`, `Mkdir`, and `Rename` return a permission error. `Unlink` resolves the task through the current custom view result and applies the configured `--rm` action by full UUID, reusing the existing removal semantics. The root's `Rmdir` handler treats a custom view name like a virtual view: it calls the sweep filter and returns `ENOTEMPTY` while the action would change a matching task, otherwise succeeds without changing tasks. The view definition is not removed, so `rm -rf @Name` converges after unlinking the files and the saved custom view remains visible.

This is more restrictive than virtual-tag views on purpose. A virtual-tag view has a known source criterion to drop or apply; a raw filter does not, so move gestures cannot be made to match the destination. Allowing `rm` preserves the useful task action without inventing move semantics.

**Custom views get their own inode identity.** A custom view directory hashes `view:<name>` for its inode. Task files in it use `view:<name>` as the `fileNode.viewKey`, so the same task in a normal view and a custom view has distinct inodes. This preserves the existing reason for per-view identity and leaves the door open for future gestures without changing file naming.

**Home Manager quotes every generated argument.** `nix/home-manager.nix` currently joins mount flags with spaces for `ExecStart`, which splits a view value containing spaces into several arguments. The generated command uses argument escaping for each flag item and the mount path. The module test gains a `--view` value containing spaces and asserts that the value reaches the command as one argument. A typed `services.task-fs.automount.views` option is deferred; it would be a separate module surface, not a requirement of this change.

## Risks / Trade-offs

- [Taskwarrior expression parser behavior changes across 3.x] → Pin the grouped-expression contract with backend tests against the packaged Taskwarrior version and keep the wrapper in one backend helper.
- [Trailing-colon filters break inside parentheses] → Always emit `( <filter> )` with interior spaces; cover `status.any:` in the backend tests.
- [Unknown attributes pass validation but match nothing] → Validation is explicitly structural plus backend-parse-only; documentation states that a zero-match custom view filter is valid.
- [`@` shadows a real project with the same root name] → Custom view lookup wins and the precedence is documented; a configurable prefix remains a possible future escape hatch.
- [`rm -rf @Name` applies the removal action to every matching task] → This is the same recursive-removal semantics as other directory views; the view-emptiness rule prevents `rmdir` alone from changing tasks and lets `rm -rf` converge.
- [Flat custom view results lose project context in file names] → Existing task-file naming is unchanged; users can resolve the full UUID via xattr or open the task from its canonical project path.
- [No `mv` out of a custom view] → Accepted because an arbitrary filter has no known inverse; `rm` remains the supported task action from the view.
- [Custom views are fixed for the mount session] → Consistent with mount-time virtual-tag selection; changing a definition requires a remount.
- [Home Manager escaping changes every generated flag] → Existing flags without spaces render equivalently; the module test covers both the existing flag set and a space-containing view value.

## Migration Plan

No data migration. Existing mounts without `--view` behave as before. The Home Manager change only fixes argument quoting for flag values that contain spaces. Rollback is unmounting or reverting the change; no task data is modified by custom view directories other than through the existing `rm` action.
