# Spec Delta

## ADDED Requirements

### Requirement: Custom view directories expose only their filter and matching tasks

A runtime view directory SHALL list its `_filter` file once it exists, plus exactly one file per task matching that filter across all projects, using the standard task file naming. It SHALL NOT list project, tag, virtual-tag, or nested runtime view subdirectories. Lookup SHALL resolve `_filter` when it exists and minted task-file names present in the view result; every other name SHALL fail with ENOENT.

#### Scenario: A view without a filter is empty

- **WHEN** the user creates a view with `mkdir @Due` and lists it before creating `_filter`
- **THEN** the listing contains neither `_filter` nor any task files

#### Scenario: Runtime view lists tasks across projects

- **WHEN** the runtime view filter matches a task in project `home` and a task with no project, and the user lists the view directory
- **THEN** both task files appear directly in that directory along with `_filter`

#### Scenario: Runtime view directories do not expose filter vocabulary

- **WHEN** the user lists a runtime view whose results include tasks from project `home` and tagged `next`
- **THEN** the listing contains no `home` directory, no `+next` directory, and no virtual-tag directory

#### Scenario: Only the filter file and view results resolve

- **WHEN** the user attempts to enter a runtime view directory followed by `home`, `+next`, or another name that is neither `_filter` nor a listed task file
- **THEN** the lookup fails with ENOENT

#### Scenario: Same task appears in its runtime view and filter view

- **WHEN** a task matches a runtime view and also resides in `/home`
- **THEN** its file is listed in both the runtime view directory and `/home`

### Requirement: Runtime view entries are distinguished by persisted definitions

At the root, a name beginning with `@` SHALL resolve as a runtime view when a persisted view definition with that name exists, and otherwise as an ordinary project segment. An `@NAME` project SHALL remain reachable whenever no runtime view of that name exists.

#### Scenario: At-prefixed project resolves without a view definition

- **WHEN** a task has project `@Due`, no runtime view `@Due` exists, and the user looks up `@Due` at the root
- **THEN** the lookup resolves the project segment

#### Scenario: Runtime view definition turns the root entry into a view

- **WHEN** a task has project `@Due` and a runtime view `@Due` exists
- **THEN** looking up `@Due` at the root resolves the runtime view directory rather than the project

#### Scenario: Removing the definition restores the project

- **WHEN** the user removes the runtime view definition `@Due` while a task still has project `@Due`
- **THEN** looking up `@Due` at the root resolves the project segment again

## MODIFIED Requirements

### Requirement: Path segments are an order-free filter

The system SHALL interpret a filesystem path as the conjunction of its segments' filters, where a `+COMPLETED` or `+DELETED` segment is a status virtual-tag filter, any other segment starting with `+` is a tag filter, and every other segment is a project filter. At the root, a segment matching a persisted runtime view definition SHALL select that view directory instead of a project filter; view directories do not accumulate filters. The order of segments SHALL NOT change the resulting filter.

#### Scenario: Equivalent paths in any order

- **WHEN** the user lists `/+next/home` and `/home/+next` on a database with a task in project `home.clean-garage` tagged `next`
- **THEN** both listings show the same set of task files

#### Scenario: Deepening under a tag

- **WHEN** the user is in `/+next` and lists its entries
- **THEN** the listing contains project segments for projects of tasks tagged `next` (for example `home`, `finances`) and tag segments for other tags of those tasks

### Requirement: Unknown segments are not directories

The system SHALL fail directory lookup with ENOENT for a segment that is neither a project segment, a tag of any task matching the current filter, nor a persisted runtime view definition at the root.

#### Scenario: Typo in project name

- **WHEN** the user attempts to enter `/homi` while pending tasks exist in project `home.*` but not `homi`
- **THEN** the lookup fails with ENOENT rather than presenting an empty directory

### Requirement: Root lists configured custom views

