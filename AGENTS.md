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
ffh --tab-jump <statefile>                           open nested fzf to fuzzy-select a tab by name and set it current; called by Ctrl-/ execute (paired with a --tab-list delta=0 reload)
ffh --show-help                                      open nested fzf listing every sshMode key binding; called by "?" execute
ffh --check-host <name> [<sshconfig>]                print TCP UP/DOWN status; called by Ctrl-P preview
ffh --copy-ssh-cmd <name> [<sshconfig>]              copy resolved ssh command to clipboard; called by Ctrl-Y execute
ffh --history --list                                 print history lines; called by Ctrl-D reload in history mode
```

## File Structure

| File | Purpose |
|---|---|
| `main.go` | Entry point, flag dispatch, host check, `sshMode`/`hostsMode`/`historyMode`/`execTag`, ssh exec |
| `tabs.go` | Tab state persistence, tab-bar rendering (`renderHeader`'s sliding window), tab list/jump/toggle fzf reload handlers, the `?` help overlay |
| `clipboard.go` | `Ctrl-Y` copy-ssh-command support: building the command string and writing it to the system clipboard |
| `editor.go` | `Ctrl-G` nested ssh -G view, its per-option preview, and the inline directive-edit-with-rollback flow (`editHostOption`/`updateHostDirective`) |
| `parser.go` | SSH config parser (Include resolution, Host block extraction, multi-hostname expansion, Description extraction) |
| `hosts.go` | hosts(5) file reader for `--hosts` mode (loopback filtered out) |
| `config.go` | Resolution of SSH config path, hosts file path, tab-source, and `~/.config/ffh/config` parsing |
| `history.go` | Connection history persisted to `~/.local/share/ffh/history.json` |
| `i18n.go` | English/Japanese message tables and help text; language resolution |
| `ssh_options.go` | Localized descriptions for `ssh -G` output, shown in the Ctrl-G nested preview |
| `credential.go` | Backend-agnostic credential resolution (`credentialBackend` interface, ssh_config item resolution, `SSH_ASKPASS` self-invocation) |
| `credential_op.go` | `opBackend`: the `credentialBackend` implementation for 1Password's `op` CLI |

## Testing

```
go test ./...    # or: make test
```

Unit tests cover:
- parser.go: Include glob resolution, wildcard skipping, Description extraction,
  case-insensitive keywords, back-to-back Host blocks, no-trailing-newline files,
  multi-hostname `Host` lines, duplicate host names (first-match-wins)
- hosts.go: Loopback filtering, multi-name lines, comment/blank skipping
- config.go: SSH config / hosts file / tab-source / tag-delimiter / language
  resolution priority
- history.go: record/find/delete/sort of history entries
- editor.go (editor_test.go): inline directive edit + rollback on `ssh -G` syntax
  error; `hostBlockMatches` matching any name on a multi-hostname `Host` line
  (including a wildcard-first line like `Host web* db-1`), not just the first token
- tabs.go (tabs_test.go): `tagSegments`/`buildTabState`/`filterHosts` behavior with
  and without a configured `tag_delimiter`; `tabIndexByLabel` matching in both "tag"
  and "source" grouping modes; `renderHeader` always emitting the tab-bar line plus
  the key-hints line (2 lines total); `formatHelpLines` column-alignment to the
  longest `Key` string
- main.go (main_test.go): `hasLoginOverride` detection of an explicit `-l`/`-o User=`
  in the caller's ssh-args; `credentialSSHArgs`' `-l <username>` override (and the
  caller's own `-l`/`-o User=` outranking it)
- credential.go: backend-agnostic `ssh -G` output parsing (SetEnv override vs.
  resolved-user fallback, the `off`/empty-value disable sentinel including precedence
  over a catch-all `Match all` block), and the no-backend-configured/no-ssh-config-path
  nil cases of `resolveCredential`
- credential_op_test.go: `opBackend`-specific coverage -- `op_vault` resolution
  priority, `op`/`ssh`-dependent `fetchSecret`/`isAuthError`/`runAskpass` paths
  (skipped if the respective binary is not in PATH), the `notSignedIn` signal via a
  fake `op` script, and `applyOpSessionEnv` parsing. The username-override path
  itself (a 1Password item's `username` field differing from ssh_config's `User`)
  isn't covered by an automated test — it needs a live signed-in `op` session against
  a real fixture item, same limitation as the existing password-fetch tests.

Tests do NOT require fzf; a few editor and credential tests skip themselves if
`ssh` or `op` is not in PATH.

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
- In `Tag` grouping mode, a single `Tag` value is split into multiple tab keys via
  `tagSegments` (tabs.go), using `/` as the delimiter by default: a host tagged
  `/hoge/fuga/` appears under both the `hoge` and `fuga` tabs. Override the delimiter
  with `tag_delimiter` (config file) / `FFH_TAG_DELIMITER` (env var), or set either to
  `off` (case-insensitive, normalized in `normalizeTagDelimiter`) to disable splitting
  and use each `Tag` value whole, as before this feature existed. `buildTabState` and
  `filterHosts` both take an explicit `tagDelimiter` argument rather than resolving it
  internally, to keep them pure/unit-testable; callers fetch it once via
  `resolveTagDelimiter()` (config.go). `execTag` (the `--exec <tag>` backend) uses the
  same `tagSegments` matching for consistency with tab filtering.
- `renderHeader` (tabs.go) always emits two lines: the tab bar, then a dim, always-
  visible key hint line (`renderKeyHints`, sourced from `msgs.keyHintsSSH`).
  `sshMode`'s fzf invocation therefore uses `--header-lines=2`, and every reload path
  that re-prints `renderHeader`'s output (`tabList`, `tabSourceToggle`) must keep
  emitting both lines or the outer fzf's header parsing gets out of sync.
  `keyHintsSSH` is intentionally just `Ctrl-/:jump tab  ?:help` — a full inline list
  of every binding was tried first but visually blended into the tab bar right above
  it (dim-on-dim, no separation); the full list moved to the `?` overlay instead
  (`showHelp`) so the persistent line stays short and legible.
- `Ctrl-/` is bound to a k9s-style "jump to tab by name" command: it `execute()`s
  `ffh --tab-jump <statefile>` (a nested fzf over the current tab labels, matched back
  to an index via `tabIndexByLabel`), then chains `+reload(--tab-list ... 0 ...)` to
  re-render at the newly-selected index. A literal `:` could not be used as the key
  (fzf's own `--bind` syntax uses `:` as the KEY:ACTION separator, so a bare `:` key
  spec is rejected with "key name required" — verified empirically against fzf 0.44.1);
  `ctrl-/` was chosen as an unused, conventionally "search/command"-associated key
  instead, since any plain printable character would also collide with the main fzf's
  fuzzy-search query input.
- `?` opens `showHelp` (tabs.go): a nested fzf listing every `sshMode` key binding
  from `msgs.helpKeyBindings` ( `[]keyBinding{Key, Action}` ), column-aligned by
  `formatHelpLines` to the width of the longest `Key` string so the layout stays
  correct across languages regardless of translated string length. Unlike other
  printable-character concerns, `?` is bound directly (not via a Ctrl-combo) since
  reserving `?` for help is a strong, near-universal convention (vim, less, k9s) and
  host names essentially never contain a literal `?`.

## Credential (password manager) Integration

Password-manager access is behind a `credentialBackend` interface (credential.go),
so ffh isn't hardwired to 1Password specifically:

```go
type credentialBackend interface {
	name() string                                        // e.g. "op"; used in FFH_CRED_BACKEND
	displayName() string                                  // e.g. "1Password"; used in messages
	vault() string                                        // "" if this backend isn't configured
	fetchSecret(vault, item, field string) (string, error)
	isAuthError(err error) bool                           // "not signed in" vs. e.g. missing item
	signin() error                                        // interactive auth, updates process env
	askpassEnv(vault, item string) []string               // env for the SSH_ASKPASS re-invocation
}
```

`opBackend` (credential_op.go) is the only implementation today. `credentialBackends`
(credential.go) is the registry `resolveCredentialBackend` polls in order to find the
first one with a non-empty `vault()`; adding a new backend (e.g. a future
`bwBackend` for Bitwarden's `bw` CLI in `credential_bitwarden.go`) means implementing
the interface and appending it to that slice — `resolveCredential`, `execSSH`, and
`runAskpass` need no changes. Each backend owns its own config-key/env-var naming
end to end (`opBackend.vault()` calls `resolveOpVault()` in config.go, which reads
`op_vault` / `FFH_OP_VAULT` — kept as-is for backward compatibility); a Bitwarden
backend would introduce its own `bw_vault` / `FFH_BW_VAULT` independently.

- Disabled by default; enabled by setting `op_vault` (config file `op_vault = <vault>`
  or `FFH_OP_VAULT` env var). No behavior change when unset.
- The password-manager item name is the effective `User`, resolved via a single
  `ssh -F <config> -G <host>` call (parser.go's `Host.User` is not used here since it
  is usually empty — `User` is normally set via `Match Tagged` in the main config,
  and the parser skips `Match` blocks entirely). This resolution (`resolveCredentialItem`)
  is backend-agnostic and shared by whichever backend is configured.
- A `SetEnv FFH_CREDENTIAL=<item>` directive on the `Host` (or a covering `Match`)
  overrides the item name — the one supported way to special-case a host without a
  custom ssh_config keyword (unknown keywords make `ssh -G` fail outright).
- `SetEnv FFH_CREDENTIAL=off` (case-insensitive) or an empty value
  (`SetEnv FFH_CREDENTIAL=`, also valid ssh_config syntax) is a reserved sentinel
  that opts a host out of credential resolution entirely — `parseCredentialItem`
  treats either as an unresolvable item, so `resolveCredential` returns `(nil, nil,
  false)` exactly as it would for "no matching item". Needed because a catch-all
  `Match all` default block (commonly used to set `User`/`IdentityFile`/etc. for
  every host) can also set `FFH_CREDENTIAL` for every host, including key-only ones.
  Since ssh_config keeps the first-obtained value per key, the opt-out only wins
  when it's resolved before the catch-all — i.e. the host-specific `Host` block with
  `SetEnv FFH_CREDENTIAL=off` (or `=`) must appear earlier in the config (or its
  Includes) than the `Match all` block. Prefer scoping `FFH_CREDENTIAL=<item>` to
  only the hosts that actually need it (a dedicated `Match Tag <x>` block, not a
  global default) over relying on the disable sentinel as a per-host patch — it's
  the safety net, not the primary defense.
- `resolveCredential` (credential.go) validates the password field is fetchable
  *before* touching the environment; on any failure (no matching item, backend not
  signed in, etc.) it returns `(nil, nil, false)` and `execSSH` proceeds with ssh's
  normal interactive prompt and ssh_config's own `User` — never breaks a key-only
  connection. Its second and third return values, `backend` and `notSignedIn`, are
  non-nil/true only when the configured backend's `isAuthError` classifies the
  failure as "not signed in" (op's checks the `op` CLI's stderr text, captured for
  free through `*exec.ExitError.Stderr` since `opBackend.fetchSecret` never sets
  `cmd.Stderr` itself) — as opposed to e.g. no matching item, the ordinary case for
  key-only hosts, which stays fully silent (`backend` is nil in that case).
- When `notSignedIn` is true, `execSSH` calls `confirmCredSignin(backend)`
  (credential.go), which prints `msgs.warnCredNotSignedIn`/`msgs.promptCredSignin`
  (both `func(backendDisplayName string) string`, so the message names whichever
  backend is in play) and reads a y/n line from stdin — safe to do at this point
  because fzf reads its candidate list from an in-memory reader, never from
  `os.Stdin` (see `runFzf`), so stdin is still the real terminal when `execSSH` runs.
  On "y", `backend.signin()` runs the backend's own interactive auth flow attached to
  the real stdin/stderr (op's implementation runs `op signin`, parses any
  `export OP_SESSION_<account>=...` line from its stdout via `applyOpSessionEnv`, and
  sets it with `os.Setenv` — a no-op for desktop-app-integrated accounts, which print
  no such line), then `execSSH` calls `resolveCredential` again before building the
  real ssh invocation. Declining, or a failed/unavailable signin, just leaves `cred`
  nil and the connection proceeds without the password manager, same as the
  silent-fallback case. The `FFH_ASKPASS_MODE` branch in `main()` looks the backend
  back up via `credentialBackendByName(os.Getenv("FFH_CRED_BACKEND"))` and reuses its
  `isAuthError` too, for the rarer case where the session drops between
  `resolveCredential`'s check and the actual askpass invocation — but doesn't prompt
  there, since SSH_ASKPASS is invoked by ssh itself with no interactive terminal
  guaranteed.
- If the resolved item also has a non-empty `username` field, `execSSH` passes it as
  `-l <username>` on the real ssh invocation, which outranks ssh_config's `User`
  directive. This only matters for hosts using the `SetEnv FFH_CREDENTIAL` override
  above (the default item-name-equals-`User` resolution would just fetch the same
  value back). An explicit `-l`/`-o User=` already present in the caller's own
  ssh-args (`hasLoginOverride`/`credentialSSHArgs` in main.go) always wins over the
  credential-provided username.
- When enabled, `execSSH` sets `SSH_ASKPASS_REQUIRE=force` and points `SSH_ASKPASS`
  at ffh's own executable (`selfPath()`), via the backend's own `askpassEnv(vault,
  item)` (`opBackend`'s also sets `FFH_CRED_BACKEND=op`). `main()` checks
  `FFH_ASKPASS_MODE=1` before any other dispatch and, if set, runs as the askpass
  helper instead of the normal fzf UI (see `runAskpass` in credential.go).
  Backend/vault/item are passed via `FFH_CRED_BACKEND`/`FFH_CRED_VAULT`/`FFH_CRED_ITEM`
  env vars, not CLI args, since `SSH_ASKPASS` only supports a bare executable path;
  `runAskpass` looks the backend back up with `credentialBackendByName` before calling
  its `fetchSecret`. The `username` field, unlike `password`, isn't a secret in this
  flow — it's read directly in `resolveCredential` and passed as a plain CLI argument
  rather than through the askpass indirection.
- `execTag` (main.go, the `--exec <tag>` backend) also resolves and applies a
  credential per host, via the same `credentialSSHArgs` helper `execSSH` uses — but
  it never calls `confirmCredSignin`/`backend.signin()`: targets run concurrently
  (one goroutine per host), and a per-host interactive sign-in prompt against N hosts
  at once wouldn't make sense. A backend that isn't signed in just means that one
  host connects without it, same as the ordinary "no matching item" fallback.
