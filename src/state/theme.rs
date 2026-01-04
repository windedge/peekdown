use gpui::*;

pub struct Theme {
    pub bg_base: Rgba,
    pub bg_header: Rgba,
    pub bg_footer: Rgba,
    pub border: Rgba,
    pub text_primary: Rgba,
    pub text_secondary: Rgba,
}

impl Theme {
    pub fn dark() -> Self {
        Self {
            bg_base: rgb(0x1e1e1e),
            bg_header: rgb(0x2d2d2d),
            bg_footer: rgb(0x252526),
            border: rgb(0x3d3d3d),
            text_primary: rgb(0xffffff),
            text_secondary: rgb(0xcccccc),
        }
    }
}
