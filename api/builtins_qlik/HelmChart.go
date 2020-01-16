package builtins_qlik

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/pkg/errors"

	"sigs.k8s.io/kustomize/api/builtins_qlik/utils"
	"sigs.k8s.io/kustomize/api/ifc"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/yaml"


	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"

)

type HelmChartPlugin struct {
	ChartName        string                 `json:"chartName,omitempty" yaml:"chartName,omitempty"`
	ChartHome        string                 `json:"chartHome,omitempty" yaml:"chartHome,omitempty"`
	TmpChartHome     string                 `json:"tmpChartHome,omitempty" yaml:"tmpChartHome,omitempty"`
	ChartVersion     string                 `json:"chartVersion,omitempty" yaml:"chartVersion,omitempty"`
	ChartRepo        string                 `json:"chartRepo,omitempty" yaml:"chartRepo,omitempty"`
	ValuesFrom       string                 `json:"valuesFrom,omitempty" yaml:"valuesFrom,omitempty"`
	Values           map[string]interface{} `json:"values,omitempty" yaml:"values,omitempty"`
	HelmHome         string                 `json:"helmHome,omitempty" yaml:"helmHome,omitempty"`
	HelmBin          string                 `json:"helmBin,omitempty" yaml:"helmBin,omitempty"`
	ReleaseName      string                 `json:"releaseName,omitempty" yaml:"releaseName,omitempty"`
	ReleaseNamespace string                 `json:"releaseNamespace,omitempty" yaml:"releaseNamespace,omitempty"`
	ExtraArgs        string                 `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	ChartPatches     string                 `json:"chartPatches,omitempty" yaml:"chartPatches,omitempty"`
	SubChart         string                 `json:"subChart,omitempty" yaml:"subChart,omitempty"`
	ChartVersionExp  string
	ldr              ifc.Loader
	rf               *resmap.Factory
	logger           *log.Logger
}


var (
	settings *cli.EnvSettings
	helmDir  = filepath.Join("helm", "repository")
)

func (p *HelmChartPlugin) Config(h *resmap.PluginHelpers, c []byte) (err error) {
	p.ldr = h.Loader()
	p.rf = h.ResmapFactory()
	return yaml.Unmarshal(c, p)
}

func (p *HelmChartPlugin) Generate() (resmap.ResMap, error) {

	// make temp directory
	dir, err := ioutil.TempDir("", "tempRoot")
	if err != nil {
		p.logger.Printf("error creating temporaty directory: %v\n", err)
		return nil, err
	}
	dir = path.Join(dir, "../")

	if p.HelmHome == "" {
		// make home for helm stuff
		directory := fmt.Sprintf("%s/%s", dir, "dotHelm")
		p.HelmHome = directory
	}

	if p.ChartHome == "" && p.TmpChartHome != "" {
		p.ChartHome = path.Join(os.TempDir(), p.TmpChartHome)
	}

	if p.ChartHome == "" {
		// make home for chart stuff
		directory := fmt.Sprintf("%s/%s", dir, p.ChartName)
		p.ChartHome = directory
	}

	if p.HelmBin == "" {
		p.HelmBin = "helm"
	}

	if p.ChartVersion != "" {
		p.ChartVersionExp = fmt.Sprintf("--version=%s", p.ChartVersion)
	} else {
		p.ChartVersionExp = ""
	}

	if p.ChartRepo == "" {
		p.ChartRepo = "https://kubernetes-charts.storage.googleapis.com"
	}

	if p.ReleaseName == "" {
		p.ReleaseName = "release-name"
	}

	if p.ReleaseNamespace == "" {
		p.ReleaseName = "default"
	}

	err = p.initHelm()
	settings = cli.New()
	if err != nil {
		p.logger.Printf("error executing initHelm(), error: %v\n", err)
		return nil, err
	}
	if _, err := os.Stat(p.ChartHome); os.IsNotExist(err) {
		err = p.helmFetch()
		if err != nil {
			p.logger.Printf("error executing fetchHelm(), error: %v\n", err)
			return nil, err
		}
	} else if err != nil {
		p.logger.Printf("error executing stat on file: %v, error: %v\n", p.ChartHome, err)
	}

	err = p.deleteRequirements()
	if err != nil {
		p.logger.Printf("error executing deleteRequirements() for dir: %v, error: %v\n", p.ChartHome, err)
		return nil, err
	}

	templatedYaml, err := p.templateHelm()
	if err != nil {
		p.logger.Printf("error executing templateHelm(), error: %v\n", err)
		return nil, err
	}

	if len(p.ChartPatches) > 0 {
		templatedYaml, err = p.applyPatches(templatedYaml)
		if err != nil {
			p.logger.Printf("error executing applyPatches(), error: %v\n", err)
			return nil, err
		}
	}

	return p.rf.NewResMapFromBytes(templatedYaml)
}

func (p *HelmChartPlugin) deleteRequirements() error {
	repoFile := settings.RepositoryCache
	d, err := os.Open(repoFile)
	if err != nil {
		p.logger.Printf("error opening directory %v, error: %v\n", repoFile, err)
		return err
	}
	defer d.Close()

	files, err := d.Readdir(-1)
	if err != nil {
		p.logger.Printf("error listing directory %v, error: %v\n", d.Name(), err)
		return err
	}

	for _, file := range files {
		if file.Mode().IsRegular() {
			ext := filepath.Ext(file.Name())
			name := file.Name()[0 : len(file.Name())-len(ext)]
			if name == "requirements" {
				fmt.Println("here")
				filePath := repoFile + "/" + file.Name()
				err := os.Remove(filePath)
				if err != nil {
					p.logger.Printf("error deleting the requirements file %v, error: %v\n", filePath, err)
					return err
				}
			}
		}
	}

	return nil
}

func (p *HelmChartPlugin) initHelm() error {
	// build helm flags
	home := fmt.Sprintf("--home=%s", p.HelmHome)
	helmCmd := exec.Command(p.HelmBin, "init", home, "--client-only")
	err := helmCmd.Run()
	if err != nil {
		p.logger.Printf("error executing command: %v with args: %v, error: %v\n", helmCmd.Path, helmCmd.Args, err)
		return err
	}
	return nil
}

// RepoAdd adds repo with given name and url
func  (p *HelmChartPlugin) helmFetch() error {
	var (
		repoFile    = settings.RepositoryConfig
		fileLock    *flock.Flock
		lockContext context.Context
		cancel      context.CancelFunc
		locked      bool
		err         error
		b           []byte
		f           repo.File
		r           *repo.ChartRepository
		c           = repo.Entry{
			Name: p.ChartName,
			URL:  p.ChartRepo,
		}
	)

	if err = os.MkdirAll(filepath.Dir(repoFile), os.ModePerm); err != nil && !os.IsExist(err) {
		return err
	}

	// Acquire a file lock for process synchronization
	fileLock = flock.New(strings.Replace(repoFile, filepath.Ext(repoFile), ".lock", 1))

	lockContext, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	locked, err = fileLock.TryLockContext(lockContext, time.Second)
	if err == nil && locked {
		defer fileLock.Unlock()
	}
	if err != nil {
		return err
	}
	if b, err = ioutil.ReadFile(repoFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err = yaml.Unmarshal(b, &f); err != nil {
		return err
	}

	if f.Has(p.ChartName) {
		//fmt.Printf("repository name (%s) already exists\n", name)
		return nil
	}
	if r, err = repo.NewChartRepository(&c, getter.All(settings)); err != nil {
		fmt.Println(repoFile)
		return err
	}

	if _, err = r.DownloadIndexFile(); err != nil {
		return errors.Wrapf(err, "looks like %q is not a valid chart repository or cannot be reached", p.ChartRepo)
	}

	f.Update(&c)

	if err = f.WriteFile(repoFile, 0644); err != nil {
		return err
	}
	return nil
}

func (p *HelmChartPlugin) templateHelm() ([]byte, error) {

	valuesYaml, err := yaml.Marshal(p.Values)
	if err != nil {
		p.logger.Printf("error marshalling values to yaml, error: %v\n", err)
		return nil, err
	}
	file, err := ioutil.TempFile("", "yaml")
	if err != nil {
		p.logger.Printf("error creating temp file, error: %v\n", err)
		return nil, err
	}
	_, err = file.Write(valuesYaml)
	if err != nil {
		p.logger.Printf("error writing yaml to file: %v, error: %v\n", file.Name(), err)
		return nil, err
	}

	// build helm flags
	home := fmt.Sprintf("--home=%s", p.HelmHome)
	values := fmt.Sprintf("--values=%s", file.Name())
	name := fmt.Sprintf("--name=%s", p.ReleaseName)
	nameSpace := fmt.Sprintf("--namespace=%s", p.ReleaseNamespace)
	chart := p.ChartHome
	if len(p.SubChart) > 0 {
		chart = p.ChartHome + "/charts/" + p.SubChart
	}
	helmCmd := exec.Command("helm", "template", home, values, name, nameSpace, chart)

	if len(p.ExtraArgs) > 0 && p.ExtraArgs != "null" {
		helmCmd.Args = append(helmCmd.Args, p.ExtraArgs)
	}

	if len(p.ValuesFrom) > 0 && p.ValuesFrom != "null" {
		templatedValues := fmt.Sprintf("--values=%s", p.ValuesFrom)
		helmCmd.Args = append(helmCmd.Args, templatedValues)
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	helmCmd.Stdout = &out
	helmCmd.Stderr = &stderr
	err = helmCmd.Run()
	if err != nil {
		p.logger.Printf("error executing command: %v with args: %v, error: %v, stderr: %v\n", helmCmd.Path, helmCmd.Args, err, stderr.String())
		return nil, err
	}
	return out.Bytes(), nil
}

func (p *HelmChartPlugin) applyPatches(templatedHelm []byte) ([]byte, error) {
	// get the patches
	path := filepath.Join(p.ChartHome + "/" + p.ChartPatches + "/kustomization.yaml")
	origYamlBytes, err := ioutil.ReadFile(path)
	if err != nil {
		p.logger.Printf("error reading file: %v, error: %v\n", path, err)
		return nil, err
	}

	var originalYamlMap map[string]interface{}

	if err := yaml.Unmarshal(origYamlBytes, &originalYamlMap); err != nil {
		p.logger.Printf("error unmarshalling kustomization yaml from file: %v, error: %v\n", path, err)
	}

	// helmoutput file for kustomize build
	helpOutputPath := p.ChartHome + "/" + p.ChartPatches + "/helmoutput.yaml"
	f, err := os.Create(helpOutputPath)
	if err != nil {
		p.logger.Printf("error creating helm output file: %v, error: %v\n", helpOutputPath, err)
		return nil, err
	}

	_, err = f.Write(templatedHelm)
	if err != nil {
		p.logger.Printf("error writing to helm output file: %v, error: %v\n", helpOutputPath, err)
		return nil, err
	}

	kustomizeYaml, err := ioutil.ReadFile(path)
	if err != nil {
		p.logger.Printf("error reading file: %v, error: %v\n", path, err)
		return nil, err
	}

	var kustomizeYamlMap map[string]interface{}
	if err := yaml.Unmarshal(kustomizeYaml, &kustomizeYamlMap); err != nil {
		p.logger.Printf("error unmarshalling kustomization yaml from file: %v, error: %v\n", path, err)
	}

	delete(kustomizeYamlMap, "resources")

	kustomizeYamlMap["resources"] = []string{"helmoutput.yaml"}

	yamlM, err := yaml.Marshal(kustomizeYamlMap)
	if err != nil {
		p.logger.Printf("error marshalling kustomization yaml map, error: %v\n", err)
		return nil, err
	}

	if err := ioutil.WriteFile(path, yamlM, 0644); err != nil {
		p.logger.Printf("error writing kustomization yaml to file: %v, error: %v\n", path, err)
	}

	// kustomize build
	templatedHelm, err = p.buildPatches()
	if err != nil {
		p.logger.Printf("error executing buildPatches(), error: %v\n", err)
		return nil, err
	}

	return templatedHelm, nil
}

func (p *HelmChartPlugin) buildPatches() ([]byte, error) {
	path := filepath.Join(p.ChartHome + "/" + p.ChartPatches)
	kustomizeCmd := exec.Command("kustomize", "build", path)

	var out bytes.Buffer
	kustomizeCmd.Stdout = &out

	err := kustomizeCmd.Run()
	if err != nil {
		p.logger.Printf("error executing command: %v with args: %v, error: %v\n", kustomizeCmd.Path, kustomizeCmd.Args, err)
		return nil, err
	}
	return out.Bytes(), nil
}

func NewHelmChartPlugin() resmap.GeneratorPlugin {
	return &HelmChartPlugin{logger: utils.GetLogger("HelmChartPlugin")}
}
