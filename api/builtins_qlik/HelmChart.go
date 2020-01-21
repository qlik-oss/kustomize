package builtins_qlik

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
    "time"

	"github.com/gofrs/flock"
    "github.com/pkg/errors"

	"sigs.k8s.io/kustomize/api/builtins_qlik/utils"
	"sigs.k8s.io/kustomize/api/ifc"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/strvals"
)

type HelmChartPlugin struct {
	ChartName        string                 `json:"chartName,omitempty" yaml:"chartName,omitempty"`
	ChartHome        string                 `json:"chartHome,omitempty" yaml:"chartHome,omitempty"`
	TmpChartHome     string                 `json:"tmpChartHome,omitempty" yaml:"tmpChartHome,omitempty"`
	ChartVersion     string                 `json:"chartVersion,omitempty" yaml:"chartVersion,omitempty"`
	ChartRepo        string                 `json:"chartRepo,omitempty" yaml:"chartRepo,omitempty"`
	ChartRepoName    string                 `json:"chartRepoName,omitempty" yaml:"chartRepoName,omitempty"`
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
	hash             string
}

var (
    settings *cli.EnvSettings
    helmDir  = filepath.Join("helm", "repository")
)

func (p *HelmChartPlugin) Config(h *resmap.PluginHelpers, c []byte) (err error) {
	p.ldr = h.Loader()
	p.rf = h.ResmapFactory()
	chartHash := sha256.New()
	chartHash.Write(c)
	p.hash = hex.EncodeToString(chartHash.Sum(nil))
	return yaml.Unmarshal(c, p)
}

