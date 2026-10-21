# Spec Delta

## MODIFIED Requirements

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
- **THEN** the tasks carrying `urgent` under the current filter are deleted and the declaration is removed, so `/home/+urgent` no longer appears

## ADDED Requirements

### Requirement: Virtual-tag views do not nest

A directory whose accumulated filter already carries a virtual-tag segment SHALL NOT list further virtual-tag entries. Without this bound the tree below a project is combinatorial and a recursive walk of a project directory cannot finish.

#### Scenario: Inside a virtual-tag view no further views are listed

- **WHEN** the user lists `/this/.+PENDING`
- **THEN** the listing contains no `.+`-prefixed entries

### Requirement: Virtual-tag directories are removed only when their view is empty

The system SHALL fail removal of a virtual-tag directory (`.+NAME`) with ENOTEMPTY while any task with a status other than deleted matches the directory's filter, and SHALL succeed without deleting any task once no such task matches. The system SHALL NOT treat removal of a virtual-tag directory as a deletion gesture, because users cannot assign virtual tags to tasks directly.

#### Scenario: Recursive removal of a project completes through the hidden views

- **WHEN** the user runs `rm -rf this` at the root, so each task file inside `this/.+PENDING` and `this/.+COMPLETED` is unlinked before the virtual-tag directories themselves are removed
- **THEN** the virtual-tag directory removals succeed once their views are empty, the project's remaining matching tasks are deleted when `this` itself is removed, and no error is reported

#### Scenario: Virtual-tag directory with matching tasks is not removable

- **WHEN** the user runs `rmdir .+PENDING` in `/this` while pending tasks in `this.*` exist
- **THEN** the operation fails with ENOTEMPTY and no task data changes
