// Package session 提供会话存储，同一接口两种后端。
//
// 会话 id 是**不透明随机串**而非 JWT：服务端只保存它（Redis 直接以它为键，
// Postgres 存 sha256 摘要），因此存储泄露也无法直接冒用会话。
// 两种实现都支持即时撤销——这是自包含 JWT 做不到的。
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/Carey996/travel_map/api/internal/model"
)

// Store 是会话存储的统一接口。
type Store interface {
	Create(ctx context.Context, userID uuid.UUID, userAgent, ip string) (string, error)
	Resolve(ctx context.Context, sid string) (uuid.UUID, bool, error)
	Revoke(ctx context.Context, sid string) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}

// NewToken 生成 256 位熵的不透明会话 id。
func NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成会话令牌失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken 返回令牌的 sha256 摘要（Postgres 后端只存摘要）。
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// Redis 实现（默认）
// ---------------------------------------------------------------------------

// RedisStore 把会话放在 Redis，靠 TTL 过期；多实例天然共享。
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
	prefix string
}

// NewRedisStore 构造 Redis 会话存储。
func NewRedisStore(client *redis.Client, ttl time.Duration) *RedisStore {
	return &RedisStore{client: client, ttl: ttl, prefix: "tb:sess:"}
}

func (s *RedisStore) key(sid string) string { return s.prefix + sid }

// userKey 是反向索引，用于「退出所有设备」。
func (s *RedisStore) userKey(userID uuid.UUID) string {
	return "tb:user:" + userID.String() + ":sess"
}

// Create 写入会话并返回会话 id。
func (s *RedisStore) Create(ctx context.Context, userID uuid.UUID, _, _ string) (string, error) {
	sid, err := NewToken()
	if err != nil {
		return "", err
	}
	pipe := s.client.Pipeline()
	pipe.Set(ctx, s.key(sid), userID.String(), s.ttl)
	pipe.SAdd(ctx, s.userKey(userID), sid)
	pipe.Expire(ctx, s.userKey(userID), s.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("写入会话失败: %w", err)
	}
	return sid, nil
}

// Resolve 由会话 id 反查用户。
func (s *RedisStore) Resolve(ctx context.Context, sid string) (uuid.UUID, bool, error) {
	raw, err := s.client.Get(ctx, s.key(sid)).Result()
	if errors.Is(err, redis.Nil) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("读取会话失败: %w", err)
	}
	id, parseErr := uuid.Parse(raw)
	if parseErr != nil {
		return uuid.Nil, false, nil
	}
	return id, true, nil
}

// Revoke 撤销单个会话（登出当前设备）。
func (s *RedisStore) Revoke(ctx context.Context, sid string) error {
	raw, err := s.client.Get(ctx, s.key(sid)).Result()
	pipe := s.client.Pipeline()
	pipe.Del(ctx, s.key(sid))
	if err == nil && raw != "" {
		pipe.SRem(ctx, s.userKey(uuid.MustParse(raw)), sid)
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("撤销会话失败: %w", err)
	}
	return nil
}

// RevokeAllForUser 撤销该用户的全部会话（登出所有设备）。
func (s *RedisStore) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	sids, err := s.client.SMembers(ctx, s.userKey(userID)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("读取会话索引失败: %w", err)
	}
	keys := make([]string, 0, len(sids))
	for _, sid := range sids {
		keys = append(keys, s.key(sid))
	}
	if len(keys) > 0 {
		if err := s.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("批量撤销会话失败: %w", err)
		}
	}
	if err := s.client.Del(ctx, s.userKey(userID)).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("清理会话索引失败: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Postgres 实现（SESSION_BACKEND=db）
// ---------------------------------------------------------------------------

// DBStore 把会话落进 Postgres，适合零额外组件起步；代价是每个请求多一次查询。
type DBStore struct {
	gdb *gorm.DB
	ttl time.Duration
}

// NewDBStore 构造 Postgres 会话存储。
func NewDBStore(gdb *gorm.DB, ttl time.Duration) *DBStore {
	return &DBStore{gdb: gdb, ttl: ttl}
}

// Create 写入会话记录。
func (s *DBStore) Create(ctx context.Context, userID uuid.UUID, userAgent, ip string) (string, error) {
	sid, err := NewToken()
	if err != nil {
		return "", err
	}
	if len(userAgent) > 400 {
		userAgent = userAgent[:400]
	}
	if len(ip) > 64 {
		ip = ip[:64]
	}
	row := model.UserSession{
		UserID:    userID,
		TokenHash: HashToken(sid),
		UserAgent: userAgent,
		IP:        ip,
		ExpiresAt: time.Now().UTC().Add(s.ttl),
	}
	if err := s.gdb.WithContext(ctx).Create(&row).Error; err != nil {
		return "", fmt.Errorf("写入会话失败: %w", err)
	}
	return sid, nil
}

// Resolve 查询未撤销且未过期的会话。
func (s *DBStore) Resolve(ctx context.Context, sid string) (uuid.UUID, bool, error) {
	var row model.UserSession
	err := s.gdb.WithContext(ctx).
		Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", HashToken(sid), time.Now().UTC()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("查询会话失败: %w", err)
	}
	return row.UserID, true, nil
}

// Revoke 标记单个会话已撤销。
func (s *DBStore) Revoke(ctx context.Context, sid string) error {
	now := time.Now().UTC()
	err := s.gdb.WithContext(ctx).Model(&model.UserSession{}).
		Where("token_hash = ?", HashToken(sid)).
		Update("revoked_at", now).Error
	if err != nil {
		return fmt.Errorf("撤销会话失败: %w", err)
	}
	return nil
}

// RevokeAllForUser 撤销该用户全部有效会话。
func (s *DBStore) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	now := time.Now().UTC()
	err := s.gdb.WithContext(ctx).Model(&model.UserSession{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
	if err != nil {
		return fmt.Errorf("批量撤销会话失败: %w", err)
	}
	return nil
}

// itoa 保留给调试日志使用，避免额外引入 strconv 到调用方。
func itoa(n int) string { return strconv.Itoa(n) }

var _ = itoa
