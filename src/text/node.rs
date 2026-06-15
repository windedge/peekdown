use std::{
    collections::HashMap,
    ops::Range,
    path::PathBuf,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Mutex,
    },
};

use regex::RegexBuilder;

use gpui::{
    AnyElement, App, ClipboardItem, DefiniteLength, Div, ElementId, FontStyle, FontWeight, Half,
    HighlightStyle, InteractiveElement as _, IntoElement, Length, ObjectFit, ParentElement,
    SharedString, SharedUri, StatefulInteractiveElement, Styled, StyledImage as _, Window, div,
    img, prelude::FluentBuilder as _, px, relative, rems,
};
use markdown::mdast;

use gpui_component::{
    ActiveTheme as _, Icon, IconName, StyledExt, h_flex,
    highlighter::HighlightTheme,

    menu::ContextMenuExt,
    tooltip::Tooltip,
    v_flex,
};

use crate::state::frontmatter::Value as FrontmatterValue;

use super::{
    CodeBlockActionsFn,
    document::NodeRenderOptions,
    highlighter_cache::highlight_code,
    inline::{Inline, InlineState},
    TextViewStyle,
    utils::list_item_prefix,
};

/// The block-level nodes.
#[derive(Debug, Clone, PartialEq)]
pub(crate) enum BlockNode {
    /// Something like a Div container in HTML.
    Root {
        children: Vec<BlockNode>,
        span: Option<Span>,
    },
    Paragraph(Paragraph),
    Heading {
        level: u8,
        children: Paragraph,
        span: Option<Span>,
        /// The anchor ID generated from heading text (e.g., "my-heading").
        id: Option<SharedString>,
    },
    Blockquote {
        children: Vec<BlockNode>,
        span: Option<Span>,
    },
    List {
        /// Only contains ListItem, others will be ignored
        children: Vec<BlockNode>,
        ordered: bool,
        span: Option<Span>,
    },
    ListItem {
        children: Vec<BlockNode>,
        spread: bool,
        /// Whether the list item is checked, if None, it's not a checkbox
        checked: Option<bool>,
        span: Option<Span>,
    },
    CodeBlock(CodeBlock),
    Table(Table),
    Break {
        html: bool,
        span: Option<Span>,
    },
    Frontmatter(FrontmatterBlock),
    Divider {
        span: Option<Span>,
    },
    /// Use for to_markdown get raw definition
    Definition {
        identifier: SharedString,
        url: SharedString,
        title: Option<SharedString>,
        span: Option<Span>,
    },
    Unknown,
}

/// Generate an anchor slug from heading text.
///
/// - Lowercases the text
/// - Replaces whitespace with `-`
/// - Removes characters that are not alphanumeric or `-`
/// - Trims leading/trailing `-`
pub(crate) fn slugify(text: &str) -> String {
    text.to_lowercase()
        .chars()
        .map(|c| if c.is_whitespace() { '-' } else { c })
        .filter(|c| c.is_alphanumeric() || *c == '-')
        .collect::<String>()
        .trim_matches('-')
        .to_string()
}

impl BlockNode {
    pub(super) fn is_list_item(&self) -> bool {
        matches!(self, Self::ListItem { .. })
    }

    pub(super) fn is_break(&self) -> bool {
        matches!(self, Self::Break { .. })
    }

    #[allow(dead_code)]
    pub(super) fn is_frontmatter(&self) -> bool {
        matches!(self, Self::Frontmatter(_))
    }

/// Combine all children, omitting the empt parent nodes.
    pub(super) fn compact(self) -> BlockNode {
        match self {
            Self::Root { mut children, .. } if children.len() == 1 => children.remove(0).compact(),
            _ => self,
        }
    }

    /// Get the span of the node.
    pub(super) fn span(&self) -> Option<Span> {
        match self {
            BlockNode::Root { span, .. } => *span,
            BlockNode::Paragraph(paragraph) => paragraph.span,
            BlockNode::Heading { span, .. } => *span,
            BlockNode::Blockquote { span, .. } => *span,
            BlockNode::List { span, .. } => *span,
            BlockNode::ListItem { span, .. } => *span,
            BlockNode::CodeBlock(code_block) => code_block.span,
            BlockNode::Table(table) => table.span,
            BlockNode::Break { span, .. } => *span,
            BlockNode::Frontmatter(fm) => fm.span,
            BlockNode::Divider { span, .. } => *span,
            BlockNode::Definition { span, .. } => *span,
            BlockNode::Unknown => None,
        }
    }

    /// Clear all InlineState selections in this block and its children.
    pub(super) fn clear_selection(&self) {
        match self {
            BlockNode::Root { children, .. }
            | BlockNode::List { children, .. }
            | BlockNode::ListItem { children, .. }
            | BlockNode::Blockquote { children, .. } => {
                for c in children.iter() {
                    c.clear_selection();
                }
            }
            BlockNode::Paragraph(paragraph) => {
                paragraph.clear_selection();
            }
            BlockNode::Heading { children, .. } => {
                children.clear_selection();
            }
            BlockNode::Table(table) => {
                for row in table.children.iter() {
                    for cell in row.children.iter() {
                        cell.children.clear_selection();
                    }
                }
            }
            BlockNode::CodeBlock(code_block) => {
                code_block.clear_selection();
            }
            BlockNode::Definition { .. }
            | BlockNode::Break { .. }
            | BlockNode::Frontmatter(_)
            | BlockNode::Divider { .. }
            | BlockNode::Unknown => {}
        }
    }

    pub(super) fn selected_text(&self) -> String {
        let mut text = String::new();
        match self {
            BlockNode::Root { children, .. } => {
                let mut block_text = String::new();
                for c in children.iter() {
                    block_text.push_str(&c.selected_text());
                }
                if !block_text.is_empty() {
                    text.push_str(&block_text);
                    text.push('\n');
                }
            }
            BlockNode::Paragraph(paragraph) => {
                let mut block_text = String::new();
                block_text.push_str(&paragraph.selected_text());
                if !block_text.is_empty() {
                    text.push_str(&block_text);
                    text.push('\n');
                }
            }
            BlockNode::Heading { children, .. } => {
                let mut block_text = String::new();
                block_text.push_str(&children.selected_text());
                if !block_text.is_empty() {
                    text.push_str(&block_text);
                    text.push('\n');
                }
            }
            BlockNode::List { children, .. } => {
                for c in children.iter() {
                    text.push_str(&c.selected_text());
                }
            }
            BlockNode::ListItem { children, .. } => {
                for c in children.iter() {
                    text.push_str(&c.selected_text());
                }
            }
            BlockNode::Blockquote { children, .. } => {
                let mut block_text = String::new();
                for c in children.iter() {
                    block_text.push_str(&c.selected_text());
                }

                if !block_text.is_empty() {
                    text.push_str(&block_text);
                    text.push('\n');
                }
            }
            BlockNode::Table(table) => {
                let mut block_text = String::new();
                for row in table.children.iter() {
                    let mut row_texts = vec![];
                    for cell in row.children.iter() {
                        row_texts.push(cell.children.selected_text());
                    }
                    if !row_texts.is_empty() {
                        block_text.push_str(&row_texts.join(" "));
                        block_text.push('\n');
                    }
                }

                if !block_text.is_empty() {
                    text.push_str(&block_text);
                    text.push('\n');
                }
            }
            BlockNode::CodeBlock(code_block) => {
                let block_text = code_block.selected_text();
                if !block_text.is_empty() {
                    text.push_str(&block_text);
                    text.push('\n');
                }
            }
            BlockNode::Frontmatter(fm) => {
                for (key, value) in &fm.entries {
                    if !key.is_empty() {
                        text.push_str(key);
                        text.push_str(": ");
                        text.push_str(&value.to_string());
                        text.push('\n');
                    }
                }
            }
            BlockNode::Definition { .. }
            | BlockNode::Break { .. }
            | BlockNode::Divider { .. }
            | BlockNode::Unknown => {}
        }

        text
    }
}

