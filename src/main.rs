#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use gpui::*;
use gpui_component_assets::Assets;
use crate::state::config::AppConfig;

mod services;
mod state;
mod text;
mod workspace;
mod registry;
mod ipc;
mod file_watcher;

fn main() {
    tracing_subscriber::fmt::init();

    let args: Vec<String> = std::env::args().collect();

    if args.len() > 1 && args[1] == "--register" {
        if let Err(e) = registry::register_file_association() {
            eprintln!("Failed to register file association: {:?}", e);
            std::process::exit(1);
        }
        return;
    }

    let mut initial_files: Vec<std::path::PathBuf> = args.iter().skip(1).map(std::path::PathBuf::from).collect();

    // Attempt IPC
    // Send OpenFiles or FocusWindow
    let msg = if initial_files.is_empty() {
        ipc::IpcMessage::FocusWindow
    } else {
        ipc::IpcMessage::OpenFiles(initial_files.clone())
    };

    if let Ok(_) = ipc::send_message(msg) {
        return;
    }

    // We are the server
    let (tx, rx) = smol::channel::unbounded();
    if let Err(e) = ipc::spawn_ipc_server(tx) {
        eprintln!("Failed to spawn IPC server: {}", e);
    }

    // If we didn't send via IPC (server mode), but we were launched with files, check if we need to filter out files that failed to send?
    // No, if send_message failed, we assume we are the server, so we open them ourselves.

    Application::new()
        .with_assets(Assets)
        .run(move |cx: &mut App| {
            gpui_component::init(cx);
            crate::text::init(cx);
            let config_model = cx.new(|_| AppConfig::load());
            workspace::init(cx, initial_files.clone(), Some(rx), config_model);
        });
}
