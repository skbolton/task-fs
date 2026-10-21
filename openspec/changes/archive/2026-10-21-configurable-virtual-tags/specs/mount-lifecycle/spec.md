# Spec Delta: mount-lifecycle

## ADDED Requirements

### Requirement: Mount options select the exposed virtual tags

The mount command SHALL accept `--virtual-tags`/`-t` with a comma-separated list of virtual-tag names written without the `+` sign, or the single special value `ALL`. When the flag is absent the selection SHALL default to `COMPLETED,OVERDUE,DUE,ACTIVE,WAITING`. Every entry SHALL either be `ALL` or the fully-uppercase name of a virtual tag the backend discovers; a mount with any other entry SHALL fail with an error naming the invalid entry, and nothing SHALL be mounted.

#### Scenario: Default selection

- **WHEN** the user mounts without `--virtual-tags`
- **THEN** the views for `COMPLETED`, `OVERDUE`, `DUE`, `ACTIVE`, and `WAITING` are available and views for other discovered virtual tags are not

#### Scenario: Explicit list narrows the views

- **WHEN** the user runs `task-fs mount ~/m --virtual-tags "COMPLETED,DUE"`
- **THEN** only the `COMPLETED` and `DUE` virtual-tag views are available

#### Scenario: ALL restores the full discovered set

- **WHEN** the user runs `task-fs mount ~/m --virtual-tags ALL`
- **THEN** every fully-uppercase virtual tag the backend discovers is available as a view

#### Scenario: Invalid entry fails the mount

- **WHEN** the user runs `task-fs mount ~/m --virtual-tags "COMPLETED,bogus"`
- **THEN** the command exits non-zero with an error naming `bogus` and mounts nothing

### Requirement: Presentation of virtual-tag directories is selectable

The mount command SHALL accept `--virtual-hidden`/`-H` to present virtual-tag directories as hidden, dot-prefixed entries named `.+NAME`. Without the flag they SHALL be presented as regular, `+`-prefixed entries named `+NAME`. Presentation SHALL affect only entry names and listing visibility, not the filter a virtual-tag view resolves to.

#### Scenario: Hidden presentation

- **WHEN** the user mounts with `--virtual-hidden` and lists the root with a plain `ls`
- **THEN** the virtual-tag entries `.+COMPLETED`, `.+DUE`, ... are absent from the output and present with `ls -a`

#### Scenario: Regular presentation

- **WHEN** the user mounts without `--virtual-hidden` and lists the root with a plain `ls`
- **THEN** the virtual-tag entries `+COMPLETED`, `+DUE`, ... appear among the directories

#### Scenario: Presentation does not change the filter

- **WHEN** the user lists `/+COMPLETED` on a mount without `--virtual-hidden` and `/.+COMPLETED` on a mount with `--virtual-hidden` over the same database
- **THEN** both listings show the same completed task files
