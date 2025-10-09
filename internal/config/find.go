package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	configFilenames = []string{
		".todox.yaml",
		".todox.yml",
		".todox.toml",
		".todox.json",
	}
	xdgFilenames = []string{
		"config.yaml",
		"config.yml",
		"config.toml",
		"config.json",
	}
)

func Find(repoDir, explicitPath, xdgHome, home string) (string, string, error) {
	if explicit := strings.TrimSpace(explicitPath); explicit != "" {
		candidate := explicit
		if !filepath.IsAbs(candidate) {
			cwd, err := os.Getwd()
			if err != nil {
				return "", "", err
			}
			candidate = filepath.Join(cwd, candidate)
		}
		info, err := os.Stat(candidate)
		if err != nil {
			return "", "", err
		}
		if info.IsDir() {
			return "", "", fmt.Errorf("TODOX_CONFIG %q points to a directory", candidate)
		}
		return candidate, "explicit", nil
	}

	start := strings.TrimSpace(repoDir)
	if start == "" {
		start = "."
	}
	absStart, err := filepath.Abs(start)
	if err != nil {
		return "", "", err
	}
	dir := absStart
	for {
		for _, name := range configFilenames {
			candidate := filepath.Join(dir, name)
			if fileExists(candidate) {
				return candidate, "cwd-up", nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	xdgRoot := strings.TrimSpace(xdgHome)
	if xdgRoot == "" {
		homeDir := strings.TrimSpace(home)
		if homeDir == "" {
			if h, err := os.UserHomeDir(); err == nil {
				homeDir = h
			}
		}
		if homeDir != "" {
			xdgRoot = filepath.Join(homeDir, ".config")
		}
	}
	if xdgRoot != "" {
		for _, name := range xdgFilenames {
			candidate := filepath.Join(xdgRoot, "todox", name)
			if fileExists(candidate) {
				return candidate, "xdg", nil
			}
		}
	}

	homeDir := strings.TrimSpace(home)
	if homeDir == "" {
		if h, err := os.UserHomeDir(); err == nil {
			homeDir = h
		}
	}
	if homeDir != "" {
		for _, name := range configFilenames {
			candidate := filepath.Join(homeDir, name)
			if fileExists(candidate) {
				return candidate, "home", nil
			}
		}
	}

	return "", "", nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}
