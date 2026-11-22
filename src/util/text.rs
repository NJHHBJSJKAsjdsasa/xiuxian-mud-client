//! 文本：千分位、按字符截断。

/// `12340` → `12,340`；负数保留符号。
pub fn thousands(n: i64) -> String {
    let neg = n < 0;
    let s = n.unsigned_abs().to_string();
    let mut out = String::with_capacity(s.len() + s.len() / 3 + 1);
    if neg {
        out.push('-');
    }
    for (i, ch) in s.chars().enumerate() {
        if i > 0 && (s.len() - i).is_multiple_of(3) {
            out.push(',');
        }
        out.push(ch);
    }
    out
}

/// 按**字符**截断（不是字节），超出加省略号。
pub fn truncate_chars(s: &str, max: usize) -> String {
    let n = s.chars().count();
    if n <= max {
        return s.to_string();
    }
    let mut out: String = s.chars().take(max.saturating_sub(1)).collect();
    out.push('…');
    out
}

pub fn percent(cur: i64, max: i64) -> f32 {
    if max <= 0 {
        return 0.0;
    }
    (cur as f32 / max as f32).clamp(0.0, 1.0)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn thousand_separators() {
        assert_eq!(thousands(0), "0");
        assert_eq!(thousands(999), "999");
        assert_eq!(thousands(1000), "1,000");
        assert_eq!(thousands(12340), "12,340");
        assert_eq!(thousands(1234567), "1,234,567");
        assert_eq!(thousands(-4200), "-4,200");
    }

    #[test]
    fn truncate_is_char_based() {
        assert_eq!(truncate_chars("青云·望云台", 3), "青云…");
        assert_eq!(truncate_chars("短", 3), "短");
    }
}
