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
	Task    string `yaml:"task"`
	Install bool   `yaml:"install"`
}

func (NPM) Name() string { return "npm" }

func (NPM) Validate(raw yaml.Node) error {
	if err := validateAllowedKeys(raw, "npm", map[string]bool{"task": true, "install": true}); err != nil {
		return err
	}
	var cfg npmConfig
	if err := raw.Decode(&cfg); err != nil {
		return err
	}
	if cfg.Task == "" && !cfg.Install {
		return fmt.Errorf("npm.task is required unless npm.install is true")
	}
	return nil
}

func (NPM) Run(ctx context.Context, task TaskContext, raw yaml.Node) (Output, error) {
	var cfg npmConfig
	if err := raw.Decode(&cfg); err != nil {
		return Output{}, err
	}
	var stdout, stderr bytes.Buffer
	if cfg.Install {
		if err := runNPMCommand(ctx, task, &stdout, &stderr, "install"); err != nil {
			return Output{Stdout: stdout.String(), Stderr: stderr.String()}, err
		}
	}
	if cfg.Task == "" {
		return Output{Stdout: stdout.String(), Stderr: stderr.String()}, nil
	}
	err := runNPMCommand(ctx, task, &stdout, &stderr, "run", cfg.Task)
	return Output{Stdout: stdout.String(), Stderr: stderr.String()}, err
}

func runNPMCommand(ctx context.Context, task TaskContext, stdout, stderr *bytes.Buffer, args ...string) error {
	cmd := exec.CommandContext(ctx, "npm", args...)
	cmd.Dir = task.WorkDir
	cmd.Env = task.Env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
