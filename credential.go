package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// credentialDisableSentinel is a reserved SetEnv FFH_CREDENTIAL value that opts a
// host out of credential resolution entirely, even when a covering Match block (e.g.
// a catch-all "Match all" default) would otherwise supply an item name for it. An
// empty value (SetEnv FFH_CREDENTIAL=) has the same effect, for the common case of
// just deleting the item name rather than typing a specific word. First-obtained-
// value ssh_config semantics mean either form only wins if it is resolved before the
// block supplying the item -- normally satisfied by placing a host-specific override
// earlier in the file than a catch-all default.
const credentialDisableSentinel = "off"

// credentialBackend abstracts a password-manager CLI (1Password's `op` today; a
// future backend such as Bitwarden's `bw` implements the same interface) so
// resolveCredential's ssh_config-driven item resolution and the SSH_ASKPASS plumbing
// stay backend-agnostic. Adding a backend means: implement this interface (see
// credential_op.go for the reference implementation), then add it to
// credentialBackends below -- nothing else in this file changes.
type credentialBackend interface {
	// name identifies this backend in the FFH_CRED_BACKEND env var passed to the
	// SSH_ASKPASS re-invocation, so runAskpass can pick the same implementation
	// back out (e.g. "op").
	name() string
	// displayName is used in user-facing messages, e.g. "1Password".
	displayName() string
	// vault returns this backend's configured vault name, or "" if unconfigured.
	// Config-key/env-var naming and resolution priority is entirely up to the
	// implementation (op's is resolveOpVault in config.go).
	vault() string
	// fetchSecret returns the value of the named field of item in vault.
	fetchSecret(vault, item, field string) (string, error)
	// isAuthError reports whether err indicates this backend's CLI session isn't
	// authenticated, as opposed to e.g. a missing item -- the ordinary, silent
	// fallback case for key-only hosts.
	isAuthError(err error) bool
	// signin runs this backend's interactive authentication flow attached to the
	// current terminal and, on success, updates the process environment so
	// subsequent fetchSecret calls in this process (including the SSH_ASKPASS
	// re-invocation, which inherits os.Environ()) are authenticated.
	signin() error
	// askpassEnv returns the extra environment variables execSSH must set so the
	// re-invoked ffh (FFH_ASKPASS_MODE=1) can fetch the same secret via runAskpass.
	askpassEnv(vault, item string) []string
}

// credentialBackends lists every supported password-manager backend, polled in
// order by resolveCredentialBackend. Only one is expected to be configured at a
// time; order only matters as a tie-breaker if more than one vault is set.
var credentialBackends = []credentialBackend{
	opBackend{},
}

// resolveCredentialBackend returns the first configured backend (non-empty vault)
// among credentialBackends, or (nil, "") if none is configured.
func resolveCredentialBackend() (credentialBackend, string) {
	for _, b := range credentialBackends {
		if vault := b.vault(); vault != "" {
			return b, vault
		}
	}
	return nil, ""
}

// credentialBackendByName returns the backend whose name() matches, or nil.
func credentialBackendByName(name string) credentialBackend {
	for _, b := range credentialBackends {
		if b.name() == name {
			return b
		}
	}
	return nil
}

// resolveCredentialItem determines the password-manager item name to use for
// hostAlias by running `ssh -F sshConfigPath -G hostAlias` once and reading the
// resolved config. A "setenv FFH_CREDENTIAL=<item>" line (set via a native SSH
// `SetEnv` directive) takes priority, for the rare case where a shared login user
// has more than one password in use; otherwise the resolved "user" value is used,
// since passwords here are managed per login user rather than per host.
func resolveCredentialItem(sshConfigPath, hostAlias string) (string, error) {
	out, err := exec.Command("ssh", "-F", sshConfigPath, "-G", hostAlias).Output()
	if err != nil {
		return "", fmt.Errorf("ssh -G %s: %w", hostAlias, err)
	}
	return parseCredentialItem(string(out))
}

