package builtins_qlik

import (
	"fmt"

	"sigs.k8s.io/kustomize/api/builtins_qlik/utils"

	"sigs.k8s.io/kustomize/api/builtins"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/yaml"
	"go.uber.org/zap"
)

type SuperConfigMapPlugin struct {
	Data   map[string]interface{} `json:"data,omitempty" yaml:"data,omitempty"`
	logger *zap.SugaredLogger
	builtins.ConfigMapGeneratorPlugin
	SuperMapPluginBase
}

func (p *SuperConfigMapPlugin) Config(h *resmap.PluginHelpers, c []byte) (err error) {
	p.SuperMapPluginBase = NewBase(h.ResmapFactory(), p)
	p.Data = make(map[string](interface{}))
	err = yaml.Unmarshal(c, p)
	if err != nil {
		p.logger.Errorf("error unmarshalling yaml, error: %v\n", err)
		return err
	}
	err = p.SuperMapPluginBase.SetupTransformerConfig(h.Loader())
	if err != nil {
		p.logger.Errorf("error setting up transformer config, error: %v\n", err)
		return err
	}
	return p.ConfigMapGeneratorPlugin.Config(h, c)
}

func (p *SuperConfigMapPlugin) Generate() (resmap.ResMap, error) {
	for k, v := range p.Data {
		p.LiteralSources = append(p.LiteralSources, fmt.Sprintf("%v=%v", k, v))
	}
	return p.ConfigMapGeneratorPlugin.Generate()
}

func (p *SuperConfigMapPlugin) Transform(m resmap.ResMap) error {
	return p.SuperMapPluginBase.Transform(m)
}

func (p *SuperConfigMapPlugin) GetLogger() *zap.SugaredLogger {
	return p.logger
}

func (p *SuperConfigMapPlugin) GetName() string {
	return p.ConfigMapGeneratorPlugin.Name
}

func (p *SuperConfigMapPlugin) GetNamespace() string {
	return p.ConfigMapGeneratorPlugin.Namespace
}

func (p *SuperConfigMapPlugin) SetNamespace(namespace string) {
	p.ConfigMapGeneratorPlugin.Namespace = namespace
	p.ConfigMapGeneratorPlugin.GeneratorArgs.Namespace = namespace
}

func (p *SuperConfigMapPlugin) GetType() string {
	return "ConfigMap"
}

func (p *SuperConfigMapPlugin) GetConfigData() map[string]interface{} {
	return p.Data
}

func (p *SuperConfigMapPlugin) ShouldBase64EncodeConfigData() bool {
	return false
}

func (p *SuperConfigMapPlugin) GetDisableNameSuffixHash() bool {
	if p.ConfigMapGeneratorPlugin.Options != nil {
		return p.ConfigMapGeneratorPlugin.Options.DisableNameSuffixHash
	}
	return false
}

func NewSuperConfigMapTransformerPlugin() resmap.TransformerPlugin {
	return &SuperConfigMapPlugin{logger: utils.GetLogger("SuperConfigMapTransformerPlugin")}
}

func NewSuperConfigMapGeneratorPlugin() resmap.GeneratorPlugin {
	return &SuperConfigMapPlugin{logger: utils.GetLogger("SuperConfigMapGeneratorPlugin")}
}
