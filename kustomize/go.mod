module sigs.k8s.io/kustomize/kustomize/v4

go 1.16

require (
	github.com/google/go-cmp v0.5.5
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	sigs.k8s.io/kustomize/api v0.8.9
	sigs.k8s.io/kustomize/cmd/config v0.9.11
	sigs.k8s.io/kustomize/kyaml v0.10.19
	sigs.k8s.io/yaml v1.2.0
)

exclude (
	sigs.k8s.io/kustomize/api v0.2.0
	sigs.k8s.io/kustomize/cmd/config v0.2.0
)

replace sigs.k8s.io/kustomize/kyaml => ../kyaml

replace sigs.k8s.io/kustomize/cmd/config => ../cmd/config

replace sigs.k8s.io/kustomize/api => ../api

replace (
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
	helm.sh/helm/v3 => github.com/qlik-oss/helm/v3 v3.5.5-0.20210512001905-0b788664d855
	k8s.io/api => k8s.io/api v0.20.4
	k8s.io/client-go => k8s.io/client-go v0.20.4
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20210323165736-1a6458611d18
	k8s.io/kube-openapi/compat => ../compat/k8s.io/kube-openapi
)
