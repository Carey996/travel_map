// Package storage 封装 S3 兼容对象存储。
//
// 关键设计：后端**不接收图片字节流**。这里只签发预签名 URL，
// 浏览器拿到后直接 PUT 到对象存储。因此 API 进程既不需要本地磁盘，
// 也不会被大文件拖垮内存——这是「无状态」在文件上传场景下的具体落法。
package storage

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/Carey996/travel_map/api/internal/config"
)

// Storage 持有两个客户端：一个走内部地址做管理操作，
// 一个走浏览器可达地址用于签发——签名是纯本地计算，不发网络请求。
type Storage struct {
	admin   *minio.Client
	presign *minio.Client
	bucket  string
	expires time.Duration
}

// New 构造对象存储客户端。
func New(cfg *config.Config) (*Storage, error) {
	admin, err := newClient(cfg.S3Endpoint, cfg)
	if err != nil {
		return nil, fmt.Errorf("构造对象存储客户端失败: %w", err)
	}

	presign := admin
	if cfg.BrowserEndpoint() != cfg.S3Endpoint {
		if presign, err = newClient(cfg.BrowserEndpoint(), cfg); err != nil {
			return nil, fmt.Errorf("构造浏览器侧对象存储客户端失败: %w", err)
		}
	}

	return &Storage{
		admin:   admin,
		presign: presign,
		bucket:  cfg.S3Bucket,
		expires: cfg.S3PresignExpires,
	}, nil
}

func newClient(endpoint string, cfg *config.Config) (*minio.Client, error) {
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: cfg.S3UseSSL,
		Region: cfg.S3Region,
	})
}

// EnsureBucket 幂等地确保 bucket 存在，供启动时调用。
func (s *Storage) EnsureBucket(ctx context.Context) error {
	exists, err := s.admin.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("检查 bucket 失败: %w", err)
	}
	if exists {
		slog.Info("对象存储 bucket 已存在", "bucket", s.bucket)
		return nil
	}
	if err := s.admin.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		// 并发启动时可能被其他实例抢先创建
		if existsNow, checkErr := s.admin.BucketExists(ctx, s.bucket); checkErr == nil && existsNow {
			return nil
		}
		return fmt.Errorf("创建 bucket 失败: %w", err)
	}
	slog.Info("已创建对象存储 bucket", "bucket", s.bucket)
	return nil
}

// PresignPut 签发上传用的预签名 URL。
func (s *Storage) PresignPut(ctx context.Context, key string) (string, error) {
	u, err := s.presign.PresignedPutObject(ctx, s.bucket, key, s.expires)
	if err != nil {
		return "", fmt.Errorf("签发上传 URL 失败: %w", err)
	}
	return u.String(), nil
}

// PresignGet 签发读取用的预签名 URL。
//
// 每个请求现签即可：签名是纯本地计算，既不必在库里存易过期的 URL，也不需要任何缓存。
func (s *Storage) PresignGet(ctx context.Context, key string) (string, error) {
	u, err := s.presign.PresignedGetObject(ctx, s.bucket, key, s.expires, nil)
	if err != nil {
		return "", fmt.Errorf("签发读取 URL 失败: %w", err)
	}
	return u.String(), nil
}

// Delete 批量删除对象；不存在的键会被忽略。
func (s *Storage) Delete(ctx context.Context, keys []string) error {
	objects := make([]minio.ObjectInfo, 0, len(keys))
	for _, k := range keys {
		if k != "" {
			objects = append(objects, minio.ObjectInfo{Key: k})
		}
	}
	if len(objects) == 0 {
		return nil
	}
	errs := s.admin.RemoveObjects(ctx, s.bucket, objectsCh(objects), minio.RemoveObjectsOptions{})
	var firstErr error
	for e := range errs {
		if e.Err != nil && firstErr == nil {
			firstErr = e.Err
		}
	}
	if firstErr != nil {
		return fmt.Errorf("删除对象失败: %w", firstErr)
	}
	return nil
}

func objectsCh(objects []minio.ObjectInfo) <-chan minio.ObjectInfo {
	ch := make(chan minio.ObjectInfo, len(objects))
	go func() {
		defer close(ch)
		for _, o := range objects {
			ch <- o
		}
	}()
	return ch
}
