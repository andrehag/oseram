package actions

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

type Copy struct{}

type copyConfig struct {
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
}

func (Copy) Name() string { return "copy" }

func (Copy) Validate(raw yaml.Node) error {
	if err := validateAllowedKeys(raw, "copy", map[string]bool{"source": true, "destination": true}); err != nil {
		return err
	}
	var cfg copyConfig
	if err := raw.Decode(&cfg); err != nil {
		return err
	}
	if cfg.Source == "" {
		return fmt.Errorf("copy.source is required")
	}
	if cfg.Destination == "" {
		return fmt.Errorf("copy.destination is required")
	}
	return nil
}

func (Copy) Run(ctx context.Context, task TaskContext, raw yaml.Node) (Output, error) {
	var cfg copyConfig
	if err := raw.Decode(&cfg); err != nil {
		return Output{}, err
	}

	pattern := task.ResolveBuildPath(cfg.Source)
	destination := task.ResolveBuildPath(cfg.Destination)
	base := globBase(pattern)

	matches, err := doublestar.FilepathGlob(pattern)
	if err != nil {
		return Output{}, err
	}
	for _, match := range matches {
		select {
		case <-ctx.Done():
			return Output{}, ctx.Err()
		default:
		}
		info, err := os.Stat(match)
		if err != nil {
			return Output{}, err
		}
		if info.IsDir() {
			continue
		}
		rel, err := filepath.Rel(base, match)
		if err != nil {
			return Output{}, err
		}
		if strings.HasPrefix(rel, "..") {
			rel = filepath.Base(match)
		}
		to := filepath.Join(destination, rel)
		if err := copyFile(match, to, info.Mode()); err != nil {
			return Output{}, err
		}
	}
	return Output{}, nil
}

func globBase(pattern string) string {
	idx := strings.IndexAny(pattern, "*?[")
	if idx == -1 {
		return filepath.Dir(filepath.Clean(pattern))
	}
	prefix := pattern[:idx]
	if prefix == "" {
		return "."
	}
	if strings.HasSuffix(prefix, string(filepath.Separator)) {
		return filepath.Clean(prefix)
	}
	return filepath.Dir(filepath.Clean(prefix))
}

func copyFile(from, to string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
