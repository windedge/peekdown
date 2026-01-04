#![warn(clippy::all)]

mod app;
mod core;
mod io;
mod parser;
mod ui;

fn main() -> eframe::Result {
    tracing_subscriber::fmt::init();
    app::run()
}
