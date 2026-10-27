package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"unicode"

	"github.com/hanwen/go-fuse/v2/fs"

	"task-fs/internal/backend"
	"task-fs/internal/naming"
	"task-fs/internal/vfs"
	"task-fs/internal/view"
)

const usage = "usage: task-fs mount [--virtual-tags LIST | -t LIST] [--virtual-hidden | -H] [--rm complete|delete] [--view NAME=FILTER ...] <mountpoint>"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] != "mount" {
		return fmt.Errorf("%s", usage)
	}

	mountpoint, selection, hidden, action, views, err := parseMountFlags(args[1:])
	if err != nil {
		return err
	}

	query := backend.NewQuery(backend.NewRunner("task", os.Environ()))
	if err := query.CheckAvailable(); err != nil {
		return err
	}
	shell := view.NewShellRunner(os.Environ(), view.DefaultTimeout)
	renderView := views.renderer(shell)
	if err := validateViews(query, views, renderView); err != nil {
		return err
	}
	entries, err := query.AllTags()
	if err != nil {
		return err
	}
	discovered := backend.VirtualTagsFromDiscovery(entries)
	var selected []string
	if selection == "" {
		selected = backend.DefaultVirtualTags(discovered)
	} else {
		selected, err = backend.ResolveVirtualTags(selection, discovered)
		if err != nil {
			return err
		}
	}

	root := vfs.New(query, vfs.Config{
		VirtualTags:  selected,
		Hidden:       hidden,
		RemoveAction: action,
		Views:        views.views,
		RenderView:   renderView,
	})
	server, err := fs.Mount(mountpoint, root, &fs.Options{})
	if err != nil {
		return err
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	<-sig
	return server.Unmount()
}

// parseMountFlags parses the mount flags in any position relative to
// the mountpoint, which the flag package alone does not support.
func parseMountFlags(args []string) (mountpoint, selection string, hidden bool, action backend.RemoveAction, views compiledViews, err error) {
	var rmValue string
	var viewValues viewFlag
	flags := flag.NewFlagSet("mount", flag.ContinueOnError)
	flags.Usage = func() { fmt.Fprintln(os.Stderr, usage) }
	flags.StringVar(&selection, "virtual-tags", "",
		"comma-separated virtual tags to expose without the + sign, or ALL (default: "+backend.DefaultVirtualTagSelection+")")
	flags.StringVar(&selection, "t", "", "shorthand for --virtual-tags")
	flags.BoolVar(&hidden, "virtual-hidden", false, "present virtual-tag directories as hidden .+NAME entries")
	flags.BoolVar(&hidden, "H", false, "shorthand for --virtual-hidden")
	flags.StringVar(&rmValue, "rm", string(backend.RemoveComplete), "what removing a task does: complete or delete")
	flags.Var(&viewValues, "view", "repeatable saved view as NAME=FILTER; {{ command }} blocks render when the view is browsed")
	if err := flags.Parse(reorderFlags(args)); err != nil {
		return "", "", false, "", compiledViews{}, err
	}
	rest := flags.Args()
	if len(rest) != 1 {
		return "", "", false, "", compiledViews{}, fmt.Errorf("%s", usage)
	}
	action, err = backend.ParseRemoveAction(rmValue)
	if err != nil {
		return "", "", false, "", compiledViews{}, err
	}
	views, err = parseViews(viewValues)
	if err != nil {
		return "", "", false, "", compiledViews{}, err
	}
	return rest[0], selection, hidden, action, views, nil
}

type viewFlag []string

func (q *viewFlag) String() string { return strings.Join(*q, ", ") }

func (q *viewFlag) Set(value string) error {
	*q = append(*q, value)
	return nil
}

type compiledViews struct {
	views     []vfs.View
	templates map[string]*view.Template
}

func (c compiledViews) renderer(runner view.Runner) func(context.Context, vfs.View) (string, error) {
	return func(ctx context.Context, def vfs.View) (string, error) {
		template, ok := c.templates[def.Name]
		if !ok {
			return def.Filter, nil
		}
		return template.Render(ctx, runner)
	}
}

func parseViews(values []string) (compiledViews, error) {
	var views compiledViews
	views.templates = map[string]*view.Template{}
	seen := map[string]bool{}
	for _, value := range values {
		name, filter, ok := strings.Cut(value, "=")
		if !ok {
			return compiledViews{}, fmt.Errorf("invalid view %q: expected NAME=FILTER", value)
		}
		name = strings.TrimSpace(strings.TrimPrefix(name, "@"))
		if err := validateViewName(name); err != nil {
			return compiledViews{}, fmt.Errorf("invalid view %q: %w", value, err)
		}
		if seen[name] {
			return compiledViews{}, fmt.Errorf("duplicate view name %q", name)
		}
		seen[name] = true
		if strings.TrimSpace(filter) == "" {
			return compiledViews{}, fmt.Errorf("invalid view %q: empty filter", value)
		}
		template, err := view.Parse(filter)
		if err != nil {
			return compiledViews{}, fmt.Errorf("invalid view %q: %w", value, err)
		}
		views.views = append(views.views, vfs.View{Name: name, Filter: filter})
		views.templates[name] = template
	}
	return views, nil
}

func validateViewName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("name is empty")
	case len(name) > 254:
		return fmt.Errorf("name is longer than 254 bytes")
	case name == "." || name == "..":
		return fmt.Errorf("name is reserved")
	case strings.ContainsRune(name, '/'):
		return fmt.Errorf("name contains '/'")
	case strings.ContainsFunc(name, unicode.IsControl):
		return fmt.Errorf("name contains a control character")
	case naming.IsMintedName(name):
		return fmt.Errorf("name looks like a task file")
	}
	return nil
}

func validateViews(query *backend.Query, views compiledViews, render func(context.Context, vfs.View) (string, error)) error {
	for _, def := range views.views {
		filter := def.Filter
		if render != nil {
			rendered, err := render(context.Background(), def)
			if err != nil {
				return fmt.Errorf("view %q: %w", def.Name, err)
			}
			filter = rendered
		}
		if _, err := query.ExportQuery(filter); err != nil {
			return fmt.Errorf("view %q: %w", def.Name, err)
		}
	}
	return nil
}

// reorderFlags moves flag tokens ahead of positional arguments so
// `mount <mp> --virtual-tags ALL` parses. Only the value-taking flags
// below need their argument pulled along; everything else is position
// independent.
func reorderFlags(args []string) []string {
	valueFlags := map[string]bool{"-t": true, "--virtual-tags": true, "--rm": true, "--view": true}
	var flagArgs, rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			rest = append(rest, a)
			continue
		}
		flagArgs = append(flagArgs, a)
		if valueFlags[a] && !strings.Contains(a, "=") && i+1 < len(args) {
			flagArgs = append(flagArgs, args[i+1])
			i++
		}
	}
	return append(flagArgs, rest...)
}
