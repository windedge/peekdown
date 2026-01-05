use gpui::*;
use gpui_component::text::markdown;
use gpui_component::ActiveTheme;
use crate::state::document::Document;

pub struct MarkdownView {
    document: Entity<Document>,
}

impl MarkdownView {
    pub fn new(document: Entity<Document>) -> Self {
        Self { document }
    }

    fn render_markdown(&self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let document = self.document.read(cx);
        let content = &document.content;
        let theme = cx.theme();

        div()
            .id("markdown-container")
            .size_full()
            .bg(theme.background)
            .flex()
            .justify_center()
            .child(
                div()
                    .w_full()
                    .h_full()
                    .max_w(px(1200.))
                    .min_w(px(0.))
                    .child(
                        markdown(content.clone())
                            .scrollable(true)
                            .selectable(true)
                            .p_8()
                            .text_size(rems(1.0))
                    )
            )
    }
}

impl Render for MarkdownView {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        self.render_markdown(window, cx)
    }
}
