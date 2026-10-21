# Tasks

## 1. Project scaffold

- [x] 1.1 Initialize the Go module (`go mod init`), add `github.com/hanwen/go-fuse/v2` as a dependency, and create the `cmd/task-fs` entrypoint with `mount` subcommand parsing that errors without a mountpoint argument; verify `go build ./...` succeeds and `go run ./cmd/task-fs mount` exits non-zero with usage
- [x] 1.2 Update `flake.nix` devshell to include `go` and `fuse3` (or equivalent FUSE package) and verify `nix develop -c go version` and `fusermount3 --version` both succeed

## 2. Backend query layer

- [x] 2.1 Implement the `task` invocation wrapper: run `task <args> export` with the mount process environment, parse the JSON array into a task struct (uuid, description, project, tags, status), and fail with a clear error when `task` is missing or exits non-zero; verify with a unit test over canned export output
- [x] 2.2 Implement the filter type that accumulates project segments, tags, and virtual-tag segments, and renders itself as `task` filter arguments (projects joined with `.`, `+tag` verbatim, `+COMPLETED` mapped to `status:completed`, uppercase virtual tags verbatim); verify with table-driven unit tests covering the translation table in design.md
- [x] 2.3 Verify prefix semantics against the real CLI: assert `task project:dk status:pending export` includes subproject tasks, mirroring the observed 27-vs-26 behavior on the live database

## 3. Navigation tree

- [x] 3.1 Implement the directory node: holds an accumulated filter, resolves child project segments from `task <filter> status:pending _unique project` (splitting full dotted names into next-level segments) and child tag segments from `task <filter> status:pending _unique tags` (splitting comma-joined combinations), and returns ENOENT for segments absent from those results and from in-memory mkdir declarations; verify with a unit test using a stubbed query layer
- [x] 3.2 Implement `Readdir`: derive one entry per first-level project segment, one per tag, one per discovered virtual tag, and one per task file, with task files sourced from `task <filter> status:pending export`; verify with a unit test reproducing the root-listing scenario from the spec (home, finances, +next)
- [x] 3.3 Implement status defaulting: directories with no status segment query with `status:pending`, and `+COMPLETED` subtrees query with `status:completed`; verify with a unit test that a completed task appears only under `/+COMPLETED`
- [x] 3.4 Implement mount-time virtual-tag discovery: run `task _tags` once at mount, keep the fully-uppercase entries as the virtual-tag universe, map `COMPLETED`/`DELETED` to status gestures, and treat all others as read-only filter segments that translate verbatim and reject mutation gestures; verify with a unit test on discovery output parsing, filter rendering, and gesture rejection

## 4. Task files

- [x] 4.1 Implement task file naming: `<description>.<first-8-hex-of-uuid>.md` with deterministic sanitization to the Windows-legal character set (ban `\ / : * ? " < > |`, handle reserved device names and trailing dots/spaces) and 255-byte truncation retaining the suffix and extension; verify with unit tests covering slash-containing descriptions, over-long descriptions, duplicate descriptions with distinct UUIDs, and suffix stability
- [x] 4.2 Implement task file reads returning empty content, `Getattr` reporting zero size, and `Getxattr`/`Listxattr` exposing `user.taskwarrior.uuid` set to the full UUID; verify with a unit test that reads return empty and the xattr equals the task's UUID from the export data
- [x] 4.3 Implement `create` detection: an empty file whose name does not end in `.<8-hex>.md` is a task-creation gesture, and any create matching the minted-name pattern is rejected; verify with unit tests for both cases
- [x] 4.4 Implement write rejection on task files (read-only in v1, write path reserved for the future note system where file edits map to task annotations); verify with a unit test that a write to a task file fails and the task is unchanged

## 5. Mutation gestures

- [x] 5.1 Implement touch-as-add: derive the write filter by dropping virtual-tag segments from the accumulated filter, run `task add proj:<dir project> <dir tags> "<description>"`, then verify the new file appears with its UUID suffix on the next listing and lands outside virtual-tag-only views (unit test with stubbed command runner)
- [x] 5.2 Implement rm-as-delete: resolve the inode to its full UUID via the `user.taskwarrior.uuid` xattr and run `task <full-uuid> delete` on file removal; confirm the task disappears from subsequent pending listings (unit test with stubbed command runner)
- [x] 5.3 Implement mv-as-status-change: moving into `+COMPLETED` runs `task <full-uuid> done` ignoring the rest of the target path, moving out runs `task <full-uuid> modify status:pending` plus the target directory's project/tags, and mv between two arbitrary filter directories fails with a not-supported error leaving the task unchanged (unit tests for all three)
- [x] 5.4 Verify mutation commands receive the mount process environment so `TASKDATA`-scoped databases are mutated correctly (unit test asserting environment propagation through the command runner)
- [x] 5.5 Verify gestures never emit short-UUID prefixes, integer IDs, or bare description text as task identifiers to `task` (unit test asserting every mutation command argument references the full UUID from the export data)
- [x] 5.6 Implement segment declaration: `mkdir +name` or `mkdir <project-segment>` registers the namespace in daemon memory (per mount session) when no task matches it, rejects uppercase virtual-tag names, returns EEXIST when the segment already exists in the filter, and enforces ENOTEMPTY on rmdir while tasks carry the tag; verify with unit tests for declare, redeclare, in-use rmdir, and virtual-tag rejection
- [x] 5.7 Implement the unified delta mv: compute the filter delta between source and destination, issue `task <full-uuid> modify project:<dest project> <-tags +tags>`, mint a destination's final component as a project segment when it does not exist (no-trailing-slash form), keep status moves special-cased (into `+COMPLETED` completes and ignores the rest of the path; out reopens plus delta), and reject renames that would change a description; verify with unit tests for inbox-to-project, tag delta both directions, parent-project exactness, minted destination, unresolvable parent chain, and rename rejection

