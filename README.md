# Oseram

Oseram is a small Go build tool that reads `build.yml` and runs tasks in dependency order. Independent tasks are run in parallel when possible.

## Install
Install easily with go install:

```
go install github.com/andrehag/oseram/cmd/oseram@latest
go install github.com/andrehag/oseram/cmd/oseram-init@latest
```

## Build

```sh
go build -o oseram ./cmd/oseram
go build -o oseram-init ./cmd/oseram-init
```

Or install into your Go bin directory:

```sh
go install ./cmd/oseram
go install ./cmd/oseram-init
```

## Initialize a Build File

Create a suggested `build.yml` by scanning the current folder for Go entry points and `package.json` files:

```sh
./oseram-init
```

Scan another directory:

```sh
./oseram-init --working-directory path/to/project
```

If `build.yml` already exists in the working directory, `oseram-init` exits with a message and leaves it unchanged. Suggested outputs are placed under `dist/`.

## Run

Run the default task from `global.default`, or the single final/root task inferred from `build.yml`:

```sh
./oseram
```

Run a specific task and all of its dependencies:

```sh
./oseram server
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

## Command Line Parameters

```text
oseram [flags] [task]
```

`oseram-init` flags:

- `--working-directory <path>`: directory to scan and write `build.yml` into. Defaults to `.`.
- `--help`: show a short command line summary.

`oseram` flags:

- `--file <path>`: build file to read. Defaults to `build.yml`.
- `--dry-run`: print the execution plan without running tasks.
- `--list`: list available tasks and exit.
- `--verbose`: print captured stdout/stderr and execution details.
- `--help`: show a short command line summary.

Arguments:

- `[task]`: optional task name. If omitted, Oseram runs `global.default` when set; otherwise it runs the single final task that no other task depends on. If multiple final tasks exist and no default is set, Oseram fails and asks for an explicit task.
