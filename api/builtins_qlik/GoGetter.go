package builtins_qlik

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-getter"

	"sigs.k8s.io/kustomize/api/builtins_qlik/utils"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/ifc"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
)

var goGetterMutex sync.Mutex

// GoGetterPlugin ...
type GoGetterPlugin struct {
	types.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	URL              string `json:"url,omitempty" yaml:"url,omitempty"`
	Pwd              string
	ldr              ifc.Loader
	rf               *resmap.Factory
	logger           *log.Logger
	newldr           ifc.Loader
}

// Config ...
func (p *GoGetterPlugin) Config(h *resmap.PluginHelpers, c []byte) (err error) {
	p.ldr = h.Loader()
	p.rf = h.ResmapFactory()
	p.Pwd = h.Loader().Root()
	return yaml.Unmarshal(c, p)
}

// Generate ...
func (p *GoGetterPlugin) Generate() (resmap.ResMap, error) {

	dir, err := konfig.DefaultAbsPluginHome(filesys.MakeFsOnDisk())
	if err != nil {
		dir = filepath.Join(konfig.HomeDir(), konfig.XdgConfigHomeEnvDefault, konfig.ProgramName, konfig.RelPluginHome)
		p.logger.Printf("No kustomize plugin directory, will create default: %v\n", dir)
	}
	repodir := filepath.Join(dir, "qlik", "v1", "repos")
	dir = filepath.Join(repodir, p.ObjectMeta.Name)
	err = os.MkdirAll(repodir, 0777)
	if err != nil {
		p.logger.Printf("error creating directory: %v, error: %v\n", dir, err)
		return nil, err
	}
	opts := []getter.ClientOption{}
	client := &getter.Client{
		Ctx:     context.TODO(),
		Src:     p.URL,
		Dst:     dir,
		Pwd:     p.Pwd,
		Mode:    getter.ClientModeAny,
		Options: opts,
	}
	goGetterMutex.Lock()
	defer goGetterMutex.Unlock()
	err = client.Get()
	if err != nil {
		p.logger.Printf("Error fetching repository: %v\n", err)
		return nil, err
	}
	currentExe, err := os.Executable()
	if err != nil {
		p.logger.Printf("Unable to get kustomize executable: %v\n", err)
		return nil, err
	}
	cmd := exec.Command(currentExe, "build", dir)
	cmd.Stderr = os.Stderr
	kustomizedYaml, err := cmd.Output()
	if err != nil {
		p.logger.Printf("Error executing kustomize as a child process: %v\n", err)
		return nil, err
	}
	return p.rf.NewResMapFromBytes(kustomizedYaml)
}

// NewGoGetterPlugin ...
func NewGoGetterPlugin() resmap.GeneratorPlugin {
	return &GoGetterPlugin{logger: utils.GetLogger("GoGetterPlugin")}
}