#[allow(unused)]
#[derive(Debug, Default, Clone, PartialEq)]
pub struct LinkMark {
    pub url: SharedString,
    /// Optional identifier for footnotes.
    pub identifier: Option<SharedString>,
    pub title: Option<SharedString>,
}

#[derive(Debug, Default, Clone, PartialEq)]
pub struct TextMark {
    pub bold: bool,
    pub italic: bool,
    pub strikethrough: bool,
    pub code: bool,
    pub link: Option<LinkMark>,
}

impl TextMark {
    pub fn bold(mut self) -> Self {
        self.bold = true;
        self
    }

    pub fn italic(mut self) -> Self {
        self.italic = true;
        self
    }

    pub fn strikethrough(mut self) -> Self {
        self.strikethrough = true;
        self
    }

    pub fn code(mut self) -> Self {
        self.code = true;
        self
    }

    pub fn link(mut self, link: impl Into<LinkMark>) -> Self {
        self.link = Some(link.into());
        self
    }
}

/// The bytes
#[derive(Debug, Default, Copy, Clone, PartialEq)]
pub struct Span {
    pub start: usize,
    pub end: usize,
}

impl From<Span> for ElementId {
    fn from(value: Span) -> Self {
        ElementId::Name(format!("md-{}:{}", value.start, value.end).into())
    }
}

#[allow(unused)]
#[derive(Debug, Default, Clone)]
pub struct ImageNode {
    pub url: SharedUri,
    /// Resolved local file path for relative image URLs
    pub local_path: Option<PathBuf>,
    pub link: Option<LinkMark>,
    pub title: Option<SharedString>,
    pub alt: Option<SharedString>,
    pub width: Option<DefiniteLength>,
    pub height: Option<DefiniteLength>,
}

impl ImageNode {
    pub fn title(&self) -> String {
        self.title
            .clone()
            .unwrap_or_else(|| self.alt.clone().unwrap_or_default())
            .to_string()
    }
}

impl PartialEq for ImageNode {
    fn eq(&self, other: &Self) -> bool {
        self.url == other.url
            && self.local_path == other.local_path
            && self.link == other.link
            && self.title == other.title
            && self.alt == other.alt
            && self.width == other.width
            && self.height == other.height
    }
}

#[derive(Default, Clone, Debug)]
pub(crate) struct InlineNode {
    /// The text content.
    pub(crate) text: SharedString,
    pub(crate) image: Option<ImageNode>,
    /// The text styles, each tuple contains the range of the text and the style.
    pub(crate) marks: Vec<(Range<usize>, TextMark)>,
}

impl PartialEq for InlineNode {
    fn eq(&self, other: &Self) -> bool {
        self.text == other.text && self.image == other.image && self.marks == other.marks
    }
}

impl InlineNode {
    pub(crate) fn new(text: impl Into<SharedString>) -> Self {
        Self {
            text: text.into(),
            image: None,
            marks: vec![],
        }
    }

    pub(crate) fn image(image: ImageNode) -> Self {
        let mut this = Self::new("");
        this.image = Some(image);
        this
    }

    pub(crate) fn marks(mut self, marks: Vec<(Range<usize>, TextMark)>) -> Self {
        self.marks = marks;
        self
    }
}

/// The paragraph element, contains multiple text nodes.
///
/// Unlike other Element, this is cloneable, because it is used in the Node AST.
/// We are keep the selection state inside this AST Nodes.
#[derive(Debug, Clone, Default)]
pub(crate) struct Paragraph {
    pub(super) span: Option<Span>,
    pub(super) children: Vec<InlineNode>,
    /// The link references in this paragraph, used for reference links.
    ///
    /// The key is the identifier, the value is the url.
    pub(super) link_refs: HashMap<SharedString, SharedString>,

    /// State for the last (or only) text segment in this paragraph.
    /// For paragraphs with images, intermediate segments use `segment_states`.
    pub(crate) state: Arc<Mutex<InlineState>>,
    /// Additional segment states for paragraphs with images.
    /// Each entry corresponds to a text segment before an image break.
    pub(super) segment_states: Arc<Mutex<Vec<Arc<Mutex<InlineState>>>>>,
}

impl PartialEq for Paragraph {
    fn eq(&self, other: &Self) -> bool {
        self.span == other.span
            && self.children == other.children
            && self.link_refs == other.link_refs
    }
}

impl Paragraph {
    pub(crate) fn new(text: String) -> Self {
        Self {
            span: None,
            children: vec![InlineNode::new(&text)],
            link_refs: HashMap::new(),
            state: Arc::new(Mutex::new(InlineState::default())),
            segment_states: Arc::new(Mutex::new(vec![])),
        }
    }

    pub(super) fn selected_text(&self) -> String {
        let mut text = String::new();

        let seg_states = self.segment_states.lock().unwrap();
        for seg_state in seg_states.iter() {
            let state = seg_state.lock().unwrap();
            if let Some(selection) = &state.selection {
                let part_text = state.text.clone();
                text.push_str(&part_text[selection.start..selection.end]);
            }
        }
        drop(seg_states);

        let state = self.state.lock().unwrap();
        if let Some(selection) = &state.selection {
            let all_text = state.text.clone();
            text.push_str(&all_text[selection.start..selection.end]);
        }

        text
    }

    /// Clear all InlineState selections.
    pub(super) fn clear_selection(&self) {
        let seg_states = self.segment_states.lock().unwrap();
        for seg_state in seg_states.iter() {
            seg_state.lock().unwrap().selection = None;
        }
        drop(seg_states);
        self.state.lock().unwrap().selection = None;
    }
}

#[derive(Debug, Clone, Default, PartialEq)]
pub(crate) struct Table {
    pub(crate) children: Vec<TableRow>,
    pub(crate) column_aligns: Vec<ColumnumnAlign>,
    /// Pre-calculated column widths based on text content length.
    /// Computed once during parsing to avoid repeated calculation in render.
    pub(crate) column_widths: Vec<usize>,
    pub(crate) span: Option<Span>,
}

impl Table {
    const DEFAULT_COLUMN_WIDTH: usize = 5;

    pub(crate) fn column_align(&self, index: usize) -> ColumnumnAlign {
        self.column_aligns.get(index).copied().unwrap_or_default()
    }

