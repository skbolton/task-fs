# Spec Delta

## ADDED Requirements

### Requirement: The mount resolves a persistent runtime view store

The mount SHALL resolve a user-scoped directory for runtime view definitions at `$XDG_STATE_HOME/task-fs/views` when `XDG_STATE_HOME` names an absolute path, and otherwise at `$HOME/.local/state/task-fs/views`. It SHALL create the directory if it does not exist and SHALL fail the mount with an error naming the location when the directory cannot be resolved or created. View definitions SHALL persist across unmounts and SHALL be shared by mounts for every Taskwarrior database of the same user.

#### Scenario: Views persist across mounts

- **WHEN** the user creates a runtime view, unmounts, and mounts again without any view-related flag
- **THEN** the root lists the same `@NAME` directory and its `_filter` contents are unchanged

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

The system SHALL scan the mountpoint directory for `@NAME` subdirectories before mounting and create or update runtime views from them. A directory without `_filter` SHALL become a no-op view. A newer `_filter` or directory SHALL replace an older stored definition, while an older mountpoint copy SHALL NOT overwrite a newer stored one. Invalid names and entries that are not readable directories or regular files SHALL be ignored without failing the mount.

#### Scenario: Pre-created view appears after mount

- **WHEN** the user creates `@Due/_filter` under the mountpoint while unmounted, then mounts
- **THEN** the mounted root lists `@Due` and its directory shows `_filter` and the matching task files

#### Scenario: Pre-created empty directory is a no-op view

- **WHEN** the user creates `@Empty` under the mountpoint without a `_filter` file, then mounts
- **THEN** the mounted root lists `@Empty` and its directory is empty

#### Scenario: Pre-created empty filter is visible

- **WHEN** the user creates an empty `@Blank/_filter` under the mountpoint, then mounts
- **THEN** the mounted root lists `@Blank` and its directory lists `_filter` with no task files

#### Scenario: Newer stored definition wins

- **WHEN** a stored view definition is newer than the mountpoint's `@NAME/_filter`
- **THEN** the mounted view keeps the stored filter

#### Scenario: Newer mountpoint definition is imported

- **WHEN** the mountpoint's `@NAME/_filter` was modified after the stored definition
- **THEN** the mounted view uses the mountpoint filter

#### Scenario: Invalid mountpoint entries are ignored

- **WHEN** the mountpoint contains `@`, `@Report.a1b2c3d4.md`, a regular file named `@file`, or another invalid `@` entry
- **THEN** those entries do not become views and the mount succeeds

### Requirement: Removed view flag fails the mount

The mount command SHALL reject `--view` as an unknown option, exit non-zero with a usage message, and mount nothing.

#### Scenario: Removed view flag fails the mount

- **WHEN** the user runs `task-fs mount ~/m --view "Due=due.before:now"`
- **THEN** the command exits non-zero with a usage message and mounts nothing

#### Scenario: Removed view flag fails in any position

- **WHEN** the user runs `task-fs mount --view "Due=due.before:now" ~/m`
- **THEN** the command exits non-zero with a usage message and mounts nothing

## MODIFIED Requirements

### Requirement: Custom view filters support command substitutions

A filter stored in a runtime view's `_filter` file SHALL support zero or more `{{ command }}` blocks. Each block SHALL run in a POSIX shell and its standard output, with one trailing newline removed and surrounding whitespace trimmed, SHALL replace the block as literal filter text. A backslash before an opening delimiter SHALL produce a literal `{{`, and blocks SHALL be rendered independently.

#### Scenario: Command output supplies a filter value

- **WHEN** a view's `_filter` contains `status:pending due.before:{{ payday-date }}` and `payday-date` writes `2026-11-01`
- **THEN** the view's filter is `status:pending due.before:2026-11-01`

#### Scenario: Multiple blocks are rendered in one filter

- **WHEN** a view's `_filter` contains two command blocks and both commands succeed
- **THEN** each block is replaced by its own command's trimmed output and the rest of the filter is unchanged

