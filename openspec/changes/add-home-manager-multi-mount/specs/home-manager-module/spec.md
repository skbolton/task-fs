# Spec Delta

## ADDED Requirements

### Requirement: Mounts are declared by configuration key

The module SHALL expose `services.task-fs.mounts` as an attribute set of mount submodules, where each key names one mount. Declaring an entry SHALL create and start that mount's service; removing it SHALL remove the service. `services.task-fs.enable` SHALL remain the global switch that installs the package and enables all declared mounts.

#### Scenario: Several mounts run at once

- **WHEN** the user sets `services.task-fs.mounts.work.path = "~/work-tasks"` and `services.task-fs.mounts.personal.path = "~/personal-tasks"`
- **THEN** both mounts start at session start and each lists tasks at its own path

#### Scenario: Removing a mount removes its service

- **WHEN** the user deletes a `services.task-fs.mounts.<name>` entry and rebuilds
- **THEN** that mount's unit is absent and its mountpoint is no longer mounted

#### Scenario: Declaring mounts while disabled starts nothing

- **WHEN** `services.task-fs.enable` is false and mounts are declared
- **THEN** the module installs no package and starts no mount service

### Requirement: A mount can select its Taskwarrior database

A mount submodule SHALL accept an optional `taskData` path. When set, the mount service SHALL run with `TASKDATA` set to that directory. When unset, the service SHALL inherit the systemd user manager environment.

#### Scenario: Mounts use different databases

- **WHEN** `mounts.work.taskData = "/home/me/work-tasks"` and `mounts.personal.taskData = "/home/me/personal-tasks"`
- **THEN** each mount lists and mutates tasks in its own database

#### Scenario: Unset taskData inherits the session database

- **WHEN** a mount sets no `taskData` and the user manager has `TASKDATA=/home/me/other-tasks`
- **THEN** that mount operates on `/home/me/other-tasks`

### Requirement: Mount names produce valid systemd units

Each mount key SHALL name its systemd user unit as `task-fs-<name>.service`. The module SHALL reject an empty key or one containing characters outside letters, digits, hyphens, and underscores.

#### Scenario: Mount key names the unit

- **WHEN** the user declares `services.task-fs.mounts.work`
- **THEN** the generated unit is `systemd.user.services.task-fs-work`

#### Scenario: Invalid mount name fails evaluation

- **WHEN** the user declares a mount key containing `/`, whitespace, or another unsupported character
- **THEN** evaluation fails with an error naming the allowed characters

## MODIFIED Requirements

### Requirement: The flake exposes the Home Manager module

The flake SHALL expose the Home Manager module as `homeModules.task-fs`, so a user importing `inputs.task-fs.homeModules.task-fs` gains the `services.task-fs` option tree in their configuration.

#### Scenario: Importing the module from a flake input

- **WHEN** a user imports `inputs.task-fs.homeModules.task-fs` into their Home Manager configuration
- **THEN** `services.task-fs.enable` and `services.task-fs.mounts.*` evaluate without an unknown-option error

### Requirement: Enabling the service installs task-fs

The system SHALL, when `services.task-fs.enable` is true, make the `task-fs` binary from `services.task-fs.package` available to the user environment — defaulting to the package built by this flake — and SHALL NOT when it is false.

#### Scenario: Enable installs the binary

- **WHEN** the user sets `services.task-fs.enable = true` and rebuilds
- **THEN** `task-fs` resolves on `$PATH` in the user session

#### Scenario: Disabled installs nothing

- **WHEN** `services.task-fs.enable` is false
- **THEN** the module adds no `task-fs` binary and starts no services

#### Scenario: Package override propagates

- **WHEN** the user sets `services.task-fs.package` to a custom build and enables a mount
- **THEN** both the installed binary and every mount unit use that package

### Requirement: Automount mounts the filesystem at the configured path

When `services.task-fs.mounts.<name>.path` is set, the system SHALL create that path's directory if it does not exist and SHALL mount the Taskwarrior filesystem there when the user session starts, without manual commands. Each mount SHALL operate independently at its own path.

#### Scenario: Automount at an existing path

- **WHEN** the user session starts with a mount whose `path` points at an existing directory
- **THEN** the Taskwarrior filesystem is mounted at that directory and its root lists task entries

