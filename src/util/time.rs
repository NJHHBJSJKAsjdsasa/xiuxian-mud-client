//! 时间：服务端毫秒时间戳与展示格式。

use time::{OffsetDateTime, UtcOffset, macros::format_description};

pub fn now_ms() -> i64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_millis() as i64)
        .unwrap_or(0)
}

/// 服务端时区固定为 Asia/Shanghai（东八区）。
fn offset() -> UtcOffset {
    UtcOffset::from_hms(8, 0, 0).unwrap_or(UtcOffset::UTC)
}

/// 毫秒时间戳 → `HH:MM:SS`。
#[allow(dead_code)] // 日志时刻，留给 M5 的检索视图
pub fn fmt_hms(ts_ms: i64) -> String {
    let secs = ts_ms.div_euclid(1000);
    let Ok(dt) = OffsetDateTime::from_unix_timestamp(secs) else {
        return "--:--:--".into();
    };
    let dt = dt.to_offset(offset());
    dt.format(format_description!("[hour]:[minute]:[second]"))
        .unwrap_or_else(|_| "--:--:--".into())
}

/// 毫秒 → `MM:SS`（负数按 0）。
pub fn fmt_countdown(ms: i64) -> String {
    let total = (ms.max(0) + 999) / 1000;
    format!("{:02}:{:02}", total / 60, total % 60)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn hms_in_east8() {
        // 2026-01-01T00:00:00Z = 08:00:00 +08:00
        assert_eq!(fmt_hms(1_767_225_600_000), "08:00:00");
    }

    #[test]
    fn countdown_rounds_up() {
        assert_eq!(fmt_countdown(3_400), "00:04");
        assert_eq!(fmt_countdown(0), "00:00");
        assert_eq!(fmt_countdown(-5), "00:00");
        assert_eq!(fmt_countdown(61_000), "01:01");
    }
}
