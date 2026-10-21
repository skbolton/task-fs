# Spec Delta: task-mutations

## MODIFIED Requirements

### Requirement: Moving into COMPLETED completes the task

The system SHALL mark the task completed when the user moves its file into a status virtual-tag directory — spelled `+COMPLETED` or, equivalently, `.+COMPLETED` — ignoring the remainder of the destination path beyond the status segment. The system SHALL reopen the task (status pending) when the user moves its file out of a status virtual-tag directory and SHALL additionally apply the destination filter delta per the match-the-destination rule.

#### Scenario: Completing from a project directory

- **WHEN** the user runs `mv "Do taxes.a1b2c3d4.md" /+COMPLETED/`
- **THEN** the task's status becomes completed and its file no longer appears in pending listings

#### Scenario: Moving out of COMPLETED reopens and re-locates

- **WHEN** the user moves a task file from `/.+COMPLETED/` into the existing directory `/finances/`
- **THEN** the task's status becomes pending, its project becomes `finances`, and its file appears in `/finances`
