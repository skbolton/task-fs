# Design

## Context

The flake (flake-parts) already builds the `task-fs` binary as `packages.default`. Home Manager modules are plain module-system files; the module itself needs no home-manager flake input. The mount command (`task-fs mount <path>`) runs in the foreground, requires a pre-existing mountpoint directory, delegates all backend access to a `task` binary on `$PATH`, and unmounts on interrupt or `fusermount3 -u`. Linux FUSE is a hard requirement.

## Goals / Non-Goals

**Goals:**

- `inputs.task-fs.homeModules.task-fs` importable from any Home Manager configuration with no extra wiring.
- `services.task-fs.enable` installs the binary; `services.task-fs.automount.*` manages the mount over the user session lifecycle.
- Module evaluates and is verifiable without a full Home Manager installation.

**Non-Goals:**

- NixOS system-level module or NixOS `fileSystems` integration (the mount is user-scoped; FUSE user mounts do not need root).
- macOS/darwin support.
- Passthrough options for alternate Taskwarrior databases (e.g. `TASKDATA`) — the mount inherits the systemd user manager environment, which users can extend themselves.

## Decisions

### Module location and flake wiring

The module lives at `nix/home-manager.nix` and is wired into the flake outputs as `flake.homeModules.task-fs = import ./nix/home-manager.nix;` inside the flake-parts `flake` block.

*Alternative considered*: a per-system `flake.homeModules` under `perSystem` — rejected because flake outputs like `homeModules` are system-independent; module files are functions of `config`/`pkgs`, not of the host system.

### Mount mechanism: systemd user service

Automount is implemented as `systemd.user.services.task-fs`:

- `ExecStartPre` creates the mountpoint with `mkdir -p` (satisfies the "directory must already exist" requirement of the mount command).
- `ExecStart` runs `task-fs mount <path>` in the foreground.
- `ExecStopPost` runs `fusermount3 -u <path>` so stopping the unit unmounts cleanly.
- `WantedBy = default.target`, `Restart = on-failure` (transient FUSE failures self-heal; a clean stop stays stopped).
- The unit's environment gets `fusermount3` and `taskwarrior3` on `PATH` so the mount works out of the box, while the `task-fs` binary comes from the configured `package` option.

*Alternatives considered*:
- A login-shell hook or `home.sessionActivation` — rejected: no clean unmount path on session end and no restart supervision.
- A NixOS `fileSystems` entry — rejected: requires root, couples a user's task data to system evaluation, and breaks the Home Manager-only usage the request asks for.

### Path handling

`automount.path` is a string option. A leading `~` or `~/` is expanded at evaluation time against `config.home.homeDirectory`; absolute paths pass through unchanged. Nix-side expansion (rather than a shell `~` expansion in the unit) keeps the unit file self-contained and avoids depending on `$HOME` being set in the service environment.

### Package options

- `services.task-fs.package` defaults to the flake's own package (`inputs.self.packages.<system>.default` passed into the module via `flake-config`/`perSystem` wiring, or as a module arg). Override lets users pin or patch the binary.
- `services.task-fs.automount` is a submodule with `enable` (bool, default false) and `path` (string, no default; asserting it is set when `automount.enable` is true avoids mounting at an empty path).

### Linux-only enforcement

An `assertions` entry requiring `pkgs.stdenv.isLinux`, with a message naming the platform limitation. Home Manager surfaces assertion failures as clear evaluation errors.

### Verification without Home Manager installed

The module is kept free of home-manager-specific library calls beyond standard option types, so a flake check can evaluate it with `lib.evalModules`, supplying stub `home.homeDirectory` and HM service-module base options. The check asserts the generated unit contains the expected mount path, `fusermount3 -u`, and the package binary, and that evaluation on a non-Linux platform fails.

*Alternative considered*: requiring home-manager as a flake input for an integration test — heavier and unnecessary for evaluation-level checks; can be added later if behavior drift becomes a concern.

## Risks / Trade-offs

- [The `task` binary pinned in the unit may differ from the user's CLI taskwarrior] → The unit gets `taskwarrior3` from nixpkgs only so the mount works by default; users needing a specific build or `TASKDATA` override the unit's environment (`systemd.user.services.task-fs.Environment` or `sessionEnvironment`).
- [Systemd user services do not inherit the interactive shell's environment] → Documented in the README section added with this change; the module exposes only the minimum options, so the escape hatch is standard systemd user configuration.
- [`fusermount3 -u` on an already-unmounted path fails, making `ExecStopPost` report failure] → Prefix with `-` in the unit so a failed unmount does not fail the stop job.
- [Mount lingers if the user session manager dies without running ExecStopPost] → The next `mount` attempt on a stale FUSE mountpoint fails visibly; `fusermount3 -u` by hand recovers. Accepted for v1.

## Migration Plan

Purely additive: new `nix/` directory and a new flake output. Rollback is reverting the commit. Existing flake consumers see no change.

## Open Questions

None.