#### Scenario: Automount creates a missing directory

- **WHEN** a mount's configured path does not exist when the mount starts
- **THEN** the directory is created and the filesystem is mounted there

#### Scenario: Independent mounts do not share a path

- **WHEN** two mounts declare different paths
- **THEN** each path holds its own mounted filesystem and neither mount writes through the other

### Requirement: Automount unmounts cleanly on session end

When a mount service stops — because the session ends, that unit is stopped, or the user removes the mount entry and rebuilds — the system SHALL unmount the filesystem cleanly at that mount's path rather than leaving a stale mount. Stopping one mount SHALL NOT unmount another mount.

#### Scenario: Stopping the automount unmounts

- **WHEN** one mount unit is stopped
- **THEN** the filesystem is unmounted from that mount's path and the mountpoint is free for reuse

#### Scenario: Restarting the automount

- **WHEN** a mount unit is restarted after being stopped
- **THEN** the filesystem mounts again at the same path without manual intervention

#### Scenario: Session end unmounts every mount

- **WHEN** the user session ends
- **THEN** every declared mount unmounts from its own path

### Requirement: Path tilde and environment variables are resolved as the user

The module SHALL interpret `services.task-fs.mounts.<name>.path` and `services.task-fs.mounts.<name>.taskData` relative to the user's home directory, expanding a leading `~` so that values like `~/some/path` name entries under the user's home.

#### Scenario: Tilde path resolves under the user's home

- **WHEN** the user sets `services.task-fs.mounts.work.path = "~/some/path"`
- **THEN** the filesystem mounts at `$HOME/some/path`

#### Scenario: Tilde taskData resolves under the user's home

- **WHEN** the user sets `services.task-fs.mounts.work.taskData = "~/task-data/work"`
- **THEN** the mount service runs with `TASKDATA=$HOME/task-data/work`

### Requirement: Automount mount flags are configurable

The module SHALL expose `services.task-fs.mounts.<name>.mountFlags` as a list of strings, defaulting to the empty list, that are appended to that mount's `task-fs mount` command, so the user can pass mount options declaratively — for example `--rm delete`. Each list item SHALL be one command-line token. The generated `ExecStart` SHALL quote each list item and the mount path so that an item containing spaces reaches `task-fs mount` as one argument. Flags SHALL apply only to the mount whose submodule declares them.

#### Scenario: Mount flags reach the mount command

- **WHEN** one mount sets `mountFlags = [ "--rm" "delete" ]` and another sets none
- **THEN** only the first mount's command carries those flags

#### Scenario: Flag value containing spaces stays one argument

- **WHEN** a mount flag value contains spaces and the mount starts
- **THEN** `task-fs mount` receives the entire value as one argument

#### Scenario: Unset flags change nothing

- **WHEN** a mount's `mountFlags` is not set
- **THEN** its mount command is `task-fs mount <path>` with no extra flags

### Requirement: Automount provides the dynamic view command runtime

Each mount service SHALL make a POSIX shell and coreutils commands available to `{{ ... }}` blocks in runtime view definitions. A mount's `extraPackages` SHALL be appended to that service's `PATH`. Commands outside that baseline SHALL be referenced by absolute path or through an executable script, and the mount process SHALL read view definition text unchanged.

#### Scenario: Coreutils command resolves in the service

- **WHEN** a Home Manager user writes a `_view` definition containing `{{ date -d "+7 days" +%F }}`
- **THEN** the mount service can run `date` and the view renders successfully

#### Scenario: Extra package applies only to its mount

- **WHEN** one mount sets `extraPackages = [ pkgs.delta.cycle ]` and another sets none
- **THEN** `cycle` resolves by name only in the service that declares it

#### Scenario: External command script uses an absolute path

- **WHEN** a `{{ ... }}` block invokes a script outside the coreutils baseline
- **THEN** the user can reference the script by absolute Nix store path or through an executable wrapper and the block runs in the service environment

#### Scenario: Template value stays one argument

- **WHEN** a view definition contains spaces and `{{ ... }}` blocks
- **THEN** the mount process receives the complete definition text with the braces unchanged
