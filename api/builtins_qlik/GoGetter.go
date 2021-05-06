package builtins_qlik

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"syscall"

	version "github.com/hashicorp/go-version"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"sigs.k8s.io/kustomize/api/builtins_qlik/utils"
	yamlv3 "sigs.k8s.io/kustomize/api/builtins_qlik/yaegi"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/ifc"
	"sigs.k8s.io/kustomize/api/konfig"
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

var defaultBranchRegexp = regexp.MustCompile(`\s->\sorigin/(.*)`)

// GoGetterPlugin ...
type GoGetterPlugin struct {
	types.ObjectMeta    `json:"metadata,omitempty" yaml:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	URL                 string   `json:"url,omitempty" yaml:"url,omitempty"`
	Cwd                 string   `json:"cwd,omitempty" yaml:"cwd,omitempty"`
	PreBuildArgs        []string `json:"preBuildArgs,omitempty" yaml:"preBuildArgs,omitempty"`
	PreBuildScript      string   `json:"preBuildScript,omitempty" yaml:"preBuildScript,omitempty"`
	PreBuildScriptFile  string   `json:"preBuildScriptFile,omitempty" yaml:"preBuildScriptFile,omitempty"`
	PostBuildArgs       []string `json:"postBuildArgs,omitempty" yaml:"postBuildArgs,omitempty"`
	PostBuildScript     string   `json:"postBuildScript,omitempty" yaml:"postBuildScript,omitempty"`
	PostBuildScriptFile string   `json:"postBuildScriptFile,omitempty" yaml:"postBuildScriptFile,omitempty"`
	PartialCloneDir     string   `json:"partialCloneDir,omitempty" yaml:"partialCloneDir,omitempty"`
	/* CloneFilter
	   The best filter would likely be the unsupported as of 2.30 "combine:blob:none+tree:0"
	   Therefor a filter must be chosen.

	   For repos with limited branches but many files:
	   - blob:none

	   For repos with large trees and many files (default):
	   - tree:0

	   The following can be used to test (ex. engine)

	   ```
	   time sh -c " \
	     git clone --bare --filter=tree:0 --no-checkout https://github.com/kubernetes/kubernetes kubernetes/.git ; \
	     cd kubernetes ; \
		 git config --local --bool core.bare false ; \
		 git sparse-checkout init --cone ; \
		 git sparse-checkout set manifests ; \
		 git checkout"
	*/
	CloneFilter        string `json:"cloneFilter,omitempty" yaml:"cloneFilter,omitempty"`
	Pwd                string
	ldr                ifc.Loader
	rf                 *resmap.Factory
	logger             *log.Logger
	newldr             ifc.Loader
	executableResolver iExecutableResolverT
	yamlBytes          []byte
}

// Config ...
func (p *GoGetterPlugin) Config(h *resmap.PluginHelpers, c []byte) (err error) {
	p.ldr = h.Loader()
	p.rf = h.ResmapFactory()
	p.Pwd = h.Loader().Root()
	p.yamlBytes = c
	return yaml.Unmarshal(c, p)
}

