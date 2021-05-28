module sigs.k8s.io/kustomize/api

go 1.16

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/cnf/structhash v0.0.0-20201127153200-e1b16c1ebc08 // indirect
	github.com/evanphx/json-patch v4.9.0+incompatible
	github.com/go-errors/errors v1.0.1
	github.com/gofrs/flock v0.8.0
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/google/uuid v1.2.0
	github.com/hairyhenderson/gomplate/v3 v3.9.0
	github.com/hashicorp/go-version v1.3.0
	github.com/imdario/mergo v0.3.11
	github.com/mholt/archiver/v3 v3.5.0
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.7.0
	github.com/traefik/yaegi v0.9.17
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	helm.sh/helm/v3 v3.5.4
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/kube-openapi v0.0.0-20210421082810-95288971da7e
	sigs.k8s.io/kustomize/kyaml v0.10.19
	sigs.k8s.io/yaml v1.2.0
)

replace gopkg.in/yaml.v3 => gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c

replace sigs.k8s.io/kustomize/kyaml => ../kyaml

replace (
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
	helm.sh/helm/v3 => github.com/qlik-oss/helm/v3 v3.5.5-0.20210512001905-0b788664d855
	k8s.io/api => k8s.io/api v0.20.4
	k8s.io/client-go => k8s.io/client-go v0.20.4
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20210323165736-1a6458611d18
	k8s.io/kube-openapi/compat => ../compat/k8s.io/kube-openapi
)
