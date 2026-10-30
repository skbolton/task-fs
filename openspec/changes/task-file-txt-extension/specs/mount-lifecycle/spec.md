# Spec Delta

## MODIFIED Requirements

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

- **WHEN** the user runs `task-fs mount ~/m --view "Report.a1b2c3d4.txt=status:pending"`
- **THEN** the command exits non-zero and mounts nothing
