package utils

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hairyhenderson/gomplate/v3"
)

var gomplateLockFilePath string

func init() {
	gomplateLockFilePath = filepath.Join(os.TempDir(), "qkp-gomplate.flock")
}

func RunGomplate(dataSource string, pwd string, env []string, template string,
	lockTimeoutSeconds, retryDelayMinMilliseconds, retryDelayMaxMilliseconds int, logger *log.Logger) ([]byte, error) {

	var opts gomplate.Config
	opts.DataSources = []string{fmt.Sprintf("data=%s", filepath.Join(pwd, dataSource))}
	opts.Input = template
	opts.LDelim = "(("
	opts.RDelim = "))"

	for _, envVar := range env {
		if envVarParts := strings.Split(envVar, "="); len(envVarParts) == 2 {
			if err := os.Setenv(envVarParts[0], envVarParts[1]); err != nil {
				logger.Printf("error setting env variable: %v=%v, error: %v\n", envVarParts[0], envVarParts[1], err)
			}
		}
	}

	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	opts.OutputFiles = []string{tmpFile.Name()}

	if unlockFn, err := LockPath(gomplateLockFilePath, lockTimeoutSeconds, retryDelayMinMilliseconds, retryDelayMaxMilliseconds, logger); err != nil {
		logger.Printf("error locking %v, error: %v\n", gomplateLockFilePath, err)
		return nil, err
	} else {
		defer unlockFn()
	}

	logger.Printf("executing gomplate.RunTemplates() with opts: %v\n", opts)
	if err := gomplate.RunTemplates(&opts); err != nil {
		logger.Printf("error calling gomplate API with config: %v, error: %v\n", opts.String(), err)
		return nil, err
	}
	return ioutil.ReadFile(tmpFile.Name())
}
