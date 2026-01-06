use gpui::*;
use gpui_component::text::markdown;
use gpui_component::ActiveTheme;
use crate::state::document::Document;

pub struct MarkdownView {
    document: Entity<Document>,
    cached_width: Pixels,
    cached_padding: Pixels,
}

impl MarkdownView {
    pub fn new(document: Entity<Document>) -> Self {
        Self {
            document,
            cached_width: px(0.),
            cached_padding: px(32.),
        }
    }

    fn render_markdown(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let document = self.document.read(cx);
        let content = &document.content;
        let theme = cx.theme();

        let window_width = window.viewport_size().width;

        // Only recalculate padding when window width changes significantly
        if (window_width - self.cached_width).abs() > px(10.) {
            self.cached_width = window_width;
            let content_max_width = px(900.);
            let min_padding = px(32.);
            self.cached_padding = if window_width > content_max_width + min_padding * 2.0 {
                (window_width - content_max_width) / 2.0
            } else {
                min_padding
            };
        }

        div()
            .id("markdown-container")
            .size_full()
            .bg(theme.background)
            .child(
                markdown(content.clone())
                    .scrollable(true)
                    .selectable(true)
                    .py_8()
                    .pl(self.cached_padding)
                    .pr(self.cached_padding)
                    .text_size(rems(1.0))
            )
    }
}

impl Render for MarkdownView {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        self.render_markdown(window, cx)
    }
}
