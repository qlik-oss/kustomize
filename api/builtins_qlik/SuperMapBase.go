package builtins_qlik

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"sigs.k8s.io/kustomize/kyaml/filtersutil"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"

	"go.uber.org/zap"
	"sigs.k8s.io/kustomize/api/ifc"
	"sigs.k8s.io/kustomize/api/internal/accumulator"
	"sigs.k8s.io/kustomize/api/internal/plugins/builtinconfig"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/api/types"
)

type IDecorator interface {
	GetLogger() *zap.SugaredLogger
	GetName() string
	GetNamespace() string
	SetNamespace(namespace string)
	GetType() string
	GetConfigData() map[string]interface{}
	ShouldBase64EncodeConfigData() bool
	GetDisableNameSuffixHash() bool
	Generate() (resmap.ResMap, error)
}

type SuperMapPluginBase struct {
	AssumeTargetWillExist bool   `json:"assumeTargetWillExist,omitempty" yaml:"assumeTargetWillExist,omitempty"`
	Prefix                string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	Rf                    *resmap.Factory
	Hasher                ifc.KustHasher
	Decorator             IDecorator
	Configurations        []string `json:"configurations,omitempty" yaml:"configurations,omitempty"`
	tConfig               *builtinconfig.TransformerConfig
}

func NewBase(rf *resmap.Factory, decorator IDecorator) SuperMapPluginBase {
	return SuperMapPluginBase{
		AssumeTargetWillExist: true,
		Prefix:                "",
		Rf:                    rf,
		Decorator:             decorator,
		Hasher:                rf.RF().Hasher(),
		Configurations:        make([]string, 0),
		tConfig:               nil,
	}
}

func (b *SuperMapPluginBase) SetupTransformerConfig(ldr ifc.Loader) error {
	b.tConfig = &builtinconfig.TransformerConfig{}
	tCustomConfig, err := builtinconfig.MakeTransformerConfig(ldr, b.Configurations)
	if err != nil {
		b.Decorator.GetLogger().Info("error making transformer config, error: %v\n", err)
		return err
	}
	b.tConfig, err = b.tConfig.Merge(tCustomConfig)
	if err != nil {
		b.Decorator.GetLogger().Info("error merging transformer config, error: %v\n", err)
		return err
	}
	return nil
}

func (b *SuperMapPluginBase) Transform(m resmap.ResMap) error {
	resource := b.find(b.Decorator.GetName(), b.Decorator.GetType(), m)
	if resource != nil {
		return b.executeBasicTransform(resource, m)
	} else if b.AssumeTargetWillExist && !b.Decorator.GetDisableNameSuffixHash() {
		return b.executeAssumeWillExistTransform(m)
	} else {
		b.Decorator.GetLogger().Info("NOT executing anything because resource: %v is NOT in the input stream and AssumeTargetWillExist: %v, disableNameSuffixHash: %v\n", b.Decorator.GetName(), b.AssumeTargetWillExist, b.Decorator.GetDisableNameSuffixHash())
	}
	return nil
}

func (b *SuperMapPluginBase) executeAssumeWillExistTransform(m resmap.ResMap) error {
	b.Decorator.GetLogger().Info("executeAssumeWillExistTransform() for imaginary resource: %v\n", b.Decorator.GetName())

	if b.Decorator.GetNamespace() == "" {
		if anyExistingResource := m.GetByIndex(0); anyExistingResource != nil && anyExistingResource.GetNamespace() != "" {
			b.Decorator.SetNamespace(anyExistingResource.GetNamespace())
		}
	}

	generateResourceMap, err := b.Decorator.Generate()
	if err != nil {
		b.Decorator.GetLogger().Info("error generating temp resource: %v, error: %v\n", b.Decorator.GetName(), err)
		return err
	}
	tempResource := b.find(b.Decorator.GetName(), b.Decorator.GetType(), generateResourceMap)
	if tempResource == nil {
		err := fmt.Errorf("error locating generated temp resource: %v", b.Decorator.GetName())
		b.Decorator.GetLogger().Info("%v\n", err)
		return err
	}

	err = m.Append(tempResource)
	if err != nil {
		b.Decorator.GetLogger().Info("error appending temp resource: %v to the resource map, error: %v\n", b.Decorator.GetName(), err)
		return err
	}

	resourceName := b.Decorator.GetName()
	if len(b.Prefix) > 0 {
		resourceName = fmt.Sprintf("%s%s", b.Prefix, resourceName)
	}
	tempResource.StorePreviousId()
	tempResource.SetName(resourceName)

	nameWithHash, err := b.generateNameWithHash(tempResource)
	if err != nil {
		b.Decorator.GetLogger().Info("error hashing resource: %v, error: %v\n", resourceName, err)
		return err
	}
	tempResource.StorePreviousId()
	tempResource.SetName(nameWithHash)

	err = b.executeNameReferencesTransformer(m)
	if err != nil {
		b.Decorator.GetLogger().Info("error executing nameReferenceTransformer.Transform(): %v\n", err)
		return err
	}
	err = m.Remove(tempResource.CurId())
	if err != nil {
		b.Decorator.GetLogger().Info("error removing temp resource: %v from the resource map, error: %v\n", b.Decorator.GetName(), err)
		return err
	}
	return nil
}

