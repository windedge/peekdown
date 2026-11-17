# Peekdown

A Markdown reader with a native UI. One Go binary on [mygo](https://github.com/egoist/mygo): no webview, no Node, no frontend build. Reads `.md` and `.markdown` files in tabs.

## Features

**Rendering**

- GitHub Flavored Markdown: tables with grid lines, per-column alignment and a tinted header band; task-list checkboxes drawn as boxes and checks; footnotes; block quotes.
- Syntax-highlighted code blocks with a copy button and horizontal scrolling for long lines.
- Mermaid diagrams rendered with the app's theme (2x for high-DPI); a failed diagram shows an error panel over the source instead of dying; wide diagrams scroll sideways.
- Embedded HTML rendered semantically: tables, images, headings, inline bold/italic/code/links survive; `script`, `style`, `iframe` and 20-odd other tags are pruned.
- Images: relative paths resolve against the document, explicit `width`/`height` are honoured, oversized ones scale to fit, and an image that is itself a link is clickable.
- Frontmatter shown as a key-value card with copy buttons and a collapse toggle; TOML frontmatter and zh frontmatter keys display as-is.
- Math syntax stays as literal, selectable text (LaTeX typesetting is not implemented).

**Reading**

- Tabs; drag selection with double-click for a word and triple-click for a line; `Ctrl+C` / `Ctrl+A` copy the document's own selection.
- Search (`Ctrl+F`): plain text or regular expression, case toggle, hit counter, `F3` / `Shift+F3` navigation, closes with `Esc`; invalid patterns say so instead of failing silently.
- Outline sidebar: click an entry to jump, the current section follows the scroll position.
- Files sidebar: expandable tree of the document's directory, sorted by name or mtime, with the root optionally snapped to the nearest project root (detected via `Cargo.toml`, `package.json`, `pyproject.toml`, `go.mod`, and friends, configurable).
- A 2px reading-progress bar at the top of the document area, plus jump-to-top and jump-to-bottom buttons in the corner that hide at the extremes.
- Text zoom from 50% to 300% in 5% steps; `Ctrl+0` resets to 100. The chrome keeps its size; the status bar shows the percentage.
- Auto refresh: an open file rewritten by another program reloads in place, keeping the reading position; off in settings.
- Recent files: the welcome screen lists the last ten, with the parent directory under each name.

**Integration**

- Single instance: opening a file again forwards it to the running window and activates it.
- Clickable links: `http`/`https`/`mailto` open in the browser, `#anchor` links scroll to the heading, unsafe schemes (`javascript:` and friends) open nothing.
- File links and drag & drop open documents in tabs; opening a file that is already open focuses its tab instead of duplicating it (case-insensitive on Windows).
- Themes follow the system, or light/dark in settings; language is English or 简体中文.
- Settings in `settings.json`, edited through the dialog (`Ctrl+,`).

## Install

Grab an installer or archive from [Releases](https://github.com/windedge/peekdown/releases), or via Scoop:

```sh
scoop bucket add ladle https://github.com/windedge/ladle-bucket
scoop install ladle/peekdown
```

The NSIS installer registers the `.md` and `.markdown` file associations. Scoop installs a zip that skips them; if you run the portable exe and want the associations, register them once yourself:

```sh
Peekdown.exe --register
```

## Keyboard shortcuts

| Shortcut | Action |
| --- | --- |
| `Ctrl+O` | Open a file |
| `Ctrl+W`, `Ctrl+F4` | Close the current tab |
| `Ctrl+Tab`, `Ctrl+Shift+Tab` | Next tab, previous tab |
| `Ctrl+C` / `Ctrl+A` | Copy selection / select all |
| `Ctrl+F` | Find |
| `F3`, `Shift+F3` | Next match, previous match |
| `Esc` | Close the search bar |
| `Ctrl+Home`, `Ctrl+End` | Scroll to the top, the bottom |
| `Ctrl+B` | Toggle the outline sidebar |
| `Ctrl+Shift+E` | Toggle the files sidebar |
| `F5`, `Ctrl+R` | Reread the document from disk |
| `Ctrl+=`, `Ctrl++`, `Ctrl+-`, `Ctrl+0` | Zoom in, zoom in, zoom out, reset to 100% |
| `Ctrl+,` | Settings |
| `Ctrl+Q` | Quit |

Right-clicking the document area: Copy (greyed out with no selection), Select All, Find, Refresh, Reveal in Sidebar, Open in File Manager, Copy Path. Right-clicking a tab: Reveal in Sidebar, Open in File Manager, Copy File Path, Close Tab, Close Others, Close All.

Every chord above is registered by the menu bar or, for the second chord of an action (`Ctrl+F4`, `Ctrl+R`, `Ctrl++`), by the window; on macOS `CmdOrCtrl` means `Cmd`.

## Limitations

- Frontmatter cards' text is copyable but not selectable with the mouse; long list/map values collapse to one line past 120 characters.
- Mermaid CJK labels are only font-injected on Windows; on macOS and Linux they may render as boxes.
- Linux shows both the native menu bar and the in-window menu button (double menus).
- Tables wider than the reading column are cut off; code blocks and Mermaid diagrams do scroll sideways.
- Open failures are silent (a log line, no toast).

## Build from source

Requires Go 1.27.3 or later. No WebView2, no C compiler.

```sh
go tool mygo dev    # run it
go test ./...       # test it
go tool mygo build  # build it; lands in build/<os>-<arch>/
```

Release builds on Windows add the file-association flag so `OnOpenFile` accepts paths:

```sh
go build -trimpath -tags mygo_noinspector \
  -ldflags "-s -w -H=windowsgui -X github.com/egoist/mygo.production=1 -X github.com/egoist/mygo.packageFileExtensions=md,markdown"
```

A plain `go build` without that flag ignores file arguments. See [docs/release.md](docs/release.md) for packaging.

## Configuration

Settings live in `settings.json` in the app's data directory: `%AppData%\Peekdown` on Windows, `~/Library/Application Support/Peekdown` on macOS, `$XDG_CONFIG_HOME/Peekdown` (or `~/.config/Peekdown`) on Linux. Window position, size and maximized state are kept by mygo next to it. The Rust version's `config.toml` is not read or migrated.

## License

MIT. See [LICENSE](LICENSE).
