//! UI Views and layout.

use gpui::*;

pub fn init(cx: &mut App) {
    cx.open_window(WindowOptions::default(), |_, cx| {
        cx.new(|_cx| {
            EmptyView
        })
    }).unwrap();
}

// Temporary empty view for scaffolding
struct EmptyView;

impl Render for EmptyView {
    fn render(&mut self, _window: &mut Window, _cx: &mut Context<Self>) -> impl IntoElement {
        div().child("Peekdown - GPUI Skeleton")
    }
}