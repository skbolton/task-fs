# Spec Delta

## MODIFIED Requirements

### Requirement: Automount mount flags are configurable

The module SHALL expose `services.task-fs.automount.mountFlags` as a list of strings, defaulting to the empty list, that are appended to the automount's `task-fs mount` command, so the user can pass mount options declaratively — for example `--virtual-tags "COMPLETED,OVERDUE,DUE,ACTIVE,WAITING"`, `--virtual-hidden`, and `--view "Due Next Week=status:pending sched.after:sonw sched.before:eonw"`. Each list item SHALL be one command-line token. The generated `ExecStart` SHALL quote each list item and the mount path so that an item containing spaces reaches `task-fs mount` as one argument.

#### Scenario: Mount flags reach the mount command

- **WHEN** the user sets `services.task-fs.automount.mountFlags = [ "--virtual-tags" "COMPLETED,DUE" "--virtual-hidden" ]` and the automount starts
- **THEN** the mount command carries those flags, so the mounted tree exposes only the `COMPLETED` and `DUE` views as hidden `.+NAME` entries

#### Scenario: Flag value containing spaces stays one argument

- **WHEN** the user sets `services.task-fs.automount.mountFlags = [ "--view" "Due Next Week=status:pending sched.after:sonw sched.before:eonw" ]` and the automount starts
- **THEN** `task-fs mount` receives the entire `--view` value as one argument and the mounted tree exposes the `@Due Next Week` custom view directory

#### Scenario: Unset flags change nothing

- **WHEN** `services.task-fs.automount.mountFlags` is not set
- **THEN** the mount command is `task-fs mount <path>` with no extra flags, and the default virtual-tag selection and presentation apply
