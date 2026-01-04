use gpui::*;
use pulldown_cmark::{Parser, Options, Event, Tag};
use crate::state::document::Document;
use crate::state::theme::Theme;

pub struct MarkdownView {
    document: Entity<Document>,
}

enum Block {
    Header(String, u32),
    Paragraph(String),
}

impl MarkdownView {
    pub fn new(document: Entity<Document>) -> Self {
        Self { document }
    }

    fn render_markdown(&self, cx: &mut Context<Self>) -> Div {
        let document = self.document.read(cx);
        let content = &document.content;
        
        let mut options = Options::empty();
        options.insert(Options::ENABLE_STRIKETHROUGH);
        options.insert(Options::ENABLE_TABLES);
        options.insert(Options::ENABLE_TASKLISTS);

        let parser = Parser::new_ext(content, options);
        let theme = Theme::dark();

        let mut blocks = Vec::new();
        let mut current_text = String::new();
        let mut current_level = 0;
        let mut in_header = false;

        for event in parser {
            match event {
                Event::Start(Tag::Heading(level, _, _)) => {
                    in_header = true;
                    // pulldown_cmark::HeadingLevel::H1 -> 1, etc.
                    current_level = match level {
                        pulldown_cmark::HeadingLevel::H1 => 1,
                        pulldown_cmark::HeadingLevel::H2 => 2,
                        pulldown_cmark::HeadingLevel::H3 => 3,
                        pulldown_cmark::HeadingLevel::H4 => 4,
                        pulldown_cmark::HeadingLevel::H5 => 5,
                        pulldown_cmark::HeadingLevel::H6 => 6,
                    };
                    current_text.clear();
                }
                Event::End(Tag::Heading(_, _, _)) => {
                    in_header = false;
                    blocks.push(Block::Header(current_text.clone(), current_level));
                    current_text.clear();
                }
                Event::Start(Tag::Paragraph) => {
                    current_text.clear();
                }
                Event::End(Tag::Paragraph) => {
                    if !current_text.trim().is_empty() {
                        blocks.push(Block::Paragraph(current_text.clone()));
                    }
                    current_text.clear();
                }
                Event::Text(text) => {
                    current_text.push_str(&text);
                }
                Event::Code(text) => {
                    current_text.push_str(&text);
                }
                Event::SoftBreak | Event::HardBreak => {
                    current_text.push(' ');
                }
                _ => {}
            }
        }

        let mut doc_div = div()
            .flex()
            .flex_col()
            .size_full()
            .p_8()
            .gap_4()
            .bg(theme.bg_base)
            .overflow_y_scroll();

        for block in blocks {
            match block {
                Block::Header(text, level) => {
                    let size = match level {
                        1 => text_3xl(),
                        2 => text_2xl(),
                        3 => text_xl(),
                        _ => text_lg(),
                    };
                    doc_div = doc_div.child(
                        div()
                            .child(text)
                            .text_color(theme.text_primary)
                            .font_weight(FontWeight::BOLD)
                            .with_text_style(size)
                    );
                }
                Block::Paragraph(text) => {
                    doc_div = doc_div.child(
                        div()
                            .child(text)
                            .text_color(theme.text_secondary)
                            .text_base()
                    );
                }
            }
        }

        doc_div
    }
}

impl Render for MarkdownView {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        self.render_markdown(cx)
    }
}
