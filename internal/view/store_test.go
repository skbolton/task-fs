package view

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreDir(t *testing.T) {
	t.Run("absolute state home", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "/state")
		t.Setenv("HOME", "/home/me")
		got, err := StoreDir()
		if err != nil {
			t.Fatalf("StoreDir: %v", err)
		}
		if want := "/state/task-fs/views"; got != want {
			t.Errorf("StoreDir = %q, want %q", got, want)
		}
	})

	t.Run("relative state home is ignored", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "state")
		t.Setenv("HOME", "/home/me")
		got, err := StoreDir()
		if err != nil {
			t.Fatalf("StoreDir: %v", err)
		}
		if want := "/home/me/.local/state/task-fs/views"; got != want {
			t.Errorf("StoreDir = %q, want %q", got, want)
		}
	})

	t.Run("home fallback", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "")
		t.Setenv("HOME", "/home/me")
		got, err := StoreDir()
		if err != nil {
			t.Fatalf("StoreDir: %v", err)
		}
		if want := "/home/me/.local/state/task-fs/views"; got != want {
			t.Errorf("StoreDir = %q, want %q", got, want)
		}
	})

	t.Run("unresolvable", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "")
		t.Setenv("HOME", "")
		if _, err := StoreDir(); err == nil {
			t.Fatal("StoreDir with no HOME succeeded")
		}
	})
}

func TestNewStoreCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "deep", "task-fs", "views")
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if store.Dir() != dir {
		t.Errorf("Dir = %q, want %q", store.Dir(), dir)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat store: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("store permissions = %o, want 700", perm)
	}
}

func TestStoreLifecyclePersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.Create("Due"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	filter, ok, err := store.Get("Due")
	if err != nil || !ok || filter != "" {
		t.Fatalf("Get new view = %q, %v, %v; want empty filter", filter, ok, err)
	}
	if err := store.Put("Due", "status:pending +next"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := store.Create("Zulu"); err != nil {
		t.Fatalf("Create Zulu: %v", err)
	}
	if err := store.Create("Alpha"); err != nil {
		t.Fatalf("Create Alpha: %v", err)
	}

	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore reopen: %v", err)
	}
	filter, ok, err = reopened.Get("Due")
	if err != nil || !ok || filter != "status:pending +next" {
		t.Fatalf("Get after reopen = %q, %v, %v", filter, ok, err)
	}
	names, err := reopened.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := strings.Join(names, ","); got != "Alpha,Due,Zulu" {
		t.Errorf("List = %q, want sorted names", got)
	}

	if err := reopened.Delete("Due"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, err := reopened.Get("Due"); err != nil || ok {
		t.Fatalf("Get deleted = %v, %v; want missing", ok, err)
	}
	if err := reopened.Delete("Due"); !errors.Is(err, ErrNotExist) {
		t.Errorf("Delete missing error = %v, want ErrNotExist", err)
	}
}

func TestStoreCreateReportsExistingName(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.Create("Due"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Create("Due"); !errors.Is(err, ErrExists) {
		t.Errorf("duplicate Create error = %v, want ErrExists", err)
	}
}

func TestStoreValidateName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"Due", true},
		{"Due Next Week", true},
		{"@Due", true},
		{"under_score", true},
		{"a", true},
		{strings.Repeat("a", 254), true},
		{"", false},
		{strings.Repeat("a", 255), false},
		{".", false},
		{"..", false},
		{"a/b", false},
		{"bad\tname", false},
		{"bad\nname", false},
		{"Report.a1b2c3d4.md", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.name)
			if tt.valid && err != nil {
				t.Errorf("ValidateName(%q) = %v, want nil", tt.name, err)
			}
			if !tt.valid && !errors.Is(err, ErrInvalidName) {
				t.Errorf("ValidateName(%q) = %v, want ErrInvalidName", tt.name, err)
			}
		})
	}
}

func TestStoreFilterTextRoundTrips(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	filters := map[string]string{
		"Spaces": "status:pending project:home or project:finances",
		"Blocks": "status:pending due.before:{{ date +%F }}",
		"At":     "@literal status:pending",
		"Lines":  "status:pending\n+next\n",
	}
	for name, filter := range filters {
		if err := store.Put(name, filter); err != nil {
			t.Fatalf("Put %q: %v", name, err)
		}
	}
	for name, want := range filters {
		got, ok, err := store.Get(name)
		if err != nil || !ok || got != want {
			t.Errorf("Get %q = %q, %v, %v; want %q", name, got, ok, err, want)
		}
	}
}

