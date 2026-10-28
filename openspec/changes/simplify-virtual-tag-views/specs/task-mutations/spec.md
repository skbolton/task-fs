# Spec Delta

## MODIFIED Requirements

### Requirement: rm applies the configured removal action

The system SHALL apply the configured removal action when the user removes a task file with `rm`, resolving the task by its full UUID. With `delete` it SHALL mark the task deleted; with `complete` it SHALL mark a pending or waiting task completed. It SHALL leave a task already completed or deleted unchanged and still report success, so a recursive walk over tombstones and already-completed files converges.

#### Scenario: Removing a pending task under complete

- **WHEN** the user mounts with `--rm complete` and runs `rm "Do taxes.a1b2c3d4.md"` in a directory listing that pending task
- **THEN** the task's status becomes completed and its file no longer appears in pending listings

#### Scenario: Removing a pending task under delete

- **WHEN** the user mounts with `--rm delete` and removes a pending task's file
- **THEN** the task's status becomes deleted and its file no longer appears in pending listings

#### Scenario: Completed task is unchanged under complete

- **WHEN** the user mounts with `--rm complete` and removes a task file listed in `/+COMPLETED`
- **THEN** the task remains completed and no completion command is issued

#### Scenario: Removing a deleted task's file is a no-op

- **WHEN** the user removes a task file listed in `/+DELETED`, under either removal action
- **THEN** the operation succeeds without invoking Taskwarrior's delete or done, and the task remains in the deleted status

### Requirement: Moving into COMPLETED completes the task

The system SHALL mark the task completed when the user moves its file into the `+COMPLETED` directory from a filter directory other than a custom view directory, ignoring the remainder of the destination path beyond the status segment. `+COMPLETED` SHALL resolve whether or not any task is currently completed, and `.+COMPLETED` SHALL NOT be a valid destination. The system SHALL reopen the task (status pending) when the user moves its file out of `+COMPLETED` and SHALL additionally apply the destination filter delta per the match-the-destination rule. Moves whose source or destination is a custom view directory are rejected.

#### Scenario: Completing from a project directory

- **WHEN** the user runs `mv "Do taxes.a1b2c3d4.md" /+COMPLETED/`
- **THEN** the task's status becomes completed and its file no longer appears in pending listings

#### Scenario: Completing works when no task is completed yet

- **WHEN** the database has no completed tasks and the user runs `mv "Do taxes.a1b2c3d4.md" /+COMPLETED/`
- **THEN** the task's status becomes completed

#### Scenario: Dot-prefixed completion destination is rejected

- **WHEN** the user runs `mv "Do taxes.a1b2c3d4.md" /.+COMPLETED/`
- **THEN** the move fails with ENOENT and the task's status is unchanged

#### Scenario: Moving out of COMPLETED reopens and re-locates

- **WHEN** the user moves a task file from `/+COMPLETED/` into the existing directory `/finances/`
- **THEN** the task's status becomes pending, its project becomes `finances`, and its file appears in `/finances`

### Requirement: Moving into DELETED deletes the task

The system SHALL mark the task deleted when the user moves its file into the `+DELETED` directory from a filter directory other than a custom view directory, ignoring the remainder of the destination path beyond the status segment. `+DELETED` SHALL resolve whether or not any task is currently deleted, and `.+DELETED` SHALL NOT be a valid destination. The system SHALL leave an already-deleted task unchanged and report success. Moving a task file out of `+DELETED` reopens it under the same rule as moving out of `+COMPLETED`. Moves whose source or destination is a custom view directory are rejected.

#### Scenario: Deleting by moving into DELETED

- **WHEN** the user runs `mv "Do taxes.a1b2c3d4.md" /+DELETED/`
- **THEN** the task's status becomes deleted and its file no longer appears in pending listings

#### Scenario: Deleting works when no task is deleted yet

- **WHEN** the database has no deleted tasks and the user runs `mv "Do taxes.a1b2c3d4.md" /+DELETED/`
- **THEN** the task's status becomes deleted

#### Scenario: Dot-prefixed deletion destination is rejected

- **WHEN** the user runs `mv "Do taxes.a1b2c3d4.md" /.+DELETED/`
- **THEN** the move fails with ENOENT and the task's status is unchanged

#### Scenario: Moving out of DELETED reopens and re-locates

- **WHEN** the user moves a task file from `/+DELETED/` into the existing directory `/finances/`
- **THEN** the task's status becomes pending, its project becomes `finances`, and its file appears in `/finances`

#### Scenario: Already-deleted task is unchanged

- **WHEN** the user moves a task file from `/+DELETED/` into another directory filtered by `+DELETED`
- **THEN** the task remains deleted and the operation succeeds without issuing another delete
