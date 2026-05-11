package docker

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFile reads a docker catalog YAML file (top-level list of Group entries)
// and returns the parsed groups.
func LoadFile(path string) ([]Group, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var groups []Group
	if err := yaml.Unmarshal(data, &groups); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	return groups, nil
}

// LoadDir reads every *.yaml / *.yml file in dir and merges their Group lists
// in lexical order. Returns nil groups (no error) when dir does not exist —
// callers in search/list context degrade gracefully without a catalog.
func LoadDir(dir string) ([]Group, error) {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walking containers dir: %w", err)
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading containers dir %s: %w", dir, err)
	}
	if len(paths) == 0 {
		// Treat an empty dir the same as a missing dir — both mean the
		// catalog hasn't been populated yet. Callers handle nil groups
		// with their own "no catalog" message.
		return nil, nil
	}

	var all []Group
	for _, p := range paths {
		groups, err := LoadFile(p)
		if err != nil {
			return nil, err
		}
		all = append(all, groups...)
	}
	return all, nil
}
