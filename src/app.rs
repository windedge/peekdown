use eframe::egui;

#[derive(Default)]
pub struct App {
    // TODO: Add application state
}

impl App {
    pub fn new(_cc: &eframe::CreationContext<'_>) -> Self {
        Self::default()
    }
}

impl eframe::App for App {
    fn update(&mut self, ctx: &egui::Context, _frame: &mut eframe::Frame) {
        egui::CentralPanel::default().show(ctx, |ui| {
            ui.heading("Peekdown");
        });
    }
}

pub fn run() -> eframe::Result {
    let options = eframe::NativeOptions::default();
    eframe::run_native(
        "Peekdown",
        options,
        Box::new(|cc| Ok(Box::new(App::new(cc)))),
    )
}
