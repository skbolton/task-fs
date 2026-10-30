# Spec Delta

## ADDED Requirements

### Requirement: View definitions use _view and may add a modification line

A runtime view's definition file SHALL be named `_view`. Its first line SHALL be the view filter. It MAY contain a second line of Taskwarrior modify arguments that describe how a task enters the view. A definition with more than two non-empty lines SHALL be invalid. A second line that is absent, empty, or whitespace-only SHALL leave the view read-only; a second line with content SHALL make the view writable.

#### Scenario: View without a modification line is read-only

- **WHEN** a view's `_view` file contains only `status:pending +next`
- **THEN** the view lists matching tasks and moving a task into it is rejected as read-only

#### Scenario: View with a modification line is writable

- **WHEN** a view's `_view` file contains `sched.after:sonw sched.before:eonw` on its first line and `sched:{{cycle this week}}` on its second line
- **THEN** the view is writable and moving a task into it applies the rendered modification line

#### Scenario: Blank modification line does not make a view writable

- **WHEN** a view's `_view` file has a filter line followed by an empty line
- **THEN** the view remains read-only

#### Scenario: Extra definition lines are invalid

- **WHEN** a view's `_view` file contains a filter line, a modification line, and a third non-empty line
- **THEN** listing the view fails with EIO and the mount log names the view

### Requirement: View listings use only the filter line

A view listing and task-file lookup SHALL use the rendered first line of `_view` and SHALL ignore a valid second line. Changing the modification line SHALL NOT change which tasks are listed unless the first line also changes.

#### Scenario: Modification line does not filter the view

- **WHEN** a view's first line matches a scheduled task and its second line changes a task's schedule on entry
- **THEN** the view's listing before any move shows only tasks matching the first line

#### Scenario: Editing only the modification line keeps the listing

- **WHEN** the user rewrites the second line of a view's `_view` file and lists the view again
- **THEN** the listed task files are unchanged while the view is writable with the new arguments

## MODIFIED Requirements

### Requirement: Custom view directories expose only their filter and matching tasks

A runtime view directory SHALL list its `_view` file once it exists, plus exactly one file per task matching that view's filter across all projects, using the standard task file naming. It SHALL NOT list project, tag, virtual-tag, or nested runtime view subdirectories. Lookup SHALL resolve `_view` when it exists and minted task-file names present in the view result; every other name SHALL fail with ENOENT.

#### Scenario: A view without a definition is empty

- **WHEN** the user creates a view with `mkdir @Due` and lists it before creating `_view`
- **THEN** the listing contains neither `_view` nor any task files

#### Scenario: Runtime view lists tasks across projects

- **WHEN** the runtime view filter matches a task in project `home` and a task with no project, and the user lists the view directory
- **THEN** both task files appear directly in that directory along with `_view`

#### Scenario: Runtime view directories do not expose filter vocabulary

- **WHEN** the user lists a runtime view whose results include tasks from project `home` and tagged `next`
- **THEN** the listing contains no `home` directory, no `+next` directory, and no virtual-tag directory

#### Scenario: Only the definition file and view results resolve

- **WHEN** the user attempts to enter a runtime view directory followed by `home`, `+next`, or another name that is neither `_view` nor a listed task file
- **THEN** the lookup fails with ENOENT

#### Scenario: Same task appears in its runtime view and filter view

- **WHEN** a task matches a runtime view and also resides in `/home`
- **THEN** its file is listed in both the runtime view directory and `/home`

### Requirement: Root lists configured custom views

The root directory SHALL list one `@Name` directory for each persisted runtime view definition, including a view with no `_view` or whose filter matches no tasks. Runtime view directories SHALL be listed only at the root; project, tag, virtual-tag, and runtime view directories SHALL NOT list them.

#### Scenario: Custom view directories appear at the root

- **WHEN** the user creates a view with `mkdir "@Due Next Week"` and writes `status:pending sched.after:sonw sched.before:eonw` into its `_view` file, then lists the root
- **THEN** the listing contains the directory `@Due Next Week`

#### Scenario: Empty custom view directory remains visible

- **WHEN** a runtime view's filter matches no task, or it has no `_view`, and the user lists the root
- **THEN** its `@Name` directory is still present, and listing it shows no task files

#### Scenario: Custom view directories are not listed below the root

- **WHEN** the user lists a project directory, a tag directory, or a virtual-tag directory
- **THEN** the listing does not contain any persisted `@Name` runtime view directory

### Requirement: Custom view contents follow the rendered filter

A runtime view's listing and task-file lookups SHALL read its `_view` file and use the first line rendered for the current access. A view with no `_view` SHALL produce no entries. An empty or whitespace-only first line SHALL list `_view` and no task files. A malformed filter, invalid template, failed command, or invalid definition SHALL make the access fail with EIO. A task that starts or stops matching after the definition or a command's output changes SHALL appear or disappear on the next view access without remounting.

#### Scenario: Editing the filter changes the result

- **WHEN** the user lists a runtime view, rewrites the first line of its `_view` with a different valid filter, and lists the view again
- **THEN** the second listing reflects the new filter without remounting

#### Scenario: Newly matching task appears on the next listing

- **WHEN** a view's `_view` contains a date-producing command and a command result change makes an existing task match
- **THEN** listing the view after the change includes that task's file without remounting

#### Scenario: No-longer-matching task disappears on the next listing

- **WHEN** a command result change makes a previously listed task stop matching the runtime view
- **THEN** listing the view after the change omits that task's file

#### Scenario: Lookup uses the current result

- **WHEN** the user looks up a task-file name that was listed before a definition or command result change but is not in the currently rendered result
- **THEN** the lookup fails with ENOENT

#### Scenario: Empty filter yields an empty view

- **WHEN** the user lists a runtime view whose `_view` exists but whose first line is empty
- **THEN** the listing contains `_view` and no task files

#### Scenario: Missing definition file yields an empty view

- **WHEN** the user lists a runtime view with no `_view`
- **THEN** the listing contains neither `_view` nor task files

#### Scenario: Malformed definition makes the view unreadable

- **WHEN** a runtime view's `_view` contains a malformed Taskwarrior filter, an invalid template, or more than two definition lines
- **THEN** listing the view fails with EIO and the mount log names the view

#### Scenario: Static custom views are unaffected

- **WHEN** a runtime view's `_view` contains no command substitutions
- **THEN** its listing and lookup behavior use that exact filter
