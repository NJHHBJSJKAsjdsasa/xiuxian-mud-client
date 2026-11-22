import re, sys, unicodedata
from PIL import Image, ImageDraw, ImageFont

src, dst = sys.argv[1], sys.argv[2]
raw = open(src, 'r', encoding='utf-8').read().split('\n')
pat = re.compile(r'\x1b\[38;2;(\d+);(\d+);(\d+)m\x1b\[48;2;(\d+);(\d+);(\d+)m')


def cell_w(ch):
    """和终端一致的列宽：全角/宽字符占 2 列，其余 1 列。
    用 east_asian_width 而不是码位分界——破折号、间隔号这类"宽窄两可"的字符
    在终端里是 1 列，按码位算会把后面整个字挤歪。"""
    return 2 if unicodedata.east_asian_width(ch) in ('W', 'F') else 1

CW, CH = 8, 18
font = ImageFont.truetype(r'C:\Windows\Fonts\msyh.ttc', 15)

parsed = []
for li, line in enumerate(raw):
    cells = []
    pos = 0
    while pos < len(line):
        m = pat.match(line, pos)
        if not m:
            break
        fg = (int(m.group(1)), int(m.group(2)), int(m.group(3)))
        bg = (int(m.group(4)), int(m.group(5)), int(m.group(6)))
        pos = m.end()
        ch = line[pos] if pos < len(line) else ' '
        pos += 1
        cells.append((ch, fg, bg))
    if cells:
        parsed.append((li, cells))

cols = max(len(c) for _, c in parsed)
W, H = cols * CW, len(raw) * CH
img = Image.new('RGB', (W, H), (5, 7, 9))
d = ImageDraw.Draw(img)

for li, cells in parsed:
    x = 0
    for ch, fg, bg in cells:
        w = cell_w(ch)
        d.rectangle([x * CW, li * CH, (x + w) * CW - 1, (li + 1) * CH - 1], fill=bg)
        if ch.strip():
            d.text((x * CW, li * CH), ch, font=font, fill=fg)
        x += w

img.save(dst)
print('saved', dst, img.size, 'lines', len(parsed), 'rows', len(raw))
