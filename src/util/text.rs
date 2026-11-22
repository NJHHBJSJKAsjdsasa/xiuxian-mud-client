//! 文本：千分位、按显示宽度截断与补齐。

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

/// 一个字符在终端里占几列。
///
/// 直接用 `unicode-width`：ratatui 排版也用它，两边算出来的列数必须一致，
/// 否则快照导出、地图标签会错位（全角标点、破折号都是 2 列）。
pub fn char_width(c: char) -> usize {
    unicode_width::UnicodeWidthChar::width(c).unwrap_or(0)
}

/// 字符串的显示宽度（列数）。
pub fn width(s: &str) -> usize {
    unicode_width::UnicodeWidthStr::width(s)
}

/// 按**显示宽度**截断，超出部分用省略号占两列。
pub fn truncate_width(s: &str, max: usize) -> String {
    if width(s) <= max {
        return s.to_string();
    }
    let mut out = String::new();
    let mut used = 0;
    for c in s.chars() {
        let w = char_width(c);
        if used + w + 2 > max {
            break;
        }
        out.push(c);
        used += w;
    }
    out.push('…');
    out
}

/// 补空格到指定显示宽度；已超出则原样返回。
pub fn pad_to(s: &str, target: usize) -> String {
    let w = width(s);
    if w >= target {
        return s.to_string();
    }
    format!("{s}{}", " ".repeat(target - w))
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

    #[test]
    fn width_counts_cjk_as_two_columns() {
        assert_eq!(width("abc"), 3);
        assert_eq!(width("青云"), 4);
        assert_eq!(width("青云 abc"), 8);
    }

    #[test]
    fn truncate_width_never_overflows() {
        for max in 0..12 {
            assert!(width(&truncate_width("青云·望云台 12s", max)) <= max.max(2));
        }
        assert_eq!(truncate_width("短", 4), "短");
    }

    #[test]
    fn pad_to_uses_display_width() {
        assert_eq!(pad_to("青云", 6), "青云  ");
        assert_eq!(width(&pad_to("青云", 6)), 6);
        assert_eq!(pad_to("青云问道阁", 4), "青云问道阁");
    }
}
