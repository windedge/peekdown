# Peekdown

A lightweight, native Markdown viewer. This is the Go rewrite on [mygo](https://github.com/egoist/mygo)'s native UI: a single Go binary, no webview, no frontend build.

> Status: scaffolding. The reading features are being ported from the Rust implementation, [windedge/peekdown-rust](https://github.com/windedge/peekdown-rust). Work is tracked in the [rewrite map](https://github.com/windedge/peekdown/issues/1). Opening a file from the command line or a second instance already updates the window title and shows its path; rendering arrives with the document view.

## Features

The v0.3.0 target is parity with the Rust implementation:

- Fast cold startup and a single-instance app.
- Tabs, a Files / Outline sidebar, and recent files.
- GitHub Flavored Markdown, code highlighting, Mermaid diagrams, embedded HTML, and YAML frontmatter.
- In-document search with regular expressions and case matching.
- Light / dark / system themes, fonts, zoom, and layout settings.
- Auto-refresh when files change, plus Windows file association for `.md` and `.markdown`.
- Settings in `settings.json` under the app's user data directory.

## Keyboard Shortcuts

The native menu registers every chord, so each one is also listed there with its key.

| Shortcut | Action |
| --- | --- |
| `Ctrl+O` | Open a file |
| `Ctrl+W`, `Ctrl+F4` | Close the current tab |
| `Ctrl+Tab`, `Ctrl+Shift+Tab` | Next tab, previous tab |
| `Ctrl+C` | Copy the selected text |
| `Ctrl+A` | Select the whole document |
| `Ctrl+F` | Find |
| `F3`, `Shift+F3` | Next match, previous match |
| `Ctrl+Home`, `Ctrl+End` | Scroll to the top, the bottom |
| `Ctrl+B` | Toggle the outline sidebar |
| `Ctrl+Shift+E` | Toggle the files sidebar |
| `F5`, `Ctrl+R` | Reread the document from disk |
| `Ctrl+=`, `Ctrl++`, `Ctrl+-`, `Ctrl+0` | Zoom the text in, out, back to 100% |
| `Ctrl+,` | Settings |

On macOS the chords the menu registers as `CmdOrCtrl` use `Cmd`: `Cmd+O`, `Cmd+W`, `Cmd+F`, `Cmd+Home`, `Cmd+B` and the rest. `Ctrl+F4`, `Ctrl+R` and the function keys keep their keys there.

Some actions answer to two chords (`Close Tab`: `Ctrl+W` and `Ctrl+F4`, `Refresh`: `F5` and `Ctrl+R`, `Zoom In`: `Ctrl+=` and `Ctrl++`). The menu shows the first one; the second rides mygo's window shortcuts, so the View menu lists each action once.

Right-clicking the document area opens its menu: Copy (greyed out with no selection), Select All, Find, Refresh, Reveal in Sidebar, Open in File Manager and Copy Path. Right-clicking a tab opens Reveal in Sidebar, Open in File Manager, Copy File Path and the close commands.

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
  -ldflags "-s -w -H=windowsgui -X github.com/egoist/mygo.production=1 -X github.com/egoist/mygo.packageFileExtensions=md,markdown"
```

On macOS and Linux, leave out `-H=windowsgui`. `go tool mygo build` applies these flags itself, including the file extensions that let `OnOpenFile` accept paths; a plain `go build` without the `packageFileExtensions` flag ignores file arguments.

## Configuration

Settings live in `settings.json` in the app's user data directory: `%AppData%\Peekdown\settings.json` on Windows, `$XDG_CONFIG_HOME/Peekdown/settings.json` (or `~/.config/Peekdown/settings.json`) on Linux, and `~/Library/Application Support/Peekdown/settings.json` on macOS. The Rust version's `config.toml` is not read or migrated.

Window position, size and maximized state are kept by mygo next to it, and recent documents by the system and mygo's own list of ten. File associations come from the installer or the macOS app bundle; there is no `--register` flag.

## Release

See [docs/release.md](docs/release.md).

## License

MIT. See [LICENSE](LICENSE).
