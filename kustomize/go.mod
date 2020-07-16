module sigs.k8s.io/kustomize/kustomize/v3

go 1.14

require (
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	k8s.io/client-go v0.17.3
	sigs.k8s.io/kustomize/api v0.5.1
	sigs.k8s.io/kustomize/cmd/config v0.2.0
	sigs.k8s.io/yaml v1.2.0
)

exclude (
	github.com/Azure/go-autorest v12.0.0+incompatible
	github.com/russross/blackfriday v2.0.0+incompatible
	sigs.k8s.io/kustomize/api v0.2.0
)
	github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309
	k8s.io/client-go => k8s.io/client-go v0.17.0
