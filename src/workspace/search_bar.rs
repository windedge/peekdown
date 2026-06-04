//! Search bar component for document search.

use gpui::*;
use gpui_component::{
    ActiveTheme, Icon, IconName, Selectable, Sizable,
    button::{Button, ButtonVariants},
    input::{Input, InputEvent, InputState},
    h_flex,
};
use regex::RegexBuilder;

/// Maximum number of search matches to collect.
/// Prevents excessive memory/CPU usage with broad regex patterns (e.g `.` or `\w+`).
const MAX_MATCHES: usize = 10_000;

/// A single search match in the document.
#[derive(Debug, Clone)]
pub struct SearchMatch {
    /// The block index where the match was found.
    pub block_index: usize,
    /// The byte range within the source text.
    #[allow(dead_code)] // Reserved for future text highlighting
    pub byte_range: std::ops::Range<usize>,
}

/// Search state containing query and matches.
#[derive(Default)]
pub struct SearchState {
    /// The search query string.
    pub query: String,
    /// All matches found in the document.
    pub matches: Vec<SearchMatch>,
    /// Index of the currently focused match.
    pub current_match: usize,
    /// Whether to use regex mode.
    pub is_regex: bool,
    /// Whether to use case-sensitive search.
    pub is_case_sensitive: bool,
}

impl SearchState {
    /// Perform search in the source text with current mode settings.
    /// Supports 4 modes: regex/case-sensitive, regex/case-insensitive,
    /// plain/case-sensitive, plain/case-insensitive.
    pub fn search(&mut self, source: &str, query: &str, block_spans: &[(usize, std::ops::Range<usize>)]) {
        self.query = query.to_string();
        self.matches.clear();
        self.current_match = 0;

        if query.is_empty() {
            return;
        }

        // Binary search: block_spans are sorted by span.start (from document order).
        // O(log n) instead of O(n) linear scan per match.
        let find_block = |byte_pos: usize| -> usize {
            match block_spans.binary_search_by(|(_, span)| {
                if byte_pos < span.start {
                    std::cmp::Ordering::Less
                } else if byte_pos >= span.end {
                    std::cmp::Ordering::Greater
                } else {
                    std::cmp::Ordering::Equal
                }
            }) {
                Ok(idx) => block_spans[idx].0,
                Err(_) => 0,
            }
        };

        if self.is_regex {
            // --- Regex mode ---
            // Configure safety limits to prevent pathological patterns from
            // consuming excessive CPU or memory during compilation/execution.
            let re = if self.is_case_sensitive {
                RegexBuilder::new(query)
                    .nest_limit(50)
                    .size_limit(1 << 20) // 1 MB NFA budget
                    .build()
            } else {
                RegexBuilder::new(query)
                    .case_insensitive(true)
                    .nest_limit(50)
                    .size_limit(1 << 20)
                    .build()
            };

            let Ok(re) = re else {
                // Invalid regex, no matches (error display deferred to future)
                return;
            };

            for mat in re.find_iter(source) {
                if self.matches.len() >= MAX_MATCHES {
                    break;
                }
                let byte_range = mat.start()..mat.end();
                let block_index = find_block(mat.start());
                self.matches.push(SearchMatch {
                    block_index,
                    byte_range,
                });
            }
        } else {
            // --- Plain text mode ---
            if self.is_case_sensitive {
                // Case-sensitive find
                let mut start = 0;
                while start < source.len() {
                    let Some(pos) = source[start..].find(query) else {
                        break;
                    };
                    let absolute_pos = start + pos;
                    let byte_range = absolute_pos..(absolute_pos + query.len());
                    let block_index = find_block(absolute_pos);
                    if self.matches.len() >= MAX_MATCHES {
                        break;
                    }
                    self.matches.push(SearchMatch {
                        block_index,
                        byte_range,
                    });
                    start = absolute_pos + query.len();
                    while start < source.len() && !source.is_char_boundary(start) {
                        start += 1;
                    }
                }
            } else {
                // Case-insensitive find (original logic)
                let query_lower = query.to_lowercase();
                let source_lower = source.to_lowercase();
                let mut start = 0;
                while start < source_lower.len() {
                    let Some(pos) = source_lower[start..].find(&query_lower) else {
                        break;
                    };
                    let absolute_pos = start + pos;
                    let byte_range = absolute_pos..(absolute_pos + query_lower.len());
                    let block_index = find_block(absolute_pos);
                    if self.matches.len() >= MAX_MATCHES {
                        break;
                    }
                    self.matches.push(SearchMatch {
                        block_index,
                        byte_range,
                    });
                    start = absolute_pos + query_lower.len();
                    while start < source_lower.len() && !source_lower.is_char_boundary(start) {
                        start += 1;
                    }
                }
            }
        }
    }

