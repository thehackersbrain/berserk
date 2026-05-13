// Package docker handles the docker related things
// keeping the docker and tools registry seperate
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

// LoadFile reads a docker catalog YAML file (top-level list of Container entries)
// and returns the parsed containers.
func LoadFile(path string) ([]Container, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var containers []Container
	if err := yaml.Unmarshal(data, &containers); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	return containers, nil
}

// LoadDir reads every *.yaml / *.yml file in dir and merges their Container lists
// in lexical order. Returns nil containers (no error) when dir does not exist —
// callers in search/list context degrade gracefully without a catalog.
func LoadDir(dir string) ([]Container, error) {
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
		return nil, nil
	}

	var all []Container
	for _, p := range paths {
		containers, err := LoadFile(p)
		if err != nil {
			return nil, err
		}
		all = append(all, containers...)
	}
	return all, nil
}

// Validate checks that every container's categories are present in the allowed
// list. Returns a joined error of all violations.
func Validate(containers []Container, allowed []string) error {
	if len(allowed) == 0 {
		return nil
	}
	valid := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		valid[a] = true
	}

	var errs []error
	for _, c := range containers {
		for _, cat := range c.Category {
			if !valid[cat] {
				errs = append(errs, fmt.Errorf("container %q references unknown category %q (add it to categories.yaml)", c.Name, cat))
			}
		}
	}
	return errors.Join(errs...)
}

