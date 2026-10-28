# task-navigation Specification

## Purpose

Owns the directory tree of the mounted filesystem and its meaning as a task filter: the path grammar, the segment kinds, order-free path parsing, and which entries are visible at each level.

## Requirements

### Requirement: Path segments are an order-free filter

The system SHALL interpret a filesystem path as the conjunction of its segments' filters, where a `+COMPLETED` or `+DELETED` segment is a status virtual-tag filter, any other segment starting with `+` is a tag filter, and every other segment is a project filter. At the root, a segment matching a configured custom view name SHALL select that custom view directory instead of a project filter; custom view directories do not accumulate filters. The order of segments SHALL NOT change the resulting filter.

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

The root directory SHALL list one entry per first-level project segment, one entry per tag of pending tasks, the `+COMPLETED` and `+DELETED` virtual-tag views, and one file per pending task that has no project.

#### Scenario: Root of the example database

- **WHEN** pending tasks exist in projects `home.clean-garage` and `finances.2026-taxes`, with task `home.clean-garage` tagged `next`
- **THEN** the root listing contains the directories `home`, `finances`, `+next`, `+COMPLETED`, and `+DELETED`

### Requirement: Paths without a status segment mean pending

The system SHALL filter a filter directory to `status:pending` unless the path contains a status virtual-tag segment such as `+COMPLETED`. A custom view directory SHALL use its defined filter exactly as written and SHALL NOT gain an implicit status criterion.

#### Scenario: Default tree excludes completed work

- **WHEN** a task is completed and the user lists its project directory outside `+COMPLETED`
- **THEN** the completed task's file does not appear

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

### Requirement: Unknown segments are not directories

The system SHALL fail directory lookup with ENOENT for a segment that is neither a project segment, a tag of any task matching the current filter, nor a configured custom view name at the root.

#### Scenario: Typo in project name

- **WHEN** the user attempts to enter `/homi` while pending tasks exist in project `home.*` but not `homi`
- **THEN** the lookup fails with ENOENT rather than presenting an empty directory

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

### Requirement: Custom view contents follow the rendered filter

A custom view's listing and task-file lookups SHALL use the filter rendered for the current access. A task that starts or stops matching after a command's output changes SHALL appear or disappear on the next view access without remounting. Lookups SHALL resolve only minted task-file names in that current result.

#### Scenario: Newly matching task appears on the next listing

- **WHEN** a custom view filter contains a date-producing command and a command result change makes an existing task match
- **THEN** listing the custom view after the change includes that task's file without remounting

#### Scenario: No-longer-matching task disappears on the next listing

- **WHEN** a command result change makes a previously listed task stop matching the custom view
- **THEN** listing the custom view after the change omits that task's file

#### Scenario: Lookup uses the current result

- **WHEN** the user looks up a task-file name that was listed before a command result change but is not in the currently rendered result
- **THEN** the lookup fails with ENOENT

#### Scenario: Static custom views are unaffected

- **WHEN** a custom view contains no command substitutions
- **THEN** its listing and lookup behavior are identical to the existing fixed-filter custom view behavior

### Requirement: Custom view names take precedence at the root

At the root, lookup SHALL resolve a configured custom view name before a project segment of the same name. A root project whose first segment matches a configured custom view name SHALL be shadowed while that custom view is configured.

#### Scenario: Configured custom view wins over a same-named project

- **WHEN** a task has project `@Due` and the user mounts with a custom view named `Due`, then looks up `@Due` at the root
- **THEN** the lookup resolves the custom view directory rather than the project

#### Scenario: At-prefixed project resolves when no custom view uses the name

- **WHEN** a task has project `@Due` and the user mounts without a custom view named `Due`, then looks up `@Due` at the root
- **THEN** the lookup resolves the project segment
