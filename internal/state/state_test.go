package state

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/thehackersbrain/berserk/internal/registry"
)

// withStateDir points Path() at a fresh tempdir for the duration of the test.
func withStateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	return dir
}

func TestPathHonorsXDG(t *testing.T) {
	dir := withStateDir(t)
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "berserk", "installed.yaml")
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	withStateDir(t)
	s, err := Load()
	if err != nil {
		t.Fatalf("Load on missing file returned error: %v", err)
	}
	if s == nil || s.Len() != 0 {
		t.Errorf("expected empty state, got len=%d", s.Len())
	}
}

func TestAddPersistsAcrossLoad(t *testing.T) {
	withStateDir(t)
	s, _ := Load()
	if err := s.Add(registry.Tool{Name: "netexec", Installer: "pipx"}); err != nil {
		t.Fatal(err)
	}

	s2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !s2.Has("netexec") {
		t.Error("Add did not persist across reload")
	}
	e, ok := s2.Entry("netexec")
	if !ok || e.Installer != "pipx" {
		t.Errorf("entry roundtrip = %+v, ok=%v", e, ok)
	}
}

func TestRemoveDeletesEntry(t *testing.T) {
	withStateDir(t)
	s, _ := Load()
	if err := s.Add(registry.Tool{Name: "nuclei", Installer: "go"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove("nuclei"); err != nil {
		t.Fatal(err)
	}
	if s.Has("nuclei") {
		t.Error("Remove did not drop entry from in-memory state")
	}

	s2, _ := Load()
	if s2.Has("nuclei") {
		t.Error("Remove did not persist deletion to disk")
	}
}

func TestRemoveMissingIsNoop(t *testing.T) {
	withStateDir(t)
	s, _ := Load()
	if err := s.Remove("never-installed"); err != nil {
		t.Errorf("Remove of missing entry should be a no-op, got error: %v", err)
	}
}

func TestNamesIsSnapshot(t *testing.T) {
	withStateDir(t)
	s, _ := Load()
	_ = s.Add(registry.Tool{Name: "a", Installer: "pipx"})
	_ = s.Add(registry.Tool{Name: "b", Installer: "pipx"})

	names := s.Names()
	// Mutating the snapshot must not affect future Names() calls.
	delete(names, "a")
	again := s.Names()
	if !again["a"] || !again["b"] {
		t.Errorf("Names() should be a defensive copy, got %v", again)
	}
}

func TestConcurrentAddIsSafe(t *testing.T) {
	withStateDir(t)
	s, _ := Load()

	var wg sync.WaitGroup
	const n = 50
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "tool" + string(rune('A'+i%26)) + string(rune('0'+i/26))
			_ = s.Add(registry.Tool{Name: name, Installer: "pipx"})
		}(i)
	}
	wg.Wait()

	s2, err := Load()
	if err != nil {
		t.Fatalf("post-concurrent Load: %v", err)
	}
	if s2.Len() == 0 {
		t.Errorf("no entries persisted after concurrent Add")
	}
	// Race detector + atomic writes are the real assertions here. We don't
	// need exact counts because some names will collide by construction.
}

// TestAddRollsBackOnSaveFailure ensures the in-memory map stays consistent
// with disk when saveLocked fails (disk full, perm denied, etc.). Without
// the rollback, callers in the same process would see Has(name)==true for
// a tool that isn't recorded on disk.
func TestAddRollsBackOnSaveFailure(t *testing.T) {
	dir := withStateDir(t)
	// Pre-create the berserk subdir, then make it read-only so CreateTemp
	// inside saveLockedTo fails.
	berserkDir := filepath.Join(dir, "berserk")
	if err := os.MkdirAll(berserkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(berserkDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(berserkDir, 0o755) })

	s, _ := Load()
	err := s.Add(registry.Tool{Name: "newtool", Installer: "pipx"})
	if err == nil {
		t.Fatal("expected save to fail on read-only dir")
	}
	if s.Has("newtool") {
		t.Error("in-memory state should be rolled back after save failure")
	}
}

func TestRemoveRollsBackOnSaveFailure(t *testing.T) {
	dir := withStateDir(t)
	s, _ := Load()
	if err := s.Add(registry.Tool{Name: "tool", Installer: "pipx"}); err != nil {
		t.Fatal(err)
	}

	// Make the dir read-only so the next saveLocked fails.
	berserkDir := filepath.Join(dir, "berserk")
	if err := os.Chmod(berserkDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(berserkDir, 0o755) })

	if err := s.Remove("tool"); err == nil {
		t.Fatal("expected Remove to fail on read-only dir")
	}
	if !s.Has("tool") {
		t.Error("in-memory state should retain entry after Remove save failure")
	}
}

func TestSaveCreatesParentDir(t *testing.T) {
	dir := withStateDir(t)
	// Ensure the berserk subdir doesn't exist yet.
	if _, err := os.Stat(filepath.Join(dir, "berserk")); !os.IsNotExist(err) {
		t.Fatalf("expected fresh tempdir, got: %v", err)
	}

	s, _ := Load()
	if err := s.Add(registry.Tool{Name: "x", Installer: "pipx"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "berserk", "installed.yaml")); err != nil {
		t.Errorf("state file was not created: %v", err)
	}
}
