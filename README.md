# modwhy

ASCII **why-tree** for Go module dependencies — a friendlier view of why a module is in your build.

```
example.com/cmd/app
└── github.com/foo/bar@v1.2.3
    └── github.com/target/mod@v0.1.0
```

## Install

```bash
go install github.com/MrBeldum/modwhy@latest
```

Requires Go 1.22+ and a working module (`go.mod`).

## Usage

Run from inside a Go module (or pass `-C`):

```bash
modwhy github.com/some/dependency
modwhy -m github.com/some/dependency
modwhy -json -max 3 golang.org/x/sys
modwhy -C /path/to/module github.com/foo/bar
```

### Flags

| Flag | Description |
|------|-------------|
| `-C <dir>` | Module root (default `.`) |
| `-m` | Treat argument as module path only |
| `-json` | Emit JSON on stdout (ASCII tree is the default) |
| `-max <n>` | Max paths to show (default `5`) |

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | At least one path found |
| `1` | Target not in the module graph / no path |
| `2` | Usage / flag / no `go.mod` error |

## How it works

`modwhy` runs `go mod graph`, builds reverse-friendly edges, and DFS-walks from the main module to the target, printing up to `-max` distinct ASCII trees (versions preserved). Package arguments are best-effort resolved to a module via `go list` when `-m` is not set.

## License

MIT — Copyright (c) 2026 Daniel Bae
