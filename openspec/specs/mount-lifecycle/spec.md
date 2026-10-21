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
