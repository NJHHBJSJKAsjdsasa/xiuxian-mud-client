//! 日志环形缓冲：5000 行上限，O(1) 淘汰。

use std::collections::VecDeque;

use crate::protocol::msg::LogLine;
use crate::util::time;

pub struct Entry {
    #[allow(dead_code)] // 供 M5 日志搜索按序号定位。
    pub seq: u64,
    #[allow(dead_code)] // 对白框不显示时刻，留给 M5 的日志检索。
    pub ts: i64,
    pub line: LogLine,
}

pub struct LogBuf {
    cap: usize,
    lines: VecDeque<Entry>,
    seq: u64,
    /// 因超限被丢弃的行数，用于界面上"以上已省略 N 行"。
    dropped: u64,
}

impl LogBuf {
    pub fn new(cap: usize) -> Self {
        Self {
            cap: cap.max(1),
            lines: VecDeque::with_capacity(cap.min(512)),
            seq: 0,
            dropped: 0,
        }
    }

    pub fn push(&mut self, line: LogLine) {
        self.push_at(line, time::now_ms());
    }

    pub fn push_at(&mut self, line: LogLine, ts: i64) {
        self.seq += 1;
        self.lines.push_back(Entry {
            seq: self.seq,
            ts,
            line,
        });
        while self.lines.len() > self.cap {
            self.lines.pop_front();
            self.dropped += 1;
        }
    }

    pub fn len(&self) -> usize {
        self.lines.len()
    }

    #[allow(dead_code)]
    pub fn dropped(&self) -> u64 {
        self.dropped
    }

    #[allow(dead_code)]
    pub fn iter(&self) -> impl Iterator<Item = &Entry> {
        self.lines.iter()
    }

    pub fn get(&self, idx: usize) -> Option<&Entry> {
        self.lines.get(idx)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn line(s: &str) -> LogLine {
        LogLine {
            k: "sys".into(),
            s: s.into(),
        }
    }

    #[test]
    fn caps_and_counts_dropped() {
        let mut b = LogBuf::new(3);
        for i in 0..5 {
            b.push_at(line(&i.to_string()), 1000 + i);
        }
        assert_eq!(b.len(), 3);
        assert_eq!(b.dropped(), 2);
        assert_eq!(b.get(0).unwrap().line.s, "2");
        assert_eq!(b.get(0).unwrap().ts, 1002);
    }
}
