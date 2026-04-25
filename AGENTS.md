# ffh — Agent Harness Documentation

## Purpose

ffh is a Go CLI tool that wraps `ssh` with `fzf`-based interactive host selection
from `~/.ssh/config` (including `Include` directives) and optionally from
`/mnt/c/Windows/System32/drivers/etc/hosts` (WSL mode).

The left preview pane in fzf displays HostName, User, Port, ProxyJump, IdentityFile,
Description, and Source file for the currently highlighted host.

## Build

Requires: Go 1.24+, `fzf` (apt install fzf), `make`

```
make build         # produces ./ffh binary
make install       # installs to /usr/local/bin/ffh (may need sudo)
```

## Usage

```
ffh [ssh-args]          # interactive host selection from SSH config
ffh --hosts [ssh-args]  # interactive selection from Windows hosts file
```

## Internal flags (used by fzf preview, not for direct use)

```
ffh --preview-host <name>         # print host details to stdout; called by fzf preview
ffh --ssh-config-view <hostname>  # open nested fzf with ssh -G output; called by Ctrl-G execute binding
ffh --preview-option <option-line> # print Japanese description of an SSH option; called by nested fzf preview
```

## File Structure

| File | Purpose |
|---|---|
| `main.go` | Entry point, flag dispatch, fzf subprocess, ssh exec |
| `parser.go` | SSH config parser (Include resolution, Host block extraction) |
| `hosts.go` | Windows hosts file reader for --hosts mode |

## Testing

```
go test ./...    # or: make test
```

Unit tests cover:
- parser.go: Include glob resolution, wildcard skipping, Description extraction,
  case-insensitive keywords, back-to-back Host blocks, no-trailing-newline files
- hosts.go: Loopback filtering, multi-name lines, comment/blank skipping

Tests do NOT require fzf or ssh to be installed.

## SSH Config Parsing Notes

- Include paths with `~/` are expanded to `$HOME`
- Relative Include paths are resolved relative to the config file's directory
- Host patterns containing `*` or `?` are skipped
- `Match` blocks are skipped entirely
- Keywords are matched case-insensitively
- Description is parsed from `# Description: ...` on the line immediately above
  the `Host` line (no blank lines between comment and Host)
- First occurrence wins for duplicate host names across included files
