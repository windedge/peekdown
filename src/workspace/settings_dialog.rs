//! Settings dialog for configuring application preferences.

use gpui::*;
use gpui_component::{
    StyledExt,
    h_flex, v_flex,
    checkbox::Checkbox,
    radio::Radio,
    slider::{Slider, SliderState},
    select::{Select, SelectState, SearchableVec},
    WindowExt,
};
use crate::state::config::{AppConfig, AppThemeMode, LayoutMode};

/// Pinned common content fonts (displayed at top of list)
const PINNED_CONTENT_FONTS: &[&str] = &[
    "Microsoft YaHei",
    "Source Han Sans SC",
    "PingFang SC",
    "Noto Sans SC",
    "SimSun",
    "SimHei",
];

/// Pinned common monospace fonts (displayed at top of list)
const PINNED_MONO_FONTS: &[&str] = &[
    "Consolas",
    "Cascadia Code",
    "JetBrains Mono",
    "Fira Code",
    "Source Code Pro",
    "Menlo",
];

/// Build font list with pinned fonts at top
fn build_font_list(all_fonts: &[String], pinned: &[&str]) -> Vec<String> {
    let mut result = vec!["".to_string()]; // System Default

    // Add pinned fonts (only those that exist in system)
    for font in pinned {
        if all_fonts.iter().any(|f| f == *font) {
            result.push(font.to_string());
        }
    }

    // Add remaining fonts (excluding already pinned ones)
    for font in all_fonts {
        if !pinned.contains(&font.as_str()) && font != ".SystemUIFont" {
            result.push(font.clone());
        }
    }

    result
}

/// Custom font item that displays "System Default" for empty string
#[derive(Clone, Debug)]
struct FontItem {
    value: String,
}

impl FontItem {
    fn new(value: String) -> Self {
        Self { value }
    }
}

impl gpui_component::select::SelectItem for FontItem {
    type Value = String;

    fn title(&self) -> SharedString {
        if self.value.is_empty() {
            "System Default".into()
        } else {
            self.value.clone().into()
        }
    }

    fn value(&self) -> &Self::Value {
        &self.value
    }

    fn matches(&self, query: &str) -> bool {
        if self.value.is_empty() {
            "system default".contains(&query.to_lowercase())
        } else {
            self.value.to_lowercase().contains(&query.to_lowercase())
        }
    }
}

/// Settings dialog content as a View for reactive updates
struct SettingsContent {
    selected_theme: Entity<AppThemeMode>,
    selected_layout: Entity<LayoutMode>,
    slider_state: Entity<SliderState>,
    inertia_scroll: Entity<bool>,
    // Font settings
    font_family_state: Entity<SelectState<SearchableVec<FontItem>>>,
    font_size_slider: Entity<SliderState>,
    mono_font_family_state: Entity<SelectState<SearchableVec<FontItem>>>,
    mono_font_size_slider: Entity<SliderState>,
}

impl SettingsContent {
    fn new(
        selected_theme: Entity<AppThemeMode>,
        selected_layout: Entity<LayoutMode>,
        slider_state: Entity<SliderState>,
        inertia_scroll: Entity<bool>,
        font_family_state: Entity<SelectState<SearchableVec<FontItem>>>,
        font_size_slider: Entity<SliderState>,
        mono_font_family_state: Entity<SelectState<SearchableVec<FontItem>>>,
        mono_font_size_slider: Entity<SliderState>,
        cx: &mut Context<Self>,
    ) -> Self {
        // Observe changes to re-render when values change
        cx.observe(&selected_theme, |_, _, cx| cx.notify()).detach();
        cx.observe(&selected_layout, |_, _, cx| cx.notify()).detach();
        cx.observe(&inertia_scroll, |_, _, cx| cx.notify()).detach();

        Self {
            selected_theme,
            selected_layout,
            slider_state,
            inertia_scroll,
            font_family_state,
            font_size_slider,
            mono_font_family_state,
            mono_font_size_slider,
        }
    }
}

impl Render for SettingsContent {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        // Read current values dynamically
        let current_theme = *self.selected_theme.read(cx);
        let current_layout = *self.selected_layout.read(cx);
        let current_inertia = *self.inertia_scroll.read(cx);

        v_flex()
            .gap_4()
            // Theme section
            .child(settings_section("Theme", theme_options(self.selected_theme.clone(), current_theme)))
            // Layout section
            .child(settings_section("Layout", layout_options(self.selected_layout.clone(), current_layout)))
            // Scroll speed section
            .child(settings_section(
                "Scroll Speed",
                scroll_speed_control(self.slider_state.clone(), self.inertia_scroll.clone(), current_inertia),
            ))
            // Content font section
            .child(settings_section(
                "Content Font",
                font_control(self.font_family_state.clone(), self.font_size_slider.clone(), 12.0, 24.0),
            ))
            // Code font section
            .child(settings_section(
                "Code Font",
                font_control(self.mono_font_family_state.clone(), self.mono_font_size_slider.clone(), 10.0, 18.0),
            ))
    }
}