func TestStorePutFailureKeepsPreviousDefinition(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.Put("Due", "status:pending"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	tmpDir := filepath.Join(dir, ".tmp")
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if err := os.Chmod(tmpDir, 0o500); err != nil {
		t.Fatalf("chmod tmp: %v", err)
	}
	defer os.Chmod(dir, 0o700)

	if err := store.Put("Due", "status:completed"); err == nil {
		t.Fatal("Put into read-only store succeeded")
	}
	if err := os.Chmod(tmpDir, 0o700); err != nil {
		t.Fatalf("chmod tmp back: %v", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod back: %v", err)
	}
	filter, ok, err := store.Get("Due")
	if err != nil || !ok || filter != "status:pending" {
		t.Errorf("Get after failed Put = %q, %v, %v; want previous filter", filter, ok, err)
	}
}

func TestStoreListIgnoresNonViews(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.Put("Due", "status:pending"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o700); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Report.a1b2c3d4.md"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write junk: %v", err)
	}
	names, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := strings.Join(names, ","); got != "Due" {
		t.Errorf("List = %q, want only Due", got)
	}
}

func TestStoreImportViews(t *testing.T) {
	mount := t.TempDir()
	for _, dir := range []string{"@Due", "@Empty", "@Blank", "@", "@Report.a1b2c3d4.md", "plain"} {
		if err := os.Mkdir(filepath.Join(mount, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(mount, "@Due", FilterFileName), []byte("status:pending +next"), 0o600); err != nil {
		t.Fatalf("write filter: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mount, "@Blank", FilterFileName), nil, 0o600); err != nil {
		t.Fatalf("write empty filter: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mount, "@file"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write @file: %v", err)
	}

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.ImportViews(mount); err != nil {
		t.Fatalf("ImportViews: %v", err)
	}
	names, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := strings.Join(names, ","); got != "Blank,Due,Empty" {
		t.Errorf("List after import = %q, want Blank,Due,Empty", got)
	}
	if filter, ok, err := store.Get("Due"); err != nil || !ok || filter != "status:pending +next" {
		t.Errorf("imported Due = %q, %v, %v", filter, ok, err)
	}
	if filter, ok, err := store.Get("Empty"); err != nil || !ok || filter != "" {
		t.Errorf("imported Empty = %q, %v, %v; want empty no-op", filter, ok, err)
	}
	if has, err := store.HasFilter("Empty"); err != nil || has {
		t.Errorf("imported Empty HasFilter = %v, %v; want false", has, err)
	}
	if has, err := store.HasFilter("Blank"); err != nil || !has {
		t.Errorf("imported Blank HasFilter = %v, %v; want true", has, err)
	}
}

func TestStoreImportViewsUsesNewestDefinition(t *testing.T) {
	mount := t.TempDir()
	dueDir := filepath.Join(mount, "@Due")
	if err := os.Mkdir(dueDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	filterPath := filepath.Join(dueDir, FilterFileName)
	if err := os.WriteFile(filterPath, []byte("project:home"), 0o600); err != nil {
		t.Fatalf("write filter: %v", err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(filterPath, old, old); err != nil {
		t.Fatalf("chtimes filter: %v", err)
	}
	if err := os.Chtimes(dueDir, old, old); err != nil {
		t.Fatalf("chtimes dir: %v", err)
	}

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.Put("Due", "status:pending"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := store.ImportViews(mount); err != nil {
		t.Fatalf("ImportViews old source: %v", err)
	}
	if filter, _, _ := store.Get("Due"); filter != "status:pending" {
		t.Errorf("older source replaced newer store: %q", filter)
	}

	future := time.Now().Add(2 * time.Hour)
	if err := os.WriteFile(filterPath, []byte("project:finances"), 0o600); err != nil {
		t.Fatalf("rewrite filter: %v", err)
	}
	if err := os.Chtimes(filterPath, future, future); err != nil {
		t.Fatalf("chtimes filter future: %v", err)
	}
	if err := store.ImportViews(mount); err != nil {
		t.Fatalf("ImportViews new source: %v", err)
	}
	if filter, _, _ := store.Get("Due"); filter != "project:finances" {
		t.Errorf("newer source did not replace store: %q", filter)
	}

	if err := os.Remove(filterPath); err != nil {
		t.Fatalf("remove filter: %v", err)
	}
	dirFuture := future.Add(time.Hour)
	if err := os.Chtimes(dueDir, dirFuture, dirFuture); err != nil {
		t.Fatalf("chtimes dir future: %v", err)
	}
	if err := store.ImportViews(mount); err != nil {
		t.Fatalf("ImportViews removed filter: %v", err)
	}
	if filter, ok, err := store.Get("Due"); err != nil || !ok || filter != "" {
		t.Errorf("removed source filter = %q, %v, %v; want empty no-op", filter, ok, err)
	}
}

func TestStoreEmptyFilterIsPresent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.Create("Due"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if has, err := store.HasFilter("Due"); err != nil || has {
		t.Fatalf("new view HasFilter = %v, %v; want false", has, err)
	}
	if err := store.Put("Due", ""); err != nil {
		t.Fatalf("Put empty: %v", err)
	}
	if has, err := store.HasFilter("Due"); err != nil || !has {
		t.Fatalf("empty filter HasFilter = %v, %v; want true", has, err)
	}
	if filter, ok, err := store.Get("Due"); err != nil || !ok || filter != "" {
		t.Fatalf("empty filter Get = %q, %v, %v", filter, ok, err)
	}

	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore reopen: %v", err)
	}
	if has, err := reopened.HasFilter("Due"); err != nil || !has {
		t.Errorf("empty filter after reopen HasFilter = %v, %v; want true", has, err)
	}
	if err := reopened.DeleteFilter("Due"); err != nil {
		t.Fatalf("DeleteFilter: %v", err)
	}
	if has, err := reopened.HasFilter("Due"); err != nil || has {
		t.Errorf("deleted filter HasFilter = %v, %v; want false", has, err)
	}
	if filter, ok, err := reopened.Get("Due"); err != nil || !ok || filter != "" {
		t.Errorf("deleted filter Get = %q, %v, %v; want empty view", filter, ok, err)
	}
}

func TestStoreMigrationPreservesTimeForImport(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "Due")
	if err := os.WriteFile(legacy, nil, 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(legacy, old, old); err != nil {
		t.Fatalf("chtimes legacy: %v", err)
	}
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if has, err := store.HasFilter("Due"); err != nil || has {
		t.Fatalf("legacy HasFilter = %v, %v; want false", has, err)
	}

	mount := t.TempDir()
	if err := os.Mkdir(filepath.Join(mount, "@Due"), 0o755); err != nil {
		t.Fatalf("mkdir mount view: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mount, "@Due", FilterFileName), []byte("status:pending"), 0o600); err != nil {
		t.Fatalf("write mount filter: %v", err)
	}
	if err := store.ImportViews(mount); err != nil {
		t.Fatalf("ImportViews: %v", err)
	}
	if filter, ok, err := store.Get("Due"); err != nil || !ok || filter != "status:pending" {
		t.Errorf("imported filter = %q, %v, %v", filter, ok, err)
	}
}

func TestStoreMigratesLegacyEmptyAsNoFilter(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Due"), nil, 0o600); err != nil {
		t.Fatalf("write empty legacy view: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Other"), []byte("status:pending"), 0o600); err != nil {
		t.Fatalf("write populated legacy view: %v", err)
	}
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if has, err := store.HasFilter("Due"); err != nil || has {
		t.Errorf("legacy empty HasFilter = %v, %v; want false", has, err)
	}
	if has, err := store.HasFilter("Other"); err != nil || !has {
		t.Errorf("legacy populated HasFilter = %v, %v; want true", has, err)
	}
	if err := store.Put("Fresh", ""); err != nil {
		t.Fatalf("Put empty after migration: %v", err)
	}
	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore reopen: %v", err)
	}
	if has, err := reopened.HasFilter("Fresh"); err != nil || !has {
		t.Errorf("post-migration empty filter HasFilter = %v, %v; want true", has, err)
	}
}
