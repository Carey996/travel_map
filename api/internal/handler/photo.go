package handler

import (
	"errors"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Carey996/travel_map/api/internal/apperr"
	"github.com/Carey996/travel_map/api/internal/dto"
	"github.com/Carey996/travel_map/api/internal/model"
	"github.com/Carey996/travel_map/api/internal/service"
)

var allowedExtensions = map[string]struct{}{
	".jpg": {}, ".jpeg": {}, ".png": {}, ".webp": {}, ".gif": {},
	".tif": {}, ".tiff": {}, ".heic": {}, ".heif": {}, ".avif": {}, ".bmp": {},
}

// fileExtension 优先信任文件名的后缀，其次按 MIME 猜，最后退化为 .jpg。
func fileExtension(filename, contentType string) string {
	ext := strings.ToLower(path.Ext(filename))
	if _, ok := allowedExtensions[ext]; ok {
		if ext == ".jpeg" {
			return ".jpg"
		}
		return ext
	}
	if guessed, err := mime.ExtensionsByType(contentType); err == nil && len(guessed) > 0 {
		for _, g := range guessed {
			if _, ok := allowedExtensions[g]; ok {
				return g
			}
		}
	}
	return ".jpg"
}

// validStorageKey 校验客户端提交的对象键确实属于当前用户。
//
// 上传 URL 由服务端签发，键格式固定为 "u/<user.UUID>/<hash>.<ext>"（见 CreateUploadURL）。
// 登记照片时浏览器会把键原样传回来：若不校验，用户 A 只要提交用户 B 的对象键，
// 就能在响应的预签名直链里拿到 B 的照片——对象存储层的租户隔离会完全失效。
func validStorageKey(userID uuid.UUID, key string) bool {
	if userID == uuid.Nil || key == "" {
		return false
	}
	// 对象键由服务端生成，绝不应该出现路径语义；出现即视为恶意或损坏数据
	if strings.Contains(key, "..") || strings.Contains(key, `\`) || strings.ContainsRune(key, 0) {
		return false
	}
	prefix := "u/" + userID.String() + "/"
	if !strings.HasPrefix(key, prefix) {
		return false
	}
	rest := strings.TrimPrefix(key, prefix)
	return rest != "" && !strings.HasPrefix(rest, "/")
}

// validContentHash 校验客户端提交的内容哈希可以安全地拼进对象键。
//
// content_hash 会被直接拼成 "u/<uid>/<content_hash><ext>"，若允许 "/"、".."，
// 用户就能让服务端为任意键签发 PUT。前端用的是 SHA-256 十六进制串，这里按此收紧。
func validContentHash(hash string) bool {
	if len(hash) < 8 || len(hash) > 64 {
		return false
	}
	for _, r := range hash {
		isDigit := r >= '0' && r <= '9'
		isLowerHex := r >= 'a' && r <= 'f'
		isUpperHex := r >= 'A' && r <= 'F'
		if !isDigit && !isLowerHex && !isUpperHex {
			return false
		}
	}
	return true
}

// normalizedKey 把「传了空串」归一成「没传」，避免把空对象键写进库。
func normalizedKey(key *string) *string {
	if key == nil || strings.TrimSpace(*key) == "" {
		return nil
	}
	return key
}

// invalidStorageKeyError 统一的对象键校验失败响应。
func invalidStorageKeyError(field string) *apperr.Error {
	return apperr.BadRequest("invalid_storage_key", "对象键不属于当前用户").
		WithDetail(map[string]any{"field": field})
}

// CreateUploadURL 签发直传用的预签名 URL，并对同一用户的重复上传短路。
func (h *Handler) CreateUploadURL(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	var req dto.UploadURLRequest
	if !h.bind(c, &req) {
		return
	}
	// content_hash 会进入对象键，必须先确认它不含路径分隔符等危险字符
	if !validContentHash(req.ContentHash) {
		h.fail(c, apperr.BadRequest("invalid_content_hash", "content_hash 必须是 8-64 位十六进制字符串"))
		return
	}

	contentType := req.ContentType
	if contentType == "" {
		contentType = "image/jpeg"
	}

	// 幂等：同一用户重复上传同一张图时不浪费带宽，直接告知已有记录
	var duplicate model.Photo
	dupErr := h.DB.WithContext(ctx(c)).First(&duplicate, "user_id = ? AND content_hash = ?", user.ID, req.ContentHash).Error
	if dupErr != nil && !errors.Is(dupErr, gorm.ErrRecordNotFound) {
		h.fail(c, dupErr)
		return
	}

	base := "u/" + user.ID.String() + "/" + req.ContentHash
	originalKey := base + fileExtension(req.Filename, contentType)

	resp := dto.UploadURLResponse{}
	originalURL, err := h.Storage.PresignPut(ctx(c), originalKey)
	if err != nil {
		h.fail(c, err)
		return
	}
	resp.Original = dto.SignedUpload{
		Key:     originalKey,
		URL:     originalURL,
		Headers: map[string]string{"Content-Type": contentType},
	}

	if req.Preview != nil {
		key := base + ".preview.jpg"
		url, err := h.Storage.PresignPut(ctx(c), key)
		if err != nil {
			h.fail(c, err)
			return
		}
		resp.Preview = &dto.SignedUpload{Key: key, URL: url, Headers: map[string]string{"Content-Type": previewType(req.Preview.ContentType)}}
	}
	if req.Thumb != nil {
		key := base + ".thumb.jpg"
		url, err := h.Storage.PresignPut(ctx(c), key)
		if err != nil {
			h.fail(c, err)
			return
		}
		resp.Thumb = &dto.SignedUpload{Key: key, URL: url, Headers: map[string]string{"Content-Type": previewType(req.Thumb.ContentType)}}
	}

	if dupErr == nil {
		out, err := dto.NewPhotoOut(ctx(c), &duplicate, h.Storage)
		if err != nil {
			h.fail(c, err)
			return
		}
		resp.Duplicate = out
	}
	c.JSON(http.StatusOK, resp)
}

func previewType(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "image/jpeg"
	}
	return raw
}

// CreatePhoto 登记照片元数据，并按 GPS 自动归入地点。
func (h *Handler) CreatePhoto(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	var req dto.PhotoCreate
	if !h.bind(c, &req) {
		return
	}

	// 幂等：重复提交只返回已有记录
	var existing model.Photo
	dupErr := h.DB.WithContext(ctx(c)).First(&existing, "user_id = ? AND content_hash = ?", user.ID, req.ContentHash).Error
	if dupErr == nil {
		out, err := dto.NewPhotoOut(ctx(c), &existing, h.Storage)
		if err != nil {
			h.fail(c, err)
			return
		}
		c.JSON(http.StatusOK, out)
		return
	}
	if !errors.Is(dupErr, gorm.ErrRecordNotFound) {
		h.fail(c, dupErr)
		return
	}

	// 请求体里的 place_id 与对象键都来自客户端：入库前必须确认它们属于当前用户。
	// 少了这一步，知道他人地点 UUID 就能把照片挂进去并改写对方的 date_from/date_to；
	// 提交他人对象键则能在响应里拿到对方的预签名直链。
	if req.PlaceID != nil && !h.requireOwnedPlace(c, user.ID, *req.PlaceID) {
		return
	}
	previewKey := normalizedKey(req.PreviewKey)
	thumbKey := normalizedKey(req.ThumbKey)
	if !validStorageKey(user.ID, req.StorageKey) {
		h.fail(c, invalidStorageKeyError("storage_key"))
		return
	}
	if previewKey != nil && !validStorageKey(user.ID, *previewKey) {
		h.fail(c, invalidStorageKeyError("preview_key"))
		return
	}
	if thumbKey != nil && !validStorageKey(user.ID, *thumbKey) {
		h.fail(c, invalidStorageKeyError("thumb_key"))
		return
	}

	photo := model.Photo{
		UserID:       user.ID,
		PlaceID:      req.PlaceID,
		StorageKey:   req.StorageKey,
		PreviewKey:   previewKey,
		ThumbKey:     thumbKey,
		OriginalName: req.OriginalName,
		Mime:         orDefault(req.Mime, "image/jpeg"),
		Bytes:        req.Bytes,
		Width:        req.Width,
		Height:       req.Height,
		Lat:          req.Lat,
		Lng:          req.Lng,
		Altitude:     req.Altitude,
		TakenAt:      req.TakenAt,
		CameraMake:   req.CameraMake,
		CameraModel:  req.CameraModel,
		Lens:         req.Lens,
		ISO:          req.ISO,
		FNumber:      req.FNumber,
		Exposure:     req.Exposure,
		FocalLength:  req.FocalLength,
		Orientation:  req.Orientation,
		ContentHash:  req.ContentHash,
	}

	tx := h.DB.WithContext(ctx(c)).Begin()
	if tx.Error != nil {
		h.fail(c, tx.Error)
		return
	}
	if err := tx.Create(&photo).Error; err != nil {
		tx.Rollback()
		h.fail(c, err)
		return
	}

	// 有 GPS 就自动归入最近地点（1.5 km 内），否则新建一个待命名的地点
	place, err := service.AssignPhoto(ctx(c), tx, user.ID, &photo, service.AutoAssignRadiusM)
	if err != nil {
		tx.Rollback()
		h.fail(c, err)
		return
	}
	// AssignPhoto 只改内存里的 photo.PlaceID，而这一行是插入之后才确定的，
	// 因此必须显式回写一次。早期版本把条件写成「值不同才更新」，
	// 而 AssignPhoto 恰好已经把它设成同一个值——于是唯一需要写库的那次被跳过了，
	// 表现为照片在地图上永远挂不到地点下。
	if place != nil {
		if err := tx.Model(&model.Photo{}).Where("id = ? AND user_id = ?", photo.ID, user.ID).
			Update("place_id", place.ID).Error; err != nil {
			tx.Rollback()
			h.fail(c, err)
			return
		}
		photo.PlaceID = &place.ID
	}
	if err := tx.Commit().Error; err != nil {
		h.fail(c, err)
		return
	}

	out, err := dto.NewPhotoOut(ctx(c), &photo, h.Storage)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

// ListPhotos 查询照片，支持按地点、是否定位、视口与日期过滤。
func (h *Handler) ListPhotos(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}

	query := h.DB.WithContext(ctx(c)).Model(&model.Photo{}).Where("user_id = ?", user.ID)

	if raw := c.Query("place_id"); raw != "" {
		placeID, err := uuid.Parse(raw)
		if err != nil {
			h.fail(c, apperr.BadRequest("invalid_id", "place_id 不是合法的 UUID"))
			return
		}
		query = query.Where("place_id = ?", placeID)
	}
	if unplaced, present := queryBool(c, "unplaced"); present && unplaced {
		query = query.Where("place_id IS NULL")
	}
	if located, present := queryBool(c, "located"); present {
		if located {
			query = query.Where("lat IS NOT NULL AND lng IS NOT NULL")
		} else {
			query = query.Where("lat IS NULL OR lng IS NULL")
		}
	}

	bounds, err := parseBBox(c.Query("bbox"))
	if err != nil {
		h.fail(c, err)
		return
	}
	if bounds != nil {
		query = query.Where("lat IS NOT NULL AND lng IS NOT NULL AND lng >= ? AND lng <= ? AND lat >= ? AND lat <= ?",
			bounds.MinLng, bounds.MaxLng, bounds.MinLat, bounds.MaxLat)
	}

	from, err := queryDate(c, "date_from")
	if err != nil {
		h.fail(c, err)
		return
	}
	if from != nil {
		query = query.Where("taken_at >= ?", *from)
	}
	to, err := queryDate(c, "date_to")
	if err != nil {
		h.fail(c, err)
		return
	}
	if to != nil {
		query = query.Where("taken_at < ?", to.AddDate(0, 0, 1))
	}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		pattern := "%" + q + "%"
		query = query.Where("caption ILIKE ? OR original_name ILIKE ?", pattern, pattern)
	}

	var photos []model.Photo
	err = query.
		Order("taken_at DESC NULLS LAST, created_at DESC").
		Limit(queryInt(c, "limit", 500, 1, 2000)).
		Offset(queryInt(c, "offset", 0, 0, 1<<30)).
		Find(&photos).Error
	if err != nil {
		h.fail(c, err)
		return
	}

	out := make([]*dto.PhotoOut, 0, len(photos))
	for i := range photos {
		item, err := dto.NewPhotoOut(ctx(c), &photos[i], h.Storage)
		if err != nil {
			h.fail(c, err)
			return
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, out)
}

// UpdatePhoto 更新说明、归属或修正坐标。
func (h *Handler) UpdatePhoto(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	photo, ok := h.ownedPhoto(c, user.ID)
	if !ok {
		return
	}
	var req dto.PhotoUpdate
	if !h.bind(c, &req) {
		return
	}

	coordinatesChanged := false
	if req.Caption != nil {
		photo.Caption = *req.Caption
	}
	if req.ClearPlace {
		photo.PlaceID = nil
	} else if req.PlaceID != nil {
		// PATCH 也能改归属，同样不能信任客户端给的 place_id
		if !h.requireOwnedPlace(c, user.ID, *req.PlaceID) {
			return
		}
		photo.PlaceID = req.PlaceID
	}
	if req.Lat != nil {
		photo.Lat = req.Lat
		coordinatesChanged = true
	}
	if req.Lng != nil {
		photo.Lng = req.Lng
		coordinatesChanged = true
	}
	if req.TakenAt != nil {
		photo.TakenAt = req.TakenAt
	}

	tx := h.DB.WithContext(ctx(c)).Begin()
	if tx.Error != nil {
		h.fail(c, tx.Error)
		return
	}
	if err := tx.Save(photo).Error; err != nil {
		tx.Rollback()
		h.fail(c, err)
		return
	}
	// 手动修正坐标后重新计算归属
	if coordinatesChanged && photo.Lat != nil && photo.Lng != nil {
		place, err := service.AssignPhoto(ctx(c), tx, user.ID, photo, service.AutoAssignRadiusM)
		if err != nil {
			tx.Rollback()
			h.fail(c, err)
			return
		}
		if place != nil {
			if err := tx.Model(&model.Photo{}).Where("id = ? AND user_id = ?", photo.ID, user.ID).Update("place_id", place.ID).Error; err != nil {
				tx.Rollback()
				h.fail(c, err)
				return
			}
			photo.PlaceID = &place.ID
		}
	}
	if err := tx.Commit().Error; err != nil {
		h.fail(c, err)
		return
	}

	out, err := dto.NewPhotoOut(ctx(c), photo, h.Storage)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// DeletePhoto 删除照片及其在对象存储中的文件。
func (h *Handler) DeletePhoto(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	photo, ok := h.ownedPhoto(c, user.ID)
	if !ok {
		return
	}

	keys := []string{photo.StorageKey}
	if photo.PreviewKey != nil {
		keys = append(keys, *photo.PreviewKey)
	}
	if photo.ThumbKey != nil {
		keys = append(keys, *photo.ThumbKey)
	}

	if err := h.DB.WithContext(ctx(c)).Where("id = ? AND user_id = ?", photo.ID, user.ID).Delete(&model.Photo{}).Error; err != nil {
		h.fail(c, err)
		return
	}
	if err := h.Storage.Delete(ctx(c), keys); err != nil {
		// 数据库已删成功；对象清理失败只记录，不影响本次响应
		h.logWarn("清理对象失败", err)
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ownedPhoto(c *gin.Context, userID uuid.UUID) (*model.Photo, bool) {
	id, ok := h.pathUUID(c, "photo_id")
	if !ok {
		return nil, false
	}
	var photo model.Photo
	err := h.DB.WithContext(ctx(c)).First(&photo, "id = ? AND user_id = ?", id, userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		h.fail(c, apperr.NotFound("photo_not_found", "照片不存在"))
		return nil, false
	}
	if err != nil {
		h.fail(c, err)
		return nil, false
	}
	return &photo, true
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

var _ = time.Now // 保留 time 导入，供后续扩展使用
