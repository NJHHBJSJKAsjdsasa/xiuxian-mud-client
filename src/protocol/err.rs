//! 错误码与关闭码。

pub const E_AUTH: &str = "E-AUTH";
pub const E_NOCHAR: &str = "E-NOCHAR";
pub const E_BADREQ: &str = "E-BADREQ";
pub const E_BUSY: &str = "E-BUSY";
pub const E_COOLDOWN: &str = "E-COOLDOWN";
pub const E_RATE: &str = "E-RATE";
pub const E_TERRAIN: &str = "E-TERRAIN";
pub const E_NOSTAMINA: &str = "E-NOSTAMINA";
pub const E_NOREALM: &str = "E-NOREALM";
pub const E_NOMP: &str = "E-NOMP";
pub const E_NOITEM: &str = "E-NOITEM";
pub const E_NOSTONE: &str = "E-NOSTONE";
pub const E_FULL: &str = "E-FULL";
pub const E_INTERNAL: &str = "E-INTERNAL";

/// WebSocket 关闭码及其处置。
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CloseKind {
    /// 正常关闭：不重连。
    Normal,
    /// 心跳超时/写失败：退避重连。
    Retry,
    /// 帧过大：不重连（客户端 bug）。
    FrameTooBig,
    /// 服务端内部错误：退避重连。
    ServerError,
    /// 顶号：不重连。
    Evicted,
    /// token 失效：清 token，回登录。
    TokenExpired,
    /// 协议版本不兼容：不重连，显示致命错误。
    ProtoMismatch,
    /// 频率超限被踢：60 秒后重连。
    RateLimited,
    /// 其它：按可重连处理。
    Unknown,
}

pub fn classify(code: u16) -> CloseKind {
    match code {
        1000 => CloseKind::Normal,
        1001 => CloseKind::Retry,
        1009 => CloseKind::FrameTooBig,
        1011 => CloseKind::ServerError,
        4001 => CloseKind::Evicted,
        4003 => CloseKind::TokenExpired,
        4004 => CloseKind::ProtoMismatch,
        4029 => CloseKind::RateLimited,
        _ => CloseKind::Unknown,
    }
}

impl CloseKind {
    pub fn retryable(self) -> bool {
        matches!(
            self,
            CloseKind::Retry | CloseKind::ServerError | CloseKind::RateLimited | CloseKind::Unknown
        )
    }

    /// 退避秒数；`attempt` 从 1 开始。
    pub fn backoff_secs(self, attempt: u32) -> u64 {
        if self == CloseKind::RateLimited {
            return 60;
        }
        let exp = 1u64 << attempt.min(5);
        exp.min(30)
    }

    pub fn user_message(self) -> &'static str {
        match self {
            CloseKind::Normal => "连接已关闭",
            CloseKind::Retry => "连接中断，正在重连",
            CloseKind::FrameTooBig => "发出的消息过大，已被服务端断开（客户端缺陷）",
            CloseKind::ServerError => "服务端异常，稍后再试",
            CloseKind::Evicted => "账号在别处登录",
            CloseKind::TokenExpired => "登录已过期，请重新登录",
            CloseKind::ProtoMismatch => "客户端版本与服务端不兼容，请升级客户端",
            CloseKind::RateLimited => "操作过频被踢下线，60 秒后可重连",
            CloseKind::Unknown => "连接断开，正在重连",
        }
    }
}
