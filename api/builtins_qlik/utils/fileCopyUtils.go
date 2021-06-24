package utils

import (
	"io"
	"os"

	"go.uber.org/zap"
)

// copy source file to destination location
func CopyFile(source string, dest string, logger *zap.SugaredLogger) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		logger.Errorf("error opening file %v, error: %v\n", source, err)
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dest)
	if err != nil {
		logger.Errorf("error creating file %v, error: %v\n", dest, err)
		return err
	}

	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		logger.Errorf("error copying file to %v, from %v, error: %v\n", destinationFile, sourceFile, err)
	} else {
		sourceinfo, err := os.Stat(source)
		if err != nil {
			logger.Errorf("error stating file %v, error: %v\n", source, err)
			err = os.Chmod(dest, sourceinfo.Mode())
			if err != nil {
				logger.Errorf("error chmod-ing file %v to %v, error: %v\n", dest, sourceinfo.Mode(), err)
			}
			return err
		}
	}
	return nil
}

//copy source directory to destination
func CopyDir(source string, dest string, logger *zap.SugaredLogger) error {
	sourceinfo, err := os.Stat(source)
	if err != nil {
		logger.Errorf("error stating file %v, error: %v\n", source, err)
		return err
	}

	err = os.MkdirAll(dest, sourceinfo.Mode())
	if err != nil {
		logger.Errorf("error creating directory %v with permissions: %v, error: %v\n", dest, sourceinfo.Mode(), err)
		return err
	}
	sourceDirectory, err := os.Open(source)
	if err != nil {
		logger.Errorf("error opening source directory %v, error: %v\n", source, err)
		return err
	}

	// read everything within source directory
	objects, err := sourceDirectory.Readdir(-1)
	if err != nil {
		logger.Errorf("error listing source directory %v, error: %v\n", sourceDirectory, err)
		return err
	}

	// go through all files/directories
	for _, obj := range objects {

		sourceFileName := source + "/" + obj.Name()

		destinationFileName := dest + "/" + obj.Name()

		if obj.IsDir() {
			err := CopyDir(sourceFileName, destinationFileName, logger)
			if err != nil {
				logger.Errorf("error copying directory from: %v, to: %v, error: %v\n", sourceFileName, destinationFileName, err)
				return err
			}
		} else {
			err := CopyFile(sourceFileName, destinationFileName, logger)
			if err != nil {
				logger.Errorf("error copying file from: %v, to: %v, error: %v\n", sourceFileName, destinationFileName, err)
				return err
			}
		}

	}
	return nil
}
