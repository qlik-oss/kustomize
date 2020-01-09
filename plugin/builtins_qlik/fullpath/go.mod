module sigs.k8s.io/kustomize/plugin/builtins_qlik/fullpath

go 1.13

require (
	github.com/stretchr/testify v1.4.0
	sigs.k8s.io/kustomize/api v0.3.1
	sigs.k8s.io/kustomize/plugin/builtins_qlik/utils v0.0.0-00010101000000-000000000000
	sigs.k8s.io/yaml v1.1.0
)

replace sigs.k8s.io/kustomize/api => ../../../api

replace sigs.k8s.io/kustomize/plugin/builtins_qlik/utils => ../utils
