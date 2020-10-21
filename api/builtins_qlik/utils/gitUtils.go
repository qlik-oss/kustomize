package utils

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// GetGitDescribeForHead returns tag/version from a git repository based on HEAD
func GetGitDescribeForHead(dir string) (string, error) {
	// Test if dir is git repository
	out, err := exec.Command("git", "-C", dir, "rev-parse").CombinedOutput()
	errMsg := errors.New(strings.TrimSpace(string(out)))
	if err != nil {
		return "", errMsg
	}

	out, err = exec.Command("git", "-C", dir, "describe", "--tags", "--abbrev=7", "--match", "v[0-9]*.[0-9]*.[0-9]*").Output()
	if err != nil {
		out, err = exec.Command("git", "-C", dir, "rev-parse", "--short=7", "HEAD").Output()
		if err != nil {
			return "", err
		}
		out = []byte(fmt.Sprintf("0.0.0-0-g%v", string(out)))
	}

	tag := strings.TrimSpace(string(out))
	version := strings.TrimPrefix(tag, "v")

	return string(version), nil
}
