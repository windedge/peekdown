use gpui::*;
use std::path::PathBuf;

use crate::state::frontmatter::Frontmatter;

pub struct Document {
    pub content: SharedString,
    pub path: PathBuf,
    #[allow(dead_code)] // Exposed for future use by metadata panel
    pub frontmatter: Option<Frontmatter>,
}

impl Document {
    pub fn new(content: String, path: PathBuf) -> Self {
        let frontmatter = extract_frontmatter(&content);
        Self {
            content: content.into(),
            path,
            frontmatter,
        }
    }
}

/// Extract and parse YAML frontmatter from a Markdown source.
///
/// Expects the content to start with `---` on its own line, followed by YAML,
/// then a closing `---` on its own line.
fn extract_frontmatter(content: &str) -> Option<Frontmatter> {
    let content = content.trim_start();
    let mut lines = content.lines();

    // Must start with `---` on its own line
    if lines.next()?.trim() != "---" {
        return None;
    }

    // Collect lines until closing `---`
    let mut yaml_lines = Vec::new();
    for line in lines {
        if line.trim() == "---" {
            let yaml_text = yaml_lines.join("\n").trim().to_string();
            if yaml_text.is_empty() {
                return None;
            }
            return crate::state::frontmatter::parse(&yaml_text).ok();
        }
        yaml_lines.push(line);
    }

    None
}