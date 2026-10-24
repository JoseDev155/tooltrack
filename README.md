# tooltrack

[Español](README.es.md)

A cross-platform terminal tool written in Go that detects the installed version of your development tools, checks the latest release, and updates them for you.

> **Status:** early development. There are no published releases yet; build it from source (see below). The full technical specification is in [spec/spec.md](spec/spec.md) (in Spanish).

## Features

- Detects the local version of `uv`, `bun`, `deno`, `rustup`, `fnm`, `conda` and `gh`.
- Checks the latest version through the GitHub Releases API (or the tool's own command).
- Interactive TUI (Bubble Tea): dashboard, update all, pick tools to update or to exclude, live progress, final summary.
- Finds tools outside `PATH` (for example `conda.bat` on Windows or `~/miniconda3/bin/conda`).
- Update commands that depend on the operating system (`winget`, `brew`, `cargo`, native updaters).
- Output of every update goes to a log file, never over the interface.
- Spanish and English interface.

Targets: Windows 11 (x86_64), Linux (x86_64), macOS (x86_64 and arm64). CLI binaries are static (`CGO_ENABLED=0`).

## Usage

```sh
go run ./cmd/cli          # from source
./tooltrack               # after building
./tooltrack --version
./tooltrack --home DIR    # keep data/ and logs/ in DIR
```

The program opens a menu (view tool status, preferences, exit). Versions are looked up when you choose **View tool status**, which shows the dashboard; press `Enter` there for the actions (update all, pick tools to update or to exclude).

| Key | Action |
| --- | --- |
| `↑` / `k`, `↓` / `j` | Move |
| `Space` | Toggle a checkbox, or change a preference |
| `Enter` | Confirm |
| `Esc` | Go back |
| `r` | Check again (dashboard) |
| `l` | Open the log (after a failed update) |
| `q` / `Ctrl+C` | Quit (cancels a running update first) |

## Configuration

Files are created on first run in your user config directory (`%APPDATA%\tooltrack`, `~/.config/tooltrack`, `~/Library/Application Support/tooltrack`). Set `TOOLTRACK_HOME` or pass `--home` to use another directory.

```text
data/config.json   preferences
data/tools.json    tool catalog (your entries win over the built-in presets)
logs/updater.log   output of every update, rotated at 5 MB
```

`config.json` fields: `language` (`es` | `en`), `default_action` (`ask` | `update_all` | `update_selected` | `exclude_selected`), `github_token`, `max_concurrent_checks` and `timeout_seconds`. A GitHub personal access token is optional and raises the API limit from 60 to 5,000 requests per hour; the `GITHUB_TOKEN` environment variable is used when the file leaves it empty.

## Build

Requires Go 1.24 or newer.

```sh
make check        # gofmt, go vet and go test
make cli-all      # static binaries for all platforms in dist/
make cli-linux    # or cli-windows, cli-darwin-amd64, cli-darwin-arm64
scripts/build.sh  # same, without make (scripts/build.ps1 on Windows)
```

## Contributing

Work is merged into the `develop` branch; `main` is not touched directly. See [CLAUDE.md](CLAUDE.md) for the conventions used by contributors and coding agents.

## License

MIT. See [LICENSE](LICENSE).
