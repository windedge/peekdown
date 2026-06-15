# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - 2026-06-14

### Added

- **Instant cold startup** — open Markdown files in under one second.
- **Single-instance IPC** — double-clicking a `.md` file opens it as a new tab in the existing window via Windows named pipes.
- **Tabbed multi-document interface** — open, switch, close tabs with keyboard shortcuts and tab tooltips.
- **GFM Markdown rendering** — tables, task lists, fenced code blocks, strikethrough, and more via `pulldown-cmark`.
- **Code syntax highlighting** — 300+ languages highlighted with `syntect`, configurable theme support.
- **Mermaid diagram rendering** — render `mermaid` code blocks as flowcharts, sequence diagrams, and other diagrams using a native Rust library.
- **Embedded HTML support** — render `<details>`, `<img>`, `<kbd>`, and other inline HTML tags via `html5ever`.
- **YAML frontmatter parsing** — extract and display frontmatter as an interactive property table.
- **Local image display** — render local image files with relative path resolution.
- **Outline sidebar** — navigate between headings with a structured table of contents.
- **File explorer sidebar** — browse Markdown files in the current directory or project root, with sorting by name and modification time.
- **Multi-root explorer** — reveal files in sidebar and support multiple root directories.
- **File watcher auto-refresh** — watch files for external changes and reload automatically.
- **In-document search** — search for text with regex support, highlight all matches, and auto-scroll to the first result.
- **Appearance settings** — light / dark / system theme, custom font family and size, mono font, centered or full-width layout, and window size persistence.
- **Inertia scrolling** — physics-based smooth scrolling with configurable speed and toggle.
- **Keyboard shortcuts** — `Ctrl+O` open, `Ctrl+F` search, `Ctrl+Tab` / `Ctrl+Shift+Tab` tab switching, `Ctrl+W` close tab, `Ctrl++`/`Ctrl+-`/`Ctrl+0` font zoom, `Ctrl+R` refresh, `Home`/`End` scroll to top/bottom.
- **Context menu and text selection** — right-click context menus and mouse-based text selection in rendered documents.
- **Windows file association** — `--register` command to register Peekdown as the default handler for `.md` files.
- **Welcome screen** — display a welcome page with an open-file button when no document is loaded.
- **GPUI-native scrollbar** — custom scrollbar positioned at the window edge with smooth performance.
- **Window size and state persistence** — remember window dimensions, sidebar visibility, and zoom level across sessions.
- **Configurable via TOML** — user settings stored in the platform config directory (`~/.config/peekdown/config.toml`).
