package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Carey996/travel_map/api/internal/apperr"
)

// bbox 表示地图视口范围。
type bbox struct {
	MinLng, MinLat, MaxLng, MaxLat float64
}

// parseBBox 解析 minLng,minLat,maxLng,maxLat 形式的视口参数。
func parseBBox(raw string) (*bbox, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return nil, apperr.BadRequest("invalid_bbox", "bbox 需要 4 个数字：minLng,minLat,maxLng,maxLat")
	}
	values := make([]float64, 4)
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, apperr.BadRequest("invalid_bbox", "bbox 含有无法解析的数字")
		}
		values[i] = v
	}
	if values[0] > values[2] || values[1] > values[3] {
		return nil, apperr.BadRequest("invalid_bbox", "bbox 的最小值必须小于最大值")
	}
	return &bbox{MinLng: values[0], MinLat: values[1], MaxLng: values[2], MaxLat: values[3]}, nil
}

// queryInt 读取整型查询参数。
func queryInt(c *gin.Context, key string, fallback, min, max int) int {
	raw := c.Query(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// queryBool 读取布尔查询参数。
func queryBool(c *gin.Context, key string) (value, present bool) {
	raw := c.Query(key)
	if raw == "" {
		return false, false
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes":
		return true, true
	default:
		return false, true
	}
}

// queryDate 读取 YYYY-MM-DD 形式的日期参数。
func queryDate(c *gin.Context, key string) (*time.Time, error) {
	raw := c.Query(key)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, apperr.BadRequest("invalid_date", key+" 需为 YYYY-MM-DD 格式")
	}
	return &parsed, nil
}
