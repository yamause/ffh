package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveOpVault_DisabledByDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FFH_OP_VAULT", "")
	resetConfigCache()
	if v := resolveOpVault(); v != "" {
		t.Errorf("got %q, want empty (disabled by default)", v)
	}
}

func TestResolveOpVault_EnvOverride(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FFH_OP_VAULT", "Private")
	resetConfigCache()
	if v := resolveOpVault(); v != "Private" {
		t.Errorf("got %q, want %q", v, "Private")
	}
}

func TestOpBackend_FetchSecret_ErrorWhenItemMissing(t *testing.T) {
	if _, err := exec.LookPath("op"); err != nil {
		t.Skip("op not found in PATH")
	}
	backend := opBackend{}
	if _, err := backend.fetchSecret("__ffh_test_nonexistent_vault__", "__ffh_test_nonexistent_item__", "password"); err == nil {
		t.Error("expected error for nonexistent vault/item")
	}
}

func TestOpBackend_FetchSecret_ErrorWhenItemMissing_UsernameField(t *testing.T) {
	if _, err := exec.LookPath("op"); err != nil {
		t.Skip("op not found in PATH")
	}
	backend := opBackend{}
	if _, err := backend.fetchSecret("__ffh_test_nonexistent_vault__", "__ffh_test_nonexistent_item__", "username"); err == nil {
		t.Error("expected error for nonexistent vault/item")
	}
}

func TestRunAskpass_ErrorWhenSecretMissing(t *testing.T) {
	if _, err := exec.LookPath("op"); err != nil {
		t.Skip("op not found in PATH")
	}
	t.Setenv("FFH_CRED_BACKEND", "op")
	t.Setenv("FFH_CRED_VAULT", "__ffh_test_nonexistent_vault__")
	t.Setenv("FFH_CRED_ITEM", "__ffh_test_nonexistent_item__")
	if err := runAskpass(); err == nil {
		t.Error("expected error for nonexistent vault/item")
	}
}

func TestOpBackend_IsAuthError_DetectsNotSignedIn(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not found in PATH")
	}
	backend := opBackend{}
	_, err := exec.Command("sh", "-c", "echo '[ERROR] you are not currently signed in' >&2; exit 1").Output()
	if !backend.isAuthError(err) {
		t.Error("expected auth error to be detected")
	}
}

func TestOpBackend_IsAuthError_IgnoresOtherErrors(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not found in PATH")
	}
	backend := opBackend{}
	_, err := exec.Command("sh", "-c", "echo '[ERROR] isnt a vault, item, or file' >&2; exit 1").Output()
	if backend.isAuthError(err) {
		t.Error("expected non-auth error to not be classified as an auth error")
	}
}

func TestResolveCredential_NotSignedInWhenOpAuthFails(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not found in PATH")
	}
	dir := t.TempDir()
	fakeOp := filepath.Join(dir, "op")
	script := "#!/bin/sh\necho '[ERROR] you are not currently signed in' >&2\nexit 1\n"
	if err := os.WriteFile(fakeOp, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FFH_OP_VAULT", "Private")
	resetConfigCache()
	content := "Host testhost\n  HostName 127.0.0.1\n  User pocuser\n"
	path := writeTemp(t, content)
	cred, backend, notSignedIn := resolveCredential(path, "testhost")
	if cred != nil {
		t.Errorf("expected nil credential, got %v", cred)
	}
	if backend == nil || backend.name() != "op" {
		t.Errorf("expected the op backend to be returned, got %v", backend)
	}
	if !notSignedIn {
		t.Error("expected notSignedIn=true when op reports not being signed in")
	}
}

func TestApplyOpSessionEnv_SetsExportedSessionVar(t *testing.T) {
	defer os.Unsetenv("OP_SESSION_test")
	applyOpSessionEnv(`export OP_SESSION_test="abc123"` + "\n")
	if got := os.Getenv("OP_SESSION_test"); got != "abc123" {
		t.Errorf("got %q, want %q", got, "abc123")
	}
}

func TestApplyOpSessionEnv_IgnoresUnrelatedOutput(t *testing.T) {
	defer os.Unsetenv("OTHER_VAR")
	applyOpSessionEnv("some informational line\nexport OTHER_VAR=xyz\n")
	if got := os.Getenv("OTHER_VAR"); got != "" {
		t.Errorf("expected non-OP_SESSION vars to be ignored, got %q", got)
	}
}

func TestResolveCredential_NilWhenItemUnresolvable(t *testing.T) {
	if _, err := exec.LookPath("op"); err != nil {
		t.Skip("op not found in PATH")
	}
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not found in PATH")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FFH_OP_VAULT", "__ffh_test_nonexistent_vault__")
	resetConfigCache()
	content := "Host testhost\n  HostName 127.0.0.1\n  User __ffh_test_nonexistent_item__\n"
	path := writeTemp(t, content)
	if cred, _, _ := resolveCredential(path, "testhost"); cred != nil {
		t.Errorf("expected nil when the password field can't be fetched, got %v", cred)
	}
}