#### Scenario: Output containing spaces becomes filter text

- **WHEN** a command block writes `project:home or project:finances`
- **THEN** the view filter contains that complete filter expression rather than one literal argument name

#### Scenario: Escaped delimiter stays literal

- **WHEN** a view's `_filter` contains `\{{ not a command }}`
- **THEN** the view filter contains the literal text `{{ not a command }}` and no command is executed for it

### Requirement: Dynamic view commands use the mount environment

Command blocks in a view's `_filter` SHALL run with the mount process's environment and working directory, so commands see the same Taskwarrior configuration and database selection as backend queries.

#### Scenario: Taskwarrior database selection is inherited

- **WHEN** the mount runs with `TASKDATA=$HOME/other-tasks` and a view command reads `TASKDATA`
- **THEN** the command sees `$HOME/other-tasks`

### Requirement: Dynamic view command failures make the view unreadable

The system SHALL execute command blocks with a bounded timeout. A non-zero exit, timeout, missing shell, empty output, NUL byte, or embedded newline SHALL fail the view operation with EIO and SHALL report the view name, command, and stderr to the mount process's standard error.

#### Scenario: Failed command yields an unreadable view

- **WHEN** a view command exits non-zero during a directory listing
- **THEN** the listing fails with EIO and the mount log names the view and command

#### Scenario: Hung command times out

- **WHEN** a view command does not finish within the execution timeout
- **THEN** the operation fails with EIO rather than blocking indefinitely

#### Scenario: Empty command output is rejected

- **WHEN** a view command succeeds but writes no non-whitespace output
- **THEN** the operation fails with EIO

#### Scenario: Multi-line output is rejected

- **WHEN** a view command writes more than one line of output
- **THEN** the operation fails with EIO instead of splicing ambiguous multi-line text

### Requirement: Dynamic view filters are re-rendered for view access

Before listing a runtime view or resolving, removing, or sweeping a task in it, the system SHALL read that view's `_filter` and render its command substitutions for that operation. A rendered result MAY be reused only for concurrent or immediately following lookups within one traversal; a later traversal SHALL render again.

#### Scenario: Changed command output appears without remounting

- **WHEN** a view command starts writing a date that brings a new task into the filter and the user lists the view again
- **THEN** the new task's file appears without remounting

#### Scenario: One traversal does not run the command per file

- **WHEN** a directory traversal lists a runtime view and then looks up several of its files
- **THEN** the command runs once for that traversal rather than once per file lookup

#### Scenario: A later traversal observes new output

- **WHEN** the command output changes after one listing completes and the user lists the view again
- **THEN** the second listing uses the newly rendered filter

## REMOVED Requirements

### Requirement: Mount option defines named custom views

**Reason**: Runtime `@NAME/_filter` directories replace mount-time view definitions, so a mount no longer needs a repeatable view option.

**Migration**: Create the view once at the mounted root with `mkdir @NAME` and write the filter into `@NAME/_filter`; the definition persists for later mounts.

### Requirement: Custom view names are validated at mount

**Reason**: View names now come from `mkdir @NAME` gestures rather than flag values, so validation belongs to the creation gesture.

**Migration**: Use a non-empty name without `/`, control characters, or a minted task-file shape; an invalid `mkdir @NAME` fails without creating a view.

### Requirement: Custom view filters are validated at mount

**Reason**: View filters are created and changed while mounted, so there is no mount-time definition to validate.

**Migration**: A malformed or unrenderable `_filter` makes access to that view fail with EIO and is reported to the mount log; correct the file to restore the view.

### Requirement: Command substitution templates are validated at mount

**Reason**: Templates are stored in `_filter` and may change at any time, so parsing no longer happens at mount.

**Migration**: Write a syntactically valid template to `_filter`; an empty command block, unclosed block, or trailing escape makes the view unreadable until corrected.
