package builtins_qlik

import (
	"io/ioutil"
	"reflect"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"sigs.k8s.io/kustomize/api/builtins_qlik/utils"
	"sigs.k8s.io/kustomize/api/builtins_qlik/yaegi/yamlv3"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
	"go.uber.org/zap"
)

// YaegiPlugin ...
type YaegiPlugin struct {
	types.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	BuildArgs        []string `json:"buildArgs,omitempty" yaml:"buildArgs,omitempty"`
	BuildScript      string   `json:"buildScript,omitempty" yaml:"buildScript,omitempty"`
	BuildScriptFile  string   `json:"buildScriptFile,omitempty" yaml:"buildScriptFile,omitempty"`
	rf               *resmap.Factory
	logger           *zap.SugaredLogger
	yamlBytes        []byte
}

// Config ...
func (p *YaegiPlugin) Config(h *resmap.PluginHelpers, c []byte) (err error) {
	p.rf = h.ResmapFactory()
	p.yamlBytes = c
	return yaml.Unmarshal(c, p)
}

func (p *YaegiPlugin) Transform(m resmap.ResMap) error {
	var byteArray [][]byte
	var err error
	var resources = m.Resources()
	for _, r := range resources {
		yamlByte, err := r.AsYAML()
		if err != nil {
			p.logger.Errorf("Go Yaml Error: %v\n", err)
			return err
		}
		byteArray = append(byteArray, yamlByte)
	}
	var (
		Yaegi = interp.Exports{
			"yaegi": map[string]reflect.Value{
				"GetResources": reflect.ValueOf(func() [][]byte {
					return byteArray
				}),
				"GetPlugin": reflect.ValueOf(func() []byte {
					return p.yamlBytes
				}),
			},
		}
	)

	i := interp.New(interp.Options{})
	i.Use(stdlib.Symbols)
	i.Use(yamlv3.Symbols)
	i.Use(Yaegi)

	if len(p.BuildScript) > 0 {
		_, err = i.Eval(p.BuildScript)
	} else {
		var gocode []byte
		gocode, err = ioutil.ReadFile(p.BuildScriptFile)
		if err != nil {
			p.logger.Errorf("Error loading go file: %v\n", err)
			return err
		}
		_, err = i.Eval(string(gocode))
	}
	if err != nil {
		p.logger.Errorf("Go Script Error: %v\n", err)
		return err
	}
	v, err := i.Eval("kust.Transform")

	if err != nil {
		p.logger.Errorf("Go Script Error: %v\n", err)
		return err
	}
	transform := v.Interface().(func([]string) (*[][]byte, error))
	transformRet, err := transform(p.BuildArgs)
	if err != nil {
		p.logger.Errorf("Error from post-Build: %v\n", err)
		return err
	}
	if transformRet != nil {
		kustBytes := [][]byte(*transformRet)
		for _, r := range kustBytes {
			res, err := p.rf.RF().FromBytes(r)
			if err != nil {
				p.logger.Errorf("error unmarshalling resource from bytes: %v\n", err)
				return err
			}
			origres, _ := m.GetById(res.CurId())
			if origres == nil {
				m.Append(res)
			} else {
				if jsonBytes, err := res.MarshalJSON(); err != nil {
					return err
				} else if err := origres.UnmarshalJSON(jsonBytes); err != nil {
					return err
				}
			}
		}
	}

	return nil

}
func (p *YaegiPlugin) Generate() (resmap.ResMap, error) {

	var err error
	var (
		Yaegi = interp.Exports{
			"yaegi": map[string]reflect.Value{
				"GetResources": reflect.ValueOf(func() [][]byte {
					return nil
				}),
				"GetPlugin": reflect.ValueOf(func() []byte {
					return p.yamlBytes
				}),
			},
		}
	)

	i := interp.New(interp.Options{})

	i.Use(stdlib.Symbols)
	i.Use(yamlv3.Symbols)
	i.Use(Yaegi)

	if len(p.BuildScript) > 0 {
		_, err = i.Eval(p.BuildScript)
	} else {
		var gocode []byte
		gocode, err = ioutil.ReadFile(p.BuildScriptFile)
		if err != nil {
			p.logger.Errorf("Error loading go file: %v\n", err)
			return nil, err
		}
		_, err = i.Eval(string(gocode))
	}
	if err != nil {
		p.logger.Errorf("Go Script Error: %v\n", err)
		return nil, err
	}
	v, err := i.Eval("kust.Generate")
	if err != nil {
		p.logger.Errorf("Go Script Error: %v\n", err)
		return nil, err
	}
	generate := v.Interface().(func([]string) (*[][]byte, error))
	generateRet, err := generate(p.BuildArgs)
	if err != nil {
		p.logger.Errorf("Error from post-Build: %v\n", err)
		return nil, err
	}
	var retVal []*resource.Resource
	if generateRet != nil {
		kustBytes := [][]byte(*generateRet)
		for _, r := range kustBytes {
			res, err := p.rf.RF().FromBytes(r)
			if err != nil {
				p.logger.Errorf("error unmarshalling resource from bytes: %v\n", err)
				return nil, err
			}
			retVal = append(retVal, res)
		}
	}
	return p.rf.FromResourceSlice(retVal), nil
}

func NewYaegiTransformerPlugin() resmap.TransformerPlugin {
	return &YaegiPlugin{logger: utils.GetLogger("YaegiTransformerPlugin")}
}

func NewYaegiGeneratorPlugin() resmap.GeneratorPlugin {
	return &YaegiPlugin{logger: utils.GetLogger("YaegiGeneratorPlugin")}
}
