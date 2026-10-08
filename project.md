# Osseram Project Plan

## Overview

Osseram is a small build tool written in Go. It reads a `build.yml` file and runs named tasks in dependency order. Tasks are declared at the root level of the YAML file and may depend on each other using `dependsOn`.

The initial supported actions are:

- `go`: compile Go code
- `npm`: run an npm script
- `zip`: create a zip archive

The implementation should make it straightforward to add more actions later without changing the scheduler or YAML loading logic.

Example input is available at `example/build.yml`.

## Configuration Model

A `build.yml` file contains task names at the root level. The reserved root key `global` is used for shared settings and is not a task.

Each task may define:

- exactly one action block, such as `go`, `npm`, or `zip`
- `dependsOn`, as either a single string or a list of strings
- `workingDirectory`, setting the task execution directory
- `environment`, setting task-specific environment variables

`global` supports:

- `workingDirectory`
- `environment`
- `verbose`

Task-level values override global values. Action blocks should contain only action-specific options; `workingDirectory` and `environment` belong at task level, not action level.

All paths are resolved relative to the directory containing the active build file, not necessarily the process working directory.

Example shape:

```yaml
global:
  workingDirectory: ./
  environment:
    CGO_ENABLED: 0
  verbose: false

server:
  dependsOn: frontend
  workingDirectory: ./
  environment:
    GOOS: linux
  go:
    entry: .
    output: dist/server

frontend:
  workingDirectory: ./frontend
  npm:
    task: build
```

## Execution Behavior

1. Load `build.yml`.
2. Parse root-level task definitions, excluding reserved keys such as `global`.
3. Validate the configuration:
   - each task has exactly one action
   - `dependsOn` is either a string or list of strings
   - dependencies reference existing tasks
   - dependency graph has no cycles
   - required fields for each action are present
   - there is exactly one root/final task that no other task depends on
4. Build a directed dependency graph.
5. If a task is explicitly requested, run that task and all of its dependencies.
6. If no task is explicitly requested, run the single root/final task and all of its dependencies.
7. If no task is requested and more than one root/final task exists, do not run anything; report a configuration error.
8. Schedule selected tasks so dependencies run before dependents.
9. Run independent tasks in parallel when their dependencies have completed.
10. For each task:
    - merge global and task-level working directory settings
    - merge global and task-level environment variables
    - invoke the selected action handler
11. Every selected task always runs; Osseram does not skip tasks based on timestamps or existing outputs.
12. Stop scheduling new tasks after the first failure and return a non-zero exit code. Already-running parallel tasks may finish before shutdown completes.

## Proposed CLI

Initial command:

```sh
osseram [task]
```

Behavior:

- If a task name is supplied, run that task and all of its dependencies.
- If no task name is supplied, infer the single root/final task and run it with its dependencies.
- If no task name is supplied and multiple root/final tasks exist, fail without running tasks.
- There is no reserved `default` task or `default` setting.
- By default, read `build.yml` from the current directory.

Initial/future flags:

- `--file <path>`: use a config file other than `build.yml`
- `--dry-run`: print execution order without running commands
- `--list`: list available tasks
- `--verbose`: print captured stdout/stderr and resolved execution details

Command stdout/stderr should be captured by default. Captured output is printed when `--verbose` is set or when `global.verbose: true` is configured. On failure, captured stdout/stderr should be printed to aid debugging.

## Initial Actions

### Go Action

Compiles Go code using `go build`.

Suggested fields:

- `entry`: package or path to build, for example `.`
- `output`: output binary path

Command shape:

```sh
go build -o <output> <entry>
```

### NPM Action

Runs an npm script.

Suggested fields:

- `task`: npm script name

Command shape:

```sh
npm run <task>
```

### Zip Action

Creates a zip archive. The zip action always overwrites the output zip file if it already exists.

Suggested fields:

- `source`: file or directory to archive
- `output`: zip file path
- `include`: optional list of doublestar-style glob patterns to include
- `exclude`: optional list of doublestar-style glob patterns to exclude

Zip include/exclude patterns should support `**` doublestar semantics. Implementation should use Go's standard `archive/zip` package rather than shelling out to a platform-specific zip command.

