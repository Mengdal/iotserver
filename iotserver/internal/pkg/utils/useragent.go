package utils

import "strings"

// GetBrowser 从 User-Agent 中简单识别浏览器
func GetBrowser(userAgent string) string {
	ua := strings.ToLower(userAgent)
	switch {
	case ua == "":
		return "Unknown"
	case strings.Contains(ua, "edg"):
		return "Edge"
	case strings.Contains(ua, "micromessenger"):
		return "WeChat"
	case strings.Contains(ua, "opr/") || strings.Contains(ua, "opera"):
		return "Opera"
	case strings.Contains(ua, "chrome"):
		return "Chrome"
	case strings.Contains(ua, "safari"):
		return "Safari"
	case strings.Contains(ua, "firefox"):
		return "Firefox"
	case strings.Contains(ua, "msie") || strings.Contains(ua, "trident"):
		return "IE"
	case strings.Contains(ua, "curl"):
		return "Curl"
	default:
		return "Unknown"
	}
}

// GetOs 从 User-Agent 中简单识别操作系统
func GetOs(userAgent string) string {
	ua := strings.ToLower(userAgent)
	switch {
	case ua == "":
		return "Unknown"
	case strings.Contains(ua, "windows"):
		return "Windows"
	case strings.Contains(ua, "android"):
		return "Android"
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") || strings.Contains(ua, "ios"):
		return "iOS"
	case strings.Contains(ua, "mac os") || strings.Contains(ua, "macintosh"):
		return "macOS"
	case strings.Contains(ua, "linux"):
		return "Linux"
	default:
		return "Unknown"
	}
}
