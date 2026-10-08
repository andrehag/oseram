package actions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Output struct {
	Stdout string
	Stderr string
}

type TaskContext struct {
	Name    string
	BaseDir string
	WorkDir string
	Env     []string
}

func (tc TaskContext) ResolveBuildPath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(tc.BaseDir, path)
}

func (tc TaskContext) ResolveWorkPath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(tc.WorkDir, path)
}

type Action interface {
	Name() string
	Validate(raw yaml.Node) error
	Run(ctx context.Context, task TaskContext, raw yaml.Node) (Output, error)
}

type Registry map[string]Action

func Builtins() Registry {
	return Registry{
		"copy": Copy{},
		"go":   Go{},
		"npm":  NPM{},
		"zip":  Zip{},
	}
}

func (r Registry) Names() map[string]bool {
	names := map[string]bool{}
	for name := range r {
		names[name] = true
	}
	return names
}

func MergeEnv(base []string, overrides map[string]string) []string {
	out := append([]string{}, base...)
	index := map[string]int{}
	for i, kv := range out {
		for j, ch := range kv {
			if ch == '=' {
				index[kv[:j]] = i
				break
			}
		}
	}
	for k, v := range overrides {
		kv := k + "=" + v
		if i, ok := index[k]; ok {
			out[i] = kv
		} else {
			index[k] = len(out)
			out = append(out, kv)
		}
	}
	return out
}

func CurrentEnv() []string { return os.Environ() }

func validateAllowedKeys(raw yaml.Node, actionName string, allowed map[string]bool) error {
	if raw.Kind != yaml.MappingNode {
		return fmt.Errorf("%s action must be a mapping", actionName)
	}
	for i := 0; i < len(raw.Content); i += 2 {
		key := raw.Content[i].Value
		if !allowed[key] {
			return fmt.Errorf("%s action has unknown key %q", actionName, key)
		}
	}
	return nil
}
