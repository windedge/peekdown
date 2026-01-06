use anyhow::{Context, Result};
use std::env;
use winreg::enums::*;
use winreg::RegKey;

pub fn register_file_association() -> Result<()> {
    let exe_path = env::current_exe()?;
    let exe_path_str = exe_path.to_str().context("Failed to convert path to string")?;
    
    let hkcu = RegKey::predef(HKEY_CURRENT_USER);
    let classes = hkcu.open_subkey(r"Software\Classes")?;

    // 1. Register .md extension
    let md_key = classes.create_subkey(".md")?.0;
    md_key.set_value("", &"Peekdown.Markdown")?;

    // 2. Register ProgID
    let prog_id_key = classes.create_subkey("Peekdown.Markdown")?.0;
    prog_id_key.set_value("", &"Markdown Document")?;
    prog_id_key.set_value("FriendlyAppName", &"Peekdown")?;
    
    let icon_key = prog_id_key.create_subkey("DefaultIcon")?.0;
    icon_key.set_value("", &format!("{},0", exe_path_str))?;

    let shell_key = prog_id_key.create_subkey("shell")?.0;
    let open_key = shell_key.create_subkey("open")?.0;
    open_key.set_value("FriendlyAppName", &"Peekdown")?; // Also here for context menus sometimes
    let command_key = open_key.create_subkey("command")?.0;
    command_key.set_value("", &format!("\"{}\" \"%1\"", exe_path_str))?;

    // 3. Register Applications key for Open With list
    if let Some(exe_name) = exe_path.file_name().and_then(|s| s.to_str()) {
        let app_key_path = format!(r"Applications\{}", exe_name);
        let (app_key, _) = classes.create_subkey(&app_key_path)?;
        app_key.set_value("FriendlyAppName", &"Peekdown")?;
        
        let shell = app_key.create_subkey("shell")?.0;
        let open = shell.create_subkey("open")?.0;
        let command = open.create_subkey("command")?.0;
        command.set_value("", &format!("\"{}\" \"%1\"", exe_path_str))?;
    }

    println!("Successfully registered file association for .md files.");
    Ok(())
}
