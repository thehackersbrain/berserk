package registry

import (
	"os"
	"path/filepath"
	"testing"
)

const testYAML = `
config:
  install_dir: /usr/local/bin
  parallel: true

tools:
  - name: mytool
    description: "A test tool"
    category: [recon, web]
    profiles: [recon, web]
    installer: pipx
    repo: owner/mytool

  - name: rustytool
    description: "A rust tool"
    category: [recon]
    profiles: [recon]
    installer: cargo
    package: rustytool

profiles:
  - name: recon
    description: "Reconnaissance"
  - name: web
    description: "Web testing"

categories:
  - name: recon
  - name: web
`

func writeTestYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tools.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad(t *testing.T) {
	path := writeTestYAML(t, testYAML)
	reg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(reg.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(reg.Tools))
	}
	if reg.Config.InstallDir != "/usr/local/bin" {
		t.Errorf("install_dir = %q", reg.Config.InstallDir)
	}
	if len(reg.Profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(reg.Profiles))
	}
	if len(reg.Categories) != 2 {
		t.Errorf("expected 2 categories, got %d", len(reg.Categories))
	}
}

func TestFindTool(t *testing.T) {
	path := writeTestYAML(t, testYAML)
	reg, _ := Load(path)

	t.Run("found", func(t *testing.T) {
		tool, ok := reg.FindTool("mytool")
		if !ok {
			t.Fatal("expected to find mytool")
		}
		if tool.Installer != "pipx" {
			t.Errorf("installer = %q, want pipx", tool.Installer)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, ok := reg.FindTool("nonexistent")
		if ok {
			t.Fatal("expected not found")
		}
	})
}

func TestToolsForProfile(t *testing.T) {
	path := writeTestYAML(t, testYAML)
	reg, _ := Load(path)

	tools, err := reg.ToolsForProfile("recon")
	if err != nil {
		t.Fatalf("ToolsForProfile error: %v", err)
	}
	if len(tools) != 2 {
		t.Errorf("expected 2 tools in recon, got %d", len(tools))
	}

	_, err = reg.ToolsForProfile("nosuch")
	if err == nil {
		t.Error("expected error for unknown profile")
	}
}

func TestSearch(t *testing.T) {
	path := writeTestYAML(t, testYAML)
	reg, _ := Load(path)

	results := reg.Search("rust")
	if len(results) != 1 || results[0].Name != "rustytool" {
		t.Errorf("search 'rust': unexpected results %v", results)
	}

	results = reg.Search("test tool")
	if len(results) != 1 || results[0].Name != "mytool" {
		t.Errorf("search 'test tool': unexpected results %v", results)
	}

	results = reg.Search("zzznomatch")
	if len(results) != 0 {
		t.Errorf("expected empty results, got %v", results)
	}
}

func TestSearchRanksExactNameFirst(t *testing.T) {
	yaml := `tools:
  - name: nmap
    description: "network scanner"
    category: [recon]
    installer: system
  - name: nmap-helper
    description: "wraps nmap with extras"
    category: [recon]
    installer: pipx
    repo: x/nmap-helper
  - name: zzz
    description: "mentions nmap somewhere"
    category: [misc]
    installer: pipx
    repo: x/zzz
`
	path := writeTestYAML(t, yaml)
	reg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got := reg.Search("nmap")
	if len(got) != 3 {
		t.Fatalf("want 3 results, got %d", len(got))
	}
	if got[0].Name != "nmap" {
		t.Errorf("exact-name match should rank first, got %q", got[0].Name)
	}
	if got[2].Name != "zzz" {
		t.Errorf("description-only match should rank last, got %q", got[2].Name)
	}
}

func TestSearchMatchesAliasAndCategory(t *testing.T) {
	yaml := `tools:
  - name: netexec
    description: "AD swiss army"
    category: [ad, post-exploitation]
    installer: pipx
    repo: x/netexec
    aliases: [nxc, cme]
  - name: bloodhound
    description: "AD graph"
    category: [ad]
    installer: pipx
    repo: x/bh
  - name: gobuster
    description: "dir brute"
    category: [web]
    installer: go
    package: example.com/gobuster
`
	path := writeTestYAML(t, yaml)
	reg, _ := Load(path)

	if got := reg.Search("nxc"); len(got) != 1 || got[0].Name != "netexec" {
		t.Errorf("alias search 'nxc' returned %v", got)
	}
	if got := reg.Search("ad"); len(got) != 2 {
		t.Errorf("category search 'ad' wanted 2, got %d", len(got))
	}
}

func TestSearchWithFilters(t *testing.T) {
	yaml := `tools:
  - name: a
    description: "alpha"
    category: [web]
    installer: pipx
    repo: x/a
  - name: b
    description: "beta"
    category: [web]
    installer: go
    package: x/b
  - name: c
    description: "gamma"
    category: [recon]
    installer: pipx
    repo: x/c
`
	path := writeTestYAML(t, yaml)
	reg, _ := Load(path)

	got := reg.SearchWith(SearchOpts{Category: "web"})
	if len(got) != 2 {
		t.Errorf("category=web wanted 2, got %d", len(got))
	}
	got = reg.SearchWith(SearchOpts{Category: "web", Installer: "pipx"})
	if len(got) != 1 || got[0].Name != "a" {
		t.Errorf("category+installer wanted [a], got %v", names(got))
	}
	got = reg.SearchWith(SearchOpts{Installer: "pipx"})
	if len(got) != 2 {
		t.Errorf("installer=pipx wanted 2, got %d", len(got))
	}
	got = reg.SearchWith(SearchOpts{Query: "alpha", Installer: "go"})
	if len(got) != 0 {
		t.Errorf("query+filter no-match wanted 0, got %d", len(got))
	}
}

func names(tools []Tool) []string {
	out := make([]string, len(tools))
	for i, t := range tools {
		out[i] = t.Name
	}
	return out
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/tools.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadBadYAML(t *testing.T) {
	path := writeTestYAML(t, "tools: [bad: yaml: }: :")
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestValidationCatchesMissingFields(t *testing.T) {
	cases := map[string]string{
		"missing name": `tools:
  - installer: pipx
    repo: foo/bar`,
		"missing installer": `tools:
  - name: foo`,
		"binary without repo": `tools:
  - name: foo
    installer: binary`,
		"unknown installer": `tools:
  - name: foo
    installer: hopium`,
		"custom without steps field": `tools:
  - name: foo
    installer: custom`,
		"duplicate name": `tools:
  - name: dup
    installer: pipx
    repo: a/b
  - name: dup
    installer: pipx
    repo: c/d`,
		"tool references unknown profile": `tools:
  - name: real
    installer: pipx
    repo: a/b
    profiles: [ghost]
profiles:
  - name: real-only
`,
		"tool references unknown category": `tools:
  - name: real
    installer: pipx
    repo: a/b
    category: [ghost]
categories:
  - name: real-only
`,
		"profile includes unknown profile": `tools:
  - name: a
    installer: pipx
    repo: x/a
profiles:
  - name: known
  - name: composed
    includes: [unknown]
`,
		"profile includes itself": `profiles:
  - name: loopy
    includes: [loopy]
`,
	}
	for label, yaml := range cases {
		t.Run(label, func(t *testing.T) {
			path := writeTestYAML(t, yaml)
			_, err := Load(path)
			if err == nil {
				t.Fatalf("expected validation error for %q", label)
			}
		})
	}
}

func TestAliasResolution(t *testing.T) {
	yaml := `tools:
  - name: netexec
    installer: pipx
    repo: Pennyw0rth/NetExec
    aliases: [nxc, cme]`
	path := writeTestYAML(t, yaml)
	reg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"netexec", "nxc", "cme"} {
		if _, ok := reg.FindTool(query); !ok {
			t.Errorf("FindTool(%q) returned not-found", query)
		}
	}
}

func TestProfileComposition(t *testing.T) {
	yaml := `tools:
  - name: a
    installer: pipx
    repo: x/a
    profiles: [ab]
  - name: b
    installer: pipx
    repo: x/b
    profiles: [ab]
  - name: c
    installer: pipx
    repo: x/c
    profiles: [abc]
profiles:
  - name: ab
  - name: abc
    includes: [ab]
`
	path := writeTestYAML(t, yaml)
	reg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("expansion rolls up included profile members", func(t *testing.T) {
		got, err := reg.ToolsForProfile("abc")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Errorf("abc profile: got %d tools, want 3 (a, b via include + c direct)", len(got))
		}
	})

	t.Run("dedup across direct + includes", func(t *testing.T) {
		yaml := `tools:
  - name: a
    installer: pipx
    repo: x/a
    profiles: [ab, dup]
  - name: b
    installer: pipx
    repo: x/b
    profiles: [ab]
profiles:
  - name: ab
  - name: dup
    includes: [ab]
`
		path := writeTestYAML(t, yaml)
		reg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := reg.ToolsForProfile("dup")
		if len(got) != 2 {
			t.Errorf("dup profile: got %d tools, want 2 (a appears via both direct + include but should dedup)", len(got))
		}
	})
}

func TestProfileCycleDetected(t *testing.T) {
	yaml := `tools:
  - name: a
    installer: pipx
    repo: x/a
    profiles: [loop2]
profiles:
  - name: loop1
    includes: [loop2]
  - name: loop2
    includes: [loop1]
`
	path := writeTestYAML(t, yaml)
	if _, err := Load(path); err == nil {
		t.Error("expected cycle detection error")
	}
}

func TestDefaultInstallDir(t *testing.T) {
	path := writeTestYAML(t, "tools: []\n")
	reg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reg.Config.InstallDir != "/usr/local/bin" {
		t.Errorf("default install_dir = %q, want /usr/local/bin", reg.Config.InstallDir)
	}
}

func TestProfileMemberCount(t *testing.T) {
	yaml := `tools:
  - name: x
    installer: pipx
    repo: x/x
    profiles: [base]
  - name: y
    installer: pipx
    repo: x/y
    profiles: [base]
profiles:
  - name: base
  - name: composite
    includes: [base]
`
	path := writeTestYAML(t, yaml)
	reg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reg.ProfileMemberCount("base"); got != 2 {
		t.Errorf("base count = %d, want 2", got)
	}
	if got := reg.ProfileMemberCount("composite"); got != 2 {
		t.Errorf("composite count = %d, want 2 (rolled up via include)", got)
	}
	if got := reg.ProfileMemberCount("nonexistent"); got != 0 {
		t.Errorf("unknown profile count = %d, want 0", got)
	}
}

func TestProfileByName(t *testing.T) {
	path := writeTestYAML(t, testYAML)
	reg, _ := Load(path)

	p, ok := reg.ProfileByName("recon")
	if !ok {
		t.Fatal("ProfileByName(recon) not found")
	}
	if p.Description != "Reconnaissance" {
		t.Errorf("description = %q", p.Description)
	}
	if _, ok := reg.ProfileByName("unknown"); ok {
		t.Error("expected ProfileByName(unknown) to return false")
	}
}

// writeFiles drops a set of yaml files into a fresh tempdir for LoadDir tests.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	return dir
}

