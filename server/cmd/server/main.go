// 服务入口：加载配置、组装依赖并启动 HTTP 服务，支持优雅退出。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/jerion/wecom-auth-center/server/internal/audit"
	"github.com/jerion/wecom-auth-center/server/internal/buildinfo"
	"github.com/jerion/wecom-auth-center/server/internal/config"
	"github.com/jerion/wecom-auth-center/server/internal/handler"
	"github.com/jerion/wecom-auth-center/server/internal/metrics"
	"github.com/jerion/wecom-auth-center/server/internal/service"
	"github.com/jerion/wecom-auth-center/server/internal/store"
)

// versionText 组装 -v/--version 输出的版本信息文本
func versionText() string {
	return fmt.Sprintf("wecom-auth-center %s\ncommit: %s\nbuild: %s\ngo: %s\n", buildinfo.Version, buildinfo.Commit, buildinfo.BuildDate, runtime.Version())
}

// setUsage 自定义 -h/--help 输出：-v 与 -version 合并一行，各参数描述统一换行缩进对齐
func setUsage() {
	flag.Usage = func() {
		w := flag.CommandLine.Output()
		fmt.Fprintf(w, "Usage of %s:\n", os.Args[0])
		flag.VisitAll(func(f *flag.Flag) {
			if f.Name == "version" {
				return
			}
			if f.Name == "v" {
				fmt.Fprintf(w, "  -v, -version\n    \t%s\n", f.Usage)
				return
			}
			name, usage := flag.UnquoteUsage(f)
			fmt.Fprintf(w, "  -%s %s\n    \t%s (default %q)\n", f.Name, name, usage, f.DefValue)
		})
	}
}

func main() {
	showVersion := flag.Bool("v", false, "显示版本信息并退出")
	flag.BoolVar(showVersion, "version", false, "显示版本信息并退出")
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	setUsage()
	flag.Parse()

	if *showVersion {
		fmt.Print(versionText())
		os.Exit(0)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("加载配置失败", "error", err)
		os.Exit(1)
	}
	warnHTTPApps(cfg, logger)

	var wcom service.WeCom
	if cfg.Wecom.Mock {
		logger.Warn("启用 mock 模式：不访问企业微信真实接口，仅用于本地演练")
		wcom = service.MockClient{}
	} else {
		if cfg.Wecom.FetchProfile && cfg.Wecom.ContactSecret == "" {
			logger.Warn("已开启 fetch_profile 但未配置 contact_secret，邮箱等敏感字段可能获取不到（2022-06-20 后创建的自建应用受限）")
		}
		wcom = service.NewRealClient(service.ClientOptions{
			CorpID:         cfg.Wecom.CorpID,
			AgentID:        cfg.Wecom.AgentID,
			Secret:         cfg.Wecom.Secret,
			ContactSecret:  cfg.Wecom.ContactSecret,
			FetchName:      cfg.Wecom.FetchName,
			FetchProfile:   cfg.Wecom.FetchProfile,
			JobNumberField: cfg.Wecom.JobNumberExtattr,
		})
	}

	st := buildStore(cfg, logger)
	auditLog := openAudit(cfg, logger)
	defer auditLog.Close()

	met := metrics.New(time.Now)
	stopMetricsSave := startMetricsSaver(cfg, met, logger)
	defer stopMetricsSave()

	ssoSvc := service.NewSSO(st, cfg.TTL.State, cfg.TTL.Ticket)
	srv := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           handler.Router(handler.New(cfg, ssoSvc, wcom, logger, auditLog, met, st)),
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

// warnHTTPApps 对使用 HTTP 的业务系统域名提示票据明文传输风险
func warnHTTPApps(cfg *config.Config, logger *slog.Logger) {
	for name, app := range cfg.Apps {
		if strings.HasPrefix(app.Domain, "http://") {
			logger.Warn("业务系统域名使用 HTTP，ticket 将明文传输，请确认内网可信", "app", name, "domain", app.Domain)
		}
	}
}

// buildStore 按配置选择存储驱动，Redis 模式启动时先验证连通性。
func buildStore(cfg *config.Config, logger *slog.Logger) store.Store {
	if cfg.Store.Driver != "redis" {
		return store.NewMemory(time.Now)
	}
	rs := store.NewRedis(cfg.Store.Redis.Addr, cfg.Store.Redis.Password, cfg.Store.Redis.DB, logger, time.Now)
	if err := rs.Ping(context.Background()); err != nil {
		logger.Error("Redis 连接失败", "addr", cfg.Store.Redis.Addr, "error", err)
		os.Exit(1)
	}
	logger.Info("已启用 Redis 存储", "addr", cfg.Store.Redis.Addr, "db", cfg.Store.Redis.DB)
	return rs
}

// openAudit 按配置初始化审计日志，失败时拒绝启动避免安全事件失录。
func openAudit(cfg *config.Config, logger *slog.Logger) *audit.Logger {
	if !cfg.Audit.Enabled {
		return nil
	}
	auditLog, err := audit.New(cfg.Audit.Path)
	if err != nil {
		logger.Error("打开审计日志文件失败", "path", cfg.Audit.Path, "error", err)
		os.Exit(1)
	}
	logger.Info("已启用审计日志", "path", cfg.Audit.Path)
	return auditLog
}

// startMetricsSaver 启用监控时加载历史计数并周期落盘，返回停机时执行的收尾函数
func startMetricsSaver(cfg *config.Config, met *metrics.Metrics, logger *slog.Logger) func() {
	if !cfg.Status.Enabled {
		return func() {}
	}
	if err := met.Load(cfg.Status.DataPath); err != nil {
		logger.Warn("监控统计加载失败，从零开始累计", "path", cfg.Status.DataPath, "error", err)
	}
	logger.Info("已启用监控页", "path", "/status", "data_path", cfg.Status.DataPath)
	stop := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := met.Save(cfg.Status.DataPath); err != nil {
					logger.Warn("监控统计落盘失败", "path", cfg.Status.DataPath, "error", err)
				}
			case <-stop:
				return
			}
		}
	}()
	return func() {
		close(stop)
		<-stopped
		if err := met.Save(cfg.Status.DataPath); err != nil {
			logger.Warn("监控统计落盘失败", "path", cfg.Status.DataPath, "error", err)
		}
	}
}
