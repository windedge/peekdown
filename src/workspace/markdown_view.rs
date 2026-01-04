use gpui::*;
use gpui_component::text::TextView;
use crate::state::document::Document;
use crate::state::theme::Theme;

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
        let theme = Theme::dark();

        div()
            .size_full()
            .bg(theme.bg_base)
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
    }
}

impl Render for MarkdownView {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        self.render_markdown(window, cx)
    }
}