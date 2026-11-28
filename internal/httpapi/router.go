// Package httpapi 提供最小 HTTP 面：健康检查、演示聊天端点与客户端版本通道。
//
// 演示端点只用于验证 Provider 契约，不含认证、限流与业务规则；
// 生产实现应在自己的私有仓库里补充这些能力。
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jingjiangze/realtime-platform-foundation/internal/chat"
	"github.com/jingjiangze/realtime-platform-foundation/internal/config"
)

// AppUpdate 是客户端版本通道配置。
type AppUpdate struct {
	Latest       string
	MinSupported string
	Platforms    map[string]config.Release
}

// Deps 是 HTTP 层依赖。
type Deps struct {
	Chat    chat.Provider
	Version string
	App     AppUpdate
}

// NewRouter 组装路由（Go 1.22+ 方法路由）。
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": d.Version})
	})

	mux.HandleFunc("POST /v1/demo/login", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			UserID     string `json:"userId"`
			Credential string `json:"credential"`
		}
		if !decode(w, r, &body) {
			return
		}
		sess, err := d.Chat.Login(r.Context(), body.UserID, body.Credential)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, err)
			return
		}
		writeJSON(w, http.StatusOK, sess)
	})

	mux.HandleFunc("POST /v1/demo/chat/send", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ConvID      string `json:"convId"`
			SenderID    string `json:"senderId"`
			Text        string `json:"text"`
			ClientMsgID string `json:"clientMsgId"`
		}
		if !decode(w, r, &body) {
			return
		}
		msg, err := d.Chat.SendText(r.Context(), body.ConvID, body.SenderID, body.Text, body.ClientMsgID)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, msg)
	})

	mux.HandleFunc("GET /v1/demo/chat/history", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit := 50
		if v := q.Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}
		page, err := d.Chat.GetHistory(r.Context(), q.Get("userId"), q.Get("convId"), q.Get("cursor"), limit)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, page)
	})

	mux.HandleFunc("POST /v1/demo/chat/read", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ConvID string `json:"convId"`
			UserID string `json:"userId"`
			MsgID  string `json:"msgId"`
		}
		if !decode(w, r, &body) {
			return
		}
		if err := d.Chat.MarkRead(r.Context(), body.ConvID, body.UserID, body.MsgID); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	mux.HandleFunc("GET /v1/demo/chat/unread", func(w http.ResponseWriter, r *http.Request) {
		n, err := d.Chat.UnreadTotal(r.Context(), r.URL.Query().Get("userId"))
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"total": n})
	})

	mux.HandleFunc("POST /v1/demo/chat/presence", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			UserIDs []string `json:"userIds"`
		}
		if !decode(w, r, &body) {
			return
		}
		online, err := d.Chat.Presence(r.Context(), body.UserIDs)
		if errors.Is(err, chat.ErrNotSupported) {
			writeErr(w, http.StatusNotImplemented, err)
			return
		}
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, online)
	})

	mux.HandleFunc("GET /v1/app/version", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		platform := strings.ToLower(q.Get("platform"))
		if platform == "" {
			platform = "android"
		}
		rel := d.release(platform)
		current := q.Get("appVersion")
		resp := map[string]any{
			"platform":     platform,
			"latest":       rel.Latest,
			"minSupported": rel.MinSupported,
			"update":       compareVer(current, rel.Latest) < 0,
			"force":        current != "" && compareVer(current, rel.MinSupported) < 0,
		}
		writeJSON(w, http.StatusOK, resp)
	})

	return withRecover(withCORS(mux))
}

// release 返回指定平台的发布信息；未配置的平台回落到通用值。
func (d Deps) release(platform string) config.Release {
	if rel, ok := d.App.Platforms[platform]; ok && rel.Latest != "" {
		return rel
	}
	return config.Release{Latest: d.App.Latest, MinSupported: d.App.MinSupported}
}

// compareVer 比较两段 x.y.z 版本号：a<b 返回 -1，相等返回 0，a>b 返回 1。
func compareVer(a, b string) int {
	pa, pb := parseVer(a), parseVer(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

// parseVer 解析版本号，非数字段按 0 处理（忽略 -rc1 / +build 之类后缀）。
func parseVer(v string) [3]int {
	var out [3]int
	parts := strings.Split(strings.TrimSpace(v), ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		n := 0
		for _, c := range parts[i] {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
		}
		out[i] = n
	}
	return out
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "content-type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
