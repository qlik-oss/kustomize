module sigs.k8s.io/kustomize/cmd/config

go 1.16

require (
	github.com/go-errors/errors v1.0.1
	github.com/google/go-cmp v0.5.2 // indirect
	github.com/olekukonko/tablewriter v0.0.4
	github.com/spf13/cobra v1.0.0
	github.com/stretchr/testify v1.6.1
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b // indirect
	golang.org/x/text v0.3.4 // indirect
	gopkg.in/inf.v0 v0.9.1
	k8s.io/kube-openapi v0.0.0-20210421082810-95288971da7e
	sigs.k8s.io/kustomize/kyaml v0.10.19
)

replace gopkg.in/yaml.v3 => gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c

replace sigs.k8s.io/kustomize/kyaml => ../../kyaml

replace (
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20210323165736-1a6458611d18
	k8s.io/kube-openapi/compat => ../../compat/k8s.io/kube-openapi
)
