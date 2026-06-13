use std::{
    pin::Pin,
    sync::{Arc, Mutex},
    task::Poll,
    time::{Duration, Instant},
};

use gpui::{
    App, AppContext as _, AsyncApp, Bounds, ClipboardItem, Context, FocusHandle,
    InteractiveElement, IntoElement, KeyBinding, ListOffset, ListState, ParentElement as _,
    Pixels, Point, Render, SharedString, Size, Styled as _, Task, WeakEntity, Window,
    prelude::FluentBuilder as _, px,
};
use smol::{Timer, stream::StreamExt as _};

use gpui_component::{
    input::{self, Copy},
    v_flex,
};

use super::{
    CodeBlockActionsFn, TextViewStyle,
    document::{HeadingItem, ParsedDocument},
    element_ext::ElementExt,
    format,
    node::{self, NodeContext},
};

const UPDATE_DELAY: Duration = Duration::from_millis(50);

const CONTEXT: &'static str = "TextView";
pub(crate) fn init(cx: &mut App) {
    cx.bind_keys(vec![
        #[cfg(target_os = "macos")]
        KeyBinding::new("cmd-c", input::Copy, Some(CONTEXT)),
        #[cfg(not(target_os = "macos"))]
        KeyBinding::new("ctrl-c", input::Copy, Some(CONTEXT)),
    ]);
}

/// The content format of the text view.
#[derive(Clone, Copy, PartialEq, Eq)]
pub(super) enum TextViewFormat {
    /// Markdown view
    Markdown,
    /// HTML view
    Html,
}

/// Selection mode for different click behaviors.
#[derive(Clone, Copy, PartialEq, Eq, Default)]
pub(super) enum SelectionMode {
    /// Normal character-by-character selection (single click drag)
    #[default]
    Character,
    /// Word selection (double-click)
    Word,
    /// Line selection (triple-click)
    Line,
}

/// The state of a TextView.
pub struct TextViewState {
    pub(super) focus_handle: FocusHandle,
    pub(super) list_state: ListState,

    /// The bounds of the text view
    bounds: Bounds<Pixels>,

    pub(super) selectable: bool,
    pub(super) scrollable: bool,
    /// Scroll speed multiplier (1.0 = normal, 2.0 = double speed)
    pub(super) scroll_speed: f32,
    /// Whether inertia (smooth) scrolling is enabled
    pub(super) inertia_enabled: bool,
    /// Inertia scroll animation state
    pub(super) inertia_scroll: InertiaScrollState,
    pub(super) text_view_style: TextViewStyle,
    pub(super) code_block_actions: Option<Arc<CodeBlockActionsFn>>,

    pub(super) is_selecting: bool,
    /// The selection mode (character, word, or line)
    pub(super) selection_mode: SelectionMode,
    /// The local (in TextView) position of the selection.
    selection_positions: (Option<Point<Pixels>>, Option<Point<Pixels>>),
    /// Indicates if the entire document is selected via Select All.
    is_select_all: bool,

    pub(super) parsed_content: Arc<Mutex<ParsedContent>>,
    text: SharedString,
    search_query: Option<SharedString>,
    search_is_regex: bool,
    search_is_case_sensitive: bool,
    parsed_error: Option<SharedString>,
    /// Path to the source document, used for resolving relative image paths.
    document_path: Option<std::path::PathBuf>,
    tx: smol::channel::Sender<UpdateOptions>,
    _parse_task: Task<()>,
    _receive_task: Task<()>,
}

impl TextViewState {
    /// Create a Markdown TextViewState.
    ///
    /// The `doc_path` parameter is used for resolving relative image paths.
    pub fn markdown(text: &str, doc_path: Option<&std::path::Path>, cx: &mut Context<Self>) -> Self {
        Self::new(TextViewFormat::Markdown, text, doc_path, cx)
    }

    /// Create a HTML TextViewState.
    pub fn html(text: &str, cx: &mut Context<Self>) -> Self {
        Self::new(TextViewFormat::Html, text, None, cx)
    }