    /// Get column width, using pre-calculated value if available.
    pub(crate) fn column_width(&self, index: usize) -> usize {
        self.column_widths
            .get(index)
            .copied()
            .unwrap_or(Self::DEFAULT_COLUMN_WIDTH)
    }

    pub(crate) fn max_column_count(&self) -> usize {
        self.children
            .iter()
            .map(|row| row.children.len())
            .max()
            .unwrap_or(0)
    }

    /// Calculate column widths based on cell text content.
    /// Should be called once after parsing is complete.
    pub(crate) fn calculate_column_widths(&mut self) {
        let mut col_widths = vec![];

        for row in self.children.iter() {
            for (ix, cell) in row.children.iter().enumerate() {
                if col_widths.len() <= ix {
                    col_widths.push(Self::DEFAULT_COLUMN_WIDTH);
                }

                let len = cell.children.text_len();
                if len > col_widths[ix] {
                    col_widths[ix] = len;
                }
            }
        }

        self.column_widths = col_widths;
    }

    pub(crate) fn normalized_column_ratios(
        &self,
        column_count: usize,
        min_weight: usize,
        max_weight: usize,
        padding_weight: usize,
    ) -> Vec<f32> {
        if column_count == 0 {
            return vec![];
        }

        let min_weight = min_weight.max(1);
        let max_weight = max_weight.max(min_weight);
        let mut weights = Vec::with_capacity(column_count);

        for index in 0..column_count {
            let column_width = self.column_width(index);
            let weight = column_width.clamp(min_weight, max_weight) + padding_weight;
            weights.push(weight as f32);
        }

        let total_weight: f32 = weights.iter().sum();
        if total_weight <= f32::EPSILON {
            let ratio = 1.0 / column_count as f32;
            return vec![ratio; column_count];
        }

        weights.into_iter().map(|weight| weight / total_weight).collect()
    }
}

#[derive(Debug, Default, Copy, Clone, PartialEq)]
pub(crate) enum ColumnumnAlign {
    #[default]
    Left,
    Center,
    Right,
}

impl From<mdast::AlignKind> for ColumnumnAlign {
    fn from(value: mdast::AlignKind) -> Self {
        match value {
            mdast::AlignKind::None => ColumnumnAlign::Left,
            mdast::AlignKind::Left => ColumnumnAlign::Left,
            mdast::AlignKind::Center => ColumnumnAlign::Center,
            mdast::AlignKind::Right => ColumnumnAlign::Right,
        }
    }
}

#[derive(Debug, Clone, Default, PartialEq)]
pub(crate) struct TableRow {
    pub children: Vec<TableCell>,
}

#[derive(Debug, Clone, Default, PartialEq)]
pub(crate) struct TableCell {
    pub children: Paragraph,
    pub width: Option<DefiniteLength>,
}

impl Paragraph {
    pub(crate) fn take(&mut self) -> Paragraph {
        std::mem::replace(
            self,
            Paragraph {
                span: None,
                children: vec![],
                link_refs: Default::default(),
                state: Arc::new(Mutex::new(InlineState::default())),
                segment_states: Arc::new(Mutex::new(vec![])),
            },
        )
    }

    pub(crate) fn is_image(&self) -> bool {
        false
    }

    pub(crate) fn set_span(&mut self, span: Span) {
        self.span = Some(span);
    }

    pub(crate) fn push_str(&mut self, text: &str) {
        self.children.push(
            InlineNode::new(text.to_string()).marks(vec![(0..text.len(), TextMark::default())]),
        );
    }

    pub(crate) fn push(&mut self, text: InlineNode) {
        self.children.push(text);
    }

    pub(crate) fn push_image(&mut self, image: ImageNode) {
        self.children.push(InlineNode::image(image));
    }

    pub(crate) fn is_empty(&self) -> bool {
        self.children.is_empty()
            || self
                .children
                .iter()
                .all(|node| node.text.is_empty() && node.image.is_none())
    }

    /// Return length of children text.
    pub(crate) fn text_len(&self) -> usize {
        self.children
            .iter()
            .map(|node| node.text.len())
            .sum::<usize>()
    }

    pub(crate) fn merge(&mut self, other: Self) {
        self.children.extend(other.children);
        // Merge segment_states from the other paragraph
        let mut other_segs = other.segment_states.lock().unwrap();
        if !other_segs.is_empty() {
            self.segment_states.lock().unwrap().append(&mut other_segs);
        }
    }
}

/// Merged internal state for CodeBlock, stored behind a single `Arc<Mutex>`.
#[derive(Debug, Clone)]
pub(crate) struct CodeBlockState {
    pub(crate) styles: Vec<(Range<usize>, HighlightStyle)>,
    pub(crate) cached_theme: Option<String>,
    /// Path to a rendered SVG file for Mermaid diagrams.
    pub(crate) diagram_svg_path: Option<PathBuf>,
}

#[derive(Debug, Clone)]
pub struct CodeBlock {
    code: SharedString,
    lang: Option<SharedString>,
    /// Merged state: styles, cached_theme, diagram_svg_path.
    pub(crate) state: Arc<Mutex<CodeBlockState>>,
    /// Inline state kept separate because `Inline::new()` requires `Arc<Mutex<InlineState>>`.
    pub(crate) inline_state: Arc<Mutex<InlineState>>,
    pub span: Option<Span>,
    /// Whether Mermaid rendering is currently in progress.
    /// Kept as `Arc<AtomicBool>` for lock-free reads during rendering.
    pub(crate) is_rendering: Arc<AtomicBool>,
}

impl PartialEq for CodeBlock {
    fn eq(&self, other: &Self) -> bool {
        self.lang == other.lang && self.code == other.code
    }
}

impl CodeBlock {
    /// Get the language of the code block.
    pub fn lang(&self) -> Option<SharedString> {
        self.lang.clone()
    }

    /// Get the code content of the code block.
    pub fn code(&self) -> SharedString {
        self.code.clone()
    }

    pub(crate) fn new(
        code: SharedString,
        lang: Option<SharedString>,
        span: Option<impl Into<Span>>,
    ) -> Self {
        let inline_state = Arc::new(Mutex::new(InlineState::default()));
        inline_state.lock().unwrap().set_text(code.clone());

        Self {
            code,
            lang,
            state: Arc::new(Mutex::new(CodeBlockState {
                styles: vec![],
                cached_theme: None,
                diagram_svg_path: None,
            })),
            inline_state,
            span: span.map(|s| s.into()),
            is_rendering: Arc::new(AtomicBool::new(false)),
        }
    }

    pub(super) fn selected_text(&self) -> String {
        let mut text = String::new();
        let state = self.inline_state.lock().unwrap();
        if let Some(selection) = &state.selection {
            let part_text = state.text.clone();
            text.push_str(&part_text[selection.start..selection.end]);
        }
        text
    }

    /// Clear InlineState selection.
    pub(super) fn clear_selection(&self) {
        self.inline_state.lock().unwrap().selection = None;
    }

