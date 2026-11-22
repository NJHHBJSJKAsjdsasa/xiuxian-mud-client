//! 终端层：初始化、事件循环、离屏快照。

pub mod render;
pub mod scene;
pub mod theme;

use std::io::{Write, stdout};
use std::path::Path;
use std::time::Duration;

use anyhow::Result;
use crossterm::event::{self, Event};
use crossterm::execute;
use crossterm::terminal::{
    EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode,
};
use ratatui::Terminal;
use ratatui::backend::{CrosstermBackend, TestBackend};
use ratatui::style::Color;

use crate::app::App;
use crate::view::View;

pub fn run(mut app: App) -> Result<()> {
    enable_raw_mode()?;
    let mut out = stdout();
    execute!(out, EnterAlternateScreen)?;
    let backend = CrosstermBackend::new(out);
    let mut term = Terminal::new(backend)?;

    let res = event_loop(&mut term, &mut app);

    disable_raw_mode()?;
    execute!(term.backend_mut(), LeaveAlternateScreen)?;
    term.show_cursor()?;
    res
}

fn event_loop(term: &mut Terminal<CrosstermBackend<std::io::Stdout>>, app: &mut App) -> Result<()> {
    loop {
        app.drain_net();
        app.tick();
        term.draw(|f| render::draw(f, app))?;

        if event::poll(Duration::from_millis(30))? {
            match event::read()? {
                Event::Key(k) => app.on_key(k),
                Event::Resize(_, _) => {}
                _ => {}
            }
        }
        if app.quit {
            break;
        }
    }
    app.shutdown();
    Ok(())
}

/// 离屏快照：不依赖真实终端，用 120×40 的测试后端跑几秒再导出 ANSI。
pub fn snapshot(app: &mut App, path: &Path) -> Result<()> {
    let mut term = Terminal::new(TestBackend::new(120, 40))?;
    // 约 6 秒：够自动登录、建号、拿地图。每 20ms 一帧。
    for i in 0..300 {
        app.drain_net();
        app.tick();
        term.draw(|f| render::draw(f, app))?;
        std::thread::sleep(Duration::from_millis(20));
        if i == 120 && app.ui.view == View::Main {
            // 主界面已经出来，后半程留给地图与日志
        }
    }
    let buf = term.backend().buffer();
    let mut text = String::new();
    for y in 0..buf.area.height {
        let mut skip_cols = 0usize;
        for x in 0..buf.area.width {
            let cell = &buf[(x, y)];
            if skip_cols > 0 {
                // 宽字符（中文）在缓冲里占两格：第二格是占位，跳过，
                // 否则导出成 ANSI 时每个汉字后面会多一个空格。
                skip_cols -= 1;
                continue;
            }
            if cell.symbol().is_empty() {
                continue;
            }
            skip_cols = crate::util::text::width(cell.symbol()).saturating_sub(1);
            let fg = ansi_color(cell.fg, Color::Rgb(226, 226, 214));
            let bg = ansi_color(cell.bg, Color::Rgb(5, 7, 9));
            text.push_str(&format!(
                "\x1b[38;2;{};{};{}m\x1b[48;2;{};{};{}m{}",
                fg.0,
                fg.1,
                fg.2,
                bg.0,
                bg.1,
                bg.2,
                cell.symbol()
            ));
        }
        text.push_str("\x1b[0m\n");
    }
    let mut f = std::fs::File::create(path)?;
    f.write_all(text.as_bytes())?;
    tracing::info!("快照已保存：{}", path.display());
    app.shutdown();
    Ok(())
}

fn ansi_color(c: Color, fallback: Color) -> (u8, u8, u8) {
    match c {
        Color::Rgb(r, g, b) => (r, g, b),
        Color::Reset => match fallback {
            Color::Rgb(r, g, b) => (r, g, b),
            _ => (0, 0, 0),
        },
        _ => match fallback {
            Color::Rgb(r, g, b) => (r, g, b),
            _ => (0, 0, 0),
        },
    }
}
