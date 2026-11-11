package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// 测试统一用这套参数，跟 handler/router.go 的登录限流常量保持一致。
const (
	testRatePrefix = "tb:ratelimit:auth:login:5m"
	testRateLimit  = int64(10)
	testRateWindow = 5 * time.Minute
)

func init() { gin.SetMode(gin.TestMode) }

// fakeCounter 是 RateCounter 的假实现：记录被调用的 key/window，返回预设结果。
type fakeCounter struct {
	keys       []string
	windows    []time.Duration
	count      int64
	retryAfter int64
	err        error
}

func (f *fakeCounter) Hit(_ context.Context, key string, window time.Duration) (int64, int64, error) {
	f.keys = append(f.keys, key)
	f.windows = append(f.windows, window)
	return f.count, f.retryAfter, f.err
}

// newRateLimitTestRouter 复刻生产装配：限流挂成路由级中间件，handler 只负责标记被调用。
func newRateLimitTestRouter(counter RateCounter, limit int64, window time.Duration) (*gin.Engine, *bool) {
	called := new(bool)
	r := gin.New()
	r.POST("/api/v1/auth/login",
		RateLimit(counter, testRatePrefix, limit, window),
		func(c *gin.Context) {
			*called = true
			c.Status(http.StatusOK)
		})
	return r, called
}

func postLogin(t *testing.T, r http.Handler, mutate ...func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.9:5678"
	for _, m := range mutate {
		m(req)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type rateLimitErrBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Detail  map[string]any `json:"detail"`
}

func decodeRateLimitErr(t *testing.T, body []byte) rateLimitErrBody {
	t.Helper()
	var env rateLimitErrBody
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("响应不是合法 JSON: %v, body=%s", err, body)
	}
	return env
}

func TestRateLimitAllowsWithinLimit(t *testing.T) {
	counter := &fakeCounter{count: testRateLimit}
	r, called := newRateLimitTestRouter(counter, testRateLimit, testRateWindow)

	w := postLogin(t, r)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200；body=%s", w.Code, w.Body.String())
	}
	if !*called {
		t.Fatal("限内请求应该进入 handler")
	}
	if got := w.Header().Get("Retry-After"); got != "" {
		t.Fatalf("限内请求不应带 Retry-After，实际 %q", got)
	}
	if len(counter.keys) != 1 {
		t.Fatalf("计数器 Hit 调用次数 = %d, want 1", len(counter.keys))
	}
}

func TestRateLimitBlocksOverLimit(t *testing.T) {
	counter := &fakeCounter{count: testRateLimit + 1, retryAfter: 240}
	r, called := newRateLimitTestRouter(counter, testRateLimit, testRateWindow)

	w := postLogin(t, r)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("状态码 = %d, want 429；body=%s", w.Code, w.Body.String())
	}
	if *called {
		t.Fatal("超限请求不能进入 handler")
	}
	if got := w.Header().Get("Retry-After"); got != "240" {
		t.Fatalf("Retry-After = %q, want %q", got, "240")
	}
	env := decodeRateLimitErr(t, w.Body.Bytes())
	if env.Code != "rate_limited" {
		t.Fatalf("code = %q, want rate_limited", env.Code)
	}
	if env.Message == "" {
		t.Fatal("message 不应为空")
	}
	wantDetail := map[string]float64{
		"retry_after_seconds": 240,
		"limit":               10,
		"window_seconds":      300,
	}
	for key, want := range wantDetail {
		got, ok := env.Detail[key].(float64)
		if !ok {
			t.Fatalf("detail[%q] 类型 = %T, want number", key, env.Detail[key])
		}
		if got != want {
			t.Fatalf("detail[%q] = %v, want %v", key, got, want)
		}
	}
}

// Redis 出错必须 fail-open：放行 + 记 warn，而不是把认证入口一起打挂。
func TestRateLimitFailOpenOnCounterError(t *testing.T) {
	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	counter := &fakeCounter{err: errors.New("redis: connection refused")}
	r, called := newRateLimitTestRouter(counter, testRateLimit, testRateWindow)

	w := postLogin(t, r)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200（fail-open）；body=%s", w.Code, w.Body.String())
	}
	if !*called {
		t.Fatal("计数器出错时应放行进入 handler")
	}
	if got := w.Header().Get("Retry-After"); got != "" {
		t.Fatalf("fail-open 不应带 Retry-After，实际 %q", got)
	}
	if !strings.Contains(logged.String(), "fail-open") {
		t.Fatalf("应记录 fail-open 告警日志，实际日志=%q", logged.String())
	}
}

func TestRateLimitNilCounterPassesThrough(t *testing.T) {
	// 零值 Handler 未装配 Redis 时会走到这里：显式降级为不限流。
	r, called := newRateLimitTestRouter(nil, testRateLimit, testRateWindow)

	w := postLogin(t, r)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200；body=%s", w.Code, w.Body.String())
	}
	if !*called {
		t.Fatal("nil counter 时应直接放行")
	}
}

