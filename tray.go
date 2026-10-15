package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const (
	WM_APP           = 0x8000
	WM_TRAYICON      = WM_APP + 1
	WM_COMMAND       = 0x0111
	WM_HOTKEY        = 0x0312
	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONUP     = 0x0205
	WM_CONTEXTMENU   = 0x007B
	WM_DESTROY       = 0x0002

	NIM_ADD         = 0x00000000
	NIM_MODIFY      = 0x00000001
	NIM_DELETE      = 0x00000002
	NIF_MESSAGE     = 0x00000001
	NIF_ICON        = 0x00000002
	NIF_TIP         = 0x00000004
	NIF_INFO        = 0x00000010

	NIIF_INFO       = 0x00000001

	MF_STRING       = 0x00000000
	MF_SEPARATOR    = 0x00000800
	MF_CHECKED      = 0x00000008
	MF_UNCHECKED    = 0x00000000
	MF_DISABLED     = 0x00000002
	MF_GRAYED       = 0x00000001

	TPM_RIGHTALIGN  = 0x0008
	TPM_BOTTOMALIGN = 0x0020
	TPM_RETURNCMD   = 0x0100

	MOD_ALT         = 0x0001
	MOD_CONTROL     = 0x0002
	MOD_SHIFT       = 0x0004
	MOD_NOREPEAT    = 0x4000

	VK_SCROLL       = 0x91
	VK_F11          = 0x7A

	ID_MODE_AUTO     = 1001
	ID_MODE_ALWAYS   = 1002
	ID_MODE_DISABLED = 1003
	ID_ADD_CURRENT   = 1004
	ID_AUTO_START    = 1005
	ID_OPEN_CONFIG   = 1006
	ID_RELOAD_CONFIG = 1007
	ID_DEBOUNCE_150  = 1008
	ID_DEBOUNCE_200  = 1009
	ID_DEBOUNCE_300  = 1010
	ID_ACTION_LEFT   = 1020
	ID_ACTION_RIGHT  = 1021
	ID_EXIT          = 1099
)

type NOTIFYICONDATA struct {
	cbSize            uint32
	hWnd              uintptr
	uID               uint32
	uFlags            uint32
	uCallbackMessage  uint32
	hIcon             uintptr
	szTip             [128]uint16
	dwState           uint32
	dwStateMask       uint32
	szInfo            [256]uint16
	uTimeoutOrVersion uint32
	szInfoTitle       [64]uint16
	dwInfoFlags       uint32
	guidItem          [16]byte
	hBalloonIcon      uintptr
}

type WNDCLASSEX struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type MSG struct {
	hwnd     uintptr
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       struct{ X, Y int32 }
	lPrivate uint32
}

var (
	shell32                  = syscall.NewLazyDLL("shell32.dll")
	procShell_NotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
	procRegisterClassExW     = user32.NewProc("RegisterClassExW")
	procCreateWindowExW      = user32.NewProc("CreateWindowExW")
	procDefWindowProcW       = user32.NewProc("DefWindowProcW")
	procDestroyWindow        = user32.NewProc("DestroyWindow")
	procPostQuitMessage      = user32.NewProc("PostQuitMessage")
	procGetMessageW          = user32.NewProc("GetMessageW")
	procTranslateMessage     = user32.NewProc("TranslateMessage")
	procDispatchMessageW     = user32.NewProc("DispatchMessageW")
	procCreatePopupMenu      = user32.NewProc("CreatePopupMenu")
	procAppendMenuW          = user32.NewProc("AppendMenuW")
	procTrackPopupMenu       = user32.NewProc("TrackPopupMenu")
	procDestroyMenu          = user32.NewProc("DestroyMenu")
	procSetForegroundWindow  = user32.NewProc("SetForegroundWindow")
	procGetCursorPos         = user32.NewProc("GetCursorPos")
	procRegisterHotKey       = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey     = user32.NewProc("UnregisterHotKey")
	procMessageBeep          = user32.NewProc("MessageBeep")
	procLoadImageW           = user32.NewProc("LoadImageW")
	procCreateIcon           = user32.NewProc("CreateIcon")

	mainHwnd         uintptr
	nid              NOTIFYICONDATA
	currentAppIcon   uintptr
	hotkeyRegistered bool
)

