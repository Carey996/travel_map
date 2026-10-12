// Command server 是旅行地图博客的后端入口。
//
// 本服务刻意保持无状态：进程内不保存会话、不写本地磁盘、不做内存缓存。
// 所有持久化都落在外部共享存储（Postgres / Redis / S3），
// 因此任意实例都能处理任意请求，可以随时重启与水平扩展。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Carey996/travel_map/api/internal/config"
	"github.com/Carey996/travel_map/api/internal/db"
	"github.com/Carey996/travel_map/api/internal/geocode"
	"github.com/Carey996/travel_map/api/internal/handler"
	"github.com/Carey996/travel_map/api/internal/session"
	"github.com/Carey996/travel_map/api/internal/storage"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := run(); err != nil {
		slog.Error("服务启动失败", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ---- 数据库 ----
	gdb, err := db.Connect(cfg.DatabaseURL, cfg.IsDev())
	if err != nil {
		return err
	}
	if err := db.Migrate(gdb); err != nil {
		return err
	}

	// ---- Redis ----
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return err
	}
	rdb := redis.NewClient(redisOpts)
	defer func() { _ = rdb.Close() }()
	if pingErr := rdb.Ping(ctx).Err(); pingErr != nil {
		return pingErr
	}

	// ---- 对象存储 ----
	store, err := storage.New(cfg)
	if err != nil {
		return err
	}
	if err := store.EnsureBucket(ctx); err != nil {
		// 对象存储暂时不可用不应阻止启动：就绪探针会如实报告
		slog.Warn("对象存储暂不可用，跳过后台校验", "err", err)
	}

	// ---- 会话存储 ----
	var sessions session.Store
	if cfg.SessionBackend == "db" {
		sessions = session.NewDBStore(gdb, cfg.SessionTTL)
		slog.Info("会话存储使用 Postgres")
	} else {
		sessions = session.NewRedisStore(rdb, cfg.SessionTTL)
		slog.Info("会话存储使用 Redis")
	}

	// ---- HTTP ----
	h := handler.New(cfg, gdb, rdb, sessions, store, geocode.New(cfg))
	srv := &http.Server{
		Addr:              "127.0.0.1:" + strconv.Itoa(cfg.APIPort),
		Handler:           h.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("服务已启动", "app", cfg.AppName, "version", cfg.AppVersion,
			"env", cfg.Environment, "addr", srv.Addr)
		if serveErr := srv.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	}()

	select {
	case serveErr := <-errCh:
		return serveErr
	case <-ctx.Done():
		slog.Info("收到退出信号，开始优雅关闭")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	slog.Info("已关闭")
	return nil
}
