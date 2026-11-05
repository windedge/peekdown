# Peekdown

A lightweight, native Markdown viewer. This is the Go rewrite on [mygo](https://github.com/egoist/mygo)'s native UI: a single Go binary, no webview, no frontend build.

> Status: scaffolding. The reading features are being ported from the Rust implementation, [windedge/peekdown-rust](https://github.com/windedge/peekdown-rust). Work is tracked in the [rewrite map](https://github.com/windedge/peekdown/issues/1).

## Features

The v0.3.0 target is parity with the Rust implementation:

- Fast cold startup and a single-instance app.
- Tabs, a Files / Outline sidebar, and recent files.
- GitHub Flavored Markdown, code highlighting, Mermaid diagrams, embedded HTML, and YAML frontmatter.
- In-document search with regular expressions and case matching.
- Light / dark / system themes, fonts, zoom, and layout settings.
- Auto-refresh when files change, plus Windows file association for `.md` and `.markdown`.
- The existing `%APPDATA%\peekdown\config.toml` keeps working.

## Requirements

- Go 1.27.3 or later.
- No Bun, no WebView2, no C compiler.

## Develop

```sh
go tool mygo dev
```

## Test

```sh
go test ./...
```

## Build

```sh
go tool mygo build
```

Builds land in `build/<os>-<arch>/`:

- Windows: `Peekdown.exe` and `peekdown-setup-<version>-amd64.exe`.
- macOS: `Peekdown.app` and `peekdown-<version>.dmg`.
- Linux: `peekdown`, the Debian package, and the archive.

Release builds use:

```sh
go build -trimpath -tags mygo_noinspector \
  -ldflags "-s -w -H=windowsgui -X github.com/egoist/mygo.production=1"
```

## Configuration

The app reads `%APPDATA%\peekdown\config.toml` on Windows, `~/.config/peekdown/config.toml` on Linux, and `~/Library/Application Support/peekdown/config.toml` on macOS. The file format is being carried over from the Rust implementation.

## Release

See [docs/release.md](docs/release.md).

## License

MIT. See [LICENSE](LICENSE).
