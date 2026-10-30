# Spec Delta

## ADDED Requirements

### Requirement: View modification lines support command substitutions

The optional second line of a runtime view's `_view` file SHALL support the same `{{ command }}` blocks as the filter line. Each block SHALL run in a POSIX shell with the mount environment and its validated single-line output SHALL replace the block before the modification arguments are split. An invalid template, failed command, or multi-line output SHALL make the move fail with EIO and SHALL report the view name, command, and stderr to the mount process's standard error.

#### Scenario: Command output supplies a modification value

- **WHEN** a view's second line is `sched:{{cycle this week}}` and `cycle this week` writes `2026-10-26`
- **THEN** moving a task into the view applies `sched:2026-10-26`

#### Scenario: Failed modification command blocks the move

- **WHEN** a command block on the second line exits non-zero and the user moves a task into the view
- **THEN** the move fails with EIO, the task is unchanged, and the mount log names the view and command

#### Scenario: Multi-line modification output is rejected

- **WHEN** a command block on the second line writes more than one line
- **THEN** the move fails with EIO and the task is unchanged

## MODIFIED Requirements

### Requirement: The mount resolves a persistent runtime view store

The mount SHALL resolve a user-scoped directory for runtime view definitions at `$XDG_STATE_HOME/task-fs/views` when `XDG_STATE_HOME` names an absolute path, and otherwise at `$HOME/.local/state/task-fs/views`. It SHALL create the directory if it does not exist and SHALL fail the mount with an error naming the location when the directory cannot be resolved or created. View definitions SHALL persist across unmounts and SHALL be shared by mounts for every Taskwarrior database of the same user.

#### Scenario: Views persist across mounts

- **WHEN** the user creates a runtime view, unmounts, and mounts again without any view-related flag
- **THEN** the root lists the same `@NAME` directory and its `_view` contents are unchanged

#### Scenario: State home selects the store location

- **WHEN** the mount runs with `XDG_STATE_HOME=/home/me/state`
- **THEN** view definitions are stored under `/home/me/state/task-fs/views`

#### Scenario: Home fallback selects the store location

- **WHEN** the mount runs without `XDG_STATE_HOME` and with `HOME=/home/me`
- **THEN** view definitions are stored under `/home/me/.local/state/task-fs/views`

#### Scenario: Unresolvable store fails the mount

- **WHEN** the mount runs with neither `XDG_STATE_HOME` nor `HOME` usable and the store directory cannot be created
- **THEN** the mount fails with an error naming the view store location and nothing is mounted

### Requirement: Mount imports view directories from the mountpoint

The system SHALL scan the mountpoint directory for `@NAME` subdirectories before mounting and create or update runtime views from them. A directory without `_view` SHALL become a no-op view. A newer `_view` or directory SHALL replace an older stored definition, while an older mountpoint copy SHALL NOT overwrite a newer stored one. A legacy `_filter` file SHALL be ignored. Invalid names and entries that are not readable directories or regular files SHALL be ignored without failing the mount.

#### Scenario: Pre-created view appears after mount

- **WHEN** the user creates `@Due/_view` under the mountpoint while unmounted, then mounts
- **THEN** the mounted root lists `@Due` and its directory shows `_view` and the matching task files

#### Scenario: Pre-created empty directory is a no-op view

- **WHEN** the user creates `@Empty` under the mountpoint without a `_view` file, then mounts
- **THEN** the mounted root lists `@Empty` and its directory is empty

#### Scenario: Pre-created empty definition is visible

- **WHEN** the user creates an empty `@Blank/_view` under the mountpoint, then mounts
- **THEN** the mounted root lists `@Blank` and its directory lists `_view` with no task files

#### Scenario: Legacy filter file is ignored

- **WHEN** the user creates `@Due/_filter` under the mountpoint and no `_view`, then mounts
- **THEN** `@Due` becomes a no-op view and no legacy filter content is used

#### Scenario: Newer stored definition wins

- **WHEN** a stored view definition is newer than the mountpoint's `@NAME/_view`
- **THEN** the mounted view keeps the stored definition

#### Scenario: Newer mountpoint definition is imported

- **WHEN** the mountpoint's `@NAME/_view` was modified after the stored definition
- **THEN** the mounted view uses the mountpoint definition

#### Scenario: Invalid mountpoint entries are ignored

- **WHEN** the mountpoint contains `@`, `@Report.a1b2c3d4.md`, a regular file named `@file`, or another invalid `@` entry
- **THEN** those entries do not become views and the mount succeeds

### Requirement: Custom view filters support command substitutions

The first line of a runtime view's `_view` file SHALL support zero or more `{{ command }}` blocks. Each block SHALL run in a POSIX shell and its standard output, with one trailing newline removed and surrounding whitespace trimmed, SHALL replace the block as literal filter text. A backslash before an opening delimiter SHALL produce a literal `{{`, and blocks SHALL be rendered independently.

#### Scenario: Command output supplies a filter value

- **WHEN** a view's first line contains `status:pending due.before:{{ payday-date }}` and `payday-date` writes `2026-11-01`
- **THEN** the view's filter is `status:pending due.before:2026-11-01`

#### Scenario: Multiple blocks are rendered in one filter

- **WHEN** a view's first line contains two command blocks and both commands succeed
- **THEN** each block is replaced by its own command's trimmed output and the rest of the filter is unchanged

#### Scenario: Output containing spaces becomes filter text

- **WHEN** a command block writes `project:home or project:finances`
- **THEN** the view filter contains that complete filter expression rather than one literal argument name

#### Scenario: Escaped delimiter stays literal

- **WHEN** a view's first line contains `\{{ not a command }}`
- **THEN** the view filter contains the literal text `{{ not a command }}` and no command is executed for it

### Requirement: Dynamic view filters are re-rendered for view access

Before listing a runtime view or resolving, removing, sweeping, or moving a task into it, the system SHALL read that view's `_view` and render the command substitutions needed for that operation. A rendered result MAY be reused only for concurrent or immediately following lookups within one traversal; a later traversal and every move SHALL render again.

#### Scenario: Changed command output appears without remounting

- **WHEN** a view command starts writing a date that brings a new task into the filter and the user lists the view again
- **THEN** the new task's file appears without remounting

#### Scenario: One traversal does not run the command per file

- **WHEN** a directory traversal lists a runtime view and then looks up several of its files
- **THEN** the command runs once for that traversal rather than once per file lookup

#### Scenario: A later traversal observes new output

- **WHEN** the command output changes after one listing completes and the user lists the view again
- **THEN** the second listing uses the newly rendered filter

#### Scenario: A move uses freshly rendered modification arguments

- **WHEN** the modification command output changes after a listing and the user then moves a task into the view
- **THEN** the move applies the newly rendered arguments rather than the cached value
