// Package registry parses berserk's yaml-based tool catalog and exposes it as
// a queryable Registry. The schema splits responsibilities across files inside
// the config dir:
//
//   - tools.yaml (or any *.yaml/*.yml that isn't config.yaml): tool entries.
//     Each tool may declare which profiles and categories it belongs to via
//     `profiles:` and `category:` lists.
//   - profiles.yaml: master list of valid profile names plus optional
//     `includes:` for composition (one profile rolling up another's members).
//   - categories.yaml: master list of valid category names.
//
// Validation enforces the contract: every name used in a tool's `profiles:` or
// `category:` field must be declared in profiles.yaml or categories.yaml
// respectively. Composition (`Profile.Includes`) goes through the same
// declaration check and is checked for cycles.
package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Tool struct {
	Name          string   `yaml:"name"`
	Description   string   `yaml:"description"`
	Category      []string `yaml:"category"`
	Profiles      []string `yaml:"profiles"`
	Aliases       []string `yaml:"aliases"`
	Installer     string   `yaml:"installer"`
	Repo          string   `yaml:"repo"`
	Branch        string   `yaml:"branch"`
	EntryScript   string   `yaml:"entry_script"`
	Runtime       string   `yaml:"runtime"`
	PipDeps       []string `yaml:"pip_deps"`
	Package       string   `yaml:"package"`
	PythonVersion string   `yaml:"python_version"`
	AssetPattern  string   `yaml:"asset_pattern"`
	ArchPackage   string   `yaml:"arch_package"`
	DebianPackage string   `yaml:"debian_package"`
	Steps         []string `yaml:"steps"`

	// Depends names native system packages required to install this tool
	// (build prereqs for cargo/pipx tools, runtime libs for binary tools).
	// Keys are distro families: "arch" (pacman), "debian" (apt — also
	// used for Kali and Parrot). Installed via the system pkg mgr before
	// the main install runs and NOT recorded in berserk's state file —
	// they're prereqs, not berserk-managed tools.
	Depends map[string][]string `yaml:"depends"`
}

// Profile is one declaration in profiles.yaml. Tools opt into a profile by
// listing its Name in their own `profiles:` field; Includes lets a profile
// roll up the members of other profiles (e.g. red-team includes ad-attacks +
// web + post-exploitation + credentials), avoiding per-tool duplication.
type Profile struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Includes    []string `yaml:"includes"`
}

// Category is one declaration in categories.yaml. Categories are tags applied
// to tools (a tool can belong to several). Their job is to give `search -c X`
// a controlled vocabulary; `category:` values on tools must reference one of
// these declarations.
type Category struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type Config struct {
	GithubToken string `yaml:"github_token"`
	InstallDir  string `yaml:"install_dir"`
	Parallel    bool   `yaml:"parallel"`
	Verbose     bool   `yaml:"verbose"`
	// DockerDataDir overrides the root directory where berserk stores
	// container volume mounts (subdir "containers/") and docker bind
	// mounts (subdir "docker/"). Empty means default to ~/berserk.
	// Supports a leading "~/" — expanded against $HOME at use time.
	DockerDataDir string `yaml:"docker_data_dir"`
}

type Registry struct {
	// Config is loaded by LoadDir from `<dir>/config.yaml` (flat keys, no
	// `config:` wrapper) and merged on top of an in-code default. Single-file
	// Load still accepts a wrapped `config:` block for the test fixtures.
	Config     Config     `yaml:"config"`
	Tools      []Tool     `yaml:"tools"`
	Profiles   []Profile  `yaml:"profiles"`
	Categories []Category `yaml:"categories"`

	// members holds the resolved profile membership map (name → tool names)
	// after Includes-composition rolls up. Computed once in resolveProfiles.
	members map[string][]string
}

// Load parses a single yaml file containing tools, profiles, and categories.
// Useful for tests and for callers that have everything in one file. Production
// code uses LoadDir to merge a config directory.
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	expanded := os.ExpandEnv(string(data))

	var reg Registry
	if err := yaml.Unmarshal([]byte(expanded), &reg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if reg.Config.InstallDir == "" {
		reg.Config.InstallDir = "/usr/local/bin"
	}

	if err := reg.Validate(); err != nil {
		return nil, err
	}
	if err := reg.resolveProfiles(); err != nil {
		return nil, err
	}

	return &reg, nil
}

