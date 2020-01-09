package main

import (
	"testing"

	kusttest_test "sigs.k8s.io/kustomize/api/testutils/kusttest"
)

func TestHelmValuesPlugin(t *testing.T) {
	th := kusttest_test.MakeEnhancedHarness(t).PrepBuiltin("HelmValues")
	defer th.Reset()

	// make temp directory chartHome
	m := th.LoadAndRunTransformer(`
apiVersion: qlik.com/v1
kind: HelmValues
metadata:
  name: qliksense
chartName: qliksense
releaseName: qliksense
values:
  config:
    accessControl:
      testing: 1234
  qix-sessions:
    testing: true`, `
apiVersion: apps/v1
kind: HelmChart
metadata:
  name: qliksense
chartName: qliksense
releaseName: qliksense
values:
  config:
    accessControl:
      testing: 4321
---
apiVersion: apps/v1
kind: HelmChart
metadata:
  name: qix-sessions
chartName: qix-sessions
releaseName: qix-sessions
`)

	// insure output of yaml is correct
	th.AssertActualEqualsExpected(m, `
apiVersion: apps/v1
chartName: qliksense
kind: HelmChart
metadata:
  name: qliksense
releaseName: qliksense
values:
  config:
    accessControl:
      testing: 4321
  qix-sessions:
    testing: true
---
apiVersion: apps/v1
chartName: qix-sessions
kind: HelmChart
metadata:
  name: qix-sessions
releaseName: qliksense
values:
  qix-sessions:
    testing: true
`)

}
