# Oseram Usage

Oseram reads a YAML build file and runs tasks in dependency order. Independent tasks run in parallel when their dependencies are complete.

## Build File

By default Oseram reads `build.yml` from the current directory. Use `--file` to choose another file.

A build file contains tasks at the root level. The root key `global` is reserved for shared settings and is not a task.

```yaml
global:
  workingDirectory: ./
  environment:
    CGO_ENABLED: "0"
  verbose: false
  default: server

frontend:
  workingDirectory: ./frontend
  npm:
    install: true
    task: build

server:
  dependsOn: frontend
  environment:
    GOOS: linux
  go:
    entry: .
    output: dist/server
```

All relative paths are resolved relative to the directory containing the build file.

## Global Settings

```yaml
global:
  workingDirectory: ./
  environment:
    KEY: value
  verbose: false
```

Parameters:

- `workingDirectory`: default working directory for tasks.
- `environment`: environment variables applied to every task.
- `verbose`: when `true`, prints captured stdout/stderr for successful tasks too.
- `default`: optional task to run when no task argument is provided.

Task-level settings override global settings.

## Tasks

Each task is a root-level key, except `global`.

A task may contain:

- `dependsOn`: a dependency or list of dependencies.
- `workingDirectory`: task working directory.
- `environment`: task-specific environment variables.
- exactly one action: `go`, `npm`, `zip`, `copy`, or `delete`.

`dependsOn` may be either a string or a list:

```yaml
server:
  dependsOn: frontend
  go:
    entry: .
    output: dist/server

package:
  dependsOn:
    - frontend
    - server
  zip:
    source: dist
    output: dist/app.zip
```

If no task is provided on the command line, Oseram runs `global.default` when it is set. Otherwise it runs the single final task: the task that no other task depends on. If there is more than one final task and no default is set, Oseram exits with an error and asks for an explicit task.

Every selected task always runs. Oseram does not skip tasks based on timestamps or existing outputs.

## Actions

### `go`

Builds Go code using `go build`.

```yaml
server:
  go:
    entry: .
    output: dist/server
```

Parameters:

- `entry`: required. Go package or path to build.
- `output`: required. Output binary path.

Command run:

```sh
go build -o <output> <entry>
```

### `npm`

Runs npm commands.

```yaml
frontend:
  workingDirectory: ./frontend
  npm:
    install: true
    task: build
```

Parameters:

- `install`: optional boolean. If `true`, runs `npm install` before running a task.
- `task`: optional npm script name. Required unless `install` is `true`.

Examples:

```yaml
# Run npm install, then npm run build
frontend:
  npm:
    install: true
    task: build

# Only run npm install
installFrontend:
  npm:
    install: true
```

Commands run:

```sh
npm install        # if install is true
npm run <task>    # if task is set
```

### `delete`

Deletes files matching one or more doublestar-style glob patterns.

```yaml
clean:
  delete:
    patterns:
      - "dist/**/*"
```

Parameters:

- `patterns`: required list of doublestar-style glob patterns for files to delete.
- `force`: optional boolean. Defaults to `false`.

Safety guard:

- Without `force`, each pattern may only match files inside a single top-level folder, or up to 10 root-level files.
- Without `force`, Oseram aborts if one pattern matches multiple top-level folders, matches both root files and a top-level folder, or matches files outside the build root.
- Use `force: true` to bypass this check. Forced delete tasks are highlighted in the task UI.

```yaml
cleanEverything:
  delete:
    force: true
    patterns:
      - "**/*"
```

### `zip`

Creates a zip archive. Existing output zip files are overwritten.

```yaml
archive:
  zip:
    source: dist
    output: dist/app.zip
    include:
      - "**/*"
    exclude:
      - "**/*.map"
```

Parameters:

- `source`: required. File or directory to archive.
- `output`: required. Output zip file path.
- `include`: optional list of doublestar-style glob patterns.
- `exclude`: optional list of doublestar-style glob patterns.

Patterns support `**` doublestar semantics.

### `copy`

Copies files matching a doublestar-style glob pattern into a destination folder. Existing destination files are overwritten.

```yaml
copyAssets:
  copy:
    source: "assets/**/*"
    destination: "dist/assets"
```

Parameters:

- `source`: required. Doublestar-style glob pattern for files to copy.
- `destination`: required. Destination folder.

Matching directories are ignored.

## Command Line

```text
oseram [flags] [task]
```

Arguments:

- `[task]`: optional task name. If provided, Oseram runs that task and all of its dependencies. If omitted, Oseram uses `global.default` when set, otherwise it infers the single final task.

Flags:

- `--file <path>`: build file to read. Defaults to `build.yml`.
- `--dry-run`: print the execution plan without running tasks.
- `--list`: list available tasks and exit.
- `--verbose`: print captured stdout/stderr for successful tasks.
- `--help`: show a short command line summary.

Examples:

```sh
# Run inferred final task from build.yml
oseram

# Run a specific task and its dependencies
oseram server

# Use another build file
oseram --file example/build.yml

# Preview execution order
oseram --dry-run

# List tasks
oseram --list

# Show captured output while running
oseram --verbose
```

## Console Output

During execution, Oseram shows a task list with status icons:

- `○`: waiting
- `▶`: running
- `✓`: success
- `✗`: failure

On a successful non-dry-run build, Oseram prints a random quote after the results.
