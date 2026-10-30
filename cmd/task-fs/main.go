package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"

	"task-fs/internal/backend"
	"task-fs/internal/vfs"
	"task-fs/internal/view"
)

const usage = "usage: task-fs mount [--rm complete|delete] <mountpoint>"

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

	mountpoint, action, err := parseMountFlags(args[1:])
	if err != nil {
		return err
	}

	query := backend.NewQuery(backend.NewRunner("task", os.Environ()))
	if err := query.CheckAvailable(); err != nil {
		return err
	}
	store, err := openViewStore()
	if err != nil {
		return err
	}
	if err := store.ImportViews(mountpoint); err != nil {
		fmt.Fprintf(os.Stderr, "task-fs: import views from %s: %v\n", mountpoint, err)
	}
	shell := view.NewShellRunner(os.Environ(), view.DefaultTimeout)
	renderTemplate := newTemplateRenderer(shell)

	root := vfs.New(query, vfs.Config{
		RemoveAction:   action,
		ViewStore:      store,
		RenderTemplate: renderTemplate,
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

func openViewStore() (*view.Store, error) {
	return view.OpenStore()
}

func newTemplateRenderer(runner view.Runner) func(context.Context, string) (string, error) {
	return func(ctx context.Context, raw string) (string, error) {
		template, err := view.Parse(strings.TrimSpace(raw))
		if err != nil {
			return "", err
		}
		return template.Render(ctx, runner)
	}
}

// parseMountFlags parses the mount flags in any position relative to
// the mountpoint, which the flag package alone does not support.
func parseMountFlags(args []string) (mountpoint string, action backend.RemoveAction, err error) {
	var rmValue string
	flags := flag.NewFlagSet("mount", flag.ContinueOnError)
	flags.Usage = func() { fmt.Fprintln(os.Stderr, usage) }
	flags.StringVar(&rmValue, "rm", string(backend.RemoveComplete), "what removing a task does: complete or delete")
	if err := flags.Parse(reorderFlags(args)); err != nil {
		return "", "", err
	}
	rest := flags.Args()
	if len(rest) != 1 {
		return "", "", fmt.Errorf("%s", usage)
	}
	action, err = backend.ParseRemoveAction(rmValue)
	if err != nil {
		return "", "", err
	}
	return rest[0], action, nil
}

// reorderFlags moves flag tokens ahead of positional arguments so
// `mount <mp> --rm delete` parses. Only the value-taking flags below
// need their argument pulled along; everything else is position
// independent.
func reorderFlags(args []string) []string {
	valueFlags := map[string]bool{"--rm": true}
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
