# linkwise

The Linkwise command line interface: your Linkwise library, from the terminal.

This is Task 1 of a larger build. Today the CLI only knows its own version.
Every later task adds packages and commands to this same binary, wrapping the
Linkwise public HTTP API for reading, saving and searching your library
without leaving a terminal.

## Install

None of these are published yet. They will work starting with the release
task later in the build; until then, build from source (see below).

```bash
brew install LinkwiseApp/tap/linkwise
```

```bash
npm install -g @linkwise/cli
```

```bash
curl -fsSL https://linkwise.app/install.sh | sh
```

## Build from source

```bash
go build -o linkwise ./cmd/linkwise
./linkwise --version
```

## Documentation

Full command reference, authentication and exit codes: https://linkwise.app/developers/cli
