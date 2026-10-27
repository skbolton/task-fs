# Spec Delta

## ADDED Requirements

### Requirement: Custom view filters support command substitutions

A custom view filter SHALL support zero or more `{{ command }}` blocks. Each block SHALL run in a POSIX shell and its standard output, with one trailing newline removed and surrounding whitespace trimmed, SHALL replace the block as literal filter text. A backslash before an opening delimiter SHALL produce a literal `{{`, and blocks SHALL be rendered independently.

#### Scenario: Command output supplies a filter value

- **WHEN** the user mounts with `--view 'Payday=status:pending due.before:{{ payday-date }}'` and `payday-date` writes `2026-11-01`
- **THEN** the custom view's filter is `status:pending due.before:2026-11-01`

#### Scenario: Multiple blocks are rendered in one filter

- **WHEN** a custom view filter contains two command blocks and both commands succeed
- **THEN** each block is replaced by its own command's trimmed output and the rest of the filter is unchanged

#### Scenario: Output containing spaces becomes filter text

- **WHEN** a command block writes `project:home or project:finances`
- **THEN** the view filter contains that complete filter expression rather than one literal argument name

#### Scenario: Escaped delimiter stays literal

- **WHEN** the user defines a filter containing `\{{ not a command }}`
- **THEN** the view filter contains the literal text `{{ not a command }}` and no command is executed for it

### Requirement: Command substitution templates are validated at mount

At mount, every custom view filter SHALL be parsed as a template. A block with an empty command, an unclosed opening delimiter, or a trailing escape SHALL fail the mount with an error naming the custom view, and nothing SHALL be mounted.

#### Scenario: Empty command block fails the mount

- **WHEN** the user runs `task-fs mount ~/m --view 'Broken=due.before:{{ }}'`
- **THEN** the command exits non-zero with an error naming `Broken` and mounts nothing

#### Scenario: Unclosed block fails the mount

- **WHEN** the user runs `task-fs mount ~/m --view 'Broken=due.before:{{ date +%F'`
- **THEN** the command exits non-zero with an error naming `Broken` and mounts nothing

### Requirement: Dynamic view commands use the mount environment

Command blocks SHALL run with the mount process's environment and working directory, so commands see the same Taskwarrior configuration and database selection as backend queries.

#### Scenario: Taskwarrior database selection is inherited

- **WHEN** the mount runs with `TASKDATA=$HOME/other-tasks` and a view command reads `TASKDATA`
- **THEN** the command sees `$HOME/other-tasks`

### Requirement: Dynamic view command failures make the view unreadable

The system SHALL execute command blocks with a bounded timeout. A non-zero exit, timeout, missing shell, empty output, NUL byte, or embedded newline SHALL fail the view operation with EIO and SHALL report the view name, command, and stderr to the mount process's standard error.

#### Scenario: Failed command yields an unreadable view

- **WHEN** a view command exits non-zero during a directory listing
- **THEN** the listing fails with EIO and the mount log names the view and command

#### Scenario: Hung command times out

- **WHEN** a view command does not finish within the execution timeout
- **THEN** the operation fails with EIO rather than blocking indefinitely

#### Scenario: Empty command output is rejected

- **WHEN** a view command succeeds but writes no non-whitespace output
- **THEN** the operation fails with EIO

#### Scenario: Multi-line output is rejected

- **WHEN** a view command writes more than one line of output
- **THEN** the operation fails with EIO instead of splicing ambiguous multi-line text

### Requirement: Dynamic view filters are re-rendered for view access

Before listing a custom view or resolving, removing, or sweeping a task in it, the system SHALL render its command substitutions for that operation. A rendered result MAY be reused only for concurrent or immediately following lookups within one traversal; a later traversal SHALL render again.

#### Scenario: Changed command output appears without remounting

- **WHEN** a view command starts writing a date that brings a new task into the filter and the user lists the view again
- **THEN** the new task's file appears without remounting

#### Scenario: One traversal does not run the command per file

- **WHEN** a directory traversal lists a custom view and then looks up several of its files
- **THEN** the command runs once for that traversal rather than once per file lookup

#### Scenario: A later traversal observes new output

- **WHEN** the command output changes after one listing completes and the user lists the view again
- **THEN** the second listing uses the newly rendered filter

## MODIFIED Requirements

### Requirement: Custom view filters are validated at mount

The mount command SHALL render each custom view filter and run the resulting filter against the backend before mounting. A command that fails, produces unacceptable output, or yields a filter the backend rejects as malformed SHALL fail the mount with an error naming the offending custom view, and nothing SHALL be mounted. A valid filter that matches zero tasks SHALL be accepted.

#### Scenario: Malformed filter fails the mount

- **WHEN** the user runs `task-fs mount ~/m --view "Broken=( status:pending"`
- **THEN** the command exits non-zero with an error naming `Broken` and mounts nothing

#### Scenario: Zero-match filter is valid

- **WHEN** the user runs `task-fs mount ~/m --view "Nothing=project:does-not-exist"` on a database with no such project
- **THEN** the mount succeeds and the root exposes an empty `@Nothing` custom view directory

#### Scenario: Dynamic filter is validated after rendering

- **WHEN** the user mounts with a dynamic view whose command succeeds and whose rendered filter is valid
- **THEN** the mount succeeds even if the rendered filter matches zero tasks

#### Scenario: Dynamic command failure fails the mount

- **WHEN** a custom view command exits non-zero during mount validation
- **THEN** the command exits non-zero with an error naming the custom view and mounts nothing

#### Scenario: Malformed rendered filter fails the mount

- **WHEN** a command succeeds but writes output that makes the rendered filter malformed
- **THEN** the command exits non-zero with an error naming the custom view and mounts nothing
