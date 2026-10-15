package main

import (
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const (
	WH_MOUSE_LL           = 14
	WM_MOUSEWHEEL         = 0x020A
	MOUSEEVENTF_LEFTDOWN  = 0x0002
	MOUSEEVENTF_LEFTUP    = 0x0004
	MOUSEEVENTF_RIGHTDOWN = 0x0008
	MOUSEEVENTF_RIGHTUP   = 0x0010
	EXTRA_FLAG_MAGIC      = 0x53414B55 // "SAKU"
)

var (
	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procMouseEvent          = user32.NewProc("mouse_event")

	mouseHookHandle uintptr
	lastTriggerTime int64 // UnixMilli
	clickCount      int64 // 统计触发次数
)

func mouseHookProc(nCode int32, wParam uintptr, lParam uintptr) uintptr {
	if nCode >= 0 && wParam == WM_MOUSEWHEEL {
		// lParam + 8 为 mouseData (uint32)
		mouseData := *(*uint32)(unsafe.Pointer(lParam + 8))
		wheelDelta := int16(mouseData >> 16)

		// 仅处理向下滚轮 (wheelDelta < 0)
		// 向上滚轮 (wheelDelta > 0) 原样放行，保留 Galgame 向上回溯历史文本 (Backlog) 的核心操作
		if wheelDelta < 0 {
			cfg := getConfig()

			shouldTrigger := false
			if cfg.Mode == "always" {
				shouldTrigger = true
			} else if cfg.Mode == "auto" {
				fg := getForegroundInfo()
				if fg.IsGalgame {
					shouldTrigger = true
				}
			}

			if shouldTrigger {
				now := time.Now().UnixMilli()
				debounce := int64(cfg.DebounceMs)
				if debounce <= 0 {
					debounce = 180
				}

				if now-lastTriggerTime < debounce {
					// 冷却期内吸收多余的下滚消息，避免右键过快连击导致菜单闪烁
					return 1
				}

				lastTriggerTime = now
				atomic.AddInt64(&clickCount, 1)

				action := cfg.TargetAction

				// 异步派发点击事件，避免阻塞底层钩子线程引发 Windows 钩子超时
				go func() {
					if action == "right" {
						procMouseEvent.Call(
							uintptr(MOUSEEVENTF_RIGHTDOWN),
							0,
							0,
							0,
							uintptr(EXTRA_FLAG_MAGIC),
						)
						time.Sleep(12 * time.Millisecond)
						procMouseEvent.Call(
							uintptr(MOUSEEVENTF_RIGHTUP),
							0,
							0,
							0,
							uintptr(EXTRA_FLAG_MAGIC),
						)
					} else {
						// 默认：模拟鼠标左键单击 (推进剧情/确认)
						procMouseEvent.Call(
							uintptr(MOUSEEVENTF_LEFTDOWN),
							0,
							0,
							0,
							uintptr(EXTRA_FLAG_MAGIC),
						)
						time.Sleep(12 * time.Millisecond)
						procMouseEvent.Call(
							uintptr(MOUSEEVENTF_LEFTUP),
							0,
							0,
							0,
							uintptr(EXTRA_FLAG_MAGIC),
						)
					}
				}()

				// 拦截本次下滚消息，不向目标窗口传递 WM_MOUSEWHEEL
				return 1
			}
		}
	}

	r, _, _ := procCallNextHookEx.Call(mouseHookHandle, uintptr(nCode), wParam, lParam)
	return r
}

func installMouseHook() error {
	cb := syscall.NewCallback(mouseHookProc)
	hHook, _, err := procSetWindowsHookExW.Call(
		uintptr(WH_MOUSE_LL),
		cb,
		0,
		0,
	)
	if hHook == 0 {
		return err
	}
	mouseHookHandle = hHook
	return nil
}

func uninstallMouseHook() {
	if mouseHookHandle != 0 {
		procUnhookWindowsHookEx.Call(mouseHookHandle)
		mouseHookHandle = 0
	}
}