    /// Create a new TextViewState.
    fn new(format: TextViewFormat, text: &str, doc_path: Option<&std::path::Path>, cx: &mut Context<Self>) -> Self {
        let focus_handle = cx.focus_handle();

        let (tx, rx) = smol::channel::unbounded::<UpdateOptions>();
        let (tx_result, rx_result) = smol::channel::unbounded::<Result<(), SharedString>>();
        let _receive_task = cx.spawn({
            async move |weak_self, cx| {
                while let Ok(parsed_result) = rx_result.recv().await {
                    _ = weak_self.update(cx, |state, cx| {
                        if let Err(err) = &parsed_result {
                            state.parsed_error = Some(err.clone());
                        }
                        state.clear_selection();
                        state.render_mermaid_blocks(cx);
                        cx.notify();
                    });
                }
            }
        });

        let _parse_task = cx.background_spawn(UpdateFuture::new(format, rx, tx_result));

        let mut this = Self {
            focus_handle,
            bounds: Bounds::default(),
            selection_positions: (None, None),
            selection_mode: SelectionMode::default(),
            selectable: false,
            scrollable: false,
            scroll_speed: 1.0,
            inertia_enabled: true,
            inertia_scroll: InertiaScrollState::default(),
            list_state: ListState::new(0, gpui::ListAlignment::Top, px(1000.)),
            text_view_style: TextViewStyle::default(),
            code_block_actions: None,
            is_selecting: false,
            is_select_all: false,
            parsed_content: Default::default(),
            parsed_error: None,
            text: text.to_string().into(),
            search_query: None,
            search_is_regex: false,
            search_is_case_sensitive: false,
            document_path: doc_path.map(|p| p.to_path_buf()),
            tx,
            _parse_task,
            _receive_task,
        };
        this.increment_update(&text, false, cx);
        this
    }

    /// Get the text content.
    pub(crate) fn source(&self) -> SharedString {
        self.parsed_content.lock().unwrap().document.source.clone()
    }

    /// Set whether the text is selectable, default false.
    pub fn selectable(mut self, selectable: bool) -> Self {
        self.selectable = selectable;
        self
    }

    /// Set whether the text is selectable, default false.
    pub fn set_selectable(&mut self, selectable: bool, cx: &mut Context<Self>) {
        self.selectable = selectable;
        cx.notify();
    }

    /// Set whether the text is selectable, default false.
    pub fn scrollable(mut self, scrollable: bool) -> Self {
        self.scrollable = scrollable;
        self
    }

    /// Set whether the text is selectable, default false.
    pub fn set_scrollable(&mut self, scrollable: bool, cx: &mut Context<Self>) {
        self.scrollable = scrollable;
        cx.notify();
    }

    /// Set scroll speed multiplier (1.0 = normal, 2.0 = double speed).
    pub fn scroll_speed(mut self, speed: f32) -> Self {
        self.scroll_speed = speed;
        self
    }

    /// Set scroll speed multiplier (1.0 = normal, 2.0 = double speed).
    pub fn set_scroll_speed(&mut self, speed: f32, cx: &mut Context<Self>) {
        self.scroll_speed = speed;
        cx.notify();
    }

    /// Get the current scroll speed multiplier.
    pub fn get_scroll_speed(&self) -> f32 {
        self.scroll_speed
    }

    /// Set whether inertia (smooth) scrolling is enabled.
    pub fn inertia_enabled(mut self, enabled: bool) -> Self {
        self.inertia_enabled = enabled;
        self
    }

    /// Set whether inertia (smooth) scrolling is enabled.
    pub fn set_inertia_enabled(&mut self, enabled: bool, cx: &mut Context<Self>) {
        self.inertia_enabled = enabled;
        if !enabled {
            self.inertia_scroll.stop();
        }
        cx.notify();
    }

    /// Check if inertia scrolling is enabled.
    pub fn is_inertia_enabled(&self) -> bool {
        self.inertia_enabled
    }

    /// Add impulse to inertia scroll and start animation if not running.
    pub fn add_scroll_impulse(&mut self, delta_px: f32) {
        self.inertia_scroll.add_impulse(delta_px, self.scroll_speed);
        if !self.inertia_scroll.is_animating() {
            self.inertia_scroll.start();
        }
    }

