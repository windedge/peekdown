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
- Embedded HTML blocks rendered semantically instead of as raw source: block-level tables, images, rules, preformatted code, headings and paragraphs, containers expanded in place, inline styles (bold, italic, underline, strikethrough, code, safe links) mapped to the reader's spans, and a whitelist that prunes script, style, iframe, form and friends; unknown tags degrade to their text.
- Auto refresh (default on): the reader watches every open file through mygo's file watcher and reloads a document in place when another app writes it, keeping the reading position and dropping the search; the `auto_refresh` setting's switch and the settings dialog's Save start and stop the watching.
- Edit menu with Copy (`Ctrl+C`) and Select All (`Ctrl+A`) over the document's own selection model: Copy writes the selected text to the clipboard, Select All selects the whole document.
- The Explorer settings apply: the file pane starts at the document's directory or, with `Explorer Root = Project Root`, at the nearest directory above it holding a project root marker, and sorts entries by name or modification time, ascending or descending, directories first, as `Explorer Sort` says.
- Math syntax (`$$...$$`, `\(...\)`, `\[...\]`) shown as plain text like the Rust version: documents parse without the math extensions, so the literal characters stay in the runs, searchable and selectable, and unmapped block kinds fall back to their source lines instead of a blank row. LaTeX typesetting is not in v0.3.0.
- The Rust version's remaining shortcuts: `Ctrl+Home` / `Ctrl+End` scroll to the top and the bottom, `F5` and `Ctrl+R` reread the open file even with auto refresh off, `Ctrl+F4` closes the tab besides `Ctrl+W`, `Ctrl+B` toggles the outline and `Ctrl+Shift+E` the files sidebar. Every one is also a menu item, so the key shows beside it.
- Zoom scales the document's text, not the window: `Ctrl+=`, `Ctrl++` and `Ctrl+-` step the text zoom by 5 percent between 50 and 300, `Ctrl+0` (Reset Zoom) returns to 100, and the status bar reads the current percentage. The settings' `font_size` and `mono_font_size` are the 100 percent base, so headings, tables, code and diagrams grow with the text, while the chrome keeps its size. The zoom lives in the session only and never touches `settings.json`.
- The document area's context menu: Copy (greyed out with no selection), Select All, Find, Refresh, Reveal in Sidebar, Open in File Manager and Copy Path, all acting on the open document wherever the click landed; the tab strip's menu gains Open in File Manager, which reveals the file through mygo's system shell (`explorer.exe /select,` on Windows, Finder's reveal on macOS, the file manager's `ShowItems` on Linux).
- The Files sidebar is a real tree: folders expand and collapse in place, a directory's listing is read once and cached, the expanded set persists in `settings.json`, and the pane honours `Explorer Root` (document directory or project root), `Explorer Sort` (name or time, either direction, directories first) and the project root markers. Compiled classes: only Markdown files and their folders are listed, dotfiles skipped.
- Links work as in the Rust reader: `http`, `https` and `mailto` destinations open in the system browser, `#anchor` links and the outline's entries scroll the matching heading into view (the outline follows the reader's scroll position), and unsafe schemes such as `javascript:` open nothing. The outline's entries are clickable and their hover and current-heading highlight track the document.
- Every code block carries a copy button in its panel's top-right corner: it copies the block's source, newlines and indentation intact, and shows a check with `Copied` for two seconds. Lines longer than the reading column scroll sideways inside the panel instead of wrapping, so the code keeps its shape; the horizontal bar hugs the panel's bottom edge and the last line stays above it.
- Tables render as a grid: rules between every row and column, a tinted header band, and each column laid out at its own measured width and aligned as the delimiter row says (`:---`, `:---:` and `---:`). An oversized table is split into consecutive slices of 64 rows, each repeating the header, so a table of a thousand rows stays scrollable instead of stalling the frame.
- Typography follows the settings: `font_family` and `mono_font_family` apply to the body and the code, `font_size` and `mono_font_size` set the scale every heading, table, code block and diagram rides, and inline code draws as a tinted monospaced chip. The search bar is now a floating panel over the document's top-right corner instead of a row that pushed the text down on every open.
- Opening a file that is already in a tab focuses that tab instead of adding a second copy (Windows folds the path's case, macOS and Linux compare it exactly); closing tabs drives the closed documents' memory back to the OS, a big document's heap freed in one scavenger run per batch rather than one per tab.

### Fixed

- A wrapped list item's bullet or number stayed beside the item's middle lines instead of its first line.
- A split table's repeated header cells mapped to the last slice's run, so a search or selection that started in the header highlighted the wrong runes; the header cell now resolves to no run rather than another slice's.
- The overlay scrollbar covered the document's last characters: the rows keep clearance on the right, and the bar hugs the window's edge instead of floating inside the column.
- Long code lines were cut off at the panel's edge; they now scroll sideways inside the block's panel.
- A table's cells always read left and the grid drew no rules between its columns; each column now follows the delimiter row's alignment and the rows and columns carry grid lines.

The entries below describe the Rust implementation, kept for history.

## [0.2.0] - 2026-07-21

Fixed window state restoration when closing while maximized, copying only the selected text, scroll memory and idle CPU regressions, Mermaid memory use, diagram layout, outline flicker, and scroll position when diagrams finish rendering.

## [0.1.0] - 2026-06-14

First release of the Rust implementation: fast startup, tabs, GFM rendering, syntax highlighting, Mermaid diagrams, embedded HTML, frontmatter, outline and file explorer sidebars, in-document search, appearance settings, file watching, single-instance IPC, Windows file association, welcome screen, and TOML configuration.