    /// Move to the next match.
    pub fn next_match(&mut self) -> Option<&SearchMatch> {
        if self.matches.is_empty() {
            return None;
        }
        self.current_match = (self.current_match + 1) % self.matches.len();
        self.matches.get(self.current_match)
    }

    /// Move to the previous match.
    pub fn prev_match(&mut self) -> Option<&SearchMatch> {
        if self.matches.is_empty() {
            return None;
        }
        if self.current_match == 0 {
            self.current_match = self.matches.len() - 1;
        } else {
            self.current_match -= 1;
        }
        self.matches.get(self.current_match)
    }

    /// Get the current match.
    pub fn current(&self) -> Option<&SearchMatch> {
        self.matches.get(self.current_match)
    }

    /// Get match count display string.
    pub fn count_display(&self) -> String {
        if self.matches.is_empty() {
            "0/0".to_string()
        } else {
            format!("{}/{}", self.current_match + 1, self.matches.len())
        }
    }
}

/// Callback type for search events.
pub type OnSearchNavigate = Box<dyn Fn(usize, &mut Window, &mut App) + 'static>;
pub type OnSearchClose = Box<dyn Fn(&mut Window, &mut App) + 'static>;
pub type OnSearchChange = Box<dyn Fn(&str, &mut App) + 'static>;

/// Search bar view.
pub struct SearchBar {
    search_state: SearchState,
    input_state: Entity<InputState>,
    #[allow(dead_code)] // Used for keyboard event handling
    focus_handle: FocusHandle,
    on_navigate: Option<OnSearchNavigate>,
    on_close: Option<OnSearchClose>,
    on_change: Option<OnSearchChange>,
}

impl SearchBar {
    pub fn new(window: &mut Window, cx: &mut Context<Self>) -> Self {
        let input_state = cx.new(|cx| InputState::new(window, cx).placeholder("Search..."));

        // Subscribe to input change events
        cx.subscribe(&input_state, |this, entity, event: &InputEvent, cx| {
            if let InputEvent::Change = event {
                let text = entity.read(cx).text().to_string();
                this.search_state.query = text.clone();
                // Trigger on_change callback
                if let Some(on_change) = &this.on_change {
                    on_change(&text, cx);
                }
                cx.notify();
            }
        }).detach();

        Self {
            search_state: SearchState::default(),
            input_state,
            focus_handle: cx.focus_handle(),
            on_navigate: None,
            on_close: None,
            on_change: None,
        }
    }

    pub fn on_navigate(mut self, callback: impl Fn(usize, &mut Window, &mut App) + 'static) -> Self {
        self.on_navigate = Some(Box::new(callback));
        self
    }

    pub fn on_close(mut self, callback: impl Fn(&mut Window, &mut App) + 'static) -> Self {
        self.on_close = Some(Box::new(callback));
        self
    }

    pub fn on_change(mut self, callback: impl Fn(&str, &mut App) + 'static) -> Self {
        self.on_change = Some(Box::new(callback));
        self
    }

    /// Update the search state with new matches.
    pub fn update_state(&mut self, state: SearchState, cx: &mut Context<Self>) {
        self.search_state = state;
        cx.notify();
    }

    /// Get the current search query.
    #[allow(dead_code)] // May be useful for external access
    pub fn query(&self) -> &str {
        &self.search_state.query
    }