// LoadDir reads every *.yaml / *.yml file in dir and all subdirectories
// and merges the tools, profiles, and categories sections into one
// Registry. Cross-file duplicates (same tool name, alias, profile name,
// or category name) are errors so silent collisions surface immediately.
// Files are merged in lexical order (depth-first walk) for determinism.
//
// config.yaml is special-cased: instead of being parsed as a catalog
// (which would have to wrap its keys under `config:` to avoid the
// validator complaining about a tool entry with no name), it is read
// once at top-level into Registry.Config using the flat schema users
// actually write.
func LoadDir(dir string) (*Registry, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walking config dir: %w", err)
		}
		if d.IsDir() {
			// containers/ holds the docker catalog (YAML lists, not registry maps).
			if d.Name() == "containers" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		base := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		if base == "config" {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading config dir %s: %w", dir, err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no tool yaml files found in %s (looking for *.yaml/*.yml other than config.yaml)", dir)
	}

	sort.Strings(paths)

	merged := &Registry{}

	// Load config.yaml (flat schema, optional). Catalog files never carry
	// runtime config so this is the one file we read separately, before
	// folding everything else in.
	configPath := filepath.Join(dir, "config.yaml")
	if data, err := os.ReadFile(configPath); err == nil {
		expanded := os.ExpandEnv(string(data))
		var cfg Config
		if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", configPath, err)
		}
		merged.Config = cfg
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("reading %s: %w", configPath, err)
	}

	seenTool := map[string]string{}
	seenProfile := map[string]string{}
	seenCategory := map[string]string{}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", p, err)
		}
		expanded := os.ExpandEnv(string(data))

		var part Registry
		if err := yaml.Unmarshal([]byte(expanded), &part); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", p, err)
		}

		for _, t := range part.Tools {
			if where, dup := seenTool[t.Name]; dup {
				return nil, fmt.Errorf("tool %q in %s collides with definition in %s", t.Name, p, where)
			}
			seenTool[t.Name] = p
			for _, a := range t.Aliases {
				if where, dup := seenTool[a]; dup {
					return nil, fmt.Errorf("alias %q on tool %q in %s collides with %s", a, t.Name, p, where)
				}
				seenTool[a] = p
			}
			merged.Tools = append(merged.Tools, t)
		}

		for _, prof := range part.Profiles {
			if where, dup := seenProfile[prof.Name]; dup {
				return nil, fmt.Errorf("profile %q redefined in %s (first defined in %s)", prof.Name, p, where)
			}
			seenProfile[prof.Name] = p
			merged.Profiles = append(merged.Profiles, prof)
		}

		for _, cat := range part.Categories {
			if where, dup := seenCategory[cat.Name]; dup {
				return nil, fmt.Errorf("category %q redefined in %s (first defined in %s)", cat.Name, p, where)
			}
			seenCategory[cat.Name] = p
			merged.Categories = append(merged.Categories, cat)
		}
	}

	if merged.Config.InstallDir == "" {
		merged.Config.InstallDir = "/usr/local/bin"
	}

	if err := merged.Validate(); err != nil {
		return nil, err
	}
	if err := merged.resolveProfiles(); err != nil {
		return nil, err
	}

	return merged, nil
}

