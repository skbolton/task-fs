# Spec Delta: task-navigation

## MODIFIED Requirements

### Requirement: Virtual-tag segments expose derived filters

The system SHALL support `+` segments as virtual-tag filters mapping to Taskwarrior virtual tags — entries named `+COMPLETED`, `.+DUE`, `.+OVERDUE`, and similar — including `+COMPLETED` (completed status) and the read-only filters Taskwarrior derives. A virtual-tag view SHALL resolve under both the `+NAME` and `.+NAME` spelling of a virtual tag selected at mount time, regardless of the presentation mode, and SHALL NOT resolve for a virtual tag that is not selected. Entries SHALL be listed only in the presentation form chosen at mount time: `+NAME` by default and `.+NAME` with hidden presentation. The dot form makes virtual-tag entries hidden files, so file managers and `ls` can toggle their visibility.

#### Scenario: Browsing completed tasks

- **WHEN** the user lists `/+COMPLETED/home`
- **THEN** the listing shows files for completed tasks whose project is `home` or a subproject of it

#### Scenario: Dot spelling still resolves without hidden presentation

- **WHEN** the user mounts without `--virtual-hidden` and lists `/.+COMPLETED/home`
- **THEN** the listing shows the same files as `/+COMPLETED/home`

#### Scenario: Virtual tags are hidden entries

- **WHEN** the user mounts with `--virtual-hidden` and lists `/` with a plain `ls` that hides dot files
- **THEN** the virtual-tag directories (`.+COMPLETED`, `.+DUE`, ...) are absent from the output and present with `ls -a`

#### Scenario: Bare uppercase + segment is not a virtual tag

- **WHEN** the user attempts to enter `/+BOGUS`, an uppercase name that is neither a selected virtual tag nor a tag of any task matching the current filter
- **THEN** the lookup fails with ENOENT

#### Scenario: Non-selected virtual tag does not resolve in dot spelling

- **WHEN** the user mounts with `--virtual-tags "COMPLETED"` and attempts to enter `/.+DUE`, while no task matching the current filter carries a `DUE` tag
- **THEN** the lookup fails with ENOENT

#### Scenario: Due-date tightening without filter syntax

- **WHEN** the user lists `/finances/+DUE`
- **THEN** the listing shows pending tasks in `finances.*` due within Taskwarrior's configured due window

### Requirement: Root lists the filter vocabulary of pending tasks

The root directory SHALL list one entry per first-level project segment, one entry per tag of pending tasks, one entry per virtual tag selected at mount time in its presentation form, and one file per pending task that has no project.

#### Scenario: Root of the example database

- **WHEN** pending tasks exist in projects `home.clean-garage` and `finances.2026-taxes`, with task `home.clean-garage` tagged `next`
- **THEN** the root listing contains the directories `home`, `finances`, and `+next`, and the selected virtual-tag directories such as `+COMPLETED` in their presentation form

### Requirement: Filter segments can be declared before any task matches them

The system SHALL allow creating a `+name` directory with `mkdir` as a declaration that the tag namespace exists at that location, even when no task matching the current filter carries the tag. The system SHALL likewise allow `mkdir` of a project segment that exists nowhere in the database, so that a task can be moved into a brand-new project. Declarations persist for the life of the mount session. Creating a directory whose name is a virtual-tag segment in either spelling (`+NAME` or `.+NAME`) or any other dot-prefixed name SHALL fail.

#### Scenario: Declaring a tag that exists nowhere

- **WHEN** the user runs `mkdir +urgent` in `/home` and no task in `home.*` carries the tag
- **THEN** `/home/+urgent` exists and is listable, showing no task files until a task receives the tag

#### Scenario: Declaring a brand-new project

- **WHEN** the user runs `mkdir /side-projects` while no task has project `side-projects`
- **THEN** `/side-projects` exists and a subsequent move of a task file into it sets the task's project to `side-projects`

#### Scenario: Redeclaring an existing segment

- **WHEN** the user runs `mkdir +next` in `/home` and tasks in `home.*` already carry `next`
- **THEN** the operation fails with EEXIST and the existing directory is unchanged

#### Scenario: Virtual-tag names are rejected

- **WHEN** the user runs `mkdir +DUE` or `mkdir .+DUE`
- **THEN** the operation fails and no declaration is created

#### Scenario: Removing a declared segment that is in use

- **WHEN** the user runs `rmdir +urgent` in `/home` while tasks in `home.*` carry the tag
- **THEN** the tasks carrying `urgent` under the current filter are deleted and the declaration is removed, so `/home/+urgent` no longer appears
