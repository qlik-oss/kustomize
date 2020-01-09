package main

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	kusttest_test "sigs.k8s.io/kustomize/api/testutils/kusttest"
)

func TestChartHomeFullPathPlugin(t *testing.T) {
	th := kusttest_test.MakeEnhancedHarness(t).PrepBuiltin("ChartHomeFullPath")
	defer th.Reset()

	// create a temp directory and test file
	dir, err := ioutil.TempDir("", "test")
	require.NoError(t, err)

	file, err := ioutil.TempFile(dir, "testFile")
	require.NoError(t, err)
	defer file.Close()

	fileContents := []byte("test")
	_, err = file.Write(fileContents)
	require.NoError(t, err)

	// make temp directory chartHome
	m := th.LoadAndRunTransformer(`
apiVersion: qlik.com/
kind: ChartHomeFullPath
metadata:
  name: qliksense
chartHome: `+dir, `
apiVersion: apps/v1
kind: HelmChart
metadata:
  name: qliksense
chartName: qliksense
releaseName: qliksense
`)

	// pull out modified chartHome for plugin
	var chartHome string
	for _, r := range m.Resources() {
		chartHome, err = r.GetString("chartHome")
		require.NoError(t, err)
	}

	require.NotEqual(t, dir, chartHome)

	//open modified directory
	directory, err := os.Open(chartHome)
	require.NoError(t, err)
	objects, err := directory.Readdir(-1)
	require.NoError(t, err)

	//check the temp file was coppied over correctly
	for _, obj := range objects {
		source := chartHome + "/" + obj.Name()
		readFileContents, err := ioutil.ReadFile(source)
		require.NoError(t, err)
		require.Equal(t, fileContents, readFileContents)
	}

	// insure ouput of yaml is correct
	th.AssertActualEqualsExpected(m, `
apiVersion: apps/v1
chartHome: `+chartHome+`
chartName: qliksense
kind: HelmChart
metadata:
  name: qliksense
releaseName: qliksense
`)
}
