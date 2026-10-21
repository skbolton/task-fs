# Spec Delta

## ADDED Requirements

### Requirement: Removing a directory deletes its tasks

The system SHALL treat removal of a directory whose segment is not a virtual-tag segment as a deletion gesture over that directory's child filter: every matching task with a status other than deleted SHALL be resolved to its full UUID and deleted, regardless of project depth or whether the directory was declared or derived. Removal SHALL succeed for a name whose tasks are all already deleted. Removal of a name that never held a task or declaration SHALL fail with ENOENT and change no task data.

#### Scenario: Removing a project removes the whole subtree

- **WHEN** the user runs `rmdir this` at the root while pending tasks exist in projects `this` and `this.sub` and a completed task exists in project `this.sub`
- **THEN** all three tasks are deleted, and `this` no longer appears in the root listing

#### Scenario: Removing a tag directory deletes tagged tasks across projects

- **WHEN** the user runs `rmdir +next` at the root while tasks tagged `next` exist in projects `home.clean-garage` and `finances`
- **THEN** every pending and completed task tagged `next` is deleted, and the `+next` directory no longer appears in the root listing

#### Scenario: Deleted tasks are not re-deleted

- **WHEN** the user runs `rmdir this` while one pending task and one already-deleted task have project `this`
- **THEN** the pending task is deleted, the already-deleted task is left in the deleted status, and the operation succeeds

#### Scenario: Removing a deleted task's file is a no-op

- **WHEN** the user removes a task file listed in a `.+DELETED` view
- **THEN** the operation succeeds without invoking Taskwarrior's delete, and the task remains in the deleted status

#### Scenario: Unknown name fails without changes

- **WHEN** the user runs `rmdir homi` while no task and no declaration matches `homi`
- **THEN** the operation fails with ENOENT and no task data changes
