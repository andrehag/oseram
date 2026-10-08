package actions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

const maxSafeRootFiles = 10

type Delete struct{}

type deleteConfig struct {
	Patterns []string `yaml:"patterns"`
	Force    bool     `yaml:"force"`
}

func (Delete) Name() string { return "delete" }

func (Delete) Validate(raw yaml.Node) error {
	if err := validateAllowedKeys(raw, "delete", map[string]bool{"patterns": true, "force": true}); err != nil {
		return err
	}
	var cfg deleteConfig
	if err := raw.Decode(&cfg); err != nil {
		return err
	}
	if len(cfg.Patterns) == 0 {
		return fmt.Errorf("delete.patterns is required")
	}
	for _, pattern := range cfg.Patterns {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("delete.patterns must not contain empty patterns")
		}
	}
	return nil
}

func (Delete) Run(ctx context.Context, task TaskContext, raw yaml.Node) (Output, error) {
	var cfg deleteConfig
	if err := raw.Decode(&cfg); err != nil {
		return Output{}, err
	}

	seen := map[string]bool{}
	var files []string
	for _, pattern := range cfg.Patterns {
		resolved := task.ResolveBuildPath(pattern)
		matches, err := doublestar.FilepathGlob(resolved)
		if err != nil {
			return Output{}, err
		}
		matchedFiles, err := filterDeleteFiles(matches)
		if err != nil {
			return Output{}, err
		}
		if !cfg.Force {
			if err := validateSafeDeletePattern(task.BaseDir, pattern, matchedFiles); err != nil {
				return Output{}, err
			}
		}
		for _, file := range matchedFiles {
			if !seen[file] {
				seen[file] = true
				files = append(files, file)
			}
		}
	}
	sort.Strings(files)
	for _, file := range files {
		select {
		case <-ctx.Done():
			return Output{}, ctx.Err()
		default:
		}
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return Output{}, err
		}
	}
	return Output{Stdout: fmt.Sprintf("deleted %d file(s)\n", len(files))}, nil
}

func filterDeleteFiles(matches []string) ([]string, error) {
	var files []string
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, match)
		}
	}
	return files, nil
}

func validateSafeDeletePattern(baseDir, pattern string, files []string) error {
	topDirs := map[string]bool{}
	rootFiles := 0
	for _, file := range files {
		rel, err := filepath.Rel(baseDir, file)
		if err != nil {
			return err
		}
		if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
			return fmt.Errorf("delete pattern %q matches files outside the build root; use force: true to allow this", pattern)
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) <= 1 {
			rootFiles++
			continue
		}
		topDirs[parts[0]] = true
	}
	if len(topDirs) > 1 {
		return fmt.Errorf("delete pattern %q matches more than one top-level folder; use force: true to allow this", pattern)
	}
	if len(topDirs) == 1 && rootFiles > 0 {
		return fmt.Errorf("delete pattern %q matches both a top-level folder and root files; use force: true to allow this", pattern)
	}
	if len(topDirs) == 0 && rootFiles > maxSafeRootFiles {
		return fmt.Errorf("delete pattern %q matches %d root files; maximum without force is %d", pattern, rootFiles, maxSafeRootFiles)
	}
	return nil
}
