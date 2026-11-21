#!/usr/bin/env python3
# 客户端文档一致性：doc_id / version 逐份对上 C-00 矩阵，相对链接必须存在。
#
# 为什么客户端单独一条：这些文档不在服务端仓库了，服务端的 doc-check 看不到它们；
# 没有这条，改一份头版忘更新矩阵就没人发现。
#
# 用法：python tools/doc-check.py          # 全部通过退出 0
#       python tools/doc-check.py --quiet  # 只报失败
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DOCS = ROOT / "docs"
QUIET = "--quiet" in sys.argv
problems = []


def fail(msg):
    problems.append(msg)


def main():
    files = sorted(DOCS.glob("*.md"))
    if not files:
        fail("docs/ 下没有任何文档")
        return
    header = {}
    for f in files:
        t = f.read_text(encoding="utf-8")
        mid = re.search(r"(?m)^doc_id:\s*(C-\d+)\s*$", t)
        mver = re.search(r"(?m)^version:\s*(\S+)\s*$", t)
        if not mid or not mver:
            fail(f"{f.name} 缺 doc_id/version 头")
            continue
        num = mid.group(1).split("-")[-1]
        if not f.name.startswith(num):
            fail(f"{f.name} 的 doc_id（{mid.group(1)}）与文件名编号不对应")
        header[f.name] = (mid.group(1), mver.group(1))

    idx = (DOCS / "00-INDEX.md").read_text(encoding="utf-8")
    rows = re.findall(r"(?m)^\| `(C-\d+)` \| `([\w.-]+\.md)` \| ([0-9.]+) \|", idx)
    listed = {name: (cid, ver) for cid, name, ver in rows}
    for name, (cid, ver) in header.items():
        if name == "00-INDEX.md":
            continue
        if name not in listed:
            fail(f"docs/{name} 没进 C-00 矩阵")
        elif listed[name][1] != ver:
            fail(f"docs/{name} 头版 {ver} ≠ 矩阵 {listed[name][1]}")
    for name in listed:
        if name not in header:
            fail(f"C-00 矩阵列了 {name}，文件不存在或没有版本头")

    for f in files:
        t = f.read_text(encoding="utf-8")
        for target in re.findall(r"\]\((\.{0,2}/?[\w./-]+\.md)\)", t):
            if not (f.parent / target).resolve().exists():
                fail(f"链接失效：{f.name} → {target}")

    if problems:
        print("== 失败 ==")
        for p in problems:
            print("  " + p)
        print(f"\n{len(problems)} 项未通过")
        sys.exit(1)
    if not QUIET:
        print(f"文档一致性：{len(header)} 份，矩阵与链接一致")


main()
