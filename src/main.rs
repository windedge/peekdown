use gpui::*;
use assets::Assets;
use gpui_component::theme::{Theme, ThemeMode};

mod assets;
mod services;
mod state;
mod workspace;

fn main() {
    tracing_subscriber::fmt::init();
    
    let args: Vec<String> = std::env::args().collect();
    let initial_files: Vec<std::path::PathBuf> = args.iter().skip(1).map(std::path::PathBuf::from).collect();

    Application::new()
        .with_assets(Assets)
        .run(move |cx: &mut App| {
            gpui_component::init(cx);
            Theme::change(ThemeMode::Dark, None, cx);
            workspace::init(cx, initial_files.clone());
        });
}
