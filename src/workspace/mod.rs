//! UI Views and layout.

use gpui::*;

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

struct WorkspaceView;

impl WorkspaceView {
    pub fn new() -> Self {
        Self
    }
}

impl Render for WorkspaceView {
    fn render(&mut self, _window: &mut Window, _cx: &mut Context<Self>) -> impl IntoElement {
        div()
            .flex()
            .flex_col()
            .size_full()
            .bg(rgb(0x1e1e1e)) // Dark background base
            .text_color(rgb(0xffffff))
            .child(
                // Header
                div()
                    .flex()
                    .items_center()
                    .h_10()
                    .px_4()
                    .bg(rgb(0x2d2d2d))
                    .border_b_1()
                    .border_color(rgb(0x3d3d3d))
                    .child("Peekdown Header"),
            )
            .child(
                // Body
                div()
                    .flex()
                    .flex_grow()
                    .p_4()
                    .child("Markdown Content Area"),
            )
            .child(
                // Footer
                div()
                    .flex()
                    .items_center()
                    .h_8()
                    .px_4()
                    .bg(rgb(0x252526))
                    .border_t_1()
                    .border_color(rgb(0x3d3d3d))
                    .text_xs()
                    .child("Ready"),
            )
    }
}