    /// Scroll the list by the given distance in pixels.
    /// Positive values scroll down, negative values scroll up.
    /// This stops any ongoing inertia animation.
    pub fn scroll_by(&mut self, distance: Pixels) {
        self.inertia_scroll.stop();
        self.list_state.scroll_by(distance);
    }

    /// Scroll by the given distance without stopping inertia animation.
    /// Used internally for non-inertia scrolling mode.
    pub fn scroll_by_direct(&mut self, distance: Pixels) {
        self.list_state.scroll_by(distance);
    }

    /// Scroll to a specific block by index.
    /// This stops any ongoing inertia animation.
    pub fn scroll_to_block(&mut self, index: usize) {
        self.inertia_scroll.stop();
        self.list_state.scroll_to(ListOffset {
            item_ix: index,
            offset_in_item: px(0.),
        });
    }

    /// Scroll to a heading identified by its anchor slug.
    /// Looks up the slug in the heading_map and scrolls to the corresponding block.
    pub fn scroll_to_anchor(&mut self, anchor: &str) {
        let block_index = {
            let content = self.parsed_content.lock().unwrap();
            content.document.heading_map.get(anchor).copied()
        };
        if let Some(index) = block_index {
            self.scroll_to_block(index);
        }
    }

    /// Get the headings from the parsed document for outline display.
    pub fn headings(&self) -> Vec<HeadingItem> {
        self.parsed_content.lock().unwrap().document.extract_headings()
    }

    /// Get the number of blocks in the document.
    pub fn block_count(&self) -> usize {
        self.parsed_content.lock().unwrap().document.blocks.len()
    }

    /// Get the source text content.
    pub fn source_text(&self) -> SharedString {
        self.parsed_content.lock().unwrap().document.source.clone()
    }

    /// Get block spans (index, byte range) for search matching.
    pub fn block_spans(&self) -> Vec<(usize, std::ops::Range<usize>)> {
        self.parsed_content.lock().unwrap().document.block_spans()
    }

    pub fn set_search_query(&mut self, query: &str, is_regex: bool, case_sensitive: bool, cx: &mut Context<Self>) {
        if query.is_empty() {
            self.search_query = None;
        } else {
            self.search_query = Some(query.to_string().into());
        }
        self.search_is_regex = is_regex;
        self.search_is_case_sensitive = case_sensitive;
        cx.notify();
    }

    /// Set the text content.
    pub fn set_text(&mut self, text: &str, cx: &mut Context<Self>) {
        if self.text.as_str() == text {
            return;
        }

        self.text = text.to_string().into();
        self.parsed_error = None;
        self.increment_update(text, false, cx);
    }

    /// Append partial text content to the existing text.
    pub fn push_str(&mut self, new_text: &str, cx: &mut Context<Self>) {
        if new_text.is_empty() {
            return;
        }
        self.increment_update(new_text, true, cx);
    }

    /// Return the selected text.
    pub fn selected_text(&self) -> String {
        if self.is_select_all {
            // Return entire document source when Select All is active
            return self.parsed_content.lock().unwrap().document.source.to_string();
        }
        self.parsed_content.lock().unwrap().document.selected_text()
    }

    fn increment_update(&mut self, text: &str, append: bool, _cx: &mut Context<Self>) {
        let code_block_actions = self.code_block_actions.clone();
        let update_options = UpdateOptions {
            append,
            content: self.parsed_content.clone(),
            pending_text: text.to_string(),
            code_block_actions: code_block_actions.clone(),
            document_path: self.document_path.clone(),
        };

        // Parse at first time by blocking.
        _ = self.tx.try_send(update_options);
    }

    /// Save bounds and unselect if bounds changed.
    pub(super) fn update_bounds(&mut self, bounds: Bounds<Pixels>) {
        let width_delta = (self.bounds.size.width - bounds.size.width).abs();

        if self.bounds.size != bounds.size {
            self.clear_selection();
        }

        if width_delta > px(5.0) {
            let block_count = self.block_count();
            self.list_state.reset(block_count);
        }

        self.bounds = bounds;
    }

    pub(super) fn clear_selection(&mut self) {
        self.selection_positions = (None, None);
        self.selection_mode = SelectionMode::Character;
        self.is_selecting = false;
        self.is_select_all = false;
    }

