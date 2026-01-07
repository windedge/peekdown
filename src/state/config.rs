use gpui::{App, Window};
use gpui_component::theme::{Theme, ThemeMode};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;
use std::fs;

#[derive(Debug, Clone, Copy, PartialEq, Serialize, Deserialize, Default)]
#[serde(rename_all = "lowercase")]
pub enum AppThemeMode {
    Light,
    Dark,
    #[default]
    Auto,
}

/// Layout mode for markdown content display
#[derive(Debug, Clone, Copy, PartialEq, Serialize, Deserialize, Default)]
#[serde(rename_all = "lowercase")]
pub enum LayoutMode {
    /// Centered content with max-width (default)
    #[default]
    Centered,
    /// Full width content
    FullWidth,
}

impl AppThemeMode {
    pub fn apply(&self, window: Option<&mut Window>, cx: &mut App) {
        match self {
            AppThemeMode::Light => Theme::change(ThemeMode::Light, window, cx),
            AppThemeMode::Dark => Theme::change(ThemeMode::Dark, window, cx),
            AppThemeMode::Auto => Theme::sync_system_appearance(window, cx),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AppearanceConfig {
    #[serde(default)]
    pub theme: AppThemeMode,
    #[serde(default)]
    pub layout: LayoutMode,
    /// Scroll speed multiplier (1.0 = normal, 2.0 = double speed, etc.)
    #[serde(default = "default_scroll_speed")]
    pub scroll_speed: f32,
    /// Window width in pixels
    #[serde(default = "default_window_width")]
    pub window_width: f32,
    /// Window height in pixels
    #[serde(default = "default_window_height")]
    pub window_height: f32,
}

fn default_scroll_speed() -> f32 {
    1.0
}

fn default_window_width() -> f32 {
    1024.0
}

fn default_window_height() -> f32 {
    768.0
}

impl Default for AppearanceConfig {
    fn default() -> Self {
        Self {
            theme: AppThemeMode::default(),
            layout: LayoutMode::default(),
            scroll_speed: default_scroll_speed(),
            window_width: default_window_width(),
            window_height: default_window_height(),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct AppConfig {
    #[serde(default)]
    pub appearance: AppearanceConfig,
}

impl AppConfig {
    pub fn config_path() -> PathBuf {
        let mut path = dirs::config_dir().unwrap_or_else(|| PathBuf::from("."));
        path.push("peekdown");
        path.push("config.toml");
        path
    }

    pub fn load() -> Self {
        let path = Self::config_path();
        if path.exists() {
            match fs::read_to_string(&path) {
                Ok(content) => match toml::from_str(&content) {
                    Ok(config) => return config,
                    Err(e) => eprintln!("Failed to parse config: {}", e),
                },
                Err(e) => eprintln!("Failed to read config: {}", e),
            }
        }
        Self::default()
    }

    pub fn save(&self) {
        let path = Self::config_path();
        let config = self.clone();
        std::thread::spawn(move || {
            if let Some(parent) = path.parent() {
                if let Err(e) = fs::create_dir_all(parent) {
                    eprintln!("Failed to create config dir: {}", e);
                    return;
                }
            }
            match toml::to_string_pretty(&config) {
                Ok(content) => {
                    if let Err(e) = fs::write(path, content) {
                        eprintln!("Failed to write config: {}", e);
                    }
                }
                Err(e) => eprintln!("Failed to serialize config: {}", e),
            }
        });
    }
}
