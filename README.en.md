# ffh

English | [日本語](README.md)

A CLI tool that parses `~/.ssh/config` and lets you interactively select an SSH host via fzf.

## Features

- Recursively resolves `Include` directives in `~/.ssh/config`
- **Left preview pane** showing host details while browsing
- Tab filtering grouped by **source config file** (default) or by the **`Tag` directive** — toggle with `Ctrl-T`, fuzzy-jump to a tab by name with `Ctrl-/`
- Short persistent hint below the tab bar, plus a `?` overlay listing every key binding
- Multi-line host descriptions via `# Description:` comments
- Multiple hostnames on a single `Host` line are expanded into separate entries
- Connection history with quick reconnect (`--history`)
- Duplicate host definition detection (`--check`)
- Run a command on every host with a given tag (`--exec`)
- Copy the resolved `ssh` command to the clipboard, check TCP reachability, and view/edit `ssh -G` output inline
- Hosts file mode (path configurable via CLI, environment variable, or config file)
- UI language switchable between English and Japanese
- Automatic password entry via 1Password (`op` CLI) integration (enabled only when `op_vault` is set)

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
| `Ctrl-/` | Fuzzy-search tab names and jump straight to one |
| `?` | Show the full key-binding list as an overlay |
| `Tab` | Move to next tab |
| `Shift-Tab` | Move to previous tab |
| `Esc` / `Ctrl-C` | Cancel |
| Text input | Fuzzy search |

A short hint (`Ctrl-/:jump tab  ?:help`) stays visible right below the tab bar. Press `?` to open a full key-binding overlay (`Esc` or `Enter` to close). The persistent hint is kept intentionally short so it doesn't visually blend into the tab bar right above it.

### Tab filtering

Tabs are grouped by **the source config file each host came from** by default. Press `Ctrl-T` (or pass `--tab-source tag` / set `FFH_TAB_SOURCE=tag`) to group by the **`Tag` directive** instead.

```
  [ All ]  [ dev ]  [ prod ]
  Ctrl-/:jump tab  ?:help
```

- **All** — show every host (default)
- **tag name / source file** — show only hosts belonging to that group, depending on the active grouping mode

Switch tabs one at a time with `Tab` / `Shift-Tab`. When there are many tabs, press `Ctrl-/` to open a fuzzy-searchable list of tab names (k9s-style command bar) and jump straight to the one you want.

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

Runs the same command over SSH on every host with the given `Tag`, in parallel, prefixing each line of output with the host name. If `op_vault` is set, per-host password-manager integration applies here too, but since multiple hosts connect concurrently, the "not signed in" confirmation prompt is never shown — an affected host just falls back to ssh's normal interactive password prompt.

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
tag_delimiter = /
language = ja
op_vault = Private
```

### Automatic password entry via 1Password

When `op_vault` is set, ffh fetches the password from 1Password (`op` CLI) via `SSH_ASKPASS` and enters it automatically for hosts that need password authentication (requires an active `op` sign-in session). 1Password is the only password manager supported today, but internally this isn't hardwired to 1Password specifically — the implementation is structured so other password managers (e.g. Bitwarden) can be added later (see "Credential Integration" in `AGENTS.md` for details).

The 1Password item name is not registered per host — it's simply the **effective `User` resolved via `ssh -G <host>`**. If several hosts log in as the same user, you only need one item in 1Password.

- Item name = effective `User` (e.g. hosts that log in as `pocuser` use the 1Password item named `pocuser`, reading its `password` field)
- The rare exception — same username, different password — can be overridden per host with `SetEnv FFH_CREDENTIAL=<item name>` (see next section)
- If that item also has a `username` field set, it takes priority over ssh_config's own `User` (passed as `-l`). This only matters for hosts that already override the item via `SetEnv FFH_CREDENTIAL` for a shared login; an explicit `-l`/`-o User=` on the command line always wins over the 1Password value
- If no matching 1Password item exists for the resolved user, ffh does nothing and falls back to ssh's normal interactive password prompt and ssh_config's own `User` (key-only hosts are unaffected)
- If `op` isn't signed in (needs `op signin`), ffh shows a message to that effect before falling back — distinct from the "no matching item" case above, which stays silent. You're then asked whether to run `op signin` right away and retry, or just continue with ssh's normal interactive password prompt without signing in. Choosing to authenticate runs `op signin`, and on success the connection proceeds with 1Password's automatic password entry
- `op_vault` can be overridden with the `FFH_OP_VAULT` environment variable

---

## SSH config reference

### Tag — tab filtering

```ssh-config
Host myserver
    HostName 10.0.0.1
    Tag prod
