package sshconfig

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sshcfg "github.com/kevinburke/ssh_config"
)

type Host struct {
	Alias    string
	User     string
	HostName string
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

func Load(path string) ([]Host, []string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	cfg, err := sshcfg.Decode(file)
	if err != nil {
		return nil, nil, err
	}

	file, err = os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	aliases, skipped, err := parseAliases(file)
	if err != nil {
		return nil, nil, err
	}

	hosts := make([]Host, 0, len(aliases))
	for _, alias := range aliases {
		user, err := cfg.Get(alias, "User")
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: failed to resolve User: %v", alias, err))
			continue
		}
		hostName, err := cfg.Get(alias, "HostName")
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: failed to resolve HostName: %v", alias, err))
			continue
		}
		if strings.TrimSpace(user) == "" || strings.TrimSpace(hostName) == "" {
			skipped = append(skipped, fmt.Sprintf("%s: skipped because User or HostName is missing", alias))
			continue
		}
		hosts = append(hosts, Host{
			Alias:    alias,
			User:     strings.TrimSpace(user),
			HostName: strings.TrimSpace(hostName),
		})
	}
	return hosts, skipped, nil
}

func parseAliases(file *os.File) ([]string, []string, error) {
	var aliases []string
	var skipped []string
	seen := map[string]struct{}{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "match ") {
			skipped = append(skipped, fmt.Sprintf("%s: skipped Match entry", strings.TrimSpace(line)))
			continue
		}
		if !strings.HasPrefix(lower, "host ") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		for _, candidate := range fields[1:] {
			if strings.ContainsAny(candidate, "*?!") {
				skipped = append(skipped, fmt.Sprintf("%s: skipped pattern host", candidate))
				continue
			}
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			aliases = append(aliases, candidate)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	return aliases, skipped, nil
}
