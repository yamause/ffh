# ffh — Agent Harness Documentation

## Purpose

ffh is a Go CLI tool that wraps `ssh` with `fzf`-based interactive host selection
from `~/.ssh/config` (including `Include` directives), a hosts file (default
`/etc/hosts`, `--hosts` mode), or connection history (`--history`).

The left preview pane in fzf displays HostName, User, Port, ProxyJump, IdentityFile,
Tag, Source file, and (if available) last-used time / connection count for the
currently highlighted host.

## Build

Requires: Go 1.24+, `fzf` (apt install fzf), `make`

```
make build         # produces ./ffh binary
make install       # installs to /usr/local/bin/ffh (may need sudo)
```

## Usage

```
ffh [-F <file>] [--tab-source tag|source] [-- ssh-args]   interactive selection from SSH config
ffh --hosts [path] [-- ssh-args]                          interactive selection from a hosts file
ffh --history [-- ssh-args]                               interactive selection from connection history
ffh --history --delete <host>                             delete a history entry
ffh --check [-F <file>]                                   detect duplicate host definitions
ffh --exec <tag> <command...>                             run a command on every host with the given tag
```

## Internal flags (used by fzf preview/bind callbacks, not for direct use)

```
ffh --preview-host <name> [<sshconfig>]              print host details to stdout; called by fzf preview
ffh --ssh-config-view <hostname> [<sshconfig>]       open nested fzf with ssh -G output; called by Ctrl-G execute binding
ffh --preview-option <option-line>                   print localized description of an SSH option; called by nested fzf preview
ffh --edit-host-option <host> <sshconfig> <kw> [val] open inline edit dialog for one directive; called by Enter inside Ctrl-G view
ffh --tab-list <statefile> <delta> [<sshconfig>]     advance tab index and print header+hosts; called by Tab/Shift-Tab reload
ffh --tab-source-toggle <statefile> [<sshconfig>]    toggle tab grouping (tag/source) and print header+hosts; called by Ctrl-T reload
ffh --check-host <name> [<sshconfig>]                print TCP UP/DOWN status; called by Ctrl-P preview
ffh --copy-ssh-cmd <name> [<sshconfig>]              copy resolved ssh command to clipboard; called by Ctrl-Y execute
ffh --history --list                                 print history lines; called by Ctrl-D reload in history mode
```

## File Structure

| File | Purpose |
|---|---|
| `main.go` | Entry point, flag dispatch, fzf subprocess/bindings, tab state, clipboard, host check, inline directive editing, ssh exec |
| `parser.go` | SSH config parser (Include resolution, Host block extraction, multi-hostname expansion, Description extraction) |
| `hosts.go` | hosts(5) file reader for `--hosts` mode (loopback filtered out) |
| `config.go` | Resolution of SSH config path, hosts file path, tab-source, and `~/.config/ffh/config` parsing |
| `history.go` | Connection history persisted to `~/.local/share/ffh/history.json` |
| `i18n.go` | English/Japanese message tables and help text; language resolution |
| `ssh_options.go` | Localized descriptions for `ssh -G` output, shown in the Ctrl-G nested preview |

## Testing

```
go test ./...    # or: make test
```

Unit tests cover:
- parser.go: Include glob resolution, wildcard skipping, Description extraction,
  case-insensitive keywords, back-to-back Host blocks, no-trailing-newline files,
  multi-hostname `Host` lines, duplicate host names (first-match-wins)
- hosts.go: Loopback filtering, multi-name lines, comment/blank skipping
- config.go: SSH config / hosts file / tab-source / language resolution priority
- history.go: record/find/delete/sort of history entries
- main.go (editor_test.go): inline directive edit + rollback on `ssh -G` syntax error

Tests do NOT require fzf; a few editor tests skip themselves if `ssh` is not in PATH.

## SSH Config Parsing Notes

- Include paths with `~/` are expanded to `$HOME`
- Relative Include paths are resolved relative to the config file's directory
- Host patterns containing `*` or `?` are skipped
- Multiple hostnames on one `Host` line are expanded into separate `Host` entries
  sharing the same directives
- `Match` blocks are skipped entirely
- Keywords are matched case-insensitively
- Description is parsed from `# Description: ...` on the line immediately above
  the `Host` line (no blank lines between comment and Host)
- First occurrence wins for duplicate host names across included files
- Default tab grouping is by source config file (`--tab-source source`); pass
  `--tab-source tag` / `FFH_TAB_SOURCE=tag` / `tab_source = tag` to group by `Tag` instead
