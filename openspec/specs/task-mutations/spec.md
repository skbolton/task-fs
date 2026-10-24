# task-mutations Specification

## Purpose

Owns the filesystem gestures that modify tasks through the mounted tree: creating tasks by touching files, deleting via `rm`, and changing status via moves into and out of status virtual-tag directories.

## Requirements

### Requirement: Touching a file creates a task in the current context

The system SHALL create a new Taskwarrior task when the user creates a new file whose name does not end in the filesystem's minted-name pattern, using the file name as the task description and the current directory's filter as the new task's project and tags. Creation SHALL use the write filter derived by dropping virtual-tag segments from the current filter, mirroring Taskwarrior's context write filters, since virtual tags cannot be attached to tasks.

#### Scenario: Adding a task inside a project and tag

- **WHEN** the user runs `touch "Pick up Milk"` in `/home/+next` where the filter is project `home` and tag `next`
- **THEN** a pending task is created with description `Pick up Milk`, project `home`, and tag `next`

#### Scenario: Creation context comes from the path

- **WHEN** the user touches a file in a directory whose filter contains a multi-level project segment
- **THEN** the created task's project is the full dotted project prefix of that directory

#### Scenario: Virtual tags are dropped on creation

- **WHEN** the user runs `touch "Pick up Milk"` in `/home/+DUE`
- **THEN** a pending task is created with project `home` and no virtual-tag constraint, and the new file appears under `/home` rather than `/home/+DUE`

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

### Requirement: Moving into COMPLETED completes the task

The system SHALL mark the task completed when the user moves its file into a status virtual-tag directory — spelled `+COMPLETED` or, equivalently, `.+COMPLETED` — ignoring the remainder of the destination path beyond the status segment. The system SHALL reopen the task (status pending) when the user moves its file out of a status virtual-tag directory and SHALL additionally apply the destination filter delta per the match-the-destination rule.

#### Scenario: Completing from a project directory

- **WHEN** the user runs `mv "Do taxes.a1b2c3d4.md" /+COMPLETED/`
- **THEN** the task's status becomes completed and its file no longer appears in pending listings

#### Scenario: Moving out of COMPLETED reopens and re-locates

- **WHEN** the user moves a task file from `/.+COMPLETED/` into the existing directory `/finances/`
- **THEN** the task's status becomes pending, its project becomes `finances`, and its file appears in `/finances`


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

### Requirement: Mutations identify tasks by full UUID

The system SHALL resolve every mutation gesture to the task's full UUID (carried on the file as an extended attribute) and SHALL NOT pass short-UUID prefixes, integer IDs, or description text to `task` as task identifiers.

#### Scenario: Gesture uses the full UUID

- **WHEN** the user removes a task file and the filesystem issues the corresponding `task` command
- **THEN** the command names the task by its full UUID, so garbage-collector ID reassignment cannot redirect the operation

### Requirement: Moving a task file makes the task match the destination

When the user moves a task file between directories other than status virtual-tag directories, the system SHALL apply the filter delta between the source and destination: the task's project SHALL be set to the destination's full project prefix, tags present in the destination filter but not the source SHALL be added, and tags present in the source filter but not the destination SHALL be removed.

#### Scenario: Moving from an inbox project to its home

- **WHEN** the user moves `Fix leak.a1b2c3d4.md` from `/inbox` into the existing directory `/finances/2026-taxes/`
- **THEN** the task's project becomes `finances.2026-taxes` and its tags are unchanged

#### Scenario: Tags follow the move

- **WHEN** the user moves a task file from `/home/+next` into `/finances/+urgent/`
- **THEN** the task's project becomes `finances`, tag `next` is removed, and tag `urgent` is added

#### Scenario: Moving to a parent project is exact

- **WHEN** the user moves a task file from `/finances/2026-taxes` into `/finances/`
- **THEN** the task's project becomes exactly `finances`, dropping the `2026-taxes` subproject

### Requirement: Move destinations can name undeclared projects

The system SHALL treat the final path component of a move destination as a project segment even when no task and no declaration matches it, provided the destination's parent chain resolves.

#### Scenario: Destination names a project that exists nowhere

- **WHEN** the user moves a task file to `/finances/2026-taxes` (no trailing slash) while no task has project `finances.2026-taxes` and `/finances` exists
- **THEN** the task's project becomes `finances.2026-taxes` and the directory appears on subsequent listings

#### Scenario: Destination parent chain does not resolve

- **WHEN** the user moves a task file to `/brandnew/` where no task has project `brandnew` and no declaration exists
- **THEN** the move fails with ENOENT and the task is unchanged

### Requirement: Unimplemented modifications are rejected

The system SHALL reject filesystem operations that would modify a task in ways v1 does not support, including renaming a task file, which would change the task description.

#### Scenario: Renaming a task file

- **WHEN** the user renames a task file within a directory (`mv "Old.a1b2c3d4.md" "New.a1b2c3d4.md"`)
- **THEN** the operation fails with an error indicating the operation is not supported, and the task's description is unchanged

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
