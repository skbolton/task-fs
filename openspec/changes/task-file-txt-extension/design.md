# Design

## Context

See proposal.md - Why. Task file names are produced by `internal/naming` and every consumer asks that package whether a name is minted (`naming.FileName` and `naming.IsMintedName`). There is no stored representation of a task file: listings generate names from Taskwarrior data on demand, so the extension is not persisted anywhere.

`SuffixLenBase` is currently `13` even though `.md` needs `12` bytes (`.` + 8 hex + `.md`), making the existing truncation budget one byte conservative. `.txt` needs exactly `13` bytes (`.` + 8 hex + `.txt`).

## Goals / Non-Goals

**Goals:**

- Change the minted task file extension from `.md` to `.txt` everywhere names are minted, recognized, documented, or tested.
- Keep naming deterministic, stable, Windows-safe, and within the 255-byte limit.
- Make all user-facing examples and specifications consistent with the new extension.

**Non-Goals:**

- Adding note editing or any read/write task content surface.
- Migrating or aliasing old `.md` names.
- Changing the short-UUID suffix, sanitization rules, or any mutation behavior.

## Decisions

**Use `.txt` rather than a domain-specific extension.** `.txt` is a conventional, neutral extension that does not imply Markdown or in-app editing, while still satisfying the `<description>.<short-uuid>.<extension>` shape that distinguishes task files from ordinary names. Alternatives considered:

- `.md`: rejected because it advertises a note/Markdown workflow the filesystem does not implement and will not own.
- No extension: rejected because `<description>.<short-uuid>` without a final component reads like a strange filename and loses familiar file-type affordance.
- `.task`/`.note`/`.todo`: rejected as unusual extensions that invite tooling questions without adding information the short-UUID suffix does not already carry.

**Keep the extension in the minted-name identity.** `FileName` mints `.txt` and `IsMintedName` accepts only the `.txt` form (`<anything>.<8-hex>.txt`). All callers inherit the change: listing, lookup, create-gesture rejection, rename handling, and custom view name validation. No caller hard-codes the extension outside `internal/naming`.

**Treat `.md` names as ordinary names.** There is no stored data to migrate and no value in keeping the old extension reserved. A user who touches `Report.a1b2c3d4.md` gets a normal task-creation gesture with that literal description, and a custom view may use that name. This keeps one minted-name pattern instead of two.

**Keep `SuffixLenBase` at 13 and correct its comment.** With `.txt`, the constant now matches the real suffix length. The change does not alter the truncation formula.

## Risks / Trade-offs

- [Existing scripts or shell history refer to `.md` names] → Document the extension change in `README.md`; names are regenerated on the next listing, and users update literals. No filesystem migration is possible or needed.
- [Users expect old `.md` task names to keep resolving] → Deliberately not supported; the old extension becomes an ordinary description text. Keeping dual patterns would complicate minted-name detection for no durable benefit.
- [Missing an example or fixture leaves stale `.md` text] → Search the repository (excluding archived changes and the stale mount directory) and update README, tests, and spec scenarios; verify the test suite has no `.md` task-file expectations.
