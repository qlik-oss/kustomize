// Copyright 2021 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

//go:generate pluginator
package main

import (
	"fmt"

	"sigs.k8s.io/kustomize/api/filters/replacement"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"sigs.k8s.io/yaml"
)

// Replace values in targets with values from a source
type plugin struct {
	ReplacementList []types.ReplacementField `json:"replacements,omitempty" yaml:"replacements,omitempty"`
	Replacements    []types.Replacement      `json:"omitempty" yaml:"omitempty"`
}

//noinspection GoUnusedGlobalVariable
var KustomizePlugin plugin

func (p *plugin) Config(
	h *resmap.PluginHelpers, c []byte) (err error) {
	p.ReplacementList = []types.ReplacementField{}
	if err := yaml.Unmarshal(c, p); err != nil {
		return err
	}

	for _, r := range p.ReplacementList {
		if r.Path != "" && (r.Source != nil || len(r.Targets) != 0) {
			return fmt.Errorf("cannot specify both path and inline replacement")
		}
		if r.Path != "" {
			// load the replacement from the path
			content, err := h.Loader().Load(r.Path)
			if err != nil {
				return err
			}
			repl := types.Replacement{}
			if err := yaml.Unmarshal(content, &repl); err != nil {
				return err
			}
			p.Replacements = append(p.Replacements, repl)
		} else {
			// replacement information is already loaded
			p.Replacements = append(p.Replacements, r.Replacement)
		}
	}
	return nil
}

func (p *plugin) Transform(m resmap.ResMap) (err error) {
	var nodes []*kyaml.RNode
	for _, r := range m.Resources() {
		nodes = append(nodes, r.Node())
	}
	_, err = replacement.Filter{
		Replacements: p.Replacements,
	}.Filter(nodes)
	return err
}
