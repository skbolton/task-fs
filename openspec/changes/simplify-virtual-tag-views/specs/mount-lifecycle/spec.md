# Spec Delta

## ADDED Requirements

### Requirement: The mount exposes the status virtual-tag views

The mount command SHALL expose exactly the `+COMPLETED` and `+DELETED` virtual-tag views. Both SHALL be listed and resolvable in every filter directory that does not already carry a virtual-tag segment, whether or not any task currently matches, and SHALL remain available when their views are empty. The mount SHALL NOT discover or validate virtual-tag names against the backend.

#### Scenario: Empty database still exposes both views

- **WHEN** the database has no completed or deleted tasks and the user lists the root
- **THEN** the listing contains the directories `+COMPLETED` and `+DELETED`

#### Scenario: Status destinations are always available

- **WHEN** the database has no completed or deleted tasks and the user moves a task file into `/+COMPLETED/` or `/+DELETED/`
- **THEN** the move resolves the status view instead of failing with ENOENT and applies the corresponding complete or delete action

#### Scenario: Other derived filters are custom views

- **WHEN** the user runs `task-fs mount ~/m --view 'Due=+DUE'`
- **THEN** the root exposes the custom view directory `@Due` and does not expose a `+DUE` virtual-tag directory

### Requirement: Removed virtual-tag mount flags fail the mount

The mount command SHALL reject `--virtual-tags`, `-t`, `--virtual-hidden`, and `-H` as unknown options, exit non-zero with a usage message, and mount nothing.

#### Scenario: Removed selection flag fails the mount

- **WHEN** the user runs `task-fs mount ~/m --virtual-tags "COMPLETED,DUE"`
- **THEN** the command exits non-zero with a usage message and mounts nothing

#### Scenario: Removed presentation flag fails the mount

- **WHEN** the user runs `task-fs mount ~/m --virtual-hidden`
- **THEN** the command exits non-zero with a usage message and mounts nothing

#### Scenario: Removed short flags fail the mount

- **WHEN** the user runs `task-fs mount ~/m -t COMPLETED` or `task-fs mount ~/m -H`
- **THEN** each command exits non-zero with a usage message and mounts nothing

## REMOVED Requirements

### Requirement: Mount options select the exposed virtual tags

**Reason**: The selection flag is removed; the mount always exposes the two status virtual tags.

**Migration**: Remove `--virtual-tags`/`-t` from mount commands. Define other derived filters as custom views, for example `--view 'Due=+DUE'` instead of `--virtual-tags DUE`.

### Requirement: Presentation of virtual-tag directories is selectable

**Reason**: Hidden presentation is removed along with `--virtual-hidden`; both status views are always listed as regular `+NAME` directories.

**Migration**: Remove `--virtual-hidden`/`-H` from mount commands. Use `+COMPLETED` and `+DELETED`; the dot-prefixed spellings no longer resolve.
