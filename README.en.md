# ffh

English | [日本語](README.md)

A CLI tool that parses `~/.ssh/config` and lets you interactively select an SSH host via fzf.

## Features

- Recursively resolves `Include` directives in `~/.ssh/config`
- **Left preview pane** showing host details while browsing
- **Tab filtering** using the `Tag` directive to narrow down hosts
- Multi-line host descriptions via `# Description:` comments
- Hosts file mode (path configurable via CLI, environment variable, or config file)
- UI language switchable between English and Japanese

## Installation

```bash
# Install dependencies if not already present
sudo apt install fzf

# Build and install
make install   # places binary at /usr/local/bin/ffh
```

**Requirements**

| Tool | Version |
|------|---------|
| Go   | 1.24+   |
| fzf  | 0.44+   |
| ssh  | any     |

## Usage

### Basic

```bash
ffh
```

fzf opens with the list of hosts defined in `~/.ssh/config`. Selecting a host runs `ssh <host>`.

SSH options are passed through directly:

```bash
ffh -L 8080:localhost:8080   # port forwarding
ffh -v                        # verbose/debug output
```

### fzf key bindings

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate host list |
| `Enter` | Connect via SSH |
| `Ctrl-G` | Show full `ssh -G` config for the focused host |
| `Tab` | Move to next tag tab |
| `Shift-Tab` | Move to previous tag tab |
| `Esc` / `Ctrl-C` | Cancel |
| Text input | Fuzzy search |

### Tab filtering

When `Tag` directives are present in `~/.ssh/config`, tabs appear in the header:

```
  [ All ]  [ dev ]  [ prod ]
```

- **All** — show every host (default)
- **tag name** — show only hosts belonging to that tag

Switch tabs with `Tab` / `Shift-Tab`.

### Preview pane

Focusing a host shows its details in the left pane:

```
  Host:           myserver
  ────────────────────────────────
  HostName:       10.0.0.1
  User:           admin
  Port:           22 (default)
  ProxyJump:      bastion
  IdentityFile:   ~/.ssh/id_ed25519
  Tag:            prod
  Source:         ~/.ssh/config.d/servers

  ────────────────────────────────
  Description
  Production web server
  See the wiki for details
```

### SSH config view (Ctrl-G)

Pressing `Ctrl-G` on any focused host opens a nested fzf showing every resolved SSH option from `ssh -G <host>`. Focusing an option line displays its description in the right preview pane.

### Hosts file mode

```bash
ffh --hosts                        # use the file resolved by configuration
ffh --hosts /path/to/custom/hosts  # specify a path directly
```

Reads a hosts file, lets you select a host via fzf, and connects via SSH. Loopback addresses (`127.x.x.x`, `::1`) are excluded.

The file to use is resolved in this priority order:

| Priority | Method | Example |
|----------|--------|---------|
| 1 | CLI argument | `ffh --hosts /path/to/hosts` |
| 2 | `FFH_HOSTS_FILE` env var | `export FFH_HOSTS_FILE=/path/to/hosts` |
| 3 | Config file `~/.config/ffh/config` | `hosts_file = /path/to/hosts` |
| 4 | Default | `/etc/hosts` |

**Config file example** (`~/.config/ffh/config`):

```ini
# ffh config
hosts_file = /path/to/hosts
```

---

## Language

The UI defaults to English. Switch to Japanese with:

```bash
# Environment variable (session-scoped)
FFH_LANG=ja ffh

# Config file (persistent) — ~/.config/ffh/config
language = ja
```

If neither is set, ffh checks the system `LANG` environment variable and uses Japanese automatically when it starts with `ja`.

---

## SSH config reference

### Tag — tab filtering

```ssh-config
Host myserver
    HostName 10.0.0.1
    Tag prod
```

Hosts sharing the same `Tag` value are grouped under that tab.

### Description — host description

**Single line:**

```ssh-config
# Description: Production web server
Host myserver
    HostName 10.0.0.1
```

**Multi-line (`# Description:` as a marker):**

```ssh-config
# Description:
# Production web server
# See the wiki for details
Host myserver
    HostName 10.0.0.1
```

- The `# Description:` line is the marker. All `#` comment lines that follow it become the description body.
- A blank line between `# Description:` and the `Host` line causes the description to be ignored.

### Full config example

```ssh-config
# Description:
# Bastion server for the infra environment
# Access via ProxyJump
Host bastion
    HostName 203.0.113.10
    User ec2-user
    IdentityFile ~/.ssh/bastion_key
    Tag infra

Host dev-server
    HostName 10.0.1.20
    User admin
    ProxyJump bastion
    Tag dev

Host prod-db
    HostName 10.0.2.30
    User dbadmin
    ProxyJump bastion
    Tag prod
```
