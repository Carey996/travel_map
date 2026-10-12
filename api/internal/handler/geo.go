package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Carey996/travel_map/api/internal/apperr"
)

// 地名结果由浏览器与 CDN 缓存；后端不保存任何结果，以保持无状态。
const geocodeCacheSeconds = 60 * 60 * 24

// ReverseGeocode 由经纬度反查地名。
func (h *Handler) ReverseGeocode(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}

	lat, err := strconv.ParseFloat(c.Query("lat"), 64)
	if err != nil || lat < -90 || lat > 90 {
		h.fail(c, apperr.Validation("lat 需为 -90 到 90 之间的数字"))
		return
	}
	lng, err := strconv.ParseFloat(c.Query("lng"), 64)
	if err != nil || lng < -180 || lng > 180 {
		h.fail(c, apperr.Validation("lng 需为 -180 到 180 之间的数字"))
		return
	}

	result, err := h.Geocoder.Reverse(ctx(c), lat, lng)
	if err != nil {
		h.fail(c, apperr.BadGateway("geocode_upstream_error", "反向地理编码服务暂时不可用"))
		return
	}
	c.Header("Cache-Control", "private, max-age="+strconv.Itoa(geocodeCacheSeconds))
	c.JSON(http.StatusOK, result)
}

// SearchPlaces 地名搜索。
func (h *Handler) SearchPlaces(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}

	query := strings.TrimSpace(c.Query("q"))
	if len([]rune(query)) < 2 {
		h.fail(c, apperr.Validation("q 至少需要 2 个字符"))
		return
	}

	items, err := h.Geocoder.Search(ctx(c), query, queryInt(c, "limit", 6, 1, 20))
	if err != nil {
		h.fail(c, apperr.BadGateway("geocode_upstream_error", "地名搜索服务暂时不可用"))
		return
	}
	c.Header("Cache-Control", "private, max-age="+strconv.Itoa(geocodeCacheSeconds))
	c.JSON(http.StatusOK, items)
}
