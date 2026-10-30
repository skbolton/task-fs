# Proposal

## Why

The Home Manager module can autostart only one TaskFS mount, and that mount always inherits the systemd user manager's Taskwarrior database. Users who keep several Taskwarrior databases must hand-write systemd units, and the module offers no declarative way to point a mount at a specific `TASKDATA` directory.

## What Changes

- Replace the single `services.task-fs.automount` submodule with `services.task-fs.mounts.<name>`, an attribute set of named mount submodules. The attribute key names the mount and its systemd unit.
- Generate one systemd user service per mount (`task-fs-<name>.service`), each with its own mountpoint, mount flags, package PATH, and lifecycle.
- Add an optional `services.task-fs.mounts.<name>.taskData` path. When set, the service runs with `TASKDATA` pointing at that directory; when unset, the mount continues to inherit the session environment.
- Expand a leading `~` in `taskData` against the user's home directory, matching how `path` is already resolved.
- Validate mount names for safe use in systemd unit names and require a non-empty mount path per entry.
- Update the module checks and README with multi-mount and per-mount database examples and a migration note.
- **BREAKING** `services.task-fs.automount.{enable,path,mountFlags,extraPackages}` are removed. Users move each mount into `services.task-fs.mounts.<name>` and keep using `services.task-fs.enable` as the global switch.

Out of scope: Go/CLI changes, new Taskwarrior database flags, per-mount view stores, and non-Linux support.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `home-manager-module`: Automount configuration becomes a named collection of mounts, each with its own path, flags, PATH extras, and optional `TASKDATA` directory.

## Impact

- `nix/home-manager.nix`: replace the `automount` submodule with a named `mounts` attribute set, generate per-mount units, and inject `TASKDATA` when selected.
- `nix/hm-module-test.nix`: evaluate multiple mounts and assert per-unit paths, flags, PATH, database environment, naming, and assertions.
- `README.md`: document the new option tree, per-mount database selection, and the breaking migration from `automount`.
- No Go code, CLI behavior, task data, or view-store changes. The CLI already inherits `TASKDATA` from the mount process environment.
