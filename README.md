# linkwise

Your Linkwise library, from the terminal. Save, search, read and export
without leaving the shell.

A single static binary with no runtime to install alongside it. Every command
is a thin wrapper over one endpoint of the Linkwise public API, so anything the
CLI can do, a script can do too.

## Install

```bash
brew install linkwiseapp/tap/linkwise
```

```bash
npm install -g @linkwise/cli
```

```bash
curl -fsSL https://linkwise.app/install.sh | sh
```

The shell installer and the npm package both verify the release checksum
before unpacking anything. If it does not match, nothing is written.

## Get started

```bash
linkwise auth login          # paste a key from linkwise.app/developers/dashboard
linkwise save https://example.com --description "why I saved it"
linkwise ls --limit 5
linkwise search postgres
linkwise read <id> > piece.md
```

The key is checked against the API before it is stored, so a mistyped one
fails at the prompt rather than on your next command. It is kept in the system
keychain; on a machine without one the CLI writes it to a 0600 file and says
so.

## Output

A table when stdout is a terminal, NDJSON when it is a pipe. Detected rather
than flagged, and `--json` forces it either way.

```bash
linkwise ls --limit 3                              # a table
linkwise ls --limit 3 | jq -r '.url'               # NDJSON
linkwise ls --collection reading --json > out.json
```

## Exit codes

Distinct per failure class, so a script can tell an expired key from a missing
link without parsing stderr.

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Unclassified, including a network failure |
| 2 | Bad flags or a missing argument. Nothing was sent |
| 3 | No key, or it is invalid, expired or revoked |
| 4 | The key lacks the scope, or API access is disabled |
| 5 | No such link, collection or tag |
| 6 | Rate limited. The CLI already waited and retried once |
| 7 | A quota is exhausted, or the endpoint is Pro only |

## Configuration

`~/.config/linkwise/config.toml`, with settings resolved in this order, first
one wins: a command flag, then the environment, then the config file, then the
default.

| Variable | Meaning |
|---|---|
| `LINKWISE_TOKEN` | Personal access token. Overrides the keychain, so CI never picks up a developer's key |
| `LINKWISE_API` | Base URL, for pointing at another deploy |
| `LINKWISE_PROFILE` | Which profile to use, for multiple accounts |
| `NO_COLOR` | Any value disables colour, per no-color.org |

## Build from source

```bash
go build -o linkwise ./cmd/linkwise
./linkwise --version
```

`scripts/smoke.sh` runs the surface a fake server cannot prove (the keychain,
tty detection, real exit codes) against the live API. It needs
`LINKWISE_TOKEN` set to a read-and-write key, and it saves and deletes one
link.

## Documentation

Full command reference, authentication and exit codes:
https://linkwise.app/developers/cli

## Licence

MIT.
