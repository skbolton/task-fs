# task-files Delta

## Purpose

Owns how individual tasks appear as files in the mounted tree: their names, their file content, and the naming rules that keep one entry per task stable as the database changes.

## ADDED Requirements

### Requirement: One file per task named description plus short UUID

The system SHALL present each task matching the current filter as exactly one file named `<description>.<short-uuid>.md`, where `<short-uuid>` is the first segment of the task's UUID (its leading 8 hexadecimal characters).

#### Scenario: Listing shows readable names

- **WHEN** a pending task has description `Qt6 signals and slots` and UUID `f88d7a5f-8d68-44fd-8cd3-3725592b673e`
- **THEN** its entry appears as a file named `Qt6 signals and slots.f88d7a5f.md` in every directory whose filter matches it

#### Scenario: Duplicate descriptions remain distinct entries

- **WHEN** two tasks share the description `Do taxes`
- **THEN** the listing shows two files with the same description prefix and different UUID suffixes

### Requirement: Short UUID suffix is uniform and stable

The system SHALL append the short-UUID suffix to every task file regardless of whether the description collides, so that a task's file name does not change when other tasks are added or removed.

#### Scenario: Name survives later collisions

- **WHEN** a task file `Do taxes.a1b2c3d4.md` exists and a second task is later created with description `Do taxes`
- **THEN** the first task's file name remains `Do taxes.a1b2c3d4.md`

### Requirement: Identical names are not further disambiguated

The system SHALL NOT apply additional disambiguation when two tasks share both description and short-UUID suffix; such tasks collapse to a single visible directory entry, and the user resolves the ambiguity through Taskwarrior itself.

#### Scenario: Description and short UUID both collide

- **WHEN** two tasks have the description `Do taxes` and UUIDs that share the first 8 hexadecimal characters
- **THEN** the directory shows one entry for that name, and file operations on it act on the task the filesystem resolves for that inode

### Requirement: Task files are empty placeholders

Reading a task file SHALL return empty content; the file name is the preview and task details remain available through Taskwarrior itself.

#### Scenario: Reading a task file

- **WHEN** the user runs `cat` on a task file
- **THEN** the output is empty and the command succeeds

### Requirement: Task files reject writes

The system SHALL fail writes to a task file, since task content is owned by Taskwarrior in v1; the write path is reserved for a future note system where editing the file edits the task's notes.

#### Scenario: Writing to a task file

- **WHEN** the user redirects output into a task file (`echo x > taskfile`)
- **THEN** the write fails with an error and the task is unchanged

### Requirement: Names are sanitized deterministically

The system SHALL derive task file names from descriptions deterministically, mangling every character that is invalid in a filename on either Linux or Windows (including `/`, `\`, `:`, `*`, `?`, `"`, `<`, `>`, `|`), handling Windows reserved device names and trailing dots or spaces, and truncating names that exceed the filesystem's name-length limit while always retaining the short-UUID suffix and the `.md` extension.

#### Scenario: Description containing a slash

- **WHEN** a task has description `Fix A/B testing` with short UUID `a1b2c3d4`
- **THEN** the file name contains a single deterministic replacement for the `/` and ends in `.a1b2c3d4.md`, and the same name is produced on every listing

### Requirement: Full UUID is exposed as an extended attribute

The system SHALL expose the task's full UUID as the value of an extended attribute (`user.taskwarrior.uuid`) on every task file.

#### Scenario: Resolving identity for scripting

- **WHEN** the user reads the `user.taskwarrior.uuid` extended attribute of a task file
- **THEN** the value is the task's full UUID as reported by Taskwarrior for that task
