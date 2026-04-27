package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveHostsPath_CLIArgWins(t *testing.T) {
	t.Cleanup(resetConfigCache)
	t.Setenv("FFH_HOSTS_FILE", "/env/hosts")
	got := resolveHostsPath("/cli/hosts")
	if got != "/cli/hosts" {
		t.Errorf("got %q, want /cli/hosts", got)
	}
}

func TestResolveHostsPath_EnvVarWins(t *testing.T) {
	t.Cleanup(resetConfigCache)
	t.Setenv("FFH_HOSTS_FILE", "/env/hosts")
	got := resolveHostsPath("")
	if got != "/env/hosts" {
		t.Errorf("got %q, want /env/hosts", got)
	}
}

func TestResolveHostsPath_ConfigFileWins(t *testing.T) {
	t.Cleanup(resetConfigCache)
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
	t.Cleanup(resetConfigCache)
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

func TestSplitAtDoubleDash(t *testing.T) {
	cases := []struct {
		args        []string
		wantFFH     []string
		wantSSH     []string
	}{
		{[]string{"-F", "cfg", "--", "-L", "8080:localhost:8080"}, []string{"-F", "cfg"}, []string{"-L", "8080:localhost:8080"}},
		{[]string{"-F", "cfg"}, []string{"-F", "cfg"}, nil},
		{[]string{"--", "-v"}, []string{}, []string{"-v"}},
		{[]string{}, []string{}, nil},
		{[]string{"--"}, []string{}, []string{}},
	}
	for _, c := range cases {
		ffh, ssh := splitAtDoubleDash(c.args)
		if len(ffh) != len(c.wantFFH) {
			t.Errorf("args=%v: ffhArgs=%v, want %v", c.args, ffh, c.wantFFH)
			continue
		}
		for i := range ffh {
			if ffh[i] != c.wantFFH[i] {
				t.Errorf("args=%v: ffhArgs[%d]=%q, want %q", c.args, i, ffh[i], c.wantFFH[i])
			}
		}
		if len(ssh) != len(c.wantSSH) {
			t.Errorf("args=%v: sshArgs=%v, want %v", c.args, ssh, c.wantSSH)
		}
	}
}

func TestUnknownFFHFlag(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-F", "cfg", "--tab-source", "tag"}, ""},
		{[]string{"-F", "cfg"}, ""},
		{[]string{}, ""},
		{[]string{"-L", "8080:localhost:8080"}, "-L"},
		{[]string{"--tab-source", "tag", "--unknown"}, "--unknown"},
		{[]string{"-F", "cfg", "--tab-source"}, ""}, // value missing — out of bounds handled gracefully
	}
	for _, c := range cases {
		got := unknownFFHFlag(c.args)
		if got != c.want {
			t.Errorf("args=%v: got %q, want %q", c.args, got, c.want)
		}
	}
}

func TestLoadConfig_CommentsAndBlanks(t *testing.T) {
	t.Cleanup(resetConfigCache)
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
