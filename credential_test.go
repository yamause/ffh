package main

import (
	"os/exec"
	"testing"
)

func TestParseCredentialItem_PrefersSetEnvOverride(t *testing.T) {
	out := "user root\nhostname 192.168.255.240\nsetenv FFH_CREDENTIAL=root-str1agg\nport 22\n"
	item, err := parseCredentialItem(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item != "root-str1agg" {
		t.Errorf("got %q, want %q", item, "root-str1agg")
	}
}

func TestParseCredentialItem_FallsBackToUser(t *testing.T) {
	out := "user pocuser\nhostname 192.168.254.2\nport 22\n"
	item, err := parseCredentialItem(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item != "pocuser" {
		t.Errorf("got %q, want %q", item, "pocuser")
	}
}

func TestParseCredentialItem_IgnoresUnrelatedSetEnv(t *testing.T) {
	out := "user admin\nsetenv OTHER_VAR=xyz\n"
	item, err := parseCredentialItem(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item != "admin" {
		t.Errorf("got %q, want %q", item, "admin")
	}
}

func TestParseCredentialItem_NoUserResolved(t *testing.T) {
	out := "hostname 192.168.254.2\nport 22\n"
	if _, err := parseCredentialItem(out); err == nil {
		t.Error("expected error when no user is resolved")
	}
}

func TestParseCredentialItem_EmptyValueDisables(t *testing.T) {
	out := "user pocuser\nsetenv FFH_CREDENTIAL=\n"
	if _, err := parseCredentialItem(out); err == nil {
		t.Error("expected error when FFH_CREDENTIAL is empty, got a resolved item")
	}
}

func TestParseCredentialItem_OffSentinelDisables(t *testing.T) {
	out := "user pocuser\nsetenv FFH_CREDENTIAL=off\n"
	if _, err := parseCredentialItem(out); err == nil {
		t.Error("expected error when FFH_CREDENTIAL=off, got a resolved item")
	}
}

func TestParseCredentialItem_OffSentinelCaseInsensitive(t *testing.T) {
	out := "user pocuser\nsetenv FFH_CREDENTIAL=OFF\n"
	if _, err := parseCredentialItem(out); err == nil {
		t.Error("expected error when FFH_CREDENTIAL=OFF, got a resolved item")
	}
}

func TestResolveCredential_NilWhenNoBackendConfigured(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FFH_OP_VAULT", "")
	resetConfigCache()
	if cred, backend, _ := resolveCredential("/some/ssh_config", "somehost"); cred != nil || backend != nil {
		t.Errorf("expected (nil, nil) when no backend is configured, got (%v, %v)", cred, backend)
	}
}

func TestResolveCredential_NilWhenNoSSHConfigPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FFH_OP_VAULT", "Private")
	resetConfigCache()
	if cred, backend, _ := resolveCredential("", "somehost"); cred != nil || backend != nil {
		t.Errorf("expected (nil, nil) when sshConfigPath is empty (hosts-file mode), got (%v, %v)", cred, backend)
	}
}

func TestResolveCredentialItem_ViaRealSSH(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not found in PATH")
	}
	content := "Host testhost\n  HostName 127.0.0.1\n  User alice\n"
	path := writeTemp(t, content)
	item, err := resolveCredentialItem(path, "testhost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item != "alice" {
		t.Errorf("got %q, want %q", item, "alice")
	}
}

func TestResolveCredentialItem_SetEnvOverride(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not found in PATH")
	}
	content := "Host testhost\n  HostName 127.0.0.1\n  User root\n  SetEnv FFH_CREDENTIAL=root-str1agg\n"
	path := writeTemp(t, content)
	item, err := resolveCredentialItem(path, "testhost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item != "root-str1agg" {
		t.Errorf("got %q, want %q", item, "root-str1agg")
	}
}

func TestResolveCredentialItem_OffOverridesCatchAllMatch(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not found in PATH")
	}
	// Mirrors a real-world config shape: a host-specific opt-out placed before a
	// catch-all "Match all" default that would otherwise set FFH_CREDENTIAL for
	// every host, including key-only ones.
	content := "Host keyonly\n  HostName 127.0.0.1\n  User alice\n  SetEnv FFH_CREDENTIAL=off\n" +
		"\nMatch all\n  SetEnv FFH_CREDENTIAL=tacacs\n"
	path := writeTemp(t, content)
	if _, err := resolveCredentialItem(path, "keyonly"); err == nil {
		t.Error("expected error (disabled) for keyonly host, got a resolved item")
	}
}

func TestResolveCredentialItem_EmptyValueOverridesCatchAllMatch(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not found in PATH")
	}
	content := "Host keyonly\n  HostName 127.0.0.1\n  User alice\n  SetEnv FFH_CREDENTIAL=\n" +
		"\nMatch all\n  SetEnv FFH_CREDENTIAL=tacacs\n"
	path := writeTemp(t, content)
	if _, err := resolveCredentialItem(path, "keyonly"); err == nil {
		t.Error("expected error (disabled) for keyonly host, got a resolved item")
	}
}
