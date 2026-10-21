# task-navigation Specification

## Purpose

Owns the directory tree of the mounted filesystem and its meaning as a task filter: the path grammar, the segment kinds, order-free path parsing, and which entries are visible at each level.

## Requirements

### Requirement: Path segments are an order-free filter

The system SHALL interpret a filesystem path as the conjunction of its segments' filters, where each segment starting with `+` is a tag filter, each segment starting with `.+` whose remainder is fully uppercase is a virtual-tag filter, and every other segment is a project filter. The order of segments SHALL NOT change the resulting filter.

#### Scenario: Equivalent paths in any order

- **WHEN** the user lists `/+next/home` and `/home/+next` on a database with a task in project `home.clean-garage` tagged `next`
- **THEN** both listings show the same set of task files

#### Scenario: Deepening under a tag

- **WHEN** the user is in `/+next` and lists its entries
- **THEN** the listing contains project segments for projects of tasks tagged `next` (for example `home`, `finances`) and tag segments for other tags of those tasks

### Requirement: Project segments navigate hierarchically but list tasks exactly

A project segment SHALL filter to tasks whose project is that segment or any dotted subproject of it, and SHALL present a directory for the next-level subproject of each task below it. A task file SHALL be listed only in the directory whose accumulated project filter equals the task's exact project: ancestor project directories show the subproject directories but not the task file. The bare root lists only project-less tasks (the inbox); tag and virtual-tag views whose accumulated filter has no project segment list every matching task file regardless of project depth.

#### Scenario: Subproject tasks appear only at their leaf

- **WHEN** a pending task exists in project `finances.2026-taxes` and the user lists `/finances`
- **THEN** the listing contains a `2026-taxes` directory and does not contain that task's file

#### Scenario: Exact project directory lists its own tasks

- **WHEN** the user lists `/finances/2026-taxes` while a pending task exists in project `finances.2026-taxes`
- **THEN** the listing contains that task's file

#### Scenario: Tag views list tasks regardless of project depth

- **WHEN** the user lists `/+next` and a task in project `home.clean-garage` is tagged `next`
- **THEN** the listing contains that task's file and a `home` directory

### Requirement: Root lists the filter vocabulary of pending tasks

The root directory SHALL list one entry per first-level project segment, one entry per tag of pending tasks, entries for the virtual tags the system supports, and one file per pending task that has no project.

#### Scenario: Root of the example database

- **WHEN** pending tasks exist in projects `home.clean-garage` and `finances.2026-taxes`, with task `home.clean-garage` tagged `next`
- **THEN** the root listing contains the directories `home`, `finances`, and `+next`

### Requirement: Paths without a status segment mean pending

The system SHALL filter to `status:pending` unless the path contains a status virtual-tag segment such as `+COMPLETED`.

#### Scenario: Default tree excludes completed work

- **WHEN** a task is completed and the user lists its project directory outside `+COMPLETED`
- **THEN** the completed task's file does not appear

### Requirement: Virtual-tag segments expose derived filters

The system SHALL support dot-prefixed `+` segments as virtual-tag filters mapping to Taskwarrior virtual tags — entries named `.+COMPLETED`, `.+DUE`, `.+OVERDUE`, and similar — including `.+COMPLETED` (completed status) and the read-only filters Taskwarrior derives. Virtual-tag entries SHALL be spelled only in this dot-prefixed form: a bare uppercase `+` segment SHALL NOT resolve as a virtual tag. The dot prefix makes virtual-tag entries hidden files, so file managers and `ls` can toggle their visibility.

#### Scenario: Browsing completed tasks

- **WHEN** the user lists `/.+COMPLETED/home`
- **THEN** the listing shows files for completed tasks whose project is `home` or a subproject of it

#### Scenario: Virtual tags are hidden entries

- **WHEN** the user lists `/` with a plain `ls` that hides dot files
- **THEN** the virtual-tag directories (`.+COMPLETED`, `.+DUE`, ...) are absent from the output and present with `ls -a`

#### Scenario: Bare uppercase + segment is not a virtual tag

- **WHEN** the user attempts to enter `/+COMPLETED` with the bare spelling
- **THEN** the lookup fails with ENOENT

#### Scenario: Due-date tightening without filter syntax

- **WHEN** the user lists `/finances/.+DUE`
- **THEN** the listing shows pending tasks in `finances.*` due within Taskwarrior's configured due window

### Requirement: Filter segments can be declared before any task matches them

The system SHALL allow creating a `+name` directory with `mkdir` as a declaration that the tag namespace exists at that location, even when no task matching the current filter carries the tag. The system SHALL likewise allow `mkdir` of a project segment that exists nowhere in the database, so that a task can be moved into a brand-new project. Declarations persist for the life of the mount session. Creating a directory whose name is a virtual-tag segment (`.+NAME`) or any other dot-prefixed name SHALL fail.

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

- **WHEN** the user runs `mkdir .+DUE` or `mkdir .hidden`
- **THEN** the operation fails and no declaration is created

#### Scenario: Removing a declared segment that is in use

- **WHEN** the user runs `rmdir +urgent` in `/home` while tasks in `home.*` carry the tag
- **THEN** the operation fails with ENOTEMPTY and no task data changes

### Requirement: Unknown segments are not directories

The system SHALL fail directory lookup with ENOENT for a segment that is neither a project segment nor a tag of any task matching the current filter.

#### Scenario: Typo in project name

- **WHEN** the user attempts to enter `/homi` while pending tasks exist in project `home.*` but not `homi`
- **THEN** the lookup fails with ENOENT rather than presenting an empty directory
