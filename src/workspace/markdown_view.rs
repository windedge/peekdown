use gpui::*;
use crate::state::document::{Document, Block};
use crate::state::theme::Theme;

pub struct MarkdownView {
    document: Entity<Document>,
}

impl MarkdownView {
    pub fn new(document: Entity<Document>) -> Self {
        Self { document }
    }

    fn render_markdown(&self, cx: &mut Context<Self>) -> impl IntoElement {
        let document = self.document.read(cx);
        let blocks = &document.blocks;
        let theme = Theme::dark();

        div()
            .flex()
            .flex_col()
            .size_full()
            .bg(theme.bg_base)
            .id("markdown-content")
            .overflow_scroll()
            .child(
                div()
                    .flex()
                    .flex_col()
                    .p_8()
                    .gap_4()
                    .children(blocks.iter().map(|block| render_block(block, &theme)))
            )
    }
}

fn render_block(block: &Block, theme: &Theme) -> Div {
    match block {
        Block::Header(text, level) => {
            let size = match level {
                1 => rems(2.25),
                2 => rems(1.75),
                3 => rems(1.5),
                _ => rems(1.25),
            };
            div().child(
                div()
                    .child(text.clone())
                    .text_color(theme.text_primary)
                    .font_weight(FontWeight::BOLD)
                    .text_size(size)
                    .pb_2()
            )
        }
        Block::Paragraph(text) => {
             div().child(
                div()
                    .child(text.clone())
                    .text_color(theme.text_secondary)
                    .text_size(rems(1.0))
                    .line_height(rems(1.6))
            )
        }
        Block::Code(code, lang) => {
            div()
                .p_4()
                .bg(rgb(0x2d2d2d))
                .rounded_md()
                .border_1()
                .border_color(theme.border)
                .child(
                    div()
                        .flex()
                        .justify_between()
                        .child(
                             div().child(code.clone())
                                .text_color(theme.text_secondary)
                                .font_family("Consolas")
                                .text_size(rems(0.9))
                        )
                        .child(
                             div().child(lang.clone())
                                .text_xs()
                                .text_color(rgb(0x888888))
                        )
                )
        }
        Block::List(items, _is_ordered) => {
             div()
                .flex()
                .flex_col()
                .gap_2()
                .pl_4() // Indent
                .children(items.iter().map(|item| render_block(item, theme)))
        }
        Block::ListItem(children) => {
             div()
                .flex()
                .flex_col()
                .children(children.iter().map(|c| render_block(c, theme)))
        }
        Block::Quote(children) => {
             div()
                .border_l_4()
                .border_color(rgb(0x666666))
                .pl_4()
                .italic()
                .children(children.iter().map(|c| render_block(c, theme)))
        }
        Block::Rule => {
            div().h_px().bg(theme.border).my_4()
        }
        Block::Image(src, _alt) => {
             // TODO: Resolve relative paths and support file:// scheme
             div().child(
                 img(src.clone())
                    .w_full()
                    .rounded_md()
             )
        }
        // _ => div() // Fallback
    }
}

impl Render for MarkdownView {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        self.render_markdown(cx)
    }
}
