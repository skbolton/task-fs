# Design

## Context

See `proposal.md` for motivation. Runtime views replace the mount-time `vfs.View` slice, which `main` builds from `--view` flags and which `FS` stores in `viewDefs`/`viewDefsByName`. Today `viewNode` renders a fixed filter through an injected `RenderView` function and caches the rendered value for one second. Everything else about views — flat task listings, rejection of moves and creation, task removal under the configured action — already works and should survive the change.

The mounted tree is the only place a user can create views, but it cannot itself persist data: every listing is an on-demand `task` query and task files are synthetic. A runtime view therefore needs a host-side definition file plus a small in-memory representation in the VFS. The existing `internal/view` package already owns template parsing and shell execution; it is the natural home for the definition store and name rules.

## Goals / Non-Goals

**Goals:**

- Make `@NAME/_filter` the durable definition of a view and the only way to create one.
- Keep view reads on demand: editing `_filter` changes the next listing without remounting.
- Let `@NAME` directories staged under the mountpoint while unmounted appear on the next mount.
- Keep the view read surface flat and the mutation surface limited to `_filter` edits, view deletion, and task-file removal.
- Preserve the one-second render reuse within a traversal and the existing failure reporting through EIO plus stderr.
- Keep the VFS testable without a real FUSE mount or a real task database.

**Non-Goals:**

- Per-database view stores or a configurable views directory.
- Nested views or views that compose with project/tag segments.
- Renaming a view directory or a `_filter` file.
- Validating `_filter` content at write time.
- Writing view definitions into Taskwarrior task data.

## Decisions

**Store one file per view under the XDG state directory.** The store lives at `$XDG_STATE_HOME/task-fs/views`, falling back to `$HOME/.local/state/task-fs/views`. `XDG_STATE_HOME` is honored only when absolute, matching the XDG rule that relative values are ignored. The mount creates the directory with user-only permissions (0700) and fails with a clear error when neither variable yields a writable location. Each view is a file named exactly for the view name (without `@`) whose bytes are the raw `_filter` text, or a no-filter marker when the view has no `_filter`. Writes go to a temporary file in a hidden subdirectory followed by `rename`, so a reader never observes a partial edit or a stray temporary view. Files are created 0600.

The state directory is user-scoped rather than placed under `TASKDATA`, so one set of saved views applies to every Taskwarrior database the user mounts. That is a deliberate consequence of treating views as saved filters rather than task data; the README states it.

**Import pre-created mountpoint directories at startup.** The mountpoint is hidden once FUSE mounts, so `main` calls `Store.ImportViews(mountpoint)` before `fs.Mount`. That scan treats each valid `@NAME` directory as a view: a missing `_filter` produces a no-op view, an empty `_filter` is imported as a present, empty filter, and a `_filter` or directory newer than the stored definition replaces the stored content. An older mountpoint copy never overwrites newer runtime content, and the scan never deletes stored views absent from the mountpoint. Unreadable or invalid entries are skipped so a stray folder cannot abort the mount.

This is a one-way import. Views created or edited through the mounted tree stay in the state store and are not written back into the mountpoint, which avoids needing a directory handle that survives the FUSE mount. The alternative—making the mountpoint the live view store—would require opening the underlying directory before mount and using `os.Root`-style access afterwards, and would make views mountpoint-scoped instead of user-scoped. Import covers the staging workflow without changing the persistence model.

Alternatives considered:

- A single JSON index of all views. Rejected because per-view files mirror the `@NAME/_filter` shape, make external inspection possible, and need no schema or merge logic for concurrent edits.
- A directory inside `TASKDATA`. Rejected because it would duplicate views for every database and couple view definitions to task data; the selected behavior is host-persistent views.
- An in-memory map only. Rejected because views would disappear on every remount, which contradicts "create them on demand" as a durable workflow.

**Expose the store to the VFS through a narrow interface.** `vfs.Config` replaces `Views []View` with `ViewStore`, an interface offering `List`, `Get`, `HasFilter`, `Create`, `Put`, `DeleteFilter`, and `Delete`. `main` passes a `*view.Store`; tests pass either the real store rooted in `t.TempDir()` or a fake. The VFS does not read environment variables or know file layout. `view.Store` owns name validation and returns distinguishable errors for invalid names, existing names, and missing names, which the VFS maps to `EINVAL`, `EEXIST`, and `ENOENT`.

A nil store is treated as an empty, read-only store so existing VFS tests that do not care about views keep working; attempts to create a view then fail with `EIO`.

