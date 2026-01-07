use gpui::*;
use crate::text::{TextView, TextViewState};
use gpui_component::ActiveTheme;
use crate::state::document::Document;
use crate::state::config::{AppConfig, LayoutMode};

pub struct MarkdownView {
    #[allow(dead_code)] // Reserved for future file reload functionality
    document: Entity<Document>,
    config: Entity<AppConfig>,
    text_view_state: Entity<TextViewState>,
    cached_width: Pixels,
    cached_padding: Pixels,
    cached_layout: LayoutMode,
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
            cached_width: px(0.),
            cached_padding: px(32.),
            cached_layout: LayoutMode::default(),
        }
    }
}

impl Render for MarkdownView {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let theme = cx.theme();
        let layout_mode = self.config.read(cx).appearance.layout;
        let scroll_speed = self.config.read(cx).appearance.scroll_speed;

        let window_width = window.viewport_size().width;

        // Recalculate padding when window width or layout mode changes
        let width_changed = (window_width - self.cached_width).abs() > px(10.);
        let layout_changed = layout_mode != self.cached_layout;

        if width_changed || layout_changed {
            self.cached_width = window_width;
            self.cached_layout = layout_mode;
            self.cached_padding = match layout_mode {
                LayoutMode::Centered => {
                    let content_max_width = px(900.);
                    let min_padding = px(32.);
                    if window_width > content_max_width + min_padding * 2.0 {
                        (window_width - content_max_width) / 2.0
                    } else {
                        min_padding
                    }
                }
                LayoutMode::FullWidth => px(32.),
            };
        }

        let padding = self.cached_padding;

        div()
            .id("markdown-container")
            .relative()
            .size_full()
            .bg(theme.background)
            .child(
                TextView::new(&self.text_view_state)
                    .scrollable(true)
                    .scroll_speed(scroll_speed)
                    .selectable(true)
                    .py_8()
                    .pl(padding)
                    .pr(padding)
                    .text_size(rems(1.0))
            )
    }
}
