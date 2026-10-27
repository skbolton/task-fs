# Spec Delta

## MODIFIED Requirements

### Requirement: Custom view task removal applies the configured action

Removing a task file in a custom view directory SHALL resolve the task against the filter rendered for that removal, apply the configured removal action, resolve the task by its full UUID, and leave a task already completed or deleted unchanged while reporting success, consistent with removal elsewhere in the tree.

#### Scenario: Removing a custom view result under complete

- **WHEN** the user mounts with `--rm complete` and removes a pending task's file from a custom view directory
- **THEN** the task becomes completed and no longer appears in that custom view

#### Scenario: Removing a custom view result under delete

- **WHEN** the user mounts with `--rm delete` and removes a pending task's file from a custom view directory
- **THEN** the task becomes deleted and no longer appears in that custom view

#### Scenario: Already-completed custom view result is unchanged under complete

- **WHEN** the user mounts with `--rm complete` and removes a completed task's file listed in a custom view directory
- **THEN** the operation succeeds and the task remains completed

#### Scenario: Render failure blocks task removal

- **WHEN** a custom view command fails while the user removes a task file from that view
- **THEN** the removal fails with EIO and no task data changes

### Requirement: Custom view directory removal follows the view-emptiness rule

Removing a custom view directory SHALL NOT change any task and SHALL fail with ENOTEMPTY while a task the configured removal action would change matches the custom view filter rendered for that removal. It SHALL succeed once no such task matches. The configured custom view directory SHALL remain present after a successful removal, so recursive removal converges without deleting the saved custom view.

#### Scenario: Custom view directory removal is guarded under complete

- **WHEN** the user mounts with `--rm complete` and runs `rmdir "@Due Next Week"` while pending tasks match the custom view filter
- **THEN** the operation fails with ENOTEMPTY and no task changes

#### Scenario: Custom view directory removal is guarded under delete

- **WHEN** the user mounts with `--rm delete` and runs `rmdir "@Due Next Week"` while any non-deleted task matches the custom view filter
- **THEN** the operation fails with ENOTEMPTY and no task changes

#### Scenario: Recursive custom view removal converges

- **WHEN** the user runs `rm -rf "@Due Next Week"` and every matching task is unlinked under the configured action
- **THEN** the directory removal succeeds, the custom view directory remains configured, and no error is reported

#### Scenario: Custom view directory permits removal once no task would change

- **WHEN** the user mounts with `--rm complete` and runs `rmdir "@Completed"` while the custom view filter matches only completed tasks
- **THEN** the operation succeeds without changing any task and the custom view directory remains visible

#### Scenario: Render failure blocks directory removal

- **WHEN** a custom view command fails while the user removes that custom view directory
- **THEN** the removal fails with EIO and no task changes
