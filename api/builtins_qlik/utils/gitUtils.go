package utils

import (
	"os/exec"
	"strings"
)

// GetGitDescribeForHead returns tag/version from a git repository based on HEAD
func GetGitDescribeForHead(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "describe", "--tags", "--abbrev=7", "--match", "v*").Output()
	if err != nil {
		return "", err
	}

	tag := strings.TrimSpace(string(out))
	version := strings.TrimPrefix(tag, "v")

	return string(version), nil
}