    /// Get the current scroll offset in pixels (Y component only).
    pub fn scroll_offset_y(&self) -> Pixels {
        // scroll_px_offset_for_scrollbar returns Point with negative Y
        -self.list_state.scroll_px_offset_for_scrollbar().y
    }

    /// Get the maximum scrollable offset (total content height minus viewport).
    pub fn max_scroll_y(&self) -> Pixels {
        self.list_state.max_offset_for_scrollbar().height
    }

    pub(super) fn start_selection(&mut self, pos: Point<Pixels>) {
        // Clear all previous InlineState selections before starting new selection
        self.parsed_content.lock().unwrap().document.clear_all_selections();

        // Convert window coordinates to content coordinates (add scroll offset)
        let scroll_y = self.scroll_offset_y();
        let pos = Point {
            x: pos.x - self.bounds.origin.x,
            y: pos.y - self.bounds.origin.y + scroll_y,
        };
        self.selection_positions = (Some(pos), Some(pos));
        self.selection_mode = SelectionMode::Character;
        self.is_selecting = true;
        self.is_select_all = false;
    }

    pub(super) fn update_selection(&mut self, pos: Point<Pixels>) {
        // Convert window coordinates to content coordinates (add scroll offset)
        let scroll_y = self.scroll_offset_y();
        let pos = Point {
            x: pos.x - self.bounds.origin.x,
            y: pos.y - self.bounds.origin.y + scroll_y,
        };
        if let (Some(start), Some(_)) = self.selection_positions {
            self.selection_positions = (Some(start), Some(pos))
        }
    }

    pub(super) fn end_selection(&mut self) {
        self.is_selecting = false;
    }

    pub(crate) fn has_selection(&self) -> bool {
        if let (Some(start), Some(end)) = self.selection_positions {
            // For Word/Line mode, we have a selection even if start == end
            // The actual selection range will be expanded during rendering
            if self.selection_mode != SelectionMode::Character {
                return true;
            }
            start != end
        } else {
            false
        }
    }

    /// Get the current selection mode.
    pub(super) fn selection_mode(&self) -> SelectionMode {
        self.selection_mode
    }

    /// Select all text in the document.
    pub fn select_all(&mut self, cx: &mut Context<Self>) {
        // Set selection to cover the entire document bounds
        // Use a very large value for end position to ensure full coverage
        self.selection_positions = (
            Some(Point { x: px(0.), y: px(0.) }),
            Some(Point {
                x: self.bounds.size.width,
                y: px(f32::MAX / 2.0), // Use a large but safe value
            }),
        );
        self.selection_mode = SelectionMode::Character;
        self.is_selecting = false;
        self.is_select_all = true;
        cx.notify();
    }

    /// Start word selection at given position (for double-click).
    /// Sets selection mode to Word - actual word boundary detection happens during rendering.
    pub fn start_word_selection(&mut self, pos: Point<Pixels>, _cx: &mut Context<Self>) {
        // Convert window coordinates to content coordinates (add scroll offset)
        let scroll_y = self.scroll_offset_y();
        let local_pos = Point {
            x: pos.x - self.bounds.origin.x,
            y: pos.y - self.bounds.origin.y + scroll_y,
        };
        // Store the click position - word boundary will be calculated during rendering
        self.selection_positions = (Some(local_pos), Some(local_pos));
        self.selection_mode = SelectionMode::Word;
        self.is_selecting = false;
        self.is_select_all = false;
    }

    /// Start line selection at given position (for triple-click).
    /// Sets selection mode to Line - actual line boundary detection happens during rendering.
    pub fn start_line_selection(&mut self, pos: Point<Pixels>, _cx: &mut Context<Self>) {
        // Convert window coordinates to content coordinates (add scroll offset)
        let scroll_y = self.scroll_offset_y();
        let local_pos = Point {
            x: pos.x - self.bounds.origin.x,
            y: pos.y - self.bounds.origin.y + scroll_y,
        };
        // Store the click position - line boundary will be calculated during rendering
        self.selection_positions = (Some(local_pos), Some(local_pos));
        self.selection_mode = SelectionMode::Line;
        self.is_selecting = false;
        self.is_select_all = false;
    }

