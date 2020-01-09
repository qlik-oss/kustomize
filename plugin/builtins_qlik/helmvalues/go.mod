module sigs.k8s.io/kustomize/plugin/builtins_qlik/helmvalues

go 1.13

require (
	github.com/imdario/mergo v0.3.5
	sigs.k8s.io/kustomize/api v0.3.1
	sigs.k8s.io/yaml v1.1.0
)

replace sigs.k8s.io/kustomize/api => ../../../api