/// Opens the settings dialog.
pub fn open_settings_dialog(
    config: Entity<AppConfig>,
    window: &mut Window,
    cx: &mut App,
) {
    let current_theme = config.read(cx).appearance.theme;
    let current_layout = config.read(cx).appearance.layout;
    let current_scroll_speed = config.read(cx).appearance.scroll_speed;
    let current_inertia_scroll = config.read(cx).appearance.inertia_scroll;
    let current_font_family = config.read(cx).appearance.font_family.clone();
    let current_font_size = config.read(cx).appearance.font_size;
    let current_mono_font_family = config.read(cx).appearance.mono_font_family.clone();
    let current_mono_font_size = config.read(cx).appearance.mono_font_size;

    // Get system font list
    let all_fonts = cx.text_system().all_font_names();

    // Build font lists with pinned fonts at top
    let content_fonts: Vec<FontItem> = build_font_list(&all_fonts, PINNED_CONTENT_FONTS)
        .into_iter()
        .map(FontItem::new)
        .collect();
    let mono_fonts: Vec<FontItem> = build_font_list(&all_fonts, PINNED_MONO_FONTS)
        .into_iter()
        .map(FontItem::new)
        .collect();

    // Create slider state for scroll speed
    let slider_state = cx.new(|_| {
        SliderState::new()
            .min(0.5)
            .max(3.0)
            .step(0.1)
            .default_value(current_scroll_speed)
    });

    // Create font family select states
    let font_family_state = cx.new(|cx| {
        let items = SearchableVec::new(content_fonts);
        let mut state = SelectState::new(items, None, window, cx).searchable(true);
        if !current_font_family.is_empty() {
            state.set_selected_value(&current_font_family, window, cx);
        } else {
            // Select "System Default" (empty string at index 0)
            state.set_selected_value(&String::new(), window, cx);
        }
        state
    });

    let mono_font_family_state = cx.new(|cx| {
        let items = SearchableVec::new(mono_fonts);
        let mut state = SelectState::new(items, None, window, cx).searchable(true);
        if !current_mono_font_family.is_empty() {
            state.set_selected_value(&current_mono_font_family, window, cx);
        } else {
            state.set_selected_value(&String::new(), window, cx);
        }
        state
    });

    // Create font size sliders
    let font_size_slider = cx.new(|_| {
        SliderState::new()
            .min(12.0)
            .max(24.0)
            .step(1.0)
            .default_value(current_font_size)
    });

    let mono_font_size_slider = cx.new(|_| {
        SliderState::new()
            .min(10.0)
            .max(18.0)
            .step(1.0)
            .default_value(current_mono_font_size)
    });

    // Track selected values
    let selected_theme = cx.new(|_| current_theme);
    let selected_layout = cx.new(|_| current_layout);
    let inertia_scroll = cx.new(|_| current_inertia_scroll);

    // Create content view for reactive updates
    let content = cx.new(|cx| SettingsContent::new(
        selected_theme.clone(),
        selected_layout.clone(),
        slider_state.clone(),
        inertia_scroll.clone(),
        font_family_state.clone(),
        font_size_slider.clone(),
        mono_font_family_state.clone(),
        mono_font_size_slider.clone(),
        cx,
    ));

    window.open_dialog(cx, move |dialog, _window, _cx| {
        let config_for_save = config.clone();
        let slider_for_save = slider_state.clone();
        let theme_for_save = selected_theme.clone();
        let layout_for_save = selected_layout.clone();
        let inertia_for_save = inertia_scroll.clone();
        let font_family_for_save = font_family_state.clone();
        let font_size_for_save = font_size_slider.clone();
        let mono_font_family_for_save = mono_font_family_state.clone();
        let mono_font_size_for_save = mono_font_size_slider.clone();

        dialog
            .title("Settings")
            .w(px(480.))
            .overlay_closable(false)
            .confirm()
            .on_ok(move |_, window, cx| {
                // Save all settings when OK is clicked
                let new_speed = slider_for_save.read(cx).value().start();
                let new_theme = *theme_for_save.read(cx);
                let new_layout = *layout_for_save.read(cx);
                let new_inertia = *inertia_for_save.read(cx);

                // Get font settings
                let new_font_family = font_family_for_save.read(cx)
                    .selected_value()
                    .cloned()
                    .unwrap_or_default();
                let new_font_size = font_size_for_save.read(cx).value().start();
                let new_mono_font_family = mono_font_family_for_save.read(cx)
                    .selected_value()
                    .cloned()
                    .unwrap_or_default();
                let new_mono_font_size = mono_font_size_for_save.read(cx).value().start();

                config_for_save.update(cx, |config, cx| {
                    config.appearance.scroll_speed = new_speed;
                    config.appearance.theme = new_theme;
                    config.appearance.layout = new_layout;
                    config.appearance.inertia_scroll = new_inertia;
                    config.appearance.font_family = new_font_family;
                    config.appearance.font_size = new_font_size;
                    config.appearance.mono_font_family = new_mono_font_family;
                    config.appearance.mono_font_size = new_mono_font_size;

                    // Apply font settings immediately
                    config.appearance.apply_font_settings(cx);

                    // Notify observers of config changes
                    cx.notify();

                    // Refresh window to apply changes
                    window.refresh();

                    config.save();
                });
                true // Close dialog
            })
            .child(content.clone())
    });
}