    /// Ensure styles are computed for the current highlight theme.
    /// Lazily computes syntax highlighting styles on first render or when theme changes.
    fn ensure_styles(&self, highlight_theme: &HighlightTheme) {
        let theme_key = format!("{:p}", highlight_theme as *const HighlightTheme);

        let mut cs = self.state.lock().unwrap();
        if cs.cached_theme.as_deref() == Some(&theme_key) && !cs.styles.is_empty() {
            return;
        }

        if let Some(lang) = &self.lang {
            cs.styles = highlight_code(self.code.as_ref(), lang, highlight_theme);
        } else {
            cs.styles.clear();
        }
        cs.cached_theme = Some(theme_key);
    }

    fn render(
        &self,
        options: &NodeRenderOptions,
        node_cx: &NodeContext,
        window: &mut Window,
        cx: &mut App,
    ) -> AnyElement {
        // Mermaid diagram rendering: show rendered image or placeholder
        if self.lang.as_ref().map(|s| s.as_str()) == Some("mermaid") {
            // Check if we have a rendered SVG file
            if self.state.lock().unwrap().diagram_svg_path.as_ref().is_some_and(|p| p.exists()) {
                let svg_path = self.state.lock().unwrap().diagram_svg_path.clone().unwrap();
                return div()
                    .when(!options.is_last, |this| this.pb(node_cx.style.paragraph_gap))
                    .child(
                        div()
                            .id(("mermaid", options.ix))
                            .child(
                                img(svg_path)
                                    .max_w(relative(1.0))
                                    .object_fit(ObjectFit::Contain),
                            ),
                    )
                    .into_any_element();
            }

            // Show placeholder while rendering
            if self.is_rendering.load(Ordering::Relaxed) {
                return div()
                    .when(!options.is_last, |this| this.pb(node_cx.style.paragraph_gap))
                    .child(
                        div()
                            .id(("mermaid-placeholder", options.ix))
                            .p_3()
                            .rounded(cx.theme().radius)
                            .bg(cx.theme().muted)
                            .text_color(cx.theme().muted_foreground)
                            .child("Rendering diagram..."),
                    )
                    .into_any_element();
            }

            // Not rendered yet and not rendering: fall through to show
            // source code block (default display)
        }

        let style = &node_cx.style;

        // Ensure styles are computed for current highlight theme
        self.ensure_styles(cx.theme().highlight_theme.as_ref());

        // Only clone and merge highlights if there's an active search query
        let cs = self.state.lock().unwrap();
        let code_highlights = match node_cx.search_query.as_ref() {
            Some(query) if !query.is_empty() => {
                let ranges = search_ranges(self.code.as_ref(), query, node_cx.search_is_regex, node_cx.search_is_case_sensitive);
                if ranges.is_empty() {
                    cs.styles.clone()
                } else {
                    let search_style = HighlightStyle {
                        background_color: Some(cx.theme().selection.alpha(0.35)),
                        ..Default::default()
                    };
                    let search_highlights = ranges
                        .into_iter()
                        .map(|range| (range, search_style))
                        .collect::<Vec<_>>();
                    gpui::combine_highlights(cs.styles.clone(), search_highlights).collect()
                }
            }
            _ => cs.styles.clone(),
        };
        drop(cs);

        div()
            .when(!options.is_last, |this| this.pb(style.paragraph_gap))
            .child(
                div()
                    .id(("codeblock", options.ix))
                    .p_3()
                    .rounded(cx.theme().radius)
                    .bg(cx.theme().muted)
                    .font_family(cx.theme().mono_font_family.clone())
                    .text_size(cx.theme().mono_font_size)
                    .relative()
                    .refine_style(&style.code_block)
                    .child(Inline::new(
                        "code",
                        self.inline_state.clone(),
                        vec![],
                        code_highlights,
                    ))
                    .when_some(node_cx.code_block_actions.clone(), |this, actions| {
                        this.child(
                            div()
                                .id(("codeblock-actions", options.ix))
                                .absolute()
                                .top_2()
                                .right_2()
                                .bg(cx.theme().muted)
                                .rounded(cx.theme().radius)
                                .child(actions(self, window, cx)),
                        )
                    }),
            )
            .into_any_element()
    }
}

/// A parsed frontmatter block rendered as a key-value table.
#[derive(Debug, Clone, PartialEq)]
pub(crate) struct FrontmatterBlock {
    /// Ordered key-value entries.
    pub entries: Vec<(String, FrontmatterValue)>,
    /// Byte span in the original source.
    pub span: Option<Span>,
}

/// Create an [`InlineState`] pre-loaded with text for selection support.
fn inline_state_for(text: &str) -> Arc<Mutex<InlineState>> {
    let state = Arc::new(Mutex::new(InlineState::default()));
    state.lock().unwrap().text = SharedString::from(text.to_string());
    state
}

/// A context for rendering nodes, contains link references.
#[derive(Default, Clone)]
pub(crate) struct NodeContext {
    /// The byte offset of the node in the original markdown text.
    /// Used for incremental updates.
    pub(crate) offset: usize,
    pub(crate) link_refs: HashMap<SharedString, LinkMark>,
    pub(crate) search_query: Option<SharedString>,
    pub(crate) search_is_regex: bool,
    pub(crate) search_is_case_sensitive: bool,
    pub(crate) style: TextViewStyle,
    pub(crate) code_block_actions: Option<Arc<CodeBlockActionsFn>>,
    /// The path of the source document, used for resolving relative image paths.
    pub(crate) document_path: Option<PathBuf>,
}

impl NodeContext {
    pub(super) fn add_ref(&mut self, identifier: SharedString, link: LinkMark) {
        self.link_refs.insert(identifier, link);
    }
}

impl PartialEq for NodeContext {
    fn eq(&self, other: &Self) -> bool {
        self.link_refs == other.link_refs
            && self.search_query == other.search_query
            && self.search_is_regex == other.search_is_regex
            && self.search_is_case_sensitive == other.search_is_case_sensitive
            && self.style == other.style
        // Note: code_block_buttons is intentionally not compared (closures can't be compared)
    }
}

fn search_ranges(text: &str, query: &str, is_regex: bool, case_sensitive: bool) -> Vec<Range<usize>> {
    if query.is_empty() || text.is_empty() {
        return vec![];
    }

    if is_regex {
        return search_ranges_regex(text, query, case_sensitive);
    }

    if case_sensitive {
        return search_ranges_case_sensitive(text, query);
    }

    // Fast path: ASCII-only case-insensitive search
    // This avoids expensive Unicode lowercase conversions for common cases
    let is_ascii_only = text.is_ascii() && query.is_ascii();

    if is_ascii_only {
        return search_ranges_ascii(text, query);
    }

    // Slow path: Unicode-aware search
    let query_lower = query.to_lowercase();
    let text_lower = text.to_lowercase();
    let mut ranges = Vec::new();

    // Build a mapping from lowercase byte positions to original byte positions
    let mut char_mapping: Vec<(usize, usize)> = Vec::with_capacity(text.chars().count() + 1);
    let mut orig_pos = 0;
    let mut lower_pos = 0;

    for c in text.chars() {
        char_mapping.push((orig_pos, lower_pos));
        orig_pos += c.len_utf8();
        for lc in c.to_lowercase() {
            lower_pos += lc.len_utf8();
        }
    }
    char_mapping.push((orig_pos, lower_pos)); // End sentinel

    let mut start = 0;
    while start < text_lower.len() {
        let Some(pos) = text_lower[start..].find(&query_lower) else {
            break;
        };
        let lower_start = start + pos;
        let lower_end = lower_start + query_lower.len();

        // Map lowercase positions back to original positions using binary search
        let orig_start = char_mapping
            .binary_search_by_key(&lower_start, |(_, lp)| *lp)
            .map(|i| char_mapping[i].0)
            .unwrap_or_else(|i| if i > 0 { char_mapping[i - 1].0 } else { lower_start });
        let orig_end = char_mapping
            .iter()
            .find(|(_, lp)| *lp >= lower_end)
            .map(|(op, _)| *op)
            .unwrap_or(lower_end);

        ranges.push(orig_start..orig_end);

        // Move past the current match
        start = lower_end;
        // Ensure we're at a valid char boundary
        while start < text_lower.len() && !text_lower.is_char_boundary(start) {
            start += 1;
        }
    }
    ranges
}

