package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Carey996/travel_map/api/internal/middleware"
)

// errEnvelope 是统一错误形状，测试里只关心这三个字段。
type errEnvelope struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Detail  map[string]any `json:"detail"`
}

func decodeErr(t *testing.T, body []byte) errEnvelope {
	t.Helper()
	var env errEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("响应不是合法 JSON: %v, body=%s", err, body)
	}
	return env
}

func doJSON(t *testing.T, r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestPasswordTooLong(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		pw   string
		want bool
	}{
		{"空密码", "", false},
		{"71 字节 ASCII", strings.Repeat("a", 71), false},
		{"72 字节 ASCII（恰好上限）", strings.Repeat("a", 72), false},
		{"73 字节 ASCII（超一字节）", strings.Repeat("a", 73), true},
		{"24 个汉字 = 72 字节", strings.Repeat("中", 24), false},
		{"25 个汉字 = 75 字节", strings.Repeat("中", 25), true},
		{"18 个 4 字节 emoji = 72 字节", strings.Repeat("😀", 18), false},
		{"19 个 4 字节 emoji = 76 字节", strings.Repeat("😀", 19), true},
		{"混合：71 字节 + 1 汉字 = 74 字节", strings.Repeat("a", 71) + "密", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := passwordTooLong(tc.pw); got != tc.want {
				t.Fatalf("passwordTooLong(%d 字节) = %v, want %v", len([]byte(tc.pw)), got, tc.want)
			}
		})
	}
}

// 同一毫秒创建的 UUIDv7 前 12 位 hex 相同（时间戳前缀），旧实现只取这 12 位，
// 两个行程会撞 slug 唯一索引。这里用两个只差尾部的 UUID 固定这个回归。
func TestTripSlugUsesFullUUID(t *testing.T) {
	t.Parallel()

	id1 := uuid.MustParse("0190a1b2-c3d4-7000-8000-000000000001")
	id2 := uuid.MustParse("0190a1b2-c3d4-7000-8000-000000000002")

	slug1, slug2 := tripSlug(id1), tripSlug(id2)
	if slug1 == slug2 {
		t.Fatalf("前 12 位相同的两个 UUID 生成了相同 slug: %q", slug1)
	}
	want := "trip-0190a1b2c3d470008000000000000001"
	if slug1 != want {
		t.Fatalf("tripSlug(%s) = %q, want %q（应为完整 32 位 hex）", id1, slug1, want)
	}
	if len(slug2) != len("trip-")+32 {
		t.Fatalf("slug 长度 = %d, want %d", len(slug2), len("trip-")+32)
	}
}

func TestRegisterRejectsOverlongPassword(t *testing.T) {
	t.Parallel()

	// 超长密码必须在碰数据库之前就被拒绝，因此这里用零值 Handler 即可。
	// 若实现顺序写反，nil DB 会直接 panic，测试会明确失败。
	h := &Handler{}
	r := gin.New()
	r.POST("/api/v1/auth/register", h.Register)

	body := `{"email":"user@example.com","password":"` + strings.Repeat("a", 73) + `","display_name":"tester"}`
	w := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, want 400；body=%s", w.Code, w.Body.String())
	}
	env := decodeErr(t, w.Body.Bytes())
	if env.Code != "validation_error" {
		t.Fatalf("code = %q, want validation_error", env.Code)
	}
	if !strings.Contains(env.Message, "72") {
		t.Fatalf("message 应说明 72 字节上限，实际 %q", env.Message)
	}
	if _, ok := env.Detail["errors"]; ok {
		t.Fatalf("这不是 binding 校验失败，不应带 detail.errors: %v", env.Detail)
	}
}

func TestLoginRejectsOverlongPassword(t *testing.T) {
	t.Parallel()

	// 同样要在查库之前拒绝：超长密码不可能匹配任何合法哈希。
	// 实现仍会做一次 dummy bcrypt 比对以保持时序，所以这条跑得比纯逻辑测试慢。
	h := &Handler{}
	r := gin.New()
	r.POST("/api/v1/auth/login", h.Login)

	body := `{"email":"user@example.com","password":"` + strings.Repeat("a", 73) + `"}`
	w := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", body)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("状态码 = %d, want 401（不能是 500）；body=%s", w.Code, w.Body.String())
	}
	env := decodeErr(t, w.Body.Bytes())
	if env.Code != "invalid_credentials" {
		t.Fatalf("code = %q, want invalid_credentials", env.Code)
	}
}

// bodyLimitRouter 复刻生产装配：LimitBody 挂在分组上，bind 负责把超限翻译成 413。
func bodyLimitRouter(maxBytes int64) *gin.Engine {
	h := &Handler{}
	r := gin.New()
	g := r.Group("/api/v1")
	g.Use(middleware.LimitBody(maxBytes))
	g.POST("/echo", func(c *gin.Context) {
		var body map[string]any
		if !h.bind(c, &body) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestBodyLimitRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	const limit = 64
	r := bodyLimitRouter(limit)

	// 合法 JSON，但远超 64 字节上限。
	body := `{"data":"` + strings.Repeat("a", 512) + `"}`
	w := doJSON(t, r, http.MethodPost, "/api/v1/echo", body)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("状态码 = %d, want 413；body=%s", w.Code, w.Body.String())
	}
	env := decodeErr(t, w.Body.Bytes())
	if env.Code != "payload_too_large" {
		t.Fatalf("code = %q, want payload_too_large", env.Code)
	}
	if env.Message == "" {
		t.Fatal("message 不应为空")
	}
	if got, ok := env.Detail["limit_bytes"].(float64); !ok || int64(got) != limit {
		t.Fatalf("detail.limit_bytes = %v (%T), want %d", env.Detail["limit_bytes"], env.Detail["limit_bytes"], limit)
	}
}

// 上限之内的请求必须照常通过，证明中间件没有误伤正常请求。
func TestBodyLimitAllowsNormalBody(t *testing.T) {
	t.Parallel()

	const limit = 1 << 20
	r := bodyLimitRouter(limit)

	w := doJSON(t, r, http.MethodPost, "/api/v1/echo", `{"a":"b"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200；body=%s", w.Code, w.Body.String())
	}
}

// 上限之内的坏 JSON 仍然是 422，不能和 413 混为一谈。
func TestBodyLimitMalformedBodyStill422(t *testing.T) {
	t.Parallel()

	r := bodyLimitRouter(1 << 20)
	w := doJSON(t, r, http.MethodPost, "/api/v1/echo", `{"a":`)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("状态码 = %d, want 422；body=%s", w.Code, w.Body.String())
	}
	env := decodeErr(t, w.Body.Bytes())
	if env.Code != "validation_error" {
		t.Fatalf("code = %q, want validation_error", env.Code)
	}
}