// parseCredentialItem extracts the password-manager item name from `ssh -G` output text.
func parseCredentialItem(sshGOutput string) (string, error) {
	user := ""
	for _, line := range strings.Split(sshGOutput, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case "setenv":
			for _, kv := range fields[1:] {
				k, v, ok := strings.Cut(kv, "=")
				if ok && k == "FFH_CREDENTIAL" {
					if v == "" || strings.EqualFold(v, credentialDisableSentinel) {
						return "", fmt.Errorf("credential resolution disabled via SetEnv FFH_CREDENTIAL=%q", v)
					}
					return v, nil
				}
			}
		case "user":
			user = fields[1]
		}
	}
	if user == "" {
		return "", fmt.Errorf("could not resolve an effective user")
	}
	return user, nil
}

// credential holds what execSSH needs to hand a connection off to the resolved
// password-manager backend: extra environment variables that arm the SSH_ASKPASS
// helper, and optionally a login username to use instead of ssh_config's own User
// resolution.
type credential struct {
	env      []string
	username string // empty when the item has no (or an empty) username field
}

// resolveCredential determines the password-manager-backed credential for
// hostAlias, or nil when no backend is configured (no vault set for any backend),
// unreachable, or no matching item exists for hostAlias. On nil, callers must leave
// ssh's normal interactive prompt and ssh_config's own User resolution untouched --
// this is what keeps a password-manager outage non-fatal to any connection. The
// second return value is the backend that was attempted, non-nil only when the
// third return value (notSignedIn) is true, so callers can offer to authenticate
// via that backend and retry instead of just falling back.
func resolveCredential(sshConfigPath, hostAlias string) (cred *credential, backend credentialBackend, notSignedIn bool) {
	if sshConfigPath == "" {
		return nil, nil, false
	}
	b, vault := resolveCredentialBackend()
	if b == nil {
		return nil, nil, false
	}
	item, err := resolveCredentialItem(sshConfigPath, hostAlias)
	if err != nil {
		return nil, nil, false
	}
	// Validate the secret is actually retrievable before forcing askpass; the value
	// itself is discarded here and re-fetched inside the askpass sub-invocation so it
	// never sits in this process's (or ssh's) environment.
	if _, err := b.fetchSecret(vault, item, "password"); err != nil {
		return nil, b, b.isAuthError(err)
	}
	cred = &credential{env: b.askpassEnv(vault, item)}
	// A missing/empty username field (e.g. a "Password"-category item) is not an
	// error: the connection simply keeps using ssh_config's own User.
	if username, err := b.fetchSecret(vault, item, "username"); err == nil && username != "" {
		cred.username = username
	}
	return cred, nil, false
}

// confirmCredSignin tells the user backend isn't signed in and asks whether to
// authenticate now, before execSSH falls back to ssh's own interactive prompt. Reads
// a single line from stdin (still connected to the real terminal at this point --
// fzf reads its candidate list from an in-memory pipe, never from os.Stdin); anything
// other than y/yes is treated as "no".
func confirmCredSignin(backend credentialBackend) bool {
	fmt.Fprintln(os.Stderr, msgs.warnCredNotSignedIn(backend.displayName()))
	fmt.Fprint(os.Stderr, msgs.promptCredSignin(backend.displayName()))
	reply, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	reply = strings.ToLower(strings.TrimSpace(reply))
	return reply == "y" || reply == "yes"
}

// runAskpass implements the SSH_ASKPASS protocol: print the requested secret to
// stdout and nothing else. Invoked when ssh re-executes ffh itself as the
// SSH_ASKPASS helper (see main()'s FFH_ASKPASS_MODE check); backend/vault/item are
// passed via environment variables set by resolveCredential (through the backend's
// own askpassEnv), since SSH_ASKPASS only supports a bare program path with no room
// for extra arguments.
func runAskpass() error {
	backend := credentialBackendByName(os.Getenv("FFH_CRED_BACKEND"))
	if backend == nil {
		return fmt.Errorf("unknown credential backend %q", os.Getenv("FFH_CRED_BACKEND"))
	}
	vault := os.Getenv("FFH_CRED_VAULT")
	item := os.Getenv("FFH_CRED_ITEM")
	secret, err := backend.fetchSecret(vault, item, "password")
	if err != nil {
		return err
	}
	fmt.Print(secret)
	return nil
}
