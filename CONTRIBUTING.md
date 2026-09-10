# Contributing

Thanks for your interest in contributing!

## How to Contribute

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## Development Setup

The canonical build and test gate is the Nix flake:

```sh
nix build
nix flake check
```

Or with a plain Go toolchain (1.26+):

```sh
GOEXPERIMENT=jsonv2 go test ./...
```

Lint and format:

```sh
golangci-lint run ./...
dprint check
```

The `nix develop` shell ships Go, `golangci-lint`, and `gopls` with
`GOEXPERIMENT=jsonv2` preset.

## Reporting Issues

Please use GitHub Issues to report bugs or request features.
