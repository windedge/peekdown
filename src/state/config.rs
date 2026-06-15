use gpui::{App, Window, px};
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

/// Explorer root mode for file explorer
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "snake_case")]
pub enum ExplorerRootMode {
    #[default]
    CurrentDir,
    ProjectRoot,
}

/// File sorting mode for explorer
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "snake_case")]
pub enum ExplorerSortMode {
    #[default]
    /// Name ascending (A-Z)
    NameAsc,
    /// Name descending (Z-A)
    NameDesc,
    /// Modified time descending (newest first)
    TimeDesc,
    /// Modified time ascending (oldest first)
    TimeAsc,
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
    /// Whether the window was maximized when last closed
    #[serde(default)]
    pub is_maximized: bool,
    /// Whether the outline sidebar is visible (deprecated, use sidebar_visible)
    #[serde(default)]
    pub outline_visible: bool,
    /// Width of the outline sidebar in pixels
    #[serde(default = "default_outline_width")]
    pub outline_width: f32,
    /// Font family for content text (empty string means system default)
    #[serde(default = "default_font_family")]
    pub font_family: String,
    /// Font size for content text in pixels
    #[serde(default = "default_font_size")]
    pub font_size: f32,
    /// Monospace font family for code blocks (empty string means platform default)
    #[serde(default = "default_mono_font_family")]
    pub mono_font_family: String,
    /// Monospace font size for code blocks in pixels
    #[serde(default = "default_mono_font_size")]
    pub mono_font_size: f32,
    /// Whether to show FPS counter in the status bar
    #[serde(default)]
    pub show_fps: bool,
    /// Whether to enable inertia (smooth) scrolling
    #[serde(default = "default_inertia_scroll")]
    pub inertia_scroll: bool,
    /// Whether the explorer sidebar is visible (deprecated, use sidebar_visible)
    #[serde(default)]
    pub explorer_visible: bool,
    /// Width of the explorer sidebar in pixels
    #[serde(default = "default_explorer_width")]
    pub explorer_width: f32,
    /// Explorer root mode (current dir or project root)
    #[serde(default)]
    pub explorer_root_mode: ExplorerRootMode,
    /// File sorting mode in explorer
    #[serde(default)]
    pub explorer_sort_mode: ExplorerSortMode,
    /// Project root markers used when explorer_root_mode is project_root
    #[serde(default = "default_project_root_markers")]
    pub project_root_markers: Vec<String>,
    /// List of expanded directory paths
    #[serde(default)]
    pub expanded_dirs: Vec<String>,
    /// Whether to automatically refresh documents when files change
    #[serde(default = "default_auto_refresh")]
    pub auto_refresh: bool,
    /// Width of the unified sidebar in pixels
    #[serde(default = "default_sidebar_width")]
    pub sidebar_width: f32,
    /// Last active sidebar tab ("explorer" or "outline")
    #[serde(default = "default_sidebar_tab")]
    pub sidebar_tab: String,
    /// Whether the unified sidebar is visible
    #[serde(default = "default_sidebar_visible")]
    pub sidebar_visible: bool,
}

fn default_sidebar_width() -> f32 {
    200.0
}

fn default_sidebar_tab() -> String {
    "explorer".to_string()
}

fn default_sidebar_visible() -> bool {
    false
}

fn default_auto_refresh() -> bool {
    true
}

fn default_inertia_scroll() -> bool {
    true
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

fn default_outline_width() -> f32 {
    200.0
}

fn default_font_family() -> String {
    String::new()
}

fn default_font_size() -> f32 {
    16.0
}

fn default_mono_font_family() -> String {
    String::new()
}

fn default_mono_font_size() -> f32 {
    13.0
}

fn default_explorer_width() -> f32 {
    200.0
}

fn default_project_root_markers() -> Vec<String> {
    // Top-down build/project markers (find outermost)
    // VCS markers are hardcoded in find_project_root()
    vec![
        "Cargo.toml".to_string(),
        "package.json".to_string(),
        "pyproject.toml".to_string(),
        "go.mod".to_string(),
        "pom.xml".to_string(),
        "build.gradle".to_string(),
        "build.gradle.kts".to_string(),
        "CMakeLists.txt".to_string(),
    ]
}

impl Default for AppearanceConfig {
    fn default() -> Self {
        Self {
            theme: AppThemeMode::default(),
            layout: LayoutMode::default(),
            scroll_speed: default_scroll_speed(),
            window_width: default_window_width(),
            window_height: default_window_height(),
            is_maximized: false,
            outline_visible: false,
            outline_width: default_outline_width(),
            font_family: default_font_family(),
            font_size: default_font_size(),
            mono_font_family: default_mono_font_family(),
            mono_font_size: default_mono_font_size(),
            show_fps: false,
            inertia_scroll: default_inertia_scroll(),
            explorer_visible: false,
            sidebar_visible: default_sidebar_visible(),
            explorer_width: default_explorer_width(),
            explorer_root_mode: ExplorerRootMode::default(),
            explorer_sort_mode: ExplorerSortMode::default(),
            project_root_markers: default_project_root_markers(),
            expanded_dirs: Vec::new(),
            auto_refresh: default_auto_refresh(),
            sidebar_width: default_sidebar_width(),
            sidebar_tab: default_sidebar_tab(),
        }
    }
}

impl AppearanceConfig {
    /// Zoom font sizes by delta (positive for larger, negative for smaller).
    /// Clamped to [8.0, 48.0] range.
    pub fn zoom_font_size(&mut self, delta: f32) {
        self.font_size = (self.font_size + delta).clamp(8.0, 48.0);
        self.mono_font_size = (self.mono_font_size + delta).clamp(8.0, 48.0);
    }

    /// Reset font sizes to their default values.
    pub fn reset_font_size(&mut self) {
        self.font_size = default_font_size();
        self.mono_font_size = default_mono_font_size();
    }

    /// Apply font settings to the global theme
    pub fn apply_font_settings(&self, cx: &mut App) {
        let theme = Theme::global_mut(cx);

        if !self.font_family.is_empty() {
            theme.font_family = self.font_family.clone().into();
        }
        theme.font_size = px(self.font_size);

        if !self.mono_font_family.is_empty() {
            theme.mono_font_family = self.mono_font_family.clone().into();
        }
        theme.mono_font_size = px(self.mono_font_size);
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct AppConfig {
    #[serde(default)]
    pub appearance: AppearanceConfig,
    #[serde(default)]
    recent_files: Vec<PathBuf>,
}

impl AppConfig {
    /// Add a path to the recent files list.
    /// Inserts at the top, deduplicates, and limits to 15 entries.
    pub fn add_recent_file(&mut self, path: PathBuf) {
        self.recent_files.retain(|p| p != &path);
        self.recent_files.insert(0, path);
        self.recent_files.truncate(15);
    }

    /// Get the list of recent files.
    pub fn recent_files(&self) -> &[PathBuf] {
        &self.recent_files
    }

    /// Clear all recent files.
    pub fn clear_recent_files(&mut self) {
        self.recent_files.clear();
    }

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
            if let Some(parent) = path.parent()
                && let Err(e) = fs::create_dir_all(parent) {
                    eprintln!("Failed to create config dir: {}", e);
                    return;
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
