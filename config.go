package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

type Config struct {
	TargetAction            string   `json:"target_action"`              // "left" (左键，推进剧情/点击), "right" (右键)
	Mode                    string   `json:"mode"`                       // "auto", "always", "disabled"
	DebounceMs              int      `json:"debounce_ms"`                // 防抖冷却时间 (毫秒)，默认 180
	Hotkey                  string   `json:"hotkey"`                     // 切换快捷键: "ScrollLock", "F11", "Ctrl+Alt+G"
	SoundFeedback           bool     `json:"sound_feedback"`             // 切换状态时是否有提示音
	AutoStart               bool     `json:"auto_start"`                 // 是否开机自启动
	CustomWhitelistProcesses []string `json:"custom_whitelist_processes"` // 用户自定义进程白名单 (小写)
	CustomWhitelistTitles    []string `json:"custom_whitelist_titles"`    // 用户自定义标题关键字 (小写)
}

var (
	cfgLock sync.RWMutex
	currentConfig = Config{
		TargetAction: "left",
		Mode:         "auto",
		DebounceMs:   180,
		Hotkey:       "ScrollLock",
		SoundFeedback: true,
		AutoStart:    false,
		CustomWhitelistProcesses: []string{},
		CustomWhitelistTitles:    []string{},
	}
	configFilePath string
)

func initConfigPath() {
	exePath, err := os.Executable()
	if err != nil {
		configFilePath = "config.json"
	} else {
		configFilePath = filepath.Join(filepath.Dir(exePath), "config.json")
	}
}

func loadConfig() {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	initConfigPath()
	data, err := os.ReadFile(configFilePath)
	if err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err == nil {
			if cfg.TargetAction != "right" {
				cfg.TargetAction = "left"
			}
			if cfg.Mode == "" {
				cfg.Mode = "auto"
			}
			if cfg.DebounceMs <= 0 {
				cfg.DebounceMs = 180
			}
			if cfg.Hotkey == "" {
				cfg.Hotkey = "ScrollLock"
			}
			// 转换为小写规范化
			for i, p := range cfg.CustomWhitelistProcesses {
				cfg.CustomWhitelistProcesses[i] = strings.ToLower(strings.TrimSpace(p))
			}
			for i, t := range cfg.CustomWhitelistTitles {
				cfg.CustomWhitelistTitles[i] = strings.ToLower(strings.TrimSpace(t))
			}
			currentConfig = cfg
			return
		}
	}

	// 写入默认配置
	saveConfigLocked()
}

func saveConfig() {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	saveConfigLocked()
}

func saveConfigLocked() {
	data, err := json.MarshalIndent(currentConfig, "", "  ")
	if err == nil {
		_ = os.WriteFile(configFilePath, data, 0644)
	}
}

func getConfig() Config {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return currentConfig
}

func setMode(m string) {
	cfgLock.Lock()
	currentConfig.Mode = m
	saveConfigLocked()
	cfgLock.Unlock()
}

func addProcessToWhitelist(procName string) bool {
	procName = strings.ToLower(strings.TrimSpace(procName))
	if procName == "" {
		return false
	}
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for _, p := range currentConfig.CustomWhitelistProcesses {
		if p == procName {
			return false
		}
	}
	currentConfig.CustomWhitelistProcesses = append(currentConfig.CustomWhitelistProcesses, procName)
	saveConfigLocked()
	return true
}

// 注册表设置开机自启动 (HKCU\Software\Microsoft\Windows\CurrentVersion\Run)
func setAutoStart(enable bool) error {
	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	regOpenKeyEx := advapi32.NewProc("RegOpenKeyExW")
	regSetValueEx := advapi32.NewProc("RegSetValueExW")
	regDeleteValue := advapi32.NewProc("RegDeleteValueW")
	regCloseKey := advapi32.NewProc("RegCloseKey")

	const (
		HKEY_CURRENT_USER = 0x80000001
		KEY_SET_VALUE     = 0x0002
		REG_SZ            = 1
	)

	subKey, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Run`)
	valueName, _ := syscall.UTF16PtrFromString(`GalWheelHelper`)

	var hKey uintptr
	r, _, _ := regOpenKeyEx.Call(
		uintptr(HKEY_CURRENT_USER),
		uintptr(unsafe.Pointer(subKey)),
		0,
		uintptr(KEY_SET_VALUE),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if r != 0 {
		return syscall.Errno(r)
	}
	defer regCloseKey.Call(hKey)

	if enable {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		pathU16, _ := syscall.UTF16FromString(exePath)
		r, _, _ = regSetValueEx.Call(
			hKey,
			uintptr(unsafe.Pointer(valueName)),
			0,
			uintptr(REG_SZ),
			uintptr(unsafe.Pointer(&pathU16[0])),
			uintptr(len(pathU16)*2),
		)
		if r != 0 {
			return syscall.Errno(r)
		}
	} else {
		regDeleteValue.Call(hKey, uintptr(unsafe.Pointer(valueName)))
	}

	cfgLock.Lock()
	currentConfig.AutoStart = enable
	saveConfigLocked()
	cfgLock.Unlock()
	return nil
}

func isAutoStartEnabled() bool {
	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	regOpenKeyEx := advapi32.NewProc("RegOpenKeyExW")
	regQueryValueEx := advapi32.NewProc("RegQueryValueExW")
	regCloseKey := advapi32.NewProc("RegCloseKey")

	const (
		HKEY_CURRENT_USER = 0x80000001
		KEY_QUERY_VALUE   = 0x0001
	)

	subKey, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Run`)
	valueName, _ := syscall.UTF16PtrFromString(`GalWheelHelper`)

	var hKey uintptr
	r, _, _ := regOpenKeyEx.Call(
		uintptr(HKEY_CURRENT_USER),
		uintptr(unsafe.Pointer(subKey)),
		0,
		uintptr(KEY_QUERY_VALUE),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if r != 0 {
		return false
	}
	defer regCloseKey.Call(hKey)

	r, _, _ = regQueryValueEx.Call(
		hKey,
		uintptr(unsafe.Pointer(valueName)),
		0,
		0,
		0,
		0,
	)
	return r == 0
}
