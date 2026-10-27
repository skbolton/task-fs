# Proposal

## Why

Custom views are saved Taskwarrior filters, but the filter string is fixed for the life of the mount. Users who derive dates or filter terms from local state need a shell command to produce part of a custom view filter, and a long-lived Home Manager mount must recompute that part when the view is browsed rather than at session start.

## What Changes

- Extend `--view NAME=FILTER` with executable `{{ command }}` blocks inside the filter. Each block runs through `/bin/sh -c` in the mount environment and its trimmed standard output is spliced into the filter as text.
- Parse templates at mount time: reject unbalanced or empty blocks and support a literal escaped delimiter. Render each template once during mount validation so a failing command or a malformed rendered filter still aborts the mount with the custom view's name. **BREAKING** for a static filter whose literal text contains `{{`: it must now escape each opening delimiter as `\{{`.
- Re-render a custom view before its listing, task lookup, file removal, and directory-removal checks, so a long-lived mount reflects the command's current output without remounting.
- Treat execution failures as an unreadable view (`EIO`) while logging the view name, command, stderr, and exit status to the mount process's standard error; bound each command with a timeout so a hung command cannot hang FUSE indefinitely.
- Add a short render cache with concurrent-call coalescing so a single directory traversal does not execute the same command once per task-file lookup, while a later traversal still gets a fresh value.
- Make the Home Manager automount environment able to run the shell syntax: guarantee a POSIX shell and coreutils commands, and document that commands outside that baseline must use absolute paths or a wrapped script.
- Document `{{ ... }}` syntax, shell quoting in interactive commands versus Nix strings, output handling, validation, and failure behavior.

Out of scope: dynamic view names, editing view definitions through the filesystem, caching query results, running commands from any source other than mount-time view definitions, and changing existing create/move gesture restrictions inside custom views.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `mount-lifecycle`: Adds the `{{ command }}` filter template syntax, mount-time template and rendered-filter validation, command execution environment, timeouts, output handling, failure reporting, and re-render cache behavior.
- `task-navigation`: Requires custom view listings and task lookups to follow the filter rendered for the current access rather than a value frozen at mount.
- `task-mutations`: Requires custom view file removal and directory-removal emptiness checks to use the currently rendered filter, including the failure behavior when rendering fails.
- `home-manager-module`: Guarantees the automount service can execute `{{ ... }}` blocks with a POSIX shell and coreutils, and documents absolute paths for other programs.

## Impact

- CLI: `cmd/task-fs/main.go` gains template parsing, mount-time rendering validation, and usage/docs updates.
- Backend/VFS: a template renderer and command executor are wired into view validation and `viewNode`/`rmdirView` query paths; `internal/vfs/tree.go` gains render caching and failure mapping.
- Nix: `nix/home-manager.nix` adds the command runtime to the automount `PATH`; module checks cover a dynamic view.
- Documentation: `README.md` documents the syntax, quoting rules, Home Manager command environment, and failure behavior.
- Existing static `--view` definitions and all existing navigation and mutation behavior remain unchanged apart from the literal `{{` escape requirement.
