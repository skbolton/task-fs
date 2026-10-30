package view

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"task-fs/internal/naming"
)

const ViewFileName = "_view"

// noView marks a view that has no mounted _view file. An empty
// definition file is a real, visible file and is stored as empty content.
const noView = "\x00"

var (
	ErrInvalidName = errors.New("invalid view name")
	ErrExists      = errors.New("view already exists")
	ErrNotExist    = errors.New("view does not exist")
)

// Store holds one definition file per runtime view.
type Store struct {
	dir string
}

// StoreDir resolves the user-scoped view directory: an absolute
// XDG_STATE_HOME wins, otherwise the conventional local state path.
func StoreDir() (string, error) {
	if stateHome := os.Getenv("XDG_STATE_HOME"); stateHome != "" && filepath.IsAbs(stateHome) {
		return filepath.Join(stateHome, "task-fs", "views"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve view store: %w", err)
	}
	if home == "" {
		return "", fmt.Errorf("resolve view store: neither XDG_STATE_HOME nor HOME is set")
	}
	return filepath.Join(home, ".local", "state", "task-fs", "views"), nil
}

// OpenStore resolves and creates the user's runtime view directory.
func OpenStore() (*Store, error) {
	dir, err := StoreDir()
	if err != nil {
		return nil, err
	}
	return NewStore(dir)
}

// NewStore creates dir if needed and returns a store rooted there.
func NewStore(dir string) (*Store, error) {
	if dir == "" {
		return nil, fmt.Errorf("view store directory is empty")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create view store %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, fmt.Errorf("secure view store %s: %w", dir, err)
	}
	store := &Store{dir: dir}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("migrate view store %s: %w", dir, err)
	}
	return store, nil
}

// migrate converts pre-empty-definition stores to the marker encoding once.
// An empty file used to mean "no definition"; now it means an empty one.
func (s *Store) migrate() error {
	tmpDir := filepath.Join(s.dir, ".tmp")
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return err
	}
	marker := filepath.Join(tmpDir, "layout-v2")
	if _, err := os.Stat(marker); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := ValidateName(entry.Name()); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() != 0 {
			continue
		}
		path := filepath.Join(s.dir, entry.Name())
		if err := os.WriteFile(path, []byte(noView), 0o600); err != nil {
			return err
		}
		if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
			return err
		}
	}
	return os.WriteFile(marker, nil, 0o600)
}

func (s *Store) Dir() string { return s.dir }

// ValidateName checks a view name, which is also its store file name.
func ValidateName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("%w: name is empty", ErrInvalidName)
	case len(name) > 254:
		return fmt.Errorf("%w: name is longer than 254 bytes", ErrInvalidName)
	case name == "." || name == "..":
		return fmt.Errorf("%w: name is reserved", ErrInvalidName)
	case strings.ContainsRune(name, '/'):
		return fmt.Errorf("%w: name contains '/'", ErrInvalidName)
	case strings.ContainsFunc(name, unicode.IsControl):
		return fmt.Errorf("%w: name contains a control character", ErrInvalidName)
	case naming.IsMintedName(name):
		return fmt.Errorf("%w: name looks like a task file", ErrInvalidName)
	}
	return nil
}

// List returns the stored view names in sorted order. Entries that
// cannot be view names are ignored so stray files do not mask the tree.
func (s *Store) List() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := ValidateName(entry.Name()); err != nil {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

// Get returns a view's raw definition text. ok is false when no definition
// exists; an invalid name is an error of its own.
func (s *Store) Get(name string) (definition string, ok bool, err error) {
	if err := ValidateName(name); err != nil {
		return "", false, err
	}
	data, err := os.ReadFile(filepath.Join(s.dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if string(data) == noView {
		return "", true, nil
	}
	return string(data), true, nil
}

// HasView reports whether a view has a mounted _view file.
func (s *Store) HasView(name string) (bool, error) {
	if err := ValidateName(name); err != nil {
		return false, err
	}
	data, err := os.ReadFile(filepath.Join(s.dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return string(data) != noView, nil
}

// Create adds a view with no definition and fails if one already exists.
func (s *Store) Create(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(s.dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%w: %s", ErrExists, name)
	}
	if err != nil {
		return err
	}
	if _, err := file.WriteString(noView); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// Put replaces a view definition atomically.
func (s *Store) Put(name, definition string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	tmpDir := filepath.Join(s.dir, ".tmp")
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(tmpDir, "view-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(definition); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filepath.Join(s.dir, name))
}

// ImportViews brings @NAME directories found in a mountpoint into the
// store. A directory without _view becomes a no-op view; a newer
// _view or directory replaces an older stored definition.
func (s *Store) ImportViews(mountpoint string) error {
	entries, err := os.ReadDir(mountpoint)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "@") {
			continue
		}
		name := strings.TrimPrefix(entry.Name(), "@")
		if err := ValidateName(name); err != nil {
			continue
		}
		sourceTime := int64(0)
		if info, err := entry.Info(); err == nil {
			sourceTime = info.ModTime().UnixNano()
		}

		definition := ""
		hasDefinition := false
		viewPath := filepath.Join(mountpoint, entry.Name(), ViewFileName)
		if info, err := os.Stat(viewPath); err == nil {
			if !info.Mode().IsRegular() {
				continue
			}
			data, err := os.ReadFile(viewPath)
			if err != nil {
				continue
			}
			hasDefinition = true
			definition = string(data)
			if modified := info.ModTime().UnixNano(); modified > sourceTime {
				sourceTime = modified
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			continue
		}

		storedInfo, storedErr := os.Stat(filepath.Join(s.dir, name))
		if storedErr != nil && !errors.Is(storedErr, os.ErrNotExist) {
			return storedErr
		}
		if errors.Is(storedErr, os.ErrNotExist) {
			if hasDefinition {
				if err := s.Put(name, definition); err != nil {
					return err
				}
			} else if err := s.Create(name); err != nil {
				return err
			}
			continue
		}
		if sourceTime <= storedInfo.ModTime().UnixNano() {
			continue
		}
		if hasDefinition {
			if err := s.Put(name, definition); err != nil {
				return err
			}
		} else if err := s.DeleteViewFile(name); err != nil {
			return err
		}
	}
	return nil
}

// DeleteViewFile removes a view's _view while keeping the view.
func (s *Store) DeleteViewFile(name string) error {
	return s.Put(name, noView)
}

// Delete removes a view definition.
func (s *Store) Delete(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	err := os.Remove(filepath.Join(s.dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %s", ErrNotExist, name)
	}
	return err
}
