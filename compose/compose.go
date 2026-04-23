package compose

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type File struct {
	Services map[string]Service `yaml:"services"`
}

type Service struct {
	Image       string            `yaml:"image"`
	Build       *Build            `yaml:"build"`
	Ports       []string          `yaml:"ports"`
	Environment map[string]string `yaml:"environment"`
	Volumes     []string          `yaml:"volumes"`
	DependsOn   DependsOn         `yaml:"depends_on"`
	Restart     string            `yaml:"restart"`
	Command     string            `yaml:"command"`
}

// DependsOn handles both list and map forms of depends_on:
//
//	depends_on: [db, cache]
//	depends_on:
//	  db:
//	    condition: service_healthy
type DependsOn []string

func (d *DependsOn) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.SequenceNode:
		var list []string
		if err := value.Decode(&list); err != nil {
			return err
		}
		*d = list
	case yaml.MappingNode:
		var m map[string]yaml.Node
		if err := value.Decode(&m); err != nil {
			return err
		}
		names := make([]string, 0, len(m))
		for name := range m {
			names = append(names, name)
		}
		*d = names
	}
	return nil
}

// Build handles both string shorthand and map forms:
//
//	build: ./dir
//	build:
//	  context: ./dir
//	  dockerfile: Dockerfile
type Build struct {
	Context    string
	Dockerfile string
}

func (b *Build) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		b.Context = value.Value
	case yaml.MappingNode:
		type buildAlias struct {
			Context    string `yaml:"context"`
			Dockerfile string `yaml:"dockerfile"`
		}
		var a buildAlias
		if err := value.Decode(&a); err != nil {
			return err
		}
		b.Context = a.Context
		b.Dockerfile = a.Dockerfile
	}
	return nil
}

// ServiceNames returns services in their definition order.
func (f *File) ServiceNames() []string {
	names := make([]string, 0, len(f.Services))
	for name := range f.Services {
		names = append(names, name)
	}
	return names
}

func ParseFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading compose file: %w", err)
	}
	return Parse(data)
}

func Parse(data []byte) (*File, error) {
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing compose file: %w", err)
	}
	if f.Services == nil {
		return nil, fmt.Errorf("no services found in compose file")
	}
	return &f, nil
}
