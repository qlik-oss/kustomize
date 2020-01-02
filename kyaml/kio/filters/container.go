// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package filters

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// ContainerFilter filters Resources using a container image.
// The container must start a process that reads the list of
// input Resources from stdin, reads the Configuration from the env
// API_CONFIG, and writes the filtered Resources to stdout.
// If there is a error or validation failure, the process must exit
// non-zero.
// The full set of environment variables from the parent process
// are passed to the container.
type ContainerFilter struct {

	// Image is the container image to use to create a container.
	Image string `yaml:"image,omitempty"`

	// Network is the container network to use.
	Network string `yaml:"network,omitempty"`

	// StorageMounts is a list of storage options that the container will have mounted.
	StorageMounts []StorageMount

	// Config is the API configuration for the container and passed through the
	// API_CONFIG env var to the container.
	// Typically a Kubernetes style Resource Config.
	Config *yaml.RNode `yaml:"config,omitempty"`

	// args may be specified by tests to override how a container is spawned
	args []string

	checkInput func(string)
}

// StorageMount represents a container's mounted storage option(s)
type StorageMount struct {
	// Type of mount e.g. bind mount, local volume, etc.
	MountType string

	// Source for the storage to be mounted.
	// For named volumes, this is the name of the volume.
	// For anonymous volumes, this field is omitted (empty string).
	// For bind mounts, this is the path to the file or directory on the host.
	Src string

	// The path where the file or directory is mounted in the container.
	DstPath string
}

func (s *StorageMount) String() string {
	return fmt.Sprintf("type=%s,src=%s,dst=%s:ro", s.MountType, s.Src, s.DstPath)
}

// GrepFilter implements kio.GrepFilter
func (c *ContainerFilter) Filter(input []*yaml.RNode) ([]*yaml.RNode, error) {
	// get the command to filter the Resources
	cmd, err := c.getCommand()
	if err != nil {
		return nil, err
	}

	in := &bytes.Buffer{}
	out := &bytes.Buffer{}

	// write the input
	err = kio.ByteWriter{
		WrappingAPIVersion: kio.ResourceListAPIVersion,
		WrappingKind:       kio.ResourceListKind,
		Writer:             in, KeepReaderAnnotations: true, FunctionConfig: c.Config}.Write(input)
	if err != nil {
		return nil, err
	}

	// capture the command stdout for the return value
	r := &kio.ByteReader{Reader: out}

	// do the filtering
	if c.checkInput != nil {
		c.checkInput(in.String())
	}
	cmd.Stdin = in
	cmd.Stdout = out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return r.Read()
}

// getArgs returns the command + args to run to spawn the container
func (c *ContainerFilter) getArgs() []string {
	// run the container using docker.  this is simpler than using the docker
	// libraries, and ensures things like auth work the same as if the container
	// was run from the cli.

	network := "none"
	if c.Network != "" {
		network = c.Network
	}

	args := []string{"docker", "run",
		"--rm",                                              // delete the container afterward
		"-i", "-a", "STDIN", "-a", "STDOUT", "-a", "STDERR", // attach stdin, stdout, stderr

		// added security options
		"--network", network,
		"--user", "nobody", // run as nobody
		// don't make fs readonly because things like heredoc rely on writing tmp files
		"--security-opt=no-new-privileges", // don't allow the user to escalate privileges
	}

	// TODO(joncwong): Allow StorageMount fields to have default values.
	for _, storageMount := range c.StorageMounts {
		args = append(args, "--mount", storageMount.String())
	}

	// export the local environment vars to the container
	for _, pair := range os.Environ() {
		tokens := strings.Split(pair, "=")
		if tokens[0] == "" {
			continue
		}
		args = append(args, "-e", tokens[0])
	}
	return append(args, c.Image)
}

// getCommand returns a command which will apply the GrepFilter using the container image
func (c *ContainerFilter) getCommand() (*exec.Cmd, error) {
	// encode the filter command API configuration
	cfg := &bytes.Buffer{}
	if err := func() error {
		e := yaml.NewEncoder(cfg)
		defer e.Close()
		// make it fit on a single line
		c.Config.YNode().Style = yaml.FlowStyle
		return e.Encode(c.Config.YNode())
	}(); err != nil {
		return nil, err
	}

	if len(c.args) == 0 {
		c.args = c.getArgs()
	}

	cmd := exec.Command(c.args[0], c.args[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	// set stderr for err messaging
	return cmd, nil
}

// IsReconcilerFilter filters Resources based on whether or not they are Reconciler Resource.
// Resources with an apiVersion starting with '*.gcr.io', 'gcr.io' or 'docker.io' are considered
// Reconciler Resources.
type IsReconcilerFilter struct {
	// ExcludeReconcilers if set to true, then Reconcilers will be excluded -- e.g.
	// Resources with a reconcile container through the apiVersion (gcr.io prefix) or
	// through the annotations
	ExcludeReconcilers bool `yaml:"excludeReconcilers,omitempty"`

	// IncludeNonReconcilers if set to true, the NonReconciler will be included.
	IncludeNonReconcilers bool `yaml:"includeNonReconcilers,omitempty"`
}

// Filter implements kio.Filter
func (c *IsReconcilerFilter) Filter(inputs []*yaml.RNode) ([]*yaml.RNode, error) {
	var out []*yaml.RNode
	for i := range inputs {
		img, _ := GetContainerName(inputs[i])
		isContainerResource := img != ""
		if isContainerResource && !c.ExcludeReconcilers {
			out = append(out, inputs[i])
		}
		if !isContainerResource && c.IncludeNonReconcilers {
			out = append(out, inputs[i])
		}
	}
	return out, nil
}

// GetContainerName returns the container image for an API if one exists
func GetContainerName(n *yaml.RNode) (string, string) {
	meta, _ := n.GetMeta()

	// path to the function, this will be mounted into the container
	path := meta.Annotations[kioutil.PathAnnotation]

	functionAnnotation := meta.Annotations["config.k8s.io/function"]
	if functionAnnotation != "" {
		annotationContent, _ := yaml.Parse(functionAnnotation)
		image, _ := annotationContent.Pipe(yaml.Lookup("container", "image"))
		return image.YNode().Value, path
	}

	container := meta.Annotations["config.kubernetes.io/container"]
	if container != "" {
		return container, path
	}

	image, err := n.Pipe(yaml.Lookup("metadata", "configFn", "container", "image"))
	if err != nil || yaml.IsMissingOrNull(image) {
		return "", path
	}
	return image.YNode().Value, path
}
