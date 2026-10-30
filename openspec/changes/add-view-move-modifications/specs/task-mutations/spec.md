# Spec Delta

## ADDED Requirements

### Requirement: Writing _view updates the view definition

The system SHALL allow creating, writing, and truncating a runtime view's `_view` file. The first line SHALL become the view's filter and the optional second line SHALL become its modification arguments, taking effect on the next view access. Writing `_view` SHALL NOT change any task. Invalid filter, template, or definition text SHALL be stored and SHALL make the affected view operation fail with EIO until corrected.

#### Scenario: Creating an empty definition makes it visible

- **WHEN** the user runs `touch @Due/_view`
- **THEN** refreshing `@Due` lists `_view`, shows no task files, and moving a task into the view is rejected as read-only

#### Scenario: Writing a filter sets the view filter

- **WHEN** the user runs `echo 'status:pending +next' > @Due/_view`
- **THEN** listing `@Due` shows the tasks matching `status:pending +next` on the next listing and the view remains read-only

#### Scenario: Writing a modification line makes the view writable

- **WHEN** the user writes `status:pending sched.after:sonw sched.before:eonw` on the first line of `@Due Next/_view` and `sched:{{cycle this week}}` on the second line
- **THEN** the view is writable on the next move and its listing still uses only the first line

#### Scenario: Rewriting the definition replaces both lines

- **WHEN** the user rewrites an existing `_view` with a new first line and no second line
- **THEN** the next listing reflects only the new filter and the view is read-only again

#### Scenario: Truncating the definition empties the view

- **WHEN** the user truncates `@Due/_view` to zero bytes
- **THEN** the view remains listed, `_view` remains listed and is empty, and the view shows no task files

#### Scenario: Invalid definition is stored but unreadable

- **WHEN** the user writes a malformed filter, an invalid template, or more than two definition lines into `_view`
- **THEN** the write succeeds, no task changes, and the next view listing fails with EIO

### Requirement: Removing _view clears the view

Removing a runtime view's `_view` file SHALL clear the view's filter and modification line, remove `_view` from the view listing, and leave the view listed at the root with no entries. It SHALL NOT remove the view definition or change any task.

#### Scenario: Removing the definition empties but preserves the view

- **WHEN** the user runs `rm @Due/_view` while `@Due` is listed at the root
- **THEN** `@Due` remains listed as a no-op, `_view` is gone, no task files are shown, and no task changes

### Requirement: Moving into a writable view applies its modification line

Moving a task file from a filter directory into a runtime view whose `_view` has a non-empty second line SHALL render and apply that line as the Taskwarrior modify arguments for the task, as if they followed `task <uuid> modify`. The move SHALL NOT derive a project or tag delta from the source directory. A view without such a line SHALL reject the move with EACCES and SHALL NOT change the task. A failed render or rejected arguments SHALL fail the move with EIO and SHALL NOT change the task.

#### Scenario: Moving a task into a scheduled view

- **WHEN** a writable view's second line is `sched:{{cycle this week}}` and the user moves a task file from `/home` into that view
- **THEN** the task's schedule is set to the rendered week and its project and tags are unchanged

#### Scenario: Modification line uses shell-style arguments

- **WHEN** a writable view's second line is `description:"call mom" +family`
- **THEN** the task is modified with the two arguments `description:call mom` and `+family`

#### Scenario: Move into a read-only view is rejected

- **WHEN** the user moves a task file from `/home` into a view whose `_view` has no second line
- **THEN** the move fails with EACCES and the task is unchanged

#### Scenario: Failed modification command changes nothing

- **WHEN** a writable view's second line contains a command block that exits non-zero and the user moves a task file into it
- **THEN** the move fails with EIO and the task is unchanged

#### Scenario: Invalid modify arguments change nothing

- **WHEN** Taskwarrior rejects the rendered modification arguments and the user moves a task file into the view
- **THEN** the move fails with EIO and the task is unchanged

#### Scenario: Task that no longer matches disappears after the move

