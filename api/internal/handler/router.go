package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/Carey996/travel_map/api/internal/middleware"
)

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
