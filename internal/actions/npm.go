package actions

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"gopkg.in/yaml.v3"
)

type NPM struct{}

type npmConfig struct {
	Task string `yaml:"task"`
}

func (NPM) Name() string { return "npm" }

func (NPM) Validate(raw yaml.Node) error {
	if err := validateAllowedKeys(raw, "npm", map[string]bool{"task": true}); err != nil {
		return err
	}
	var cfg npmConfig
	if err := raw.Decode(&cfg); err != nil {
		return err
	}
	if cfg.Task == "" {
		return fmt.Errorf("npm.task is required")
	}
	return nil
}

func (NPM) Run(ctx context.Context, task TaskContext, raw yaml.Node) (Output, error) {
	var cfg npmConfig
	if err := raw.Decode(&cfg); err != nil {
		return Output{}, err
	}
	cmd := exec.CommandContext(ctx, "npm", "run", cfg.Task)
	cmd.Dir = task.WorkDir
	cmd.Env = task.Env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return Output{Stdout: stdout.String(), Stderr: stderr.String()}, err
}
