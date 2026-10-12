// Package db 负责数据库连接与结构迁移。
package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Carey996/travel_map/api/internal/model"
)

// Connect 建立 GORM 连接。
//
// 连接池属于进程级基础设施，不是业务状态——服务本身仍然是无状态的。
func Connect(databaseURL string, isDev bool) (*gorm.DB, error) {
	level := gormlogger.Warn
	if isDev {
		level = gormlogger.Error // 开发时也不打每条 SQL，太吵
	}

	gdb, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		// 时间统一用 UTC 存储，避免容器时区差异带来的隐性 bug
		NowFunc: func() time.Time { return time.Now().UTC() },
		Logger:  gormlogger.Default.LogMode(level),
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层连接池失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("数据库不可达: %w", err)
	}
	return gdb, nil
}

// migrateLockKey 是 Postgres 咨询锁的键，取值任意但必须全局一致。
const migrateLockKey int64 = 0x74726176656c6267

// Migrate 确保所有表与索引存在。
//
// GORM 的 AutoMigrate 只增不删：它会补齐缺失的表、列、索引，
// 不会删除已有结构，因此对已有库是安全的、可重复执行的。
//
// 但它**不是并发安全的**——多个进程同时执行 DDL 会在 CREATE TABLE 上互相冲突。
// Kubernetes 里多副本同时启动恰好会触发这个问题，所以这里用 Postgres 的会话级咨询锁
// 把迁移串行化：无论多少个实例同时进来，实际只有一个在做 DDL，其余排队等待。
func Migrate(gdb *gorm.DB) error {
	ctx := context.Background()

	sqlDB, err := gdb.DB()
	if err != nil {
		return fmt.Errorf("获取底层连接池失败: %w", err)
	}

	// 锁必须由同一条连接持有到迁移结束
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("获取迁移连接失败: %w", err)
	}
	defer func() { _ = conn.Close() }()

	locked := false
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrateLockKey); err != nil {
		// 非 Postgres（例如测试用的 SQLite）没有咨询锁，如实告警后继续
		slog.Warn("未能获取迁移锁，继续执行（非 Postgres？）", "err", err)
	} else {
		locked = true
		slog.Info("已获得迁移锁")
	}
	if locked {
		defer func() {
			if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrateLockKey); err != nil {
				slog.Warn("释放迁移锁失败", "err", err)
			}
		}()
	}

	if err := gdb.AutoMigrate(model.AllModels()...); err != nil {
		return fmt.Errorf("结构迁移失败: %w", err)
	}
	slog.Info("数据库结构已同步")
	return nil
}
