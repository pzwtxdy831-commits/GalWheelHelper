package main

import (
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetClassNameW            = user32.NewProc("GetClassNameW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procQueryFullProcessImageName = kernel32.NewProc("QueryFullProcessImageNameW")

	detectorLock   sync.Mutex
	cachedHwnd     uintptr
	cachedIsGal    bool
	cachedProcName string
	cachedTitle    string
	cachedClass    string

	// 黑名单进程（绝不可能是 Galgame，防止日常操作误触）
	systemBlacklist = map[string]bool{
		"explorer.exe":               true,
		"dwm.exe":                    true,
		"taskmgr.exe":                true,
		"shellexperiencehost.exe":    true,
		"searchhost.exe":             true,
		"startmenuexperiencehost.exe": true,
		"textinputhost.exe":          true,
		"applicationframehost.exe":   true,
		"systemsettings.exe":         true,

		// 浏览器
		"chrome.exe":    true,
		"msedge.exe":    true,
		"firefox.exe":   true,
		"brave.exe":     true,
		"opera.exe":     true,
		"vivaldi.exe":   true,
		"360chrome.exe": true,

		// 开发与文本
		"code.exe":       true,
		"cursor.exe":     true,
		"notepad.exe":    true,
		"notepad++.exe":  true,
		"devenv.exe":     true,
		"idea64.exe":     true,
		"pycharm64.exe":  true,

		// 终端
		"windowsterminal.exe": true,
		"powershell.exe":      true,
		"cmd.exe":             true,
		"bash.exe":            true,
		"mintty.exe":          true,
		"conhost.exe":         true,

		// 社交与通讯
		"wechat.exe":      true,
		"qq.exe":          true,
		"dingtalk.exe":    true,
		"discord.exe":     true,
		"telegram.exe":    true,
		"feishu.exe":      true,
		"aliworkbench.exe": true,

		// 游戏平台本体与自身
		"steam.exe":             true,
		"steamservice.exe":      true,
		"epicgameslauncher.exe": true,
		"upc.exe":               true,
		"origin.exe":            true,
		"vnite.exe":             true, // Vnite 启动器界面本身排除
		"galwheelhelper.exe":    true,
	}

	// 常见 Galgame 引擎窗口类名 (包含即命中)
	knownGalClasses = []string{
		"tvpmainwindow", // 吉里吉里 (Kirikiri 2 / krkrZ)
		"bgi",           // BGI / Ethornell
		"ethornell",
		"majiro",        // Majiro
		"catsystem2",    // CS2
		"cs2",
		"siglusengine",  // Siglus
		"reallive",      // RealLive
		"visualart",
		"onscripter",    // ONScripter
		"nscripter",     // NScripter
		"artemis",       // Artemis
		"lucas",
		"unitywndclass", // Unity (ADV/同人)
		"sdl_app",       // Ren'Py
		"yu-ris",        // YU-RIS
		"qlie",          // QLIE
		"wolfrpg",       // Wolf RPG
		"rpg_rt",        // RPG Maker
		"rgssplayer",
		"malie",
		"advplayer",
		"silky",
		"cotopha",
		"entisgls",
		"systemnnn",
		"magpie",        // Magpie 全屏放大层
	}

	// 常见 Galgame 进程名特征
	knownGalProcKeywords = []string{
		"krkr", "tvp", "siglus", "reallive", "bgi", "ethornell",
		"majiro", "malie", "yuris", "qlie", "artemis", "renpy",
		"rio.exe", "adv.exe",
	}

	// 路径包含特征
	knownPathKeywords = []string{
		"gal", "galgame", "visualnovel", "eroge", "anim", "合集", "挂削和影视gal库", "dlsite", "dmm",
	}
)

type ForegroundInfo struct {
	Hwnd        uintptr
	ProcessName string
	ProcessPath string
	WindowTitle string
	ClassName   string
	IsGalgame   bool
}

func getForegroundInfo() ForegroundInfo {
	detectorLock.Lock()
	defer detectorLock.Unlock()

	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ForegroundInfo{}
	}

	if hwnd == cachedHwnd {
		return ForegroundInfo{
			Hwnd:        cachedHwnd,
			ProcessName: cachedProcName,
			WindowTitle: cachedTitle,
			ClassName:   cachedClass,
			IsGalgame:   cachedIsGal,
		}
	}

	// 重新获取
	info := analyzeWindow(hwnd)
	cachedHwnd = hwnd
	cachedIsGal = info.IsGalgame
	cachedProcName = info.ProcessName
	cachedTitle = info.WindowTitle
	cachedClass = info.ClassName

	return info
}

