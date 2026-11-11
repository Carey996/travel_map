package middleware

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/Carey996/travel_map/api/internal/apperr"
)

// RateCounter 是限流计数器的最小抽象。
//
// 抽出接口而不是直接吃 *redis.Client，有两个目的：
//   - 单测可以用 fake 覆盖限流分支，不需要真实 Redis；
//   - 计数后端可以替换（当前只有 Redis 实现）。
//
// Hit 对 key 计一次数，并返回：
//   - count：包含本次在内的窗口内计数；
//   - retryAfter：距窗口结束的剩余秒数（客户端最早可重试的时间）；
//   - err：计数不可用；调用方应 **fail-open** 而不是拒绝请求。
type RateCounter interface {
	Hit(ctx context.Context, key string, window time.Duration) (count int64, retryAfter int64, err error)
}

// redisRateCounter 用 Redis 做固定窗口计数。
//
// 为什么是 Redis 而不是进程内 map：后端硬约束是「无状态」——不存进程内会话、
// 不做进程内缓存。进程内计数在多副本下每个实例各算各的（限额被放大成 N 倍），
// 进程重启还会清零；放 Redis 才能跨实例一致，并且跟会话共用同一份基础设施。
type redisRateCounter struct {
	rdb *redis.Client
}

// NewRedisRateCounter 构造基于 Redis 的计数器；rdb 为 nil 时返回 nil（限流自动降级为放行）。
func NewRedisRateCounter(rdb *redis.Client) RateCounter {
	if rdb == nil {
		return nil
	}
	return &redisRateCounter{rdb: rdb}
}

// Hit 实现固定窗口计数：INCR 计数 + 只在 key 还没有 TTL 时 EXPIRE。
//
// 三个命令放进 TxPipeline（MULTI/EXEC）里一次往返、顺序执行：
//   - 只 INCR 不设 TTL，一旦写成功而后续步骤丢失，key 就永不过期，
//     一次抖动就能把某个 IP 永久挡在门外；
//   - 每次 Hit 都普通 EXPIRE，窗口会被不断延长，变成「最后一次请求之后才重开」，
//     持续访问的用户会被越锁越久；
//   - EXPIRE ... NX 只在 key 没有 TTL 时生效，正好是「首次命中设置窗口」的语义。
//
// 注意 EXPIRE NX 需要 Redis ≥ 7.0；仓库所有编排（README 快速开始、deploy/compose、
// deploy/k8s）统一是 redis:7-alpine。若对着更老的 Redis 跑，EXEC 会报错，
// 上层按 fail-open 放行并在日志里留下告警——限流失效但登录不会被拦死。
func (c *redisRateCounter) Hit(ctx context.Context, key string, window time.Duration) (int64, int64, error) {
	pipe := c.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, window)
	ttl := pipe.TTL(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, 0, err
	}
	return incr.Val(), retryAfterSeconds(ttl.Val(), window), nil
}

// retryAfterSeconds 把剩余 TTL 向上取整成秒。
//
// TTL 缺失（key 刚创建还没读到、或异常返回 -1/-2）时退化为完整窗口长度，
// 保证 Retry-After 永远是有意义的正数；亚秒级剩余也至少是 1 秒。
func retryAfterSeconds(ttl, window time.Duration) int64 {
	if ttl <= 0 {
		ttl = window
	}
	secs := int64(math.Ceil(ttl.Seconds()))
	if secs < 1 {
		secs = 1
	}
	return secs
}

// RateLimit 返回按「prefix + 客户端 IP」分桶的固定窗口限流中间件。
//
// 行为：
//   - 限内：放行（c.Next()）；
//   - 超限：**不调用后续 handler**，返回 429 + Retry-After 头，错误形状仍是
//     {"code","message","detail"}（code = rate_limited）；
//   - 计数器出错：fail-open（放行 + slog.Warn）。限流是附加防线，
//     不能因为 Redis 抖一下就让登录/注册整体不可用；
//   - counter 为 nil（未配置 Redis / 显式降级）：直接放行。
//
// key 里带 window 前缀是装配方的责任（见 handler/router.go 的常量）：
// 改窗口大小时旧计数自然落到旧命名空间，不会污染新窗口。
func RateLimit(counter RateCounter, prefix string, limit int64, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if counter == nil || limit <= 0 || window <= 0 || prefix == "" {
			c.Next()
			return
		}

		key := prefix + ":" + c.ClientIP()
		count, retryAfter, err := counter.Hit(c.Request.Context(), key, window)
		if err != nil {
			// fail-open：宁可暂时不限流，也不能让认证入口全线 500/429。
			slog.Warn("限流计数器不可用，本次请求放行（fail-open）",
				"key", key, "count", count, "err", err)
			c.Next()
			return
		}
		if count <= limit {
			c.Next()
			return
		}

		if retryAfter < 1 {
			retryAfter = 1
		}
		c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
		apiErr := apperr.New(http.StatusTooManyRequests, "rate_limited", "尝试过于频繁，请稍后再试").
			WithDetail(map[string]any{
				"retry_after_seconds": retryAfter,
				"limit":               limit,
				"window_seconds":      int64(math.Ceil(window.Seconds())),
			})
		c.AbortWithStatusJSON(apiErr.Status, apiErr)
	}
}
