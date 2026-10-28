# mount-lifecycle Specification

## Purpose

Owns how the filesystem is mounted, unmounted, and connected to the Taskwarrior backend: the mount command interface, backend resolution delegated to the `task` CLI, and the freshness behavior of queries.

## Requirements

### Requirement: Mount command accepts a mountpoint

The system SHALL provide a `task-fs mount <mountpoint>` command that mounts the Taskwarrior filesystem at the given directory path and SHALL remain mounted until explicitly unmounted.

#### Scenario: Mount at a user-supplied path

- **WHEN** the user runs `task-fs mount ~/c/TaskFS`
- **THEN** the filesystem is available at `~/c/TaskFS` and its root lists task directory entries

#### Scenario: Mount fails without a mountpoint argument

- **WHEN** the user runs `task-fs mount` with no mountpoint
- **THEN** the command exits non-zero with a usage message and mounts nothing

### Requirement: Backend access is delegated to the task CLI

The system SHALL perform all task queries and mutations by invoking the `task` command-line program from the mount process environment, and SHALL NOT read or write Taskwarrior data files directly.

#### Scenario: Alternate database via environment

- **WHEN** the mount process is started with `TASKDATA=$HOME/other-tasks task-fs mount ~/c/TaskFS`
- **THEN** listings and mutations at the mountpoint operate on the Taskwarrior database in `$HOME/other-tasks`

#### Scenario: Missing task binary

- **WHEN** the `task` program is not available on `PATH` at mount time
- **THEN** the mount fails with an error naming the missing requirement rather than mounting a broken filesystem

### Requirement: Queries run on demand without caching

The system SHALL re-query the backend for each directory listing and file lookup, and SHALL NOT cache query results in v1.

#### Scenario: Concurrent change is visible immediately

- **WHEN** a task is added or modified with the `task` CLI in another terminal while the filesystem is mounted
- **THEN** the next directory listing or file read at the mountpoint reflects that change without remounting

### Requirement: The mount exposes the status virtual-tag views

The mount command SHALL expose exactly the `+COMPLETED` and `+DELETED` virtual-tag views. Both SHALL be listed and resolvable in every filter directory that does not already carry a virtual-tag segment, whether or not any task currently matches, and SHALL remain available when their views are empty. The mount SHALL NOT discover or validate virtual-tag names against the backend.

#### Scenario: Empty database still exposes both views

- **WHEN** the database has no completed or deleted tasks and the user lists the root
- **THEN** the listing contains the directories `+COMPLETED` and `+DELETED`

#### Scenario: Status destinations are always available

- **WHEN** the database has no completed or deleted tasks and the user moves a task file into `/+COMPLETED/` or `/+DELETED/`
- **THEN** the move resolves the status view instead of failing with ENOENT and applies the corresponding complete or delete action

#### Scenario: Other derived filters are custom views

- **WHEN** the user runs `task-fs mount ~/m --view 'Due=+DUE'`
- **THEN** the root exposes the custom view directory `@Due` and does not expose a `+DUE` virtual-tag directory

### Requirement: Removed virtual-tag mount flags fail the mount

The mount command SHALL reject `--virtual-tags`, `-t`, `--virtual-hidden`, and `-H` as unknown options, exit non-zero with a usage message, and mount nothing.

#### Scenario: Removed selection flag fails the mount

- **WHEN** the user runs `task-fs mount ~/m --virtual-tags "COMPLETED,DUE"`
- **THEN** the command exits non-zero with a usage message and mounts nothing

#### Scenario: Removed presentation flag fails the mount

- **WHEN** the user runs `task-fs mount ~/m --virtual-hidden`
- **THEN** the command exits non-zero with a usage message and mounts nothing

#### Scenario: Removed short flags fail the mount

- **WHEN** the user runs `task-fs mount ~/m -t COMPLETED` or `task-fs mount ~/m -H`
- **THEN** each command exits non-zero with a usage message and mounts nothing

### Requirement: Mount option selects the removal action

