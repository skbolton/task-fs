# Spec Delta

## ADDED Requirements

### Requirement: Custom view contents follow the rendered filter

A custom view's listing and task-file lookups SHALL use the filter rendered for the current access. A task that starts or stops matching after a command's output changes SHALL appear or disappear on the next view access without remounting. Lookups SHALL resolve only minted task-file names in that current result.

#### Scenario: Newly matching task appears on the next listing

- **WHEN** a custom view filter contains a date-producing command and a command result change makes an existing task match
- **THEN** listing the custom view after the change includes that task's file without remounting

#### Scenario: No-longer-matching task disappears on the next listing

- **WHEN** a command result change makes a previously listed task stop matching the custom view
- **THEN** listing the custom view after the change omits that task's file

#### Scenario: Lookup uses the current result

- **WHEN** the user looks up a task-file name that was listed before a command result change but is not in the currently rendered result
- **THEN** the lookup fails with ENOENT

#### Scenario: Static custom views are unaffected

- **WHEN** a custom view contains no command substitutions
- **THEN** its listing and lookup behavior are identical to the existing fixed-filter custom view behavior
