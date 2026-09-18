// 服务入口：加载配置、组装依赖并启动 HTTP 服务，支持优雅退出。
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jerion/wecom-auth-center/server/internal/config"
	"github.com/jerion/wecom-auth-center/server/internal/handler"
	"github.com/jerion/wecom-auth-center/server/internal/service"
	"github.com/jerion/wecom-auth-center/server/internal/store"
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("加载配置失败", "error", err)
		os.Exit(1)
	}

	var wcom service.WeCom
	if cfg.Wecom.Mock {
		logger.Warn("启用 mock 模式：不访问企业微信真实接口，仅用于本地演练")
		wcom = service.MockClient{}
	} else {
		wcom = service.NewRealClient(cfg.Wecom.CorpID, cfg.Wecom.AgentID, cfg.Wecom.Secret, cfg.Wecom.FetchName)
	}

	st := store.NewMemory(time.Now)
	ssoSvc := service.NewSSO(st, cfg.TTL.State, cfg.TTL.Ticket)
	srv := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           handler.Router(handler.New(cfg, ssoSvc, wcom, logger)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("认证中心已启动", "listen", cfg.Server.Listen, "external_url", cfg.Server.ExternalURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("服务异常退出", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("收到退出信号，正在优雅关闭")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("优雅关闭失败", "error", err)
	}
}
