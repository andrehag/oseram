package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

type Config struct {
	FilePath string
	BaseDir  string
	Global   Global
	Tasks    map[string]*Task
}

type Global struct {
	WorkingDirectory string            `yaml:"workingDirectory"`
	Environment      map[string]string `yaml:"environment"`
	Verbose          bool              `yaml:"verbose"`
}

type Task struct {
	Name             string
	DependsOn        []string
	WorkingDirectory string
	Environment      map[string]string
	ActionName       string
	Action           yaml.Node
}

var reservedTaskKeys = map[string]bool{
	"dependsOn":        true,
	"workingDirectory": true,
	"environment":      true,
}

func Load(path string, actionNames map[string]bool) (*Config, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("build file must contain a YAML mapping at the root")
	}

	cfg := &Config{FilePath: abs, BaseDir: filepath.Dir(abs), Tasks: map[string]*Task{}}
	mapping := root.Content[0]
	for i := 0; i < len(mapping.Content); i += 2 {
		key := mapping.Content[i].Value
		val := mapping.Content[i+1]
		if key == "global" {
			if err := val.Decode(&cfg.Global); err != nil {
				return nil, fmt.Errorf("decode global: %w", err)
			}
			continue
		}
		if _, exists := cfg.Tasks[key]; exists {
			return nil, fmt.Errorf("duplicate task %q", key)
		}
		task, err := parseTask(key, val, actionNames)
		if err != nil {
			return nil, err
		}
		cfg.Tasks[key] = task
	}
	return cfg, nil
}

func parseTask(name string, node *yaml.Node, actionNames map[string]bool) (*Task, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("task %q must be a mapping", name)
	}
	t := &Task{Name: name, Environment: map[string]string{}}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		switch key {
		case "dependsOn":
			deps, err := decodeDependsOn(val)
			if err != nil {
				return nil, fmt.Errorf("task %q dependsOn: %w", name, err)
			}
			t.DependsOn = deps
		case "workingDirectory":
			if err := val.Decode(&t.WorkingDirectory); err != nil {
				return nil, fmt.Errorf("task %q workingDirectory: %w", name, err)
			}
		case "environment":
			if err := val.Decode(&t.Environment); err != nil {
				return nil, fmt.Errorf("task %q environment: %w", name, err)
			}
		default:
			if !actionNames[key] {
				return nil, fmt.Errorf("task %q has unknown key/action %q", name, key)
			}
			if t.ActionName != "" {
				return nil, fmt.Errorf("task %q must define exactly one action", name)
			}
			t.ActionName = key
			t.Action = *val
		}
	}
	if t.ActionName == "" {
		return nil, fmt.Errorf("task %q must define exactly one action", name)
	}
	return t, nil
}

func decodeDependsOn(node *yaml.Node) ([]string, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		var s string
		if err := node.Decode(&s); err != nil {
			return nil, err
		}
		if s == "" {
			return nil, nil
		}
		return []string{s}, nil
	case yaml.SequenceNode:
		var deps []string
		if err := node.Decode(&deps); err != nil {
			return nil, err
		}
		return deps, nil
	default:
		return nil, fmt.Errorf("must be a string or list of strings")
	}
}

func TaskNames(tasks map[string]*Task) []string {
	names := make([]string, 0, len(tasks))
	for name := range tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
