package backend

import (
	"fmt"
	"strings"
)

// Query runs taskwarrior queries against the backend CLI.
type Query struct {
	runner CommandRunner
}

func NewQuery(runner CommandRunner) *Query {
	return &Query{runner: runner}
}

func (q *Query) run(args []string) ([]byte, error) {
	return q.runner.Run(args)
}

func (q *Query) Export(f Filter) ([]Task, error) {
	out, err := q.run(append(f.Render(), "export"))
	if err != nil {
		return nil, err
	}
	return ParseExport(out)
}

// ExportQuery runs one opaque filter as a single parenthesized expression.
// Keeping it one argument lets Taskwarrior parse the whole filter without
// treating any term as a command.
func (q *Query) ExportQuery(filter string) ([]Task, error) {
	out, err := q.run([]string{"( " + filter + " )", "export"})
	if err != nil {
		return nil, err
	}
	return ParseExport(out)
}

// SweepQuery applies the removal sweep exclusions to an opaque filter.
func (q *Query) SweepQuery(filter string, action RemoveAction) ([]Task, error) {
	args := []string{"( " + filter + " )", "status.not:deleted"}
	if action == RemoveComplete {
		args = append(args, "status.not:completed")
	}
	args = append(args, "export")
	out, err := q.run(args)
	if err != nil {
		return nil, err
	}
	return ParseExport(out)
}

// SweepTasks exports every task the removal action would change: all
// non-deleted tasks for delete, and additionally excluding completed
// tasks for complete, which Taskwarrior refuses to complete again.
func (q *Query) SweepTasks(f Filter, action RemoveAction) ([]Task, error) {
	out, err := q.run(append(f.RenderSweep(action), "export"))
	if err != nil {
		return nil, err
	}
	return ParseExport(out)
}

// HasAnyTask reports whether any task, including deleted ones, matches
// the filter. Directory removal uses it to tell a directory that once
// held tasks from a name that never existed.
func (q *Query) HasAnyTask(f Filter) (bool, error) {
	out, err := q.run(append(f.criteriaArgs(), "export"))
	if err != nil {
		return false, err
	}
	tasks, err := ParseExport(out)
	if err != nil {
		return false, err
	}
	return len(tasks) > 0, nil
}

// UniqueProjects returns the project values of tasks matching the
// filter; each value is a full dotted project name.
func (q *Query) UniqueProjects(f Filter) ([]string, error) {
	out, err := q.run(append(f.Render(), "_unique", "project"))
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

// UniqueTags returns the tag combinations of tasks matching the
// filter, comma-joined per task.
func (q *Query) UniqueTags(f Filter) ([]string, error) {
	out, err := q.run(append(f.Render(), "_unique", "tags"))
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

func splitLines(out []byte) []string {
	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// ChildProjects splits full dotted project names into the next-level
// segments below the filter's project prefix.
func ChildProjects(prefix string, projects []string) []string {
	seen := map[string]bool{}
	var children []string
	for _, p := range projects {
		if p == "" {
			continue
		}
		var rest string
		if prefix == "" {
			rest = p
		} else {
			if !strings.HasPrefix(p, prefix+".") {
				continue
			}
			rest = strings.TrimPrefix(p, prefix+".")
		}
		seg := rest
		if i := strings.Index(rest, "."); i >= 0 {
			seg = rest[:i]
		}
		if seg != "" && !seen[seg] {
			seen[seg] = true
			children = append(children, seg)
		}
	}
	return children
}

// ChildTags splits comma-joined tag combinations into individual tags.
func ChildTags(combos []string) []string {
	seen := map[string]bool{}
	var tags []string
	for _, combo := range combos {
		for _, tag := range strings.Split(combo, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" && !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

// Mutations run non-interactive: rc.confirmation=off suppresses the
// delete/modify prompts, the gesture itself is the user's confirmation.
const confirmationOff = "rc.confirmation=off"

func (q *Query) Add(f Filter, description string) error {
	args := append([]string{confirmationOff}, f.WriteFilter().Render()...)
	args = append(args, "add", description)
	_, err := q.run(args)
	return err
}

func (q *Query) Delete(uuid string) error {
	_, err := q.run([]string{confirmationOff, uuid, "delete"})
	return err
}

func (q *Query) Modify(uuid string, args []string) error {
	_, err := q.run(append([]string{confirmationOff, uuid, "modify"}, args...))
	return err
}

func (q *Query) Done(uuid string) error {
	_, err := q.run([]string{confirmationOff, uuid, "done"})
	return err
}

// Remove applies the configured removal action to one task.
func (q *Query) Remove(uuid string, action RemoveAction) error {
	switch action {
	case RemoveComplete:
		return q.Done(uuid)
	case RemoveDelete:
		return q.Delete(uuid)
	default:
		return fmt.Errorf("invalid removal action %q", action)
	}
}

func (q *Query) CheckAvailable() error {
	if _, err := q.run([]string{"--version"}); err != nil {
		return fmt.Errorf("taskwarrior CLI required but unavailable: %w", err)
	}
	return nil
}
