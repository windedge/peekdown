use gpui::*;
use assets::Assets;

mod assets;
mod services;
mod state;
mod workspace;

fn main() {
    tracing_subscriber::fmt::init();

    Application::new()
        .with_assets(Assets)
        .run(|cx: &mut App| {
            workspace::init(cx);
        });
}
