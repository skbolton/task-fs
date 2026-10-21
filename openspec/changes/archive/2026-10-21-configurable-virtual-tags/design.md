# Design

## Context

`cmd/task-fs/main.go` parses `mount <mountpoint>` by hand, runs `task _tags`, and passes `VirtualTagsFromDiscovery(entries)` (the fully-uppercase entries) to `vfs.FS.SetVirtualTags`. The vfs uses that list for both listing (Readdir emits `.+NAME`) and resolution (`isVirtualTag` gates `resolveChild` and the rmdir branch), and `Filter.WithSegment` accepts any fully-uppercase `.+NAME` segment. See proposal.md for the motivation.

## Goals / Non-Goals

**Goals:**

- User-selected virtual-tag set at mount time, validated against backend discovery.
- Selectable presentation: regular `+NAME` entries by default, dot-prefixed `.+NAME` entries with `--virtual-hidden`.
- Both spellings resolve on lookup for selected tags, regardless of presentation.

**Non-Goals:**

- Changing any rendered Taskwarrior filter; presentation is a naming/visibility concern only.
- Persisting mount options across sessions (no config file).
- Making non-selected virtual tags resolvable through any spelling.

## Decisions

### Flag parsing with stdlib `flag.FlagSet`

Replace the hand-rolled `args[0]`/`args[1]` parsing with `flag.NewFlagSet("mount", flag.ContinueOnError)`: `-t/--virtual-tags` (string, default `COMPLETED,OVERDUE,DUE,ACTIVE,WAITING`) and `-H/--virtual-hidden` (bool). Go's flag package accepts both one- and two-dash forms of the same name, so one definition covers both spellings. The usage message is updated to document both flags. Alternative: keep manual parsing — rejected, it grows brittle with value-taking flags and error messages.

### Selection resolution lives in `backend`

Add `ResolveVirtualTags(selection string, discovered []string) ([]string, error)` next to `VirtualTagsFromDiscovery`: split on `,`, trim surrounding whitespace, reject empty entries; `ALL` (exact uppercase) must appear alone and resolves to the discovered set; any other entry must be a member of the discovered set or the function errors naming it. `main.go` calls it with the discovered set and aborts the mount on error, satisfying the fail-before-mount scenario. The default value runs through the same resolver but is intersected with the discovered set instead of failing: a minimal taskwarrior configuration that does not discover, say, `WAITING` must not make the flagless mount unusable. Strict validation stays reserved for entries the user typed.

### Config passed to `vfs.New` instead of setters

Replace `SetVirtualTags` with a `vfs.Config{VirtualTags []string, Hidden bool}` parameter on `New`. The setter pattern invites reconfiguration after inodes exist; the selected set and presentation are mount-time constants. Call sites are `main.go` and tests, so the signature change is cheap. Alternative: a second setter — rejected for the reason above.

### Spelling rule: prefix is presentation, membership is selection

`FS.isVirtualTag` keeps meaning "name is in the selected set" (the selected set only contains discovered fully-uppercase names, so the uppercase constraint holds by construction). Resolution strips either prefix: a segment `+NAME` or `.+NAME` whose stripped name is selected resolves as a virtual tag, taking precedence over tag resolution in the `+` branch. Lookup therefore accepts both spellings in both modes; Readdir lists each selected tag exactly once in the presentation form (`+`+name, or `.+`+name when hidden). Non-status virtual tags already render as `+NAME` taskwarrior filters and status tags via `statusVirtualTags`, so `Filter.Render` is untouched.

Consequences handled explicitly:

- `resolveChild`'s `+` branch checks the virtual-tag set before the declared/`ChildTags` check, so a selected virtual tag shadows a real uppercase tag of the same name (e.g. a task tagged `DUE`); unselected uppercase tags keep resolving as tags.
- `rmdir`'s virtual-tag branch (non-destructive emptiness check) triggers on a stripped selected name under either prefix; the destructive declared-tag branch is untouched for everything else.
- `mkdir` needs no change: dot-prefixed and all-uppercase `+` names are already refused with EACCES, which covers `mkdir +COMPLETED` and `mkdir .+DUE` in both modes.

### Readdir and nesting unchanged in shape

The existing guard (list virtual-tag entries only when the accumulated filter carries none) stays; only the entry names and the source list change.

### Home Manager option: `automount.mountFlags` as a token list

Expose `services.task-fs.automount.mountFlags` (list of strings, default empty), interpolated into the automount unit's `ExecStart` between `mount` and the mountpoint: `task-fs mount ${concatStringsSep " " (mountFlags ++ [ mountPath ])}`. Each list item is one argv token, so a value-taking flag is two items (`"--virtual-tags" "COMPLETED,DUE"`); this avoids shell quoting in the unit and matches how systemd splits `ExecStart`. The option lives under `automount` because the mount command exists only in the automount service. The mount already accepts flags in any position relative to the mountpoint, so the placement is safe. Alternative: a freeform string option — rejected, it invites quoting bugs.

## Risks / Trade-offs

- [Default entries missing from a backend's `_tags` output are silently dropped] → Intersection is deliberate; a mount that half-honors defaults beats one that refuses to start. Documented in the flag's help text.
- [Selected virtual tag shadows a real uppercase tag of the same name] → Pre-existing tension (uppercase tags were already undeclarable); the tag remains reachable under any path whose filter carries a project that excludes the virtual-tag ambiguity, and the shadowing is visible in listings.
- [Scripts relying on hidden entries must now pass `--virtual-hidden`] → Both spellings still resolve on lookup, so only listing visibility changes; old behavior is exactly `--virtual-hidden --virtual-tags ALL`.
- [Combinatorial-tree guard regression if listing forgets the nesting rule] → Existing tree tests cover the no-nesting scenario; tasks add mode-matrix cases.

## Migration Plan

No data migration. Rollback is `task-fs mount <mp> --virtual-hidden --virtual-tags ALL`, which reproduces today's behavior exactly.

## Open Questions

None.