func analyzeWindow(hwnd uintptr) ForegroundInfo {
	info := ForegroundInfo{Hwnd: hwnd}

	// 1. 获取窗口标题
	var titleBuf [512]uint16
	r, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&titleBuf[0])), 512)
	if r > 0 {
		info.WindowTitle = syscall.UTF16ToString(titleBuf[:r])
	}

	// 2. 获取窗口类名
	var classBuf [256]uint16
	r, _, _ = procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&classBuf[0])), 256)
	if r > 0 {
		info.ClassName = syscall.UTF16ToString(classBuf[:r])
	}

	// 3. 获取进程 ID 与路径
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid > 0 {
		const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
		hProcess, _, _ := procOpenProcess.Call(PROCESS_QUERY_LIMITED_INFORMATION, 0, uintptr(pid))
		if hProcess != 0 {
			defer procCloseHandle.Call(hProcess)

			var pathBuf [1024]uint16
			size := uint32(1024)
			r, _, _ := procQueryFullProcessImageName.Call(
				hProcess,
				0,
				uintptr(unsafe.Pointer(&pathBuf[0])),
				uintptr(unsafe.Pointer(&size)),
			)
			if r != 0 && size > 0 {
				info.ProcessPath = syscall.UTF16ToString(pathBuf[:size])
				info.ProcessName = filepath.Base(info.ProcessPath)
			}
		}
	}

	info.IsGalgame = isGalgameWindow(info)
	return info
}

func isGalgameWindow(info ForegroundInfo) bool {
	procLower := strings.ToLower(info.ProcessName)
	classLower := strings.ToLower(info.ClassName)
	titleLower := strings.ToLower(info.WindowTitle)
	pathLower := strings.ToLower(info.ProcessPath)

	// 1. 黑名单绝对排除
	if systemBlacklist[procLower] {
		return false
	}

	cfg := getConfig()

	// 2. 用户自定义白名单 (最高优先级)
	for _, p := range cfg.CustomWhitelistProcesses {
		if p != "" && (procLower == p || strings.HasPrefix(procLower, p)) {
			return true
		}
	}
	for _, t := range cfg.CustomWhitelistTitles {
		if t != "" && strings.Contains(titleLower, t) {
			return true
		}
	}

	// 3. 常见 Galgame 引擎窗口类名
	for _, c := range knownGalClasses {
		if strings.Contains(classLower, c) {
			return true
		}
	}

	// 4. 进程名特征
	for _, kw := range knownGalProcKeywords {
		if strings.Contains(procLower, kw) {
			return true
		}
	}

	// 5. 汉化补丁/版本后缀特征
	if strings.HasSuffix(procLower, "_chs.exe") ||
		strings.HasSuffix(procLower, "_cn.exe") ||
		strings.HasSuffix(procLower, "_sc.exe") ||
		strings.HasSuffix(procLower, "_tc.exe") ||
		strings.HasSuffix(procLower, "_cht.exe") ||
		strings.HasSuffix(procLower, "_gpt.exe") ||
		strings.HasSuffix(procLower, "_zh.exe") {
		return true
	}

	// 6. 路径特征 (如果在 gal/visualnovel/合集 目录下运行)
	for _, kw := range knownPathKeywords {
		if strings.Contains(pathLower, kw) {
			return true
		}
	}

	return false
}