// 创建一个极简且耐看的内存图标 (16x16)
func createDefaultIcon() uintptr {
	// 尝试从本地加载 icon.ico
	exePath, err := os.Executable()
	if err == nil {
		icoPath := filepath.Join(filepath.Dir(exePath), "icon.ico")
		if _, err := os.Stat(icoPath); err == nil {
			pathU16, _ := syscall.UTF16FromString(icoPath)
			const IMAGE_ICON = 1
			const LR_LOADFROMFILE = 0x00000010
			const LR_DEFAULTSIZE = 0x00000040
			hIcon, _, _ := procLoadImageW.Call(
				0,
				uintptr(unsafe.Pointer(&pathU16[0])),
				IMAGE_ICON,
				0,
				0,
				LR_LOADFROMFILE|LR_DEFAULTSIZE,
			)
			if hIcon != 0 {
				return hIcon
			}
		}
	}

	// 动态生成一个 16x16 图标
	andMask := [32]byte{
		0b11111111, 0b11111111,
		0b11100000, 0b00000111,
		0b11000000, 0b00000011,
		0b10000000, 0b00000001,
		0b10000000, 0b00000001,
		0b00000000, 0b00000000,
		0b00000000, 0b00000000,
		0b00000000, 0b00000000,
		0b00000000, 0b00000000,
		0b00000000, 0b00000000,
		0b00000000, 0b00000000,
		0b10000000, 0b00000001,
		0b10000000, 0b00000001,
		0b11000000, 0b00000011,
		0b11100000, 0b00000111,
		0b11111111, 0b11111111,
	}

	xorMask := [32]byte{
		0x00, 0x00,
		0x1F, 0xF8,
		0x3F, 0xFC,
		0x7F, 0xFE,
		0x7F, 0xFE,
		0xFF, 0xFF,
		0xFF, 0xFF,
		0xFF, 0xFF,
		0xFF, 0xFF,
		0xFF, 0xFF,
		0xFF, 0xFF,
		0x7F, 0xFE,
		0x7F, 0xFE,
		0x3F, 0xFC,
		0x1F, 0xF8,
		0x00, 0x00,
	}

	hIcon, _, _ := procCreateIcon.Call(
		0,
		16,
		16,
		1,
		1,
		uintptr(unsafe.Pointer(&andMask[0])),
		uintptr(unsafe.Pointer(&xorMask[0])),
	)
	return hIcon
}

func initTray(hwnd uintptr) {
	currentAppIcon = createDefaultIcon()

	nid = NOTIFYICONDATA{
		cbSize:           uint32(unsafe.Sizeof(nid)),
		hWnd:             hwnd,
		uID:              1,
		uFlags:           NIF_MESSAGE | NIF_ICON | NIF_TIP,
		uCallbackMessage: WM_TRAYICON,
		hIcon:            currentAppIcon,
	}

	updateTrayTooltip()
	procShell_NotifyIconW.Call(uintptr(NIM_ADD), uintptr(unsafe.Pointer(&nid)))
}

func updateTrayTooltip() {
	cfg := getConfig()
	var modeText string
	switch cfg.Mode {
	case "always":
		modeText = "【全局强制开启】"
	case "disabled":
		modeText = "【已暂停禁用】"
	default:
		modeText = "【自动检测Gal前台】"
	}

	actionText := "左键(进剧情)"
	if cfg.TargetAction == "right" {
		actionText = "右键(隐界面)"
	}

	tip := fmt.Sprintf("Gal滚轮助手 %s ➔ %s\n双击托盘或按快捷键切换", modeText, actionText)
	tipU16, _ := syscall.UTF16FromString(tip)
	copy(nid.szTip[:], tipU16)
	procShell_NotifyIconW.Call(uintptr(NIM_MODIFY), uintptr(unsafe.Pointer(&nid)))
}

