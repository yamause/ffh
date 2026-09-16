package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// opBackend is the credentialBackend implementation backed by 1Password's `op` CLI.
// It holds no state -- every method resolves what it needs (vault, session) itself
// -- so the zero value is always usable, per credentialBackends.
type opBackend struct{}

func (opBackend) name() string        { return "op" }
func (opBackend) displayName() string { return "1Password" }

func (opBackend) vault() string { return resolveOpVault() }

// fetchSecret reads the named field of a 1Password item via the op CLI.
func (opBackend) fetchSecret(vault, item, field string) (string, error) {
	out, err := exec.Command("op", "read", fmt.Sprintf("op://%s/%s/%s", vault, item, field)).Output()
	if err != nil {
		return "", fmt.Errorf("op read %s/%s/%s: %w", vault, item, field, err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// isAuthError reports whether err is the result of `op` failing because the user is
// not currently signed in, as opposed to other failures (missing item, missing
// vault, op binary itself unavailable, etc.) that resolveCredential treats as an
// ordinary, silent fallback case.
func (opBackend) isAuthError(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	stderr := strings.ToLower(string(exitErr.Stderr))
	return strings.Contains(stderr, "not currently signed in") || strings.Contains(stderr, "op signin")
}

// signin runs `op signin` attached to the real terminal so the user can complete
// whatever authentication flow 1Password presents (master password, 2FA, etc. -- all
// of which op reads/writes via stderr or /dev/tty directly, not stdout), then applies
// any "export OP_SESSION_<account>=..." line it prints to stdout (the classic
// eval-friendly output format for accounts without desktop-app integration) to the
// current process's environment, so later `op` calls in this process -- including the
// SSH_ASKPASS re-invocation, which inherits os.Environ() -- pick up the new session.
func (opBackend) signin() error {
	cmd := exec.Command("op", "signin")
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return err
	}
	applyOpSessionEnv(stdout.String())
	return nil
}

// applyOpSessionEnv parses "export OP_SESSION_<account>=<token>" lines (op signin's
// stdout) and sets them via os.Setenv. A no-op if op signin printed no such line
// (e.g. desktop-app-integrated accounts, which authenticate without a session token).
func applyOpSessionEnv(output string) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimPrefix(strings.TrimSpace(line), "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok || !strings.HasPrefix(k, "OP_SESSION_") {
			continue
		}
		os.Setenv(k, strings.Trim(v, `"`))
	}
}

func (b opBackend) askpassEnv(vault, item string) []string {
	return []string{
		"SSH_ASKPASS_REQUIRE=force",
		"SSH_ASKPASS=" + selfPath(),
		"FFH_ASKPASS_MODE=1",
		"FFH_CRED_BACKEND=" + b.name(),
		"FFH_CRED_VAULT=" + vault,
		"FFH_CRED_ITEM=" + item,
	}
}