    /// Return the bounds of the selection in window coordinates.
    pub(crate) fn selection_bounds(&self) -> Bounds<Pixels> {
        // Convert content coordinates back to window coordinates (subtract scroll offset)
        let scroll_y = self.scroll_offset_y();
        let start = self.selection_positions.0.map(|p| Point {
            x: p.x,
            y: p.y - scroll_y,
        });
        let end = self.selection_positions.1.map(|p| Point {
            x: p.x,
            y: p.y - scroll_y,
        });
        selection_bounds(start, end, self.bounds)
    }

    pub(super) fn on_action_copy(&mut self, _: &Copy, _: &mut Window, cx: &mut Context<Self>) {
        let selected_text = self.selected_text().trim().to_string();
        if selected_text.is_empty() {
            return;
        }

        cx.write_to_clipboard(ClipboardItem::new_string(selected_text));
    }

    pub(crate) fn is_selectable(&self) -> bool {
        self.selectable
    }

    /// Start rendering all mermaid code blocks in the document.
    ///
    /// Scans the parsed content for code blocks with the "mermaid" language tag
    /// that haven't been rendered yet and spawns async tasks to render them.
    /// Falls back to showing the source code if rendering fails.
    fn render_mermaid_blocks(&mut self, cx: &mut Context<Self>) {
        use std::sync::atomic::Ordering;

        let mut mermaid_blocks: Vec<(usize, String)> = Vec::new();

        let parsed = self.parsed_content.lock().unwrap();
        for (i, block) in parsed.document.blocks.iter().enumerate() {
            if let node::BlockNode::CodeBlock(cb) = block {
                if cb.lang().as_ref().map(|s| s.as_str()) == Some("mermaid")
                    && cb.state.lock().unwrap().diagram_svg_path.is_none()
                    && !cb.is_rendering.load(Ordering::Relaxed)
                {
                    cb.is_rendering.store(true, Ordering::Relaxed);
                    mermaid_blocks.push((i, cb.code().to_string()));
                }
            }
        }
        drop(parsed);

        // Spawn async rendering tasks for each mermaid block
        for (block_idx, source) in mermaid_blocks {
            let this = cx.entity().downgrade();
            cx.spawn(async move |_: WeakEntity<Self>, cx: &mut AsyncApp| {
                match crate::text::mermaid::MermaidRenderer::render_to_file(&source).await {
                    Ok(path) => {
                        this.update(cx, |state, inner_cx| {
                            let parsed = state.parsed_content.lock().unwrap();
                            if block_idx < parsed.document.blocks.len() {
                                if let node::BlockNode::CodeBlock(cb) =
                                    &parsed.document.blocks[block_idx]
                                {
                                    if cb.is_rendering.load(Ordering::Relaxed) {
                                        cb.state.lock().unwrap().diagram_svg_path =
                                            Some(path);
                                        cb.is_rendering
                                            .store(false, Ordering::Relaxed);
                                    }
                                }
                            }
                            inner_cx.notify();
                        })
                        .ok();
                    }
                    Err(e) => {
                        tracing::warn!(
                            "Mermaid diagram rendering failed for block {}: {}",
                            block_idx,
                            e
                        );
                        // Reset rendering flag so the block shows source code
                        this.update(cx, |state, _inner_cx| {
                            let parsed = state.parsed_content.lock().unwrap();
                            if block_idx < parsed.document.blocks.len() {
                                if let node::BlockNode::CodeBlock(cb) =
                                    &parsed.document.blocks[block_idx]
                                {
                                    cb.is_rendering
                                        .store(false, Ordering::Relaxed);
                                }
                            }
                        })
                        .ok();
                    }
                }
            })
            .detach();
        }
    }
}

impl Render for TextViewState {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let state = cx.entity();
        // Pre-capture fields that don't require locking
        let search_query = self.search_query.clone();
        let search_is_regex = self.search_is_regex;
        let search_is_case_sensitive = self.search_is_case_sensitive;
        let code_block_actions = self.code_block_actions.clone();
        let content_max_width = self.text_view_style.content_max_width;
        let scrollable = self.scrollable;
        let list_state = self.list_state.clone();

