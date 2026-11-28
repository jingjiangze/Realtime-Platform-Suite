package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jingjiangze/realtime-platform-foundation/internal/chat/memory"
	"github.com/jingjiangze/realtime-platform-foundation/internal/config"
)

func TestCompareVer(t *testing.T) {
	cases := []struct {
		a    string
		b    string
		want int
	}{
		{"0.1.0", "0.1.1", -1},
		{"1.2.3", "1.2.3", 0},
		{"1.10.0", "1.9.9", 1},
		{"0.1.0-rc1", "0.1.0", 0},
		{"", "0.1.0", -1},
	}
	for _, c := range cases {
		if got := compareVer(c.a, c.b); got != c.want {
			t.Fatalf("compareVer(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestVersionEndpoint_PlatformIsolation(t *testing.T) {
	h := NewRouter(Deps{
		Chat:    memory.New(),
		Version: "test",
		App: AppUpdate{
			Latest:       "0.2.0",
			MinSupported: "0.1.0",
			Platforms:    map[string]config.Release{"ios": {Latest: "1.0.0", MinSupported: "1.0.0"}},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/app/version?platform=ios&appVersion=0.9.0", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`"latest":"1.0.0"`, `"force":true`, `"platform":"ios"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %s missing %s", body, want)
		}
	}
}

func TestHealthz(t *testing.T) {
	h := NewRouter(Deps{Chat: memory.New(), Version: "test"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("healthz = %d %s", rec.Code, rec.Body.String())
	}
}
