# Design

## Context

See `proposal.md` for motivation. Runtime views currently persist one file per view under the XDG state directory and expose it at `@NAME/_filter`. `internal/view` owns the file-name constant, the store, the template parser, and the shell runner. `internal/vfs` renders the stored filter through an injected callback, caches the render for one second, and rejects every rename whose source or destination is a `viewNode`.

This design builds on the completed `runtime-view-queries` change. The main specs still describe mount-time `--view` definitions, so the delta specs use the requirement headers that `runtime-view-queries` will leave behind. Archive `runtime-view-queries` before this change; until then `openspec validate` reports those MODIFIED headers as not yet present.

## Goals / Non-Goals

**Goals:**

- Make `_view` the single definition file and let its optional second line authorize moves into the view.
- Keep view reads driven by the first line and moves driven by the second line, without remounting.
- Reuse the existing template parser, shell runner, render cache, store, and error reporting instead of adding a parallel mechanism.
- Make the definition format and argument splitting explicit enough to test at the package boundary.

**Non-Goals:**

- Migrating `_filter` files, mountpoint definitions, or store layout beyond keeping stored bytes readable.
- Moving tasks out of views, renaming on the way into a view, or deriving a source-to-view filter delta.
- Per-database stores, nested views, or changing view removal semantics.
- Executing the modification line as a shell command line.

## Decisions

**Keep one raw definition per view in the store.** The store file content becomes exactly the `_view` bytes: first line, newline, optional second line. `Store.Get` already returns the raw text, so existing stored definitions (a single filter line) remain valid without migration. Rename the store methods and constants around the definition rather than the filter: `ViewFileName = "_view"`, `HasView`, `DeleteViewFile`, and a rename of the internal no-file marker. The VFS `ViewStore` interface follows. This keeps persistence dumb: parsing and rendering stay in the view and VFS layers.

Alternatives considered:

- Storing parsed filter and modification fields as JSON. Rejected because it changes the on-disk format, needs a schema, and would not keep existing definitions readable.
- Keeping `_filter` as an alias. Rejected by the agreed clean break; two names would make writability and import behavior ambiguous.

**Parse the definition as at most two non-empty lines.** Split on `\n`, trim a trailing `\r` from each line, and tolerate a trailing empty line. Line one is the filter. Line two, when non-empty after trimming, is the modification line and makes the view writable. An empty or whitespace-only second line is treated as absent, so a file ending in a newline stays read-only. A non-empty third line is an error. Splitting the definition belongs in `internal/view` so store, VFS, and tests share one rule.

The "non-empty second line" rule is deliberately conservative: a blank line cannot accidentally turn a read-only view into one that accepts moves with no arguments. It also gives users a simple way to disable writability without deleting the filter.

Alternatives considered:

- Treating the whole remainder after the first newline as the modification line. Rejected because a third line would silently become arguments.
- Making a blank second line writable with zero arguments. Rejected because the move would appear to succeed while changing nothing.

**Render both lines through the existing template callback.** `RenderFilter` is already a generic `func(context.Context, string) (string, error)`; rename it to `RenderTemplate` or reuse it unchanged and call it for each non-empty line. Keeping one renderer preserves the `{{ command }}` syntax, shell environment, timeout, single-line output validation, and stderr reporting. The mount wiring does not need a second shell runner.

The render cache grows from `{source, filter}` to `{source, filter, modifyArgs, at, valid}`. Listings may reuse a render within the one-second window. A move calls the renderer with `refresh=true`, matching removal, so a stale cached command result can never decide what modification to apply.

Alternatives considered:

- Rendering the whole file once. Rejected because the second line uses different validation and tokenization after rendering.
- Caching the two lines separately. Rejected because they share one source and one render failure mode; a single cache entry is simpler and avoids partial state.