// Validate checks structural integrity across the merged registry. Errors
// accumulate so the user sees every problem at once instead of fix-and-rerun.
func (r *Registry) Validate() error {
	var errs []string

	profileNames := map[string]bool{}
	for _, p := range r.Profiles {
		if p.Name == "" {
			errs = append(errs, "profile entry missing required field 'name'")
			continue
		}
		profileNames[p.Name] = true
	}

	categoryNames := map[string]bool{}
	for _, c := range r.Categories {
		if c.Name == "" {
			errs = append(errs, "category entry missing required field 'name'")
			continue
		}
		categoryNames[c.Name] = true
	}

	seenTool := map[string]bool{}
	for i, t := range r.Tools {
		ctx := fmt.Sprintf("tool[%d]", i)
		if t.Name != "" {
			ctx = "tool " + t.Name
		}

		if t.Name == "" {
			errs = append(errs, fmt.Sprintf("%s: missing required field 'name'", ctx))
			continue
		}
		// Names and aliases flow into filepath.Join(installDir, name) for
		// the binary installer and into `sudo install` for the staging
		// dance. A name like "../../etc/passwd" would escape installDir and
		// let a malicious catalog overwrite arbitrary files as root. Require
		// names to be simple filenames (no separators, not relative parts).
		if !isSafeToolName(t.Name) {
			errs = append(errs, fmt.Sprintf("%s: name %q must be a simple identifier (no path separators, not '.' or '..')", ctx, t.Name))
		}
		if seenTool[t.Name] {
			errs = append(errs, fmt.Sprintf("%s: duplicate tool name", ctx))
		}
		seenTool[t.Name] = true

		for _, a := range t.Aliases {
			if !isSafeToolName(a) {
				errs = append(errs, fmt.Sprintf("%s: alias %q must be a simple identifier (no path separators, not '.' or '..')", ctx, a))
			}
			if seenTool[a] {
				errs = append(errs, fmt.Sprintf("%s: alias %q collides with another tool/alias", ctx, a))
			}
			seenTool[a] = true
		}

		switch t.Installer {
		case "":
			errs = append(errs, fmt.Sprintf("%s: missing required field 'installer'", ctx))
		case "pipx", "go":
			if t.Repo == "" && t.Package == "" {
				errs = append(errs, fmt.Sprintf("%s: %s installer needs 'repo' or 'package'", ctx, t.Installer))
			}
		case "binary":
			if t.Repo == "" {
				errs = append(errs, fmt.Sprintf("%s: binary installer needs 'repo'", ctx))
			}
		case "git":
			if t.Repo == "" {
				errs = append(errs, fmt.Sprintf("%s: git installer needs 'repo'", ctx))
			}
			if t.EntryScript != "" && t.Runtime == "" {
				errs = append(errs, fmt.Sprintf("%s: entry_script requires 'runtime' to be set", ctx))
			}
			if t.Runtime != "" && t.Runtime != "python" {
				errs = append(errs, fmt.Sprintf("%s: unsupported runtime %q (supported: python)", ctx, t.Runtime))
			}
		case "custom":
			if len(t.Steps) == 0 {
				errs = append(errs, fmt.Sprintf("%s: custom installer needs a non-empty 'steps' array", ctx))
			}
		case "cargo", "gem", "npm", "system":
			// package/system fields default to t.Name, so always valid
		default:
			errs = append(errs, fmt.Sprintf("%s: unknown installer %q", ctx, t.Installer))
		}

		// Tool's profile and category references must be declared. We only
		// run these checks when the corresponding declaration list is
		// non-empty — empty lists mean the user hasn't adopted the system at
		// all, and silently skipping the check keeps minimal test fixtures
		// usable without forcing them to define every taxonomy.
		if len(r.Profiles) > 0 {
			for _, p := range t.Profiles {
				if !profileNames[p] {
					errs = append(errs, fmt.Sprintf("%s: references unknown profile %q (add it to profiles.yaml)", ctx, p))
				}
			}
		}
		if len(r.Categories) > 0 {
			for _, c := range t.Category {
				if !categoryNames[c] {
					errs = append(errs, fmt.Sprintf("%s: references unknown category %q (add it to categories.yaml)", ctx, c))
				}
			}
		}
	}

	for _, p := range r.Profiles {
		for _, inc := range p.Includes {
			if !profileNames[inc] {
				errs = append(errs, fmt.Sprintf("profile %q: includes unknown profile %q", p.Name, inc))
			}
			if inc == p.Name {
				errs = append(errs, fmt.Sprintf("profile %q: includes itself", p.Name))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("tool registry validation:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// isSafeToolName reports whether name is a simple filename — no path
// separators, no parent/self references. Tool names flow into filepath.Join
// for binary install paths and into `sudo install` invocations; without
// this check, a catalog entry like `name: "../../etc/passwd"` could be used
// to overwrite arbitrary files when berserk is run with binary-installer
// tools that need sudo.
func isSafeToolName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, `/\`) {
		return false
	}
	// filepath.Base is the final guard: any sneaky encoding that gets
	// past the literal checks still fails Base == name on platforms where
	// the OS interprets separators differently.
	if filepath.Base(name) != name {
		return false
	}
	return true
}

// resolveProfiles computes the full member set for every declared profile,
// rolling up Includes transitively with cycle detection. The result lives in
// r.members so later calls to ToolsForProfile are O(1).
func (r *Registry) resolveProfiles() error {
	// Direct membership: which tools name this profile in their `profiles:`.
	direct := map[string][]string{}
	for _, t := range r.Tools {
		for _, p := range t.Profiles {
			direct[p] = append(direct[p], t.Name)
		}
	}

	// Index profile declarations for Includes traversal.
	byName := map[string]Profile{}
	for _, p := range r.Profiles {
		byName[p.Name] = p
	}

	resolved := map[string][]string{}
	visiting := map[string]bool{}

	var resolve func(name string) ([]string, error)
	resolve = func(name string) ([]string, error) {
		if r, ok := resolved[name]; ok {
			return r, nil
		}
		if visiting[name] {
			return nil, fmt.Errorf("profile cycle involving %q", name)
		}
		visiting[name] = true
		defer delete(visiting, name)

		prof, ok := byName[name]
		if !ok {
			// Validate already catches unknown-profile references; this branch
			// only triggers when callers ask for a never-declared profile.
			return nil, fmt.Errorf("unknown profile %q", name)
		}

		seen := map[string]bool{}
		var out []string
		for _, t := range direct[name] {
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
		for _, inc := range prof.Includes {
			inner, err := resolve(inc)
			if err != nil {
				return nil, err
			}
			for _, t := range inner {
				if !seen[t] {
					seen[t] = true
					out = append(out, t)
				}
			}
		}
		resolved[name] = out
		return out, nil
	}

	for _, p := range r.Profiles {
		if _, err := resolve(p.Name); err != nil {
			return err
		}
	}
	r.members = resolved
	return nil
}

// CategoryNames returns the names of all declared categories.
func (r *Registry) CategoryNames() []string {
	names := make([]string, len(r.Categories))
	for i, c := range r.Categories {
		names[i] = c.Name
	}
	return names
}

func (r *Registry) FindTool(name string) (*Tool, bool) {
	for i, t := range r.Tools {
		if t.Name == name {
			return &r.Tools[i], true
		}
		for _, a := range t.Aliases {
			if a == name {
				return &r.Tools[i], true
			}
		}
	}
	return nil, false
}

// ToolsForProfile returns the resolved tool list for a profile (direct
// members plus transitively-included profiles' members). Returns an error if
// the profile name isn't declared in profiles.yaml.
func (r *Registry) ToolsForProfile(profile string) ([]Tool, error) {
	names, ok := r.members[profile]
	if !ok {
		return nil, fmt.Errorf("unknown profile: %s", profile)
	}
	tools := make([]Tool, 0, len(names))
	for _, name := range names {
		if t, found := r.FindTool(name); found {
			tools = append(tools, *t)
		}
	}
	return tools, nil
}

// ProfileByName returns the declaration metadata for a profile (description,
// includes, etc.). Useful for `berserk profiles` rendering.
func (r *Registry) ProfileByName(name string) (*Profile, bool) {
	for i := range r.Profiles {
		if r.Profiles[i].Name == name {
			return &r.Profiles[i], true
		}
	}
	return nil, false
}

// ProfileMemberCount returns how many tools roll up into the given profile
// after Includes resolution. Returns 0 if the profile is unknown.
func (r *Registry) ProfileMemberCount(name string) int {
	return len(r.members[name])
}

// CategoryToolCount returns how many tools list the given category.
func (r *Registry) CategoryToolCount(name string) int {
	n := 0
	for _, t := range r.Tools {
		for _, c := range t.Category {
			if c == name {
				n++
				break
			}
		}
	}
	return n
}

// ToolsForCategories returns all tools that belong to any of the given categories.
func (r *Registry) ToolsForCategories(categories []string) []Tool {
	if len(categories) == 0 {
		return nil
	}
	want := make(map[string]bool, len(categories))
	for _, c := range categories {
		want[c] = true
	}

	seen := map[string]bool{}
	var out []Tool
	for _, t := range r.Tools {
		for _, cat := range t.Category {
			if want[cat] && !seen[t.Name] {
				seen[t.Name] = true
				out = append(out, t)
				break
			}
		}
	}
	return out
}

// SearchOpts narrows a search by orthogonal facets. An empty field is ignored.
// Filters are AND-combined with each other and with Query.
type SearchOpts struct {
	Query     string
	Category  string
	Installer string
}

// Search returns tools relevance-ranked against query. Match sources, in
// descending precedence: exact name, exact alias, exact category, name prefix,
// name substring, alias substring, category substring, description substring.
// An empty query returns every tool in catalog order.
func (r *Registry) Search(query string) []Tool {
	return r.SearchWith(SearchOpts{Query: query})
}

// SearchWith is the filterable form of Search.
func (r *Registry) SearchWith(opts SearchOpts) []Tool {
	q := strings.ToLower(strings.TrimSpace(opts.Query))
	cat := strings.ToLower(opts.Category)
	inst := strings.ToLower(opts.Installer)

	type scored struct {
		tool  Tool
		score int
		idx   int
	}
	var hits []scored

	for i, t := range r.Tools {
		if !matchesFacet(t.Category, cat) {
			continue
		}
		if inst != "" && strings.ToLower(t.Installer) != inst {
			continue
		}

		s := scoreTool(t, q)
		if q != "" && s == 0 {
			continue
		}
		hits = append(hits, scored{tool: t, score: s, idx: i})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].idx < hits[j].idx
	})

	out := make([]Tool, len(hits))
	for i, h := range hits {
		out[i] = h.tool
	}
	return out
}

// matchesFacet returns true if either no filter is set or any value in the
// haystack equals the (lower-cased) filter value.
func matchesFacet(haystack []string, filter string) bool {
	if filter == "" {
		return true
	}
	for _, v := range haystack {
		if strings.EqualFold(v, filter) {
			return true
		}
	}
	return false
}

// scoreTool ranks tool relevance against q. 0 means no match. Higher is better.
// Empty q always scores 1 so callers can keep every tool when only filtering.
func scoreTool(t Tool, q string) int {
	if q == "" {
		return 1
	}
	name := strings.ToLower(t.Name)
	desc := strings.ToLower(t.Description)

	if name == q {
		return 100
	}
	for _, a := range t.Aliases {
		if strings.EqualFold(a, q) {
			return 90
		}
	}
	for _, c := range t.Category {
		if strings.EqualFold(c, q) {
			return 70
		}
	}
	if strings.HasPrefix(name, q) {
		return 60
	}
	if strings.Contains(name, q) {
		return 50
	}
	for _, a := range t.Aliases {
		if strings.Contains(strings.ToLower(a), q) {
			return 40
		}
	}
	for _, c := range t.Category {
		if strings.Contains(strings.ToLower(c), q) {
			return 30
		}
	}
	if strings.Contains(desc, q) {
		return 10
	}
	return 0
}

// SearchSlice runs SearchWith-style scoring against an arbitrary tool slice
// instead of the full registry. Used when the caller has already narrowed
// the set (e.g. a profile subset).
func SearchSlice(tools []Tool, opts SearchOpts) []Tool {
	q := strings.ToLower(strings.TrimSpace(opts.Query))
	cat := strings.ToLower(opts.Category)
	inst := strings.ToLower(opts.Installer)

	type scored struct {
		tool  Tool
		score int
		idx   int
	}
	var hits []scored

	for i, t := range tools {
		if !matchesFacet(t.Category, cat) {
			continue
		}
		if inst != "" && strings.ToLower(t.Installer) != inst {
			continue
		}
		s := scoreTool(t, q)
		if q != "" && s == 0 {
			continue
		}
		hits = append(hits, scored{tool: t, score: s, idx: i})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].idx < hits[j].idx
	})

	out := make([]Tool, len(hits))
	for i, h := range hits {
		out[i] = h.tool
	}
	return out
}
