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
	Entry  string `yaml:"entry"`
	Output string `yaml:"output"`
}

func (Go) Name() string { return "go" }

func (Go) Validate(raw yaml.Node) error {
	if err := validateAllowedKeys(raw, "go", map[string]bool{"entry": true, "output": true}); err != nil {
		return err
	}
	var cfg goConfig
	if err := raw.Decode(&cfg); err != nil {
		return err
	}
	if cfg.Entry == "" {
		return fmt.Errorf("go.entry is required")
	}
	if cfg.Output == "" {
		return fmt.Errorf("go.output is required")
	}
	return nil
}

func (Go) Run(ctx context.Context, task TaskContext, raw yaml.Node) (Output, error) {
	var cfg goConfig
	if err := raw.Decode(&cfg); err != nil {
		return Output{}, err
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-o", task.ResolveBuildPath(cfg.Output), task.ResolveBuildPath(cfg.Entry))
	cmd.Dir = task.WorkDir
	cmd.Env = task.Env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return Output{Stdout: stdout.String(), Stderr: stderr.String()}, err
}
