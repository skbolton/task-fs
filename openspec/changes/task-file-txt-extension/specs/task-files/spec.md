# Spec Delta

## MODIFIED Requirements

### Requirement: One file per task named description plus short UUID

The system SHALL present each task matching the current filter as exactly one file named `<description>.<short-uuid>.txt`, where `<short-uuid>` is the first segment of the task's UUID (its leading 8 hexadecimal characters).

#### Scenario: Listing shows readable names

- **WHEN** a pending task has description `Qt6 signals and slots` and UUID `f88d7a5f-8d68-44fd-8cd3-3725592b673e`
- **THEN** its entry appears as a file named `Qt6 signals and slots.f88d7a5f.txt` in every directory whose filter matches it

#### Scenario: Duplicate descriptions remain distinct entries

- **WHEN** two tasks share the description `Do taxes`
- **THEN** the listing shows two files with the same description prefix and different UUID suffixes

### Requirement: Short UUID suffix is uniform and stable

The system SHALL append the short-UUID suffix to every task file regardless of whether the description collides, so that a task's file name does not change when other tasks are added or removed.

#### Scenario: Name survives later collisions

- **WHEN** a task file `Do taxes.a1b2c3d4.txt` exists and a second task is later created with description `Do taxes`
- **THEN** the first task's file name remains `Do taxes.a1b2c3d4.txt`

### Requirement: Task files reject writes

The system SHALL fail writes to a task file, since task content is owned by Taskwarrior and the filesystem does not provide an editing surface.

#### Scenario: Writing to a task file

- **WHEN** the user redirects output into a task file (`echo x > taskfile`)
- **THEN** the write fails with an error and the task is unchanged

### Requirement: Names are sanitized deterministically

The system SHALL derive task file names from descriptions deterministically, mangling every character that is invalid in a filename on either Linux or Windows (including `/`, `\`, `:`, `*`, `?`, `"`, `<`, `>`, `|`), handling Windows reserved device names and trailing dots or spaces, and truncating names that exceed the filesystem's name-length limit while always retaining the short-UUID suffix and the `.txt` extension.

#### Scenario: Description containing a slash

- **WHEN** a task has description `Fix A/B testing` with short UUID `a1b2c3d4`
- **THEN** the file name contains a single deterministic replacement for the `/` and ends in `.a1b2c3d4.txt`, and the same name is produced on every listing
