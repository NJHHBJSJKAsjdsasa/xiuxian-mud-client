九州问道 · 客户端
=================

双击「九州问道.exe」打开登录/注册窗口，成功后自动进入游戏。

目录说明
--------
  九州问道.exe          启动器：登录 / 注册 / 拉起游戏
  game/gamehost.exe     游戏宿主：负责加载 DLL
  game/playcli.dll      游戏本体（DLL；不要单独移动）
  assets/banner.bmp     启动器横幅（缺失时退回纯文字标题）
  assets/icon.ico       窗口图标
  data/manifest.json    版本、游戏路径与默认服务器

个人配置
--------
  %APPDATA%\xiuxian-client\playcli.json
  只保存服务器地址与上次道号，不保存密码。

排错
----
  · 提示“没找到游戏本体”：确认 game/gamehost.exe 与 game/playcli.dll 都在，或检查 manifest 的 game。
  · 登录失败：先在登录窗口把服务器改成正确地址；错误原因会显示在窗口底部。
  · 游戏画面可直接双击窗口标题栏拖动；舆图页支持鼠标拖动、双击回中、滚轮翻页。
