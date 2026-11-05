# Peekdown

A lightweight, native Markdown viewer. This is the Go rewrite on [mygo](https://github.com/egoist/mygo)'s native UI: a single Go binary, no webview, no frontend build.

> Status: scaffolding. The reading features are being ported from the Rust implementation, [windedge/peekdown-rust](https://github.com/windedge/peekdown-rust). Work is tracked in the [rewrite map](https://github.com/windedge/peekdown/issues/1). The Windows installer already registers the `.md` and `.markdown` associations; opening a file from the system arrives with the document view.

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

Release builds on Windows use:

```sh
go build -trimpath -tags mygo_noinspector \
  -ldflags "-s -w -H=windowsgui -X github.com/egoist/mygo.production=1"
```

On macOS and Linux, leave out `-H=windowsgui`. `go tool mygo build` applies these flags itself.

## Configuration

Settings live in `settings.json` in the app's user data directory: `%AppData%\Peekdown\settings.json` on Windows, `~/.config/Peekdown/settings.json` on Linux, and `~/Library/Application Support/Peekdown/settings.json` on macOS. The Rust version's `config.toml` is not read or migrated.

Window position, size and maximized state are kept by mygo next to it, and recent documents by the system and mygo's own list of ten. File associations come from the installer or the macOS app bundle; there is no `--register` flag.

## Release

See [docs/release.md](docs/release.md).

## License

MIT. See [LICENSE](LICENSE).