/// Fast ASCII-only case-insensitive search (no allocations for lowercase conversion)
fn search_ranges_ascii(text: &str, query: &str) -> Vec<Range<usize>> {
    let text_bytes = text.as_bytes();
    let query_bytes = query.as_bytes();
    let query_len = query_bytes.len();
    let mut ranges = Vec::new();

    if query_len > text_bytes.len() {
        return ranges;
    }

    let mut i = 0;
    while i <= text_bytes.len() - query_len {
        let mut matched = true;
        for j in 0..query_len {
            if !text_bytes[i + j].eq_ignore_ascii_case(&query_bytes[j]) {
                matched = false;
                break;
            }
        }
        if matched {
            ranges.push(i..i + query_len);
            i += query_len; // Move past the match
        } else {
            i += 1;
        }
    }
    ranges
}

/// Regex-based search using the query as a regex pattern.
fn search_ranges_regex(text: &str, query: &str, case_sensitive: bool) -> Vec<Range<usize>> {
    let Ok(re) = RegexBuilder::new(query)
        .case_insensitive(!case_sensitive)
        .nest_limit(50)
        .size_limit(1 << 20)
        .build()
    else {
        return vec![];
    };
    re.find_iter(text).map(|m| m.start()..m.end()).collect()
}

/// Case-sensitive substring search.
fn search_ranges_case_sensitive(text: &str, query: &str) -> Vec<Range<usize>> {
    let mut ranges = Vec::new();
    let mut start = 0;
    while let Some(pos) = text[start..].find(query) {
        let abs_start = start + pos;
        let abs_end = abs_start + query.len();
        ranges.push(abs_start..abs_end);
        start = abs_end;
        while start < text.len() && !text.is_char_boundary(start) {
            start += 1;
        }
    }
    ranges
}

impl Paragraph {
    fn render(
        &self,
        node_cx: &NodeContext,
        _window: &mut Window,
        cx: &mut App,
    ) -> impl IntoElement {
        let span = self.span;
        let children = &self.children;
        let search_query = node_cx.search_query.as_ref().map(|query| query.as_ref());
        let search_style = HighlightStyle {
            background_color: Some(cx.theme().selection.alpha(0.35)),
            ..Default::default()
        };

        let mut child_nodes: Vec<AnyElement> = vec![];

        let mut text = String::new();
        let mut highlights: Vec<(Range<usize>, HighlightStyle)> = vec![];
        let mut links: Vec<(Range<usize>, LinkMark)> = vec![];
        let mut offset = 0;

        let mut ix = 0;
        for inline_node in children {
            let text_len = inline_node.text.len();
            text.push_str(&inline_node.text);

            if let Some(image) = &inline_node.image {
                if !text.is_empty() {
                    let seg_state = Arc::new(Mutex::new(InlineState::default()));
                    seg_state.lock().unwrap().set_text(text.clone().into());
                    self.segment_states.lock().unwrap().push(seg_state.clone());
                    let inline_highlights = if let Some(query) = search_query {
                        let ranges = search_ranges(&text, query, node_cx.search_is_regex, node_cx.search_is_case_sensitive);
                        if ranges.is_empty() {
                            highlights.clone()
                        } else {
                            let search_highlights = ranges
                                .into_iter()
                                .map(|range| (range, search_style))
                                .collect::<Vec<_>>();
                            gpui::combine_highlights(highlights.clone(), search_highlights).collect()
                        }
                    } else {
                        highlights.clone()
                    };
                    child_nodes.push(
                        Inline::new(
                            ix,
                            seg_state,
                            links.clone(),
                            inline_highlights,
                        )
                        .into_any_element(),
                    );
                }
                // Use local path for file system images, otherwise use URL
                let image_element = if let Some(path) = &image.local_path {
                    img(path.clone())
                } else {
                    img(image.url.clone())
                };
                child_nodes.push(
                    image_element
                        .id(ix)
                        .object_fit(ObjectFit::Contain)
                        .max_w(relative(1.))
                        .max_h(px(2000.))
                        .when_some(image.width, |this, width| this.w(width))
                        .when_some(image.link.clone(), |this, link| {
                            let title = image.title();
                            this.cursor_pointer()
                                .tooltip(move |window, cx| {
                                    Tooltip::new(title.clone()).build(window, cx)
                                })
                                .on_click(move |_, _, cx| {
                                    cx.stop_propagation();
                                    cx.open_url(&link.url);
                                })
                        })
                        .into_any_element(),
                );

                text.clear();
                links.clear();
                highlights.clear();
                offset = 0;
            } else {
                let mut node_highlights = vec![];
                for (range, style) in &inline_node.marks {
                    let inner_range = (offset + range.start)..(offset + range.end);

                    let mut highlight = HighlightStyle::default();
                    if style.bold {
                        highlight.font_weight = Some(FontWeight::BOLD);
                    }
                    if style.italic {
                        highlight.font_style = Some(FontStyle::Italic);
                    }
                    if style.strikethrough {
                        highlight.strikethrough = Some(gpui::StrikethroughStyle {
                            thickness: gpui::px(1.),
                            ..Default::default()
                        });
                    }
                    if style.code {
                        highlight.background_color = Some(cx.theme().accent);
                    }

                    if let Some(mut link_mark) = style.link.clone() {
                        highlight.color = Some(cx.theme().link);
                        highlight.underline = Some(gpui::UnderlineStyle {
                            thickness: gpui::px(1.),
                            ..Default::default()
                        });

                        // convert link references, replace link
                        if let Some(identifier) = link_mark.identifier.as_ref()
                            && let Some(mark) = node_cx.link_refs.get(identifier) {
                                link_mark = mark.clone();
                            }

                        links.push((inner_range.clone(), link_mark));
                    }

                    node_highlights.push((inner_range, highlight));
                }

                highlights = gpui::combine_highlights(highlights, node_highlights).collect();
                offset += text_len;
            }
            ix += 1;
        }

        // Add the last text node
        if !text.is_empty() {
            // For the last node, we can move highlights instead of cloning
            let inline_highlights = if let Some(query) = search_query {
                let ranges = search_ranges(&text, query, node_cx.search_is_regex, node_cx.search_is_case_sensitive);
                if ranges.is_empty() {
                    highlights  // Move instead of clone for last node
                } else {
                    let search_highlights = ranges
                        .into_iter()
                        .map(|range| (range, search_style))
                        .collect::<Vec<_>>();
                    gpui::combine_highlights(highlights, search_highlights).collect()
                }
            } else {
                highlights  // Move instead of clone for last node
            };
            self.state.lock().unwrap().set_text(text.into());
            child_nodes
                .push(Inline::new(ix, self.state.clone(), links, inline_highlights).into_any_element());
        }

        div()
            .id(span.unwrap_or_default())
            .w_full()
            .min_w(px(0.))
            .max_w(relative(1.))
            .children(child_nodes)
    }
}

