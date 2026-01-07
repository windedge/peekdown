//! Global cache for SyntaxHighlighter instances.
//!
//! This module provides a thread-safe cache to reuse SyntaxHighlighter
//! instances across multiple code blocks, avoiding the expensive cost of
//! creating new Parser and Query objects for each code block.

use std::{
    collections::HashMap,
    sync::{LazyLock, Mutex},
};

use gpui::{HighlightStyle, SharedString};
use gpui_component::{
    highlighter::{HighlightTheme, SyntaxHighlighter},
    input::Rope,
};

/// Global cache for SyntaxHighlighter instances, keyed by language name.
/// This is shared across all tabs and documents.
static HIGHLIGHTER_CACHE: LazyLock<Mutex<HashMap<SharedString, SyntaxHighlighter>>> =
    LazyLock::new(|| Mutex::new(HashMap::new()));

/// Highlight code using cached SyntaxHighlighter.
///
/// This function reuses existing SyntaxHighlighter instances for the same language,
/// which avoids the expensive cost of creating new Parser and Query objects.
pub(crate) fn highlight_code(
    code: &str,
    lang: &SharedString,
    highlight_theme: &HighlightTheme,
) -> Vec<(std::ops::Range<usize>, HighlightStyle)> {
    let mut cache = HIGHLIGHTER_CACHE.lock().unwrap();

    // Get or create highlighter for this language
    let highlighter = cache
        .entry(lang.clone())
        .or_insert_with(|| SyntaxHighlighter::new(lang.as_ref()));

    // Update the highlighter with new code
    let rope = Rope::from_str(code);
    highlighter.update(None, &rope);

    // Get styles
    highlighter.styles(&(0..code.len()), highlight_theme)
}

/// Clear the highlighter cache.
/// This can be used when switching themes or when memory pressure is high.
#[allow(dead_code)]
pub(crate) fn clear_cache() {
    let mut cache = HIGHLIGHTER_CACHE.lock().unwrap();
    cache.clear();
}
