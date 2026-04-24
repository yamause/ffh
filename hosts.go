package main

import (
	"bufio"
	"os"
	"strings"
)

type HostEntry struct {
	IP       string
	Hostname string
}

func parseHostsFile(path string) ([]HostEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []HostEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip := fields[0]
		if strings.HasPrefix(ip, "127.") || ip == "::1" {
			continue
		}
		entries = append(entries, HostEntry{IP: ip, Hostname: fields[1]})
	}
	return entries, scanner.Err()
}