func TestLoadDirMergesFiles(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"ad.yaml": `tools:
  - name: netexec
    installer: pipx
    repo: x/netexec
    aliases: [nxc]
    profiles: [ad]
`,
		"web.yaml": `tools:
  - name: nuclei
    installer: go
    repo: projectdiscovery/nuclei
    profiles: [web]
`,
		"profiles.yaml": `profiles:
  - name: ad
  - name: web
`,
	})
	reg, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir error: %v", err)
	}
	if len(reg.Tools) != 2 {
		t.Errorf("merged tools = %d, want 2", len(reg.Tools))
	}
	if _, ok := reg.FindTool("nxc"); !ok {
		t.Error("alias from merged file not found")
	}
	if len(reg.Profiles) != 2 {
		t.Errorf("merged profiles = %d, want 2", len(reg.Profiles))
	}
}

func TestLoadDirReadsConfigYAML(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"config.yaml": `github_token: secret
install_dir: /opt/bin
parallel: true
verbose: true
`,
		"tools.yaml": `tools:
  - name: a
    installer: pipx
    repo: x/a
`,
	})
	reg, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir error: %v", err)
	}
	if len(reg.Tools) != 1 || reg.Tools[0].Name != "a" {
		t.Errorf("tools = %v, want [a]", reg.Tools)
	}
	// config.yaml must be loaded into Registry.Config — these keys drive
	// runtime behaviour and used to silently no-op when only viper read them.
	if reg.Config.GithubToken != "secret" {
		t.Errorf("github_token = %q, want %q", reg.Config.GithubToken, "secret")
	}
	if reg.Config.InstallDir != "/opt/bin" {
		t.Errorf("install_dir = %q, want /opt/bin", reg.Config.InstallDir)
	}
	if !reg.Config.Parallel {
		t.Error("parallel should be true")
	}
	if !reg.Config.Verbose {
		t.Error("verbose should be true")
	}
}

