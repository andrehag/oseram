package actions

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

type Zip struct{}

type zipConfig struct {
	Source  string   `yaml:"source"`
	Output  string   `yaml:"output"`
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

func (Zip) Name() string { return "zip" }

func (Zip) Validate(raw yaml.Node) error {
	if err := validateAllowedKeys(raw, "zip", map[string]bool{"source": true, "output": true, "include": true, "exclude": true}); err != nil {
		return err
	}
	var cfg zipConfig
	if err := raw.Decode(&cfg); err != nil {
		return err
	}
	if cfg.Source == "" {
		return fmt.Errorf("zip.source is required")
	}
	if cfg.Output == "" {
		return fmt.Errorf("zip.output is required")
	}
	return nil
}

func (Zip) Run(ctx context.Context, task TaskContext, raw yaml.Node) (Output, error) {
	var cfg zipConfig
	if err := raw.Decode(&cfg); err != nil {
		return Output{}, err
	}
	source := task.ResolveBuildPath(cfg.Source)
	output := task.ResolveBuildPath(cfg.Output)
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return Output{}, err
	}
	tmp := output + ".tmp"
	_ = os.Remove(tmp)
	file, err := os.Create(tmp)
	if err != nil {
		return Output{}, err
	}
	zw := zip.NewWriter(file)
	cleanup := func() {
		_ = zw.Close()
		_ = file.Close()
		_ = os.Remove(tmp)
	}

	info, err := os.Stat(source)
	if err != nil {
		cleanup()
		return Output{}, err
	}
	base := source
	if !info.IsDir() {
		base = filepath.Dir(source)
	}
	err = filepath.WalkDir(source, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !matches(rel, cfg.Include, true) || matches(rel, cfg.Exclude, false) {
			return nil
		}
		return addZipFile(zw, path, rel)
	})
	if err != nil {
		cleanup()
		return Output{}, err
	}
	if err := zw.Close(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return Output{}, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return Output{}, err
	}
	if err := os.Rename(tmp, output); err != nil {
		_ = os.Remove(tmp)
		return Output{}, err
	}
	return Output{}, nil
}

func matches(path string, patterns []string, defaultValue bool) bool {
	if len(patterns) == 0 {
		return defaultValue
	}
	path = strings.TrimPrefix(filepath.ToSlash(path), "./")
	for _, pattern := range patterns {
		pattern = strings.TrimPrefix(filepath.ToSlash(pattern), "./")
		ok, _ := doublestar.Match(pattern, path)
		if ok {
			return true
		}
	}
	return false
}

func addZipFile(zw *zip.Writer, path, name string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = name
	header.Method = zip.Deflate
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}