**Split rendered modification text with a small quote-aware tokenizer.** The second line describes arguments after `task <uuid> modify`, so `description:"call mom"` must become one argument. Add `view.SplitArgs` that supports single quotes, double quotes, backslash escapes outside quotes, and rejects unterminated quotes. It performs no variable expansion, globbing, or command substitution; only `{{ }}` blocks render. This keeps the file declarative and avoids evaluating arbitrary shell command lines.

Alternatives considered:

- `strings.Fields`. Rejected because quoted values containing spaces would be split incorrectly.
- Running `/bin/sh -c` on the line. Rejected because it would execute the modification text as shell code and make the file unsafe to share.
- Passing the whole line as one argument. Rejected because Taskwarrior expects separate modify arguments.

**Treat a view node as a valid rename destination only when writable.** `dirNode.Rename` gains a view branch before the generic destination filter resolution. A move into a `viewNode` is allowed when the view has a non-empty modification line and the destination name is the same minted task-file name; otherwise it fails with `EACCES`. The task is resolved in the source directory, then `Query.Modify(uuid, args)` runs with the rendered arguments. No `deltaModifyArgs` and no status handling run, because a view has no filter path to compare against.

The standard `mv file @NAME` gesture resolves `@NAME` as the destination directory and arrives as a rename into the `viewNode`, so the same branch covers it. A direct rename onto the root entry `@NAME` is treated the same way when the destination name matches the source file name. Renaming a task while moving it into a view remains rejected, because a view cannot express a destination filter or a new file name.

Source views stay non-writable: `viewNode.Rename` continues to return `EACCES`, which covers moves out of a view, moves between views, and renames inside a view.

Alternatives considered:

- Deriving a source-to-view delta. Rejected: there is no destination project or tag set, and the modification line is the user's explicit instruction.
- Allowing a rename into a view. Rejected because the view does not store per-task names and the task may not match the filter anyway.
- Allowing moves out through the ordinary filter-delta path. Rejected to keep scope and safety unchanged.

**A move may not leave the task visible in the view.** The modification line is authoritative. If it does not make the task match the first line, the file disappears from the view after the move. This mirrors ordinary filter moves, where the destination filter decides visibility, and avoids pretending the view can always contain the task. The modification call is a single Taskwarrior `modify`; a render or backend failure returns `EIO` before or during that call and leaves the task unchanged as far as the backend allows.

**Keep import and legacy behavior explicit.** `ImportViews` reads `@NAME/_view`, stores its raw bytes, and ignores `_filter`. Directory and file modification times continue to decide whether a mountpoint copy replaces a stored definition. Because stored definitions are raw bytes, a pre-existing single-line store file imported before this change still renders as a read-only view.

## Risks / Trade-offs

- **Archive ordering** → The delta specs reference requirements introduced by `runtime-view-queries`; that change must be archived first. Until then validation emits informational "not found" issues and archiving this change would fail.
- **Task vanishes after a successful move** → Document that the modification line must make the task match the filter, and test that non-matching moves succeed but do not list the file.
- **Modification arguments are powerful** → A view definition can change any Taskwarrior field. This is intentional, but the definition file is user-owned; do not execute it as shell code.
- **Quote tokenizer edge cases** → Implement and test a narrow grammar (quotes, escapes, no expansion). Document that shell expansion is unavailable outside `{{ }}` blocks.
- **Definition edits during a move** → Moves always re-read and re-render, so a partially written editor save can fail the move with `EIO`; the next move after the save succeeds.
- **Breaking rename** → Users with `_filter` files must rename them. The error is an empty view rather than a mount failure, so document the migration prominently.

## Migration Plan

1. Archive `runtime-view-queries` so the runtime-view requirements reach the main specs.
2. Ship the code and documentation change. Existing stored view definitions keep working because their single line is the filter.
3. Rename any mountpoint-staged `@NAME/_filter` to `@NAME/_view`; `_filter` is ignored.
4. Rollback is code-only: restore the previous binary and mount; stored definitions remain compatible because the new format is a superset of the old.

## Open Questions

None.
