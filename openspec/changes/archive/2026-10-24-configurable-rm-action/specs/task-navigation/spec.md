# Spec Delta

## MODIFIED Requirements

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
- **THEN** those tasks are completed under `complete` or deleted under `delete` according to the configured removal action, and the declaration is removed, so `/home/+urgent` no longer appears

## ADDED Requirements

### Requirement: Virtual-tag directory removal follows the configured action

Removing a virtual-tag directory SHALL NOT change any task. Under `delete` it SHALL fail with ENOTEMPTY while any task with a status other than deleted matches the view, and succeed once none does. Under `complete` it SHALL fail only while the view holds a task that is neither completed nor deleted, and otherwise succeed, so recursive removal converges after completion moves tasks into the completed view.

#### Scenario: Delete action keeps the view-emptiness guard

- **WHEN** the user mounts with `--rm delete` and runs `rmdir .+PENDING` in `/this` while pending tasks in `this.*` exist
- **THEN** the operation fails with ENOTEMPTY and no task data changes

#### Scenario: Delete action completes a recursive project removal

- **WHEN** the user mounts with `--rm delete` and runs `rm -rf this` at the root, so each task file inside the project's virtual-tag views is unlinked before those directories are removed
- **THEN** the view removals succeed once their tasks are deleted, the project's remaining matching tasks are deleted when `this` is removed, and no error is reported

#### Scenario: Complete action converges with pre-existing completed tasks

- **WHEN** the user mounts with `--rm complete` and runs `rm -rf this` while project `this` has pending tasks and already-completed tasks
- **THEN** the pending tasks become completed, the completed tasks are unchanged, every virtual-tag directory removal succeeds, and no error is reported

#### Scenario: Complete action still guards an actionable view

- **WHEN** the user mounts with `--rm complete` and runs `rmdir .+PENDING` in `/this` while pending tasks in `this.*` exist
- **THEN** the operation fails with ENOTEMPTY and no task data changes

#### Scenario: Complete action allows removing the completed view

- **WHEN** the user mounts with `--rm complete` and runs `rmdir .+COMPLETED` while completed tasks match the view
- **THEN** the operation succeeds without changing any task

## REMOVED Requirements

### Requirement: Virtual-tag directories are removed only when their view is empty

**Reason**: The emptiness guard now depends on the configured removal action; under `complete`, completed tasks do not make the completed view block a recursive removal.

**Migration**: Mount with `--rm delete` to keep the previous emptiness behavior.
