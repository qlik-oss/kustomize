package builtins_qlik

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-getter"
	"sigs.k8s.io/kustomize/api/builtins_qlik/utils"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/ifc"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/loader"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
)

type iExecutableResolverT interface {
	Executable() (string, error)
}

type osExecutableResolverT struct {
}

func (r *osExecutableResolverT) Executable() (string, error) {
	return os.Executable()
}

// GoGetterPlugin ...
type GoGetterPlugin struct {
	types.ObjectMeta   `json:"metadata,omitempty" yaml:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	URL                string `json:"url,omitempty" yaml:"url,omitempty"`
	Pwd                string
	ldr                ifc.Loader
	rf                 *resmap.Factory
	logger             *log.Logger
	newldr             ifc.Loader
	executableResolver iExecutableResolverT
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
	loader.GoGetterMutex.Lock()
	// In case it was an update (slighty inefficient but easy)
	// Second time is not a full clone
	// go getter doesn't do --tags so we can "fake it"
	if _, err := os.Stat(dir); err != nil {
		// First Time
		if os.IsNotExist(err) {
			err = client.Get()
		}
	}
	// read the whole file at once
	b, err := ioutil.ReadFile(filepath.Join(dir, ".git", "config"))
	if err != nil {
		p.logger.Printf("error git config file: %v, error: %v\n", filepath.Join(dir, ".git", "config"), err)
		return nil, err
	}
	if !strings.Contains(string(b), "+refs/tags/*:refs/tags/*") {
		cmd := exec.Command("git", "config", "-f", filepath.Join(dir, ".git", "config"), "--add", "remote.origin.fetch", "+refs/tags/*:refs/tags/*")
		cmd.Stderr = os.Stderr
		cmd.Run()
	}
	err = client.Get()
	// Since we are checking for existance we should not need
	// cmd := exec.Command("git", "config", "-f", filepath.Join(dir, ".git", "config"), "--unset", "remote.origin.fetch", `\+refs\/tags\/\*\:refs\/tags\/\*`)
	// cmd.Stderr = os.Stderr
	// cmd.Run()
	loader.GoGetterMutex.Unlock()

	if err != nil {
		p.logger.Printf("Error fetching repository: %v\n", err)
		return nil, err
	}
	currentExe, err := p.executableResolver.Executable()
	if err != nil {
		p.logger.Printf("Unable to get kustomize executable: %v\n", err)
		return nil, err
	}

	// Convert to relative path due to kustomize bug with drive letters
	// thinks its a remote ref
	oswd, _ := os.Getwd()
	err = os.Chdir(dir)
	if err != nil {
		p.logger.Printf("Error: Unable to set working dir %v: %v\n", dir, err)
		return nil, err
	}
	cmd := exec.Command(currentExe, "build", ".")
	cmd.Stderr = os.Stderr
	kustomizedYaml, err := cmd.Output()
	if err != nil {
		p.logger.Printf("Error executing kustomize as a child process: %v\n", err)
		return nil, err
	}
	os.Chdir(oswd)
	return p.rf.NewResMapFromBytes(kustomizedYaml)
}

// NewGoGetterPlugin ...
func NewGoGetterPlugin() resmap.GeneratorPlugin {
	return &GoGetterPlugin{logger: utils.GetLogger("GoGetterPlugin"), executableResolver: &osExecutableResolverT{}}
}
