package backend

import (
	"fmt"
	"strings"
)

// Status virtual tags carry status semantics instead of being passed
// through as +NAME filters.
var statusVirtualTags = map[string]string{
	"COMPLETED": "status:completed",
	"DELETED":   "status:deleted",
}

// Filter is the accumulated taskwarrior filter of a filesystem path.
type Filter struct {
	Project     string
	Tags        []string
	VirtualTags []string
}

// HasStatus reports whether the filter constrains the task status.
func (f Filter) HasStatus() bool {
	for _, vt := range f.VirtualTags {
		if _, ok := statusVirtualTags[vt]; ok {
			return true
		}
	}
	return false
}

// IsVirtualTag reports whether a segment name (without '+') names a
// virtual tag.
func IsVirtualTag(name string) bool {
	_, ok := statusVirtualTags[name]
	return ok
}

// StatusVirtualTag returns the status filter for a status-carrying
// virtual tag, or false if the name is not one.
func StatusVirtualTag(name string) (string, bool) {
	s, ok := statusVirtualTags[name]
	return s, ok
}

// WithSegment returns the filter extended by one path segment: ".+X"
// where X is fully uppercase is a virtual tag, "+x" is a tag, anything
// else is a project segment appended to the dotted prefix.
func (f Filter) WithSegment(seg string) (Filter, error) {
	if seg == "" {
		return f, fmt.Errorf("empty segment")
	}
	if strings.HasPrefix(seg, ".+") {
		name := seg[2:]
		if name == "" {
			return f, fmt.Errorf("empty virtual-tag segment %q", seg)
		}
		if !isUppercase(name) {
			return f, fmt.Errorf("invalid virtual-tag segment %q", seg)
		}
		for _, vt := range f.VirtualTags {
			if vt == name {
				return f, fmt.Errorf("segment %q already in filter", seg)
			}
		}
		out := f.clone()
		out.VirtualTags = append(out.VirtualTags, name)
		return out, nil
	}
	if strings.HasPrefix(seg, "+") {
		name := seg[1:]
		if name == "" {
			return f, fmt.Errorf("empty tag segment %q", seg)
		}
		for _, t := range f.Tags {
			if t == name {
				return f, fmt.Errorf("segment %q already in filter", seg)
			}
		}
		out := f.clone()
		out.Tags = append(out.Tags, name)
		return out, nil
	}
	if strings.HasPrefix(seg, ".") {
		return f, fmt.Errorf("dot-prefixed segment %q is reserved", seg)
	}
	if f.Project != "" {
		if seg == lastSegment(f.Project) {
			return f, fmt.Errorf("segment %q already in filter", seg)
		}
		out := f.clone()
		out.Project = out.Project + "." + seg
		return out, nil
	}
	out := f.clone()
	out.Project = seg
	return out, nil
}

// WriteFilter drops virtual-tag segments: virtual tags cannot be
// attached to tasks, mirroring taskwarrior's context write filters.
func (f Filter) WriteFilter() Filter {
	out := f.clone()
	out.VirtualTags = nil
	return out
}

// Render produces the taskwarrior filter arguments. A filter without a
// status virtual tag defaults to status:pending.
func (f Filter) Render() []string {
	args := f.criteriaArgs()
	if !f.HasStatus() {
		args = append(args, "status:pending")
	}
	return args
}

// RenderSweep renders the filter for a deletion sweep: the same
// criteria as Render but without the implicit status:pending default,
// and excluding already-deleted tasks, so pending and completed work
// both match.
func (f Filter) RenderSweep() []string {
	args := f.criteriaArgs()
	args = append(args, "status.not:deleted")
	return args
}

func (f Filter) criteriaArgs() []string {
	var args []string
	if f.Project != "" {
		args = append(args, "project:"+f.Project)
	}
	for _, t := range f.Tags {
		args = append(args, "+"+t)
	}
	for _, vt := range f.VirtualTags {
		if s, ok := StatusVirtualTag(vt); ok {
			args = append(args, s)
		} else {
			args = append(args, "+"+vt)
		}
	}
	return args
}

func (f Filter) clone() Filter {
	return Filter{
		Project:     f.Project,
		Tags:        append([]string(nil), f.Tags...),
		VirtualTags: append([]string(nil), f.VirtualTags...),
	}
}

func lastSegment(project string) string {
	if i := strings.LastIndex(project, "."); i >= 0 {
		return project[i+1:]
	}
	return project
}

func isUppercase(s string) bool {
	return s == strings.ToUpper(s)
}