func showBalloon(title, message string) {
	nid.uFlags |= NIF_INFO
	nid.dwInfoFlags = NIIF_INFO

	titleU16, _ := syscall.UTF16FromString(title)
	copy(nid.szInfoTitle[:], titleU16)

	msgU16, _ := syscall.UTF16FromString(message)
	copy(nid.szInfo[:], msgU16)

	procShell_NotifyIconW.Call(uintptr(NIM_MODIFY), uintptr(unsafe.Pointer(&nid)))

	if getConfig().SoundFeedback {
		const MB_ICONASTERISK = 0x00000040
		procMessageBeep.Call(uintptr(MB_ICONASTERISK))
	}
}

func toggleQuickMode() {
	cfg := getConfig()
	if cfg.Mode == "disabled" {
		setMode("auto")
		showBalloon("Gal滚轮助手已开启", "当前为【自动检测模式】，仅在Galgame前台生效。")
	} else {
		setMode("disabled")
		showBalloon("Gal滚轮助手已暂停", "鼠标下滚已恢复为默认滚动行为。")
	}
	updateTrayTooltip()
}

func setupHotkey(hwnd uintptr) {
	cfg := getConfig()
	hotkeyStr := strings.ToLower(strings.TrimSpace(cfg.Hotkey))

	var mod uint32 = MOD_NOREPEAT
	var vk uint32 = VK_SCROLL

	if strings.Contains(hotkeyStr, "ctrl") {
		mod |= MOD_CONTROL
	}
	if strings.Contains(hotkeyStr, "alt") {
		mod |= MOD_ALT
	}
	if strings.Contains(hotkeyStr, "shift") {
		mod |= MOD_SHIFT
	}

	if strings.Contains(hotkeyStr, "f11") {
		vk = VK_F11
	} else if strings.Contains(hotkeyStr, "scroll") {
		vk = VK_SCROLL
	} else if len(hotkeyStr) == 1 && hotkeyStr[0] >= 'a' && hotkeyStr[0] <= 'z' {
		vk = uint32(hotkeyStr[0] - 'a' + 'A')
	}

	const HOTKEY_ID = 1
	r, _, _ := procRegisterHotKey.Call(hwnd, uintptr(HOTKEY_ID), uintptr(mod), uintptr(vk))
	if r != 0 {
		hotkeyRegistered = true
	}
}

