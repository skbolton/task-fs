# Spec Delta

## ADDED Requirements

### Requirement: Root mkdir creates a runtime view

The system SHALL create a persisted runtime view when the user runs `mkdir @NAME` at the root. The new view SHALL be listed at the root as a no-op with an empty directory; `_filter` SHALL appear once it is created, even if empty. Creating a view SHALL NOT change any task. Creating a view whose name a persisted runtime view already uses SHALL fail with EEXIST.

#### Scenario: Creating a view

- **WHEN** the user runs `mkdir @Due` at the root
- **THEN** the root lists `@Due`, listing `@Due` shows no entries, and no task changes

#### Scenario: New view survives remount

- **WHEN** the user creates `mkdir @Due`, writes a filter into `_filter`, and remounts
- **THEN** `@Due` is listed with the same filter

#### Scenario: Duplicate view fails

- **WHEN** the runtime view `@Due` already exists and the user runs `mkdir @Due` at the root
- **THEN** the operation fails with EEXIST and the existing filter is unchanged

### Requirement: Runtime view names are validated

The name following `@` SHALL be non-empty, no longer than 254 bytes, free of `/` and control characters, distinct from `.` and `..`, and SHALL NOT match the filesystem's minted task-file pattern. An invalid name SHALL fail the creation with an error and SHALL NOT create a view or change any task.

#### Scenario: Empty view name is rejected

- **WHEN** the user runs `mkdir @` at the root
- **THEN** the operation fails and no view is created

#### Scenario: Control character in a view name is rejected

- **WHEN** the user runs `mkdir` with a name containing a control character
- **THEN** the operation fails and no view is created

#### Scenario: Dot name is rejected

- **WHEN** the user runs `mkdir @..` at the root
- **THEN** the operation fails and no view is created

#### Scenario: Minted task-file name is rejected

- **WHEN** the user runs `mkdir @Report.a1b2c3d4.md` at the root
- **THEN** the operation fails and no view is created

#### Scenario: Overlong view name is rejected

- **WHEN** the user runs `mkdir` with an `@` name longer than 254 bytes
- **THEN** the operation fails and no view is created

### Requirement: Writing _filter updates the view filter

The system SHALL allow creating, writing, and truncating a runtime view's `_filter` file. The persisted text SHALL become the view's filter and SHALL take effect on the next view access. Writing `_filter` SHALL NOT change any task. Invalid filter or template text SHALL be stored and SHALL make view access fail with EIO until corrected.

#### Scenario: Creating an empty filter makes it visible

- **WHEN** the user runs `touch @Due/_filter`
- **THEN** refreshing `@Due` lists `_filter` and shows no task files

#### Scenario: Creating the filter after mkdir

- **WHEN** the user creates `@Due/_filter` with an editor and writes `status:pending +next`
- **THEN** refreshing `@Due` lists `_filter` and the tasks matching `status:pending +next`

#### Scenario: Writing a filter sets the view filter

- **WHEN** the user runs `echo 'status:pending +next' > @Due/_filter`
- **THEN** listing `@Due` shows the tasks matching `status:pending +next` on the next listing

#### Scenario: Rewriting the filter replaces the view filter

- **WHEN** the user writes a second valid filter into an existing `_filter`
- **THEN** the next listing reflects only the new filter

#### Scenario: Truncating a filter empties the view

- **WHEN** the user truncates `@Due/_filter` to zero bytes
- **THEN** the view remains listed, `_filter` remains listed and is empty, and the view shows no task files

#### Scenario: Invalid filter is stored but unreadable

- **WHEN** the user writes text that Taskwarrior rejects into `_filter`
- **THEN** the write succeeds, no task changes, and the next listing fails with EIO

### Requirement: Removing _filter clears the view

Removing a runtime view's `_filter` file SHALL clear the view's filter, remove `_filter` from the view listing, and leave the view listed at the root with no entries. It SHALL NOT remove the view definition or change any task.

#### Scenario: Removing the filter empties but preserves the view

- **WHEN** the user runs `rm @Due/_filter` while `@Due` is listed at the root
- **THEN** `@Due` remains listed as a no-op, `_filter` is gone, no task files are shown, and no task changes

