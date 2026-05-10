// Package state tracks tools that berserk has successfully installed.
//
// PATH lookup ("is `nmap` on $PATH") is unreliable as a source-of-truth for
// "did we install X" — many tools install scripts whose names don't match
// the catalog name. impacket, for example, installs as smbserver.py /
// secretsdump.py / etc., never as a binary named `impacket`. So we keep our
// own state file and only ever record entries on confirmed install success.
package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/thehackersbrain/berserk/internal/registry"
	"gopkg.in/yaml.v3"
)

// Entry is one row in the state file.
type Entry struct {
	Installer string `yaml:"installer"`
}

// State tracks tools installed by berserk. The mutex serializes Save calls
// when install runs in parallel — yaml.Marshal walks the Tools map, so we
// must not let writers mutate it during marshal.
//
// The mutex must NOT be in the same struct that gets passed to yaml.Marshal:
// the reflection walk inside encoding races with concurrent Lock/Unlock on the
// mutex's atomic state field even though the field is unexported. We keep a
// plain wire struct for serialization and expose accessors instead.
type State struct {
	mu    sync.Mutex
	tools map[string]Entry
}

type stateFile struct {
	Tools map[string]Entry `yaml:"tools"`
}

// Path returns the canonical state file path. Honors XDG_STATE_HOME per the
// XDG Base Directory Specification, with the standard ~/.local/state fallback.
func Path() (string, error) {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "berserk", "installed.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home dir: %w", err)
	}
	return filepath.Join(home, ".local", "state", "berserk", "installed.yaml"), nil
}

// Load reads state from disk. A missing file is not an error — first runs
// have no state yet, and we return an empty (but writable) State.
func Load() (*State, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	return loadFrom(path)
}

func loadFrom(path string) (*State, error) {
	s := &State{tools: map[string]Entry{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var wire stateFile
	if err := yaml.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if wire.Tools != nil {
		s.tools = wire.Tools
	}
	return s, nil
}

// Add records a successful install and persists immediately so a crash
// after install (but before the next state operation) doesn't lose the
// record. The disk write is atomic via temp file + rename.
func (s *State) Add(t registry.Tool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tools == nil {
		s.tools = map[string]Entry{}
	}
	s.tools[t.Name] = Entry{
		Installer: t.Installer,
	}
	return s.saveLocked()
}

// Remove drops a tool from state and persists. Removing a missing entry is
// a no-op — `berserk remove` for a tool we never installed shouldn't fail
// here just because the state file lags reality.
func (s *State) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tools[name]; !ok {
		return nil
	}
	delete(s.tools, name)
	return s.saveLocked()
}

// Has reports whether berserk has recorded an install for name.
func (s *State) Has(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.tools[name]
	return ok
}

// Names returns a snapshot of installed tool names. The returned map is the
// caller's to mutate — it's a copy, not the live one.
func (s *State) Names() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]bool, len(s.tools))
	for n := range s.tools {
		out[n] = true
	}
	return out
}

// Len returns the number of recorded installs.
func (s *State) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.tools)
}

// Entry returns the recorded entry for name, if any.
func (s *State) Entry(name string) (Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.tools[name]
	return e, ok
}

// saveLocked writes state to disk. Caller must hold s.mu.
func (s *State) saveLocked() error {
	path, err := Path()
	if err != nil {
		return err
	}
	return s.saveLockedTo(path)
}

func (s *State) saveLockedTo(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}

	data, err := yaml.Marshal(stateFile{Tools: s.tools})
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	// Atomic write: a crashed write must not leave a half-written state file
	// (which would deserialize as half the catalog missing).
	tmp, err := os.CreateTemp(filepath.Dir(path), ".installed-*.yaml")
	if err != nil {
		return fmt.Errorf("creating staging file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("writing staging file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("closing staging file: %w", err)
	}
	return os.Rename(tmp.Name(), path)
}
