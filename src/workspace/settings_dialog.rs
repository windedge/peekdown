//! Settings dialog for configuring application preferences.

use gpui::*;
use gpui_component::{
    StyledExt,
    h_flex, v_flex,
    radio::Radio,
    slider::{Slider, SliderState},
    WindowExt,
};
use crate::state::config::{AppConfig, AppThemeMode, LayoutMode};

/// Settings dialog content as a View for reactive updates
struct SettingsContent {
    selected_theme: Entity<AppThemeMode>,
    selected_layout: Entity<LayoutMode>,
    slider_state: Entity<SliderState>,
}

impl SettingsContent {
    fn new(
        selected_theme: Entity<AppThemeMode>,
        selected_layout: Entity<LayoutMode>,
        slider_state: Entity<SliderState>,
        cx: &mut Context<Self>,
    ) -> Self {
        // Observe changes to re-render when values change
        cx.observe(&selected_theme, |_, _, cx| cx.notify()).detach();
        cx.observe(&selected_layout, |_, _, cx| cx.notify()).detach();

        Self {
            selected_theme,
            selected_layout,
            slider_state,
        }
    }
}

impl Render for SettingsContent {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        // Read current values dynamically
        let current_theme = *self.selected_theme.read(cx);
        let current_layout = *self.selected_layout.read(cx);

        v_flex()
            .gap_4()
            // Theme section
            .child(settings_section("Theme", theme_options(self.selected_theme.clone(), current_theme)))
            // Layout section
            .child(settings_section("Layout", layout_options(self.selected_layout.clone(), current_layout)))
            // Scroll speed section
            .child(settings_section(
                "Scroll Speed",
                scroll_speed_control(self.slider_state.clone()),
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

    // Create slider state outside the dialog closure
    let slider_state = cx.new(|_| {
        SliderState::new()
            .min(0.5)
            .max(3.0)
            .step(0.1)
            .default_value(current_scroll_speed)
    });

    // Track selected values
    let selected_theme = cx.new(|_| current_theme);
    let selected_layout = cx.new(|_| current_layout);

    // Create content view for reactive updates
    let content = cx.new(|cx| SettingsContent::new(
        selected_theme.clone(),
        selected_layout.clone(),
        slider_state.clone(),
        cx,
    ));

    window.open_dialog(cx, move |dialog, _window, _cx| {
        let config_for_save = config.clone();
        let slider_for_save = slider_state.clone();
        let theme_for_save = selected_theme.clone();
        let layout_for_save = selected_layout.clone();

        dialog
            .title("Settings")
            .w(px(420.))
            .overlay_closable(false)
            .confirm()
            .on_ok(move |_, _, cx| {
                // Save all settings when OK is clicked
                let new_speed = slider_for_save.read(cx).value().start();
                let new_theme = *theme_for_save.read(cx);
                let new_layout = *layout_for_save.read(cx);

                config_for_save.update(cx, |config, _| {
                    config.appearance.scroll_speed = new_speed;
                    config.appearance.theme = new_theme;
                    config.appearance.layout = new_layout;
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
) -> impl IntoElement {
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
}
