/*
Old kustomize kustruct functions that do proper fieldref handling
https://github.com/kubernetes-sigs/kustomize/blob/v3.3.1/k8sdeps/kunstruct/helper.go
Sections are
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"fmt"
	"strconv"
	"strings"

	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

func appendNonEmpty(section []string, field string) []string {
	if len(field) != 0 {
		return append(section, field)
	}
	return section
}

func parseFields(path string) ([]string, error) {
	result := []string{}
	if !strings.Contains(path, "[") {
		section := strings.Split(path, ".")
		result = append(result, section...)
		return result, nil
	}

	start := 0
	insideParentheses := false
	for i, c := range path {
		switch c {
		case '.':
			if !insideParentheses {
				result = appendNonEmpty(result, path[start:i])
				start = i + 1
			}
		case '[':
			if !insideParentheses {
				result = appendNonEmpty(result, path[start:i])
				start = i + 1
				insideParentheses = true
			} else {
				return nil, fmt.Errorf("nested parentheses are not allowed: %s", path)
			}
		case ']':
			if insideParentheses {
				result = appendNonEmpty(result, path[start:i])
				start = i + 1
				insideParentheses = false
			} else {
				return nil, fmt.Errorf("invalid field path %s", path)
			}
		}
	}
	if start < len(path)-1 {
		result = appendNonEmpty(result, path[start:])
	}
	for i, f := range result {
		if strings.HasPrefix(f, "\"") || strings.HasPrefix(f, "'") {
			result[i] = strings.Trim(f, "\"'")
		}
	}
	return result, nil
}

func GetFieldValue(rn *kyaml.RNode, path string) (interface{}, error) {
	fields, err := parseFields(path)
	if err != nil {
		return nil, err
	}
	rn, err = rn.Pipe(kyaml.Lookup(fields...))
	if err != nil {
		return nil, err
	}
	if rn == nil {
		return nil, kyaml.NoFieldError{path}
	}
	yn := rn.YNode()

	// If this is an alias node, resolve it
	if yn.Kind == kyaml.AliasNode {
		yn = yn.Alias
	}

	// Return value as map for DocumentNode and MappingNode kinds
	if yn.Kind == kyaml.DocumentNode || yn.Kind == kyaml.MappingNode {
		var result map[string]interface{}
		if err := yn.Decode(&result); err != nil {
			return nil, err
		}
		return result, err
	}

	// Return value as slice for SequenceNode kind
	if yn.Kind == kyaml.SequenceNode {
		var result []interface{}
		if err := yn.Decode(&result); err != nil {
			return nil, err
		}
		return result, nil
	}
	if yn.Kind != kyaml.ScalarNode {
		return nil, fmt.Errorf("expected ScalarNode, got Kind=%d", yn.Kind)
	}

	switch yn.Tag {
	case kyaml.NodeTagString:
		return yn.Value, nil
	case kyaml.NodeTagInt:
		return strconv.Atoi(yn.Value)
	case kyaml.NodeTagFloat:
		return strconv.ParseFloat(yn.Value, 64)
	case kyaml.NodeTagBool:
		return strconv.ParseBool(yn.Value)
	default:
		// Possibly this should be an error or log.
		return yn.Value, nil
	}
}
