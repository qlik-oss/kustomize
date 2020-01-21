package builtins_qlik

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"


	"helm.sh/helm/v3/pkg/repo/repotest"
	"sigs.k8s.io/kustomize/api/builtins_qlik/utils/loadertest"
	"sigs.k8s.io/kustomize/api/internal/k8sdeps/transformer"
	"sigs.k8s.io/kustomize/api/k8sdeps/kunstruct"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	valtest_test "sigs.k8s.io/kustomize/api/testutils/valtest"
)

type mockPlugin struct {
}

func TestHelmChart(t *testing.T) {
	srv, err := repotest.NewTempServer("testdata/testcharts/*.tgz*")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}

	// all flags will get "-d outdir" appended.
	tests := []struct {
		name         string
		args         string
		expectFile   string
		expectDir    bool
		pluginConfig string
	}{
		{
			name:       "Fetch and untar",
			args:       "test/signtest --untar --untardir signtest",
			expectFile: "./signtest",
			expectDir:  true,
			pluginConfig: `
apiVersion: apps/v1
kind: HelmChart
metadata:
  name: qliksense
chartName: qliksense
releaseName: qliksense
chartRepoName: qliksense
chartRepo: https://qlik.bintray.com/edge
releaseNamespace: qliksense
releaseName: qliksense
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resourceFactory := resmap.NewFactory(resource.NewFactory(
				kunstruct.NewKunstructuredFactoryImpl()), transformer.NewFactoryImpl())

			plugin := NewHelmChartPlugin()
			err = plugin.Config(resmap.NewPluginHelpers(loadertest.NewFakeLoader("/"), valtest_test.MakeFakeValidator(), resourceFactory), []byte(tt.pluginConfig))
			if err != nil {
				t.Fatalf("Err: %v", err)
			}

			outdir := srv.Root()

			resMap, err := plugin.Generate()
			if err != nil {
				t.Fatalf("Err: %v", err)
			}

			for _, res := range resMap.Resources() {
				fmt.Printf("--res: %v\n", res.String())
			}

			ef := filepath.Join(outdir, tt.expectFile)
			fi, err := os.Stat(ef)
			if err != nil {
				t.Errorf("%q: expected a file at %s. %s", tt.name, ef, err)
			}
			if fi.IsDir() != tt.expectDir {
				t.Errorf("%q: expected directory=%t, but it's not.", tt.name, tt.expectDir)
			}
		})
	}
}