Alternatives considered:

- Passing the concrete `*view.Store` into `vfs`. Rejected because it couples tree code to persistence and makes tests create directories for behaviors unrelated to views.
- Keeping `ViewStore` logic in `main` behind callbacks. Rejected because root listing, mkdir, rmdir, and `_filter` writes are all tree operations and would need several callbacks with awkward error mapping.

**Name validation moves from flag parsing into the store.** A view name is the part after `@` and must be non-empty, at most 254 bytes, free of `/` and control characters, not `.` or `..`, and not shaped like a minted task file. Because the name is also the store file name, rejecting `/`, `.`/`..`, and control characters makes the mapping safe without escaping. Names may contain spaces, `@`, and other printable characters.

`mkdir @NAME` at the root calls `Create`; `Create` fails when the file exists, which gives `EEXIST`. A project whose first segment is `@NAME` does not block creating a view of the same name: the view is a separate root entry and shadows the project while it exists, matching the existing precedence rule. Removing the view makes the project reachable again.

Alternatives considered:

- Sanitizing names into safe file names. Rejected because two names could collide after sanitizing; rejecting unsafe names is simpler and already close to the old mount-time rule.
- Storing an index with arbitrary names in JSON. Rejected with the one-file-per-view decision.

**`mkdir @NAME` creates a no-op view; `_filter` appears once created, even if empty.** The view definition exists before any filter file is written, so root listings are stable while the user is between `mkdir` and creating the filter. The view directory is empty until `_filter` exists; lookup returns `ENOENT` until then, so an editor or `touch` creates it through the normal `Create` path. The store distinguishes a missing `_filter` from an empty `_filter`, so `touch` makes the file visible while an empty filter still yields no tasks. The directory lists `_filter` first once it exists, then the matching task files.

A `_filter` file is a real-looking regular file in the view directory with mode 0644, owned by the mount process user, and no UUID xattr. `Lookup` resolves it by exact name once it exists; every other non-minted name remains `ENOENT`. `Create` allows only `_filter`, records an empty filter immediately, and returns the filter file node, so `touch` shows the file and an editor can write it; other names remain `EACCES`. Removing `_filter` writes the no-filter marker rather than deleting the view, so the view becomes a no-op again.

Alternatives considered:

- Hide `_filter` from listings. Rejected because users need to discover the file and editors and shell completion work better when it is listed.
- Make `_filter` removal delete the view. Rejected because a recursive removal can visit `_filter` before or after the task files; clearing the filter keeps the definition stable until the directory removal.
- Require a filter at `mkdir` time. Rejected because a directory cannot carry file contents on creation; creating `_filter` after the directory is the natural editor gesture.

**Writes buffer per open handle and persist on flush.** `filterFileNode` returns a handle initialized from the current store text. Writes append at the requested offset into the handle buffer, `O_TRUNC` and a zero-size `Setattr` reset it, reads on the handle see the buffer, and `Flush`, `Fsync`, and `Release` write the final buffer through `Store.Put`. `Create` records an empty filter first, and removing the file writes the no-filter marker instead of deleting the view. This makes `echo ... > _filter`, editor writes, and `touch _filter` all work. A store failure surfaces as `EIO` from the flush and does not replace the previous definition.

Filter validation stays lazy. FUSE close errors are not reliably delivered to shell redirection, and editors commonly write a file in several partial chunks, so validating every chunk would reject valid intermediate states. On access, the system parses the template and evaluates the filter; malformed text, an invalid template, a failing command, or a backend rejection makes the view operation fail with `EIO` and a stderr line naming the view. This is the same failure contract dynamic views already have.

Alternatives considered:

- Validate the whole buffer on flush. Rejected because the user may never see the error from `echo >` or an editor's atomic save, and the next access would need the same handling anyway.
- Persist on every `Write` call. Rejected because multi-chunk writes would expose partial filters to concurrent readers and cause unnecessary backend queries.

**Read the filter and render it per view operation, with a source-aware cache.** `viewNode` reads the store text on each listing and lookup. A successful render is cached by view name with the source text, rendered filter, and timestamp. `Readdir` forces a read and render; lookups within the existing one-second window reuse the cached value only when the source text is unchanged. A `_filter` write through the mount invalidates that view's cache. External edits are picked up by `Readdir` or after the TTL, which matches the existing freshness contract. An empty source skips parsing and evaluation entirely; a non-empty source is parsed with the existing `view.Parse` and rendered with `ShellRunner` through an injected `RenderFilter` function.

