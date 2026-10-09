package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type buildFile map[string]any

type packageJSON struct {
	Scripts map[string]string `json:"scripts"`
}

type taskSuggestion struct {
	Name string
	Body map[string]any
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	flag.Usage = printHelp
	workingDirectory := flag.String("working-directory", ".", "directory to scan and write build.yml into")
	flag.Parse()
	if flag.NArg() > 0 {
		return fmt.Errorf("oseram-init does not accept positional arguments")
	}

	cwd, err := filepath.Abs(*workingDirectory)
	if err != nil {
		return err
	}
	info, err := os.Stat(cwd)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("working directory %q is not a directory", cwd)
	}
	buildPath := filepath.Join(cwd, "build.yml")
	if _, err := os.Stat(buildPath); err == nil {
		fmt.Println("build.yml already exists; leaving it unchanged")
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	goTasks, err := findGoTasks(cwd)
	if err != nil {
		return err
	}
	npmTasks, err := findNPMTasks(cwd)
	if err != nil {
		return err
	}
	tasks := append(goTasks, npmTasks...)
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Name < tasks[j].Name })
	if len(tasks) == 0 {
		fmt.Println("no Go entry points or package.json files found; no build.yml created")
		return nil
	}

	root := buildFile{
		"global": map[string]any{
			"workingDirectory": "./",
		},
	}
	if len(tasks) == 1 {
		root["global"].(map[string]any)["default"] = tasks[0].Name
	}
	for _, task := range tasks {
		root[task.Name] = task.Body
	}

	data, err := marshalYAML(root, tasks)
	if err != nil {
		return err
	}
	if err := os.WriteFile(buildPath, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("created build.yml with %d suggested task(s)\n", len(tasks))
	return nil
}

func findGoTasks(root string) ([]taskSuggestion, error) {
	var tasks []taskSuggestion
	seenDirs := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipDir(path, root) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		dir := filepath.Dir(path)
		if seenDirs[dir] {
			return nil
		}
		ok, err := isMainPackageWithMainFunc(path)
		if err != nil || !ok {
			return err
		}
		seenDirs[dir] = true
		relDir := relSlash(root, dir)
		namePart := safeName(relDir)
		if relDir == "." {
			namePart = filepath.Base(root)
		}
		taskName := uniqueTaskName("go-"+namePart, existingNames(tasks))
		entry := "./" + relDir
		if relDir == "." {
			entry = "."
		}
		outputName := namePart
		tasks = append(tasks, taskSuggestion{
			Name: taskName,
			Body: map[string]any{
				"go": map[string]any{
					"entry":  entry,
					"output": "dist/" + outputName,
				},
			},
		})
		return nil
	})
	return tasks, err
}

func findNPMTasks(root string) ([]taskSuggestion, error) {
	var tasks []taskSuggestion
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipDir(path, root) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Base(path) != "package.json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var pkg packageJSON
		if err := json.Unmarshal(data, &pkg); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		dir := filepath.Dir(path)
		relDir := relSlash(root, dir)
		namePart := safeName(relDir)
		if relDir == "." {
			namePart = "frontend"
		}
		npm := map[string]any{"install": true}
		if _, ok := pkg.Scripts["build"]; ok {
			npm["task"] = "build"
		}
		body := map[string]any{"npm": npm}
		if relDir != "." {
			body["workingDirectory"] = "./" + relDir
		}
		taskName := uniqueTaskName("npm-"+namePart, existingNames(tasks))
		tasks = append(tasks, taskSuggestion{Name: taskName, Body: body})
		return nil
	})
	return tasks, err
}

func isMainPackageWithMainFunc(path string) (bool, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return false, err
	}
	if file.Name.Name != "main" {
		return false, nil
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "main" && fn.Recv == nil {
			return true, nil
		}
	}
	return false, nil
}

func shouldSkipDir(path, root string) bool {
	if path == root {
		return false
	}
	base := filepath.Base(path)
	switch base {
	case ".git", "dist", "node_modules", "vendor", ".idea", ".vscode":
		return true
	default:
		return strings.HasPrefix(base, ".")
	}
}

func relSlash(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "" {
		return "."
	}
	return filepath.ToSlash(rel)
}

func safeName(path string) string {
	path = strings.Trim(path, "./")
	if path == "" || path == "." {
		return "root"
	}
	parts := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\' || r == '_' || r == ' ' || r == '.'
	})
	return strings.ToLower(strings.Join(parts, "-"))
}

func existingNames(tasks []taskSuggestion) map[string]bool {
	names := map[string]bool{}
	for _, task := range tasks {
		names[task.Name] = true
	}
	return names
}

func uniqueTaskName(base string, existing map[string]bool) string {
	if !existing[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !existing[candidate] {
			return candidate
		}
	}
}

func marshalYAML(root buildFile, tasks []taskSuggestion) ([]byte, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	addMapEntry(node, "global", root["global"])
	for _, task := range tasks {
		addMapEntry(node, task.Name, task.Body)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	formatted := addTopLevelSpacing(strings.TrimRight(buf.String(), "\n"))
	header := fmt.Sprintf("# Autogenerated by oseram-init at %s\n\n", time.Now().Format(time.RFC3339))
	return []byte(header + formatted + "\n"), nil
}

func addTopLevelSpacing(yamlText string) string {
	lines := strings.Split(yamlText, "\n")
	var out []string
	for i, line := range lines {
		if i > 0 && line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "-") {
			out = append(out, "")
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func printHelp() {
	fmt.Fprintf(flag.CommandLine.Output(), `Oseram Init - create a suggested build.yml for the current folder.

Usage:
  oseram-init [flags]

Flags:
  --working-directory <path>  Directory to scan and write build.yml into (default: .)
  --help                      Show this help text

Scans for Go entry points and package.json files. If build.yml already exists,
it is left unchanged.
`)
}

func addMapEntry(node *yaml.Node, key string, value any) {
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, mustNode(value))
}

func mustNode(value any) *yaml.Node {
	var node yaml.Node
	if err := node.Encode(value); err != nil {
		panic(err)
	}
	return &node
}
