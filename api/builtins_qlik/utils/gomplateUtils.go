package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hairyhenderson/gomplate/v3"
	"go.uber.org/zap"
)

var gomplateMutex sync.Mutex

func RunGomplate(dataSource string, pwd string, env []string, template string, logger *zap.SugaredLogger) ([]byte, error) {

	var opts gomplate.Config
	opts.DataSources = []string{fmt.Sprintf("data=%s", filepath.Join(pwd, dataSource))}
	opts.Input = template
	opts.LDelim = "(("
	opts.RDelim = "))"

	for _, envVar := range env {
		if envVarParts := strings.Split(envVar, "="); len(envVarParts) == 2 {
			if err := os.Setenv(envVarParts[0], envVarParts[1]); err != nil {
				logger.Errorf("error setting env variable: %v=%v, error: %v\n", envVarParts[0], envVarParts[1], err)
			}
		}
	}

	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	opts.OutputFiles = []string{tmpFile.Name()}

	gomplateMutex.Lock()
	defer gomplateMutex.Unlock()

	logger.Debugf("executing gomplate.RunTemplates() with opts: %v\n", opts)
	if err := gomplate.RunTemplates(&opts); err != nil {
		logger.Errorf("error calling gomplate API with config: %v, error: %v\n", opts.String(), err)
		return nil, err
	}
	return ioutil.ReadFile(tmpFile.Name())
}

func RunGomplateFromConfig(dataSources []string, pwd string, env map[string]string, template string, logger *zap.SugaredLogger, ldelim string, rdelim string) ([]byte, error) {

	var opts gomplate.Config
	opts.DataSources = dataSources
	opts.Input = template
	opts.LDelim = ldelim
	opts.RDelim = rdelim

	for key, value := range env {
		if err := os.Setenv(key, value); err != nil {
			logger.Errorf("error setting env variable: %v=%v, error: %v\n", key, value, err)
		}
	}

	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	opts.OutputFiles = []string{tmpFile.Name()}

	gomplateMutex.Lock()
	defer gomplateMutex.Unlock()

	logger.Debugf("executing gomplate.RunTemplates() with opts: %v\n", opts)
	if err := gomplate.RunTemplates(&opts); err != nil {
		fmt.Printf("%v", err)
		logger.Errorf("error calling gomplate API with config: %v, error: %v\n", opts.String(), err)
		return nil, err
	}
	return ioutil.ReadFile(tmpFile.Name())
}
