//! 三个 REST 端点：register / login / logout。

use anyhow::{Result, bail};
use serde_json::json;

#[derive(Debug, Clone)]
pub struct Auth {
    pub token: String,
    #[allow(dead_code)] // 登录页目前不用它决定跳转，留给"记住我"完善时用。
    pub has_char: bool,
}

/// `ws://host:port` → `http://host:port`；`wss://` → `https://`。
pub fn http_base(server: &str) -> String {
    let s = server.trim().trim_end_matches('/');
    let s = s
        .strip_prefix("ws://")
        .map(|r| format!("http://{r}"))
        .unwrap_or_else(|| {
            s.strip_prefix("wss://")
                .map(|r| format!("https://{r}"))
                .unwrap_or_else(|| s.to_string())
        });
    if s.starts_with("http://") || s.starts_with("https://") {
        s
    } else {
        format!("http://{s}")
    }
}

pub fn ws_url(server: &str) -> String {
    let s = server.trim().trim_end_matches('/');
    let s = if let Some(r) = s.strip_prefix("http://") {
        format!("ws://{r}")
    } else if let Some(r) = s.strip_prefix("https://") {
        format!("wss://{r}")
    } else {
        s.to_string()
    };
    let s = if s.starts_with("ws://") || s.starts_with("wss://") {
        s
    } else {
        format!("ws://{s}")
    };
    if s.ends_with("/ws") {
        s
    } else {
        format!("{s}/ws")
    }
}

async fn post_json(url: &str, body: serde_json::Value) -> Result<serde_json::Value> {
    let client = reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(8))
        .build()?;
    let resp = client.post(url).json(&body).send().await?;
    let status = resp.status();
    let text = resp.text().await.unwrap_or_default();
    if !status.is_success() {
        // 服务端错误体：{code,msg}
        if let Ok(v) = serde_json::from_str::<serde_json::Value>(&text)
            && let Some(msg) = v.get("msg").and_then(|m| m.as_str())
        {
            bail!("{msg}");
        }
        bail!("服务端返回 {status}");
    }
    Ok(serde_json::from_str(&text)?)
}

pub async fn login(server: &str, name: &str, pass: &str) -> Result<Auth> {
    let v = post_json(
        &format!("{}/api/login", http_base(server)),
        json!({"name": name, "pass": pass}),
    )
    .await?;
    Ok(Auth {
        token: v["token"].as_str().unwrap_or_default().to_string(),
        has_char: v["hasChar"].as_bool().unwrap_or(false),
    })
}

pub async fn register(server: &str, name: &str, pass: &str) -> Result<Auth> {
    let v = post_json(
        &format!("{}/api/register", http_base(server)),
        json!({"name": name, "pass": pass}),
    )
    .await?;
    Ok(Auth {
        token: v["token"].as_str().unwrap_or_default().to_string(),
        has_char: v["hasChar"].as_bool().unwrap_or(false),
    })
}

pub async fn logout(server: &str, token: &str) -> Result<()> {
    let client = reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(5))
        .build()?;
    let _ = client
        .post(format!("{}/api/logout", http_base(server)))
        .bearer_auth(token)
        .send()
        .await?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn base_conversion() {
        assert_eq!(http_base("ws://127.0.0.1:8080"), "http://127.0.0.1:8080");
        assert_eq!(
            http_base("wss://mud.example.com"),
            "https://mud.example.com"
        );
        assert_eq!(http_base("127.0.0.1:8080/"), "http://127.0.0.1:8080");
        assert_eq!(ws_url("http://127.0.0.1:8080"), "ws://127.0.0.1:8080/ws");
        assert_eq!(ws_url("ws://127.0.0.1:8080/ws"), "ws://127.0.0.1:8080/ws");
    }
}