impl Paragraph {
    fn to_markdown(&self) -> String {
        let mut text = self
            .children
            .iter()
            .map(|text_node| {
                let mut text = text_node.text.to_string();
                for (range, style) in &text_node.marks {
                    if style.bold {
                        text = format!("**{}**", &text_node.text[range.clone()]);
                    }
                    if style.italic {
                        text = format!("*{}*", &text_node.text[range.clone()]);
                    }
                    if style.strikethrough {
                        text = format!("~~{}~~", &text_node.text[range.clone()]);
                    }
                    if style.code {
                        text = format!("`{}`", &text_node.text[range.clone()]);
                    }
                    if let Some(link) = &style.link {
                        text = format!("[{}]({})", &text_node.text[range.clone()], link.url);
                    }
                }

                if let Some(image) = &text_node.image {
                    let alt = image.alt.clone().unwrap_or_default();
                    let title = image
                        .title
                        .clone()
                        .map_or(String::new(), |t| format!(" \"{}\"", t));
                    text.push_str(&format!("![{}]({}{})", alt, image.url, title))
                }

                text
            })
            .collect::<Vec<_>>()
            .join("");

        text.push_str("\n\n");
        text
    }
}

impl BlockNode {
    /// Converts the node to markdown format.
    ///
    /// This is used to generate markdown for test.
    #[allow(dead_code)]
    pub(crate) fn to_markdown(&self) -> String {
        match self {
            BlockNode::Root { children, .. } => children
                .iter()
                .map(|child| child.to_markdown())
                .collect::<Vec<_>>()
                .join("\n\n"),
            BlockNode::Paragraph(paragraph) => paragraph.to_markdown(),
            BlockNode::Heading {
                level, children, ..
            } => {
                let hashes = "#".repeat(*level as usize);
                format!("{} {}", hashes, children.to_markdown())
            }
            BlockNode::Blockquote { children, .. } => {
                let content = children
                    .iter()
                    .map(|child| child.to_markdown())
                    .collect::<Vec<_>>()
                    .join("\n\n");

                content
                    .lines()
                    .map(|line| format!("> {}", line))
                    .collect::<Vec<_>>()
                    .join("\n")
            }
            BlockNode::List {
                children, ordered, ..
            } => children
                .iter()
                .enumerate()
                .map(|(i, child)| {
                    let prefix = if *ordered {
                        format!("{}. ", i + 1)
                    } else {
                        "- ".to_string()
                    };
                    format!("{}{}", prefix, child.to_markdown())
                })
                .collect::<Vec<_>>()
                .join("\n"),
            BlockNode::ListItem {
                children, checked, ..
            } => {
                let checkbox = if let Some(checked) = checked {
                    if *checked { "[x] " } else { "[ ] " }
                } else {
                    ""
                };
                format!(
                    "{}{}",
                    checkbox,
                    children
                        .iter()
                        .map(|child| child.to_markdown())
                        .collect::<Vec<_>>()
                        .join("\n")
                )
            }
            BlockNode::CodeBlock(code_block) => {
                format!(
                    "```{}\n{}\n```",
                    code_block.lang.clone().unwrap_or_default(),
                    code_block.code()
                )
            }
            BlockNode::Table(table) => {
                let header = table
                    .children
                    .first()
                    .map(|row| {
                        row.children
                            .iter()
                            .map(|cell| cell.children.to_markdown())
                            .collect::<Vec<_>>()
                            .join(" | ")
                    })
                    .unwrap_or_default();
                let alignments = table
                    .column_aligns
                    .iter()
                    .map(|align| {
                        match align {
                            ColumnumnAlign::Left => ":--",
                            ColumnumnAlign::Center => ":-:",
                            ColumnumnAlign::Right => "--:",
                        }
                        .to_string()
                    })
                    .collect::<Vec<_>>()
                    .join(" | ");
                let rows = table
                    .children
                    .iter()
                    .skip(1)
                    .map(|row| {
                        row.children
                            .iter()
                            .map(|cell| cell.children.to_markdown())
                            .collect::<Vec<_>>()
                            .join(" | ")
                    })
                    .collect::<Vec<_>>()
                    .join("\n");
                format!("{}\n{}\n{}", header, alignments, rows)
            }
            BlockNode::Frontmatter(fm) => {
                let lines: Vec<String> = fm
                    .entries
                    .iter()
                    .map(|(k, v)| format!("{}: {}", k, v))
                    .collect();
                format!("---\n{}\n---", lines.join("\n"))
            }
            BlockNode::Break { html, .. } => {
                if *html {
                    "<br>".to_string()
                } else {
                    "\n".to_string()
                }
            }
            BlockNode::Divider { .. } => "---".to_string(),
            BlockNode::Definition {
                identifier,
                url,
                title,
                ..
            } => {
                if let Some(title) = title {
                    format!("[{}]: {} \"{}\"", identifier, url, title)
                } else {
                    format!("[{}]: {}", identifier, url)
                }
            }
            BlockNode::Unknown => "".to_string(),
        }
        .trim()
        .to_string()
    }
}