func TestRateLimitDisabledConfigPassesThrough(t *testing.T) {
	cases := []struct {
		name    string
		counter RateCounter
		limit   int64
		window  time.Duration
	}{
		{"limit 为 0", &fakeCounter{count: 999}, 0, testRateWindow},
		{"window 为 0", &fakeCounter{count: 999}, testRateLimit, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, called := newRateLimitTestRouter(tc.counter, tc.limit, tc.window)
			w := postLogin(t, r)
			if w.Code != http.StatusOK || !*called {
				t.Fatalf("配置无效时应放行，实际 code=%d called=%v", w.Code, *called)
			}
		})
	}
}

// key 必须是 prefix:ip，且 window 原样传给计数器（窗口由计数器控制 TTL）。
func TestRateLimitKeyComposition(t *testing.T) {
	counter := &fakeCounter{count: 1}
	r, _ := newRateLimitTestRouter(counter, testRateLimit, testRateWindow)

	postLogin(t, r, func(req *http.Request) { req.RemoteAddr = "203.0.113.9:5678" })

	if len(counter.keys) != 1 {
		t.Fatalf("计数器 Hit 调用次数 = %d, want 1", len(counter.keys))
	}
	if want := testRatePrefix + ":203.0.113.9"; counter.keys[0] != want {
		t.Fatalf("限流 key = %q, want %q", counter.keys[0], want)
	}
	if counter.windows[0] != testRateWindow {
		t.Fatalf("window = %v, want %v", counter.windows[0], testRateWindow)
	}
}

// 反代链路（nginx / Pod 内 127.0.0.1）追加的 X-Forwarded-For 应被采信：
// handler/router.go 通过 SetTrustedProxies(私有网段) 打开这个行为。
func TestRateLimitUsesXFFFromTrustedProxy(t *testing.T) {
	counter := &fakeCounter{count: 1}
	r, _ := newRateLimitTestRouter(counter, testRateLimit, testRateWindow)
	if err := r.SetTrustedProxies([]string{"127.0.0.1/8"}); err != nil {
		t.Fatalf("SetTrustedProxies: %v", err)
	}

	postLogin(t, r, func(req *http.Request) {
		req.RemoteAddr = "127.0.0.1:4321"
		req.Header.Set("X-Forwarded-For", "203.0.113.9")
	})

	if want := testRatePrefix + ":203.0.113.9"; counter.keys[0] != want {
		t.Fatalf("限流 key = %q, want %q（应采信可信代理注入的真实 IP）", counter.keys[0], want)
	}
}

// 公网直连对端伪造 XFF 不应改变分桶：否则限流既能被绕过，也能被用来占别人的桶。
func TestRateLimitIgnoresSpoofedXFFFromUntrustedPeer(t *testing.T) {
	counter := &fakeCounter{count: 1}
	r, _ := newRateLimitTestRouter(counter, testRateLimit, testRateWindow)
	if err := r.SetTrustedProxies([]string{"127.0.0.1/8"}); err != nil {
		t.Fatalf("SetTrustedProxies: %v", err)
	}

	postLogin(t, r, func(req *http.Request) {
		req.RemoteAddr = "198.51.100.7:1234"
		req.Header.Set("X-Forwarded-For", "203.0.113.9")
	})

	if want := testRatePrefix + ":198.51.100.7"; counter.keys[0] != want {
		t.Fatalf("限流 key = %q, want %q（不可信对端的 XFF 必须被忽略）", counter.keys[0], want)
	}
}

func TestRetryAfterSeconds(t *testing.T) {
	window := 5 * time.Minute
	cases := []struct {
		name string
		ttl  time.Duration
		want int64
	}{
		{"亚秒剩余至少 1 秒", 500 * time.Millisecond, 1},
		{"向上取整 4.2s -> 5", 4200 * time.Millisecond, 5},
		{"整数秒不变", 5 * time.Second, 5},
		{"窗口自然剩余", 240 * time.Second, 240},
		{"TTL 缺失退化为完整窗口", 0, 300},
		{"Redis 的 -1 退化为完整窗口", -1, 300},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := retryAfterSeconds(tc.ttl, window); got != tc.want {
				t.Fatalf("retryAfterSeconds(%v) = %d, want %d", tc.ttl, got, tc.want)
			}
		})
	}
}

// nil 客户端要变成 nil counter（限流降级为放行），而不是一个调用就 panic 的接口值。
func TestNewRedisRateCounterNilClient(t *testing.T) {
	if counter := NewRedisRateCounter(nil); counter != nil {
		t.Fatalf("NewRedisRateCounter(nil) = %#v, want nil", counter)
	}
}

// 连接不上 Redis 时 Hit 必须返回错误（由 RateLimit fail-open），而不是 panic 或挂住。
// 指向 127.0.0.1:1（保留端口，必然拒连），不依赖真实 Redis 实例。
func TestRedisRateCounterHitReturnsErrorWhenRedisUnavailable(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 200 * time.Millisecond,
		MaxRetries:  -1,
	})
	t.Cleanup(func() { _ = rdb.Close() })

	counter := NewRedisRateCounter(rdb)
	count, retryAfter, err := counter.Hit(context.Background(), testRatePrefix+":203.0.113.9", testRateWindow)
	if err == nil {
		t.Fatalf("Redis 不可达时应返回错误，实际 count=%d retryAfter=%d", count, retryAfter)
	}
	if count != 0 || retryAfter != 0 {
		t.Fatalf("出错时计数与 retryAfter 应为零值，实际 count=%d retryAfter=%d", count, retryAfter)
	}
}
