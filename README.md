# Peekdown

**Peekdown** is a lightweight, cross-platform, native Markdown viewer built with Rust, designed specifically for developers who need to read documentation efficiently without bloating their system resources.

Unlike Electron or Tauri-based solutions that consume hundreds of megabytes of RAM, Peekdown is native, fast, and keeps its memory footprint under 100MB, even with multiple documents open.

## 🚀 Features

- **Extreme Performance**: Native Rust application with < 100MB memory usage.
- **Fast Startup**: Cold start in under 1 second.
- **Tabbed Interface**: Open and switch between multiple Markdown files seamlessly.
- **Syntax Highlighting**: Beautiful code block highlighting for Rust, Python, JS, Go, and more.
- **Outline Navigation**: Auto-generated table of contents for quick navigation.
- **In-Document Search**: Fast search with keyword highlighting.
- **Standard Markdown Support**: Renders GFM (GitHub Flavored Markdown) including tables, task lists, and images.

## 🛠️ Technology Stack

- **Language**: Rust 🦀
- **GUI Framework**: [GPUI](https://github.com/zed-industries/gpui) (The high-performance UI framework powering Zed)
- **Markdown Parser**: `pulldown-cmark`
- **Syntax Highlighting**: `syntect`
- **Async Runtime**: `smol`

## 📦 Installation

*(Installation instructions will be added once the first release is ready. Currently under active development.)*

### Build from Source

Requirements:
- Rust (latest stable)
- Cargo

```bash
git clone https://github.com/yourusername/peekdown.git
cd peekdown
cargo run --release
```

## 🎯 Usage

- **Open File**: Drag and drop files into the window, or use `Ctrl+O` (planned).
- **Command Line**: `peekdown README.md`
- **Tabs**: Click tabs to switch documents.

## 🗺️ Roadmap

- [x] Basic Window & GPUI Setup
- [x] Markdown Rendering Core
- [ ] Tab System
- [ ] Syntax Highlighting
- [ ] Outline Sidebar
- [ ] Search Functionality
- [ ] Windows Installer / Association

## 📄 License

MIT License