## 6. Mount integration

- [x] 6.1 Wire the filesystem into go-fuse's `fs` root: mount at the supplied mountpoint, serve until interrupt, and unmount cleanly on SIGINT; verify by mounting in a temp directory, listing the root, and confirming `fusermount3 -u` detaches without leaving the mount busy
- [x] 6.2 Check `task` availability on `PATH` at startup and fail the mount with a named-requirement error before attaching the filesystem; verify by running the mount with `PATH` stripped

## 7. End-to-end verification

- [x] 7.1 Against the live database in a throwaway mountpoint: navigate `home` -> `+next`, confirm listings match `task project:home +next status:pending export`, and confirm `cd /+next/home` equals `cd /home/+next`; record results in the change
- [x] 7.2 Full gesture loop on a scratch `TASKDATA` database: `touch "Pick up Milk"` in `/home/+next`, confirm the task exists via `task export`, `mv` it to `/+COMPLETED/`, confirm status, `mv` it back to `/home`, confirm reopen, `mv` it into another project directory and confirm the project changed via `task export`, `rm` it, confirm deletion; record results in the change
- [x] 7.3 Write README usage documentation covering mounting, the path grammar (project segments, `+tag`, uppercase virtual tags, pending-by-default), the gesture table, and task file naming (including the `user.taskwarrior.uuid` xattr and that task files are empty placeholders); verify every documented command example runs as written

## 8. Hidden virtual tags (post-review)

- [x] 8.1 Update the filter type so virtual-tag segments are spelled `.+NAME` (dot-prefixed hidden form, uppercase remainder), remove the bare-uppercase-`+` virtual spelling so `+NEXT` resolves as an ordinary tag, and update the table-driven unit tests
- [x] 8.2 Update the navigation tree: listings emit `.+NAME` entries for virtual tags, lookup accepts only the dot form, and `mkdir` rejects virtual-tag and other dot-prefixed names; update unit tests (virtual-only move rejection, mkdir rejection, listings)
- [x] 8.3 Verify on a live mount that a plain `ls` hides the virtual-tag directories and `ls -a` reveals them, and that `+COMPLETED` no longer resolves; record results in the change
- [x] 8.4 Update README's path-grammar and gesture documentation for the dot-prefixed hidden form

## 9. Exact project placement (post-review)

- [x] 9.1 List task files only in the directory whose accumulated project filter equals the task's exact project (ancestor project directories show only subproject directories; project-less views list all matching tasks), and apply the same rule to file lookup and create resolution; verify with unit tests for the deep-project scenario and lookup strictness
- [x] 9.2 Verify on a live mount that `/this` shows only subproject directories while `/this/is` and `/this/is/a/test` show their own tasks; record results in the change
- [x] 9.3 Update README to document exact task placement

## 10. Self-referential tag listing fix (post-review)

- [x] 10.1 Exclude segments already present in the accumulated filter from Readdir (a `+next` view must not list `+next` itself, whether the tag comes from task data or a mkdir declaration), and map invalid filter extensions during lookup to ENOENT instead of EINVAL; verify with unit tests and record the live-mount result in the change
- [x] 10.2 Exclude virtual tags already present in the accumulated filter the same way (a `.+COMPLETED` view must not list `.+COMPLETED` itself); verify with a unit test and record the live-mount result in the change

## 11. Per-view inode identity (post-review)

- [x] 11.1 Key task-file kernel inodes by view (filter) plus UUID instead of UUID alone, so a task visible in both source and destination views is not detected as "the same file" by `mv` and the tag delta can be applied; verify with a unit test (distinct stable attrs across views, stable within a view) and the live gesture `mv /home/+next/<file> /home/` recorded in the change

## Workflow follow-up

- Archive the change after the project's review requirements are satisfied.
- Verify the archived result.