func showContextMenu(hwnd uintptr) {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	appendMenuItem := func(id uintptr, text string, flags uint32) {
		tU16, _ := syscall.UTF16FromString(text)
		procAppendMenuW.Call(hMenu, uintptr(flags), id, uintptr(unsafe.Pointer(&tU16[0])))
	}

	cfg := getConfig()
	fg := getForegroundInfo()

	// 标题
	appendMenuItem(0, "🌸 Gal 滚轮助手 v1.0", MF_DISABLED|MF_GRAYED)
	appendMenuItem(0, "", MF_SEPARATOR)

	// 映射目标设置 (左键 / 右键)
	leftFlag := uint32(MF_UNCHECKED)
	rightFlag := uint32(MF_UNCHECKED)
	if cfg.TargetAction == "right" {
		rightFlag = MF_CHECKED
	} else {
		leftFlag = MF_CHECKED
	}
	appendMenuItem(uintptr(ID_ACTION_LEFT), "🖱️ 滚轮下滚 ➔ 鼠标左键 (推进剧情/点击)", MF_STRING|leftFlag)
	appendMenuItem(uintptr(ID_ACTION_RIGHT), "🖱️ 滚轮下滚 ➔ 鼠标右键 (隐藏界面/菜单)", MF_STRING|rightFlag)
	appendMenuItem(0, "", MF_SEPARATOR)

	// 运行模式单选
	autoFlag := uint32(MF_UNCHECKED)
	alwaysFlag := uint32(MF_UNCHECKED)
	disabledFlag := uint32(MF_UNCHECKED)
	switch cfg.Mode {
	case "always":
		alwaysFlag = MF_CHECKED
	case "disabled":
		disabledFlag = MF_CHECKED
	default:
		autoFlag = MF_CHECKED
	}
	appendMenuItem(uintptr(ID_MODE_AUTO), "🟢 自动检测模式 (推荐，仅Gal前台生效)", MF_STRING|autoFlag)
	appendMenuItem(uintptr(ID_MODE_ALWAYS), "🟡 全局强制模式 (所有窗口生效)", MF_STRING|alwaysFlag)
	appendMenuItem(uintptr(ID_MODE_DISABLED), "⚪ 暂停 / 禁用", MF_STRING|disabledFlag)

	appendMenuItem(0, "", MF_SEPARATOR)

	// 当前前台检测状态显示
	var fgStatus string
	if fg.ProcessName != "" {
		if fg.IsGalgame {
			fgStatus = fmt.Sprintf("🔍 当前前台: %s 【已识别为Gal】", fg.ProcessName)
		} else {
			fgStatus = fmt.Sprintf("🔍 当前前台: %s 【未识别】", fg.ProcessName)
		}
	} else {
		fgStatus = "🔍 当前前台: 无"
	}
	appendMenuItem(0, fgStatus, MF_DISABLED|MF_GRAYED)

	if fg.ProcessName != "" && !fg.IsGalgame {
		appendMenuItem(uintptr(ID_ADD_CURRENT), fmt.Sprintf("➕ 将 %s 加入白名单", fg.ProcessName), MF_STRING)
	}

	appendMenuItem(0, "", MF_SEPARATOR)

	// 冷却防抖菜单
	d150Flag := uint32(MF_UNCHECKED)
	d200Flag := uint32(MF_UNCHECKED)
	d300Flag := uint32(MF_UNCHECKED)
	if cfg.DebounceMs <= 160 {
		d150Flag = MF_CHECKED
	} else if cfg.DebounceMs <= 250 {
		d200Flag = MF_CHECKED
	} else {
		d300Flag = MF_CHECKED
	}
	appendMenuItem(uintptr(ID_DEBOUNCE_150), "⏱️ 滚轮防抖: 150毫秒 (极速灵敏)", MF_STRING|d150Flag)
	appendMenuItem(uintptr(ID_DEBOUNCE_200), "⏱️ 滚轮防抖: 200毫秒 (标准平衡)", MF_STRING|d200Flag)
	appendMenuItem(uintptr(ID_DEBOUNCE_300), "⏱️ 滚轮防抖: 300毫秒 (稳健防误触)", MF_STRING|d300Flag)

	appendMenuItem(0, "", MF_SEPARATOR)

	// 开机启动
	autoStartFlag := uint32(MF_UNCHECKED)
	if isAutoStartEnabled() {
		autoStartFlag = MF_CHECKED
	}
	appendMenuItem(uintptr(ID_AUTO_START), "🚀 开机自动启动", MF_STRING|autoStartFlag)
	appendMenuItem(uintptr(ID_OPEN_CONFIG), "⚙️ 打开配置文件 (config.json)", MF_STRING)
	appendMenuItem(uintptr(ID_RELOAD_CONFIG), "🔄 重新载入配置", MF_STRING)

	appendMenuItem(0, "", MF_SEPARATOR)
	appendMenuItem(uintptr(ID_EXIT), "❌ 退出", MF_STRING)

	// 获取鼠标光标位置
	var pt struct{ X, Y int32 }
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	procSetForegroundWindow.Call(hwnd)
	cmd, _, _ := procTrackPopupMenu.Call(
		hMenu,
		uintptr(TPM_RIGHTALIGN|TPM_BOTTOMALIGN|TPM_RETURNCMD),
		uintptr(pt.X),
		uintptr(pt.Y),
		0,
		hwnd,
		0,
	)

	handleMenuCommand(cmd, fg.ProcessName)
}

