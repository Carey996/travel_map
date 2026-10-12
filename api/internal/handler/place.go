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

// ListPlaces 列出地点，支持视口与关键字过滤。
func (h *Handler) ListPlaces(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}

	query := h.DB.WithContext(ctx(c)).Model(&model.Place{}).Where("user_id = ?", user.ID)

	bounds, err := parseBBox(c.Query("bbox"))
	if err != nil {
		h.fail(c, err)
		return
	}
	if bounds != nil {
		query = query.Where("lng >= ? AND lng <= ? AND lat >= ? AND lat <= ?",
			bounds.MinLng, bounds.MaxLng, bounds.MinLat, bounds.MaxLat)
	}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		pattern := "%" + q + "%"
		query = query.Where("name ILIKE ? OR summary_md ILIKE ?", pattern, pattern)
	}

	var places []model.Place
	if err := query.Order("updated_at DESC").Limit(queryInt(c, "limit", 1000, 1, 5000)).Find(&places).Error; err != nil {
		h.fail(c, err)
		return
	}

	photoCounts, err := h.photoCounts(c, user.ID)
	if err != nil {
		h.fail(c, err)
		return
	}
	noteCounts, err := h.noteCounts(c, user.ID)
	if err != nil {
		h.fail(c, err)
		return
	}

	out := make([]dto.PlaceDetailOut, 0, len(places))
	for i := range places {
		out = append(out, dto.PlaceDetailOut{
			PlaceOut:   dto.NewPlaceOut(&places[i]),
			PhotoCount: photoCounts[places[i].ID],
			NoteCount:  noteCounts[places[i].ID],
		})
	}
	c.JSON(http.StatusOK, out)
}

// CreatePlace 新建地点。
func (h *Handler) CreatePlace(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	var req dto.PlaceCreate
	if !h.bind(c, &req) {
		return
	}

	kind := req.Kind
	if kind == "" {
		kind = "spot"
	}
	place := model.Place{
		UserID:   user.ID,
		Name:     strings.TrimSpace(req.Name),
		Kind:     kind,
		Lat:      req.Lat,
		Lng:      req.Lng,
		Address:  req.Address,
		Geocoded: req.Address != "",
		Tags:     model.StringList{},
	}
	if err := h.DB.WithContext(ctx(c)).Create(&place).Error; err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.NewPlaceOut(&place))
}

// GetPlace 地点详情。
func (h *Handler) GetPlace(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	place, ok := h.ownedPlace(c, user.ID)
	if !ok {
		return
	}

	var photoCount, noteCount int64
	h.DB.WithContext(ctx(c)).Model(&model.Photo{}).Where("place_id = ?", place.ID).Count(&photoCount)
	h.DB.WithContext(ctx(c)).Model(&model.Note{}).Where("place_id = ?", place.ID).Count(&noteCount)

	c.JSON(http.StatusOK, dto.PlaceDetailOut{
		PlaceOut:   dto.NewPlaceOut(place),
		PhotoCount: photoCount,
		NoteCount:  noteCount,
	})
}

// UpdatePlace 更新地点。
func (h *Handler) UpdatePlace(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	place, ok := h.ownedPlace(c, user.ID)
	if !ok {
		return
	}
	var req dto.PlaceUpdate
	if !h.bind(c, &req) {
		return
	}

	// 一旦人为改名或补了地址，就认为名称已确认，不再需要自动地理编码
	if req.Name != nil {
		place.Name = strings.TrimSpace(*req.Name)
		place.Geocoded = true
	}
	if req.Kind != nil {
		place.Kind = *req.Kind
	}
	if req.Lat != nil {
		place.Lat = *req.Lat
	}
	if req.Lng != nil {
		place.Lng = *req.Lng
	}
	if req.SummaryMD != nil {
		place.SummaryMD = *req.SummaryMD
	}
	if req.Tags != nil {
		place.Tags = model.StringList(*req.Tags)
	}
	if req.Rating != nil {
		place.Rating = *req.Rating
	}
	if req.DateFrom != nil {
		place.DateFrom = req.DateFrom.PtrTime()
	}
	if req.DateTo != nil {
		place.DateTo = req.DateTo.PtrTime()
	}
	if req.TripID != nil {
		place.TripID = req.TripID
	}

	if err := h.DB.WithContext(ctx(c)).Save(place).Error; err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewPlaceOut(place))
}

