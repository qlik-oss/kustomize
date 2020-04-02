package builtins_qlik

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/internal/k8sdeps/transformer"
	"sigs.k8s.io/kustomize/api/k8sdeps/kunstruct"
	"sigs.k8s.io/kustomize/api/loader"
	"sigs.k8s.io/kustomize/api/resid"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	valtest_test "sigs.k8s.io/kustomize/api/testutils/valtest"
)

func TestSearchReplacePlugin(t *testing.T) {

	testCases := []struct {
		name                 string
		pluginConfig         string
		pluginInputResources string
		checkAssertions      func(*testing.T, resmap.ResMap)
	}{
		{
			name: "relaxed is dangerous",
			pluginConfig: `
apiVersion: qlik.com/v1
kind: SearchReplace
metadata:
 name: notImportantHere
target:
 kind: Foo
 name: some-foo
path: fooSpec/fooTemplate/fooContainers/env/value
search: far
replace: not far
`,
			pluginInputResources: `
apiVersion: qlik.com/v1
kind: Foo
metadata:
 name: some-foo
fooSpec:
 fooTemplate:
   fooContainers:
   - name: have-env
     env:
     - name: FOO
       value: far
     - name: BOO
       value: farther than it looks
`,
			checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
				res := resMap.GetByIndex(0)

				envVars, err := res.GetFieldValue("fooSpec.fooTemplate.fooContainers[0].env")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				fooEnvVar := envVars.([]interface{})[0].(map[string]interface{})
				if "FOO" != fooEnvVar["name"].(string) {
					t.Fatalf("unexpected: %v\n", fooEnvVar["name"].(string))
				}
				if "not far" != fooEnvVar["value"].(string) {
					t.Fatalf("unexpected: %v\n", fooEnvVar["value"].(string))
				}

				booEnvVar := envVars.([]interface{})[1].(map[string]interface{})
				if "BOO" != booEnvVar["name"].(string) {
					t.Fatalf("unexpected: %v\n", booEnvVar["name"].(string))
				}
				if "not farther than it looks" != booEnvVar["value"].(string) {
					t.Fatalf("unexpected: %v\n", booEnvVar["value"].(string))
				}
			},
		},
		{
			name: "strict is safer",
			pluginConfig: `
apiVersion: qlik.com/v1
kind: SearchReplace
metadata:
 name: notImportantHere
target:
 kind: Foo
 name: some-foo
path: fooSpec/fooTemplate/fooContainers/env/value
search: ^far$
replace: not far
`,
			pluginInputResources: `
apiVersion: qlik.com/v1
kind: Foo
metadata:
 name: some-foo
fooSpec:
 fooTemplate:
   fooContainers:
   - name: have-env
     env:
     - name: FOO
       value: far
     - name: BOO
       value: farther than it looks
`,
			checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
				res := resMap.GetByIndex(0)

				envVars, err := res.GetFieldValue("fooSpec.fooTemplate.fooContainers[0].env")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				fooEnvVar := envVars.([]interface{})[0].(map[string]interface{})
				if "FOO" != fooEnvVar["name"].(string) {
					t.Fatalf("unexpected: %v\n", fooEnvVar["name"].(string))
				}
				if "not far" != fooEnvVar["value"].(string) {
					t.Fatalf("unexpected: %v\n", fooEnvVar["value"].(string))
				}

				booEnvVar := envVars.([]interface{})[1].(map[string]interface{})
				if "BOO" != booEnvVar["name"].(string) {
					t.Fatalf("unexpected: %v\n", booEnvVar["name"].(string))
				}
				if "farther than it looks" != booEnvVar["value"].(string) {
					t.Fatalf("unexpected: %v\n", booEnvVar["value"].(string))
				}
			},
		},
		{
			name: "object reference, GVK-only match",
			pluginConfig: `
apiVersion: qlik.com/v1
kind: SearchReplace
metadata:
 name: notImportantHere
target:
 kind: Foo
 name: some-foo
path: fooSpec/fooTemplate/fooContainers/env/value
search: ^far$
replaceWithObjRef:
 objref:
   apiVersion: qlik.com/v1
   kind: Bar
 fieldref:
   fieldpath: metadata.labels.myproperty
`,
			pluginInputResources: `
apiVersion: qlik.com/v1
kind: Foo
metadata:
 name: some-foo
fooSpec:
 fooTemplate:
   fooContainers:
   - name: have-env
     env:
     - name: FOO
       value: far
     - name: BOO
       value: farther than it looks
---
apiVersion: qlik.com/v1
kind: Bar
metadata:
 name: some-bar
 labels:
   myproperty: not far
fooSpec:
 test: test
---
apiVersion: qlik.com/v1
kind: Foo
metadata:
 name: some-Foo
 labels:
   myproperty: not good
`,
			checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
				res := resMap.GetByIndex(0)

				envVars, err := res.GetFieldValue("fooSpec.fooTemplate.fooContainers[0].env")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				fooEnvVar := envVars.([]interface{})[0].(map[string]interface{})
				if "FOO" != fooEnvVar["name"].(string) {
					t.Fatalf("unexpected: %v\n", fooEnvVar["name"].(string))
				}
				if "not far" != fooEnvVar["value"].(string) {
					t.Fatalf("unexpected: %v\n", fooEnvVar["value"].(string))
				}

				booEnvVar := envVars.([]interface{})[1].(map[string]interface{})
				if "BOO" != booEnvVar["name"].(string) {
					t.Fatalf("unexpected: %v\n", booEnvVar["name"].(string))
				}
				if "farther than it looks" != booEnvVar["value"].(string) {
					t.Fatalf("unexpected: %v\n", booEnvVar["value"].(string))
				}
			},
		},
		{
			name: "object reference, first GVK-only match",
			pluginConfig: `
apiVersion: qlik.com/v1
kind: SearchReplace
metadata:
 name: notImportantHere
target:
 kind: Foo
 name: some-foo
path: fooSpec/fooTemplate/fooContainers/env/value
search: ^far$
replaceWithObjRef:
 objref:
   apiVersion: qlik.com/
   kind: Bar
 fieldref:
   fieldpath: metadata.labels.myproperty
`,
			pluginInputResources: `
apiVersion: qlik.com/v1
kind: Foo
metadata:
 name: some-foo
fooSpec:
 fooTemplate:
   fooContainers:
   - name: have-env
     env:
     - name: FOO
       value: far
     - name: BOO
       value: farther than it looks
---
apiVersion: qlik.com/v1
kind: Bar
metadata:
 name: some-bar-1
 labels:
   myproperty: not far
fooSpec:
 test: test
---
apiVersion: qlik.com/v1
kind: Bar
metadata:
 name: some-bar-2
 labels:
   myproperty: too far
fooSpec:
 test: test
---
apiVersion: qlik.com/v1
kind: Foo
metadata:
 name: some-Foo
 labels:
   myproperty: not good
`,
			checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
				res := resMap.GetByIndex(0)

				envVars, err := res.GetFieldValue("fooSpec.fooTemplate.fooContainers[0].env")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				fooEnvVar := envVars.([]interface{})[0].(map[string]interface{})
				if "FOO" != fooEnvVar["name"].(string) {
					t.Fatalf("unexpected: %v\n", fooEnvVar["name"].(string))
				}
				if "not far" != fooEnvVar["value"].(string) {
					t.Fatalf("unexpected: %v\n", fooEnvVar["value"].(string))
				}

				booEnvVar := envVars.([]interface{})[1].(map[string]interface{})
				if "BOO" != booEnvVar["name"].(string) {
					t.Fatalf("unexpected: %v\n", booEnvVar["name"].(string))
				}
				if "farther than it looks" != booEnvVar["value"].(string) {
					t.Fatalf("unexpected: %v\n", booEnvVar["value"].(string))
				}
			},
		},
		{
			name: "object reference, GVK and name match",
			pluginConfig: `
apiVersion: qlik.com/v1
kind: SearchReplace
metadata:
 name: notImportantHere
target:
 kind: Foo
 name: some-foo
path: fooSpec/fooTemplate/fooContainers/env/value
search: ^far$
replaceWithObjRef:
 objref:
   apiVersion: qlik.com/
   kind: Bar
   name: some-bar
 fieldref:
   fieldpath: metadata.labels.myproperty
`,
			pluginInputResources: `
apiVersion: qlik.com/v1
kind: Foo
metadata:
 name: some-foo
fooSpec:
 fooTemplate:
   fooContainers:
   - name: have-env
     env:
     - name: FOO
       value: far
     - name: BOO
       value: farther than it looks
---
apiVersion: qlik.com/v1
kind: Bar
metadata:
 name: some-chocolate-bar
 labels:
   myproperty: not far enough
fooSpec:
 test: test
---
apiVersion: qlik.com/v1
kind: Bar
metadata:
 name: some-bar
 labels:
   myproperty: not far
fooSpec:
 test: test
---
apiVersion: qlik.com/v1
kind: Foo
metadata:
 name: some-Foo
 labels:
   myproperty: not good
`,
			checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
				res := resMap.GetByIndex(0)

				envVars, err := res.GetFieldValue("fooSpec.fooTemplate.fooContainers[0].env")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				fooEnvVar := envVars.([]interface{})[0].(map[string]interface{})
				if "FOO" != fooEnvVar["name"].(string) {
					t.Fatalf("unexpected: %v\n", fooEnvVar["name"].(string))
				}
				if "not far" != fooEnvVar["value"].(string) {
					t.Fatalf("unexpected: %v\n", fooEnvVar["value"].(string))
				}

				booEnvVar := envVars.([]interface{})[1].(map[string]interface{})
				if "BOO" != booEnvVar["name"].(string) {
					t.Fatalf("unexpected: %v\n", booEnvVar["name"].(string))
				}
				if "farther than it looks" != booEnvVar["value"].(string) {
					t.Fatalf("unexpected: %v\n", booEnvVar["value"].(string))
				}
			},
		},
		{
			name: "object reference, no match bo replace",
			pluginConfig: `
apiVersion: qlik.com/v1
kind: SearchReplace
metadata:
 name: notImportantHere
target:
 kind: Foo
 name: some-foo
path: fooSpec/fooTemplate/fooContainers/env/value
search: ^far$
replaceWithObjRef:
 objref:
   apiVersion: qlik.com/
   kind: Bar
   name: Foo
 fieldref:
   fieldpath: metadata.labels.myproperty
`,
			pluginInputResources: `
apiVersion: qlik.com/v1
kind: Foo
metadata:
 name: some-foo
fooSpec:
 fooTemplate:
   fooContainers:
   - name: have-env
     env:
     - name: FOO
       value: far
     - name: BOO
       value: farther than it looks
---
apiVersion: qlik.com/v1
kind: Bar
metadata:
 name: some-chocolate-bar
 labels:
   myproperty: not far enough
fooSpec:
 test: test
---
apiVersion: qlik.com/v1
kind: Bar
metadata:
 name: some-bar
 labels:
   myproperty: not far
fooSpec:
 test: test
---
apiVersion: qlik.com/v1
kind: Foo
metadata:
 name: some-Foo
 labels:
   myproperty: not good
`,
			checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
				res := resMap.GetByIndex(0)

				envVars, err := res.GetFieldValue("fooSpec.fooTemplate.fooContainers[0].env")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				fooEnvVar := envVars.([]interface{})[0].(map[string]interface{})
				if "FOO" != fooEnvVar["name"].(string) {
					t.Fatalf("unexpected: %v\n", fooEnvVar["name"].(string))
				}
				if "far" != fooEnvVar["value"].(string) {
					t.Fatalf("unexpected: %v\n", fooEnvVar["value"].(string))
				}

				booEnvVar := envVars.([]interface{})[1].(map[string]interface{})
				if "BOO" != booEnvVar["name"].(string) {
					t.Fatalf("unexpected: %v\n", booEnvVar["name"].(string))
				}
				if "farther than it looks" != booEnvVar["value"].(string) {
					t.Fatalf("unexpected: %v\n", booEnvVar["value"].(string))
				}
			},
		},
		{
			name: "can replace with a blank",
			pluginConfig: `
apiVersion: qlik.com/v1
kind: SearchReplace
metadata:
 name: notImportantHere
target:
 kind: Foo
 name: some-foo
path: fooSpec/fooTemplate/fooContainers/env/value
search: ^far$
replace: ""
`,
			pluginInputResources: `
apiVersion: qlik.com/v1
kind: Foo
metadata:
 name: some-foo
fooSpec:
 fooTemplate:
   fooContainers:
   - name: have-env
     env:
     - name: FOO
       value: far
     - name: BOO
       value: farther than it looks
`,
			checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
				res := resMap.GetByIndex(0)

				envVars, err := res.GetFieldValue("fooSpec.fooTemplate.fooContainers[0].env")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				fooEnvVar := envVars.([]interface{})[0].(map[string]interface{})
				if "FOO" != fooEnvVar["name"].(string) {
					t.Fatalf("unexpected: %v\n", fooEnvVar["name"].(string))
				}
				if "" != fooEnvVar["value"].(string) {
					t.Fatalf("unexpected: %v\n", fooEnvVar["value"].(string))
				}

				booEnvVar := envVars.([]interface{})[1].(map[string]interface{})
				if "BOO" != booEnvVar["name"].(string) {
					t.Fatalf("unexpected: %v\n", booEnvVar["name"].(string))
				}
				if "farther than it looks" != booEnvVar["value"].(string) {
					t.Fatalf("unexpected: %v\n", booEnvVar["value"].(string))
				}
			},
		},
		{
			name: "can replace with a blank from object ref",
			pluginConfig: `
apiVersion: qlik.com/v1
kind: SearchReplace
metadata:
 name: notImportantHere
target:
 kind: Foo
 name: some-foo
path: fooSpec/fooTemplate/fooContainers/env/value
search: ^far$
replaceWithObjRef:
 objref:
   apiVersion: qlik.com/
   kind: Bar
   name: Foo
 fieldref:
   fieldpath: metadata.labels.myproperty
`,
			pluginInputResources: `
apiVersion: qlik.com/v1
kind: Foo
metadata:
 name: some-foo
fooSpec:
 fooTemplate:
   fooContainers:
   - name: have-env
     env:
     - name: FOO
       value: far
     - name: BOO
       value: farther than it looks
---
apiVersion: qlik.com/v1
kind: Bar
metadata:
 name: Foo
 labels:
   myproperty: ""
fooSpec:
 test: test
`,
			checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
				res := resMap.GetByIndex(0)

				envVars, err := res.GetFieldValue("fooSpec.fooTemplate.fooContainers[0].env")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				fooEnvVar := envVars.([]interface{})[0].(map[string]interface{})
				if "FOO" != fooEnvVar["name"].(string) {
					t.Fatalf("unexpected: %v\n", fooEnvVar["name"].(string))
				}
				if "" != fooEnvVar["value"].(string) {
					t.Fatalf("unexpected: %v\n", fooEnvVar["value"].(string))
				}

				booEnvVar := envVars.([]interface{})[1].(map[string]interface{})
				if "BOO" != booEnvVar["name"].(string) {
					t.Fatalf("unexpected: %v\n", booEnvVar["name"].(string))
				}
				if "farther than it looks" != booEnvVar["value"].(string) {
					t.Fatalf("unexpected: %v\n", booEnvVar["value"].(string))
				}
			},
		},
		{
			name: "replace label keys",
			pluginConfig: `
apiVersion: qlik.com/v1
kind: SearchReplace
metadata:
  name: notImportantHere
target:
  kind: Deployment
path: spec/template/metadata/labels
search: \b[^"]*-messaging-nats-client\b
replace: foo-messaging-nats-client
`,
			pluginInputResources: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deployment-1
spec:
  template:
    metadata:
      labels:
        app: some-app
        something-messaging-nats-client: "true"
        release: some-release
    spec:
      containers:
      - name: name-1
        image: image-1:latest
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deployment-2
spec:
  template:
    metadata:
      labels:
        app: some-app
        something-messaging-nats-client: "true"
        release: some-release
    spec:
      containers:
      - name: name-2
        image: image-2:latest
`,
			checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
				for _, res := range resMap.Resources() {
					labels, err := res.GetFieldValue("spec.template.metadata.labels")
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}

					appLabel := labels.(map[string]interface{})["app"].(string)
					if "some-app" != appLabel {
						t.Fatalf("unexpected: %v\n", appLabel)
					}

					natsClientLabe := labels.(map[string]interface{})["foo-messaging-nats-client"].(string)
					if "true" != natsClientLabe {
						t.Fatalf("unexpected: %v\n", natsClientLabe)
					}

					releaseLabel := labels.(map[string]interface{})["release"].(string)
					if "some-release" != releaseLabel {
						t.Fatalf("unexpected: %v\n", releaseLabel)
					}
				}
			},
		},
		{
			name: "replace label key for a custom type and a dollar-variable",
			pluginConfig: `
apiVersion: qlik.com/v1
kind: SearchReplace
metadata:
  name: notImportantHere
target:
  kind: Engine
path: spec/metadata/labels
search: \$\(PREFIX\)-messaging-nats-client
replace: foo-messaging-nats-client
`,
			pluginInputResources: `
apiVersion: qixmanager.qlik.com/v1
kind: Engine
metadata:
  name: whatever-engine
spec:
  metadata:
    labels:
      $(PREFIX)-messaging-nats-client: "true"
`,
			checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
				for _, res := range resMap.Resources() {
					labels, err := res.GetFieldValue("spec.metadata.labels")
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}

					natsClientLabe := labels.(map[string]interface{})["foo-messaging-nats-client"].(string)
					if "true" != natsClientLabe {
						t.Fatalf("unexpected: %v\n", natsClientLabe)
					}
				}
			},
		},
		{
			name: "base64-encoded target and replacement",
			pluginConfig: `
apiVersion: qlik.com/v1
kind: SearchReplace
metadata:
  name: notImportantHere
target:
  kind: Secret
  name: keycloak-secret
path: data/idpConfigs
search: \$\(QLIKSENSE_DOMAIN\)
replaceWithObjRef:
  objref:
    apiVersion: qlik.com/v1
    kind: Secret
    name: gke-configs
  fieldref:
    fieldpath: data.qlikSenseDomain
`,
			pluginInputResources: `
apiVersion: v1
kind: Secret
metadata:
  name: keycloak-secret
type: Opaque
data:
  idpConfigs: JChRTElLU0VOU0VfRE9NQUlOKS5iYXIuY29t
---
apiVersion: v1
kind: Secret
metadata:
  name: gke-configs
type: Opaque
data:
  qlikSenseDomain: Zm9v
`,
			checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
				res, err := resMap.GetById(resid.NewResId(resid.Gvk{
					Group:   "",
					Version: "v1",
					Kind:    "Secret",
				}, "keycloak-secret"))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				base64EncodedIdpConfigs, err := res.GetFieldValue("data.idpConfigs")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				decodedIdpConfigsBytes, err := base64.StdEncoding.DecodeString(base64EncodedIdpConfigs.(string))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if string(decodedIdpConfigsBytes) != "foo.bar.com" {
					t.Fatalf("expected %v to equal %v", string(decodedIdpConfigsBytes), "foo.bar.com")
				}
			},
		},
		{
			name: "base64-encoded target only",
			pluginConfig: `
apiVersion: qlik.com/v1
kind: SearchReplace
metadata:
  name: notImportantHere
target:
  kind: Secret
  name: keycloak-secret
path: data/idpConfigs
search: \$\(QLIKSENSE_DOMAIN\)
replaceWithObjRef:
  objref:
    apiVersion: qlik.com/v1
    kind: ConfigMap
    mame: gke-configs
  fieldref:
    fieldpath: data.qlikSenseDomain
`,
			pluginInputResources: `
apiVersion: v1
kind: Secret
metadata:
  name: keycloak-secret
type: Opaque
data:
  idpConfigs: JChRTElLU0VOU0VfRE9NQUlOKS5iYXIuY29t
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: gke-configs
data:
  qlikSenseDomain: foo
`,
			checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
				res, err := resMap.GetById(resid.NewResId(resid.Gvk{
					Group:   "",
					Version: "v1",
					Kind:    "Secret",
				}, "keycloak-secret"))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				base64EncodedIdpConfigs, err := res.GetFieldValue("data.idpConfigs")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				decodedIdpConfigsBytes, err := base64.StdEncoding.DecodeString(base64EncodedIdpConfigs.(string))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if string(decodedIdpConfigsBytes) != "foo.bar.com" {
					t.Fatalf("expected %v to equal %v", string(decodedIdpConfigsBytes), "foo.bar.com")
				}
			},
		},
		{
			name: "base64-encoded replacement only",
			pluginConfig: `
apiVersion: qlik.com/v1
kind: SearchReplace
metadata:
  name: notImportantHere
target:
  kind: ConfigMap
  name: keycloak-config
path: data/idpConfigs
search: \$\(QLIKSENSE_DOMAIN\)
replaceWithObjRef:
  objref:
    apiVersion: qlik.com/v1
    kind: Secret
    mame: gke-secrets
  fieldref:
    fieldpath: data.qlikSenseDomain
`,
			pluginInputResources: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: keycloak-config
data:
  idpConfigs: $(QLIKSENSE_DOMAIN).bar.com
---
apiVersion: v1
kind: Secret
metadata:
  name: gke-secrets
type: Opaque
data:
  qlikSenseDomain: Zm9v
`,
			checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
				res, err := resMap.GetById(resid.NewResId(resid.Gvk{
					Group:   "",
					Version: "v1",
					Kind:    "ConfigMap",
				}, "keycloak-config"))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				idpConfigs, err := res.GetFieldValue("data.idpConfigs")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if idpConfigs.(string) != "foo.bar.com" {
					t.Fatalf("expected %v to equal %v", idpConfigs.(string), "foo.bar.com")
				}
			},
		},
		{
			name: "replace SuperSecret stringData from SuperConfigMap data",
			pluginConfig: `
apiVersion: qlik.com/v1
kind: SearchReplace
metadata:
  name: keycloak-realm-name
target:
  kind: SuperSecret
  name: keycloak-realm
path: stringData/realm.json
search: \$\(REALM_NAME\)
replaceWithObjRef:
  objref:
    apiVersion: qlik.com/v1
    kind: SuperConfigMap
    mame: keycloak-configs
  fieldref:
    fieldpath: data.realmName
`,
			pluginInputResources: `
apiVersion: qlik.com/v1
assumeTargetWillExist: true
data:
  idpHostName: keycloak-dev.qseok.tk
  imageRegistry: qlik-docker-qsefe.bintray.io
  ingressAuthUrl: http://$(PREFIX)-edge-auth.$(NAMESPACE).svc.cluster.local:8080/v1/auth
  ingressClass: qlik-nginx
  qlikSenseIp: 35.203.97.246
  realmName: QSEoK
  staticIpName: keycloak-dev-ip
  storageClassName: ""
disableNameSuffixHash: false
kind: SuperConfigMap
metadata:
  labels:
    app: keycloak
  name: keycloak-configs
prefix: $(PREFIX)-
---
apiVersion: qlik.com/v1
kind: SuperSecret
metadata:
  labels:
    app: keycloak
  name: keycloak-realm
stringData:
  realm.json: |-
    {
        "realm": "$(REALM_NAME)",
        "enabled": true,
        "sslRequired": "external",
        "registrationAllowed": false,
        "requiredCredentials": [
            "password"
        ],
        "roles": {
            "realm": [
                {
                    "name": "user",
                    "description": "User privileges"
                },
                {
                    "name": "admin",
                    "description": "Administrator privileges"
                }
            ]
        },
        "groups": [
            {
                "name": "Accounting",
                "path": "/Accounting"
            },
            {
                "name": "Administrators",
                "path": "/Administrators"
            },
            {
                "name": "Engineering",
                "path": "/Engineering"
            },
            {
                "name": "Everyone",
                "path": "/Everyone"
            },
            {
                "name": "Marketing",
                "path": "/Marketing"
            },
            {
                "name": "Sales",
                "path": "/Sales"
            },
            {
                "name": "Support",
                "path": "/Support"
            }
        ],
        "users": [
            {
                "username": "barb",
                "enabled": true,
                "emailVerified": true,
                "firstName": "Barb",
                "lastName": "Stovin",
                "email": "barb@qlik.example",
                "credentials": [
                    {
                        "type": "password",
                        "value": "$(DEFAULT_USER_PASSWORD)"
                    }
                ],
                "requiredActions": [
                    "UPDATE_PASSWORD"
                ],
                "realmRoles": [
                    "user"
                ],
                "clientRoles": {
                    "account": [
                        "manage-account",
                        "view-profile"
                    ]
                },
                "groups": [
                    "/Everyone",
                    "/Support"
                ]
            },
            {
                "username": "evan",
                "enabled": true,
                "emailVerified": true,
                "firstName": "Evan",
                "lastName": "Highman",
                "email": "evan@qlik.example",
                "credentials": [
                    {
                        "type": "password",
                        "value": "$(DEFAULT_USER_PASSWORD)"
                    }
                ],
                "requiredActions": [
                    "UPDATE_PASSWORD"
                ],
                "realmRoles": [
                    "user"
                ],
                "clientRoles": {
                    "account": [
                        "manage-account",
                        "view-profile"
                    ]
                },
                "groups": [
                    "/Engineering",
                    "/Everyone"
                ]
            },
            {
                "username": "franklin",
                "enabled": true,
                "emailVerified": true,
                "firstName": "Franklin",
                "lastName": "Glamart",
                "email": "franklin@qlik.example",
                "credentials": [
                    {
                        "type": "password",
                        "value": "$(DEFAULT_USER_PASSWORD)"
                    }
                ],
                "requiredActions": [
                    "UPDATE_PASSWORD"
                ],
                "realmRoles": [
                    "user"
                ],
                "clientRoles": {
                    "account": [
                        "manage-account",
                        "view-profile"
                    ]
                },
                "groups": [
                    "/Everyone",
                    "/Sales"
                ]
            },
            {
                "username": "harley",
                "enabled": true,
                "emailVerified": true,
                "firstName": "Harley",
                "lastName": "Kiffe",
                "email": "harley@qlik.example",
                "credentials": [
                    {
                        "type": "password",
                        "value": "$(DEFAULT_USER_PASSWORD)"
                    }
                ],
                "requiredActions": [
                    "UPDATE_PASSWORD"
                ],
                "realmRoles": [
                    "user"
                ],
                "clientRoles": {
                    "account": [
                        "manage-account",
                        "view-profile"
                    ]
                },
                "groups": [
                    "/Everyone",
                    "/Sales"
                ]
            },
            {
                "username": "marne",
                "enabled": true,
                "emailVerified": true,
                "firstName": "Marne",
                "lastName": "Probetts",
                "email": "marne@qlik.example",
                "credentials": [
                    {
                        "type": "password",
                        "value": "$(DEFAULT_USER_PASSWORD)"
                    }
                ],
                "requiredActions": [
                    "UPDATE_PASSWORD"
                ],
                "realmRoles": [
                    "user"
                ],
                "clientRoles": {
                    "account": [
                        "manage-account",
                        "view-profile"
                    ]
                },
                "groups": [
                    "/Everyone",
                    "/Marketing"
                ]
            },
            {
                "username": "peta",
                "enabled": true,
                "emailVerified": true,
                "firstName": "Peta",
                "lastName": "Sammon",
                "email": "peta@qlik.example",
                "credentials": [
                    {
                        "type": "password",
                        "value": "$(DEFAULT_USER_PASSWORD)"
                    }
                ],
                "requiredActions": [
                    "UPDATE_PASSWORD"
                ],
                "realmRoles": [
                    "user"
                ],
                "clientRoles": {
                    "account": [
                        "manage-account",
                        "view-profile"
                    ]
                },
                "groups": [
                    "/Engineering",
                    "/Everyone"
                ]
            },
            {
                "username": "phillie",
                "enabled": true,
                "emailVerified": true,
                "firstName": "Phillie",
                "lastName": "Smeed",
                "email": "phillie@qlik.example",
                "credentials": [
                    {
                        "type": "password",
                        "value": "$(DEFAULT_USER_PASSWORD)"
                    }
                ],
                "requiredActions": [
                    "UPDATE_PASSWORD"
                ],
                "realmRoles": [
                    "user"
                ],
                "clientRoles": {
                    "account": [
                        "manage-account",
                        "view-profile"
                    ]
                },
                "notBefore": 0,
                "groups": [
                    "/Everyone",
                    "/Marketing",
                    "/Sales"
                ]
            },
            {
                "username": "quinn",
                "enabled": true,
                "emailVerified": true,
                "firstName": "Quinn",
                "lastName": "Leeming",
                "email": "quinn@qlik.example",
                "credentials": [
                    {
                        "type": "password",
                        "value": "$(DEFAULT_USER_PASSWORD)"
                    }
                ],
                "requiredActions": [
                    "UPDATE_PASSWORD"
                ],
                "realmRoles": [
                    "user"
                ],
                "clientRoles": {
                    "account": [
                        "manage-account",
                        "view-profile"
                    ]
                },
                "notBefore": 0,
                "groups": [
                    "/Accounting",
                    "/Everyone"
                ]
            },
            {
                "username": "rootadmin",
                "enabled": true,
                "emailVerified": true,
                "firstName": "Root",
                "lastName": "Admin",
                "email": "rootadmin@qlik.example",
                "credentials": [
                    {
                        "type": "password",
                        "value": "$(DEFAULT_USER_PASSWORD)"
                    }
                ],
                "requiredActions": [
                    "UPDATE_PASSWORD"
                ],
                "realmRoles": [
                    "user",
                    "admin"
                ],
                "clientRoles": {
                    "account": [
                        "realm-admin"
                    ]
                },
                "notBefore": 0,
                "groups": [
                    "/Administrators",
                    "/Everyone"
                ]
            },
            {
                "username": "sibylla",
                "enabled": true,
                "emailVerified": true,
                "firstName": "Sibylla",
                "lastName": "Meadows",
                "email": "sibylla@qlik.example",
                "credentials": [
                    {
                        "type": "password",
                        "value": "$(DEFAULT_USER_PASSWORD)"
                    }
                ],
                "requiredActions": [
                    "UPDATE_PASSWORD"
                ],
                "realmRoles": [
                    "user"
                ],
                "clientRoles": {
                    "account": [
                        "manage-account",
                        "view-profile"
                    ]
                },
                "groups": [
                    "/Accounting",
                    "/Everyone"
                ]
            },
            {
                "username": "sim",
                "enabled": true,
                "emailVerified": true,
                "firstName": "Sim",
                "lastName": "Cleaton",
                "email": "sim@qlik.example",
                "credentials": [
                    {
                        "type": "password",
                        "value": "$(DEFAULT_USER_PASSWORD)"
                    }
                ],
                "requiredActions": [
                    "UPDATE_PASSWORD"
                ],
                "realmRoles": [
                    "user"
                ],
                "clientRoles": {
                    "account": [
                        "manage-account",
                        "view-profile"
                    ]
                },
                "groups": [
                    "/Accounting",
                    "/Everyone"
                ]
            }
        ],
        "clients": [
            {
                "clientId": "edge-auth",
                "enabled": true,
                "directAccessGrantsEnabled": true,
                "clientAuthenticatorType": "client-secret",
                "secret": "$(CLIENT_SECRET)",
                "redirectUris": [
                    "*"
                ],
                "publicClient": false,
                "protocol": "openid-connect",
                "protocolMappers": [
                    {
                        "name": "groups",
                        "protocol": "openid-connect",
                        "protocolMapper": "oidc-group-membership-mapper",
                        "config": {
                            "full.path": "false",
                            "claim.name": "groups"
                        }
                    },
                    {
                        "name": "Client IP Address",
                        "protocol": "openid-connect",
                        "protocolMapper": "oidc-usersessionmodel-note-mapper",
                        "config": {
                            "user.session.note": "clientAddress",
                            "claim.name": "clientAddress",
                            "id.token.claim": "true",
                            "access.token.claim": "true",
                            "jsonType.label": "String"
                        }
                    },
                    {
                        "name": "Client ID",
                        "protocol": "openid-connect",
                        "protocolMapper": "oidc-usersessionmodel-note-mapper",
                        "config": {
                            "user.session.note": "clientId",
                            "claim.name": "clientId",
                            "id.token.claim": "true",
                            "access.token.claim": "true",
                            "jsonType.label": "String"
                        }
                    },
                    {
                        "name": "Client Host",
                        "protocol": "openid-connect",
                        "protocolMapper": "oidc-usersessionmodel-note-mapper",
                        "config": {
                            "user.session.note": "clientHost",
                            "claim.name": "clientHost",
                            "id.token.claim": "true",
                            "access.token.claim": "true",
                            "jsonType.label": "String"
                        }
                    }
                ]
            }
        ]
    }
---
apiVersion: qlik.com/v1
assumeTargetWillExist: true
disableNameSuffixHash: false
kind: SuperSecret
literals: []
metadata:
  labels:
    app: keycloak
  name: keycloak-secrets
prefix: $(PREFIX)-
stringData:
  clientSecret: ((- "\n"))(( index (ds "data") "clientSecret" | base64.Decode | indent
    8 ))
  defaultUserPassword: ((- "\n"))(( index (ds "data") "defaultUserPassword" | base64.Decode
    | indent 8 ))
  password: ((- "\n"))(( index (ds "data") "password" | base64.Decode | indent 8 ))
  postgresqlPassword: A0Xz4X0oS4
  something: else
`,
			checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
				res, err := resMap.GetById(resid.NewResId(resid.Gvk{
					Group:   "qlik.com",
					Version: "v1",
					Kind:    "SuperSecret",
				}, "keycloak-realm"))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				stringData, err := res.GetFieldValue("stringData")
				realmJson, found := stringData.(map[string]interface{})["realm.json"]
				if !found {
					t.Fatalf("could not find realm.json")
				}

				if strings.Contains(realmJson.(string), "$(REALM_NAME)") {
					t.Fatalf("expected %v to not contain $(REALM_NAME)", realmJson.(string))
				}
			},
		},
	}
	plugin := SearchReplacePlugin{logger: log.New(os.Stdout, "", 0)}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resourceFactory := resmap.NewFactory(resource.NewFactory(kunstruct.NewKunstructuredFactoryImpl()), transformer.NewFactoryImpl())

			resMap, err := resourceFactory.NewResMapFromBytes([]byte(testCase.pluginInputResources))
			if err != nil {
				t.Fatalf("Err: %v", err)
			}

			h := resmap.NewPluginHelpers(loader.NewFileLoaderAtRoot(filesys.MakeFsInMemory()), valtest_test.MakeHappyMapValidator(t), resourceFactory)
			if err := plugin.Config(h, []byte(testCase.pluginConfig)); err != nil {
				t.Fatalf("Err: %v", err)
			}

			if err := plugin.Transform(resMap); err != nil {
				t.Fatalf("Err: %v", err)
			}

			for _, res := range resMap.Resources() {
				fmt.Printf("--res: %v\n", res.String())
			}

			testCase.checkAssertions(t, resMap)
		})
	}
}
