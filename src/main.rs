//! 《九州问道》终端客户端入口。
//!
//! 参数：`--server ws://host:port`、`--login 道号:密码`、`--create 道号`、
//! `--page map`、`--move 0`、`--snapshot out.ansi`、`--log-level info`。

mod app;
mod config;
mod net;
mod protocol;
mod store;
mod tui;
mod util;
mod view;

use std::path::PathBuf;

use app::App;

struct Args {
    server: Option<String>,
    login: Option<(String, String)>,
    create: Option<String>,
    page: Option<String>,
    move_index: Option<usize>,
    snapshot: Option<PathBuf>,
    log_level: String,
}

fn parse_args() -> Args {
    let mut a = Args {
        server: None,
        login: None,
        create: None,
        page: None,
        move_index: None,
        snapshot: None,
        log_level: "warn".into(),
    };
    let mut it = std::env::args().skip(1);
    while let Some(arg) = it.next() {
        match arg.as_str() {
            "--server" => a.server = it.next(),
            "--login" => {
                if let Some(v) = it.next()
                    && let Some((u, p)) = v.split_once(':')
                {
                    a.login = Some((u.to_string(), p.to_string()));
                }
            }
            "--create" => a.create = it.next(),
            "--page" => a.page = it.next(),
            "--move" => a.move_index = it.next().and_then(|v| v.parse().ok()),
            "--snapshot" => a.snapshot = it.next().map(PathBuf::from),
            "--log-level" => {
                if let Some(v) = it.next() {
                    a.log_level = v;
                }
            }
            "--help" | "-h" => {
                println!(
                    "xiuxian-client（终端版）
  --server ws://127.0.0.1:8080
  --login 道号:密码        跳过登录页
  --create 道号            登录后自动建号
  --page game|character|bag|skills|social|market|sect|map|settings
  --move 0                 进主界面后自动走第 n 个出口
  --snapshot out.ansi      离屏跑一屏并导出（排查/截图用）
  --log-level info|debug|warn"
                );
                std::process::exit(0);
            }
            other => eprintln!("忽略未知参数：{other}"),
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

fn main() -> anyhow::Result<()> {
    let args = parse_args();
    init_tracing(&args.log_level);

    let mut cfg = config::Config::load();
    if let Some(s) = &args.server {
        cfg.server = s.clone();
    }

    let mut app = App::new(cfg);
    if let Some((name, pass)) = args.login.clone() {
        app.auto_login(name, pass);
    }
    if let Some(name) = args.create.clone() {
        app.auto_create(name);
    }
    if let Some(p) = args.page.clone() {
        app.start_page(&p);
    }
    if let Some(i) = args.move_index {
        app.auto_move(i);
    }

    match args.snapshot {
        Some(path) => tui::snapshot(&mut app, &path),
        None => tui::run(app),
    }
}
