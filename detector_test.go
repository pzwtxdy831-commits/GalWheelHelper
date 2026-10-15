package main

import (
	"fmt"
	"testing"
)

func TestDetection(t *testing.T) {
	testCases := []struct {
		info     ForegroundInfo
		expected bool
		desc     string
	}{
		{
			info:     ForegroundInfo{ProcessName: "chrome.exe", WindowTitle: "Google Chrome", ClassName: "Chrome_WidgetWin_1"},
			expected: false,
			desc:     "浏览器应被黑名单拦截",
		},
		{
			info:     ForegroundInfo{ProcessName: "explorer.exe", WindowTitle: "", ClassName: "Progman"},
			expected: false,
			desc:     "Windows桌面不应触发",
		},
		{
			info:     ForegroundInfo{ProcessName: "code.exe", WindowTitle: "Visual Studio Code", ClassName: "Chrome_WidgetWin_1"},
			expected: false,
			desc:     "VSCode不应触发",
		},
		{
			info:     ForegroundInfo{ProcessName: "tvpwin32.exe", WindowTitle: "Fate/stay night", ClassName: "TVPMainWindow"},
			expected: true,
			desc:     "吉里吉里Kirikiri游戏应被识别",
		},
		{
			info:     ForegroundInfo{ProcessName: "Game_CHS.exe", WindowTitle: "游戏标题", ClassName: "Direct3DWindowClass"},
			expected: true,
			desc:     "CHS汉化后缀应被识别",
		},
		{
			info:     ForegroundInfo{ProcessName: "SiglusEngine.exe", WindowTitle: "Summer Pockets", ClassName: "SiglusWindowClass"},
			expected: true,
			desc:     "Siglus引擎应被识别",
		},
		{
			info:     ForegroundInfo{ProcessName: "game.exe", ProcessPath: "D:\\GalGame合集\\WHITE ALBUM2\\game.exe", WindowTitle: "WA2"},
			expected: true,
			desc:     "路径包含GalGame应被识别",
		},
		{
			info:     ForegroundInfo{ProcessName: "Magpie.exe", WindowTitle: "Magpie", ClassName: "Magpie"},
			expected: true,
			desc:     "Magpie全屏超分时应被识别",
		},
	}

	for _, tc := range testCases {
		res := isGalgameWindow(tc.info)
		if res != tc.expected {
			t.Errorf("[%s] 期望 %v 但得到 %v", tc.desc, tc.expected, res)
		} else {
			fmt.Printf("✅ PASS: %s -> %v\n", tc.desc, res)
		}
	}
}
