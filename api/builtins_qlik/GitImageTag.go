package builtins_qlik

import (
	"sigs.k8s.io/kustomize/api/builtins"
	"sigs.k8s.io/kustomize/api/builtins_qlik/utils"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
	"go.uber.org/zap"
)

type GitImageTagPluginImage struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

type GitImageTagPlugin struct {
	Images []GitImageTagPluginImage `json:"images,omitempty" yaml:"images,omitempty"`
	pwd    string
	logger *zap.SugaredLogger
}

func (p *GitImageTagPlugin) Config(h *resmap.PluginHelpers, c []byte) (err error) {
	p.Images = make([]GitImageTagPluginImage, 0)
	p.pwd = h.Loader().Root()
	return yaml.Unmarshal(c, p)
}

func (p *GitImageTagPlugin) Transform(m resmap.ResMap) error {
	if gitVersionTag, err := utils.GetGitDescribeForHead(p.pwd, "", p.logger); err != nil {
		return err
	} else {
		for _, image := range p.Images {
			if err := transformForImage(&m, &image, gitVersionTag); err != nil {
				return err
			}
		}
	}
	return nil
}

func transformForImage(m *resmap.ResMap, gitImageTagPluginImage *GitImageTagPluginImage, newTag string) error {
	if newTag != "" {
		imageTagTransformerPlugin := builtins.ImageTagTransformerPlugin{
			ImageTag: types.Image{
				Name:   gitImageTagPluginImage.Name,
				NewTag: newTag,
			},
		}
		if err := imageTagTransformerPlugin.Transform(*m); err != nil {
			return err
		}
	}
	return nil
}

func NewGitImageTagPlugin() resmap.TransformerPlugin {
	return &GitImageTagPlugin{logger: utils.GetLogger("GitImageTagPlugin")}
}
