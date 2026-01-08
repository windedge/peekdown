//! Outline sidebar component for document navigation.

use gpui::*;
use gpui::prelude::FluentBuilder;
use gpui_component::{ActiveTheme, scroll::ScrollableElement, v_flex, button::Button, button::ButtonVariants, Icon, IconName, Sizable};
use crate::text::document::HeadingItem;

/// Default and minimum width for the outline sidebar.
const DEFAULT_WIDTH: f32 = 200.0;
const MIN_WIDTH: f32 = 120.0;
const MAX_WIDTH: f32 = 400.0;
const RESIZE_HANDLE_WIDTH: f32 = 6.0;

/// Callback type for when a heading is clicked.
pub type OnHeadingClick = Box<dyn Fn(usize, &mut Window, &mut App) + 'static>;

/// Callback type for when width changes.
pub type OnWidthChange = Box<dyn Fn(f32, &mut App) + 'static>;

/// Callback type for when close button is clicked.
pub type OnOutlineClose = Box<dyn Fn(&mut Window, &mut App) + 'static>;

/// Outline sidebar view showing document headings.
pub struct OutlineView {
    headings: Vec<HeadingItem>,
    on_click: Option<OnHeadingClick>,
    on_width_change: Option<OnWidthChange>,
    on_close: Option<OnOutlineClose>,
    width: f32,
    is_resizing: bool,
    resize_start_x: f32,
    resize_start_width: f32,
}

impl OutlineView {
    pub fn new(headings: Vec<HeadingItem>) -> Self {
        Self {
            headings,
            on_click: None,
            on_width_change: None,
            on_close: None,
            width: DEFAULT_WIDTH,
            is_resizing: false,
            resize_start_x: 0.0,
            resize_start_width: DEFAULT_WIDTH,
        }
    }

    pub fn on_click(mut self, callback: impl Fn(usize, &mut Window, &mut App) + 'static) -> Self {
        self.on_click = Some(Box::new(callback));
        self
    }

    pub fn on_close(mut self, callback: impl Fn(&mut Window, &mut App) + 'static) -> Self {
        self.on_close = Some(Box::new(callback));
        self
    }

    /// Update click handler
    pub fn set_on_click(
        &mut self,
        callback: impl Fn(usize, &mut Window, &mut App) + 'static,
        cx: &mut Context<Self>,
    ) {
        self.on_click = Some(Box::new(callback));
        cx.notify();
    }

    #[allow(dead_code)]
    pub fn on_width_change(mut self, callback: impl Fn(f32, &mut App) + 'static) -> Self {
        self.on_width_change = Some(Box::new(callback));
        self
    }

    #[allow(dead_code)]
    pub fn width(mut self, width: f32) -> Self {
        self.width = width.clamp(MIN_WIDTH, MAX_WIDTH);
        self
    }

    /// Update headings list
    pub fn set_headings(&mut self, headings: Vec<HeadingItem>) {
        self.headings = headings;
    }

    /// Update width and notify
    pub fn set_width(&mut self, width: f32, cx: &mut Context<Self>) {
        self.width = width.clamp(MIN_WIDTH, MAX_WIDTH);
        cx.notify();
    }

    /// Check if currently resizing
    pub fn is_resizing(&self) -> bool {
        self.is_resizing
    }

    /// Handle mouse move during resize (called from workspace)
    pub fn handle_resize_move(&mut self, mouse_x: f32, cx: &mut Context<Self>) {
        if self.is_resizing {
            let delta = mouse_x - self.resize_start_x;
            let new_width = (self.resize_start_width + delta).clamp(MIN_WIDTH, MAX_WIDTH);
            self.width = new_width;
            if let Some(on_width_change) = &self.on_width_change {
                on_width_change(new_width, cx);
            }
            cx.notify();
        }
    }

    /// End resize operation
    pub fn end_resize(&mut self, cx: &mut Context<Self>) {
        self.is_resizing = false;
        cx.notify();
    }
}

impl Render for OutlineView {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let theme = cx.theme().clone();
        let width = self.width;
        let is_resizing = self.is_resizing;

        div()
            .id("outline-container")
            .flex()
            .flex_row()
            .flex_shrink_0()
            .h_full()
            .child(
                // Main outline content
                v_flex()
                    .id("outline-view")
                    .w(px(width))
                    .h_full()
                    .flex_shrink_0()
                    .bg(theme.sidebar)
                    .child(
                        // Header - fixed at top
                        div()
                            .flex_shrink_0()
                            .px_3()
                            .py_2()
                            .flex()
                            .flex_row()
                            .items_center()
                            .justify_between()
                            .child(
                                div()
                                    .text_xs()
                                    .text_color(theme.muted_foreground)
                                    .font_weight(FontWeight::SEMIBOLD)
                                    .child("OUTLINE")
                            )
                            .child(
                                Button::new("outline-close-btn")
                                    .icon(Icon::new(IconName::Close))
                                    .ghost()
                                    .xsmall()
                                    .on_click(cx.listener(|this, _, window, cx| {
                                        if let Some(on_close) = &this.on_close {
                                            on_close(window, cx);
                                        }
                                    }))
                            )
                    )
                    .child(
                        // Scrollable content area
                        div()
                            .id("outline-scroll")
                            .relative()
                            .flex_grow()
                            .min_h_0() // Critical: allow flex item to shrink below content size
                            .overflow_hidden()
                            .child(
                                div()
                                    .id("outline-items")
                                    .size_full()
                                    .when(self.headings.is_empty(), |this| {
                                        this.child(
                                            div()
                                                .px_3()
                                                .py_2()
                                                .text_sm()
                                                .text_color(theme.muted_foreground)
                                                .child("No headings")
                                        )
                                    })
                                    .children(self.headings.iter().enumerate().map(|(ix, heading)| {
                                        let level = heading.level;
                                        let block_index = heading.block_index;
                                        let indent = ((level - 1) as f32) * 12.0;

                                        div()
                                            .id(("heading", ix))
                                            .w_full()
                                            .px_3()
                                            .py_1()
                                            .pl(px(12.0 + indent))
                                            .text_sm()
                                            .cursor_pointer()
                                            .overflow_hidden()
                                            .text_ellipsis()
                                            .whitespace_nowrap()
                                            .text_color(theme.foreground)
                                            .hover(|s| s.bg(theme.accent))
                                            .on_click(cx.listener(move |this, _, window, cx| {
                                                if let Some(on_click) = &this.on_click {
                                                    on_click(block_index, window, cx);
                                                }
                                            }))
                                            .child(heading.text.clone())
                                    }))
                                    .overflow_y_scrollbar()
                            )
                    )
            )
            .child(
                // Resize handle
                div()
                    .id("outline-resize-handle")
                    .w(px(RESIZE_HANDLE_WIDTH))
                    .h_full()
                    .cursor_col_resize()
                    .bg(theme.border)
                    .hover(|s| s.bg(theme.primary))
                    .when(is_resizing, |this| this.bg(theme.primary))
                    .on_mouse_down(MouseButton::Left, cx.listener(|this, event: &MouseDownEvent, _, cx| {
                        this.is_resizing = true;
                        this.resize_start_x = f32::from(event.position.x);
                        this.resize_start_width = this.width;
                        cx.notify();
                    }))
            )
    }
}
