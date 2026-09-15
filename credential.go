package main

import (
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

// resolveCredentialItem determines the 1Password item name to use for hostAlias by
// running `ssh -F sshConfigPath -G hostAlias` once and reading the resolved config.
// A "setenv FFH_CREDENTIAL=<item>" line (set via a native SSH `SetEnv` directive)
// takes priority, for the rare case where a shared login user has more than one
// password in use; otherwise the resolved "user" value is used, since passwords
// here are managed per login user rather than per host.
func resolveCredentialItem(sshConfigPath, hostAlias string) (string, error) {
	out, err := exec.Command("ssh", "-F", sshConfigPath, "-G", hostAlias).Output()
	if err != nil {
		return "", fmt.Errorf("ssh -G %s: %w", hostAlias, err)
	}
	return parseCredentialItem(string(out))
}

// parseCredentialItem extracts the 1Password item name from `ssh -G` output text.
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

// fetchOpSecret reads the named field of a 1Password item via the op CLI.
func fetchOpSecret(vault, item, field string) (string, error) {
	out, err := exec.Command("op", "read", fmt.Sprintf("op://%s/%s/%s", vault, item, field)).Output()
	if err != nil {
		return "", fmt.Errorf("op read %s/%s/%s: %w", vault, item, field, err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// credential holds what execSSH needs to hand a connection off to 1Password: extra
// environment variables that arm the SSH_ASKPASS helper, and optionally a login
// username to use instead of ssh_config's own User resolution.
type credential struct {
	env      []string
	username string // empty when the 1Password item has no (or an empty) username field
}

// resolveCredential determines the 1Password-backed credential for hostAlias, or nil
// when the credential backend is disabled (no op_vault configured), unreachable, or
// no matching 1Password item exists for hostAlias. On nil, callers must leave ssh's
// normal interactive prompt and ssh_config's own User resolution untouched -- this is
// what keeps a 1Password outage non-fatal to any connection.
func resolveCredential(sshConfigPath, hostAlias string) *credential {
	if sshConfigPath == "" {
		return nil
	}
	vault := resolveOpVault()
	if vault == "" {
		return nil
	}
	item, err := resolveCredentialItem(sshConfigPath, hostAlias)
	if err != nil {
		return nil
	}
	// Validate the secret is actually retrievable before forcing askpass; the value
	// itself is discarded here and re-fetched inside the askpass sub-invocation so it
	// never sits in this process's (or ssh's) environment.
	if _, err := fetchOpSecret(vault, item, "password"); err != nil {
		return nil
	}
	cred := &credential{env: []string{
		"SSH_ASKPASS_REQUIRE=force",
		"SSH_ASKPASS=" + selfPath(),
		"FFH_ASKPASS_MODE=1",
		"FFH_OP_VAULT=" + vault,
		"FFH_OP_ITEM=" + item,
	}}
	// A missing/empty username field (e.g. a "Password"-category item) is not an
	// error: the connection simply keeps using ssh_config's own User.
	if username, err := fetchOpSecret(vault, item, "username"); err == nil && username != "" {
		cred.username = username
	}
	return cred
}

// runAskpass implements the SSH_ASKPASS protocol: print the requested 1Password
// secret to stdout and nothing else. Invoked when ssh re-executes ffh itself as the
// SSH_ASKPASS helper (see main()'s FFH_ASKPASS_MODE check); vault/item are passed
// via environment variables set by resolveCredential, since SSH_ASKPASS only supports
// a bare program path with no room for extra arguments.
func runAskpass() error {
	vault := os.Getenv("FFH_OP_VAULT")
	item := os.Getenv("FFH_OP_ITEM")
	secret, err := fetchOpSecret(vault, item, "password")
	if err != nil {
		return err
	}
	fmt.Print(secret)
	return nil
}
