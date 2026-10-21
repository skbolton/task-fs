# home-manager-module Specification

## Purpose

Lets Home Manager users install TaskFS and declaratively automount their Taskwarrior filesystem in their session by importing the module from this flake, without hand-writing mount commands or startup wiring.

## Requirements

### Requirement: The flake exposes the Home Manager module

The flake SHALL expose the Home Manager module as `homeModules.task-fs`, so a user importing `inputs.task-fs.homeModules.task-fs` gains the `services.task-fs` option tree in their configuration.

#### Scenario: Importing the module from a flake input

- **WHEN** a user imports `inputs.task-fs.homeModules.task-fs` into their Home Manager configuration
- **THEN** `services.task-fs.enable` and `services.task-fs.automount.*` evaluate without an unknown-option error

### Requirement: Enabling the service installs task-fs

The system SHALL, when `services.task-fs.enable` is true, make the `task-fs` binary from `services.task-fs.package` available to the user environment — defaulting to the package built by this flake — and SHALL NOT when it is false.

#### Scenario: Enable installs the binary

- **WHEN** the user sets `services.task-fs.enable = true` and rebuilds
- **THEN** `task-fs` resolves on `$PATH` in the user session

#### Scenario: Disabled installs nothing

- **WHEN** `services.task-fs.enable` is false
- **THEN** the module adds no `task-fs` binary and starts no services

#### Scenario: Package override propagates

- **WHEN** the user sets `services.task-fs.package` to a custom build and enables the automount
- **THEN** both the installed binary and the automount unit use that package

### Requirement: Automount mounts the filesystem at the configured path

When `services.task-fs.automount.enable` is true and `services.task-fs.automount.path` is set, the system SHALL create the path's directory if it does not exist and SHALL mount the Taskwarrior filesystem there when the user session starts, without manual commands.

#### Scenario: Automount at an existing path

- **WHEN** the user session starts with `automount.enable = true` and `automount.path` pointing at an existing directory
- **THEN** the Taskwarrior filesystem is mounted at that directory and its root lists task entries

#### Scenario: Automount creates a missing directory

- **WHEN** the configured path does not exist when the mount starts
- **THEN** the directory is created and the filesystem is mounted there

### Requirement: Automount unmounts cleanly on session end

When the automount stops — because the session ends, the unit is stopped, or the user disables the automount and rebuilds — the system SHALL unmount the filesystem cleanly at the configured path rather than leaving a stale mount.

#### Scenario: Stopping the automount unmounts

- **WHEN** the automount unit is stopped
- **THEN** the filesystem is unmounted from the configured path and the mountpoint is free for reuse

#### Scenario: Restarting the automount

- **WHEN** the automount unit is restarted after being stopped
- **THEN** the filesystem mounts again at the configured path without manual intervention

### Requirement: The module is Linux only

The module SHALL refuse evaluation on non-Linux hosts with an error naming the platform limitation, since the filesystem requires Linux FUSE.

#### Scenario: Evaluation on non-Linux fails clearly

- **WHEN** a user on a non-Linux host evaluates a configuration importing the module
- **THEN** evaluation fails with an error stating TaskFS supports Linux only

### Requirement: Path tilde and environment variables are resolved as the user

The module SHALL interpret `services.task-fs.automount.path` relative to the user's home directory, expanding a leading `~` so that values like `~/some/path` name a directory under the user's home.

#### Scenario: Tilde path resolves under the user's home

- **WHEN** the user sets `services.task-fs.automount.path = "~/some/path"`
- **THEN** the filesystem mounts at `$HOME/some/path`

### Requirement: Automount mount flags are configurable

The module SHALL expose `services.task-fs.automount.mountFlags` as a list of strings, defaulting to the empty list, that are appended to the automount's `task-fs mount` command, so the user can pass mount options declaratively — for example `--virtual-tags "COMPLETED,OVERDUE,DUE,ACTIVE,WAITING"` and `--virtual-hidden`. Each list item SHALL be one command-line token.

#### Scenario: Mount flags reach the mount command

- **WHEN** the user sets `services.task-fs.automount.mountFlags = [ "--virtual-tags" "COMPLETED,DUE" "--virtual-hidden" ]` and the automount starts
- **THEN** the mount command carries those flags, so the mounted tree exposes only the `COMPLETED` and `DUE` views as hidden `.+NAME` entries

#### Scenario: Unset flags change nothing

- **WHEN** `services.task-fs.automount.mountFlags` is not set
- **THEN** the mount command is `task-fs mount <path>` with no extra flags, and the default virtual-tag selection and presentation apply
