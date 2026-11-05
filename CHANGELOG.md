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
- CI on Windows, macOS and Linux, and a release workflow that attaches the Windows archive, installer, and macOS app and disk image to a `v*` tag.
- Scoop manifest template and release notes.

The entries below describe the Rust implementation, kept for history.

## [0.2.0] - 2026-07-21

Fixed window state restoration when closing while maximized, copying only the selected text, scroll memory and idle CPU regressions, Mermaid memory use, diagram layout, outline flicker, and scroll position when diagrams finish rendering.

## [0.1.0] - 2026-06-14

First release of the Rust implementation: fast startup, tabs, GFM rendering, syntax highlighting, Mermaid diagrams, embedded HTML, frontmatter, outline and file explorer sidebars, in-document search, appearance settings, file watching, single-instance IPC, Windows file association, welcome screen, and TOML configuration.
