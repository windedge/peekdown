# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.3.0] - Unreleased

The Go rewrite on mygo's native UI. The reading features of the Rust implementation are being ported; see the [rewrite map](https://github.com/windedge/peekdown/issues/1).

### Added

- Go module scaffolded on mygo `977ea57e`, pinned in `go.mod` with the CLI as a tool.
- Native window and a view tested without a window through mygo's headless tester.
- App metadata for `Peekdown`: identifier `com.windedge.peekdown`, version `0.3.0`, and the `.md` / `.markdown` file associations.
- In-document search (`Ctrl+F`): plain text and regular expressions, a case toggle, a hit counter, `Enter` / `F3` and `Shift+Enter` / `Shift+F3` navigation, `Esc` to close, and the first 10000 matches at most.
- Drag & drop: dropping Markdown files on the window opens them as tabs, other files are skipped.
- Image loading: relative images resolve against the document's directory and draw once loaded, the description showing until then.
- The layout setting applies: centered caps the reading column at 900 DIPs, full width follows the window; the saved sidebar tab is restored at startup.
- CI on Windows, macOS and Linux, and a release workflow that attaches the Windows archive, installer, and macOS app and disk image to a `v*` tag.
- Scoop manifest template and release notes.
- Mermaid diagram rendering: flowchart, sequence, pie and other diagram types, light/dark theme following the app, 2x rendering for high-DPI displays, an LRU render cache shared across tabs, and a fallback to the source block with a red error line on syntax errors. CJK labels render with system fonts (Microsoft YaHei on Windows); the rasterizer is owned locally (vendored from go-mermaid, MIT).

### Fixed

- A wrapped list item's bullet or number stayed beside the item's middle lines instead of its first line.

The entries below describe the Rust implementation, kept for history.

## [0.2.0] - 2026-07-21

Fixed window state restoration when closing while maximized, copying only the selected text, scroll memory and idle CPU regressions, Mermaid memory use, diagram layout, outline flicker, and scroll position when diagrams finish rendering.

## [0.1.0] - 2026-06-14

First release of the Rust implementation: fast startup, tabs, GFM rendering, syntax highlighting, Mermaid diagrams, embedded HTML, frontmatter, outline and file explorer sidebars, in-document search, appearance settings, file watching, single-instance IPC, Windows file association, welcome screen, and TOML configuration.
