# berserk

Curated offensive security tool manager for BerserkArch.

---

## Install

```bash
git clone https://github.com/thehackersbrain/berserk
cd berserk
make install   # installs the binary to /usr/local/bin
berserk sync   # clones the tools catalog to /usr/share/berserk
```

or with go

```bash
go install -v github.com/thehackersbrain/berserk@latest
```

Or do it by hand:

```bash
go build -o berserk .
sudo install -m 0755 berserk /usr/local/bin/
sudo install -d /usr/share/berserk
berserk sync
```

### Arch Linux

Make sure these deps are installed

```bash
sudo pacman -Syy rust cargo nodejs npm ruby git python-pipx go pkgconf openssl base-devel cmake make --noconfirm
```

### Kali Linux

Make sure these deps are installed

```bash
sudo apt install golang-go cargo pipx gem ruby-dev make openssl libssl-dev -y
```

`berserk` reads its config directory on every invocation. By default that's
`/usr/share/berserk`; override with `--config <dir>`. The directory holds:

- `config.yaml` — runtime knobs (see [Configuration](#configuration))
- `profiles.yaml` — declarations of available profiles
- `categories.yaml` — declarations of available categories
- `packages/*.yaml` — tool catalog entries (any number of files, all merged)
- `containers/*.yaml` — Docker container catalog (for `berserk run` and `berserk list -d`)

Every `*.yaml`/`*.yml` under `packages/` is merged at load time, so you can split
however the maintainer prefers. The `containers/` subdirectory is read separately
by the `run` and `list -d` commands and is not part of the tool registry.

- configs [repo](https://github.com/berserkarch/berserk-repo)

## Configuration

`config.yaml` is a single flat-keyed YAML file (no `config:` wrapper) read
from the config dir on every invocation. Every key is optional — omitted
keys fall back to the defaults below.

```yaml
# config.yaml
github_token:    ghp_xxxxxxxxxxxx       # default: ""  (env GITHUB_TOKEN wins)
install_dir:     /usr/local/bin         # default: /usr/local/bin
parallel:        true                   # default: false
verbose:         false                  # default: false
docker_data_dir: ~/berserk              # default: ~/berserk
```

| key | type | default | effect |
| --- | --- | --- | --- |
| `github_token` | string | `""` | Auth token used by the `binary` installer's GitHub Release downloader (raises the unauthenticated 60 req/h rate limit). `$GITHUB_TOKEN` in the env overrides this — convenient for CI without leaking the token into config files. |
| `install_dir` | string | `/usr/local/bin` | Where the `binary` installer drops downloaded executables. Needs write access (berserk shells out to `sudo install` when the dir isn't user-writable). |
| `parallel` | bool | `false` | Fan out per-tool install/update across goroutines. Per-backend system pkg-mgr calls (`pacman`, `apt`) are still serialized through a single mutex — only the lookup/download/extract phases parallelize. |
| `verbose` | bool | `false` | Stream the underlying installer command's stdout/stderr to your terminal instead of capturing it. Useful when an install fails and you need to see what `pipx`/`cargo`/`go install` actually said. |
| `docker_data_dir` | string | `~/berserk` | Root directory for container volume mounts. The container catalog references `~/btweak/{containers,docker}/...` paths (the Python predecessor's layout); berserk rewrites the `~/btweak/` prefix to `<docker_data_dir>/` at runtime. Set this to relocate persistent container data — e.g. to a non-home volume or shared mount. Leading `~/` is expanded against `$HOME`; absolute paths are used as-is. `berserk --docker-clean` deletes `<docker_data_dir>/{docker,containers}`. |

Global flags that aren't in `config.yaml`:

- `--config <dir>` — override the config directory (default `/usr/share/berserk`).
- `--yes` / `-y` — auto-confirm system pkg-mgr prompts (pacman `--noconfirm`, apt `-y`). Off by default so the underlying pkg mgr can still prompt on conflicts.
- `NO_COLOR=1` — disable ANSI colors (env var).

## Quick start

```bash
berserk doctor                 # verify backends, fix PATH, create /opt/berserk for git-installer repos
berserk sync                   # fetch the latest curated tools catalog
berserk list                   # browse the curated tools
berserk list -d                # browse the Docker container catalog
berserk install --profile ad-attacks
berserk install nuclei httpx ffuf
berserk update                 # update everything via every backend
berserk install nxc            # aliases supported (nxc → netexec)

berserk run kali-cli                       # run a container (replaces process via syscall.Exec)
berserk run tor-browser -t                 # run in a new kitty terminal window
berserk run kali-cli -f "--rm --name k1"   # inject extra flags after "docker run"
berserk search kali                        # search tools AND containers by name
berserk --docker-clean                     # nuke all containers/images + ~/berserk/{docker,containers}
```

## Usage

```
berserk install [tool...]            install tools by name
berserk install --profile <name>     install all tools in a profile
berserk install --category <cat...>  install all tools in one or more categories
berserk install --all                install everything
berserk install --dry-run ...        preview without installing
berserk update [tool...]             update specific tools
berserk update --profile <name>      update a profile
berserk update                       fire all backend updaters in parallel
berserk remove <tool>                remove a tool
berserk list [-p] [-c] [-d]          list tools, profiles (-p), categories (-c), or Docker containers (-d)
berserk search [query] [filters]     ranked search across name/alias/category/desc + Docker containers by name
                  [-c CAT]             filter tools AND Docker containers by category
                  [-b INSTALLER]       filter by installer
                  [-p PROFILE]         search within a profile
                  [-i]                 only show installed tools
                  [--available]        only show tools not yet installed
                  [-n LIMIT]           limit the number of results
berserk search info <tool>           show source, repo, installer for a tool (note: subcommand of search)
berserk run <container> [-t] [-f F]  run a Docker container from the catalog (-t = new kitty terminal, -f = extra docker run flags)
berserk sync                         sync tools catalog (clone if absent, pull if present)
berserk doctor                       verify all backends, fix PATH, install helpers, create /opt/berserk
berserk self-update                  update berserk itself
berserk --docker-clean               stop all containers, rmi all images, prune, delete ~/berserk/{docker,containers}
berserk version
```

Operations that need root (system-package installs, `/usr/local/bin` writes, `berserk sync`) shell out to `sudo` directly; if you're already root or `sudo` is unavailable, run those commands as root. See [Configuration](#configuration) for global flags and `config.yaml` keys.

## Profiles & categories

Profiles and categories are **declared** in `profiles.yaml` and
`categories.yaml`. Tools and Docker containers opt into them by listing their names:

```yaml
# profiles.yaml
profiles:
  - name: ad-attacks
    description: "Active Directory attacks and lateral movement"
  - name: red-team
    description: "Composed engagement profile"
    includes: [ad-attacks, web, post-exploitation, credentials] # rolls up members

# categories.yaml
categories:
  - name: ad
    description: "Active Directory"
  - name: lateral-movement
    description: "Lateral movement techniques"

# tools.yaml
tools:
  - name: netexec
    description: "..."
    category: [ad, lateral-movement, recon] # must reference categories.yaml
    profiles: [ad-attacks] # must reference profiles.yaml
    installer: pipx
    repo: Pennyw0rth/NetExec
    depends: # optional build/runtime deps
      arch: [openssl, pkgconf]
      debian: [libssl-dev, pkg-config]
```

Validation enforces the contract: an entry referencing an undeclared profile
or category, or a profile including an undeclared profile, is a hard error.
This applies to both tools and Docker containers.

Run `berserk list --profiles` to see all declared profiles with their resolved
member counts.

## Adding a tool

Edit any tool yaml file under `packages/` in your config dir (`packages/tools.yaml`, or a category-split file like `packages/ad.yaml`). Each entry needs at minimum `name` and `installer`. Per-installer requirements:

| installer | required                                           | example                                                        |
| --------- | -------------------------------------------------- | -------------------------------------------------------------- |
| `pipx`    | `repo` or `package`                                | `repo: fortra/impacket`                                        |
| `go`      | `repo` or `package`                                | `repo: projectdiscovery/nuclei/v3/cmd/nuclei`                  |
| `cargo`   | (`package` defaults)                               | `package: rustscan`                                            |
| `gem`     | (`package` defaults)                               | `package: evil-winrm`                                          |
| `npm`     | (`package` defaults)                               | `package: <pkg>`                                               |
| `binary`  | `repo` + `asset_pattern`                           | `repo: BishopFox/sliver`, `asset_pattern: sliver-client_linux` |
| `system`  | `arch_package` or `debian_package` (default: name) |                                                                |
| `git`     | `repo`                                             | `repo: flangvik/SharpCollection`                               |

Optional fields: `description`, `category` (list, must exist in categories.yaml), `profiles` (list, must exist in profiles.yaml), `aliases` (list), `python_version` (pipx-only), `branch` (git-only, defaults to repo default branch), `entry_scripts` (git-only, list of script filenames to expose as shims), `runtime` (git-only, required when `entry_scripts` is set; currently `python`), `pip_deps` (git-only, explicit pip packages — additive with `requirements.txt`), `depends` (map of distro to package list).

#### git + Python script tools

For repos that are bare Python scripts rather than installable packages, set `entry_scripts` and `runtime: python`. After cloning, berserk creates an isolated virtualenv at `/opt/berserk/venvs/<name>`, installs deps from `requirements.txt` (if present) and any explicit `pip_deps` (additive), then writes one bash shim per entry to `/opt/berserk/bin/<script-without-extension>`:

```yaml
# single script
- name: targetedkerberoast
  installer: git
  repo: ShutdownRepo/targetedKerberoast
  entry_scripts: [targetedKerberoast.py]
  runtime: python

# multiple scripts (e.g. PKINITtools)
- name: pkinittools
  installer: git
  repo: dirkjanm/PKINITtools
  entry_scripts: [gettgtpkinit.py, gets4uticket.py, getnthash.py]
  runtime: python
  # pip_deps: [minikerberos, impacket]   # additive on top of requirements.txt
```

Each script gets its own shim: `gettgtpkinit.py` → `/opt/berserk/bin/gettgtpkinit`. `berserk doctor` creates `/opt/berserk/bin` and adds it to `PATH`. On `berserk update <name>`, deps are refreshed inside the existing venv. On `berserk remove <name>`, the clone, venv, and all shims are removed.

See `configs/packages/tools.yaml.example` for a worked entry per installer.

`berserk` validates the merged tool registry on every load — malformed entries, duplicate tool names, alias collisions, and references to undeclared profiles/categories all fail fast.

## Adding a container

Edit any YAML under `containers/` in your config dir. The catalog is a flat
list of containers. Each entry must reference valid categories from
`categories.yaml`:

```yaml
- name: kali-cli
  description: "Official Kali Linux container with full toolset"
  category: ["linux-cli"]
  command: "docker pull kalilinux/kali-rolling"
  run: "docker run -it --rm --network host --name kali -v ~/berserk/containers/kali/:/root/data kalilinux/kali-rolling /bin/bash"
  runtime_comments:
    - "- A local volume is mounted - Host: ~/berserk/containers/kali -- Container: /root/data"

- name: tor-browser
  description: "Tor Browser in a container"
  category: ["linux-gui"]
  command: "docker pull domistyle/tor-browser"
  run: "docker run --rm -e DISPLAY=$DISPLAY -v /tmp/.X11-unix:/tmp/.X11-unix domistyle/tor-browser"
```

Path rewriting: any `~/btweak/containers/` or `~/btweak/docker/` in a `run`
command is rewritten to `~/berserk/` at runtime (eases migration from the
Python predecessor). `~` is expanded to `$HOME` before exec, but `$(pwd)`,
`$DISPLAY`, `$HOME`, etc. are left intact and resolved by `/bin/sh -c`. Volume
host-paths from `-v` flags are auto-`MkdirAll`'d before exec (paths containing
`$` are skipped — the shell creates those targets itself).

`berserk run <name>` does a case-insensitive substring match, prefers an exact
name match when multiple containers match, and errors out if the catalog's
`run` is empty or `-f` can't find a literal `docker run` to inject flags after.
Use `-t` to launch in a new kitty terminal window instead of replacing the
current process via `syscall.Exec`.

`berserk list -d` displays the container catalog in a tabular format, showing
the name, assigned categories, and description for each entry.


## How install detection works

`berserk` records every successful install in a state file at
`$XDG_STATE_HOME/berserk/installed.yaml` (default
`~/.local/state/berserk/installed.yaml`). `list`, `search --installed`,
and `search --available` read this file as the source of truth — that's
how a tool like `impacket` (whose binaries are `smbserver.py`,
`secretsdump.py`, etc., never `impacket` on PATH) is correctly reported
as installed once berserk has installed it. PATH lookup is kept as a
fallback so tools you installed before berserk knew about them still register.

`git`-installer tools have no PATH fallback at all — a cloned repo never
puts a binary on PATH. The state file is the sole source of truth for them.
Repos cloned manually into `/opt/berserk` before berserk knew about them
will not appear as installed until berserk installs them itself.

## How update works

`berserk update` (no args) fires every backend updater in parallel:

- `pipx upgrade-all`
- `cargo install-update -a` (requires the `cargo-install-update` crate; `berserk doctor` will install it)
- `gem update --user-install <pkg>` for every gem-installed tool tracked in state
- `sudo npm update -g` (updates every globally-installed npm package, not just berserk's)
- `go install <pkg>@latest` for every go-installed tool tracked in state
- `git pull --ff-only` in `/opt/berserk/<name>` for every git-installed repo tracked in state

Per-tool updates (`berserk update <name>`) re-run the appropriate install/upgrade
command at latest. Use `berserk update --backend <pipx|cargo|gem|npm|go|git>` to
narrow the no-arg sweep to a single backend.
