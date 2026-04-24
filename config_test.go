package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveHostsPath_CLIArgWins(t *testing.T) {
	t.Setenv("FFH_HOSTS_FILE", "/env/hosts")
	got := resolveHostsPath("/cli/hosts")
	if got != "/cli/hosts" {
		t.Errorf("got %q, want /cli/hosts", got)
	}
}

func TestResolveHostsPath_EnvVarWins(t *testing.T) {
	t.Setenv("FFH_HOSTS_FILE", "/env/hosts")
	got := resolveHostsPath("")
	if got != "/env/hosts" {
		t.Errorf("got %q, want /env/hosts", got)
	}
}

func TestResolveHostsPath_ConfigFileWins(t *testing.T) {
	t.Setenv("FFH_HOSTS_FILE", "")

	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "ffh")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config"), []byte("hosts_file = /cfg/hosts\n"), 0600); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.UserHomeDir()
	t.Setenv("HOME", dir)
	defer t.Setenv("HOME", orig)

	got := resolveHostsPath("")
	if got != "/cfg/hosts" {
		t.Errorf("got %q, want /cfg/hosts", got)
	}
}

func TestResolveHostsPath_Default(t *testing.T) {
	t.Setenv("FFH_HOSTS_FILE", "")

	dir := t.TempDir() // no config file here
	orig, _ := os.UserHomeDir()
	t.Setenv("HOME", dir)
	defer t.Setenv("HOME", orig)

	got := resolveHostsPath("")
	if got != defaultHostsPath {
		t.Errorf("got %q, want %q", got, defaultHostsPath)
	}
}

func TestLoadConfig_CommentsAndBlanks(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "ffh")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	content := "# comment\n\nhosts_file = /some/path\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.UserHomeDir()
	t.Setenv("HOME", dir)
	defer t.Setenv("HOME", orig)

	cfg := loadConfig()
	if cfg["hosts_file"] != "/some/path" {
		t.Errorf("got %q, want /some/path", cfg["hosts_file"])
	}
}
