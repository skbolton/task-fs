# Design

## Context

See `proposal.md` for motivation. Custom views are already re-queried on every listing: `viewNode` stores a `vfs.View` with a fixed `Filter` string, and `tasks()` calls `backend.Query.ExportQuery` each time. The mount-time validator also runs the same filter through `ExportQuery`, while `rmdirView` feeds it to `SweepQuery`.

The missing piece is between the stored definition and those query calls. A dynamic view needs to turn its stored template into a concrete filter before each operation, without caching task results and without changing the existing no-cache query contract.

## Goals / Non-Goals

**Goals:**

- Parse and validate `{{ ... }}` templates once at mount.
- Render templates at view-operation time using a shell process in the mount environment.
- Make rendering failures diagnosable through the mount process's stderr rather than only as FUSE `EIO`.
- Avoid executing one command per task file during a single directory traversal.
- Keep static `--view` definitions and the existing `vfs.View` data shape behavior unchanged.

**Non-Goals:**

- Caching task query results or rendered task lists.
- A new mount flag for the timeout, render TTL, or shell path.
- Dynamic view names, dynamic root entries, or filesystem editing of view definitions.
- Composing dynamic views with project or tag path segments.

## Decisions

**Use a dedicated `internal/view` package for parsing and rendering.** The package exposes a parsed template, a renderer that replaces command blocks, and a runner abstraction for executing one command. Keeping this outside `vfs` lets the VFS continue to own navigation and filtering while the template rules are unit-testable without a FUSE tree.

`Parse` scans for `{{`, finds the matching `}}`, trims the command, and rejects an empty command or an unclosed block. `\{{` emits a literal `{{` and is not a block. Static filters become templates with one literal segment, so the same rendering path handles both cases.

Alternatives considered:

- `$(...)` substitution in the existing filter parser. Rejected because interactive shells pre-expand it, the quoting rules are easy to get wrong, and matching nested parentheses and quotes adds parser complexity.
- A whole-filter command option (`--view-command`) whose stdout is the entire filter. Simpler, but it cannot keep a static filter readable around one dynamic date and would need a second configuration surface.
- Rendering in `main` only. Rejected because listings, lookups, and removal checks happen inside `vfs` after mount.

**Inject rendering into the VFS through `Config`.** Add a `RenderView func(context.Context, View) (string, error)` field to `vfs.Config`, stored on `FS`. When it is absent or nil, rendering returns the stored filter unchanged, which keeps existing VFS tests and static view construction working. `main` compiles the templates, validates them once, and supplies a renderer that looks up the compiled template by view name.

This preserves `vfs.View{Name, Filter}` as the public definition shape. The alternative—putting a compiled template field on `View`—would force template-package types into every VFS test and make `reflect.DeepEqual` expectations carry parser internals.

**Use `/bin/sh -c` with a bounded timeout.** The shell runner captures stdout and stderr separately, inherits the mount process environment, and runs with a 5-second timeout by default. The template renderer trims one trailing newline and surrounding whitespace, rejects empty output, NUL, and embedded newlines, then splices the result as literal filter text. The command output is not re-parsed as a template and is not shell-quoted by TaskFS.

A non-zero exit, timeout, missing shell, or unacceptable output becomes a render error. `viewNode` and `rmdirView` map that to `EIO`; the renderer also writes the view name, command, stderr, and exit status to the mount process's stderr so the systemd journal shows the cause.

Alternatives considered:

- `exec.Command` with a split argument list. Rejected because it loses pipes, redirection, and the shell semantics users expect from a command block.
- Parsing and quoting output as a Taskwarrior term. Rejected because it would prevent a command from producing a multi-term filter expression, and the existing grouped single-argument boundary already keeps the whole filter out of Taskwarrior's command scanning.
- Silently falling back to the last successful value. Rejected because stale dates would be invisible and could silently change which tasks are shown or removed.

**Render once per operation, reuse only within one traversal.** `FS` keeps a per-view render entry with a mutex, a rendered value, and a timestamp. A one-second TTL is the fallback freshness bound. `Readdir` forces a fresh render, so each new directory listing sees the current command output. `Lookup` reuses a cached value when it is still within the TTL, which prevents `ls -l` or a file manager from executing the command once per file. `Unlink` and custom-view `Rmdir` force a fresh render because they make a decision about task data. Concurrent calls for the same view are serialized per entry, not by one global lock.

Each entry retains the value for at most one second when no forced render occurs, and a later `Readdir` always refreshes. This deliberately does not cache task results, so the existing on-demand query contract is unchanged.

Alternatives considered:

- No cache. Rejected because a single `ls -l` can issue a lookup per file, and a command with side effects or noticeable latency would run many times.
- Per-view indefinite cache invalidated only by `Readdir`. Rejected because a lookup-only client could use a value from an old traversal indefinitely.
- Cache the rendered task list. Rejected because it would violate the no-cache query behavior and make CLI changes invisible until expiry.

**Validate templates and rendered filters at mount.** `main` parses every filter while building view definitions, so syntax errors and empty commands fail before `vfs.New`. `validateViews` then renders each compiled template once through the same shell runner and passes the result to `ExportQuery`. Static views render to themselves, so the existing malformed-filter and zero-match behavior remains. The validation render is not stored in the access cache.

Eager validation means commands must succeed at session start and may run once before the first mount. That is deliberate: it preserves the current fail-fast mount contract and catches unavailable binaries or malformed output immediately. A view whose command depends on later session state can return a fallback value; a lazy validation mode is deferred.

**Baseline shell support belongs to the Home Manager automount environment.** `nix/home-manager.nix` adds `pkgs.coreutils` to the service `PATH` so blocks can use commands such as `date` without Nix store paths. The runner uses the absolute `/bin/sh`, so it does not depend on `sh` being on the service `PATH`. Commands outside the coreutils baseline must use an absolute path or a wrapped script; the README and module description state that rule. No change to `mountFlags` quoting is needed because `lib.escapeShellArgs` already keeps each item, including braces and spaces, as one argument.

## Risks / Trade-offs

- [Arbitrary command execution from mount configuration] → This is no broader than the existing ability to set mount flags or own the Home Manager service; keep definitions mount-time only and document the trust boundary.
- [Filters that literally contain `{{` change meaning] → Document `\{{` and treat this as the only compatibility carve-out; normal static filters are unaffected.
- [A command with side effects runs at mount and repeatedly during browsing] → Document the per-operation execution model and the one-second reuse window; keep the timeout short.
- [A slow command delays a listing or mount] → Five-second cap with `EIO` and a journal error; the cache prevents per-file repetition.
- [Command output changes while a recursive removal is in progress] → Each removal decision renders fresh, so the view may converge differently than the listing that started the walk; this is the same consistency model as external task changes.
- [Home Manager users expect their interactive `PATH` inside blocks] → The service environment is documented; coreutils is provided, other tools use absolute paths or wrappers.
- [Mount-time validation runs commands before the user browses] → Accepted for fail-fast behavior and named in the docs; a lazy mode remains a possible future flag.

## Migration Plan

No task-data migration. Existing static views behave as before unless their filter contains a literal `{{`, which must now be written `\{{`. Existing mounts need a remount to run the new binary. Home Manager users get coreutils in the service `PATH` after rebuilding. Rolling back means reverting the binary and module and remounting; no task data is changed by the feature itself.
