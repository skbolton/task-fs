# Spec Delta

## ADDED Requirements

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
