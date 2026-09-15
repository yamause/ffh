package main

import "testing"

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