// DeletePlace 删除地点；默认保留照片，仅解除归属。
func (h *Handler) DeletePlace(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	place, ok := h.ownedPlace(c, user.ID)
	if !ok {
		return
	}
	deletePhotos, _ := queryBool(c, "delete_photos")

	tx := h.DB.WithContext(ctx(c)).Begin()
	if tx.Error != nil {
		h.fail(c, tx.Error)
		return
	}

	if deletePhotos {
		var photos []model.Photo
		if err := tx.Where("place_id = ?", place.ID).Find(&photos).Error; err != nil {
			tx.Rollback()
			h.fail(c, err)
			return
		}
		keys := make([]string, 0, len(photos)*3)
		ids := make([]uuid.UUID, 0, len(photos))
		for _, p := range photos {
			ids = append(ids, p.ID)
			keys = append(keys, p.StorageKey)
			if p.PreviewKey != nil {
				keys = append(keys, *p.PreviewKey)
			}
			if p.ThumbKey != nil {
				keys = append(keys, *p.ThumbKey)
			}
		}
		if len(ids) > 0 {
			if err := tx.Where("id IN ?", ids).Delete(&model.Photo{}).Error; err != nil {
				tx.Rollback()
				h.fail(c, err)
				return
			}
		}
		// 删对象放在事务提交之后更稳妥，但这里先删数据库再删对象，
		// 失败也只是留下孤儿对象，不会破坏一致性
		defer func() { _ = h.Storage.Delete(ctx(c), keys) }()
	} else {
		if err := tx.Model(&model.Photo{}).Where("place_id = ?", place.ID).
			Update("place_id", nil).Error; err != nil {
			tx.Rollback()
			h.fail(c, err)
			return
		}
	}

	if err := tx.Where("place_id = ?", place.ID).Delete(&model.Note{}).Error; err != nil {
		tx.Rollback()
		h.fail(c, err)
		return
	}
	if err := tx.Where("id = ?", place.ID).Delete(&model.Place{}).Error; err != nil {
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

// AssignPhotos 把照片移入该地点。
func (h *Handler) AssignPhotos(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	place, ok := h.ownedPlace(c, user.ID)
	if !ok {
		return
	}
	var req dto.AssignPhotosRequest
	if !h.bind(c, &req) {
		return
	}

	if err := h.DB.WithContext(ctx(c)).Model(&model.Photo{}).
		Where("user_id = ? AND id IN ?", user.ID, req.PhotoIDs).
		Update("place_id", place.ID).Error; err != nil {
		h.fail(c, err)
		return
	}

	var photoCount, noteCount int64
	h.DB.WithContext(ctx(c)).Model(&model.Photo{}).Where("place_id = ?", place.ID).Count(&photoCount)
	h.DB.WithContext(ctx(c)).Model(&model.Note{}).Where("place_id = ?", place.ID).Count(&noteCount)

	c.JSON(http.StatusOK, dto.PlaceDetailOut{
		PlaceOut:   dto.NewPlaceOut(place),
		PhotoCount: photoCount,
		NoteCount:  noteCount,
	})
}

// MergePlace 把该地点整体并入另一个地点：先搬照片与笔记，再删除被合并者。
func (h *Handler) MergePlace(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	source, ok := h.ownedPlace(c, user.ID)
	if !ok {
		return
	}
	var req dto.MergePlaceRequest
	if !h.bind(c, &req) {
		return
	}
	if source.ID == req.TargetID {
		h.fail(c, apperr.Validation("不能合并到自身"))
		return
	}

	var target model.Place
	if err := h.DB.WithContext(ctx(c)).First(&target, "id = ? AND user_id = ?", req.TargetID, user.ID).Error; err != nil {
		h.fail(c, apperr.NotFound("place_not_found", "目标地点不存在"))
		return
	}

	tx := h.DB.WithContext(ctx(c)).Begin()
	if tx.Error != nil {
		h.fail(c, tx.Error)
		return
	}
	if err := tx.Model(&model.Photo{}).Where("place_id = ?", source.ID).Update("place_id", target.ID).Error; err != nil {
		tx.Rollback()
		h.fail(c, err)
		return
	}
	if err := tx.Model(&model.Note{}).Where("place_id = ?", source.ID).Update("place_id", target.ID).Error; err != nil {
		tx.Rollback()
		h.fail(c, err)
		return
	}
	if err := tx.Where("id = ?", source.ID).Delete(&model.Place{}).Error; err != nil {
		tx.Rollback()
		h.fail(c, err)
		return
	}
	if err := tx.Commit().Error; err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewPlaceOut(&target))
}

// ---------------------------------------------------------------------------
// 内部辅助
// ---------------------------------------------------------------------------

// ownedPlace 解析并校验地点归属；每个查询都必须带 user_id 过滤，
// 这是多租户隔离的唯一防线。
func (h *Handler) ownedPlace(c *gin.Context, userID uuid.UUID) (*model.Place, bool) {
	id, ok := h.pathUUID(c, "id")
	if !ok {
		return nil, false
	}
	var place model.Place
	err := h.DB.WithContext(ctx(c)).First(&place, "id = ? AND user_id = ?", id, userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		h.fail(c, apperr.NotFound("place_not_found", "地点不存在"))
		return nil, false
	}
	if err != nil {
		h.fail(c, err)
		return nil, false
	}
	return &place, true
}

func (h *Handler) photoCounts(c *gin.Context, userID uuid.UUID) (map[uuid.UUID]int64, error) {
	var rows []struct {
		PlaceID uuid.UUID
		Total   int64
	}
	err := h.DB.WithContext(ctx(c)).Model(&model.Photo{}).
		Select("place_id, count(*) AS total").
		Where("user_id = ? AND place_id IS NOT NULL", userID).
		Group("place_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]int64, len(rows))
	for _, r := range rows {
		out[r.PlaceID] = r.Total
	}
	return out, nil
}

func (h *Handler) noteCounts(c *gin.Context, userID uuid.UUID) (map[uuid.UUID]int64, error) {
	var rows []struct {
		PlaceID uuid.UUID
		Total   int64
	}
	err := h.DB.WithContext(ctx(c)).Model(&model.Note{}).
		Select("place_id, count(*) AS total").
		Where("user_id = ?", userID).
		Group("place_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]int64, len(rows))
	for _, r := range rows {
		out[r.PlaceID] = r.Total
	}
	return out, nil
}
