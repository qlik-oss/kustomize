package builtins_qlik

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
	"sigs.k8s.io/kustomize/api/builtins_qlik/utils"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/types"

	"github.com/pkg/errors"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/loader"
	"sigs.k8s.io/kustomize/api/resmap"
	valtest_test "sigs.k8s.io/kustomize/api/testutils/valtest"
)

func Test_ImageGitTag_getGitDescribeForHead(t *testing.T) {
	type tcT struct {
		name     string
		dir      string
		validate func(t *testing.T, tag string)
	}

	testCases := []*tcT{
		func() *tcT {
			tmpDir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			semverTag := "v1.0.0"
			subDir, shortGitRef, err := setupGitDirWithSubdir(tmpDir, []string{}, []string{semverTag})
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			return &tcT{
				name: "semver tag before head",
				dir:  subDir,
				validate: func(t *testing.T, tag string) {
					expected := fmt.Sprintf("%v-1-g%v", strings.TrimPrefix(semverTag, "v"), shortGitRef)
					if tag != expected {
						t.Fatalf("expected: %v, but got: %v\n", expected, tag)
					}
					_ = os.RemoveAll(tmpDir)
				},
			}
		}(),
		func() *tcT {
			tmpDir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			semverTag := "v1.0.0"
			subDir, _, err := setupGitDirWithSubdir(tmpDir, []string{"foobar", semverTag}, []string{"foo-tag"})
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			return &tcT{
				name: "semver tag on head",
				dir:  subDir,
				validate: func(t *testing.T, tag string) {
					if tag != strings.TrimPrefix(semverTag, "v") {
						t.Fatalf("expected: %v, but got: %v\n", semverTag, tag)
					}
					_ = os.RemoveAll(tmpDir)
				},
			}
		}(),
		func() *tcT {
			tmpDir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			semverTag := "v1.20.30"
			subDir, _, err := setupGitDirWithSubdir(tmpDir, []string{"foo", "bar", semverTag, "v4.0.0-beta"}, []string{"v1.2.1"})
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			return &tcT{
				name: "first semver tag",
				dir:  subDir,
				validate: func(t *testing.T, tag string) {
					if tag != strings.TrimPrefix(semverTag, "v") {
						t.Fatalf("expected: %v, but got: %v\n", semverTag, tag)
					}
					_ = os.RemoveAll(tmpDir)
				},
			}
		}(),
		func() *tcT {
			tmpDir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			semverTag := "0.0.0"
			subDir, shortGitRef, err := setupGitDirWithSubdir(tmpDir, []string{}, []string{})
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			return &tcT{
				name: "no tags",
				dir:  subDir,
				validate: func(t *testing.T, tag string) {
					expected := fmt.Sprintf("%v-0-g%v", strings.TrimPrefix(semverTag, "v"), shortGitRef)
					if tag != expected {
						t.Fatalf("expected: %v, but got: %v\n", expected, tag)
					}
					_ = os.RemoveAll(tmpDir)
				},
			}
		}(),
		func() *tcT {
			tmpDir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			return &tcT{
				name: "non git folder",
				dir:  tmpDir,
				validate: func(t *testing.T, tag string) {
					expected := "9.9.9"
					if tag != expected {
						t.Fatalf("expected: %v, but got: %v\n", expected, tag)
					}
					_ = os.RemoveAll(tmpDir)
				},
			}
		}(),
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tag, err := utils.GetGitDescribeForHead(testCase.dir, "9.9.9", nil)
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}
			testCase.validate(t, tag)
		})
	}
}

func execCmd(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	buf := &bytes.Buffer{}
	cmd.Stderr = buf
	if output, err := cmd.Output(); err != nil {
		return output, fmt.Errorf("error: %w, stderr: %s", err, string(buf.Bytes()))
	} else {
		return output, nil
	}
}

func writeAndCommitFile(dir, fileName, fileContent, commitMessage string) error {
	if err := ioutil.WriteFile(filepath.Join(dir, fileName), []byte(fileContent), os.ModePerm); err != nil {
		return errors.Wrapf(err, "error writing file: %v", filepath.Join(dir, fileName))
	} else if _, err := execCmd(dir, "git", "add", "."); err != nil {
		return errors.Wrap(err, "error executing git add")
	} else if _, err := execCmd(dir, "git", "commit", "-m", commitMessage); err != nil {
		return errors.Wrap(err, "error executing git commit")
	}
	return nil
}

