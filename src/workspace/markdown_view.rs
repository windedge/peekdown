use gpui::*;
use gpui_component::text::TextView;
use gpui_component::ActiveTheme;
use crate::state::document::Document;

pub struct MarkdownView {
    document: Entity<Document>,
}

impl MarkdownView {
    pub fn new(document: Entity<Document>) -> Self {
        Self { document }
    }

    fn render_markdown(&self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let document = self.document.read(cx);
        let content = &document.content;
        let theme = cx.theme();

        div()
            .size_full()
            .bg(theme.background)
            .flex()
            .justify_center() // Center horizontally
            .child(
                div()
                    .w_full()
                    .max_w(px(1200.)) // Limit reading width
                    .h_full()
                    .child(
                        TextView::markdown(
                            ElementId::Name("markdown".into()), 
                            content.clone(),
                            window,
                            cx
                        )
                            .scrollable(true)
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