// Generate ...
func (p *GoGetterPlugin) Generate() (resmap.ResMap, error) {

	if len(p.PartialCloneDir) == 0 {
		p.PartialCloneDir = "manifests"
	}
	if len(p.CloneFilter) == 0 {
		p.CloneFilter = "tree:0"
	}
	dir, err := konfig.DefaultAbsPluginHome(filesys.MakeFsOnDisk())
	if err != nil {
		dir = filepath.Join(konfig.HomeDir(), konfig.XdgConfigHomeEnvDefault, konfig.ProgramName, konfig.RelPluginHome)
		p.logger.Printf("No kustomize plugin directory, will create default: %v\n", dir)
	}

	repodir := filepath.Join(dir, "qlik", "v1", "repos")
	dir = filepath.Join(repodir, p.ObjectMeta.Name)
	if err := os.MkdirAll(repodir, 0777); err != nil {
		p.logger.Printf("error creating directory: %v, error: %v\n", dir, err)
		return nil, err
	}
	// We usually only fetch a branch at a time
	url, err := url.Parse(p.URL)
	if err != nil {
		p.logger.Printf("Bad git URL %v\n", err)
		return nil, err
	}
	url.Scheme = "https"

	if err := p.executeGitGetter(url, dir); err != nil {
		p.logger.Printf("Error fetching repository: %v\n", err)
		return nil, err
	}

	currentExe, err := p.executableResolver.Executable()
	if err != nil {
		p.logger.Printf("Unable to get kustomize executable: %v\n", err)
		return nil, err
	}

	cwd := dir
	if len(p.Cwd) > 0 {
		cwd = filepath.Join(dir, filepath.FromSlash(p.Cwd))
	}
	// Convert to relative path due to kustomize bug with drive letters
	// thinks its a remote ref
	oswd, _ := os.Getwd()
	err = os.Chdir(cwd)
	defer os.Chdir(oswd)
	if err != nil {
		p.logger.Printf("Error: Unable to set working dir %v: %v\n", cwd, err)
		return nil, err
	}

	if len(p.PreBuildScript) > 0 || len(p.PreBuildScriptFile) > 0 {
		var (
			gogetter = interp.Exports{
				"gogetter": map[string]reflect.Value{
					"GetKustomizedYaml": reflect.ValueOf(func() []byte {
						return nil
					}),
					"GetGoGetter": reflect.ValueOf(func() []byte {
						return p.yamlBytes
					}),
				},
			}
		)

		i := interp.New(interp.Options{})

		i.Use(stdlib.Symbols)
		i.Use(yamlv3.Symbols)
		i.Use(gogetter)

		if len(p.PreBuildScript) > 0 {
			_, err = i.Eval(p.PreBuildScript)
		} else {
			var gocode []byte
			gocode, err = ioutil.ReadFile(p.PreBuildScriptFile)
			if err != nil {
				p.logger.Printf("Error loading go file: %v\n", err)
				return nil, err
			}
			_, err = i.Eval(string(gocode))
		}
		if err != nil {
			p.logger.Printf("Go Script Error: %v\n", err)
			return nil, err
		}
		v, err := i.Eval("kust.PreBuild")
		if err != nil {
			p.logger.Printf("Go Script Error: %v\n", err)
			return nil, err
		}
		preBuild := v.Interface().(func([]string) error)
		err = preBuild(p.PreBuildArgs)
		if err != nil {
			p.logger.Printf("Error from pre-Build: %v\n", err)
			return nil, err
		}
	}
	var kustomizedYaml bytes.Buffer
	cmd := exec.Command(currentExe, "build", ".")
	err = p.getRunCommand(cmd, &kustomizedYaml)
	if err != nil {
		p.logger.Printf("Error executing kustomize as a child process: %v\n", err)
		return nil, err
	}
	kustBytes := kustomizedYaml.Bytes()
	if len(p.PostBuildScript) > 0 || len(p.PostBuildScriptFile) > 0 {
		var (
			gogetter = interp.Exports{
				"gogetter": map[string]reflect.Value{
					"GetKustomizedYaml": reflect.ValueOf(func() []byte {
						return kustomizedYaml.Bytes()
					}),
					"GetGoGetter": reflect.ValueOf(func() []byte {
						return p.yamlBytes
					}),
				},
			}
		)

		i := interp.New(interp.Options{})

		i.Use(stdlib.Symbols)
		i.Use(yamlv3.Symbols)
		i.Use(gogetter)

		if err != nil {
			p.logger.Printf("Go Script Error: %v\n", err)
			return nil, err
		}
		if len(p.PostBuildScript) > 0 {
			_, err = i.Eval(p.PostBuildScript)
		} else {
			var gocode []byte
			gocode, err = ioutil.ReadFile(p.PostBuildScriptFile)
			if err != nil {
				p.logger.Printf("Error loading go file: %v\n", err)
				return nil, err
			}
			_, err = i.Eval(string(gocode))
		}
		if err != nil {
			p.logger.Printf("Go Script Error: %v\n", err)
			return nil, err
		}
		v, err := i.Eval("kust.PostBuild")
		if err != nil {
			p.logger.Printf("Go Script Error: %v\n", err)
			return nil, err
		}
		postBuild := v.Interface().(func([]string) (*string, error))
		postBuildRet, err := postBuild(p.PostBuildArgs)
		if err != nil {
			p.logger.Printf("Error from post-Build: %v\n", err)
			return nil, err
		}
		if postBuildRet != nil {
			kustBytes = []byte(*postBuildRet)
		}

	}
	return p.rf.NewResMapFromBytes(kustBytes)
}

