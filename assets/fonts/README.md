# 字体

客户端**不要求**这里有文件：`src/ui/fonts.rs` 先找本目录的 `NotoSansSC-Regular.subset.otf`，
找不到就读系统字体（Windows `msyh.ttc` / macOS `PingFang.ttc` / Linux `NotoSansCJK`）。

## 为什么要内置

系统字体在三平台的字形、行高、标点宽度不一致，截图与排版回归会飘。发布版建议把子集字体编进二进制：

```bash
pip install fonttools
# charset.txt：GB2312 一级+二级汉字、ASCII、常用标点、界面符号（⏵ ✕ ✓ · — … 「」《》）
pyftsubset NotoSansSC-Regular.otf \
  --text-file=charset.txt \
  --output-file=NotoSansSC-Regular.subset.otf \
  --layout-features='*' --glyph-names --symbol-cmap --legacy-cmap \
  --notdef-glyph --notdef-outline --recommended-glyphs
```

- 目标体积：单字重 2–3MB（6763 汉字 + 标点）。
- 许可证：SIL OFL 1.1，发布时随包附 `LICENSE.txt`（内嵌字体也要保留许可声明）。
- 新增界面用字先进 `charset.txt` 再重新裁剪；缺字会在启动自检里记 warn 并列出码位。
