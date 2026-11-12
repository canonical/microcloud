package testbed

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// homeDir returns the current user's home directory.
func homeDir() (string, error) {
	home := os.Getenv("HOME")
	if home != "" {
		return home, nil
	}

	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("Cannot determine the current user: %w", err)
	}

	return u.HomeDir, nil
}

// expandHome replaces a leading "~" (or "~/...") in path with the current user's home directory.
func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}

	home, err := homeDir()
	if err != nil {
		return "", err
	}

	if path == "~" {
		return home, nil
	}

	return filepath.Join(home, path[2:]), nil
}
