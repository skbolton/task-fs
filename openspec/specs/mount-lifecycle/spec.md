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

### Requirement: Mount options select the exposed virtual tags

The mount command SHALL accept `--virtual-tags`/`-t` with a comma-separated list of virtual-tag names written without the `+` sign, or the single special value `ALL`. When the flag is absent the selection SHALL default to `COMPLETED,OVERDUE,DUE,ACTIVE,WAITING`. Every entry SHALL either be `ALL` or the fully-uppercase name of a virtual tag the backend discovers; a mount with any other entry SHALL fail with an error naming the invalid entry, and nothing SHALL be mounted.

#### Scenario: Default selection

- **WHEN** the user mounts without `--virtual-tags`
- **THEN** the views for `COMPLETED`, `OVERDUE`, `DUE`, `ACTIVE`, and `WAITING` are available and views for other discovered virtual tags are not

#### Scenario: Explicit list narrows the views

- **WHEN** the user runs `task-fs mount ~/m --virtual-tags "COMPLETED,DUE"`
- **THEN** only the `COMPLETED` and `DUE` virtual-tag views are available

#### Scenario: ALL restores the full discovered set

- **WHEN** the user runs `task-fs mount ~/m --virtual-tags ALL`
- **THEN** every fully-uppercase virtual tag the backend discovers is available as a view

#### Scenario: Invalid entry fails the mount

- **WHEN** the user runs `task-fs mount ~/m --virtual-tags "COMPLETED,bogus"`
- **THEN** the command exits non-zero with an error naming `bogus` and mounts nothing

### Requirement: Presentation of virtual-tag directories is selectable

The mount command SHALL accept `--virtual-hidden`/`-H` to present virtual-tag directories as hidden, dot-prefixed entries named `.+NAME`. Without the flag they SHALL be presented as regular, `+`-prefixed entries named `+NAME`. Presentation SHALL affect only entry names and listing visibility, not the filter a virtual-tag view resolves to.

#### Scenario: Hidden presentation

- **WHEN** the user mounts with `--virtual-hidden` and lists the root with a plain `ls`
- **THEN** the virtual-tag entries `.+COMPLETED`, `.+DUE`, ... are absent from the output and present with `ls -a`

#### Scenario: Regular presentation

- **WHEN** the user mounts without `--virtual-hidden` and lists the root with a plain `ls`
- **THEN** the virtual-tag entries `+COMPLETED`, `+DUE`, ... appear among the directories

#### Scenario: Presentation does not change the filter

- **WHEN** the user lists `/+COMPLETED` on a mount without `--virtual-hidden` and `/.+COMPLETED` on a mount with `--virtual-hidden` over the same database
- **THEN** both listings show the same completed task files

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
