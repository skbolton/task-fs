# Design

## Context

See `proposal.md` for motivation. The current module owns one `services.task-fs.automount` submodule and emits one `systemd.user.services.task-fs` unit. The mount command already inherits the process environment and passes it to every `task` invocation, so `TASKDATA` needs no Go or CLI change; only the generated unit environment changes. Module behavior is checked by evaluating `nix/home-manager.nix` through `nix/hm-module-test.nix` with stubbed Home Manager options.

The main `home-manager-module` spec still describes the older single-automount surface and `--view` flags. The delta in this change moves its mount requirements to the named-mount model and runtime `_view` terminology.

## Goals / Non-Goals

**Goals:**

- Replace the single automount with a named set of independently configured mounts.
- Give every mount a stable systemd unit name, its own mountpoint, flags, PATH extras, and optional database.
- Keep module evaluation and its checks independent of a full Home Manager installation.

**Non-Goals:**

- Changing the CLI, mount implementation, or task data.
- Creating or initializing the `TASKDATA` directory.
- Per-mount view stores or per-mount removal defaults beyond existing flags.
- Non-Linux support or NixOS system-level mounts.

## Decisions

**Use `services.task-fs.mounts` as an attribute set of submodules.** Each key is the mount name and therefore its unit identity; no separate `name` option is needed. The submodule has `path` (required, non-empty), `mountFlags` (list of strings), `extraPackages` (list of packages), and `taskData` (`nullOr str`, default `null`). Presence of an entry means the mount is active. `services.task-fs.enable` stays the global switch and still installs the package.

Alternatives considered:

- A list of submodules with a required `name` field. Rejected because the attribute key already provides a unique, stable name and avoids redundant configuration.
- Keeping `services.task-fs.automount` and adding a nested list. Rejected because it preserves a misleading single-mount namespace and leaves two ways to express the same thing.
- A compatibility alias for `automount`. Rejected because the project is alpha, the surface is small, and silently translating two shapes would complicate assertions and documentation.

**Name each unit `task-fs-<name>.service`.** This keeps units recognizable (`task-fs-work`, `task-fs-personal`) and stable across rebuilds. Mount keys are restricted to letters, digits, hyphens, and underscores with an assertion, so a key cannot produce an invalid or ambiguous systemd unit name or collide with the unit prefix.

Alternatives considered:

- Deriving the unit name from the mount path. Rejected because path edits would rename units and because paths need escaping.
- Indexing units by list position. Rejected because reordering or removing an entry silently repoints units.

**Build every unit from one shared per-mount expression.** `lib.mapAttrs'` produces the `systemd.user.services` entries, with a local `expandHome` helper shared by `path` and `taskData`. Each unit keeps the current `ExecStartPre`, `ExecStart`, `ExecStopPost`, `Restart`, and `WantedBy` behavior, but uses its own resolved path and flags. This keeps lifecycle behavior identical per mount instead of duplicating unit logic.

**Set `TASKDATA` through the unit `Environment`, not an `ExecStart` prefix.** When `taskData` is non-null, append `TASKDATA=<resolved path>` to that unit's `Environment` next to its `PATH`. The mount process and every child `task` call inherit it, and systemd owns the escaping. A null `taskData` omits the variable, preserving the existing session-environment inheritance. An empty string is treated as invalid rather than as "unset" to avoid a mount that silently targets Taskwarrior's default database.

Alternatives considered:

- Prefixing `ExecStart` with `TASKDATA=...`. Rejected because the value needs shell-style quoting and would apply only to that one command, leaving helper processes dependent on however the prefix is written.
- Adding `TASKDATA` to `sessionEnvironment`. Rejected because it is global and would defeat per-mount database selection.

**Keep PATH assembly per mount.** Each unit gets the existing baseline PATH (`taskwarrior3`, `fuse3`, `coreutils`, plus the setuid wrapper directory) and only its own `extraPackages`. The repository's `extraPackages` behavior therefore follows the mount that declares it instead of leaking to every unit.

**Assert configuration errors early.** Assertions cover the Linux-only requirement, at least one valid mount path per entry, valid mount names, and non-empty `taskData` when set. The module does not create the `taskData` directory; Taskwarrior resolves and initializes its own data location, while TaskFS only creates the mountpoint it owns.

## Risks / Trade-offs

- **Existing configurations break** → Document the old-to-new mapping in README: move `services.task-fs.automount` fields into one `services.task-fs.mounts.<name>` entry, and keep `services.task-fs.enable` as the global switch.
- **Spec archive ordering** → The main `home-manager-module` spec lags completed view changes. Archive those pending view changes before this one so the MODIFIED requirement text lands on the intended version.
- **A typo in `taskData` silently targets a different default database** → Treat an unset variable as inheritance, but reject an explicitly empty value; document that the path must be the intended Taskwarrior data directory.
- **Unit-name assertions are stricter than systemd** → Allow only letters, digits, hyphens, and underscores. This is deliberately narrower than systemd escaping so unit names stay readable and predictable.
- **Removing a mount leaves its systemd unit file until the next Home Manager activation** → Normal Home Manager rebuild behavior; `ExecStopPost` unmounts the path when the unit stops, and the next activation removes the unit.

## Migration Plan

1. Replace the single `automount` block with a named `mounts` entry, for example `services.task-fs.mounts.default = { path = "~/some/path"; }`.
2. Move `mountFlags` and `extraPackages` from the old automount block into the same entry.
3. Add `taskData = "~/task-data/work"` (or an absolute path) to any mount that should use a non-session database.
4. Keep `services.task-fs.enable = true`; it now enables all declared mounts.

Rollback is reverting the change and restoring the old `automount` block.

## Open Questions

None.
