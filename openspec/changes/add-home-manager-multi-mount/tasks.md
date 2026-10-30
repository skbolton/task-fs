# Tasks

## 1. Replace the single automount option tree

- [x] 1.1 In `nix/home-manager.nix`, remove `services.task-fs.automount` and add `services.task-fs.mounts` as `attrsOf (submodule ...)` with per-mount `path`, `taskData`, `mountFlags`, and `extraPackages` options; verify `nix eval` of the module's options has no `automount` option and exposes `mounts`.
- [x] 1.2 Generate one unit per mount with `lib.mapAttrs'` under `systemd.user.services`, naming it `task-fs-<key>`, and keep the unit gated by `services.task-fs.enable`; verify a two-entry evaluation produces two units.
- [x] 1.3 Add assertions rejecting empty mount paths, invalid mount-name characters, and an explicitly empty `taskData`; verify each invalid example yields a failing assertion in the module check.

## 2. Resolve paths and inject the per-mount database

- [x] 2.1 Add a shared `expandHome` helper and apply it to both `mounts.<name>.path` and `mounts.<name>.taskData`; verify `~` values expand against `home.homeDirectory`.
- [x] 2.2 Append `TASKDATA=<resolved path>` to a mount unit's `Environment` only when `taskData` is set; verify the generated unit contains it and that a mount without `taskData` has no `TASKDATA` entry.
- [x] 2.3 Keep PATH assembly per mount so `extraPackages` appear only in the declaring mount's unit; verify a mount without extras still has only the baseline `taskwarrior3`, `fuse3`, and `coreutils` PATH.

## 3. Rewrite the Home Manager module checks

- [x] 3.1 Rework `nix/hm-module-test.nix` around a multi-mount configuration and assert per-unit `ExecStartPre`, `ExecStart`, `ExecStopPost`, `Restart`, `WantedBy`, and default package usage; verify with `nix build .#checks.<system>.hm-module`.
- [x] 3.2 Add checks for per-mount `mountFlags`, `extraPackages`, `taskData`, tilde expansion, custom package propagation, disabled evaluation, wrapped-module evaluation, and non-Linux evaluation; verify the check target passes.
- [x] 3.3 Add checks for invalid mount names and missing paths producing the expected assertions, and confirm no generated unit advertises removed view or virtual-tag flags; verify the check target passes.

## 4. Document the new interface and migration

- [x] 4.1 Update the Home Manager section of `README.md` to use `services.task-fs.mounts.<name>`, show two mounts, and document per-mount `mountFlags` and `extraPackages`; verify the option names match `nix/home-manager.nix`.
- [x] 4.2 Document optional per-mount `taskData`, its `TASKDATA` effect, tilde expansion, and the inherited session database when unset; verify the examples match the generated unit behavior covered by the module checks.
- [x] 4.3 Add a breaking-change migration note mapping the old `services.task-fs.automount` fields to one `mounts.<name>` entry; verify no README example still uses `automount`.

## 5. Integration verification

- [x] 5.1 Run `openspec validate add-home-manager-multi-mount --strict` and confirm the delta spec and task list validate.
- [x] 5.2 Run the full flake check and the Go test suite (`go test ./...`) to confirm the module change did not disturb the existing package or filesystem behavior.

## Workflow follow-up

- Archive the pending completed view changes before this change so the modified `home-manager-module` requirements land on the intended spec text.
- Archive this change after review and verification.