// GoGit ...
func (p *GoGetterPlugin) GoGit(u *url.URL, dir string) error {
	var ref string
	q := u.Query()
	if len(q) > 0 {

		ref = q.Get("ref")
		q.Del("ref")

		// Copy the URL
		var newU url.URL = *u
		u = &newU

		u.RawQuery = q.Encode()
	}

	// Clone or update the repository
	_, err := os.Stat(dir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		err = p.update(dir, u, ref)
	} else {
		err = p.clone(dir, u, ref)
	}
	if err != nil {
		return err
	}
	return nil
}
func (p *GoGetterPlugin) findDefaultBranch(dst string) string {
	var stdoutbuf bytes.Buffer
	cmd := exec.Command("git", "branch", "-r", "--points-at", "refs/remotes/origin/HEAD")
	cmd.Dir = dst
	cmd.Stdout = &stdoutbuf
	err := cmd.Run()
	matches := defaultBranchRegexp.FindStringSubmatch(stdoutbuf.String())
	if err != nil || matches == nil {
		return "master"
	}
	return matches[len(matches)-1]
}

func (p *GoGetterPlugin) executeGitGetter(url *url.URL, dir string) error {

	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git must be available and on the PATH")
	}

	if err := p.GoGit(url, dir); err != nil {
		p.logger.Printf("Error executing go-getter: %v\n", err)
		return err
	}

	return nil
}

func (p *GoGetterPlugin) getRunCommand(cmd *exec.Cmd, stdOut *bytes.Buffer) error {
	var buf bytes.Buffer
	if stdOut == nil {
		cmd.Stdout = &buf
	} else {
		cmd.Stdout = stdOut
	}
	cmd.Stderr = &buf

	err := cmd.Run()
	if err == nil {
		return nil
	}
	if exiterr, ok := err.(*exec.ExitError); ok {
		// The program has exited with an exit code != 0
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return fmt.Errorf(
				"%s exited with %d: %s",
				cmd.Path,
				status.ExitStatus(),
				buf.String())
		}
	}

	return fmt.Errorf("error running %s: %s", cmd.Path, buf.String())
}

// NewGoGetterPlugin ...
func NewGoGetterPlugin() resmap.GeneratorPlugin {
	return &GoGetterPlugin{logger: utils.GetLogger("GoGetterPlugin"), executableResolver: &osExecutableResolverT{}}
}

func (p *GoGetterPlugin) clone(dst string, u *url.URL, ref string) error {

	// We need the fast clone with history
	// Cloning the bare repo then doing sparse checkout seems fastest across all versions
	// depth=1 is fast but we cannot use "depth" regardless because we need history for other plugins
	// (and doesn't work with < 2.30)
	// See note above about filters
	var args []string
	if err := p.checkGitVersion("2.25"); err == nil {
		args = []string{"clone", "--bare", "--filter", p.CloneFilter, "--no-checkout"}
	} else {
		return err
	}

	// git clone --bare --filter=tree:0 --no-checkout https://github.com/<org>/repo <dir>/.git
	// git -C <dir> config --local --bool core.bare false
	// git -C <dir> sparse-checkout init --cone
	// git -C <dir> sparse-checkout set <sparse directory>
	// git -C <dir> checkout
	isBranch := false
	if len(ref) != 0 {
		if err := p.getRunCommand(exec.Command("git", "ls-remote", "--exit-code", "--heads", u.String(), ref), nil); err == nil {
			args = append(args, "--branch", ref)
			isBranch = true
		}
	} else {
		isBranch = true
	}
	args = append(args, u.String(), filepath.Join(dst, ".git"))

	if err := p.getRunCommand(exec.Command("git", args...), nil); err != nil {
		p.logger.Printf("error executing git clone: %v\n", err)
		return err
	}

	cmd := exec.Command("git", "config", "--local", "--bool", "core.bare", "false")
	cmd.Dir = dst
	if err := p.getRunCommand(cmd, nil); err != nil {
		p.logger.Printf("error executing git config: %v\n", err)
		return err
	}
	if p.PartialCloneDir != "." {
		cmd = exec.Command("git", "sparse-checkout", "init", "--cone")
		cmd.Dir = dst
		if err := p.getRunCommand(cmd, nil); err != nil {
			p.logger.Printf("error executing git sparse-checkout init: %v\n", err)
			return err
		}
		// hard code for now
		cmd = exec.Command("git", "sparse-checkout", "set", p.PartialCloneDir)
		cmd.Dir = dst
		if err := p.getRunCommand(cmd, nil); err != nil {
			p.logger.Printf("error executing git space-checkout set: %v\n", err)
			return err
		}
	}
	if isBranch {
		cmd = exec.Command("git", "checkout")
	} else {
		cmd = exec.Command("git", "checkout", ref)
	}
	cmd.Dir = dst
	if err := p.getRunCommand(cmd, nil); err != nil {
		p.logger.Printf("git checkout: %v\n", err)
		return err
	}
	return nil
}

