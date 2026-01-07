package config

import (
	"os"
	"path/filepath"
)

func UserConfigPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "bambu", "config.json"), nil
}

func ProjectConfigPath(cwd string) string {
	return filepath.Join(cwd, ".bambu.json")
}
