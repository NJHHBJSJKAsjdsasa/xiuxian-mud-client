九州问道 · 客户端
=================

双击「九州问道.exe」打开登录/注册窗口，成功后自动进入游戏。
发布目录里唯一的 exe 是登录启动器；游戏宿主与本体都是 DLL。

目录说明
--------
  九州问道.exe          启动器：登录 / 注册 / 拉起游戏（唯一 exe）
  game/gamehost.dll     游戏宿主（DLL）：由系统 rundll32 加载
  game/playcli.dll      游戏本体（DLL；不要单独移动）
  assets/banner.bmp     启动器横幅（缺失时退回纯文字标题）
  assets/icon.ico       窗口图标
  data/manifest.json    版本、游戏路径与默认服务器

个人配置
--------
  %APPDATA%\xiuxian-client\playcli.json   服务器、上次道号、窗口模式
  %APPDATA%\xiuxian-client\accounts.json  启动器账号库；密码为 Windows DPAPI 密文
  删除 accounts.json 即清空已存账号。

排错
----
  · 提示“没找到游戏宿主”：确认 game/gamehost.dll 与 game/playcli.dll 都在，或检查 manifest 的 game。
  · 提示“请通过登录启动器进入游戏”：不要直接运行 game/gamehost.dll；从九州问道.exe 登录。
  · 登录失败：先在登录窗口把服务器改成正确地址；错误原因会显示在窗口底部。
  · 无边框窗口可按住四周纯黑外框拖动；F8 → 设置 → 窗口可切回带边框。