### Requirement: Runtime view definitions are removed with rmdir

Running `rmdir @NAME` at the root SHALL delete the runtime view definition and SHALL NOT change any task, even when the view matches tasks. The `@NAME` entry SHALL disappear from the root. A recursive removal (`rm -rf @NAME`) MAY first unlink matching task files under the configured removal action; `rmdir` is the gesture that discards a view without touching tasks.

#### Scenario: Removing a view with matching tasks

- **WHEN** the user runs `rmdir @Due` while tasks match its filter
- **THEN** the operation succeeds, `@Due` disappears from the root, and every task is unchanged

#### Scenario: Removing an empty view

- **WHEN** the user runs `rmdir @Due` with an empty `_filter`
- **THEN** the operation succeeds and `@Due` disappears from the root

#### Scenario: Recursive removal applies task removal first

- **WHEN** the user runs `rm -rf @Due` and the view matches pending tasks
- **THEN** the listed task files are unlinked under the configured removal action, then the view definition is deleted

## MODIFIED Requirements

### Requirement: Touching a file creates a task in the current context

The system SHALL create a new Taskwarrior task when the user creates a new file in a filter directory other than a runtime view directory, whose name does not end in the filesystem's minted-name pattern, using the file name as the task description and the current directory's filter as the new task's project and tags. Creation SHALL use the write filter derived by dropping virtual-tag segments from the current filter, mirroring Taskwarrior's context write filters, since virtual tags cannot be attached to tasks. Creating a file other than `_filter` in a runtime view directory SHALL fail.

#### Scenario: Adding a task inside a project and tag

- **WHEN** the user runs `touch "Pick up Milk"` in `/home/+next` where the filter is project `home` and tag `next`
- **THEN** a pending task is created with description `Pick up Milk`, project `home`, and tag `next`

#### Scenario: Creation context comes from the path

- **WHEN** the user touches a file in a directory whose filter contains a multi-level project segment
- **THEN** the created task's project is the full dotted project prefix of that directory

#### Scenario: Virtual tags are dropped on creation

- **WHEN** the user runs `touch "Pick up Milk"` in `/home/+COMPLETED`
- **THEN** a pending task is created with project `home` and no virtual-tag constraint, and the new file appears under `/home` rather than `/home/+COMPLETED`

#### Scenario: Creating a task file in a runtime view is rejected

- **WHEN** the user runs `touch "New task"` inside a runtime view directory
- **THEN** the operation fails and no task is created

### Requirement: Custom view directories reject creation and moves

The system SHALL fail with a permission error and change no task when the user attempts to create a file other than `_filter`, create a subdirectory, or move a task file into or out of a runtime view directory. This SHALL include moves whose source or destination is a status virtual-tag directory. Renaming a task file inside a runtime view directory SHALL also fail.

#### Scenario: Touching inside a custom view directory is rejected

- **WHEN** the user runs `touch "New task"` inside a runtime view directory
- **THEN** the operation fails and no task is created

#### Scenario: Creating a directory inside a custom view directory is rejected

- **WHEN** the user runs `mkdir +urgent` inside a runtime view directory
- **THEN** the operation fails and no declaration is created

#### Scenario: Moving a task into a custom view directory is rejected

- **WHEN** the user moves a task file from `/home` into a runtime view directory
- **THEN** the operation fails and the task's project and tags are unchanged

#### Scenario: Moving a task out of a custom view directory is rejected

- **WHEN** the user moves a task file from a runtime view directory into `/home`
- **THEN** the operation fails and the task's project and tags are unchanged

#### Scenario: Moving a custom view result into a status view is rejected

- **WHEN** the user moves a task file from a runtime view directory into `/+COMPLETED/`
- **THEN** the operation fails and the task's status is unchanged

## REMOVED Requirements

### Requirement: Custom view directory removal follows the view-emptiness rule

**Reason**: A runtime view is a saved filter rather than a task filter directory, so removing it should discard the saved view without requiring its matching tasks to be removed first.

**Migration**: Use `rmdir @NAME` to delete a runtime view definition without changing tasks. Removing `_filter` clears the filter but keeps the view; `rm -rf @NAME` still unlinks task files under the configured removal action before deleting the view.
