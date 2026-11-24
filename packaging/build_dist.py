"""组装 Windows 发布目录：启动器只是入口，游戏本体/资源/清单独立成文件。"""
import os
import shutil

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DIST = os.path.join(ROOT, "dist", "九州问道")

def main():
    if os.path.isdir(DIST):
        shutil.rmtree(DIST)
    for sub in ("game", "assets", "data"):
        os.makedirs(os.path.join(DIST, sub), exist_ok=True)
    shutil.copy2(os.path.join(ROOT, "bin", "launcher.exe"), os.path.join(DIST, "九州问道.exe"))
    shutil.copy2(os.path.join(ROOT, "bin", "gamehost.exe"), os.path.join(DIST, "game", "gamehost.exe"))
    shutil.copy2(os.path.join(ROOT, "bin", "playcli.dll"), os.path.join(DIST, "game", "playcli.dll"))
    for name in ("banner.bmp", "icon.ico"):
        shutil.copy2(os.path.join(ROOT, "assets", name), os.path.join(DIST, "assets", name))
    shutil.copy2(os.path.join(ROOT, "data", "manifest.json"), os.path.join(DIST, "data", "manifest.json"))
    shutil.copy2(os.path.join(ROOT, "packaging", "README.txt"), os.path.join(DIST, "README.txt"))
    print("dist ready")

if __name__ == "__main__":
    main()
