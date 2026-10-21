# Tasks

## 1. Module and flake wiring

- [x] 1.1 Create `nix/home-manager.nix` defining `services.task-fs` (enable, package, automount.enable, automount.path with `~` expansion against `config.home.homeDirectory`, Linux-only assertion) and expose it as `flake.homeModules.task-fs` in `flake.nix`; verify with `nix eval .#` that a config importing `inputs.task-fs.homeModules.task-fs` accepts `services.task-fs.enable = true;` without unknown-option errors
- [x] 1.2 Implement the systemd user unit (`systemd.user.services.task-fs`: `ExecStartPre` mkdir, `ExecStart task-fs mount <path>`, `ExecStopPost -fusermount3 -u <path>`, `WantedBy = default.target`, `Restart = on-failure`, `fusermount3` and `taskwarrior3` on the unit PATH) and verify the generated unit text contains the configured mount path and both executables

## 2. Evaluation checks

- [x] 2.1 Add a flake check that evaluates the module via `lib.evalModules` with stub `home.homeDirectory` and asserts the unit file content (mount path, `fusermount3 -u`, task-fs package binary), that `~`-paths resolve under the stub home, and that a custom `services.task-fs.package` propagates into the unit; verify with `nix flake check`
- [x] 2.2 Add a flake check asserting evaluation fails on a non-Linux platform with the platform message; verify with `nix flake check`

## 3. Documentation

- [x] 3.1 Document Home Manager usage in README.md (flake input, `inputs.task-fs.homeModules.task-fs` import, `services.task-fs` example from the proposal, TASKDATA/session-environment note) and verify the documented snippet evaluates via the module's check