func setupGitDirWithSubdir(tmpDir string, headTags []string, intermediateTags []string) (dir string, shortGitRef string, err error) {
	if _, err := execCmd(tmpDir, "git", "init"); err != nil {
		return "", "", err
	} else if _, err := execCmd(tmpDir, "git", "config", "user.name", "ci"); err != nil {
		return "", "", err
	} else if _, err := execCmd(tmpDir, "git", "config", "user.email", "ci@qlik.com"); err != nil {
		return "", "", err
	}

	barDir := filepath.Join(tmpDir, "bar-dir")
	if err := writeAndCommitFile(tmpDir, "foo.txt", "foo", "committing foo.txt"); err != nil {
		return "", "", err
	} else {
		for _, tag := range intermediateTags {
			if _, err := execCmd(tmpDir, "git", "tag", tag); err != nil {
				return "", "", err
			}
		}
	}

	if err := os.MkdirAll(barDir, os.ModePerm); err != nil {
		return "", "", err
	} else if err := writeAndCommitFile(barDir, "bar.txt", "bar", "committing bar.txt"); err != nil {
		return "", "", err
	} else {
		for _, tag := range headTags {
			if _, err := execCmd(tmpDir, "git", "tag", tag); err != nil {
				return "", "", err
			}
		}
	}

	if shortGitRefBytes, err := execCmd(tmpDir, "git", "rev-parse", "--short=7", "HEAD"); err != nil {
		return "", "", err
	} else {
		return barDir, string(bytes.TrimSpace(shortGitRefBytes)), nil
	}
}