        // Hold the lock and borrow the document directly, avoiding full AST clone.
        // Clone only node_cx (small) since we need to modify it with search/UI state.
        let parsed = self.parsed_content.lock().unwrap();
        let mut node_cx = parsed.node_cx.clone();
        node_cx.search_query = search_query;
        node_cx.search_is_regex = search_is_regex;
        node_cx.search_is_case_sensitive = search_is_case_sensitive;
        node_cx.code_block_actions = code_block_actions;

        v_flex()
            .size_full()
            .map(|this| match &mut self.parsed_error {
                None => this.child(parsed.document.render_root(
                    if scrollable {
                        Some(list_state.clone())
                    } else {
                        None
                    },
                    &node_cx,
                    content_max_width,
                    window,
                    cx,
                )),
                Some(err) => this.child(
                    v_flex()
                        .gap_1()
                        .child("Failed to parse content")
                        .child(err.to_string()),
                ),
            })
            // Handle scroll wheel with optional inertia
            .when(scrollable, |this| {
                let state = state.clone();
                let list_state = list_state.clone();
                this.on_scroll_wheel(move |event, _window, cx| {
                    let delta = event.delta.pixel_delta(px(20.)).y;

                    state.update(cx, |state, cx| {
                        if state.inertia_enabled {
                            // Use inertia scroll
                            state.add_scroll_impulse(f32::from(delta));
                        } else {
                            // Direct scroll without inertia
                            let scroll_distance = delta * state.scroll_speed;
                            list_state.scroll_by(-scroll_distance);
                        }
                        cx.notify();
                    });

                    // Stop propagation to prevent GPUI list's default scroll
                    cx.stop_propagation();
                })
            })
            .on_prepaint({
                let list_state = list_state.clone();
                move |bounds, window, cx| {
                    state.update(cx, |state, _| {
                        state.update_bounds(bounds);

                        // Process inertia scroll animation
                        if state.inertia_scroll.is_animating() {
                            if let Some(distance) =
                                state.inertia_scroll.update(Instant::now(), state.scroll_speed)
                            {
                                // Apply scroll distance (negative because positive delta = scroll up)
                                list_state.scroll_by(px(-distance));

                                // Request next frame if still animating
                                if state.inertia_scroll.is_animating() {
                                    window.request_animation_frame();
                                }
                            }
                        }
                    });
                }
            })
    }
}

#[derive(PartialEq, Default)]
pub(crate) struct ParsedContent {
    pub(crate) document: ParsedDocument,
    pub(crate) node_cx: node::NodeContext,
}

struct UpdateFuture {
    format: TextViewFormat,
    options: UpdateOptions,
    pending_text: String,
    timer: Timer,
    rx: Pin<Box<smol::channel::Receiver<UpdateOptions>>>,
    tx_result: smol::channel::Sender<Result<(), SharedString>>,
    delay: Duration,
}

impl UpdateFuture {
    fn new(
        format: TextViewFormat,
        rx: smol::channel::Receiver<UpdateOptions>,
        tx_result: smol::channel::Sender<Result<(), SharedString>>,
    ) -> Self {
        Self {
            format,
            pending_text: String::new(),
            options: UpdateOptions {
                append: false,
                pending_text: String::new(),
                content: Default::default(),
                code_block_actions: None,
                document_path: None,
            },
            timer: Timer::never(),
            rx: Box::pin(rx),
            tx_result,
            delay: UPDATE_DELAY,
        }
    }
}

impl Future for UpdateFuture {
    type Output = ();

    fn poll(mut self: Pin<&mut Self>, cx: &mut std::task::Context<'_>) -> Poll<Self::Output> {
        loop {
            match self.rx.poll_next(cx) {
                Poll::Ready(Some(options)) => {
                    let delay = self.delay;
                    if options.append {
                        self.pending_text.push_str(options.pending_text.as_str());
                    } else {
                        self.pending_text = options.pending_text.clone();
                    }
                    self.options = options;
                    self.timer.set_after(delay);
                    continue;
                }
                Poll::Ready(None) => return Poll::Ready(()),
                Poll::Pending => {}
            }

            match self.timer.poll_next(cx) {
                Poll::Ready(Some(_)) => {
                    let pending_text = std::mem::take(&mut self.pending_text);

                    let res = parse_content(
                        self.format,
                        &UpdateOptions {
                            pending_text,
                            ..self.options.clone()
                        },
                    );
                    _ = self.tx_result.try_send(res);
                    continue;
                }
                Poll::Ready(None) | Poll::Pending => return Poll::Pending,
            }
        }
    }
}

