package actions

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"gopkg.in/yaml.v3"
)

type Go struct{}

type goConfig struct {
	Entry  string   `yaml:"entry"`
	Output string   `yaml:"output"`
	Test   string   `yaml:"test"`
	Args   []string `yaml:"args"`
}

func (Go) Name() string { return "go" }

func (Go) Validate(raw yaml.Node) error {
	if err := validateAllowedKeys(raw, "go", map[string]bool{"entry": true, "output": true, "test": true, "args": true}); err != nil {
		return err
	}
	var cfg goConfig
	if err := raw.Decode(&cfg); err != nil {
		return err
	}
	test := goTestPattern(cfg.Test)
	if test == "" {
		if cfg.Entry == "" {
			return fmt.Errorf("go.entry is required unless go.test is set")
		}
		if cfg.Output == "" {
			return fmt.Errorf("go.output is required unless go.test is set")
		}
	}
	return nil
}

func (Go) Run(ctx context.Context, task TaskContext, raw yaml.Node) (Output, error) {
	var cfg goConfig
	if err := raw.Decode(&cfg); err != nil {
		return Output{}, err
	}

	var args []string
	if test := goTestPattern(cfg.Test); test != "" {
		args = append([]string{"test"}, cfg.Args...)
		args = append(args, test)
	} else {
		args = append([]string{"build", "-o", task.ResolveBuildPath(cfg.Output)}, cfg.Args...)
		args = append(args, task.ResolveBuildPath(cfg.Entry))
	}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = task.WorkDir
	cmd.Env = task.Env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return Output{Stdout: stdout.String(), Stderr: stderr.String()}, err
}

// goTestPattern normalizes the `test` field: `true` means "./...", a non-empty
// value is used as a package pattern, and anything else ("false" or "") means
// the build mode.
func goTestPattern(test string) string {
	switch test {
	case "", "false":
		return ""
	case "true":
		return "./..."
	default:
		return test
	}
}
