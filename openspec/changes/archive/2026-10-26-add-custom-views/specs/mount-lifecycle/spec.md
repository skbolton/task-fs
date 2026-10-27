# Spec Delta

## ADDED Requirements

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

The mount command SHALL run each custom view filter against the backend before mounting. A filter the backend rejects as malformed SHALL fail the mount with an error naming the offending custom view, and nothing SHALL be mounted. A valid filter that matches zero tasks SHALL be accepted.

#### Scenario: Malformed filter fails the mount

- **WHEN** the user runs `task-fs mount ~/m --view "Broken=( status:pending"`
- **THEN** the command exits non-zero with an error naming `Broken` and mounts nothing

#### Scenario: Zero-match filter is valid

- **WHEN** the user runs `task-fs mount ~/m --view "Nothing=project:does-not-exist"` on a database with no such project
- **THEN** the mount succeeds and the root exposes an empty `@Nothing` custom view directory