```

Hosts sharing the same `Tag` value are grouped under that tab (when tab grouping is switched to `Tag` mode via `Ctrl-T`).

#### `tag_delimiter` — splitting one Tag into multiple tabs

```ssh-config
Host myserver
    HostName 10.0.0.1
    Tag /hoge/fuga/
```

A host's `Tag` value is split on `/` by default, and each resulting piece becomes its own tab. In the example above, `myserver` shows up under both the `hoge` tab and the `fuga` tab. Empty pieces from a leading/trailing delimiter are dropped, so `/hoge/fuga/` becomes `["hoge", "fuga"]`, not `["", "hoge", "fuga", ""]`. A plain `Tag` without the delimiter (e.g. `prod`) still becomes a single tab, same as before.

```ini
# ~/.config/ffh/config
tag_delimiter = ,
```

- Default is `/`; change it with `tag_delimiter` (config file) or `FFH_TAG_DELIMITER` (env var)
- To disable splitting entirely and always use the whole `Tag` value as one tab, set `tag_delimiter = off` / `FFH_TAG_DELIMITER=off`
- `ffh --exec <tag> <command>` uses the same splitting logic for its tag match, so `ffh --exec hoge <cmd>` also matches a host with `Tag /hoge/fuga/`

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

### SetEnv FFH_CREDENTIAL — overriding the 1Password item name

```ssh-config
Host poc-str1_agg1_dc4
    HostName 192.168.255.240
    User root
    SetEnv FFH_CREDENTIAL=root-str1agg
```

When `op_vault` is set, the 1Password item name defaults to the effective `User`, but if the same username actually has different passwords on different hosts, override it per host with `SetEnv FFH_CREDENTIAL=<item name>`. `SetEnv` is a native SSH directive (OpenSSH 7.8+), so it doesn't cause syntax errors under `ssh -G` and can also be placed inside a `Match` block covering several hosts at once.

#### ⚠️ Careful with a catch-all block like `Match all`

If you use an unconditional default block such as `Match all` to set `User`/`IdentityFile`/etc. for every host, putting `SetEnv FFH_CREDENTIAL=<item name>` there means **key-only hosts inherit it too**. ssh_config uses the first value it finds for a given keyword, so unless a more specific `Host` block already sets its own `SetEnv FFH_CREDENTIAL`, that host ends up pointed at the same 1Password item.

This is especially risky for hosts whose private key has a passphrase: `SSH_ASKPASS_REQUIRE=force` also intercepts the key-passphrase prompt, so the unrelated 1Password value gets fed in and breaks key auth. If public-key auth fails for any other reason, ssh falls through to password auth and auto-submits the wrong password — risking a lockout on devices with a lockout policy.

The recommended fix is to scope `SetEnv FFH_CREDENTIAL=<item name>` to a `Match` block covering only the hosts that actually need it, rather than a global default. If you can't remove it from a shared default block for other reasons, disable it per host with `off` (next section).

### SetEnv FFH_CREDENTIAL=off / empty value — disabling per host

```ssh-config
Host keyonly-host
    HostName 192.168.10.5
    User devuser
    SetEnv FFH_CREDENTIAL=off
```

Setting `FFH_CREDENTIAL` to the reserved value `off` (case-insensitive), or to an empty value (`SetEnv FFH_CREDENTIAL=`), makes that host skip 1Password integration entirely and fall back to normal key auth / an interactive password prompt. Both behave identically — `off` reads clearly, an empty value is the quicker option if you're just deleting an existing item name. Either way, the disabling `Host` block must be resolved *before* a catch-all default like `Match all` in the file (ssh_config's "first value wins" rule), otherwise the catch-all's value still wins.

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

---

## For developers

Internal architecture details (file layout, the SSH config parser's state machine, the tab feature and 1Password integration's implementation flow, etc.) are documented in [DEVELOPMENT.md](DEVELOPMENT.md) (Japanese only) and [AGENTS.md](AGENTS.md) (English).
