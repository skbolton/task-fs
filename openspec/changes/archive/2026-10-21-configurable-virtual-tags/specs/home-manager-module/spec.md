# Spec Delta: home-manager-module

## ADDED Requirements

### Requirement: Automount mount flags are configurable

The module SHALL expose `services.task-fs.automount.mountFlags` as a list of strings, defaulting to the empty list, that are appended to the automount's `task-fs mount` command, so the user can pass mount options declaratively — for example `--virtual-tags "COMPLETED,OVERDUE,DUE,ACTIVE,WAITING"` and `--virtual-hidden`. Each list item SHALL be one command-line token.

#### Scenario: Mount flags reach the mount command

- **WHEN** the user sets `services.task-fs.automount.mountFlags = [ "--virtual-tags" "COMPLETED,DUE" "--virtual-hidden" ]` and the automount starts
- **THEN** the mount command carries those flags, so the mounted tree exposes only the `COMPLETED` and `DUE` views as hidden `.+NAME` entries

#### Scenario: Unset flags change nothing

- **WHEN** `services.task-fs.automount.mountFlags` is not set
- **THEN** the mount command is `task-fs mount <path>` with no extra flags, and the default virtual-tag selection and presentation apply
