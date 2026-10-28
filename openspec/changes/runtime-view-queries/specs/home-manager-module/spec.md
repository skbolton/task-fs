# Spec Delta

## ADDED Requirements

### Requirement: The automount shares the user's runtime view store

When the automount starts, runtime view definitions SHALL be read from and written to the same state location the user session resolves: `$XDG_STATE_HOME/task-fs/views` when `XDG_STATE_HOME` is an absolute path, otherwise `$HOME/.local/state/task-fs/views`. Views created through the mounted filesystem SHALL persist after the automount stops and SHALL reappear on the next start.

#### Scenario: View survives an automount restart

- **WHEN** the user creates a runtime view while the automount is running, stops the automount, and starts it again
- **THEN** the mounted root still lists the runtime view with the same filter

#### Scenario: Service HOME selects the store

- **WHEN** the automount service runs with `HOME=/home/me` and no `XDG_STATE_HOME`
- **THEN** runtime views are stored under `/home/me/.local/state/task-fs/views`

## MODIFIED Requirements

### Requirement: Automount mount flags are configurable

The module SHALL expose `services.task-fs.automount.mountFlags` as a list of strings, defaulting to the empty list, that are appended to the automount's `task-fs mount` command, so the user can pass mount options declaratively — for example `--rm delete`. Each list item SHALL be one command-line token. The generated `ExecStart` SHALL quote each list item and the mount path so that an item containing spaces reaches `task-fs mount` as one argument. Mount flags SHALL NOT define custom views; runtime views are created through the mounted tree.

#### Scenario: Mount flags reach the mount command

- **WHEN** the user sets `services.task-fs.automount.mountFlags = [ "--rm" "delete" ]` and the automount starts
- **THEN** the mount command carries that flag and removal gestures delete tasks

#### Scenario: Flag value containing spaces stays one argument

- **WHEN** a mount flag value contains spaces and the automount starts
- **THEN** `task-fs mount` receives the entire value as one argument

#### Scenario: Unset flags change nothing

- **WHEN** `services.task-fs.automount.mountFlags` is not set
- **THEN** the mount command is `task-fs mount <path>` with no extra flags, and the mounted tree exposes the fixed `+COMPLETED` and `+DELETED` virtual-tag views

#### Scenario: Removed view flag fails the automount

- **WHEN** `mountFlags` contains `--view`
- **THEN** the automount's mount command exits non-zero with a usage message and nothing is mounted

### Requirement: Automount provides the dynamic view command runtime

The automount service SHALL make a POSIX shell and coreutils commands available to `{{ ... }}` blocks in a runtime view's `_filter` file. Commands outside that baseline SHALL be referenced by absolute path or through an executable script.

#### Scenario: Coreutils command resolves in the service

- **WHEN** a Home Manager user writes `status:pending due.before:{{ date -d "+7 days" +%F }}` into a runtime view's `_filter`
- **THEN** the automount service can run `date` and the view renders successfully

#### Scenario: External command script uses an absolute path

- **WHEN** a `_filter` block invokes a script outside the coreutils baseline
- **THEN** the user can reference the script by absolute Nix store path or through an executable wrapper and the block runs in the service environment

#### Scenario: Template value stays one argument

- **WHEN** the user writes a `_filter` containing spaces and `{{ ... }}` blocks through the mounted filesystem
- **THEN** the view uses the complete filter text with the braces unchanged