The mount command SHALL accept `--rm complete|delete`, defaulting to `complete`, selecting the action that removal gestures apply to matching tasks: `complete` completes them, `delete` permanently deletes them. Any other value SHALL fail the mount with an error naming the invalid value, and nothing SHALL be mounted. The flag SHALL be accepted in any position relative to the mountpoint.

#### Scenario: Default action completes

- **WHEN** the user mounts without `--rm` and removes a task file
- **THEN** the task is completed rather than deleted

#### Scenario: Explicit delete action

- **WHEN** the user mounts with `--rm delete` and removes a task file
- **THEN** the task is deleted

#### Scenario: Invalid action fails the mount

- **WHEN** the user runs `task-fs mount ~/m --rm archive`
- **THEN** the command exits non-zero with an error naming `archive` and nothing is mounted

#### Scenario: Flag position does not matter

- **WHEN** the user runs `task-fs mount ~/m --rm delete` and `task-fs mount --rm delete ~/m`
- **THEN** both mounts select the delete action

### Requirement: Mount option defines named custom views

The mount command SHALL accept a repeatable `--view "Name=filter"` option. Each value SHALL split at its first `=` into a non-empty name and a non-empty filter. The filter SHALL be an exact Taskwarrior filter expression and SHALL NOT gain an implicit `status:pending` or any other criterion. The option SHALL be accepted in any position relative to the mountpoint.

#### Scenario: Defining a custom view at mount time

- **WHEN** the user runs `task-fs mount ~/m --view "Due Next Week=status:pending sched.after:sonw sched.before:eonw"`
- **THEN** the mount succeeds and the root exposes a custom view directory whose filter is exactly `status:pending sched.after:sonw sched.before:eonw`

#### Scenario: Filter is not given an implicit status

- **WHEN** a pending task and a completed task both have scheduled dates inside the next week and the user defines `--view "Next Week=sched.after:sonw sched.before:eonw"`
- **THEN** the custom view directory lists files for both tasks unless the filter itself adds a status criterion

#### Scenario: View flag position does not matter

- **WHEN** the user runs `task-fs mount --view "Due=due.before:now" ~/m` and `task-fs mount ~/m --view "Due=due.before:now"`
- **THEN** both mounts define the same custom view directory

### Requirement: Custom view names are validated at mount

Each custom view name SHALL be non-empty, no longer than 254 bytes, free of `/` and control characters, and distinct from `.` and `..`. A leading `@` in the flag value SHALL be accepted and removed. The name SHALL NOT match the filesystem's minted task-file pattern. Duplicate names SHALL fail the mount with an error naming the offending custom view, and nothing SHALL be mounted.

#### Scenario: Leading at sign is accepted

- **WHEN** the user runs `task-fs mount ~/m --view "@Due=due.before:now"`
- **THEN** the mount succeeds and the custom view directory is named `@Due`

#### Scenario: Invalid custom view name fails the mount

- **WHEN** the user runs `task-fs mount ~/m --view "a/b=due.before:now"`
- **THEN** the command exits non-zero with an error naming `a/b` and mounts nothing

#### Scenario: Duplicate custom view names fail the mount

- **WHEN** the user runs `task-fs mount ~/m --view "Due=due.before:now" --view "Due=due.after:now"`
- **THEN** the command exits non-zero with an error naming `Due` and mounts nothing

#### Scenario: Minted task-file name is rejected

- **WHEN** the user runs `task-fs mount ~/m --view "Report.a1b2c3d4.md=status:pending"`
- **THEN** the command exits non-zero and mounts nothing

### Requirement: Custom view filters are validated at mount

The mount command SHALL render each custom view filter and run the resulting filter against the backend before mounting. A command that fails, produces unacceptable output, or yields a filter the backend rejects as malformed SHALL fail the mount with an error naming the offending custom view, and nothing SHALL be mounted. A valid filter that matches zero tasks SHALL be accepted.

#### Scenario: Malformed filter fails the mount

