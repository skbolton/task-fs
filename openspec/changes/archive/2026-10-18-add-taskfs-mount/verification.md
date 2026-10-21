# Verification Log

## 2.3 Prefix semantics against the live CLI (TW 3.4.2)

- `task project:dk status:pending export` → 27 tasks
- `task project:"dk.Get proficient in C++ and Qt6" status:pending export` → 26 tasks

The parent filter `project:dk` matches strictly more tasks than the exact
subproject filter, confirming `project:X` includes every dotted subproject of
`X`, as the navigation spec requires ("Parent directory includes subproject
tasks").

Related grounding observations on the live database:

- `task status:pending _unique project` emits full dotted project names
  (including names containing spaces and dots inside words), which the
  directory node splits into next-level child segments.
- `task _tags` lists all-uppercase virtual tags (ACTIVE, COMPLETED, DUE, ...)
  alongside real tags, so keeping the fully-uppercase entries at mount time
  yields the virtual-tag universe.

## 7.1 Live-database navigation

The spec's example database (project `home`, tag `next`) does not exist on
the live database; the live vocabulary was used instead: project `Home`,
tag `shopping`.

Verified on the live database mounted in a throwaway mountpoint:

- `ls /Home/+shopping` == `ls /+shopping/Home` (byte-identical listings),
  confirming the order-free grammar (`cd /+next/home` == `cd /home/+next`
  scenario with the live equivalents).
- The task files under `/Home/+shopping` exactly match
  `task project:Home +shopping status:pending export`.
- The task files under `/Home` exactly match
  `task project:Home status:pending export`.
- `/+COMPLETED`, `+DUE`, and the other discovered virtual tags appear in
  listings as directory entries, sourced from mount-time `task _tags`
  discovery.

## 7.2 Gesture loop on a scratch TASKDATA database

Mounted with `TASKDATA=/tmp/opencode/scratch-taskdata`, seeded one task,
then exercised every gesture in sequence:

1. `mkdir home/+next; touch "home/+next/Pick up Milk"` → task created with
   description `Pick up Milk`, project `home`, tag `next`, status pending
   (confirmed via `task export`).
2. `touch "home/+DUE/Only due"` → task created with project `home` and no
   virtual tag; the file appears under `/home`, not `/home/+DUE` (write
   filter drops virtual tags, as specified).
3. `mv "Pick up Milk.<uuid>.md" +COMPLETED/` → status becomes `completed`;
   the file shows under `/+COMPLETED/home`.
4. `mv +COMPLETED/home/<file> home/` → status reopens to `pending`,
   project and tags preserved.
5. `mv home/<file> finances/2026-taxes` (no trailing slash, project
   minted on demand) → project becomes `finances.2026-taxes`.
6. `rm finances/2026-taxes/<file>` → status becomes `deleted`; the task
   disappears from pending listings.

One implementation detail confirmed necessary: Taskwarrior 3.x prompts for
confirmation on `delete`, so mutation commands run with
`rc.confirmation=off` — the filesystem gesture itself is the user's
confirmation. Without it `rm` fails at the CLI.

## 7.3 README command examples

Every command example in README.md was run as written against a live
mountpoint:

- `task-fs mount <dir>` mounts and lists; the mountpoint must pre-exist
  (documented).
- `TASKDATA=... task-fs mount <dir>` operates on the alternate database.
- `getfattr -n user.taskwarrior.uuid --only-values <file>` returns the full
  UUID.
- `fusermount3 -u <dir>` detaches cleanly; SIGINT of the foreground process
  also unmounts.
- `cd /homi` in a fresh shell reports "No such file or directory" (strict
  ENOENT for unknown segments).

## Notes on task interpretation

- Task 5.3's third unit test ("mv between two arbitrary filter directories
  fails with not-supported") described the intermediate state before task
  5.7 lands the unified delta mv, which the task-mutations spec requires to
  *succeed* (inbox-to-project, tag deltas). The implemented final behavior
  applies the delta; the not-supported path is what rejects moves whose
  destination is a non-status virtual-tag view (e.g. `/home/+DUE` →
  `/home/+OVERDUE`) — unit-tested in
  `TestRenameVirtualOnlyDestinationRejected`.
- Task 7.1's example names (`home`, `+next`) do not exist on the live
  database; the live equivalents (`Home`, `+shopping`) were used and the
  results recorded above.

## 8.3 Hidden virtual tags on a live mount

After switching virtual tags to the dot-prefixed hidden spelling:

- A plain `ls` of the root shows zero `.+` entries — virtual tags are
  hidden.
- `ls -a` reveals them: `.+ACTIVE`, `.+ANNOTATED`, `.+BLOCKED`, ... sourced
  from mount-time `task _tags` discovery.
- `ls /+COMPLETED` fails with ENOENT: the bare uppercase form no longer
  resolves.
- `ls /.+COMPLETED` lists completed tasks (with their project directories),
  confirming the status gesture semantics carried over to the dot form.

## 9.2 Exact project placement on a live mount

The reported bug — tasks from deep subprojects appearing in ancestor
directories — is fixed by exact placement:

- `ls /this` shows only the `is` directory; neither deep task appears.
- `ls /this/is` shows `a` and `Less nested project.c30264dd.md`
  (project `this.is`).
- `ls /this/is/a/test` shows `Test out deep projects.ea077a2d.md`
  (project `this.is.a.test`).
- The root lists no task files (no project-less pending tasks on the live
  database) — the root is the inbox for project-less tasks.
- `+shopping` is empty, matching the CLI: no pending task currently carries
  that tag (`task status:pending _unique tags` → `[]`).

## 10.1 Self-referential tag listing fix on a live mount

Reproduced the reported artifact: inside `/+next`, the listing contained a
`+next` entry (the view's own tag, derived from `_unique tags`); since
entering it fails (the tag is already in the filter), `ls` rendered it as
`+next?` — its convention for entries it cannot stat.

After the fix, on the live database:

- `ls /+next` shows the project directory `in` and the task
  `testing next.96e0949d.md`; no `+next` entry.
- `cd /+next/+next` fails with ENOENT ("No such file or directory").
- `mkdir .hidden` fails with EACCES; `mkdir .+DUE` fails with EEXIST at the
  kernel level because `.+DUE` already exists as a virtual-tag directory
  (the handler-level rejection is EACCES).

## 10.2 Self-referential virtual-tag listing fix on a live mount

The same artifact as 10.1, one level up in the tree: `.+COMPLETED` was
listed inside `.+COMPLETED` itself, since Readdir emitted every discovered
virtual tag unconditionally.

After the fix, on the live database:

- `ls /.+COMPLETED` no longer contains `.+COMPLETED`; it shows the project
  and tag directories of completed tasks plus their files.
- `TestSelfVirtualTagNotListed` covers the unit level.

## 11.1 Per-view inode identity on a live mount

Reported: dropping a tag while keeping the project was impossible —
`mv /home/+next/<file> /home/` failed because the task file is legitimately
visible in both views, and a global kernel inode number made GNU `mv`
detect source and destination as the same file and refuse before the
filesystem ever saw a rename.

Fix: task-file kernel inodes are keyed by view (hash of the view's filter
plus the task UUID) instead of UUID alone. Verified on a scratch
`TASKDATA` database:

- `mv "home/+next/Drop tag test.d795750f.md" home/` succeeds.
- Export after: `Drop tag test` has project `home` and no tags; the
  `Keep tag test` task remains tagged `next` and visible in `/home/+next`.
- The reverse gesture (adding a tag by moving into `/home/+next/`) still
  works.

Related robustness fix: closing a tmux window (or any terminal death) sends
SIGHUP, which previously killed the mount process without unmounting and
left a stale `Transport endpoint is not connected` / `Permission denied`
mountpoint behind. SIGHUP is now handled and unmounts cleanly (verified).









