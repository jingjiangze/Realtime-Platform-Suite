// Command server 是通用框架的入口：装配 Provider 并启动 HTTP 服务。
//
// 本仓只提供接口与内存实现；生产实现（对象存储、自托管 IM、数据集客户端）
// 由使用方在自己的私有仓库里实现 chat.Provider / storage.Provider / dataset.Provider，
// 通过依赖注入装配，不需要修改本仓。
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jingjiangze/realtime-platform-foundation/internal/chat/memory"
	"github.com/jingjiangze/realtime-platform-foundation/internal/config"
	"github.com/jingjiangze/realtime-platform-foundation/internal/httpapi"
)

var version = "dev"

func main() {
	cfg := config.Load()

	app := httpapi.AppUpdate{
		Latest:       cfg.AppLatest,
		MinSupported: cfg.AppMinSupported,
		Platforms:    cfg.Platforms,
	}
	handler := httpapi.NewRouter(httpapi.Deps{
		Chat:    memory.New(),
		Version: cfg.Version,
		App:     app,
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on %s (version=%s chat=%s)", cfg.HTTPAddr, cfg.Version, cfg.ChatProvider)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
