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
		Name          string  `json:"name" 	yaml:"name"`
		ValueFromEnv  *string `json:"valueFromEnv,omitempty" 	yaml:"valueFromEnv,omitempty"`
		ValueFromFile *string `json:"valueFromFile,omitempty" 	yaml:"valueFromFile,omitempty"`
		Value         *string `json:"value,omitempty" 	yaml:"value,omitempty"`
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
			if _, keyexists := env[envVar.Name]; !keyexists {
				if envVar.Value != nil {
					p.logger.Infof("environmental variable %v set from value", envVar.Name)
					env[envVar.Name] = *envVar.Value
				} else if envVar.ValueFromFile != nil {
					if data, err = ioutil.ReadFile(*envVar.ValueFromFile); err == nil {
						p.logger.Infof("environmental variable %v set from File %v", envVar.Name, envVar.ValueFromFile)
						stringData := string(data)
						env[envVar.Name] = stringData
					} else {
						p.logger.Warnf("environmental variable %v, unable to read file %v, %v", envVar.Name, envVar.ValueFromFile, err)
					}
				} else if envVar.ValueFromEnv != nil {
					if envValue, exists := os.LookupEnv(*envVar.ValueFromEnv); exists {
						p.logger.Infof("environmental variable %v set from env var %v", envVar.Name, envVar.ValueFromEnv)
						stringData := string(envValue)
						env[envVar.Name] = stringData
					} else {
						p.logger.Warnf("environmental variable %v, unable to read env var %v", envVar.Name, envVar.ValueFromEnv)
					}
				} else {
					if envValue, exists := os.LookupEnv(envVar.Name); exists {
						p.logger.Infof("environmental variable %v set", envVar.Name)
						stringData := string(envValue)
						env[envVar.Name] = stringData
					} else {
						p.logger.Warnf("environmental variable %v does not exist", envVar.Name)
					}
				}
			}
		}
		// Find Errors
		for _, envVar := range p.EnvVars {
			if _, exists := env[envVar.Name]; !exists {
				return fmt.Errorf("unbound env var %v", envVar.Name)
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