#[derive(Clone)]
struct UpdateOptions {
    content: Arc<Mutex<ParsedContent>>,
    pending_text: String,
    append: bool,
    code_block_actions: Option<Arc<CodeBlockActionsFn>>,
    document_path: Option<std::path::PathBuf>,
}

fn parse_content(format: TextViewFormat, options: &UpdateOptions) -> Result<(), SharedString> {
    let mut node_cx = NodeContext {
        code_block_actions: options.code_block_actions.clone(),
        document_path: options.document_path.clone(),
        ..NodeContext::default()
    };

    let mut content = options.content.lock().unwrap();
    let mut source = String::new();
    if options.append
        && let Some(last_block) = content.document.blocks.pop()
        && let Some(span) = last_block.span()
    {
        node_cx.offset = span.start;
        let last_source = &content.document.source[span.start..];
        source.push_str(last_source);
        source.push_str(&options.pending_text);
    } else {
        source = options.pending_text.to_string();
    }

    let new_content = match format {
        TextViewFormat::Markdown => {
            format::markdown::parse(&source, &mut node_cx)
        }
        TextViewFormat::Html => format::html::parse(&source, &mut node_cx),
    }?;

    if options.append {
        content.document.source =
            format!("{}{}", content.document.source, options.pending_text).into();
        content.document.blocks.extend(new_content.blocks);
    } else {
        content.document = new_content;
    }

    content.node_cx = node_cx;
    Ok(())
}

fn selection_bounds(
    start: Option<Point<Pixels>>,
    end: Option<Point<Pixels>>,
    bounds: Bounds<Pixels>,
) -> Bounds<Pixels> {
    if let (Some(start), Some(end)) = (start, end) {
        let start = start + bounds.origin;
        let end = end + bounds.origin;

        let origin = Point {
            x: start.x.min(end.x),
            y: start.y.min(end.y),
        };
        let size = Size {
            width: (start.x - end.x).abs(),
            height: (start.y - end.y).abs(),
        };

        return Bounds { origin, size };
    }

    Bounds::default()
}

#[cfg(test)]
mod tests {
    use super::*;
    use gpui::{Bounds, point, px, size};

    #[test]
    fn test_text_view_state_selection_bounds() {
        assert_eq!(
            selection_bounds(None, None, Default::default()),
            Bounds::default()
        );
        assert_eq!(
            selection_bounds(None, Some(point(px(10.), px(20.))), Default::default()),
            Bounds::default()
        );
        assert_eq!(
            selection_bounds(Some(point(px(10.), px(20.))), None, Default::default()),
            Bounds::default()
        );

        // 10,10 start
        //   |------|
        //   |      |
        //   |------|
        //         50,50
        assert_eq!(
            selection_bounds(
                Some(point(px(10.), px(10.))),
                Some(point(px(50.), px(50.))),
                Default::default()
            ),
            Bounds {
                origin: point(px(10.), px(10.)),
                size: size(px(40.), px(40.))
            }
        );
        // 10,10
        //   |------|
        //   |      |
        //   |------|
        //         50,50 start
        assert_eq!(
            selection_bounds(
                Some(point(px(50.), px(50.))),
                Some(point(px(10.), px(10.))),
                Default::default()
            ),
            Bounds {
                origin: point(px(10.), px(10.)),
                size: size(px(40.), px(40.))
            }
        );
        //        50,10 start
        //   |------|
        //   |      |
        //   |------|
        // 10,50
        assert_eq!(
            selection_bounds(
                Some(point(px(50.), px(10.))),
                Some(point(px(10.), px(50.))),
                Default::default()
            ),
            Bounds {
                origin: point(px(10.), px(10.)),
                size: size(px(40.), px(40.))
            }
        );
        //        50,10
        //   |------|
        //   |      |
        //   |------|
        // 10,50 start
        assert_eq!(
            selection_bounds(
                Some(point(px(10.), px(50.))),
                Some(point(px(50.), px(10.))),
                Default::default()
            ),
            Bounds {
                origin: point(px(10.), px(10.)),
                size: size(px(40.), px(40.))
            }
        );
    }
}

