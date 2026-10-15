# 🌸 GalWheelHelper（Gal 滚轮助手）

[中文](#中文) | [English](#english)

---

## 中文

推 Galgame / 视觉小说时，**鼠标滚轮向下滚一格 = 鼠标左键单击（推进剧情）**。
单手握鼠标、用手指轻拨滚轮就能一直过剧情，不用反复按左键。托盘菜单里也可以切换成**右键（隐藏界面 / 呼出菜单）**。

### ✨ 特性

1. **滚轮向下即点击**
   - 向下滚动：模拟一次左键（或右键）单击。
   - 向上滚动：原样放行，不影响游戏的回看（Backlog）。
   - 防抖冷却（默认 150ms，可配置）：滚得太快也不会一下跳过好几句。

2. **只在 Galgame 里生效**
   - 前台是浏览器、代码编辑器、资源管理器、终端、聊天软件时不触发，日常使用不受影响。
   - 识别常见引擎：吉里吉里（Kirikiri 2 / KirikiriZ / TVP）、Siglus / RealLive、BGI / Ethornell、CatSystem2、Majiro、Unity ADV、Ren'Py、YU-RIS、QLIE 等。
   - 识别汉化版程序名（如 `*_chs.exe`、`*_cn.exe`）以及放在 Gal / 合集目录下的游戏。
   - 兼容 Magpie 超分全屏。

3. **轻量**
   - 纯 Win32 底层鼠标钩子（`WH_MOUSE_LL`），单文件约 2.4 MB，无控制台窗口，空闲时几乎不占 CPU。

4. **托盘控制**
   - 切换左键 / 右键映射。
   - 模式：自动检测 / 全局强制 / 暂停。
   - 查看当前前台窗口是否被识别，一键加入白名单。
   - 开机自启动开关。
   - 全局热键 `Scroll Lock`：启用 ⇄ 暂停。

### 🎮 使用

1. 从 Releases 下载 `GalWheelHelper.exe`，双击运行（或使用 `启动助手(无黑框).vbs`）。
2. 右下角出现粉色托盘图标即表示运行中。
3. 打开 Galgame，向下滚动滚轮即可推进剧情。

首次运行会在程序旁生成 `config.json`，可修改映射按键、模式、防抖时间、热键与自定义白名单。

### 🔧 从源码构建

需要 Go 1.26 及以上，仅支持 Windows：

```bash
go test ./...
go build -ldflags "-H windowsgui -s -w" -o GalWheelHelper.exe
```

---

## English

GalWheelHelper turns **mouse wheel-down into a click** while a visual novel is in the foreground, so you can advance dialogue one-handed. Wheel-up passes through untouched, so the game's backlog still works.

- Auto-detects common VN engines (Kirikiri, Siglus, BGI, CatSystem2, Majiro, Ren'Py, Unity ADV, YU-RIS, QLIE and more) and stays off in browsers, editors, terminals and chat apps.
- Configurable debounce, so fast scrolling does not skip several lines.
- Tray menu: left/right click mapping, auto / always-on / paused modes, whitelist the current window, start with Windows. `Scroll Lock` toggles it globally.
- Native Win32 low-level mouse hook, ~2.4 MB single binary, no console window.

**Build:** `go build -ldflags "-H windowsgui -s -w" -o GalWheelHelper.exe` (Go 1.26+, Windows only).

---

## License

[MIT](LICENSE)
