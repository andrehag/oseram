# Oseram

Oseram is a small Go build tool that reads `build.yml` and runs tasks in dependency order. Independent tasks are run in parallel when possible. After a successful build, Oseram prints a random quote.

## Build

```sh
go build -o oseram ./cmd/oseram
```

Or install into your Go bin directory:

```sh
go install ./cmd/oseram
```

## Run

Run the single final/root task inferred from `build.yml`:

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

Flags:

- `--file <path>`: build file to read. Defaults to `build.yml`.
- `--dry-run`: print the execution plan without running tasks.
- `--list`: list available tasks and exit.
- `--verbose`: print captured stdout/stderr and execution details.
- `--help`: show a short command line summary.

Arguments:

- `[task]`: optional task name. If omitted, Oseram runs the single final task that no other task depends on. If multiple final tasks exist, Oseram fails and asks for an explicit task.