/// Inertia scroll animation state for smooth scrolling.
#[derive(Default, Clone)]
pub(super) struct InertiaScrollState {
    /// Current scroll velocity in pixels per second (positive = scroll down)
    velocity: f32,
    /// Whether animation is currently running
    is_animating: bool,
    /// Last frame timestamp for delta time calculation
    last_frame_time: Option<Instant>,
}

impl InertiaScrollState {
    /// Minimum velocity threshold below which animation stops (px/s)
    const MIN_VELOCITY: f32 = 10.0;
    /// Friction coefficient: velocity decays by this factor per frame at 60fps
    const FRICTION: f32 = 0.06;
    /// Maximum velocity cap to prevent excessive scrolling (px/s)
    const MAX_VELOCITY: f32 = 8000.0;
    /// Velocity boost per scroll wheel delta
    const VELOCITY_MULTIPLIER: f32 = 8.0;
    /// Velocity threshold above which exponential (smooth) decay is used (px/s)
    /// Only very fast scrolling triggers smooth deceleration
    const SMOOTH_THRESHOLD: f32 = 8000.0;
    /// Linear deceleration rate (px/s²) for crisp stopping
    const LINEAR_DECEL: f32 = 8000.0;

    /// Add impulse from scroll wheel event
    pub fn add_impulse(&mut self, delta_px: f32, scroll_speed: f32) {
        let impulse = delta_px * Self::VELOCITY_MULTIPLIER * scroll_speed;

        // If scrolling in same direction, accumulate velocity
        // If opposite direction, blend with some momentum preservation
        if self.velocity.signum() == impulse.signum() || self.velocity.abs() < Self::MIN_VELOCITY {
            self.velocity += impulse;
        } else {
            // Opposite direction: blend 30% old + 100% new for smoother reversal
            self.velocity = self.velocity * 0.3 + impulse;
        }

        // Clamp to max velocity
        self.velocity = self.velocity.clamp(-Self::MAX_VELOCITY, Self::MAX_VELOCITY);
    }

    /// Update animation state, returns distance to scroll this frame.
    /// Returns None if animation should stop.
    pub fn update(&mut self, now: Instant, scroll_speed: f32) -> Option<f32> {
        if !self.is_animating {
            return None;
        }

        // Calculate delta time, capped to prevent huge jumps
        let dt = self
            .last_frame_time
            .map(|t| now.duration_since(t).as_secs_f32())
            .unwrap_or(1.0 / 60.0)
            .min(0.1);

        self.last_frame_time = Some(now);

        // Calculate distance to scroll this frame
        let distance = self.velocity * dt;

        // Apply decay based on velocity magnitude
        // Threshold scales with scroll_speed for consistent feel
        let threshold = Self::SMOOTH_THRESHOLD * scroll_speed;
        if self.velocity.abs() > threshold {
            // High speed: exponential decay for smooth deceleration
            self.velocity *= (1.0 - Self::FRICTION).powf(dt * 60.0);
        } else {
            // Normal/low speed: linear decay for crisp stopping
            let decel = Self::LINEAR_DECEL * dt;
            if self.velocity.abs() <= decel {
                self.velocity = 0.0;
            } else {
                self.velocity -= decel * self.velocity.signum();
            }
        }

        // Check if should stop
        if self.velocity.abs() < Self::MIN_VELOCITY {
            self.stop();
            return None;
        }

        Some(distance)
    }

    /// Start animation if velocity is above threshold
    pub fn start(&mut self) {
        if self.velocity.abs() >= Self::MIN_VELOCITY {
            self.is_animating = true;
            self.last_frame_time = None;
        }
    }

    /// Stop animation and reset state
    pub fn stop(&mut self) {
        self.is_animating = false;
        self.velocity = 0.0;
        self.last_frame_time = None;
    }

    /// Check if currently animating
    pub fn is_animating(&self) -> bool {
        self.is_animating
    }
}

