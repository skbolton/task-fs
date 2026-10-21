# Proposal

## Why

TaskFS is already packaged by its flake, but users who run Home Manager must hand-write the mount invocation, ensure the mountpoint exists, and wire it into their session startup themselves. A first-class Home Manager module lets users pull the project in as a flake input and declare the mount declaratively, matching how everything else in their config is managed.

## What Changes

- Add a `nix/` folder to the repository containing a Home Manager module.
- Expose the module on the flake as `homeModules.task-fs`, importable from a user config via `inputs.task-fs.homeModules.task-fs`.
- Provide `services.task-fs.enable` to install the `task-fs` package.
- Provide `services.task-fs.automount.enable` and `services.task-fs.automount.path` to automatically mount the Taskwarrior filesystem at the given path when the user session starts, creating the directory first if needed.
- Provide `services.task-fs.package` (default: the package built by this flake) so users can substitute their own build or pin.
- Linux only: the module asserts the host system is Linux (FUSE requirement).
- No changes to the Go code, CLI behavior, or existing specs.

## Capabilities

### New Capabilities

- `home-manager-module`: The Home Manager integration surface — the flake's `homeModules.task-fs` output, the `services.task-fs` option tree, package installation, and the automount behavior (directory creation, session-scoped mount, clean unmount).

### Modified Capabilities

None.

## Impact

- New `nix/` directory with the module; `flake.nix` gains a flake-parts module import that wires `homeModules.task-fs` into the flake outputs.
- Depends on the existing flake package output (`packages.default`) as the mounted binary.
- Consumers are Home Manager users on Linux with FUSE available; no impact on the filesystem implementation or its specs.
