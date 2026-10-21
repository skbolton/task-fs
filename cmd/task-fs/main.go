package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"

	"task-fs/internal/backend"
	"task-fs/internal/vfs"
)

const usage = "usage: task-fs mount <mountpoint>"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 || args[0] != "mount" {
		return fmt.Errorf("%s", usage)
	}
	mountpoint := args[1]

	query := backend.NewQuery(backend.NewRunner("task", os.Environ()))
	if err := query.CheckAvailable(); err != nil {
		return err
	}
	entries, err := query.AllTags()
	if err != nil {
		return err
	}

	root := vfs.New(query)
	root.SetVirtualTags(backend.VirtualTagsFromDiscovery(entries))

	server, err := fs.Mount(mountpoint, root, &fs.Options{})
	if err != nil {
		return err
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	<-sig
	return server.Unmount()
}