func Test_ImageGitTag_Transform(t *testing.T) {
	type tcT struct {
		name                 string
		pluginConfig         string
		pluginInputResources string
		loaderRootDir        string
		checkAssertions      func(*testing.T, resmap.ResMap)
	}

	pluginInputResources := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy1
spec:
  template:
    spec:
      containers:
      - image: nginx:1.7.9
        name: nginx-tagged
      - image: nginx:latest
        name: nginx-latest
      - image: foobar:1
        name: replaced-with-digest
      - image: postgres:1.8.0
        name: postgresdb
      initContainers:
      - image: nginx
        name: nginx-notag
      - image: nginx@sha256:111111111111111111
        name: nginx-sha256
      - image: alpine:1.8.0
        name: init-alpine
`
	outputResourcesTemplate := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy1
spec:
  template:
    spec:
      containers:
      - image: nginx:%v
        name: nginx-tagged
      - image: nginx:%v
        name: nginx-latest
      - image: foobar:1
        name: replaced-with-digest
      - image: postgres:1.8.0
        name: postgresdb
      initContainers:
      - image: nginx:%v
        name: nginx-notag
      - image: nginx:%v
        name: nginx-sha256
      - image: alpine:1.8.0
        name: init-alpine
`
	testCases := []*tcT{
		func() *tcT {
			tmpDir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			subDir, hash, err := setupGitDirWithSubdir(tmpDir, []string{}, []string{"foo"})
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			return &tcT{
				name: "no semver tags",
				pluginConfig: `
apiVersion: qlik.com/v1
kind: GitImageTag
metadata:
  name: notImportantHere
images:
  - name: nginx
`,
				pluginInputResources: pluginInputResources,
				loaderRootDir:        subDir,
				checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
					expectedTag := fmt.Sprintf("v0.0.0-0-g%v", hash)
					version := strings.TrimPrefix(expectedTag, "v")
					expected := fmt.Sprintf(outputResourcesTemplate, version, version, version, version)

					actual, err := resMap.AsYaml()
					if err != nil {
						t.Fatalf("Err: %v", err)
					} else if string(actual) != expected {
						t.Fatalf("expected:\n%v\n, but got:\n%v", expected, string(actual))
					}
					_ = os.RemoveAll(tmpDir)
				},
			}
		}(),
		func() *tcT {
			tmpDir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			semverTag := "v0.0.1"
			subDir, hash, err := setupGitDirWithSubdir(tmpDir, []string{}, []string{"foo", semverTag, "bar"})
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			return &tcT{
				name: "semver tag before head",
				pluginConfig: `
apiVersion: qlik.com/v1
kind: GitImageTag
metadata:
  name: notImportantHere
images:
  - name: nginx
`,
				pluginInputResources: pluginInputResources,
				loaderRootDir:        subDir,
				checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
					version := strings.TrimPrefix(fmt.Sprintf("%v-1-g%v", semverTag, hash), "v")
					expected := fmt.Sprintf(outputResourcesTemplate, version, version, version, version)

					actual, err := resMap.AsYaml()
					if err != nil {
						t.Fatalf("Err: %v", err)
					} else if string(actual) != expected {
						t.Fatalf("expected:\n%v\n, but got:\n%v", expected, string(actual))
					}
					_ = os.RemoveAll(tmpDir)
				},
			}
		}(),
		func() *tcT {
			tmpDir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			semverTag := "v1.0.2"
			subDir, _, err := setupGitDirWithSubdir(tmpDir, []string{semverTag}, []string{"foo", "v0.0.1", "bar"})
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			return &tcT{
				name: "semver tag on head",
				pluginConfig: `
apiVersion: qlik.com/v1
kind: GitImageTag
metadata:
  name: notImportantHere
images:
  - name: nginx
`,
				pluginInputResources: pluginInputResources,
				loaderRootDir:        subDir,
				checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
					version := strings.TrimPrefix(semverTag, "v")
					expected := fmt.Sprintf(outputResourcesTemplate, version, version, version, version)

					actual, err := resMap.AsYaml()
					if err != nil {
						t.Fatalf("Err: %v", err)
					} else if string(actual) != expected {
						t.Fatalf("expected: [%v], but got: [%v]", expected, string(actual))
					}
					_ = os.RemoveAll(tmpDir)
				},
			}
		}(),
		func() *tcT {
			tmpDir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			highestSemverTag := "v5.0.0"
			subDir, _, err := setupGitDirWithSubdir(tmpDir, []string{highestSemverTag, "bar"}, []string{"foo", "v0.0.1"})
			if err != nil {
				t.Fatalf("unexpected error: %v\n", err)
			}

			return &tcT{
				name: "highest semver git tag chosen and used for multiple images",
				pluginConfig: `
apiVersion: qlik.com/v1
kind: GitImageTag
metadata:
  name: notImportantHere
images:
  - name: nginx
  - name: postgres
  - name: alpine
`,
				pluginInputResources: pluginInputResources,
				loaderRootDir:        subDir,
				checkAssertions: func(t *testing.T, resMap resmap.ResMap) {
					version := strings.TrimPrefix(highestSemverTag, "v")
					expected := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy1
spec:
  template:
    spec:
      containers:
      - image: nginx:%v
        name: nginx-tagged
      - image: nginx:%v
        name: nginx-latest
      - image: foobar:1
        name: replaced-with-digest
      - image: postgres:%v
        name: postgresdb
      initContainers:
      - image: nginx:%v
        name: nginx-notag
      - image: nginx:%v
        name: nginx-sha256
      - image: alpine:%v
        name: init-alpine
`, version, version, version, version, version, version)

					actual, err := resMap.AsYaml()
					if err != nil {
						t.Fatalf("Err: %v", err)
					} else if string(actual) != expected {
						t.Fatalf("expected:\n%v\n, but got:\n%v", expected, string(actual))
					}
					_ = os.RemoveAll(tmpDir)
				},
			}
		}(),
	}
	baseLogger, _ := zap.NewDevelopment()
	logger := baseLogger.Sugar()
	defer logger.Sync()
	plugin := GitImageTagPlugin{logger: logger}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			p := provider.NewDefaultDepProvider()
			resourceFactory := resmap.NewFactory(p.GetResourceFactory())
			resMap, err := resourceFactory.NewResMapFromBytes([]byte(testCase.pluginInputResources))
			if err != nil {
				t.Fatalf("Err: %v", err)
			}

			ldr, err := loader.NewLoader(loader.RestrictionRootOnly, testCase.loaderRootDir, filesys.MakeFsOnDisk())
			if err != nil {
				t.Fatalf("Err: %v", err)
			}

			h := resmap.NewPluginHelpers(ldr, valtest_test.MakeHappyMapValidator(t), resourceFactory, types.DisabledPluginConfig())
			if err := plugin.Config(h, []byte(testCase.pluginConfig)); err != nil {
				t.Fatalf("Err: %v", err)
			}

			if err := plugin.Transform(resMap); err != nil {
				t.Fatalf("Err: %v", err)
			}

			testCase.checkAssertions(t, resMap)
		})
	}
}