- **WHEN** the user runs `task-fs mount ~/m --view "Broken=( status:pending"`
- **THEN** the command exits non-zero with an error naming `Broken` and mounts nothing

#### Scenario: Zero-match filter is valid

- **WHEN** the user runs `task-fs mount ~/m --view "Nothing=project:does-not-exist"` on a database with no such project
- **THEN** the mount succeeds and the root exposes an empty `@Nothing` custom view directory

#### Scenario: Dynamic filter is validated after rendering

- **WHEN** the user mounts with a dynamic view whose command succeeds and whose rendered filter is valid
- **THEN** the mount succeeds even if the rendered filter matches zero tasks

#### Scenario: Dynamic command failure fails the mount

- **WHEN** a custom view command exits non-zero during mount validation
- **THEN** the command exits non-zero with an error naming the custom view and mounts nothing

#### Scenario: Malformed rendered filter fails the mount

- **WHEN** a command succeeds but writes output that makes the rendered filter malformed
- **THEN** the command exits non-zero with an error naming the custom view and mounts nothing

### Requirement: Custom view filters support command substitutions

A custom view filter SHALL support zero or more `{{ command }}` blocks. Each block SHALL run in a POSIX shell and its standard output, with one trailing newline removed and surrounding whitespace trimmed, SHALL replace the block as literal filter text. A backslash before an opening delimiter SHALL produce a literal `{{`, and blocks SHALL be rendered independently.

#### Scenario: Command output supplies a filter value

- **WHEN** the user mounts with `--view 'Payday=status:pending due.before:{{ payday-date }}'` and `payday-date` writes `2026-11-01`
- **THEN** the custom view's filter is `status:pending due.before:2026-11-01`

#### Scenario: Multiple blocks are rendered in one filter

- **WHEN** a custom view filter contains two command blocks and both commands succeed
- **THEN** each block is replaced by its own command's trimmed output and the rest of the filter is unchanged

#### Scenario: Output containing spaces becomes filter text

- **WHEN** a command block writes `project:home or project:finances`
- **THEN** the view filter contains that complete filter expression rather than one literal argument name

#### Scenario: Escaped delimiter stays literal

- **WHEN** the user defines a filter containing `\{{ not a command }}`
- **THEN** the view filter contains the literal text `{{ not a command }}` and no command is executed for it

### Requirement: Command substitution templates are validated at mount

At mount, every custom view filter SHALL be parsed as a template. A block with an empty command, an unclosed opening delimiter, or a trailing escape SHALL fail the mount with an error naming the custom view, and nothing SHALL be mounted.

#### Scenario: Empty command block fails the mount

- **WHEN** the user runs `task-fs mount ~/m --view 'Broken=due.before:{{ }}'`
- **THEN** the command exits non-zero with an error naming `Broken` and mounts nothing

#### Scenario: Unclosed block fails the mount

- **WHEN** the user runs `task-fs mount ~/m --view 'Broken=due.before:{{ date +%F'`
- **THEN** the command exits non-zero with an error naming `Broken` and mounts nothing

### Requirement: Dynamic view commands use the mount environment

Command blocks SHALL run with the mount process's environment and working directory, so commands see the same Taskwarrior configuration and database selection as backend queries.

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

Before listing a custom view or resolving, removing, or sweeping a task in it, the system SHALL render its command substitutions for that operation. A rendered result MAY be reused only for concurrent or immediately following lookups within one traversal; a later traversal SHALL render again.

#### Scenario: Changed command output appears without remounting

- **WHEN** a view command starts writing a date that brings a new task into the filter and the user lists the view again
- **THEN** the new task's file appears without remounting

#### Scenario: One traversal does not run the command per file

- **WHEN** a directory traversal lists a custom view and then looks up several of its files
- **THEN** the command runs once for that traversal rather than once per file lookup

#### Scenario: A later traversal observes new output

- **WHEN** the command output changes after one listing completes and the user lists the view again
- **THEN** the second listing uses the newly rendered filter