func handleMenuCommand(cmd uintptr, currentFgProc string) {
	switch cmd {
	case ID_MODE_AUTO:
		setMode("auto")
		updateTrayTooltip()
		showBalloon("已切换为: 自动检测模式", "仅在检测到Galgame在前台时，向下滚轮才映射为右键。")
	case ID_MODE_ALWAYS:
		setMode("always")
		updateTrayTooltip()
		showBalloon("已切换为: 全局强制模式", "所有窗口下，鼠标向下滚轮均相当于右键单击。")
	case ID_MODE_DISABLED:
		setMode("disabled")
		updateTrayTooltip()
		showBalloon("已切换为: 暂停/禁用", "滚轮已恢复原生功能。")
	case ID_ADD_CURRENT:
		if currentFgProc != "" {
			if addProcessToWhitelist(currentFgProc) {
				showBalloon("白名单添加成功", fmt.Sprintf("已将 %s 加入永久白名单！", currentFgProc))
			}
		}
	case ID_DEBOUNCE_150:
		cfgLock.Lock()
		currentConfig.DebounceMs = 150
		saveConfigLocked()
		cfgLock.Unlock()
	case ID_DEBOUNCE_200:
		cfgLock.Lock()
		currentConfig.DebounceMs = 200
		saveConfigLocked()
		cfgLock.Unlock()
	case ID_DEBOUNCE_300:
		cfgLock.Lock()
		currentConfig.DebounceMs = 300
		saveConfigLocked()
		cfgLock.Unlock()
	case ID_ACTION_LEFT:
		cfgLock.Lock()
		currentConfig.TargetAction = "left"
		saveConfigLocked()
		cfgLock.Unlock()
		updateTrayTooltip()
		showBalloon("已切换映射: 鼠标左键", "现在向下滚动滚轮将模拟【鼠标左键单击】（轻松推进剧情）。")
	case ID_ACTION_RIGHT:
		cfgLock.Lock()
		currentConfig.TargetAction = "right"
		saveConfigLocked()
		cfgLock.Unlock()
		updateTrayTooltip()
		showBalloon("已切换映射: 鼠标右键", "现在向下滚动滚轮将模拟【鼠标右键单击】。")
	case ID_AUTO_START:
		enabled := isAutoStartEnabled()
		_ = setAutoStart(!enabled)
	case ID_OPEN_CONFIG:
		go func() {
			_ = exec.Command("notepad.exe", configFilePath).Start()
		}()
	case ID_RELOAD_CONFIG:
		loadConfig()
		updateTrayTooltip()
		showBalloon("配置已重载", "已重新读取 config.json")
	case ID_EXIT:
		cleanupAndExit()
	}
}

func wndProc(hwnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	switch msg {
	case WM_TRAYICON:
		switch lParam {
		case WM_RBUTTONUP, WM_CONTEXTMENU:
			showContextMenu(hwnd)
		case WM_LBUTTONDBLCLK:
			toggleQuickMode()
		}
		return 0

	case WM_HOTKEY:
		toggleQuickMode()
		return 0

	case WM_DESTROY:
		cleanupAndExit()
		return 0
	}

	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func cleanupAndExit() {
	if hotkeyRegistered {
		procUnregisterHotKey.Call(mainHwnd, 1)
	}
	uninstallMouseHook()
	procShell_NotifyIconW.Call(uintptr(NIM_DELETE), uintptr(unsafe.Pointer(&nid)))
	procPostQuitMessage.Call(0)
}
