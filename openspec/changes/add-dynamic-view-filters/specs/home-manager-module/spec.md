# Spec Delta

## ADDED Requirements

### Requirement: Automount provides the dynamic view command runtime

The automount service SHALL make a POSIX shell and coreutils commands available to `{{ ... }}` blocks. Commands outside that baseline SHALL be referenced by absolute path or through an executable script, and a `--view` value containing template blocks SHALL reach the mount process unchanged as one argument.

#### Scenario: Coreutils command resolves in the service

- **WHEN** a Home Manager user defines `--view 'Next Week=status:pending due.before:{{ date -d "+7 days" +%F }}'`
- **THEN** the automount service can run `date` and the custom view renders successfully

#### Scenario: External command script uses an absolute path

- **WHEN** a dynamic view block invokes a script outside the coreutils baseline
- **THEN** the user can reference the script by absolute Nix store path or through an executable wrapper and the block runs in the service environment

#### Scenario: Template value stays one argument

- **WHEN** `mountFlags` contains a `--view` value with spaces and `{{ ... }}` blocks
- **THEN** `task-fs mount` receives the entire value as one argument with the braces unchanged