fn settings_section(title: &str, content: impl IntoElement) -> impl IntoElement {
    v_flex()
        .gap_2()
        .child(
            div()
                .text_sm()
                .font_semibold()
                .child(title.to_string())
        )
        .child(content)
}

fn theme_options(
    selected: Entity<AppThemeMode>,
    current: AppThemeMode,
) -> impl IntoElement {
    let selected1 = selected.clone();
    let selected2 = selected.clone();
    let selected3 = selected.clone();

    h_flex()
        .gap_4()
        .child(
            Radio::new("theme-light")
                .label("Light")
                .checked(current == AppThemeMode::Light)
                .on_click(move |_, window, cx| {
                    selected1.update(cx, |s, _| *s = AppThemeMode::Light);
                    // Apply theme immediately for preview
                    AppThemeMode::Light.apply(Some(window), cx);
                })
        )
        .child(
            Radio::new("theme-dark")
                .label("Dark")
                .checked(current == AppThemeMode::Dark)
                .on_click(move |_, window, cx| {
                    selected2.update(cx, |s, _| *s = AppThemeMode::Dark);
                    AppThemeMode::Dark.apply(Some(window), cx);
                })
        )
        .child(
            Radio::new("theme-auto")
                .label("Auto")
                .checked(current == AppThemeMode::Auto)
                .on_click(move |_, window, cx| {
                    selected3.update(cx, |s, _| *s = AppThemeMode::Auto);
                    AppThemeMode::Auto.apply(Some(window), cx);
                })
        )
}

fn layout_options(
    selected: Entity<LayoutMode>,
    current: LayoutMode,
) -> impl IntoElement {
    let selected1 = selected.clone();
    let selected2 = selected.clone();

    h_flex()
        .gap_4()
        .child(
            Radio::new("layout-centered")
                .label("Centered")
                .checked(current == LayoutMode::Centered)
                .on_click(move |_, _window, cx| {
                    selected1.update(cx, |s, _| *s = LayoutMode::Centered);
                })
        )
        .child(
            Radio::new("layout-fullwidth")
                .label("Full Width")
                .checked(current == LayoutMode::FullWidth)
                .on_click(move |_, _window, cx| {
                    selected2.update(cx, |s, _| *s = LayoutMode::FullWidth);
                })
        )
}

fn scroll_speed_control(
    slider_state: Entity<SliderState>,
    inertia_scroll: Entity<bool>,
    current_inertia: bool,
) -> impl IntoElement {
    v_flex()
        .gap_2()
        .child(
            h_flex()
                .gap_3()
                .items_center()
                .child(
                    div()
                        .text_sm()
                        .child("0.5x")
                )
                .child(
                    div()
                        .flex_1()
                        .child(Slider::new(&slider_state))
                )
                .child(
                    div()
                        .text_sm()
                        .child("3.0x")
                )
        )
        .child(
            Checkbox::new("inertia-scroll")
                .label("Smooth Scrolling")
                .checked(current_inertia)
                .on_click(move |checked, _window, cx| {
                    inertia_scroll.update(cx, |s, _| *s = *checked);
                })
        )
}

fn font_control(
    select_state: Entity<SelectState<SearchableVec<FontItem>>>,
    size_slider: Entity<SliderState>,
    min_size: f32,
    max_size: f32,
) -> impl IntoElement {
    v_flex()
        .gap_2()
        .child(
            div()
                .h(px(32.))
                .child(Select::new(&select_state))
        )
        .child(
            h_flex()
                .gap_3()
                .items_center()
                .child(
                    div()
                        .text_sm()
                        .w(px(30.))
                        .child(format!("{}px", min_size as i32))
                )
                .child(
                    div()
                        .flex_1()
                        .child(Slider::new(&size_slider))
                )
                .child(
                    div()
                        .text_sm()
                        .w(px(30.))
                        .child(format!("{}px", max_size as i32))
                )
        )
}
