//! Mermaid diagram rendering using external CLI.
//!
//! This module checks if `mmdc` (mermaid-cli) is available on the system and
//! can render Mermaid diagram source code into SVG files for display.

use std::path::{Path, PathBuf};
use std::process::Stdio;

use anyhow::Result;
use smol::io::{AsyncReadExt, AsyncWriteExt};
use smol::process::Command;

/// Mermaid diagram renderer using the external `mmdc` CLI tool.
pub struct MermaidRenderer;

impl MermaidRenderer {
    /// Check if mermaid-cli (`mmdc`) is available on the system.
    ///
    /// Runs `mmdc --version` and returns `true` if the command succeeds.
    pub fn is_available() -> bool {
        std::process::Command::new("mmdc")
            .arg("--version")
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status()
            .map(|s| s.success())
            .unwrap_or(false)
    }

    /// Render mermaid source code to an SVG string (async).
    ///
    /// Uses `mmdc` with stdin/stdout piping: reads source from stdin,
    /// produces SVG on stdout.
    ///
    /// Returns the SVG content as a string on success.
    pub async fn render_to_svg(source: &str) -> Result<String> {
        let mut child = Command::new("mmdc")
            .args(["-i", "-", "-o", "-", "--svg"])
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()?;

        // Write source to stdin and close it to signal EOF
        if let Some(mut stdin) = child.stdin.take() {
            stdin.write_all(source.as_bytes()).await?;
            drop(stdin);
        }

        // Read stdout (SVG content)
        let mut svg = String::new();
        if let Some(mut stdout) = child.stdout.take() {
            stdout.read_to_string(&mut svg).await?;
        }

        // Wait for process to finish and check exit status
        let status = child.status().await?;
        if !status.success() {
            anyhow::bail!("mermaid-cli (mmdc) exited with status: {}", status);
        }

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
        .filter(|e| e.path().extension().map_or(false, |ext| ext == "svg"))
        .filter_map(|e| {
            let meta = e.metadata().ok()?;
            let modified = meta.modified().ok()?;
            Some((e.path(), modified))
        })
        .collect();

    if entries.len() > max_files {
        // Sort by modification time, newest first
        entries.sort_by(|a, b| b.1.cmp(&a.1));
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
