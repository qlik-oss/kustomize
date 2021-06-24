package utils

import (
	"fmt"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// GetGitDescribeForHead returns tag/version from a git repository based on HEAD
func GetGitDescribeForHead(dir string, defaultVersion string, logger *zap.SugaredLogger) (string, error) {
	if logger == nil {
		logger = GetNopLogger()
	}

	if defaultVersion == "" {
		defaultVersion = "0.0.0"
	}

	out, err := exec.Command("git", "-C", dir, "describe", "--tags", "--abbrev=7", "--match", "v[0-9]*.[0-9]*.[0-9]*").Output()
	if err != nil {
		logger.Infof("Tag Not found")
		out, err = exec.Command("git", "-C", dir, "rev-parse", "--short=7", "HEAD").Output()
		if err != nil {
			logger.Warnf("Not able to get commit id nor tag, using default")
			return strings.TrimPrefix(defaultVersion, "v"), nil
		}
		out = []byte(fmt.Sprintf("0.0.0-0-g%v", string(out)))
	} else {
		logger.Infof("Found tag %v", string(out))
	}

	tag := strings.TrimSpace(string(out))
	version := strings.TrimPrefix(tag, "v")

	logger.Infof("%v: for %v", version, dir)
	return string(version), nil
}
