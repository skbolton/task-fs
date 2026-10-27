# Spec Delta

## MODIFIED Requirements

### Requirement: Path segments are an order-free filter

The system SHALL interpret a filesystem path as the conjunction of its segments' filters, where each segment starting with `+` is a tag filter, each segment starting with `.+` whose remainder is fully uppercase is a virtual-tag filter, and every other segment is a project filter. At the root, a segment matching a configured custom view name SHALL select that custom view directory instead of a project filter; custom view directories do not accumulate filters. The order of segments SHALL NOT change the resulting filter.

#### Scenario: Equivalent paths in any order

- **WHEN** the user lists `/+next/home` and `/home/+next` on a database with a task in project `home.clean-garage` tagged `next`
- **THEN** both listings show the same set of task files

#### Scenario: Deepening under a tag

- **WHEN** the user is in `/+next` and lists its entries
- **THEN** the listing contains project segments for projects of tasks tagged `next` (for example `home`, `finances`) and tag segments for other tags of those tasks

### Requirement: Paths without a status segment mean pending

The system SHALL filter a filter directory to `status:pending` unless the path contains a status virtual-tag segment such as `+COMPLETED`. A custom view directory SHALL use its defined filter exactly as written and SHALL NOT gain an implicit status criterion.

#### Scenario: Default tree excludes completed work

- **WHEN** a task is completed and the user lists its project directory outside `+COMPLETED`
- **THEN** the completed task's file does not appear

### Requirement: Filter segments can be declared before any task matches them

The system SHALL allow creating a `+name` directory with `mkdir` in a filter directory as a declaration that the tag namespace exists at that location, even when no task matching the current filter carries the tag. The system SHALL likewise allow `mkdir` of a project segment that exists nowhere in the database, so that a task can be moved into a brand-new project. Declarations persist for the life of the mount session. Creating a directory in a custom view directory SHALL fail. Creating a directory whose name is a virtual-tag segment in either spelling (`+NAME` or `.+NAME`) or any other dot-prefixed name SHALL fail.

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
- **THEN** those tasks are completed under `complete` or deleted under `delete` according to the configured removal action, and the declaration is removed, so `/home/+urgent` no longer appears

### Requirement: Unknown segments are not directories

The system SHALL fail directory lookup with ENOENT for a segment that is neither a project segment, a tag of any task matching the current filter, nor a configured custom view name at the root.

#### Scenario: Typo in project name

- **WHEN** the user attempts to enter `/homi` while pending tasks exist in project `home.*` but not `homi`
- **THEN** the lookup fails with ENOENT rather than presenting an empty directory

## ADDED Requirements

### Requirement: Root lists configured custom views

The root directory SHALL list one visible `@Name` directory for each configured custom view, including custom views that currently match no tasks. Custom view directories SHALL be listed only at the root; project, tag, virtual-tag, and custom view directories SHALL NOT list them.

#### Scenario: Custom view directories appear at the root

- **WHEN** the user mounts with `--view "Due Next Week=status:pending sched.after:sonw sched.before:eonw"` and lists the root
- **THEN** the listing contains the directory `@Due Next Week`

#### Scenario: Empty custom view directory remains visible

- **WHEN** a configured custom view filter matches no task and the user lists the root
- **THEN** its `@Name` directory is still present, and listing it shows no task files

#### Scenario: Custom view directories are not listed below the root

- **WHEN** the user lists a project directory, a tag directory, or a virtual-tag directory
- **THEN** the listing does not contain any configured `@Name` custom view directory

### Requirement: Custom view directories are lookup-only

A custom view directory SHALL list exactly one file per task matching its filter, across all projects, using the standard task file naming. It SHALL NOT list project, tag, virtual-tag, or custom view subdirectories. Lookup SHALL resolve only minted task-file names present in the custom view result; every other name SHALL fail with ENOENT.

#### Scenario: Custom view lists tasks across projects

- **WHEN** the custom view filter matches a task in project `home` and a task with no project, and the user lists the custom view directory
- **THEN** both task files appear directly in that directory

#### Scenario: Custom view directories do not expose filter vocabulary

- **WHEN** the user lists a custom view directory whose results include tasks from project `home` and tagged `next`
- **THEN** the listing contains no `home` directory, no `+next` directory, and no virtual-tag directory

#### Scenario: Only custom view results resolve

- **WHEN** the user attempts to enter a custom view directory followed by `home` or `+next`
- **THEN** the lookup fails with ENOENT

#### Scenario: Same task appears in its custom view and filter view

- **WHEN** a task matches a configured custom view and also resides in `/home`
- **THEN** its file is listed in both the custom view directory and `/home`

### Requirement: Custom view names take precedence at the root

At the root, lookup SHALL resolve a configured custom view name before a project segment of the same name. A root project whose first segment matches a configured custom view name SHALL be shadowed while that custom view is configured.

#### Scenario: Configured custom view wins over a same-named project

- **WHEN** a task has project `@Due` and the user mounts with a custom view named `Due`, then looks up `@Due` at the root
- **THEN** the lookup resolves the custom view directory rather than the project

#### Scenario: At-prefixed project resolves when no custom view uses the name

- **WHEN** a task has project `@Due` and the user mounts without a custom view named `Due`, then looks up `@Due` at the root
- **THEN** the lookup resolves the project segment