impl BlockNode {
    /// Render frontmatter as a key-value property table with selectable text.
    fn render_frontmatter(
        fm: &FrontmatterBlock,
        options: &NodeRenderOptions,
        node_cx: &NodeContext,
        cx: &mut App,
    ) -> AnyElement {
        if fm.entries.is_empty() {
            return div().into_any_element();
        }

        use gpui::prelude::FluentBuilder as _;

        // Use foreground (not muted_foreground) for readability on the muted background.
        let text_color = cx.theme().foreground;
        let border_color = cx.theme().border;
        let mono_font = cx.theme().mono_font_family.clone();
        let mono_size = cx.theme().mono_font_size;
        let max_key_chars = fm
            .entries
            .iter()
            .map(|(k, _)| k.chars().count())
            .max()
            .unwrap_or(8);
        let key_width = px(max_key_chars as f32 * f32::from(mono_size) * 0.7 + 16.0);
        let total = fm.entries.len();

        let rows: Vec<AnyElement> = fm
            .entries
            .iter()
            .enumerate()
            .map(|(i, (key, value))| {
                let is_last = i + 1 == total;

                let key_el = div()
                    .text_color(text_color)
                    .font_family(mono_font.clone())
                    .text_size(mono_size)
                    .w(key_width)
                    .flex_shrink_0()
                    .font_weight(FontWeight::MEDIUM)
                    .child(Inline::new(
                        SharedString::from(format!("fm-key-{}-{}", options.ix, i)),
                        inline_state_for(key),
                        vec![],
                        vec![],
                    ));

                let val_el = div()
                    .text_color(text_color)
                    .font_family(mono_font.clone())
                    .text_size(mono_size)
                    .flex_1()
                    .child(Inline::new(
                        SharedString::from(format!("fm-val-{}-{}", options.ix, i)),
                        inline_state_for(&value.to_string()),
                        vec![],
                        vec![],
                    ));

                h_flex()
                    .w_full()
                    .py_1()
                    .when(!is_last, |this| this.border_b_1().border_color(border_color))
                    .child(key_el)
                    .child(val_el)
                    .into_any_element()
            })
            .collect();

        let yaml_text = std::rc::Rc::new({
            let mut s = String::from("---\n");
            for (k, v) in &fm.entries {
                s.push_str(&format!("{}: {}\n", k, v));
            }
            s.push_str("---");
            s
        });

        div()
            .pt_3()
            .when(!options.is_last, |this| this.pb(node_cx.style.paragraph_gap))
            .child(
                div()
                    .id(("fm", options.ix))
                    .w_full()
                    .border_1()
                    .border_color(border_color)
                    .rounded(cx.theme().radius)
                    .bg(cx.theme().muted)
                    .p_3()
                    .context_menu(move |menu, _window, _cx| {
                        menu.item(
                            gpui_component::menu::PopupMenuItem::new("Copy").on_click({
                                let yaml_text = std::rc::Rc::clone(&yaml_text);
                                move |_, _window, cx| {
                                    cx.write_to_clipboard(ClipboardItem::new_string(
                                        yaml_text.to_string(),
                                    ));
                                }
                            }),
                        )
                        .menu("Select All", Box::new(crate::workspace::SelectAll))
                    })
                    .child(v_flex().w_full().gap_1().children(rows)),
            )
            .into_any_element()
    }

    fn render_list_item(
        item: &BlockNode,
        ix: usize,
        options: NodeRenderOptions,
        node_cx: &NodeContext,
        window: &mut Window,
        cx: &mut App,
    ) -> AnyElement {
        match item {
            BlockNode::ListItem {
                children,
                spread,
                checked,
                ..
            } => v_flex()
                .id(("li", options.ix))
                .when(*spread, |this| this.child(div()))
                .children({
                    let mut items: Vec<Div> = Vec::with_capacity(children.len());

                    for (child_ix, child) in children.iter().enumerate() {
                        match child {
                            BlockNode::Paragraph { .. } => {
                                let last_not_list = child_ix > 0
                                    && !matches!(children[child_ix - 1], BlockNode::List { .. });

                                let text = child.render_block(
                                    NodeRenderOptions {
                                        depth: options.depth + 1,
                                        todo: checked.is_some(),
                                        is_last: true,
                                        ..options
                                    },
                                    node_cx,
                                    window,
                                    cx,
                                );

                                // merge content into last item.
                                if last_not_list
                                    && let Some(item_item) = items.last_mut() {
                                        item_item.extend(vec![
                                            div().overflow_hidden().child(text).into_any_element(),
                                        ]);
                                        continue;
                                    }

                                items.push(
                                    h_flex()
                                        .flex_1()
                                        .relative()
                                        .items_start()
                                        .content_start()
                                        .when(!options.todo && checked.is_none(), |this| {
                                            this.child(list_item_prefix(
                                                ix,
                                                options.ordered,
                                                options.depth,
                                            ))
                                        })
                                        .when_some(*checked, |this, checked| {
                                            // Todo list checkbox
                                            this.child(
                                                div()
                                                    .flex()
                                                    .mt(rems(0.4))
                                                    .mr_1p5()
                                                    .size(rems(0.875))
                                                    .items_center()
                                                    .justify_center()
                                                    .rounded(cx.theme().radius.half())
                                                    .border_1()
                                                    .border_color(cx.theme().primary)
                                                    .text_color(cx.theme().primary_foreground)
                                                    .when(checked, |this| {
                                                        this.bg(cx.theme().primary).child(
                                                            Icon::new(IconName::Check)
                                                                .size_2()
                                                                .text_xs(),
                                                        )
                                                    }),
                                            )
                                        })
                                        .child(div().overflow_hidden().child(text)),
                                );
                            }
                            BlockNode::List { .. } => {
                                items.push(div().ml(rems(1.)).child(child.render_block(
                                    NodeRenderOptions {
                                        depth: options.depth + 1,
                                        todo: checked.is_some(),
                                        is_last: true,
                                        ..options
                                    },
                                    node_cx,
                                    window,
                                    cx,
                                )));
                            }
                            _ => {}
                        }
                    }
                    items
                })
                .into_any_element(),
            _ => div().into_any_element(),
        }
    }

    fn render_table(
        item: &BlockNode,
        options: &NodeRenderOptions,
        node_cx: &NodeContext,
        window: &mut Window,
        cx: &mut App,
    ) -> impl IntoElement {
        const MAX_COLUMN_WEIGHT: usize = 150;
        const MIN_COLUMN_WEIGHT: usize = 6;
        const COLUMN_PADDING_WEIGHT: usize = 3;
        const ESTIMATED_CHAR_WIDTH_PX: f32 = 9.0;

        match item {
            BlockNode::Table(table) => {
                let column_count = table.max_column_count();
                let column_ratios = table.normalized_column_ratios(
                    column_count,
                    MIN_COLUMN_WEIGHT,
                    MAX_COLUMN_WEIGHT,
                    COLUMN_PADDING_WEIGHT,
                );
                let column_weights = (0..column_count)
                    .map(|index| {
                        table.column_width(index).clamp(MIN_COLUMN_WEIGHT, MAX_COLUMN_WEIGHT)
                            + COLUMN_PADDING_WEIGHT
                    })
                    .collect::<Vec<_>>();
                let total_column_weight = column_weights.iter().sum::<usize>().max(1);
                let estimated_table_width = total_column_weight as f32 * ESTIMATED_CHAR_WIDTH_PX;

                div()
                    .pb(rems(1.))
                    .flex()
                    .flex_col()
                    .items_start()
                    .child(
                        div()
                            .id(("table", options.ix))
                            .w_full()
                            .max_w(px(estimated_table_width))
                            .border_1()
                            .border_color(cx.theme().border)
                            .rounded(cx.theme().radius)
                            .children({
                                let mut rows = Vec::with_capacity(table.children.len());
                                for (row_ix, row) in table.children.iter().enumerate() {
                                    rows.push(
                                        div()
                                            .id("row")
                                            .w_full()
                                            .when(row_ix < table.children.len() - 1, |this| {
                                                this.border_b_1()
                                            })
                                            .border_color(cx.theme().border)
                                            .flex()
                                            .flex_row()
                                            .children({
                                                let mut cells =
                                                    Vec::with_capacity(row.children.len());
                                                for (ix, cell) in row.children.iter().enumerate() {
                                                    let align = table.column_align(ix);
                                                    let is_last_col = ix == row.children.len() - 1;
                                                    let fallback_ratio =
                                                        1.0 / row.children.len().max(1) as f32;
                                                    let column_ratio = column_ratios
                                                        .get(ix)
                                                        .copied()
                                                        .unwrap_or(fallback_ratio);

                                                    cells.push(
                                                        div()
                                                            .id("cell")
                                                            .flex()
                                                            .min_w(px(0.))
                                                            .when(
                                                                align == ColumnumnAlign::Center,
                                                                |this| this.justify_center(),
                                                            )
                                                            .when(
                                                                align == ColumnumnAlign::Right,
                                                                |this| this.justify_end(),
                                                            )
                                                            .w(Length::Definite(relative(
                                                                column_ratio,
                                                            )))
                                                            .px_2()
                                                            .py_1()
                                                            .when(!is_last_col, |this| {
                                                                this.border_r_1()
                                                                    .border_color(cx.theme().border)
                                                            })
                                                            .whitespace_normal()
                                                            .child(
                                                                cell.children
                                                                    .render(node_cx, window, cx),
                                                            ),
                                                    )
                                                }
                                                cells
                                            }),
                                    )
                                }
                                rows
                            }),
                    )
                    .into_any_element()
            }
            _ => div().into_any_element(),
        }
    }