## Go Package Structure

Suggested structure:

```text
.
├── cmd/osseram/main.go
├── internal/config
│   └── config.go
├── internal/graph
│   └── graph.go
├── internal/runner
│   └── runner.go
├── internal/actions
│   ├── action.go
│   ├── go.go
│   ├── npm.go
│   └── zip.go
├── example/build.yml
└── project.md
```

### `internal/config`

Responsibilities:

- load YAML
- parse global defaults and tasks
- preserve task/action names
- validate required fields
- resolve the build file directory used as the base for relative paths

### `internal/graph`

Responsibilities:

- resolve dependencies
- detect unknown dependencies
- detect cycles
- detect the single root/final task when no task is requested
- return deterministic dependency levels or ready queues for parallel execution

### `internal/actions`

Responsibilities:

- define a common action interface
- register built-in action handlers
- isolate action-specific config parsing and validation

Suggested interface:

```go
type Action interface {
    Name() string
    Validate(raw map[string]any) error
    Run(ctx context.Context, task TaskContext, raw map[string]any) error
}
```

A registry can map action names to constructors or handlers:

```go
type Registry map[string]Action
```

This keeps adding a new action mostly limited to adding a new file under `internal/actions` and registering it.

### `internal/runner`

Responsibilities:

- schedule runnable tasks in parallel once their dependencies are complete
- stop scheduling new tasks on first failure
- merge environment variables
- resolve working directories relative to the build file location
- invoke action handlers
- capture stdout/stderr by default
- print captured output in verbose mode or on failure
- report task status

## Development Plan

### Phase 1: Project Skeleton

- Initialize a Go module.
- Create the CLI entry point at `cmd/osseram/main.go`.
- Add YAML parsing dependency, likely `gopkg.in/yaml.v3`.
- Define core config structs and action interface.

### Phase 2: Config Loading and Validation

- Load `build.yml`.
- Parse reserved `global` defaults.
- Parse root-level tasks.
- Support `dependsOn` as a string or list of strings.
- Validate that each task has exactly one known action.
- Validate required action fields.
- Validate that `workingDirectory` and `environment` are task-level/global-level settings.
- Add clear error messages for invalid YAML.

### Phase 3: Dependency Ordering

- Implement dependency graph construction.
- Detect missing dependencies.
- Detect dependency cycles.
- Detect root/final tasks, defined as tasks that no other task depends on.
- Fail when no explicit task is requested and there is not exactly one root/final task.
- Produce deterministic dependency levels or ready queues for parallel execution.

### Phase 4: Action Execution

- Implement `go` action with `os/exec`.
- Implement `npm` action with `os/exec`.
- Implement `zip` action using `archive/zip`, including doublestar-style include/exclude pattern support and overwrite behavior.
- Merge global and task-level environment variables.
- Resolve working directories and action paths relative to the build file location.
- Capture stdout/stderr by default and print it only in verbose mode or on failure.
- Run independent tasks in parallel while respecting dependency ordering.

### Phase 5: CLI UX

- Support running an explicitly selected task.
- Support inferring and running the single root/final task when no task is selected.
- Fail clearly when multiple root/final tasks exist and no task is selected.
- Print planned/executed task names.
- Return non-zero exit codes on validation or execution failures.
- Add optional flags such as `--file`, `--dry-run`, `--list`, and `--verbose`.

### Phase 6: Tests

- Unit test YAML parsing.
- Unit test `dependsOn` string and list forms.
- Unit test dependency scheduling and cycle detection.
- Unit test root/final task detection.
- Unit test environment and working directory merging.
- Unit test path resolution relative to the build file location.
- Unit test command output capture and verbose printing behavior.
- Unit test action command construction.
- Unit test zip doublestar include/exclude and overwrite behavior.
- Add integration tests using temporary directories and fixture `build.yml` files.

### Phase 7: Documentation

- Document `build.yml` format.
- Document root/final task inference.
- Document each built-in action.
- Document how to add a new action.
- Include example configurations.

## Outstanding Questions

No outstanding questions at this time.
