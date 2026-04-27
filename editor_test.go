package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "ssh_config_*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	f.Close()
	return f.Name()
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return string(b)
}

func TestUpdateHostDirective_UpdateExisting(t *testing.T) {
	content := `Host myhost
  HostName old.example.com
  User alice
  Port 22

Host other
  HostName other.example.com
`
	path := writeTemp(t, content)
	original, err := updateHostDirective(path, "myhost", "HostName", "new.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(original) != content {
		t.Errorf("original content mismatch")
	}
	got := readFile(t, path)
	want := `Host myhost
  HostName new.example.com
  User alice
  Port 22

Host other
  HostName other.example.com
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUpdateHostDirective_AddNewDirective(t *testing.T) {
	content := `Host myhost
  HostName example.com
  User alice

Host other
  HostName other.example.com
`
	path := writeTemp(t, content)
	_, err := updateHostDirective(path, "myhost", "Port", "2222")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := readFile(t, path)
	want := `Host myhost
  HostName example.com
  User alice
  Port 2222

Host other
  HostName other.example.com
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUpdateHostDirective_EOFNoTrailingNewline(t *testing.T) {
	// Block without trailing blank line (EOF closes it)
	content := "Host myhost\n  HostName example.com"
	path := writeTemp(t, content)
	_, err := updateHostDirective(path, "myhost", "User", "bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := readFile(t, path)
	want := "Host myhost\n  HostName example.com\n  User bob"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestUpdateHostDirective_MultipleBlocks_CorrectOnly(t *testing.T) {
	content := `Host alpha
  HostName alpha.example.com
  Port 22

Host beta
  HostName beta.example.com
  Port 22

`
	path := writeTemp(t, content)
	_, err := updateHostDirective(path, "beta", "Port", "2222")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := readFile(t, path)
	want := `Host alpha
  HostName alpha.example.com
  Port 22

Host beta
  HostName beta.example.com
  Port 2222

`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUpdateHostDirective_WildcardHostNotEdited(t *testing.T) {
	content := `Host *
  ServerAliveInterval 60

Host realhost
  HostName real.example.com
`
	path := writeTemp(t, content)
	_, err := updateHostDirective(path, "*", "ServerAliveInterval", "120")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Wildcard host contains * so updateHostDirective should not match it;
	// the file should be unchanged (directive was inserted at bottom as a new block).
	// Actually our implementation skips wildcards — so the file is appended at EOF
	// with no matching block found → file unchanged (inBlock never set).
	got := readFile(t, path)
	if got != content {
		t.Errorf("wildcard host should not be modified; got:\n%s", got)
	}
}

func TestUpdateHostDirective_PreservesLeadingWhitespace(t *testing.T) {
	content := "Host myhost\n\tUser alice\n"
	path := writeTemp(t, content)
	_, err := updateHostDirective(path, "myhost", "User", "bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := readFile(t, path)
	want := "Host myhost\n\tUser bob\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestUpdateHostDirective_RollbackOnWriteError(t *testing.T) {
	// Create file in a temp dir then make it read-only to force write failure.
	dir := t.TempDir()
	path := filepath.Join(dir, "ssh_config")
	content := "Host myhost\n  HostName example.com\n"
	if err := os.WriteFile(path, []byte(content), 0400); err != nil {
		t.Fatalf("setup: %v", err)
	}
	original, err := updateHostDirective(path, "myhost", "User", "bob")
	if err == nil {
		t.Error("expected write error, got nil")
	}
	if string(original) != content {
		t.Errorf("original content should still be returned on error")
	}
}

// TestApplyHostDirective_RollbackOnSyntaxError verifies that applyHostDirective restores
// the original file when ssh -G rejects the updated config.
func TestApplyHostDirective_RollbackOnSyntaxError(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not found in PATH")
	}

	content := "Host myhost\n  HostName example.com\n  Port 22\n"
	path := writeTemp(t, content)

	// "Port notanumber" is an invalid SSH config value; ssh -G should report an error.
	err := applyHostDirective(path, "myhost", path, "Port", "notanumber")
	if err == nil {
		t.Error("expected syntax error, got nil")
	}

	// The file should have been rolled back to its original content.
	got := readFile(t, path)
	if got != content {
		t.Errorf("file should be rolled back after syntax error\ngot:\n%s\nwant:\n%s", got, content)
	}
}

// TestApplyHostDirective_Success verifies that applyHostDirective updates the file
// when the resulting config is valid.
func TestApplyHostDirective_Success(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not found in PATH")
	}

	// Use the real ~/.ssh/config as sshConfigPath so ssh -G can resolve the host.
	// We write a separate source file and pass it as both sourceFile and sshConfigPath
	// because applyHostDirective only needs sshConfigPath for the syntax check.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ssh_config")
	content := "Host testhost\n  HostName 127.0.0.1\n  Port 22\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := applyHostDirective(cfgPath, "testhost", cfgPath, "Port", "2222")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readFile(t, cfgPath)
	if got == content {
		t.Error("file should have been updated")
	}
	want := "Host testhost\n  HostName 127.0.0.1\n  Port 2222\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