The root directory SHALL list one `@Name` directory for each persisted runtime view definition, including a view with no `_filter` or whose filter matches no tasks. Runtime view directories SHALL be listed only at the root; project, tag, virtual-tag, and runtime view directories SHALL NOT list them.

#### Scenario: Custom view directories appear at the root

- **WHEN** the user creates a view with `mkdir "@Due Next Week"` and writes `status:pending sched.after:sonw sched.before:eonw` into its `_filter` file, then lists the root
- **THEN** the listing contains the directory `@Due Next Week`

#### Scenario: Empty custom view directory remains visible

- **WHEN** a runtime view's filter matches no task, or it has no `_filter`, and the user lists the root
- **THEN** its `@Name` directory is still present, and listing it shows no task files

#### Scenario: Custom view directories are not listed below the root

- **WHEN** the user lists a project directory, a tag directory, or a virtual-tag directory
- **THEN** the listing does not contain any persisted `@Name` runtime view directory

### Requirement: Custom view contents follow the rendered filter

A runtime view's listing and task-file lookups SHALL read its `_filter` file and use the filter rendered for the current access. A view with no `_filter` SHALL produce no entries. An empty or whitespace-only `_filter` SHALL list `_filter` and no task files. A malformed filter, invalid template, or failed command SHALL make the access fail with EIO. A task that starts or stops matching after the filter file or a command's output changes SHALL appear or disappear on the next view access without remounting.

#### Scenario: Editing the filter changes the result

- **WHEN** the user lists a runtime view, rewrites its `_filter` with a different valid filter, and lists the view again
- **THEN** the second listing reflects the new filter without remounting

#### Scenario: Newly matching task appears on the next listing

- **WHEN** a view's `_filter` contains a date-producing command and a command result change makes an existing task match
- **THEN** listing the view after the change includes that task's file without remounting

#### Scenario: No-longer-matching task disappears on the next listing

- **WHEN** a command result change makes a previously listed task stop matching the runtime view
- **THEN** listing the view after the change omits that task's file

#### Scenario: Lookup uses the current result

- **WHEN** the user looks up a task-file name that was listed before a filter or command result change but is not in the currently rendered result
- **THEN** the lookup fails with ENOENT

#### Scenario: Empty filter yields an empty view

- **WHEN** the user lists a runtime view whose `_filter` exists but is empty
- **THEN** the listing contains `_filter` and no task files

#### Scenario: Missing filter file yields an empty view

- **WHEN** the user lists a runtime view with no `_filter`
- **THEN** the listing contains neither `_filter` nor task files

#### Scenario: Malformed filter makes the view unreadable

- **WHEN** a runtime view's `_filter` contains a malformed Taskwarrior filter or an invalid template
- **THEN** listing the view fails with EIO and the mount log names the view

#### Scenario: Static custom views are unaffected

- **WHEN** a runtime view's `_filter` contains no command substitutions
- **THEN** its listing and lookup behavior use that exact filter

### Requirement: Custom view names take precedence at the root

At the root, lookup SHALL resolve a persisted runtime view before a project segment of the same name. A root project whose first segment matches a runtime view name SHALL be shadowed while that view exists.

#### Scenario: Configured custom view wins over a same-named project

- **WHEN** a task has project `@Due` and a runtime view `@Due` exists, then the user looks up `@Due` at the root
- **THEN** the lookup resolves the runtime view directory rather than the project

#### Scenario: At-prefixed project resolves when no custom view uses the name

- **WHEN** a task has project `@Due` and no runtime view named `Due` exists, then the user looks up `@Due` at the root
- **THEN** the lookup resolves the project segment

## REMOVED Requirements

### Requirement: Custom view directories are lookup-only

**Reason**: A runtime view directory must expose its `_filter` file so the filter can be read, so its contents are no longer only task files.

**Migration**: Expect `_filter` alongside matching task files; project, tag, virtual-tag, and nested view segments still do not resolve inside a view.