func (p *HelmChartPlugin) Generate() (resmap.ResMap, error) {

	// make temp directory
	dir, err := ioutil.TempDir("", "tempRoot")
	if err != nil {
		p.logger.Printf("error creating temporary directory: %v\n", err)
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
		if err = os.MkdirAll(directory, os.ModePerm); err != nil && !os.IsExist(err) {
			return nil, err
		}
		if _, err = os.Create(fmt.Sprintf("%s/repositories.yaml",directory)); err != nil {
			return nil, err
		}
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

	hashfolder := filepath.Join(p.ChartHome, ".plugincache")
	hashfile := filepath.Join(hashfolder, p.hash)
	repositoryFile := filepath.Join(p.ChartHome, "repositories.yaml")
	cacheHome := filepath.Join(p.ChartHome, ".chartcache")
	var templatedYaml []byte
	os.Setenv("HELM_NAMESPACE", p.ReleaseNamespace)
	os.Setenv("XDG_CONFIG_HOME", p.ChartHome)
	os.Setenv("XDG_CACHE_HOME", cacheHome)
	settings = cli.New()
	settings.RepositoryConfig = repositoryFile

	if _, err = os.Stat(hashfile); err != nil {
		if os.IsNotExist(err) {
			err = p.fetchHelm()
			if err != nil {
				p.logger.Printf("error fetching repo info, error: %v\n", err)
				return nil, err
			}
			if err = repoUpdate(); err != nil {
				return nil, err
			}
	
			images, err := getImages(p.ReleaseName, p.ChartRepoName, p.ChartName, p.ChartVersion, p.ExtraArgs); 
			fmt.Println(images)
			 if err != nil {
				return nil, err
				}
			for _, image := range images {
				fmt.Println(image)
			}
			err = p.deleteRequirements()
			if err != nil {
				p.logger.Printf("error executing deleteRequirements() for dir: %v, error: %v\n", p.ChartHome, err)
				return nil, err
			}

			templatedYaml, err = p.templateHelm()
			if err != nil {
				p.logger.Printf("error executing templateHelm(), error: %v\n", err)
				return nil, err
			}
			os.MkdirAll(hashfolder, os.ModePerm)
			if err = ioutil.WriteFile(hashfile, templatedYaml, 0644); err != nil {
				p.logger.Printf("error writing kustomization yaml to file: %v, error: %v\n", hashfile, err)
			}
		} else {
			return nil, err
		}
	} else {
		templatedYaml, err = ioutil.ReadFile(hashfile)
		if err != nil {
			p.logger.Printf("error reading file: %v, error: %v\n", hashfile, err)
			return nil, err
		}
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
	dir := filepath.Dir(settings.RepositoryConfig)
	d, err := os.Open(dir)
	if err != nil {
		p.logger.Printf("error opening directory %v, error: %v\n", dir, err)
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
				filePath := dir + "/" + file.Name()
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

// RepoAdd adds repo with given name and url
func  (p *HelmChartPlugin) fetchHelm() error {
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
        //fmt.Printf("repository name (%s) already exists\n", p.ChartName)
        return nil
    }
    if r, err = repo.NewChartRepository(&c, getter.All(settings)); err != nil {
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

// RepoUpdate updates charts for all helm repos
func repoUpdate() error {
	var (
		repoFile = settings.RepositoryConfig
		err      error
		f        *repo.File
		r        *repo.ChartRepository
		repos    []*repo.ChartRepository
		cfg      *repo.Entry
		wg       sync.WaitGroup
	)
	f, err = repo.LoadFile(repoFile)
	if os.IsNotExist(errors.Cause(err)) || len(f.Repositories) == 0 {
		return errors.New("no repositories found. You must add one before updating")
	}

	for _, cfg = range f.Repositories {
		r, err = repo.NewChartRepository(cfg, getter.All(settings))
		if err != nil {
			return err
		}
		repos = append(repos, r)
	}
	

	// fmt.Printf("Downloading helm chart index ...\n")
	for _, r = range repos {
		wg.Add(1)
		go func(re *repo.ChartRepository) {
			defer wg.Done()
			if _, err = re.DownloadIndexFile(); err != nil {
				 fmt.Printf("...Unable to get an update from the %q chart repository (%s):\n\t%s\n", re.Config.Name, re.Config.URL, err)
			}
		}(r)
	}
	wg.Wait()
	return nil
}

func (p *HelmChartPlugin) templateHelm() ([]byte, error) {

	file, err := ioutil.TempFile("", "yaml")
	if err != nil {
		p.logger.Printf("error creating temp file, error: %v\n", err)
		return nil, err
	}
	valuesYaml, err := yaml.Marshal(p.Values)
	if err != nil {
		p.logger.Printf("error marshalling values to yaml, error: %v\n", err)
		return nil, err
	}
	_, err = file.Write(valuesYaml)
	if err != nil {
		p.logger.Printf("error writing yaml to file: %v, error: %v\n", file.Name(), err)
		return nil, err
	}
	chart := p.ChartHome
	if len(p.SubChart) > 0 {
		chart = p.ChartHome + "/charts/" + p.SubChart
	}

	// build helm flags
	home := fmt.Sprintf("--home=%s", p.HelmHome)
	values := fmt.Sprintf("--values=%s", file.Name())
	name := fmt.Sprintf("--name=%s", p.ReleaseName)
	nameSpace := fmt.Sprintf("--namespace=%s", p.ReleaseNamespace)

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

func debug(format string, v ...interface{}) {
	//format = fmt.Sprintf("[debug] %s\n", format)
	//log.Output(2, fmt.Sprintf(format, v...))
}

func getImages(name, repo, chart, version, args string) ([]string, error) {

	var (
		actionConfig   = new(action.Configuration)
		client         = action.NewInstall(actionConfig)
		err            error
		validate       bool
		rel            *release.Release
		m              *release.Hook
		manifests      bytes.Buffer
		splitManifests map[string]string
		manifest       string
		any            = map[string]interface{}{}
		images         = make([]string, 0)
		k              string
		v              interface{}
	)

	if err = actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), debug); err != nil {
		return images, err
	}

	client.DryRun = true
	client.ReleaseName = name
	client.Replace = true // Skip the name check
	client.ClientOnly = !validate
	if len(version) > 0 {
		client.Version = version
	}
	//	client.APIVersions = chartutil.VersionSet(extraAPIs)
	if rel, err = runInstall(name, repo, chart, args, client); err != nil {
		return images, err
	}

	fmt.Fprintln(&manifests, strings.TrimSpace(rel.Manifest))
	for _, m = range rel.Hooks {
		fmt.Fprintf(&manifests, "---\n# Source: %s\n%s\n", m.Path, m.Manifest)
	}
	// fmt.Printf("Building Image List ...\n")
	splitManifests = releaseutil.SplitManifests(manifests.String())
	for _, manifest = range splitManifests {
		if err = yaml.Unmarshal([]byte(manifest), &any); err != nil {
			return images, err
		}
		for k, v = range any {
			fmt.Println(k,v)
			images = searchImages(k, v, images)
		}
	}
	// fmt.Printf("Done Image List\n")
	return uniqueNonEmptyElementsOf(images), nil
}

func runInstall(name, repo, chartName, sets string, client *action.Install) (*release.Release, error) {
	var (
		valueOpts             = &values.Options{}
		vals                  map[string]interface{}
		p                     getter.Providers
		cp                    string
		validInstallableChart bool
		err                   error
		chartRequested        *chart.Chart
		req                   []*chart.Dependency
		man                   *downloader.Manager
	)

	debug("Original chart version: %q", client.Version)
	if client.Version == "" && client.Devel {
		debug("setting version to >0.0.0-0")
		client.Version = ">0.0.0-0"
	}

	if cp, err = client.ChartPathOptions.LocateChart(fmt.Sprintf("%s/%s", repo, chartName), settings); err != nil {
		return nil, err
	}

	debug("CHART PATH: %s\n", cp)

	p = getter.All(settings)
	if vals, err = valueOpts.MergeValues(p); err != nil {
		return nil, err
	}

	// Add args
	if err = strvals.ParseInto(sets, vals); err != nil {
		return nil, errors.Wrap(err, "failed parsing --set data")
	}
	// Check chart dependencies to make sure all are present in /charts
	// fmt.Printf("Downloading helm chart ...\n")
	if chartRequested, err = loader.Load(cp); err != nil {
		return nil, err
	}

	validInstallableChart, err = isChartInstallable(chartRequested)
	if !validInstallableChart {
		return nil, err
	}

	if req = chartRequested.Metadata.Dependencies; req != nil {
		// If CheckDependencies returns an error, we have unfulfilled dependencies.
		// As of Helm 2.4.0, this is treated as a stopping condition:
		// https://github.com/helm/helm/issues/2209
		if err = action.CheckDependencies(chartRequested, req); err != nil {
			if client.DependencyUpdate {
				man = &downloader.Manager{
					Out:              os.Stdout,
					ChartPath:        cp,
					Keyring:          client.ChartPathOptions.Keyring,
					SkipUpdate:       false,
					Getters:          p,
					RepositoryConfig: settings.RepositoryConfig,
					RepositoryCache:  settings.RepositoryCache,
				}
				if err = man.Update(); err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	}

	client.Namespace = settings.Namespace()
	return client.Run(chartRequested, vals)
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

func isChartInstallable(ch *chart.Chart) (bool, error) {
	switch ch.Metadata.Type {
	case "", "application":
		return true, nil
	}
	return false, errors.Errorf("%s charts are not installable", ch.Metadata.Type)
}

func searchImages(key string, value interface{}, images []string) []string {
	var (
		submap     map[interface{}]interface{}
		stringlist []interface{}
		k, v       interface{}
		ok         bool
		i          int
	)

	submap, ok = value.(map[interface{}]interface{})
	if ok {
		for k, v = range submap {
			images = searchImages(k.(string), v, images)
		}
		return images
	}
	stringlist, ok = value.([]interface{})
	if ok {
		images = searchImages("size", len(stringlist), images)
		for i, v = range stringlist {
			images = searchImages(fmt.Sprintf("%d", i), v, images)
		}
		return images
	}

	if key == "image" {
		images = append(images, fmt.Sprintf("%v", value))
	}
	return images
}

func uniqueNonEmptyElementsOf(s []string) []string {
	var (
		unique = make(map[string]bool, len(s))
		us     = make([]string, len(unique))
		elem   string
	)
	for _, elem = range s {
		if len(elem) != 0 {
			if !unique[elem] {
				us = append(us, elem)
				unique[elem] = true
			}
		}
	}
	sort.Strings(us)
	return us
}