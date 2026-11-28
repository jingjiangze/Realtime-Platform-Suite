// Package config 从环境变量读取运行配置。
package config

import (
	"os"
	"strings"
)

// Config 是进程级配置；全部可由环境变量覆盖。
type Config struct {
	HTTPAddr string
	Version  string

	// ChatProvider 预留：memory 为本仓内置实现，其他值由使用方装配。
	ChatProvider string

	// 客户端版本通道（L1）：最新整包版本与最低支持版本。
	AppLatest       string
	AppMinSupported string

	// Platforms 是按平台覆盖的发布信息（平台键为小写：android / ios / web）。
	Platforms map[string]Release
}

// Release 是单个客户端的发布信息（完整补丁契约见 docs/decisions/0004-hot-patch.md）。
type Release struct {
	Latest       string `json:"latest"`
	MinSupported string `json:"minSupported"`
}

// knownPlatforms 是支持独立版本配置的平台。
var knownPlatforms = []string{"android", "ios", "web"}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// Load 读取全部配置。
func Load() Config {
	cfg := Config{
		HTTPAddr:        getenv("HTTP_ADDR", ":8080"),
		Version:         getenv("APP_VERSION", "dev"),
		ChatProvider:    getenv("CHAT_PROVIDER", "memory"),
		AppLatest:       getenv("APP_LATEST", "0.1.0"),
		AppMinSupported: getenv("APP_MIN_SUPPORTED", "0.1.0"),
	}
	cfg.Platforms = make(map[string]Release, len(knownPlatforms))
	for _, p := range knownPlatforms {
		up := strings.ToUpper(p)
		cfg.Platforms[p] = Release{
			Latest:       getenv("APP_"+up+"_LATEST", cfg.AppLatest),
			MinSupported: getenv("APP_"+up+"_MIN_SUPPORTED", cfg.AppMinSupported),
		}
	}
	return cfg
}
