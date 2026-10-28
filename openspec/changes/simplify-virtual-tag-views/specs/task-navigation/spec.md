# Spec Delta

## ADDED Requirements

### Requirement: Status virtual-tag segments expose derived filters

The system SHALL support `+COMPLETED` and `+DELETED` as the only virtual-tag segments, mapping to Taskwarrior's completed and deleted status filters. Both SHALL resolve in every filter directory, including directories reached through project or tag segments, and SHALL be listed as regular `+NAME` entries unless the directory already carries a virtual-tag segment.

#### Scenario: Browsing completed tasks

- **WHEN** the user lists `/+COMPLETED/home`
- **THEN** the listing shows files for completed tasks whose project is `home` or a subproject of it

#### Scenario: Browsing deleted tasks

- **WHEN** the user lists `/+DELETED`
- **THEN** the listing shows files for deleted tasks across all projects

#### Scenario: Empty status views stay listed

- **WHEN** no task is completed and the user lists the root or a project directory
- **THEN** `+COMPLETED` is still listed and entering it shows no task files

### Requirement: Other uppercase plus segments are tag filters

The dot-prefixed status spellings `.+COMPLETED` and `.+DELETED` SHALL fail with ENOENT. Any other uppercase `+` segment SHALL be treated as a tag filter and SHALL resolve only when a task matching the current filter carries that tag.

#### Scenario: Dot spelling no longer resolves

- **WHEN** the user attempts to enter `/.+COMPLETED` or `/.+DELETED`
- **THEN** the lookup fails with ENOENT

#### Scenario: Bare uppercase + segment is not a virtual tag

- **WHEN** the user attempts to enter `/+DUE` while no task matching the current filter carries a literal `DUE` tag
- **THEN** the lookup fails with ENOENT

### Requirement: Status virtual-tag directory removal follows the configured action

Removing a `+COMPLETED` or `+DELETED` directory SHALL NOT change any task. Under `delete` it SHALL fail with ENOTEMPTY while any task with a status other than deleted matches the view, and succeed once none does. Under `complete` it SHALL fail only while the view holds a task that is neither completed nor deleted, and otherwise succeed, so recursive removal converges.

#### Scenario: Delete action guards a view with completed tasks

- **WHEN** the user mounts with `--rm delete` and runs `rmdir +COMPLETED` in `/this` while completed tasks in `this.*` exist
- **THEN** the operation fails with ENOTEMPTY and no task data changes

#### Scenario: Delete action completes a recursive project removal

- **WHEN** the user mounts with `--rm delete` and runs `rm -rf this` at the root, so each task file inside the project's status views is unlinked before those directories are removed
- **THEN** the view removals succeed once their tasks are deleted, the project's remaining matching tasks are deleted when `this` is removed, and no error is reported

#### Scenario: Complete action converges with pre-existing completed tasks

- **WHEN** the user mounts with `--rm complete` and runs `rm -rf this` while project `this` has pending tasks and already-completed tasks
- **THEN** the pending tasks become completed, the completed tasks are unchanged, every status view directory removal succeeds, and no error is reported

#### Scenario: Complete action removes either status view

- **WHEN** the user mounts with `--rm complete` and runs `rmdir +COMPLETED` and `rmdir +DELETED` in `/this` while completed and deleted tasks exist
- **THEN** both operations succeed without changing any task

## MODIFIED Requirements

### Requirement: Path segments are an order-free filter

The system SHALL interpret a filesystem path as the conjunction of its segments' filters, where a `+COMPLETED` or `+DELETED` segment is a status virtual-tag filter, any other segment starting with `+` is a tag filter, and every other segment is a project filter. At the root, a segment matching a configured custom view name SHALL select that custom view directory instead of a project filter; custom view directories do not accumulate filters. The order of segments SHALL NOT change the resulting filter.

#### Scenario: Equivalent paths in any order

- **WHEN** the user lists `/+next/home` and `/home/+next` on a database with a task in project `home.clean-garage` tagged `next`
- **THEN** both listings show the same set of task files

#### Scenario: Deepening under a tag

- **WHEN** the user is in `/+next` and lists its entries
- **THEN** the listing contains project segments for projects of tasks tagged `next` (for example `home`, `finances`) and tag segments for other tags of those tasks

### Requirement: Root lists the filter vocabulary of pending tasks

The root directory SHALL list one entry per first-level project segment, one entry per tag of pending tasks, the `+COMPLETED` and `+DELETED` virtual-tag views, and one file per pending task that has no project.

#### Scenario: Root of the example database

- **WHEN** pending tasks exist in projects `home.clean-garage` and `finances.2026-taxes`, with task `home.clean-garage` tagged `next`
- **THEN** the root listing contains the directories `home`, `finances`, `+next`, `+COMPLETED`, and `+DELETED`

### Requirement: Filter segments can be declared before any task matches them

The system SHALL allow creating a `+name` directory with `mkdir` in a filter directory as a declaration that the tag namespace exists at that location, even when no task matching the current filter carries the tag. The system SHALL likewise allow `mkdir` of a project segment that exists nowhere in the database, so that a task can be moved into a brand-new project. Declarations persist for the life of the mount session. Creating a directory in a custom view directory SHALL fail. Creating a directory whose name is `+COMPLETED`, `+DELETED`, any other fully-uppercase `+` name, or any dot-prefixed name SHALL fail.

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

- **WHEN** the user runs `mkdir +COMPLETED`, `mkdir +DUE`, or `mkdir .+COMPLETED`
- **THEN** each operation fails and no declaration is created

#### Scenario: Removing a declared segment that is in use

- **WHEN** the user runs `rmdir +urgent` in `/home` while tasks in `home.*` carry the tag
- **THEN** those tasks are completed under `complete` or deleted under `delete` according to the configured removal action, and the declaration is removed, so `/home/+urgent` no longer appears

### Requirement: Virtual-tag views do not nest

A directory whose accumulated filter already carries a virtual-tag segment SHALL NOT list further virtual-tag entries. Without this bound the tree below a project is combinatorial and a recursive walk of a project directory cannot finish.

#### Scenario: Inside a virtual-tag view no further views are listed

- **WHEN** the user lists `/+COMPLETED`
- **THEN** the listing contains neither `+COMPLETED` nor `+DELETED`

## REMOVED Requirements

### Requirement: Virtual-tag segments expose derived filters

**Reason**: Only `+COMPLETED` and `+DELETED` remain as virtual-tag segments. Discovery-driven virtual tags, the `ALL`/selected set, and the dot-prefixed presentation and lookup spelling are removed.

**Migration**: Use `+COMPLETED` or `+DELETED` for status browsing. Define other derived filters as custom views, for example `--view 'Due=+DUE'` instead of browsing `+DUE` in the tree. Replace `.+COMPLETED` and `.+DELETED` with the `+` spelling.

### Requirement: Virtual-tag directory removal follows the configured action

**Reason**: Replaced by the status-only removal rule. Views that hold actionable tasks other than the two status views no longer exist, so the former `+PENDING`-style guard scenarios no longer apply.

**Migration**: Remove virtual-tag directories with `rmdir` or `rm -rf` against `+COMPLETED` or `+DELETED`; the configured removal action remains unchanged and the view is never modified by its own removal.
