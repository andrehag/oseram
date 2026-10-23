# Oseram

Oseram is a small Go build tool that reads `build.yml` and runs tasks in dependency order. Independent tasks are run in parallel when possible. Tasks can build Go code, and install or run `npm` scripts, including running tests for both.
[See usage-instructions](usage.md) for how to create a build-file.

## Install
Install easily with go install:

```
go install github.com/andrehag/oseram/cmd/oseram@latest
```

## Build

```sh
go build -o oseram ./cmd/oseram
```

Or install into your Go bin directory:

```sh
go install ./cmd/oseram
```

## Initialize a Build File

Create a suggested `build.yml` by scanning the current folder for Go entry points and `package.json` files:

```sh
./oseram init
```

Scan another directory:

```sh
./oseram init --working-directory path/to/project
```

If `build.yml` already exists in the working directory, `oseram init` exits with a message and leaves it unchanged. Suggested outputs are placed under `dist/`. Suggested test tasks are added for Go packages that contain `_test.go` files and for `package.json` files that define a `test` script.

## Run

Run the default task from `global.default`, or the single final/root task inferred from `build.yml`:

```sh
./oseram
```

Run a specific task and all of its dependencies:

```sh
./oseram server
```

Run several tasks in one invocation. Independent targets run in parallel:

```sh
./oseram lint server
```

Use a build file at another path:

```sh
./oseram --file example/build.yml
```

Preview the execution plan without running tasks:

```sh
./oseram --dry-run
```

List available tasks:

```sh
./oseram --list
```

Print captured stdout/stderr while tasks run:

```sh
./oseram --verbose
```

Use simplified, log-friendly output for automated builds, scripts, or piped output:

```sh
./oseram --non-interactive
```

Oseram detects when stdout is not a terminal (for example in CI or when output is piped) and switches to the simplified output automatically.

## Command Line Parameters

```text
oseram [flags] [task]
oseram init [flags]
```

`oseram init` flags:

- `--working-directory <path>`: directory to scan and write `build.yml` into. Defaults to `.`.
- `--help`: show a short command line summary.

`oseram` flags:

- `--file <path>`: build file to read. Defaults to `build.yml`.
- `--dry-run`: print the execution plan without running tasks.
- `--list`: list available tasks and exit.
- `--verbose`: print captured stdout/stderr and execution details.
- `--non-interactive`: use simplified line output instead of the interactive TUI. Automatically enabled when stdout is not a terminal.
- `--help`: show a short command line summary.

Arguments:

- `[task...]`: optional task names. If omitted, Oseram runs `global.default` when set; otherwise it runs the single final task that no other task depends on. If multiple final tasks exist and no default is set, Oseram fails and asks for an explicit task. Multiple tasks are run with their combined dependencies; independent targets run in parallel.