`FS` keeps the rendered filter of the last successful access keyed by view name. If `_filter` is cleared while a recursive removal is unlinking files it enumerated earlier, task-file unlinks may still resolve against that last filter; fresh listings and lookups never use it once the source is empty. Without this, `rm -rf @NAME` could fail depending on whether the directory walk visited `_filter` before or after the task files.

Alternatives considered:

- Cache task results. Rejected because the existing contract is on-demand backend queries.
- Cache forever until a write through this mount. Rejected because another mount or a direct edit of the state directory must become visible.
- Resolve every task-file unlink from the current store unconditionally. Rejected because it makes `rm -rf` order-dependent when `_filter` is visited first.

**Deleting a view is a root `rmdir`; recursive removal still removes tasks.** `FS.Rmdir("@NAME")` deletes the definition and returns success without querying or mutating tasks, even when tasks match. This is the deliberate way to discard a saved view and is distinct from removing a project directory, which still applies the removal action. If no view definition exists, `@NAME` falls through to the existing project-directory removal.

`rm -rf @NAME` is a recursive gesture: it unlinks the listed task files under the configured removal action, clears `_filter`, and finally removes the definition. The README recommends `rmdir` for discarding a view without touching tasks. Directory renames involving a view (`mv @old @new`) remain unsupported and are rejected, consistent with the existing rule that views have no writable task inverse.

Alternatives considered:

- Guard `rmdir` on view emptiness as before. Rejected because deleting a saved filter should not require deleting the matching tasks, and the definition is not task data.
- Make `rm -rf @NAME` skip task files. Rejected because the directory listing legitimately contains them and FUSE cannot distinguish a recursive walk from individual removals.

**Move the store and name rules into `internal/view`.** The package already parses templates and runs commands; it gains `Store` and `ValidateName` with no dependency on `vfs` or FUSE. `vfs` gains only an interface and a render callback. `main` loses `viewFlag`, `compiledViews`, `parseViews`, `validateViews`, and the `--view` entry in `reorderFlags`; it resolves the store, creates it, and injects a renderer that runs `Parse` + `Render` with the existing shell runner.

`usage` becomes `task-fs mount [--rm complete|delete] <mountpoint>`. Because the standard flag package rejects unknown options, `--view` naturally fails with the usage message and mounts nothing.

Alternatives considered:

- Keep `--view` as a seed for the store. Rejected by the selected behavior: runtime definitions are the single source of truth, and a second definition path would need duplicate and shadowing rules.
- Put the store in a new top-level package. Rejected because the store's name rules and template rules are the same view vocabulary and belong together.

## Risks / Trade-offs

- [Breaking removal of `--view`] → Documented in the proposal, README, and usage error; the migration is one `mkdir` plus one `_filter` write per view.
- [Views are shared across Taskwarrior databases] → Documented; the state path is user-scoped by design.
- [Invalid `_filter` content is accepted by the write] → The next access fails with EIO and a stderr line, and the previous definition is replaced only by the user's own write; correcting the file restores the view.
- [A `_filter` write can race an external edit or another mount] → Writes use temporary file plus rename and last writer wins; there is no cross-process lock, matching the single-user assumption of the rest of the tool.
- [Recursive `rm -rf @NAME` changes tasks] → README and gesture table call out `rmdir` as the view-deletion gesture; the behavior is consistent with task files being removable in a view.
- [Clearing `_filter` leaves a last-rendered filter usable for in-flight task unlinks] → Only applies to task-file unlink resolution after the source becomes empty; listings and lookups immediately treat the view as empty.
- [A mountpoint `@NAME` directory is stale or unrelated] → Import compares modification times, skips invalid names, and never deletes runtime-only views; the README states that import is one-way.
- [A store directory that disappears while mounted] → Root listing and view access fail with EIO until the directory is restored; the mount does not silently recreate it mid-session.
- [Home Manager service resolves a different state directory than the interactive session] → The module documents the `XDG_STATE_HOME`/`HOME` resolution; a service with the user's normal environment sees the same views.

## Migration Plan

No task-data migration. Remove `--view` from mount commands and Home Manager `mountFlags`, remount with the new binary, and recreate each former view once with `mkdir "@NAME"` followed by writing the former filter into `@NAME/_filter`. The definitions then persist in the user's state directory. Rolling back means restoring the previous binary and flags; runtime view files remain in the state directory and are ignored by the old binary. Nothing in the view lifecycle writes task data except existing task-file removal gestures.
