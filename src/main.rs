//! 《九州问道》桌面客户端入口。
//!
//! 参数：`--server <ws://host:port>`、`--renderer <wgpu|glow>`、`--log-level <lvl>`。

mod app;
mod config;
mod net;
mod protocol;
mod store;
mod ui;
mod util;

use app::ClientApp;

struct Args {
    server: Option<String>,
    renderer: Option<String>,
    log_level: String,
    /// 开发/联调用：`--login 道号:密码`，跳过手动输入。
    login: Option<(String, String)>,
    /// 开发/联调用：`--create 道号`，登录后自动建号。
    create: Option<String>,
    /// 开发/联调用：`--snapshot out.png`，主界面就绪后抓一帧并退出。
    snapshot: Option<String>,
    /// 开发/联调用：`--page map`，启动后直接打开某一页（截图用）。
    page: Option<String>,
    /// 开发/联调用：`--move 0`，进主界面后自动走第 n 个出口。
    move_index: Option<usize>,
    /// 皮肤：`ink`（水墨，默认）或 `tui`（等宽字符）。
    skin: Option<String>,
}

fn parse_args() -> Args {
    let mut a = Args {
        server: None,
        renderer: None,
        log_level: "info".into(),
        login: None,
        create: None,
        snapshot: None,
        page: None,
        move_index: None,
        skin: None,
    };
    let mut it = std::env::args().skip(1);
    while let Some(arg) = it.next() {
        match arg.as_str() {
            "--server" => a.server = it.next(),
            "--renderer" => a.renderer = it.next(),
            "--login" => {
                if let Some(v) = it.next()
                    && let Some((u, p)) = v.split_once(':')
                {
                    a.login = Some((u.to_string(), p.to_string()));
                }
            }
            "--create" => a.create = it.next(),
            "--snapshot" => a.snapshot = it.next(),
            "--page" => a.page = it.next(),
            "--move" => a.move_index = it.next().and_then(|v| v.parse().ok()),
            "--skin" => a.skin = it.next(),
            "--log-level" => {
                if let Some(v) = it.next() {
                    a.log_level = v;
                }
            }
            "--help" | "-h" => {
                println!(
                    "xiuxian-client\n  --server ws://127.0.0.1:8080\n  --renderer wgpu|glow\n  --log-level info|debug|warn"
                );
                std::process::exit(0);
            }
            other => {
                eprintln!("忽略未知参数：{other}");
            }
        }
    }
    a
}

fn init_tracing(level: &str) {
    use tracing_subscriber::EnvFilter;
    let filter = EnvFilter::try_new(format!("xiuxian_client={level},info"))
        .or_else(|_| EnvFilter::try_new("info"))
        .unwrap_or_default();
    let _ = tracing_subscriber::fmt()
        .with_env_filter(filter)
        .with_target(false)
        .try_init();
}

/// Windows 上必须先声明 DPI 感知：否则系统按 150% 缩放把窗口物理尺寸压小，
/// 而 winit 仍报虚拟尺寸，egui 按虚拟尺寸排版，右侧内容会被裁到屏幕外。
#[cfg(windows)]
fn enable_dpi_awareness() {
    use windows_sys::Win32::UI::HiDpi::{
        DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2, SetProcessDpiAwarenessContext,
    };
    // SAFETY: 进程启动早期调用一次，参数是微软文档里的固定常量。
    unsafe {
        SetProcessDpiAwarenessContext(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2);
    }
}

#[cfg(not(windows))]
fn enable_dpi_awareness() {}

fn main() -> eframe::Result {
    enable_dpi_awareness();
    let args = parse_args();
    init_tracing(&args.log_level);

    // 崩溃也要留线索：窗口一闪而过时用户看不到 stderr。
    let default_hook = std::panic::take_hook();
    std::panic::set_hook(Box::new(move |info| {
        let dir = config::log_dir();
        let _ = std::fs::create_dir_all(&dir);
        let path = dir.join("panic.log");
        let line = format!("[{}] {info}\n", util::time::fmt_hms(util::time::now_ms()));
        let _ = std::fs::OpenOptions::new()
            .create(true)
            .append(true)
            .open(path)
            .and_then(|mut f| {
                use std::io::Write;
                f.write_all(line.as_bytes())
            });
        default_hook(info);
    }));

    let cfg = config::Config::load();
    let server = args.server.unwrap_or(cfg.server.clone());
    // 0.32 线默认编入 glow 后端（OpenGL 3.3），三平台兼容性最好；
    // 需要 wgpu 时在 Cargo.toml 打开 eframe 的 `wgpu` feature 并在这里加分支。
    if args.renderer.as_deref() == Some("wgpu") {
        tracing::warn!("本构建未编入 wgpu 后端，改用 glow");
    }
    let renderer = eframe::Renderer::Glow;

    let options = eframe::NativeOptions {
        viewport: eframe::egui::ViewportBuilder::default()
            .with_inner_size([1200.0, 760.0])
            .with_min_inner_size([1000.0, 640.0])
            .with_title("九州问道"),
        renderer,
        ..Default::default()
    };

    let login = args.login;
    let create = args.create;
    let snapshot = args.snapshot;
    let page = args.page;
    let move_index = args.move_index;
    if args.skin.as_deref() == Some("tui") {
        ui::theme::set_terminal(true);
    }

    eframe::run_native(
        "九州问道",
        options,
        Box::new(move |cc| {
            let mut app = ClientApp::new(cc);
            app.apply_server_override(&server);
            if let Some((name, pass)) = login.clone() {
                app.auto_login(name, pass);
            }
            if let Some(name) = create.clone() {
                app.auto_create(name);
            }
            if let Some(path) = snapshot.clone() {
                app.snapshot_to(path.into());
            }
            if let Some(p) = page.clone() {
                app.start_page(&p);
            }
            if let Some(i) = move_index {
                app.auto_move(i);
            }
            Ok(Box::new(app))
        }),
    )
}
