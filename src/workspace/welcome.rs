use gpui::*;
use gpui_component::{ActiveTheme, StyledExt};

pub struct WelcomeView {}

impl WelcomeView {
    pub fn new() -> Self {
        Self {}
    }
}

impl Render for WelcomeView {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let theme = cx.theme();
        
        div()
            .flex()
            .flex_col()
            .items_center()
            .justify_center()
            .size_full()
            .bg(theme.background)
            .text_color(theme.foreground)
            .gap_4()
            .child(
                div()
                    .text_xl()
                    .font_bold()
                    .child("Welcome to Peekdown")
            )
            .child(
                div()
                    .text_sm()
                    .text_color(theme.muted_foreground)
                    .child("Drag and drop a Markdown file here, or use the command line.")
            )
    }
}