// Package db 负责数据库连接与结构迁移。
package db

import (
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

// Migrate 确保所有表与索引存在。
//
// GORM 的 AutoMigrate 只增不删：它会补齐缺失的表、列、索引，
// 不会删除已有结构，因此对已有库是安全的、可重复执行的。
func Migrate(gdb *gorm.DB) error {
	if err := gdb.AutoMigrate(model.AllModels()...); err != nil {
		return fmt.Errorf("结构迁移失败: %w", err)
	}
	slog.Info("数据库结构已同步")
	return nil
}