func (p *GoGetterPlugin) update(dst string, u *url.URL, ref string) error {
	// Determin current sitution
	var stdoutbuf bytes.Buffer
	var clone = true
	var update = true
	if len(ref) == 0 {
		ref = p.findDefaultBranch(dst)
	}
	// Check if branch/tag/commit id changed (that order)
	cmd := exec.Command("git", "symbolic-ref", "--short", "-q", "HEAD")
	cmd.Dir = dst
	if p.getRunCommand(cmd, &stdoutbuf) != nil {
		cmd := exec.Command("git", "describe", "--tags", "--exact-match")
		cmd.Dir = dst
		if p.getRunCommand(cmd, &stdoutbuf) == nil {
			// This is a tag
			if strings.TrimSuffix(stdoutbuf.String(), "\n") == ref {
				clone = false
				// For Sake of performance let's assume tags are immutable
				update = false
			}
		} else {
			// A Commit ID
			cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
			cmd.Dir = dst
			if p.getRunCommand(cmd, &stdoutbuf) == nil {
				if strings.TrimSuffix(stdoutbuf.String(), "\n") == ref {
					clone = false
					update = false
				}
			}
		}
	} else {
		// This is a branch
		if strings.TrimSuffix(stdoutbuf.String(), "\n") == ref {
			clone = false
			// Check if we need to pull
			cmd := exec.Command("git", "rev-parse", "@")
			cmd.Dir = dst

			if p.getRunCommand(cmd, &stdoutbuf) == nil {
				localRef := strings.TrimSuffix(stdoutbuf.String(), "\n")
				cmd = exec.Command("git", "rev-parse", ref)
				cmd.Dir = dst
				if p.getRunCommand(cmd, &stdoutbuf) == nil {
					if strings.TrimSuffix(stdoutbuf.String(), "\n") == localRef {
						update = false
					}
				}
			}
		}
	}
	if clone {
		// reclone
		os.RemoveAll(dst)
		return p.clone(dst, u, ref)
	}
	if update {
		cmd = exec.Command("git", "pull", "--ff-only", "--tags", "origin", ref)
		cmd.Dir = dst
		if p.getRunCommand(cmd, nil) != nil {
			// reclone
			os.RemoveAll(dst)
			return p.clone(dst, u, ref)
		}
	}
	return nil
}

func (p *GoGetterPlugin) checkGitVersion(min string) error {
	want, err := version.NewVersion(min)
	if err != nil {
		return err
	}

	out, err := exec.Command("git", "version").Output()
	if err != nil {
		return err
	}

	fields := strings.Fields(string(out))
	if len(fields) < 3 {
		return fmt.Errorf("Unexpected 'git version' output: %q", string(out))
	}
	v := fields[2]
	if runtime.GOOS == "windows" && strings.Contains(v, ".windows.") {
		v = v[:strings.Index(v, ".windows.")]
	}

	have, err := version.NewVersion(v)
	if err != nil {
		return err
	}

	if have.LessThan(want) {
		return fmt.Errorf("Required git version = %s, have %s", want, have)
	}

	return nil
}