func TestLoadDirNoConfigYAMLUsesDefaults(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"tools.yaml": `tools:
  - name: a
    installer: pipx
    repo: x/a
`,
	})
	reg, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir error: %v", err)
	}
	if reg.Config.GithubToken != "" {
		t.Errorf("github_token = %q, want empty", reg.Config.GithubToken)
	}
	if reg.Config.Parallel {
		t.Error("parallel should default to false")
	}
}

func TestLoadDirAcceptsYmlExtension(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"recon.yml": `tools:
  - name: nmap
    installer: system
`,
	})
	reg, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir error: %v", err)
	}
	if len(reg.Tools) != 1 {
		t.Errorf("want 1 tool, got %d", len(reg.Tools))
	}
}

func TestLoadDirRejectsCrossFileDuplicates(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"a.yaml": `tools:
  - name: dup
    installer: pipx
    repo: x/a
`,
		"b.yaml": `tools:
  - name: dup
    installer: pipx
    repo: x/b
`,
	})
	if _, err := LoadDir(dir); err == nil {
		t.Error("expected duplicate tool error across files")
	}
}

func TestLoadDirRejectsAliasCollisionAcrossFiles(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"a.yaml": `tools:
  - name: real
    installer: pipx
    repo: x/real
    aliases: [shared]
`,
		"b.yaml": `tools:
  - name: other
    installer: pipx
    repo: x/other
    aliases: [shared]
`,
	})
	if _, err := LoadDir(dir); err == nil {
		t.Error("expected alias collision across files")
	}
}

