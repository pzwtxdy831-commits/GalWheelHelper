package main

import (
	"log"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	procCreateMutexW = kernel32.NewProc("CreateMutexW")
	procGetLastError = kernel32.NewProc("GetLastError")
)

const (
	ERROR_ALREADY_EXISTS = 183
)

func checkSingleInstance() (uintptr, bool) {
	mutexName, _ := syscall.UTF16PtrFromString("Local\\GalWheelHelperSingleInstanceMutex")
	hMutex, _, _ := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(mutexName)))
	lastErr, _, _ := procGetLastError.Call()
	if lastErr == ERROR_ALREADY_EXISTS {
		return hMutex, false
	}
	return hMutex, true
}

func main() {
	// 锁在 OS 线程上以确保 Win32 消息泵正常运作
	runtime.LockOSThread()

	// 1. 单实例检查
	_, isUnique := checkSingleInstance()
	if !isUnique {
		// 已有实例在运行，直接退出
		os.Exit(0)
	}

	// 2. 加载配置文件
	loadConfig()

	// 3. 注册纯后台消息窗口类
	className, _ := syscall.UTF16PtrFromString("GalWheelHelperWndClass")
	windowTitle, _ := syscall.UTF16PtrFromString("GalWheelHelperWindow")

	wndClass := WNDCLASSEX{
		cbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		lpfnWndProc:   syscall.NewCallback(wndProc),
		lpszClassName: className,
	}

	atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wndClass)))
	if atom == 0 {
		log.Fatalf("RegisterClassExW failed: %v", err)
	}

	// 4. 创建隐藏窗口
	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowTitle)),
		0,
		0, 0, 0, 0,
		0, 0, 0, 0,
	)
	if hwnd == 0 {
		log.Fatalf("CreateWindowExW failed: %v", err)
	}
	mainHwnd = hwnd

	// 5. 初始化托盘图标
	initTray(hwnd)

	// 6. 安装底层鼠标钩子
	if err := installMouseHook(); err != nil {
		showBalloon("安装鼠标钩子失败", "无法启动全局滚轮监控，请尝试以管理员身份运行。")
		cleanupAndExit()
		return
	}

	// 7. 注册全局快捷键 (如 ScrollLock)
	setupHotkey(hwnd)

	// 启动成功通知
	showBalloon(
		"Gal滚轮助手已在后台就绪 🌸",
		"当前为【自动检测模式】。前台玩Gal时向下滚轮即为【鼠标左键】轻松推进剧情！\n可按 ScrollLock 键或右键托盘切换左/右键与模式。",
	)

	// 8. Win32 消息循环
	var msg MSG
	for {
		r, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0,
			0,
			0,
		)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
