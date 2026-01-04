//! UI Views and layout.

use gpui::*;
use crate::state::theme::Theme;

mod welcome;
use welcome::WelcomeView;

pub fn init(cx: &mut App) {
    cx.open_window(
        WindowOptions {
            titlebar: Some(TitlebarOptions {
                title: Some("Peekdown".into()),
                ..Default::default()
            }),
            window_bounds: Some(WindowBounds::Windowed(Bounds::centered(
                None,
                size(1024.0.into(), 768.0.into()),
                cx,
            ))),
            ..Default::default()
        },
        |_, cx| {
            cx.new(|_cx| WorkspaceView::new())
        },
    )
    .unwrap();
}

struct WorkspaceView {
    theme: Theme,
}

impl WorkspaceView {
    pub fn new() -> Self {
        Self {
            theme: Theme::dark(),
        }
    }
}

impl Render for WorkspaceView {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let theme = &self.theme;
        
        div()
            .flex()
            .flex_col()
            .size_full()
            .bg(theme.bg_base)
            .text_color(theme.text_primary)
            .child(
                // Header
                div()
                    .flex()
                    .items_center()
                    .h_10()
                    .px_4()
                    .bg(theme.bg_header)
                    .border_b_1()
                    .border_color(theme.border)
                    .child("Peekdown Header"),
            )
            .child(
                // Body
                div()
                    .flex()
                    .flex_grow()
                    .child(cx.new(|_cx| WelcomeView::new())),
            )
            .child(
                // Footer
                div()
                    .flex()
                    .items_center()
                    .h_8()
                    .px_4()
                    .bg(theme.bg_footer)
                    .border_t_1()
                    .border_color(theme.border)
                    .text_xs()
                    .child("Ready"),
            )
    }
}