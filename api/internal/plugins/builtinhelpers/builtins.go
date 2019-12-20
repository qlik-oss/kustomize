// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package builtinhelpers

import (
	"sigs.k8s.io/kustomize/api/builtins"
	"sigs.k8s.io/kustomize/api/builtins_qlik"
	"sigs.k8s.io/kustomize/api/resmap"
)

//go:generate stringer -type=BuiltinPluginType
type BuiltinPluginType int

const (
	Unknown BuiltinPluginType = iota
	AnnotationsTransformer
	ConfigMapGenerator
	HashTransformer
	ImageTagTransformer
	InventoryTransformer
	LabelTransformer
	LegacyOrderTransformer
	NamespaceTransformer
	PatchJson6902Transformer
	PatchStrategicMergeTransformer
	PatchTransformer
	PrefixSuffixTransformer
	ReplicaCountTransformer
	SecretGenerator

	ChartHomeFullPath
	EnvUpsert
	FullPath
	Gomplate
	HelmChart
	HelmValues
	SedOnPath
	SuperVars
	SuperConfigMap
	SuperSecret
)

var stringToBuiltinPluginTypeMap map[string]BuiltinPluginType

func init() {
	stringToBuiltinPluginTypeMap = makeStringToBuiltinPluginTypeMap()

	TransformerFactories[ChartHomeFullPath] = builtins_qlik.NewChartHomeFullPathPlugin
	stringToBuiltinPluginTypeMap["ChartHomeFullPath"] = ChartHomeFullPath

	TransformerFactories[EnvUpsert] = builtins_qlik.NewEnvUpsertPlugin
	stringToBuiltinPluginTypeMap["EnvUpsert"] = EnvUpsert

	TransformerFactories[FullPath] = builtins_qlik.NewFullPathPlugin
	stringToBuiltinPluginTypeMap["FullPath"] = FullPath

	TransformerFactories[Gomplate] = builtins_qlik.NewGomplatePlugin
	stringToBuiltinPluginTypeMap["Gomplate"] = Gomplate

	GeneratorFactories[HelmChart] = builtins_qlik.NewHelmChartPlugin
	stringToBuiltinPluginTypeMap["HelmChart"] = HelmChart

	TransformerFactories[HelmValues] = builtins_qlik.NewHelmValuesPlugin
	stringToBuiltinPluginTypeMap["HelmValues"] = HelmValues

	TransformerFactories[SedOnPath] = builtins_qlik.NewSedOnPathPlugin
	stringToBuiltinPluginTypeMap["SedOnPath"] = SedOnPath

	TransformerFactories[SuperVars] = builtins_qlik.NewSuperVarsPlugin
	stringToBuiltinPluginTypeMap["SuperVars"] = SuperVars

	TransformerFactories[SuperConfigMap] = builtins_qlik.NewSuperConfigMapTransformerPlugin
	GeneratorFactories[SuperConfigMap] = builtins_qlik.NewSuperConfigMapGeneratorPlugin
	stringToBuiltinPluginTypeMap["SuperConfigMap"] = SuperConfigMap

	TransformerFactories[SuperSecret] = builtins_qlik.NewSuperSecretTransformerPlugin
	GeneratorFactories[SuperSecret] = builtins_qlik.NewSuperSecretGeneratorPlugin
	stringToBuiltinPluginTypeMap["SuperSecret"] = SuperSecret
}

func makeStringToBuiltinPluginTypeMap() (result map[string]BuiltinPluginType) {
	result = make(map[string]BuiltinPluginType, 23)
	for k := range GeneratorFactories {
		result[k.String()] = k
	}
	for k := range TransformerFactories {
		result[k.String()] = k
	}
	return
}

func GetBuiltinPluginType(n string) BuiltinPluginType {
	result, ok := stringToBuiltinPluginTypeMap[n]
	if ok {
		return result
	}
	return Unknown
}

var GeneratorFactories = map[BuiltinPluginType]func() resmap.GeneratorPlugin{
	ConfigMapGenerator: builtins.NewConfigMapGeneratorPlugin,
	SecretGenerator:    builtins.NewSecretGeneratorPlugin,
}

var TransformerFactories = map[BuiltinPluginType]func() resmap.TransformerPlugin{
	AnnotationsTransformer:         builtins.NewAnnotationsTransformerPlugin,
	HashTransformer:                builtins.NewHashTransformerPlugin,
	ImageTagTransformer:            builtins.NewImageTagTransformerPlugin,
	InventoryTransformer:           builtins.NewInventoryTransformerPlugin,
	LabelTransformer:               builtins.NewLabelTransformerPlugin,
	LegacyOrderTransformer:         builtins.NewLegacyOrderTransformerPlugin,
	NamespaceTransformer:           builtins.NewNamespaceTransformerPlugin,
	PatchJson6902Transformer:       builtins.NewPatchJson6902TransformerPlugin,
	PatchStrategicMergeTransformer: builtins.NewPatchStrategicMergeTransformerPlugin,
	PatchTransformer:               builtins.NewPatchTransformerPlugin,
	PrefixSuffixTransformer:        builtins.NewPrefixSuffixTransformerPlugin,
	ReplicaCountTransformer:        builtins.NewReplicaCountTransformerPlugin,
}
