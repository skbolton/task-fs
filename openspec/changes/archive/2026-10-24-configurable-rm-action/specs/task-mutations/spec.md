# Spec Delta

## ADDED Requirements

### Requirement: rm applies the configured removal action

The system SHALL apply the configured removal action when the user removes a task file with `rm`, resolving the task by its full UUID. With `delete` it SHALL mark the task deleted; with `complete` it SHALL mark a pending or waiting task completed. It SHALL leave a task already completed or deleted unchanged and still report success, so a recursive walk over tombstones and already-completed files converges.

#### Scenario: Removing a pending task under complete

- **WHEN** the user mounts with `--rm complete` and runs `rm "Do taxes.a1b2c3d4.md"` in a directory listing that pending task
- **THEN** the task's status becomes completed and its file no longer appears in pending listings

#### Scenario: Removing a pending task under delete

- **WHEN** the user mounts with `--rm delete` and removes a pending task's file
- **THEN** the task's status becomes deleted and its file no longer appears in pending listings

#### Scenario: Completed task is unchanged under complete

- **WHEN** the user mounts with `--rm complete` and removes a task file listed in a `.+COMPLETED` view
- **THEN** the task remains completed and no completion command is issued

#### Scenario: Removing a deleted task's file is a no-op

- **WHEN** the user removes a task file listed in a `.+DELETED` view, under either removal action
- **THEN** the operation succeeds without invoking Taskwarrior's delete or done, and the task remains in the deleted status

### Requirement: Moving into DELETED deletes the task

The system SHALL mark the task deleted when the user moves its file into the DELETED status virtual-tag directory — spelled `+DELETED` or, equivalently, `.+DELETED` — from a view without a status segment, ignoring the remainder of the destination path beyond the status segment. The system SHALL leave an already-deleted task unchanged and report success. Moving a task file out of DELETED reopens it under the same rule as moving out of COMPLETED.

#### Scenario: Deleting by moving into DELETED

- **WHEN** the user runs `mv "Do taxes.a1b2c3d4.md" /+DELETED/`
- **THEN** the task's status becomes deleted and its file no longer appears in pending listings

#### Scenario: Moving out of DELETED reopens and re-locates

- **WHEN** the user moves a task file from `/.+DELETED/` into the existing directory `/finances/`
- **THEN** the task's status becomes pending, its project becomes `finances`, and its file appears in `/finances`

#### Scenario: Already-deleted task is unchanged

- **WHEN** the user moves a task file from a DELETED view into another DELETED view
- **THEN** the task remains deleted and the operation succeeds without issuing another delete

### Requirement: Removing a directory applies the configured removal action

The system SHALL treat removal of a non-virtual-tag directory as a removal gesture over its child filter: every matching task the configured action would change SHALL be completed (`complete`) or deleted (`delete`), regardless of project depth or whether the directory was declared or derived. Removal SHALL succeed when no matching task would change, including deleted tombstones and, under `complete`, already-completed tasks. An unknown name SHALL fail with ENOENT and change nothing.

#### Scenario: Removing a project completes its pending subtree under complete

- **WHEN** the user mounts with `--rm complete` and runs `rmdir this` at the root while pending tasks exist in projects `this` and `this.sub` and a completed task exists in project `this.sub`
- **THEN** the two pending tasks become completed, the completed task is unchanged, and `this` no longer appears in the root listing

#### Scenario: Removing a project deletes its subtree under delete

- **WHEN** the user mounts with `--rm delete` and runs `rmdir this` while pending tasks exist in projects `this` and `this.sub` and a completed task exists in project `this.sub`
- **THEN** all three tasks become deleted and `this` no longer appears in the root listing

#### Scenario: Removing a tag directory completes tagged tasks under complete

- **WHEN** the user mounts with `--rm complete` and runs `rmdir +next` at the root while pending tasks tagged `next` exist in projects `home.clean-garage` and `finances`
- **THEN** every pending task tagged `next` becomes completed, any completed task tagged `next` is unchanged, and the `+next` directory no longer appears in the root listing

#### Scenario: Deleted tasks are not re-deleted

- **WHEN** the user mounts with `--rm delete` and runs `rmdir this` while one pending task and one already-deleted task have project `this`
- **THEN** the pending task is deleted, the already-deleted task is left in the deleted status, and the operation succeeds

#### Scenario: Unknown name fails without changes

- **WHEN** the user runs `rmdir homi` while no task and no declaration matches `homi`
- **THEN** the operation fails with ENOENT and no task data changes

## REMOVED Requirements

### Requirement: rm deletes the task

**Reason**: Removal is now configured by `--rm`; the default action completes instead of deleting.

**Migration**: Mount with `--rm delete` to keep the previous behavior.

### Requirement: Removing a directory deletes its tasks

**Reason**: Directory removal now follows the configured removal action, which defaults to completion.

**Migration**: Mount with `--rm delete` to keep the previous behavior.
