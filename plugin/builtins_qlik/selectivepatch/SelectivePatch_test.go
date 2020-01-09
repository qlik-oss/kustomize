package main

import (
	"os"
	"testing"

	"io/ioutil"
	kusttest_test "sigs.k8s.io/kustomize/api/testutils/kusttest"
)

func TestStrategicMergePatch(t *testing.T) {
	th := kusttest_test.MakeEnhancedHarness(t).PrepBuiltin("SelectivePatch")
	defer th.Reset()

	tmp, _ := ioutil.TempDir("", "testing")

	ioutil.WriteFile(tmp+"/patch.yaml", []byte(`
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: qliksense
  spec:
    template:
      metadata:
        labels:
          working: true
  `), 0644)
	rm := th.LoadAndRunTransformer(`
apiVersion: qlik.com/v1
kind: SelectivePatch
metadata:
  name: qliksense
enabled: true
patches:
- path: `+tmp+`/patch.yaml
  target:
    name: qliksense
`,
		`apiVersion: apps/v1
kind: Deployment
metadata:
  name: qliksense
spec:
  template:
    metadata:
      labels:
        working: false
`,
	)

	th.AssertActualEqualsExpected(rm, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: qliksense
spec:
  template:
    metadata:
      labels:
        working: true
`)
	os.RemoveAll(tmp)
}
