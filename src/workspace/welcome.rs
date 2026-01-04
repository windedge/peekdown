use gpui::*;
use crate::state::theme::Theme;

pub struct WelcomeView;

impl WelcomeView {
    pub fn new() -> Self {
        Self
    }
}

impl Render for WelcomeView {
    fn render(&mut self, _window: &mut Window, _cx: &mut Context<Self>) -> impl IntoElement {
        let theme = Theme::dark();
        
        div()
            .flex()
            .flex_col()
            .items_center()
            .justify_center()
            .size_full()
            .child(
                div()
                    .flex()
                    .flex_col()
                    .items_center()
                    .gap_4()
                    .child(
                        div()
                            .text_xl()
                            .font_weight(FontWeight::BOLD)
                            .text_color(theme.text_primary)
                            .child("Peekdown")
                    )
                    .child(
                        div()
                            .text_sm()
                            .text_color(theme.text_secondary)
                            .child("Markdown Previewer")
                    )
                    .child(
                        div()
                            .flex()
                            .flex_col()
                            .gap_2()
                            .mt_8()
                            .child(
                                div()
                                    .flex()
                                    .gap_4()
                                    .text_xs()
                                    .text_color(theme.text_secondary)
                                    .child(div().child("Open File"))
                                    .child(div().child("Ctrl+O"))
                            )
                            .child(
                                div()
                                    .flex()
                                    .gap_4()
                                    .text_xs()
                                    .text_color(theme.text_secondary)
                                    .child(div().child("Quit"))
                                    .child(div().child("Ctrl+Q"))
                            )
                    )
            )
    }
}
