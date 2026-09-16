package main

import (
	"reflect"
	"testing"
)

func TestHasLoginOverride(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"none", []string{"-F", "/etc/ssh_config", "-p", "22"}, false},
		{"separate -l", []string{"-l", "alice"}, true},
		{"combined -l", []string{"-lalice"}, true},
		{"-o User=", []string{"-o", "User=alice"}, true},
		{"-o user= lowercase value", []string{"-o", "user=alice"}, true},
		{"unrelated -o", []string{"-o", "ProxyJump=bastion"}, false},
		{"empty", []string{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasLoginOverride(c.args); got != c.want {
				t.Errorf("hasLoginOverride(%v) = %v, want %v", c.args, got, c.want)
			}
		})
	}
}

func TestCredentialSSHArgs_NilCredential(t *testing.T) {
	sshArgs, env := credentialSSHArgs("myhost", []string{"-p", "2222"}, nil)
	wantArgs := []string{"myhost", "-p", "2222"}
	if !reflect.DeepEqual(sshArgs, wantArgs) {
		t.Errorf("sshArgs = %v, want %v", sshArgs, wantArgs)
	}
	if env != nil {
		t.Errorf("env = %v, want nil", env)
	}
}

func TestCredentialSSHArgs_UsernameOverride(t *testing.T) {
	cred := &credential{env: []string{"FFH_ASKPASS_MODE=1"}, username: "alice"}
	sshArgs, env := credentialSSHArgs("myhost", []string{"-p", "2222"}, cred)
	wantArgs := []string{"-l", "alice", "myhost", "-p", "2222"}
	if !reflect.DeepEqual(sshArgs, wantArgs) {
		t.Errorf("sshArgs = %v, want %v", sshArgs, wantArgs)
	}
	if !reflect.DeepEqual(env, cred.env) {
		t.Errorf("env = %v, want %v", env, cred.env)
	}
}

func TestCredentialSSHArgs_CallerLoginOverrideWins(t *testing.T) {
	cred := &credential{env: []string{"FFH_ASKPASS_MODE=1"}, username: "alice"}
	sshArgs, _ := credentialSSHArgs("myhost", []string{"-l", "bob"}, cred)
	wantArgs := []string{"myhost", "-l", "bob"}
	if !reflect.DeepEqual(sshArgs, wantArgs) {
		t.Errorf("sshArgs = %v, want %v (caller's -l should win, no credential -l inserted)", sshArgs, wantArgs)
	}
}
