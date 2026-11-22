package config

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

// LoadEnvFile reads KEY=VALUE pairs without overriding variables already set
// by systemd's EnvironmentFile or the shell.
func LoadEnvFile(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		i := strings.IndexByte(line, '=')
		if i < 1 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		val := strings.TrimSpace(line[i+1:])
		if len(val) >= 2 && (val[0] == '"' && val[len(val)-1] == '"' || val[0] == '\'' && val[len(val)-1] == '\'') {
			val = val[1 : len(val)-1]
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	return sc.Err()
}

// Secret prefers a value entered in the panel; otherwise reads the env var.
func Secret(envName, inline string) string {
	if strings.TrimSpace(inline) != "" {
		return inline
	}
	if strings.TrimSpace(envName) == "" {
		return ""
	}
	return strings.TrimSpace(os.Getenv(envName))
}