    pub(crate) fn render_block(
        &self,
        options: NodeRenderOptions,
        node_cx: &NodeContext,
        window: &mut Window,
        cx: &mut App,
    ) -> AnyElement {
        let ix = options.ix;
        let mb = if options.in_list || options.is_last {
            rems(0.)
        } else {
            node_cx.style.paragraph_gap
        };

        match self {
            BlockNode::Root { children, .. } => div()
                .id(("div", ix))
                .children(children.iter().enumerate().map(move |(ix, node)| {
                    node.render_block(NodeRenderOptions { ix, ..options }, node_cx, window, cx)
                }))
                .into_any_element(),
            BlockNode::Paragraph(paragraph) => div()
                .id(("p", ix))
                .pb(mb)
                .child(paragraph.render(node_cx, window, cx))
                .into_any_element(),
            BlockNode::Heading {
                level, children, ..
            } => {
                let (text_size, font_weight) = match level {
                    1 => (rems(2.), FontWeight::BOLD),
                    2 => (rems(1.5), FontWeight::SEMIBOLD),
                    3 => (rems(1.25), FontWeight::SEMIBOLD),
                    4 => (rems(1.125), FontWeight::SEMIBOLD),
                    5 => (rems(1.), FontWeight::SEMIBOLD),
                    6 => (rems(1.), FontWeight::MEDIUM),
                    _ => (rems(1.), FontWeight::NORMAL),
                };

                let mut text_size = text_size.to_pixels(node_cx.style.heading_base_font_size);
                if let Some(f) = node_cx.style.heading_font_size.as_ref() {
                    text_size = (f)(*level, node_cx.style.heading_base_font_size);
                }

                div()
                    .id(SharedString::from(format!("h{}-{}", level, ix)))
                    .w_full()
                    .min_w(px(0.))
                    .max_w(relative(1.))
                    .pb(rems(0.3))
                    .whitespace_normal()
                    .text_size(text_size)
                    .font_weight(font_weight)
                    .child(children.render(node_cx, window, cx))
                    .into_any_element()
            }
            BlockNode::Blockquote { children, .. } => div()
                .w_full()
                .pb(mb)
                .child(
                    div()
                        .id(("blockquote", ix))
                        .w_full()
                        .text_color(cx.theme().muted_foreground)
                        .border_l_3()
                        .border_color(cx.theme().secondary_active)
                        .px_4()
                        .children({
                            let children_len = children.len();
                            children.iter().enumerate().map(move |(index, c)| {
                                let is_last = index == children_len - 1;
                                c.render_block(options.is_last(is_last), node_cx, window, cx)
                            })
                        }),
                )
                .into_any_element(),
            BlockNode::List {
                children, ordered, ..
            } => v_flex()
                .id((if *ordered { "ol" } else { "ul" }, ix))
                .pb(mb)
                .children({
                    let mut items = Vec::with_capacity(children.len());
                    let mut item_index = 0;
                    for (ix, item) in children.iter().enumerate() {
                        let is_item = item.is_list_item();

                        items.push(Self::render_list_item(
                            item,
                            item_index,
                            NodeRenderOptions {
                                ix,
                                ordered: *ordered,
                                ..options
                            },
                            node_cx,
                            window,
                            cx,
                        ));

                        if is_item {
                            item_index += 1;
                        }
                    }
                    items
                })
                .into_any_element(),
            BlockNode::CodeBlock(code_block) => code_block.render(&options, node_cx, window, cx),
            BlockNode::Table { .. } => {
                Self::render_table(self, &options, node_cx, window, cx).into_any_element()
            }
            BlockNode::Frontmatter(fm) => Self::render_frontmatter(fm, &options, node_cx, cx),
            BlockNode::Divider { .. } => div()
                .pb(mb)
                .child(div().id("divider").bg(cx.theme().border).h(px(2.)))
                .into_any_element(),
            BlockNode::Break { .. } => div().id("break").into_any_element(),
            BlockNode::Unknown | BlockNode::Definition { .. } => div().into_any_element(),
            _ => {
                if cfg!(debug_assertions) {
                    tracing::warn!("unknown implementation: {:?}", self);
                }

                div().into_any_element()
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::{Paragraph, Table, TableCell, TableRow};

    fn make_cell(text: &str) -> TableCell {
        let mut paragraph = Paragraph::default();
        paragraph.push_str(text);
        TableCell {
            children: paragraph,
            width: None,
        }
    }

    #[test]
    fn calculate_column_widths_uses_longest_cell_text() {
        let mut table = Table {
            children: vec![
                TableRow {
                    children: vec![make_cell("12345678"), make_cell("123456")],
                },
                TableRow {
                    children: vec![make_cell("123456789012"), make_cell("1234567")],
                },
            ],
            ..Default::default()
        };

        table.calculate_column_widths();

        assert_eq!(table.column_widths, vec![12, 7]);
    }

    #[test]
    fn normalized_column_ratios_respect_min_and_max_weights() {
        let table = Table {
            column_widths: vec![2, 10, 200],
            ..Default::default()
        };

        let ratios = table.normalized_column_ratios(3, 8, 150, 0);
        let expected = [8.0 / 168.0, 10.0 / 168.0, 150.0 / 168.0];

        assert_eq!(ratios.len(), 3);
        for (ratio, expected_ratio) in ratios.iter().zip(expected) {
            assert!((ratio - expected_ratio).abs() < 1e-6);
        }

        let total: f32 = ratios.iter().sum();
        assert!((total - 1.0).abs() < 1e-6);
    }

    #[test]
    fn normalized_column_ratios_fill_missing_columns_with_default_width() {
        let table = Table {
            column_widths: vec![20],
            ..Default::default()
        };

        let ratios = table.normalized_column_ratios(2, 8, 150, 0);

        assert_eq!(ratios.len(), 2);
        assert!((ratios[0] - (20.0 / 28.0)).abs() < 1e-6);
        assert!((ratios[1] - (8.0 / 28.0)).abs() < 1e-6);
    }
}
