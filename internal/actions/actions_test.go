package actions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// decodeAction parses "actionName:\n  <body>" and returns the node for the
// action value, mirroring how config.Load hands the action mapping to
// Validate/Run. body lines are indented so they are children of the action.
func decodeAction(t *testing.T, actionName, body string) yaml.Node {
	t.Helper()
	indented := ""
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		indented += "  " + line + "\n"
	}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(actionName+":\n"+indented), &root); err != nil {
		t.Fatalf("parse yaml: %v", err)
	}
	return *root.Content[0].Content[1]
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGoValidateBuildRequiresEntryAndOutput(t *testing.T) {
	good := decodeAction(t, "go", "entry: .\noutput: dist/app\n")
	if err := (Go{}).Validate(good); err != nil {
		t.Errorf("valid build config rejected: %v", err)
	}
	cases := []struct {
		name string
		body string
	}{
		{"missing entry", "output: dist/app\n"},
		{"missing output", "entry: .\n"},
		{"unknown key", "entry: .\noutput: dist/app\nbogus: 1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := (Go{}).Validate(decodeAction(t, "go", tc.body)); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestGoValidateTestDoesNotRequireEntryOrOutput(t *testing.T) {
	for _, body := range []string{"test: true\n", "test: ./...\n", "test: ./pkg\nentry: .\n"} {
		if err := (Go{}).Validate(decodeAction(t, "go", body)); err != nil {
			t.Errorf("go config with test %q rejected: %v", body, err)
		}
	}
}

func TestGoRunRunsTests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module sample\n\ngo 1.24.0\n")
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	writeFile(t, dir, "main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestPasses(t *testing.T) {}\n")

	ctx := context.Background()
	task := TaskContext{Name: "test", BaseDir: dir, WorkDir: dir, Env: os.Environ()}

	t.Run("test true", func(t *testing.T) {
		out, err := (Go{}).Run(ctx, task, decodeAction(t, "go", "test: true\n"))
		if err != nil {
			t.Fatalf("go test failed: %v\nstdout: %s\nstderr: %s", err, out.Stdout, out.Stderr)
		}
		if !strings.Contains(out.Stdout, "ok") {
			t.Errorf("stdout should contain ok, got: %q", out.Stdout)
		}
	})
	t.Run("test pattern", func(t *testing.T) {
		out, err := (Go{}).Run(ctx, task, decodeAction(t, "go", "test: ./...\n"))
		if err != nil {
			t.Fatalf("go test ./... failed: %v\nstdout: %s\nstderr: %s", err, out.Stdout, out.Stderr)
		}
		if !strings.Contains(out.Stdout, "ok") {
			t.Errorf("stdout should contain ok, got: %q", out.Stdout)
		}
	})
	t.Run("failing test", func(t *testing.T) {
		out, err := (Go{}).Run(ctx, task, decodeAction(t, "go", "test: ./does-not-exist\n"))
		if err == nil {
			t.Fatalf("go test on missing package should fail, got: %s", out.Stdout)
		}
	})
}

func TestGoRunBuildStillWorks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module sample\n\ngo 1.24.0\n")
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")

	ctx := context.Background()
	task := TaskContext{Name: "build", BaseDir: dir, WorkDir: dir, Env: os.Environ()}
	out, err := (Go{}).Run(ctx, task, decodeAction(t, "go", "entry: .\noutput: dist/app\n"))
	if err != nil {
		t.Fatalf("go build failed: %v\nstdout: %s\nstderr: %s", err, out.Stdout, out.Stderr)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "dist/app")); statErr != nil {
		t.Errorf("expected built binary at dist/app: %v", statErr)
	}
}

func TestNPMValidateAllowsTestOnly(t *testing.T) {
	good := decodeAction(t, "npm", "test: true\n")
	if err := (NPM{}).Validate(good); err != nil {
		t.Errorf("npm test-only config rejected: %v", err)
	}
	custom := decodeAction(t, "npm", "test: mycheck\n")
	if err := (NPM{}).Validate(custom); err != nil {
		t.Errorf("npm test-custom-script config rejected: %v", err)
	}
	i := decodeAction(t, "npm", "install: true\n")
	if err := (NPM{}).Validate(i); err != nil {
		t.Errorf("npm install-only config rejected: %v", err)
	}
	none := decodeAction(t, "npm", "\n")
	if err := (NPM{}).Validate(none); err == nil {
		t.Error("npm config with nothing set should be rejected")
	}
}

func TestNPMRunRunsTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"sample","scripts":{"test":"echo test-ran","mycheck":"echo mycheck-ran"}}`)

	ctx := context.Background()
	task := TaskContext{Name: "test", BaseDir: dir, WorkDir: dir, Env: os.Environ()}

	t.Run("npm test default script", func(t *testing.T) {
		out, err := (NPM{}).Run(ctx, task, decodeAction(t, "npm", "test: true\n"))
		if err != nil {
			t.Fatalf("npm test failed: %v\nstderr: %s", err, out.Stderr)
		}
		if !strings.Contains(out.Stdout, "test-ran") {
			t.Errorf("stdout missing script output: %q", out.Stdout)
		}
	})
	t.Run("npm test custom script", func(t *testing.T) {
		out, err := (NPM{}).Run(ctx, task, decodeAction(t, "npm", "test: mycheck\n"))
		if err != nil {
			t.Fatalf("npm run mycheck failed: %v\nstderr: %s", err, out.Stderr)
		}
		if !strings.Contains(out.Stdout, "mycheck-ran") {
			t.Errorf("stdout missing script output: %q", out.Stdout)
		}
	})
}
