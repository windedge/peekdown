//! Global cache for SyntaxHighlighter instances with LRU eviction.
//!
//! This module provides a thread-safe cache to reuse SyntaxHighlighter
//! instances across multiple code blocks, avoiding the expensive cost of
//! creating new Parser and Query objects for each code block.
//! The cache uses LRU (Least Recently Used) eviction to bound memory usage.

use std::{
    num::NonZeroUsize,
    sync::{LazyLock, Mutex},
};

use gpui::{HighlightStyle, SharedString};
use gpui_component::{
    highlighter::{HighlightTheme, SyntaxHighlighter},
    input::Rope,
};
use lru::LruCache;

/// Maximum number of SyntaxHighlighter instances to cache.
/// Each instance holds a tree-sitter Parser + Query objects,
/// so keeping this moderate bounds memory usage.
const CACHE_SIZE: usize = 20;

/// Global LRU cache for SyntaxHighlighter instances, keyed by language name.
/// This is shared across all tabs and documents.
/// When the cache exceeds CACHE_SIZE, the least recently used entry is evicted.
static HIGHLIGHTER_CACHE: LazyLock<Mutex<LruCache<SharedString, SyntaxHighlighter>>> =
    LazyLock::new(|| Mutex::new(LruCache::new(NonZeroUsize::new(CACHE_SIZE).unwrap())));

/// Highlight code using cached SyntaxHighlighter.
///
/// This function reuses existing SyntaxHighlighter instances for the same language,
/// which avoids the expensive cost of creating new Parser and Query objects.
/// The cache entry is promoted as most recently used on each access.
pub(crate) fn highlight_code(
    code: &str,
    lang: &SharedString,
    highlight_theme: &HighlightTheme,
) -> Vec<(std::ops::Range<usize>, HighlightStyle)> {
    let mut cache = HIGHLIGHTER_CACHE.lock().unwrap();

    // Get existing highlighter or create a new one
    let highlighter = if let Some(h) = cache.get_mut(lang) {
        // Cache hit: h is promoted to most recently used automatically
        h
    } else {
        // Cache miss: create a new highlighter and insert it
        cache.put(lang.clone(), SyntaxHighlighter::new(lang.as_ref()));
        cache.get_mut(lang).unwrap()
    };

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
