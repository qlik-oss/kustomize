package builtins_qlik

import (
	"fmt"
	"io/ioutil"
	"os"

	"go.uber.org/zap"
	"sigs.k8s.io/kustomize/api/builtins_qlik/utils"
	"sigs.k8s.io/kustomize/api/filters/fieldspec"
	"sigs.k8s.io/kustomize/api/ifc"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filtersutil"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"sigs.k8s.io/yaml"
)

// inserts a processed gomplate file into a kustomize resource
type GomsertPlugin struct {
	LDelim      string   `json:"leftDelimeter" 	yaml:"leftDelimeter"`
	RDelim      string   `json:"rightDelimeter" 	yaml:"rightDelimeter"`
	DataSources []string `json:"dataSources,omitempty" 	yaml:"dataSources,omitempty"`
	InputFile   string   `json:"inputFile,omitempty" 	yaml:"inputFile,omitempty"`
	EnvVars     []struct {
		Name          string `json:"name" 	yaml:"name"`
		ValueFromEnv  string `json:"valueFromEnv,omitempty" 	yaml:"valueFromEnv,omitempty"`
		ValueFromFile string `json:"valueFromFile,omitempty" 	yaml:"valueFromFile,omitempty"`
		Value         string `json:"value,omitempty" 	yaml:"value,omitempty"`
	} `json:"envVars,omitempty" 	yaml:"envVars,omitempty"`
	Target      *types.Selector `json:"target,omitempty" yaml:"target,omitempty"`
	Path        string          `json:"path,omitempty" yaml:"path,omitempty"`
	Pwd         string
	ldr         ifc.Loader
	rf          *resmap.Factory
	logger      *zap.SugaredLogger
	fieldSpec   types.FieldSpec
	replaceYaml []byte
}

func (p *GomsertPlugin) Config(h *resmap.PluginHelpers, c []byte) (err error) {
	p.ldr = h.Loader()
	p.rf = h.ResmapFactory()
	p.Pwd = h.Loader().Root()
	return yaml.Unmarshal(c, p)
}

func (p *GomsertPlugin) Transform(m resmap.ResMap) error {
	if len(p.LDelim) <= 0 {
		p.LDelim = "{{"
	}
	if len(p.RDelim) <= 0 {
		p.RDelim = "}}"
	}
	p.fieldSpec = types.FieldSpec{Path: p.Path}
	var data []byte
	var err error

	var env = make(map[string]string)

	valuesFile := p.InputFile
	if data, err = ioutil.ReadFile(valuesFile); err != nil {
		return err
	}

	if p.DataSources != nil {

		for _, envVar := range p.EnvVars {
			if len(envVar.Value) > 0 {
				env[envVar.Name] = envVar.Value
			} else if len(envVar.ValueFromFile) > 0 {
				if data, err = ioutil.ReadFile(envVar.ValueFromFile); err != nil {
					return err
				} else {
					env[envVar.Name] = string(data)
				}
			} else if len(envVar.ValueFromEnv) > 0 {
				if envValue, exists := os.LookupEnv(envVar.ValueFromEnv); err != nil || !exists {
					if err != nil {
						return err
					}
					return fmt.Errorf("environmental variable %v, does not exist", envVar.ValueFromEnv)
				} else {
					env[envVar.Name] = string(envValue)
				}
			} else {
				if envValue, exists := os.LookupEnv(envVar.Name); err != nil || !exists {
					if err != nil {
						return err
					}
					return fmt.Errorf("environmental variable %v, does not exist", envVar.Name)
				} else {
					env[envVar.Name] = string(envValue)
				}
			}
		}

		if p.replaceYaml, err = utils.RunGomplateFromConfig(p.DataSources, p.Pwd, env, string(data), p.logger, p.LDelim, p.RDelim); err != nil {
			p.logger.Errorf("error executing runGomplate() on dataSources: %v, in directory: %v, error: %v\n", p.DataSources, p.Pwd, err)
		}
		// Target
		resources, err := m.Select(*p.Target)
		if err != nil {
			p.logger.Errorf("error selecting resources based on the target selector, error: %v\n", err)
			return err
		}
		for _, r := range resources {
			if err := filtersutil.ApplyToJSON(kio.FilterFunc(func(nodes []*kyaml.RNode) ([]*kyaml.RNode, error) {
				return kio.FilterAll(kyaml.FilterFunc(func(rn *kyaml.RNode) (*kyaml.RNode, error) {
					if err := rn.PipeE(fieldspec.Filter{
						FieldSpec: p.fieldSpec,
						SetValue: func(n *kyaml.RNode) error {
							return p.searchAndReplaceRNode(n)
						},
					}); err != nil {
						return nil, err
					}
					return rn, nil
				})).Filter(nodes)
			}), r); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("error, no data sources defined for gomplate")
}

func (p *GomsertPlugin) searchAndReplaceRNode(node *kyaml.RNode) error {
	var in map[string]interface{}
	if err := yaml.Unmarshal([]byte(p.replaceYaml), &in); err != nil {
		p.logger.Infof("error unmarshalling JSON string after replacements back to interface, error: %v\n", err)
		return err
	}
	if in != nil {
		tempMap := map[string]interface{}{"tmp": in}
		if tempMapRNode, err := utils.NewKyamlRNode(tempMap); err != nil {
			return err
		} else {
			node.SetYNode(tempMapRNode.Field("tmp").Value.YNode())
		}
	}
	return nil
}

func NewGomsertPlugin() resmap.TransformerPlugin {
	return &GomsertPlugin{logger: utils.GetLogger("GomsertPlugin")}
}