- **WHEN** the user moves a task into a writable view but the applied arguments do not make it match the view's first line
- **THEN** the move succeeds and the task file does not appear in that view's next listing

#### Scenario: Moving a completed task into a view keeps its status

- **WHEN** the user moves a task file from `/+COMPLETED` into a writable view
- **THEN** the view's modification arguments are applied and the task remains completed

#### Scenario: Renaming on the way into a view is rejected

- **WHEN** the user moves a task file from `/home` into a writable view under a different file name
- **THEN** the move fails with EACCES and the task is unchanged

## MODIFIED Requirements

### Requirement: Root mkdir creates a runtime view

The system SHALL create a persisted runtime view when the user runs `mkdir @NAME` at the root. The new view SHALL be listed at the root as a no-op with an empty directory; `_view` SHALL appear once it is created, even if empty. Creating a view SHALL NOT change any task. Creating a view whose name a persisted runtime view already uses SHALL fail with EEXIST.

#### Scenario: Creating a view

- **WHEN** the user runs `mkdir @Due` at the root
- **THEN** the root lists `@Due`, listing `@Due` shows no entries, and no task changes

#### Scenario: New view survives remount

- **WHEN** the user creates `mkdir @Due`, writes a filter into `_view`, and remounts
- **THEN** `@Due` is listed with the same filter

#### Scenario: Duplicate view fails

- **WHEN** the runtime view `@Due` already exists and the user runs `mkdir @Due` at the root
- **THEN** the operation fails with EEXIST and the existing definition is unchanged

### Requirement: Touching a file creates a task in the current context

The system SHALL create a new Taskwarrior task when the user creates a new file in a filter directory other than a runtime view directory, whose name does not end in the filesystem's minted-name pattern, using the file name as the task description and the current directory's filter as the new task's project and tags. Creation SHALL use the write filter derived by dropping virtual-tag segments from the current filter, mirroring Taskwarrior's context write filters, since virtual tags cannot be attached to tasks. Creating a file other than `_view` in a runtime view directory SHALL fail.

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

The system SHALL fail with a permission error and change no task when the user attempts to create a file other than `_view`, create a subdirectory, rename a task file, or move a task file out of a runtime view directory. Moving a task into a runtime view SHALL follow the writable-view rule: EACCES without a non-empty modification line, and the rendered modification arguments otherwise. Moves out of a view SHALL be rejected even when the destination is a status virtual-tag directory.

#### Scenario: Touching inside a custom view directory is rejected

- **WHEN** the user runs `touch "New task"` inside a runtime view directory
- **THEN** the operation fails and no task is created

#### Scenario: Creating a directory inside a custom view directory is rejected

- **WHEN** the user runs `mkdir +urgent` inside a runtime view directory
- **THEN** the operation fails and no declaration is created

#### Scenario: Moving a task into a custom view directory is rejected

- **WHEN** the user moves a task file from `/home` into a runtime view with no modification line
- **THEN** the operation fails with EACCES and the task's project and tags are unchanged

#### Scenario: Moving a task out of a custom view directory is rejected

- **WHEN** the user moves a task file from a runtime view directory into `/home`
- **THEN** the operation fails and the task's project and tags are unchanged

#### Scenario: Moving a custom view result into a status view is rejected

- **WHEN** the user moves a task file from a runtime view directory into `/+COMPLETED/`
- **THEN** the operation fails and the task's status is unchanged

#### Scenario: Renaming a task file inside a custom view is rejected

- **WHEN** the user renames a task file inside a runtime view directory
- **THEN** the operation fails and no task changes

## REMOVED Requirements

### Requirement: Writing _filter updates the view filter

**Reason**: The view definition file is renamed to `_view` and can now carry a second line of modification arguments.

**Migration**: Rename `@NAME/_filter` to `@NAME/_view`. The first line keeps its previous meaning as the filter; add a second line only when the view should accept moved tasks.

### Requirement: Removing _filter clears the view

**Reason**: The view definition file is renamed to `_view`.

**Migration**: Use `rm @NAME/_view` instead of `rm @NAME/_filter`.