func (b *SuperMapPluginBase) executeBasicTransform(resource *resource.Resource, m resmap.ResMap) error {
	b.Decorator.GetLogger().Info("executeBasicTransform() for resource: %v...\n", resource)

	if err := b.appendData(resource, b.Decorator.GetConfigData(), false); err != nil {
		b.Decorator.GetLogger().Info("error appending data to resource: %v, error: %v\n", b.Decorator.GetName(), err)
		return err
	}

	if !b.Decorator.GetDisableNameSuffixHash() {
		if err := m.Remove(resource.CurId()); err != nil {
			b.Decorator.GetLogger().Info("error removing original resource on name change: %v\n", err)
			return err
		}
		if rmap, err := resource.Map(); err != nil {
			b.Decorator.GetLogger().Info("error getting resource.Map(): %v\n", err)
			return err
		} else {
			newResource := b.Rf.RF().FromMapAndOption(rmap, &types.GeneratorArgs{Behavior: "replace"})
			if err := m.Append(newResource); err != nil {
				b.Decorator.GetLogger().Info("error re-adding resource on name change: %v\n", err)
				return err
			}
			b.Decorator.GetLogger().Info("resource should have hashing enabled: %v\n", newResource)
		}
	}
	return nil
}

func (b *SuperMapPluginBase) executeNameReferencesTransformer(m resmap.ResMap) error {
	ac := accumulator.MakeEmptyAccumulator()
	if err := ac.AppendAll(m); err != nil {
		return err
	} else if err := ac.MergeConfig(b.tConfig); err != nil {
		return err
	} else if err := ac.FixBackReferences(); err != nil {
		return err
	}
	return nil
}

func (b *SuperMapPluginBase) find(name string, resourceType string, m resmap.ResMap) *resource.Resource {
	for _, res := range m.Resources() {
		if res.GetKind() == resourceType && res.GetName() == b.Decorator.GetName() {
			return res
		}
	}
	return nil
}

func (b *SuperMapPluginBase) generateNameWithHash(res *resource.Resource) (string, error) {
	hash, err := res.Hash(b.Hasher)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s", res.GetName(), hash), nil
}

func (b *SuperMapPluginBase) encodeBase64(s string) string {
	const lineLen = 70
	encLen := base64.StdEncoding.EncodedLen(len(s))
	lines := encLen/lineLen + 1
	buf := make([]byte, encLen*2+lines)
	in := buf[0:encLen]
	out := buf[encLen:]
	base64.StdEncoding.Encode(in, []byte(s))
	k := 0
	for i := 0; i < len(in); i += lineLen {
		j := i + lineLen
		if j > len(in) {
			j = len(in)
		}
		k += copy(out[k:], in[i:j])
		if lines > 1 {
			out[k] = '\n'
			k++
		}
	}
	return string(out[:k])
}

func (b *SuperMapPluginBase) appendData(res *resource.Resource, data map[string]interface{}, straightCopy bool) error {
	if err := filtersutil.ApplyToJSON(kio.FilterFunc(func(nodes []*kyaml.RNode) ([]*kyaml.RNode, error) {
		return kio.FilterAll(kyaml.FilterFunc(func(rn *kyaml.RNode) (*kyaml.RNode, error) {
			if dataRn, err := rn.Pipe(kyaml.FieldMatcher{Name: "data"}); err != nil {
				return nil, err
			} else {
				dataRnMap := make(map[string]interface{})

				if dataRn != nil {
					if jsonBytes, err := dataRn.MarshalJSON(); err != nil {
						return nil, err
					} else if err := json.Unmarshal(jsonBytes, &dataRnMap); err != nil {
						return nil, err
					}
				} else {
					dataRn = &kyaml.RNode{}
				}

				for k, v := range data {
					var val interface{}
					val = v
					if _, ok := v.(string); ok {
						if !straightCopy && b.Decorator.ShouldBase64EncodeConfigData() {
							val = b.encodeBase64(v.(string))
						}
					}
					dataRnMap[k] = val
				}

				if newJsonBytes, err := json.Marshal(dataRnMap); err != nil {
					return nil, err
				} else if err := dataRn.UnmarshalJSON(newJsonBytes); err != nil {
					return nil, err
				} else if err := rn.PipeE(kyaml.FieldSetter{Name: "data", Value: dataRn}); err != nil {
					return nil, err
				}
			}
			return rn, nil
		})).Filter(nodes)
	}), res); err != nil {
		return err
	}
	return nil
}