    /// Get the focus handle for focusing the search input.
    pub fn focus_handle(&self) -> &FocusHandle {
        &self.focus_handle
    }

    /// Get the input state for focusing.
    pub fn input_state(&self) -> &Entity<InputState> {
        &self.input_state
    }

    /// Get current search mode flags.
    pub fn search_flags(&self) -> (bool, bool) {
        (self.search_state.is_regex, self.search_state.is_case_sensitive)
    }

    fn handle_next(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        if let Some(m) = self.search_state.next_match() {
            let block_index = m.block_index;
            if let Some(on_navigate) = &self.on_navigate {
                on_navigate(block_index, window, cx);
            }
            cx.notify();
        }
    }

    fn handle_prev(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        if let Some(m) = self.search_state.prev_match() {
            let block_index = m.block_index;
            if let Some(on_navigate) = &self.on_navigate {
                on_navigate(block_index, window, cx);
            }
            cx.notify();
        }
    }

    fn handle_close(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        if let Some(on_close) = &self.on_close {
            on_close(window, cx);
        }
    }
}

impl Render for SearchBar {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let theme = cx.theme().clone();
        let count_display = self.search_state.count_display();

        h_flex()
            .id("search-bar")
            .track_focus(&self.focus_handle)
            .key_context("SearchBar")
            .on_key_down(cx.listener(|this, event: &KeyDownEvent, window, cx| {
                if event.keystroke.key == "escape" {
                    this.handle_close(window, cx);
                } else if event.keystroke.key == "enter" {
                    if event.keystroke.modifiers.shift {
                        this.handle_prev(window, cx);
                    } else {
                        this.handle_next(window, cx);
                    }
                }
            }))
            .px_3()
            .py_2()
            .gap_2()
            .bg(theme.background)
            .border_1()
            .border_color(theme.border)
            .rounded(theme.radius)
            .shadow_md()
            .items_center()
            .child(
                Input::new(&self.input_state)
                    .small()
                    .w(px(200.))
            )
            .child(
                div()
                    .text_xs()
                    .text_color(theme.muted_foreground)
                    .min_w(px(40.))
                    .child(count_display)
            )
            // Regex toggle button
            .child(
                Button::new("regex-toggle")
                    .label(".*")
                    .ghost()
                    .xsmall()
                    .tooltip("Use regular expression")
                    .selected(self.search_state.is_regex)
                    .on_click(cx.listener(|this, _, _window, cx| {
                        this.search_state.is_regex = !this.search_state.is_regex;
                        if let Some(on_change) = &this.on_change {
                            let query = this.search_state.query.clone();
                            on_change(&query, cx);
                        }
                        cx.notify();
                    }))
            )
            // Case sensitivity toggle button
            .child(
                Button::new("case-toggle")
                    .label("Aa")
                    .ghost()
                    .xsmall()
                    .tooltip("Match case")
                    .selected(self.search_state.is_case_sensitive)
                    .on_click(cx.listener(|this, _, _window, cx| {
                        this.search_state.is_case_sensitive = !this.search_state.is_case_sensitive;
                        if let Some(on_change) = &this.on_change {
                            let query = this.search_state.query.clone();
                            on_change(&query, cx);
                        }
                        cx.notify();
                    }))
            )
            .child(
                Button::new("prev-btn")
                    .icon(Icon::new(IconName::ChevronUp))
                    .ghost()
                    .xsmall()
                    .on_click(cx.listener(|this, _, window, cx| {
                        this.handle_prev(window, cx);
                    }))
            )
            .child(
                Button::new("next-btn")
                    .icon(Icon::new(IconName::ChevronDown))
                    .ghost()
                    .xsmall()
                    .on_click(cx.listener(|this, _, window, cx| {
                        this.handle_next(window, cx);
                    }))
            )
            .child(
                Button::new("close-btn")
                    .icon(Icon::new(IconName::Close))
                    .ghost()
                    .xsmall()
                    .on_click(cx.listener(|this, _, window, cx| {
                        this.handle_close(window, cx);
                    }))
            )
    }
}
