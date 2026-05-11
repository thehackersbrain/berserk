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

- `config.yaml` — runtime knobs (github_token, install_dir, parallel, verbose)
- `profiles.yaml` — declarations of available profiles
- `categories.yaml` — declarations of available categories
- `packages/*.yaml` — tool catalog entries (any number of files, all merged)
- `containers/*.yaml` — Docker container catalog (for `berserk run` and `berserk list -d`)

Every `*.yaml`/`*.yml` under `packages/` is merged at load time, so you can split
however the maintainer prefers. The `containers/` subdirectory is read separately
by the `run` and `list -d` commands and is not part of the tool registry.

- configs [repo](https://github.com/berserkarch/berserk-repo)

## Quick start

```bash
berserk doctor                 # verify pipx, cargo, go, gem, npm are installed
berserk sync                   # fetch the latest curated tools catalog
berserk list                   # browse the curated tools
berserk install --profile ad-attacks
berserk install nuclei httpx ffuf
berserk update                 # update everything via every backend
berserk install nxc            # aliases supported (nxc → netexec)
```

## Usage

```
berserk install [tool...]            install tools by name
berserk install --profile <name>     install all tools in a profile
berserk install --all                install everything
berserk install --dry-run ...        preview without installing
berserk update [tool...]             update specific tools
berserk update --profile <name>      update a profile
berserk update                       fire all backend updaters in parallel
berserk remove <tool>                remove a tool
berserk list [-p] [-c] [-d]          list tools, profiles (-p), categories (-c), or Docker container groups (-d)
berserk search [query] [filters]     ranked search across name/alias/category/desc + Docker containers by name
                  [-c CAT]             filter tools by category, OR Docker groups/categories by name
                  [-b INSTALLER]       filter by installer
                  [-p PROFILE]         search within a profile
                  [-i]                 only show installed tools
                  [--available]        only show tools not yet installed
berserk search info <tool>           show source, repo, installer for a tool (note: subcommand of search)
berserk run <container> [-t] [-f F]  run a Docker container from the catalog (-t = new kitty terminal, -f = extra docker run flags)
berserk sync                         sync tools catalog (clone if absent, pull if present)
berserk doctor                       verify all backends are available
berserk self-update                  update berserk itself
berserk --docker-clean               stop all containers, rmi all images, prune, delete ~/berserk/{docker,containers}
berserk version
```

Global flags: `--config <dir>` (config directory, default `/usr/share/berserk`). Set `NO_COLOR=1` to disable ANSI colors. Operations that need root (system-package installs, `/usr/local/bin` writes, `berserk sync`) shell out to `sudo` directly; if you're already root or `sudo` is unavailable, run those commands as root.

## Profiles & categories

Profiles and categories are **declared** in `profiles.yaml` and
`categories.yaml`. Tools opt into them by listing their names:

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
```

Validation enforces the contract: a tool referencing an undeclared profile
or category, or a profile including an undeclared profile, is a hard error.
Composition rolls up at query time — `red-team` automatically resolves to
every tool in `ad-attacks ∪ web ∪ post-exploitation ∪ credentials`.

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

Optional fields: `description`, `category` (list, must exist in categories.yaml), `profiles` (list, must exist in profiles.yaml), `aliases` (list), `python_version` (pipx-only).

See `configs/packages/tools.yaml.example` for a worked entry per installer.

`berserk` validates the merged tool registry on every load — malformed entries, duplicate tool names, alias collisions, and references to undeclared profiles/categories all fail fast.

## How install detection works

`berserk` records every successful install in a state file at
`$XDG_STATE_HOME/berserk/installed.yaml` (default
`~/.local/state/berserk/installed.yaml`). `list`, `search --installed`,
and `search --available` read this file as the source of truth — that's
how a tool like `impacket` (whose binaries are `smbserver.py`,
`secretsdump.py`, etc., never `impacket` on PATH) is correctly reported
as installed once berserk has installed it. PATH lookup is kept as a
fallback so tools you installed before berserk knew about them still register.

## How update works

`berserk update` (no args) fires every backend updater in parallel:

- `pipx upgrade-all`
- `cargo install-update -a` (requires the `cargo-install-update` crate; `berserk doctor` will install it)
- `gem update --user-install <pkg>` for every gem-installed tool tracked in state
- `sudo npm update -g` (updates every globally-installed npm package, not just berserk's)
- `go install <pkg>@latest` for every go-installed tool tracked in state

Per-tool updates (`berserk update <name>`) re-run the appropriate install/upgrade
command at latest. Use `berserk update --backend <pipx|cargo|gem|npm|go>` to
narrow the no-arg sweep to a single backend.
