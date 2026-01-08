//! Shell and file system services.

use std::path::Path;
use std::process::Command;

/// Open the containing folder in system file explorer.
pub fn open_in_explorer(path: &Path) {
    let dir = if path.is_file() {
        path.parent().unwrap_or(path)
    } else {
        path
    };

    #[cfg(target_os = "windows")]
    {
        let _ = Command::new("explorer").arg(dir).spawn();
    }

    #[cfg(target_os = "macos")]
    {
        let _ = Command::new("open").arg(dir).spawn();
    }

    #[cfg(target_os = "linux")]
    {
        let _ = Command::new("xdg-open").arg(dir).spawn();
    }
}

/// Reveal file in system file explorer (select it).
#[allow(dead_code)]
pub fn reveal_in_explorer(path: &Path) {
    #[cfg(target_os = "windows")]
    {
        let _ = Command::new("explorer")
            .args(["/select,", &path.to_string_lossy()])
            .spawn();
    }

    #[cfg(target_os = "macos")]
    {
        let _ = Command::new("open").args(["-R", &path.to_string_lossy()]).spawn();
    }

    #[cfg(target_os = "linux")]
    {
        // Linux doesn't have a standard way to select a file, just open the folder
        if let Some(parent) = path.parent() {
            let _ = Command::new("xdg-open").arg(parent).spawn();
        }
    }
}
