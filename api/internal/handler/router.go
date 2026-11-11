package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/Carey996/travel_map/api/internal/middleware"
)

// maxRequestBodyBytes 是 /api/v1 请求体的硬上限（1 MiB）。
//
// 后端从不接收图片字节流（图片走预签名直传对象存储），业务请求都是元数据、
// 坐标与 slug 之类的小 JSON，1 MiB 已经宽裕；设上限是为了防止超大 body
// 在反序列化时把内存打满。
const maxRequestBodyBytes = 1 << 20

// Router 组装全部路由与中间件。
//
// 中间件顺序很关键：CORS 必须在 CSRF 之外，跨站请求才能在预检阶段就被拒绝，
// 而不是先撞上 CSRF 校验。
func (h *Handler) Router() *gin.Engine {
	r := gin.New()
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

	auth := v1.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
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
