//! Mermaid diagram rendering using mermaid-rs-renderer library.
//!
//! Uses the pure Rust `mermaid-rs-renderer` crate to render Mermaid diagram
//! source code into SVG files for display, without requiring Node.js or mmdc CLI.

use std::path::{Path, PathBuf};

use anyhow::Result;

/// Mermaid diagram renderer using the `mermaid-rs-renderer` library.
pub struct MermaidRenderer;

impl MermaidRenderer {
    /// Check if mermaid rendering is available.
    ///
    /// Always returns `true` because rendering is built into the application.
    #[allow(dead_code)]
    pub fn is_available() -> bool {
        true
    }

    /// Render mermaid source code to an SVG string (async).
    ///
    /// Uses `mermaid-rs-renderer` via `smol::unblock` to avoid blocking the UI thread.
    ///
    /// Returns the SVG content as a string on success.
    pub async fn render_to_svg(source: &str) -> Result<String> {
        let source = source.to_string();
        let svg = smol::unblock(move || mermaid_rs_renderer::render(&source)).await?;
        Ok(svg)
    }

    /// Render mermaid source code to a temporary SVG file and return its path.
    ///
    /// The file is cached by content hash: if the same source has been rendered
    /// before, the existing file is reused.
    pub async fn render_to_file(source: &str) -> Result<PathBuf> {
        let svg = Self::render_to_svg(source).await?;
        let hash = content_hash(source);
        let dir = std::env::temp_dir().join("peekdown-mermaid");
        std::fs::create_dir_all(&dir).ok();
        let path = dir.join(format!("mermaid-{:016x}.svg", hash));

        // Only write if file doesn't exist yet (caching)
        if !path.exists() {
            std::fs::write(&path, svg.as_bytes())?;
        }

        // Clean up old cache, keeping the most recent 100 SVG files
        cleanup_old_cache(&dir, 100);

        Ok(path)
    }
}

/// Clean up old cached SVG files, keeping only the most recent `max_files`.
fn cleanup_old_cache(dir: &Path, max_files: usize) {
    let mut entries: Vec<_> = std::fs::read_dir(dir)
        .ok()
        .into_iter()
        .flatten()
        .filter_map(|e| e.ok())
        .filter(|e| e.path().extension().is_some_and(|ext| ext == "svg"))
        .filter_map(|e| {
            let meta = e.metadata().ok()?;
            let modified = meta.modified().ok()?;
            Some((e.path(), modified))
        })
        .collect();

    if entries.len() > max_files {
        // Sort by modification time, newest first
        entries.sort_by_key(|b| std::cmp::Reverse(b.1));
        for (path, _) in entries.drain(max_files..) {
            let _ = std::fs::remove_file(&path);
        }
    }
}

/// Compute a simple hash of the source content for caching.
fn content_hash(s: &str) -> u64 {
    use std::hash::{Hash, Hasher};
    let mut hasher = std::collections::hash_map::DefaultHasher::new();
    s.hash(&mut hasher);
    hasher.finish()
}
