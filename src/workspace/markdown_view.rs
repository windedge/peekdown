use std::cell::Cell;
use std::rc::Rc;

use gpui::*;
use crate::text::{TextView, TextViewState, TextViewStyle};
use crate::text::document::HeadingItem;
use crate::text::ElementExt;
use gpui_component::ActiveTheme;
use crate::state::document::Document;
use crate::state::config::{AppConfig, LayoutMode};

pub struct MarkdownView {
    #[allow(dead_code)] // Reserved for future file reload functionality
    document: Entity<Document>,
    config: Entity<AppConfig>,
    text_view_state: Entity<TextViewState>,
}

impl MarkdownView {
    pub fn new(document: Entity<Document>, config: Entity<AppConfig>, cx: &mut Context<Self>) -> Self {
        // Observe config changes to re-render when layout mode changes
        cx.observe(&config, |this, _, cx| {
            // Update scroll speed in TextViewState when config changes
            let speed = this.config.read(cx).appearance.scroll_speed;
            this.text_view_state.update(cx, |state, cx| {
                state.set_scroll_speed(speed, cx);
            });
            cx.notify();
        }).detach();

        // Create TextViewState once - content is parsed only at initialization
        let content = document.read(cx).content.clone();
        let scroll_speed = config.read(cx).appearance.scroll_speed;
        let text_view_state = cx.new(|cx| {
            TextViewState::markdown(content.as_ref(), cx)
                .scroll_speed(scroll_speed)
        });

        Self {
            document,
            config,
            text_view_state,
        }
    }

    /// Get the headings from the document for outline display.
    pub fn headings(&self, cx: &App) -> Vec<HeadingItem> {
        self.text_view_state.read(cx).headings()
    }

    /// Scroll to a specific heading by block index.
    pub fn scroll_to_heading(&self, block_index: usize, cx: &App) {
        self.text_view_state.read(cx).scroll_to_block(block_index);
    }

    /// Get a reference to the text view state entity.
    pub fn text_view_state(&self) -> &Entity<TextViewState> {
        &self.text_view_state
    }

    /// Get the source text for search.
    pub fn source_text(&self, cx: &App) -> gpui::SharedString {
        self.text_view_state.read(cx).source_text()
    }

    /// Get block spans for search matching.
    pub fn block_spans(&self, cx: &App) -> Vec<(usize, std::ops::Range<usize>)> {
        self.text_view_state.read(cx).block_spans()
    }
}

impl Render for MarkdownView {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let theme = cx.theme();
        let layout_mode = self.config.read(cx).appearance.layout;
        let scroll_speed = self.config.read(cx).appearance.scroll_speed;

        let content_max_width = px(900.);
        let min_padding = px(32.);
        let text_style = match layout_mode {
            LayoutMode::Centered => TextViewStyle::default().content_max_width(content_max_width),
            LayoutMode::FullWidth => TextViewStyle::default(),
        };

        // Use Rc<Cell> to share container bounds between on_prepaint and on_scroll_wheel
        let container_width = Rc::new(Cell::new(px(0.)));
        let container_origin_x = Rc::new(Cell::new(px(0.)));
        let container_width_for_prepaint = container_width.clone();
        let container_width_for_scroll = container_width.clone();
        let container_origin_x_for_prepaint = container_origin_x.clone();
        let container_origin_x_for_scroll = container_origin_x.clone();

        let text_state = self.text_view_state.clone();

        div()
            .id("markdown-container")
            .relative()
            .size_full()
            .bg(theme.background)
            // Use on_prepaint to get actual container bounds
            .on_prepaint(move |bounds, _, _cx| {
                container_width_for_prepaint.set(bounds.size.width);
                container_origin_x_for_prepaint.set(bounds.origin.x);
            })
            .on_scroll_wheel(move |event, _window, cx| {
                // Get current container width
                let width = container_width_for_scroll.get();
                let origin_x = container_origin_x_for_scroll.get();

                // ScrollWheelEvent position is in window coordinates
                let local_x = event.position.x - origin_x;
                if local_x < px(0.) || local_x > width {
                    return;
                }

                let side_padding = match layout_mode {
                    LayoutMode::Centered => {
                        if width > content_max_width + min_padding * 2.0 {
                            (width - content_max_width) / 2.0
                        } else {
                            min_padding
                        }
                    }
                    LayoutMode::FullWidth => min_padding,
                };

                // Check if cursor is in the padding area (left or right)
                let in_padding = local_x < side_padding
                    || local_x > (width - side_padding);
                if !in_padding {
                    return;
                }

                // Match scroll speed with TextView
                let delta = event.delta.pixel_delta(px(20.)).y;
                let distance = -delta * scroll_speed;
                text_state.read(cx).scroll_by(distance);
                cx.stop_propagation();
            })
            .child(
                TextView::new(&self.text_view_state)
                    .style(text_style)
                    .scrollable(true)
                    .scroll_speed(scroll_speed)
                    .selectable(true)
                    .pb_8()
                    .px(min_padding)
                    .text_size(rems(1.0))
            )
    }
}
