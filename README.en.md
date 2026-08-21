# ffh

English | [日本語](README.md)

A CLI tool that parses `~/.ssh/config` and lets you interactively select an SSH host via fzf.

## Features

- Recursively resolves `Include` directives in `~/.ssh/config`
- **Left preview pane** showing host details while browsing
- Tab filtering grouped by **source config file** (default) or by the **`Tag` directive** — toggle with `Ctrl-T`
- Multi-line host descriptions via `# Description:` comments
- Multiple hostnames on a single `Host` line are expanded into separate entries
- Connection history with quick reconnect (`--history`)
- Duplicate host definition detection (`--check`)
- Run a command on every host with a given tag (`--exec`)
- Copy the resolved `ssh` command to the clipboard, check TCP reachability, and view/edit `ssh -G` output inline
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

SSH options are passed after `--`:

```bash
ffh -- -L 8080:localhost:8080   # port forwarding
ffh -- -v                       # verbose/debug output
```

### Command-line options

| Option | Purpose |
|---|---|
| `-h`, `--help` | show help |
| `-v`, `--version` | show version |
| `-F <file>` | use an alternative SSH config file (overrides env var / config file) |
| `--tab-source <tag\|source>` | how tabs are grouped (default: `source`) |
| `--hosts [path]` | start in hosts-file mode |
| `--history` | select from connection history |
| `--history --delete <host>` | delete a history entry |
| `--check` | detect duplicate host definitions |
| `--exec <tag> <command...>` | run a command on every host with the given tag |

### fzf key bindings

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate host list |
| `Enter` | Connect via SSH |
| `Ctrl-G` | Show full `ssh -G` config for the focused host (`Enter` to edit inline) |
| `Ctrl-Y` | Copy the `ssh` command for the focused host to the clipboard |
| `Ctrl-P` | Show a TCP reachability check for the focused host in the preview pane |
| `Ctrl-T` | Toggle tab grouping between `Tag` and source config file |
| `Tab` | Move to next tab |
| `Shift-Tab` | Move to previous tab |
| `Esc` / `Ctrl-C` | Cancel |
| Text input | Fuzzy search |

### Tab filtering

Tabs are grouped by **the source config file each host came from** by default. Press `Ctrl-T` (or pass `--tab-source tag` / set `FFH_TAB_SOURCE=tag`) to group by the **`Tag` directive** instead.

```
  [ All ]  [ dev ]  [ prod ]
```

- **All** — show every host (default)
- **tag name / source file** — show only hosts belonging to that group, depending on the active grouping mode

Switch tabs with `Tab` / `Shift-Tab`.

### Preview pane

Focusing a host shows its details in the left pane, including last-used time and connection count if it has connection history:

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
  Last Used:      3d ago (connected x5)

  ────────────────────────────────
  Description
  Production web server
  See the wiki for details
```

### SSH config view and inline edit (Ctrl-G)

Pressing `Ctrl-G` on a focused host opens a nested fzf showing every resolved SSH option from `ssh -G <host>`. Focusing an option line displays its description (English/Japanese) in the right preview pane.

Pressing `Enter` on an option line opens a small input dialog to edit its value. On save, the directive is written to the host's source config file and validated with `ssh -G`; if the result is invalid, the change is rolled back automatically.

### Clipboard copy (Ctrl-Y)

`Ctrl-Y` copies the `ssh` command for the focused host (including `-l`/`-p`/`-J` as applicable) to the system clipboard. Requires one of `wl-copy`, `xclip`, `xsel`, or `pbcopy`.

### TCP reachability check (Ctrl-P)

`Ctrl-P` switches the preview pane to show a TCP connectivity check (UP/DOWN and response time) against the focused host's SSH port.

### Connection history (--history)

```bash
ffh --history                     # select from history and connect
ffh --history --delete myserver   # delete a history entry
```

Every successful connection is recorded to `~/.local/share/ffh/history.json`. The history list shows last-used time and connection count; press `Ctrl-D` to delete an entry.

### Duplicate host detection (--check)

```bash
ffh --check
```

Scans all `Include`d config files for `Host` names defined more than once, and shows which definition is effective (the first one encountered) versus which are ignored.

### Run a command on a tag (--exec)

```bash
ffh --exec web uptime
```

Runs the same command over SSH on every host with the given `Tag`, in parallel, prefixing each line of output with the host name.

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

### Choosing the SSH config file

Resolved in this priority order:

| Priority | Method | Example |
|----------|--------|---------|
| 1 | CLI argument `-F` | `ffh -F ~/work/ssh_config` |
| 2 | `FFH_SSH_CONFIG` env var | `export FFH_SSH_CONFIG=/path/to/ssh_config` |
| 3 | Config file `~/.config/ffh/config` | `ssh_config = /path/to/ssh_config` |
| 4 | Default | `~/.ssh/config` |

---

## Language

The default is Japanese if the system `LANG` starts with `ja`, otherwise English. Override with:

| Priority | Method | Example |
|----------|--------|---------|
| 1 | `FFH_LANG` env var | `FFH_LANG=ja ffh` |
| 2 | Config file `~/.config/ffh/config` | `language = ja` |
| 3 | System `LANG` | Japanese if it starts with `ja` |

**Config file example** (`~/.config/ffh/config`):

```ini
# ffh config
hosts_file = /path/to/hosts
ssh_config = /path/to/ssh_config
tab_source = tag
language = ja
```

---

## SSH config reference

### Tag — tab filtering

```ssh-config
Host myserver
    HostName 10.0.0.1
    Tag prod
```

Hosts sharing the same `Tag` value are grouped under that tab (when tab grouping is switched to `Tag` mode via `Ctrl-T`).

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
