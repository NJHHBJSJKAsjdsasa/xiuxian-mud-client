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
}

fn parse_args() -> Args {
    let mut a = Args {
        server: None,
        renderer: None,
        log_level: "info".into(),
        login: None,
        create: None,
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

fn main() -> eframe::Result {
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
            .with_inner_size([1360.0, 840.0])
            .with_min_inner_size([1100.0, 680.0])
            .with_title("九州问道"),
        renderer,
        ..Default::default()
    };

    let login = args.login;
    let create = args.create;

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
            Ok(Box::new(app))
        }),
    )
}
