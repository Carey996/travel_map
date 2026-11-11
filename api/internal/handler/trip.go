package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Carey996/travel_map/api/internal/apperr"
	"github.com/Carey996/travel_map/api/internal/dto"
	"github.com/Carey996/travel_map/api/internal/model"
)

// ListTrips 列出当前用户的行程，带地点数与照片数。
func (h *Handler) ListTrips(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}

	var trips []model.Trip
	err := h.DB.WithContext(ctx(c)).
		Where("user_id = ?", user.ID).
		Order("date_from DESC NULLS LAST, created_at DESC").
		Find(&trips).Error
	if err != nil {
		h.fail(c, err)
		return
	}

	placeCounts, err := h.tripPlaceCounts(c, user.ID)
	if err != nil {
		h.fail(c, err)
		return
	}
	photoCounts, err := h.tripPhotoCounts(c, user.ID)
	if err != nil {
		h.fail(c, err)
		return
	}

	out := make([]dto.TripOut, 0, len(trips))
	for i := range trips {
		item := dto.NewTripOut(&trips[i])
		item.PlaceCount = placeCounts[trips[i].ID]
		item.PhotoCount = photoCounts[trips[i].ID]
		out = append(out, item)
	}
	c.JSON(http.StatusOK, out)
}

// CreateTrip 新建行程。
func (h *Handler) CreateTrip(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	var req dto.TripCreate
	if !h.bind(c, &req) {
		return
	}

	// ID 先定下来：slug 由它派生，而 slug 有唯一索引
	id := model.NewID()
	trip := model.Trip{
		ID:        id,
		UserID:    user.ID,
		Title:     strings.TrimSpace(req.Title),
		Slug:      tripSlug(id),
		SummaryMD: req.SummaryMD,
	}
	if req.DateFrom != nil {
		trip.DateFrom = req.DateFrom.PtrTime()
	}
	if req.DateTo != nil {
		trip.DateTo = req.DateTo.PtrTime()
	}

	if err := h.DB.WithContext(ctx(c)).Create(&trip).Error; err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.NewTripOut(&trip))
}

// UpdateTrip 更新行程。
func (h *Handler) UpdateTrip(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	trip, ok := h.ownedTrip(c, user.ID)
	if !ok {
		return
	}
	var req dto.TripUpdate
	if !h.bind(c, &req) {
		return
	}

	if req.Title != nil {
		trip.Title = strings.TrimSpace(*req.Title)
	}
	if req.SummaryMD != nil {
		trip.SummaryMD = *req.SummaryMD
	}
	if req.DateFrom != nil {
		trip.DateFrom = req.DateFrom.PtrTime()
	}
	if req.DateTo != nil {
		trip.DateTo = req.DateTo.PtrTime()
	}

	if err := h.DB.WithContext(ctx(c)).Save(trip).Error; err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewTripOut(trip))
}

// DeleteTrip 删除行程；地点本身保留，只是解除归属。
func (h *Handler) DeleteTrip(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	trip, ok := h.ownedTrip(c, user.ID)
	if !ok {
		return
	}

	tx := h.DB.WithContext(ctx(c)).Begin()
	if tx.Error != nil {
		h.fail(c, tx.Error)
		return
	}
	if err := tx.Model(&model.Place{}).Where("user_id = ? AND trip_id = ?", user.ID, trip.ID).
		Update("trip_id", nil).Error; err != nil {
		tx.Rollback()
		h.fail(c, err)
		return
	}
	if err := tx.Where("id = ? AND user_id = ?", trip.ID, user.ID).Delete(&model.Trip{}).Error; err != nil {
		tx.Rollback()
		h.fail(c, err)
		return
	}
	if err := tx.Commit().Error; err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// AddTripPlaces 批量把地点加入行程（地点必须属于当前用户）。
func (h *Handler) AddTripPlaces(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	trip, ok := h.ownedTrip(c, user.ID)
	if !ok {
		return
	}
	var req dto.TripPlacesRequest
	if !h.bind(c, &req) {
		return
	}

	result := h.DB.WithContext(ctx(c)).Model(&model.Place{}).
		Where("user_id = ? AND id IN ?", user.ID, req.PlaceIDs).
		Update("trip_id", trip.ID)
	if result.Error != nil {
		h.fail(c, result.Error)
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated": result.RowsAffected})
}

// RemoveTripPlace 把某个地点移出行程。
func (h *Handler) RemoveTripPlace(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	trip, ok := h.ownedTrip(c, user.ID)
	if !ok {
		return
	}
	placeID, ok := h.pathUUID(c, "place_id")
	if !ok {
		return
	}

	if err := h.DB.WithContext(ctx(c)).Model(&model.Place{}).
		Where("user_id = ? AND id = ? AND trip_id = ?", user.ID, placeID, trip.ID).
		Update("trip_id", nil).Error; err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// 内部辅助
// ---------------------------------------------------------------------------

// tripSlug 生成稳定且唯一的行程标识（中文标题无法直接 slug 化）。
//
// 必须用**完整**的 uuid：UUIDv7 的高位是毫秒时间戳，只取前 12 位相当于
// 「同一天创建的行程几乎只有 2 位随机性」，同一毫秒建两个旅行就会撞
// slug 的唯一索引（表现为 500）。32 位 hex + "trip-" 前缀共 37 字符，
// 远在模型 size:200 之内。
func tripSlug(id uuid.UUID) string {
	return "trip-" + strings.ReplaceAll(id.String(), "-", "")
}

func (h *Handler) ownedTrip(c *gin.Context, userID uuid.UUID) (*model.Trip, bool) {
	id, ok := h.pathUUID(c, "id")
	if !ok {
		return nil, false
	}
	var trip model.Trip
	err := h.DB.WithContext(ctx(c)).First(&trip, "id = ? AND user_id = ?", id, userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		h.fail(c, apperr.NotFound("trip_not_found", "行程不存在"))
		return nil, false
	}
	if err != nil {
		h.fail(c, err)
		return nil, false
	}
	return &trip, true
}

func (h *Handler) tripPlaceCounts(c *gin.Context, userID uuid.UUID) (map[uuid.UUID]int64, error) {
	var rows []struct {
		TripID uuid.UUID
		Total  int64
	}
	err := h.DB.WithContext(ctx(c)).Model(&model.Place{}).
		Select("trip_id, count(*) AS total").
		Where("user_id = ? AND trip_id IS NOT NULL", userID).
		Group("trip_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]int64, len(rows))
	for _, r := range rows {
		out[r.TripID] = r.Total
	}
	return out, nil
}

func (h *Handler) tripPhotoCounts(c *gin.Context, userID uuid.UUID) (map[uuid.UUID]int64, error) {
	var rows []struct {
		TripID uuid.UUID
		Total  int64
	}
	err := h.DB.WithContext(ctx(c)).Model(&model.Photo{}).
		Select("places.trip_id AS trip_id, count(*) AS total").
		Joins("JOIN places ON places.id = photos.place_id").
		Where("photos.user_id = ? AND places.trip_id IS NOT NULL", userID).
		Group("places.trip_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]int64, len(rows))
	for _, r := range rows {
		out[r.TripID] = r.Total
	}
	return out, nil
}
