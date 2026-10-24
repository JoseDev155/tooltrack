# tooltrack

[Español](README.es.md)

A cross-platform terminal tool written in Go that detects the installed version of your development tools, checks the latest release, and updates them for you.

> **Status:** early development. Nothing below is released yet; this README describes the planned scope.

## Planned features

- Detects local versions of `uv`, `bun`, `deno`, `rustup`, `fnm`, `conda` and `gh`.
- Checks the latest version through the GitHub Releases API or a tool's own command.
- Interactive TUI (Bubble Tea): dashboard, select tools to update or exclude, live progress.
- Finds tools outside `$PATH` (for example `conda.bat` on Windows).
- English and Spanish interface.
- Optional desktop GUI (Wails) in a later phase.

Targets: Windows 11 (x86_64), Linux (x86_64), macOS (x86_64 and arm64). CLI binaries are static (`CGO_ENABLED=0`).

## Configuration

Settings live in `config.json` (`language`, `default_action`, `github_token`, `max_concurrent_checks`, `timeout_seconds`). A GitHub personal access token is optional and raises the API limit from 60 to 5,000 requests per hour.

## Build

Requires Go 1.24 or newer.

```sh
go build ./...
go test ./...
```

## Contributing

Work is merged into the `develop` branch; `main` is not touched directly. See [CLAUDE.md](CLAUDE.md) for the conventions used by contributors and coding agents.

## License

MIT. See [LICENSE](LICENSE).
