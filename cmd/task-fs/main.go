package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"

	"task-fs/internal/backend"
	"task-fs/internal/vfs"
)

const usage = "usage: task-fs mount [--virtual-tags LIST | -t LIST] [--virtual-hidden | -H] <mountpoint>"

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

	mountpoint, selection, hidden, err := parseMountFlags(args[1:])
	if err != nil {
		return err
	}

	query := backend.NewQuery(backend.NewRunner("task", os.Environ()))
	if err := query.CheckAvailable(); err != nil {
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

	root := vfs.New(query, vfs.Config{VirtualTags: selected, Hidden: hidden})
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
func parseMountFlags(args []string) (mountpoint, selection string, hidden bool, err error) {
	flags := flag.NewFlagSet("mount", flag.ContinueOnError)
	flags.Usage = func() { fmt.Fprintln(os.Stderr, usage) }
	flags.StringVar(&selection, "virtual-tags", "",
		"comma-separated virtual tags to expose without the + sign, or ALL (default: "+backend.DefaultVirtualTagSelection+")")
	flags.StringVar(&selection, "t", "", "shorthand for --virtual-tags")
	flags.BoolVar(&hidden, "virtual-hidden", false, "present virtual-tag directories as hidden .+NAME entries")
	flags.BoolVar(&hidden, "H", false, "shorthand for --virtual-hidden")
	if err := flags.Parse(reorderFlags(args)); err != nil {
		return "", "", false, err
	}
	rest := flags.Args()
	if len(rest) != 1 {
		return "", "", false, fmt.Errorf("%s", usage)
	}
	return rest[0], selection, hidden, nil
}

// reorderFlags moves flag tokens ahead of positional arguments so
// `mount <mp> --virtual-tags ALL` parses. Only the value-taking flags
// below need their argument pulled along; everything else is position
// independent.
func reorderFlags(args []string) []string {
	valueFlags := map[string]bool{"-t": true, "--virtual-tags": true}
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