func TestLoadDirRejectsProfileRedefinition(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"a.yaml": `profiles:
  - name: shared
`,
		"b.yaml": `profiles:
  - name: shared
`,
	})
	if _, err := LoadDir(dir); err == nil {
		t.Error("expected profile redefinition error")
	}
}

func TestLoadDirRejectsCategoryRedefinition(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"a.yaml": `categories:
  - name: web
`,
		"b.yaml": `categories:
  - name: web
`,
	})
	if _, err := LoadDir(dir); err == nil {
		t.Error("expected category redefinition error")
	}
}

func TestLoadDirProfileCanReferenceCrossFileTools(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"tools-a.yaml": `tools:
  - name: a
    installer: pipx
    repo: x/a
    profiles: [combo]
`,
		"profiles.yaml": `profiles:
  - name: combo
`,
	})
	reg, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir error: %v", err)
	}
	got, err := reg.ToolsForProfile("combo")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "a" {
		t.Errorf("combo profile = %v, want [a]", got)
	}
}

func TestLoadDirEmpty(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadDir(dir); err == nil {
		t.Error("expected error for dir with no tool yaml files")
	}
}

func TestLoadDirIgnoresNonYAML(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"README.md": "# not yaml",
		"tools.yaml": `tools:
  - name: a
    installer: pipx
    repo: x/a
`,
	})
	reg, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir error: %v", err)
	}
	if len(reg.Tools) != 1 {
		t.Errorf("want 1 tool, got %d", len(reg.Tools))
	}
}

// TestLoadDirSkipsContainersDir guards the invariant that registry.LoadDir
// must skip the containers/ subdirectory entirely. docker catalog yaml is a
// top-level list (not a registry map), and yaml.Unmarshal into Registry
// would error on it — silently breaking every command that calls
// loadContext when a user populates configs/containers/.
func TestLoadDirSkipsContainersDir(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"tools.yaml": `tools:
  - name: a
    installer: pipx
    repo: x/a
`,
	})
	containersDir := filepath.Join(dir, "containers")
	if err := os.MkdirAll(containersDir, 0o755); err != nil {
		t.Fatalf("mkdir containers: %v", err)
	}
	// A top-level YAML list — would fail Registry unmarshalling if visited.
	body := `- name: "Some Group"
  description: "..."
  containers:
    - name: foo
      command: "docker pull foo"
      run: "docker run foo"
`
	if err := os.WriteFile(filepath.Join(containersDir, "docker.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write docker.yaml: %v", err)
	}

	reg, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir should skip containers/, got error: %v", err)
	}
	if len(reg.Tools) != 1 {
		t.Errorf("want 1 tool, got %d", len(reg.Tools))
	}
}
