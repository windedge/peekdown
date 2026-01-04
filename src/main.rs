use gpui::*;
use assets::Assets;

mod assets;
mod services;
mod state;
mod workspace;

fn main() {
    tracing_subscriber::fmt::init();
    
    let args: Vec<String> = std::env::args().collect();
    let initial_file = args.get(1).map(std::path::PathBuf::from);

    Application::new()
        .with_assets(Assets)
        .run(move |cx: &mut App| {
            workspace::init(cx, initial_file.clone());
        });
}
