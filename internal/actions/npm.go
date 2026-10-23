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
	Test    string `yaml:"test"`
}

func (NPM) Name() string { return "npm" }

func (NPM) Validate(raw yaml.Node) error {
	if err := validateAllowedKeys(raw, "npm", map[string]bool{"task": true, "install": true, "test": true}); err != nil {
		return err
	}
	var cfg npmConfig
	if err := raw.Decode(&cfg); err != nil {
		return err
	}
	if cfg.Task == "" && !cfg.Install && !npmTestSet(cfg.Test) {
		return fmt.Errorf("npm.task is required unless npm.install is true or npm.test is set")
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
	if npmTestSet(cfg.Test) {
		switch cfg.Test {
		case "true":
			// npm test runs the `test` script by default.
			if err := runNPMCommand(ctx, task, &stdout, &stderr, "test"); err != nil {
				return Output{Stdout: stdout.String(), Stderr: stderr.String()}, err
			}
		default:
			if err := runNPMCommand(ctx, task, &stdout, &stderr, "run", cfg.Test); err != nil {
				return Output{Stdout: stdout.String(), Stderr: stderr.String()}, err
			}
		}
	}
	if cfg.Task == "" {
		return Output{Stdout: stdout.String(), Stderr: stderr.String()}, nil
	}
	if err := runNPMCommand(ctx, task, &stdout, &stderr, "run", cfg.Task); err != nil {
		return Output{Stdout: stdout.String(), Stderr: stderr.String()}, err
	}
	return Output{Stdout: stdout.String(), Stderr: stderr.String()}, nil
}

// npmTestSet reports whether npm.test is configured: `true` to run npm test,
// or a script name to run via `npm run <name>`.
func npmTestSet(test string) bool {
	return test != "" && test != "false"
}

func runNPMCommand(ctx context.Context, task TaskContext, stdout, stderr *bytes.Buffer, args ...string) error {
	cmd := exec.CommandContext(ctx, "npm", args...)
	cmd.Dir = task.WorkDir
	cmd.Env = task.Env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
