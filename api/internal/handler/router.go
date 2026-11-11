package handler

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Carey996/travel_map/api/internal/middleware"
)

// maxRequestBodyBytes 是 /api/v1 请求体的硬上限（1 MiB）。
//
// 后端从不接收图片字节流（图片走预签名直传对象存储），业务请求都是元数据、
// 坐标与 slug 之类的小 JSON，1 MiB 已经宽裕；设上限是为了防止超大 body
// 在反序列化时把内存打满。
const maxRequestBodyBytes = 1 << 20

// ---- 认证入口限流 ----
//
// 按「客户端 IP + 固定窗口」计数，目标是给撞库 / 批量注册加上成本，不是替代验证码：
//   - 登录 10 次 / 5 分钟 / IP：正常用户几乎不会在 5 分钟内点 10 次登录，
//     但足够挡住常见的口令喷洒；
//   - 注册 5 次 / 1 小时 / IP：注册本身低频，5 次也够家庭/小团队共用一个出口 IP。
//
// 调整方式：直接改下面三个常量。窗口值同时写在 key 前缀里（tb:ratelimit:...），
// 改窗口时请把前缀里的时长一起改掉，否则旧计数会沿用新窗口的命名空间。
// 刻意不做成环境变量——限流值是安全参数，应跟代码一起 review，而不是部署时随手调。
const (
	authLoginRateLimitPrefix = "tb:ratelimit:auth:login:5m"
	authLoginRateLimitMax    = int64(10)
	authLoginRateLimitWindow = 5 * time.Minute

	authRegisterRateLimitPrefix = "tb:ratelimit:auth:register:1h"
	authRegisterRateLimitMax    = int64(5)
	authRegisterRateLimitWindow = time.Hour
)

// trustedProxyCIDRs 声明「哪些直连对端可能是反向代理」。
//
// gin 默认信任所有来源的 X-Forwarded-For，意味着任意客户端都能伪造它：
// 对按 IP 限流来说既可以被绕过（轮换 XFF），也可以被用来占满别人的桶。
// 这里只信任回环与私有网段（nginx / Caddy / Pod 内代理都落在这里），
// ClientIP() 会从右往左取第一个非信任地址——也就是代理追加的真实客户端 IP；
// 公网直连 API 的请求则不会被采信 XFF。
var trustedProxyCIDRs = []string{
	"127.0.0.0/8",
	"::1/128",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"fd00::/8",
}

// Router 组装全部路由与中间件。
//
// 中间件顺序很关键：CORS 必须在 CSRF 之外，跨站请求才能在预检阶段就被拒绝，
// 而不是先撞上 CSRF 校验。
func (h *Handler) Router() *gin.Engine {
	r := gin.New()
	// 必须在接收请求之前设置：限流按 IP 分桶依赖 ClientIP() 采信代理注入的
	// X-Forwarded-For（见 trustedProxyCIDRs 的说明）。
	if err := r.SetTrustedProxies(trustedProxyCIDRs); err != nil {
		// 常量 CIDR 不应解析失败；真失败了 gin 会退化成不信任任何代理，
		// 也就是按直连地址分桶——更保守、不会误信 XFF。
		slog.Warn("设置可信代理网段失败，将不采信 X-Forwarded-For", "err", err)
	}
	r.Use(gin.Recovery())
	r.Use(RequestLogger())
	r.Use(middleware.CORS(h.Cfg.CORSOrigins))
	r.Use(middleware.CSRF(h.Cfg.CSRFHeaderName, h.Cfg.CSRFHeaderValue, map[string]struct{}{
		// 探针不属于版本化业务 API，也不该被 CSRF 校验挡住
		"/healthz":       {},
		"/healthz/ready": {},
	}))

	// 探针挂在根路径，便于容器编排与网关直接访问
	r.GET("/healthz", h.Liveness)
	r.GET("/healthz/ready", h.Readiness)

	v1 := r.Group("/api/v1")
	// 必须在创建子分组之前挂上：gin 的 Group() 在创建时复制父级 handlers，
	// 之后再 Use 不会回溯到已创建的子分组。
	v1.Use(middleware.LimitBody(maxRequestBodyBytes))

	// 限流计数器不持有业务状态（计数在 Redis），Router() 里构造一次、所有限流路由复用。
	authRateCounter := middleware.NewRedisRateCounter(h.Redis)

	auth := v1.Group("/auth")
	{
		// 路由级中间件在 handler 之前执行；超限时由 RateLimit 直接 429 短路，
		// 不会进到 Register / Login（bcrypt 与查库都不会跑）。
		auth.POST("/register",
			middleware.RateLimit(authRateCounter, authRegisterRateLimitPrefix, authRegisterRateLimitMax, authRegisterRateLimitWindow),
			h.Register)
		auth.POST("/login",
			middleware.RateLimit(authRateCounter, authLoginRateLimitPrefix, authLoginRateLimitMax, authLoginRateLimitWindow),
			h.Login)
		auth.POST("/logout", h.Logout)
		auth.POST("/logout-all", h.LogoutAll)
		auth.GET("/me", h.Me)
	}

	places := v1.Group("/places")
	{
		places.GET("", h.ListPlaces)
		places.POST("", h.CreatePlace)
		places.GET("/:id", h.GetPlace)
		places.PATCH("/:id", h.UpdatePlace)
		places.DELETE("/:id", h.DeletePlace)
		places.POST("/:id/assign", h.AssignPhotos)
		places.POST("/:id/merge-into", h.MergePlace)
	}

	photos := v1.Group("/photos")
	{
		photos.POST("/upload-url", h.CreateUploadURL)
		photos.POST("", h.CreatePhoto)
		photos.GET("", h.ListPhotos)
		photos.PATCH("/:photo_id", h.UpdatePhoto)
		photos.DELETE("/:photo_id", h.DeletePhoto)
	}

	trips := v1.Group("/trips")
	{
		trips.GET("", h.ListTrips)
		trips.POST("", h.CreateTrip)
		trips.PATCH("/:id", h.UpdateTrip)
		trips.DELETE("/:id", h.DeleteTrip)
		trips.POST("/:id/places", h.AddTripPlaces)
		trips.DELETE("/:id/places/:place_id", h.RemoveTripPlace)
	}

	notes := v1.Group("/notes")
	{
		notes.GET("", h.ListNotes)
		notes.POST("", h.CreateNote)
		notes.PATCH("/:note_id", h.UpdateNote)
		notes.DELETE("/:note_id", h.DeleteNote)
	}

	geo := v1.Group("/geo")
	{
		geo.GET("/reverse", h.ReverseGeocode)
		geo.GET("/search", h.SearchPlaces)
	}

	return r
}
