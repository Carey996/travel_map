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

// ListNotes 列出笔记，可按地点过滤。
func (h *Handler) ListNotes(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}

	query := h.DB.WithContext(ctx(c)).Model(&model.Note{}).Where("user_id = ?", user.ID)
	if raw := c.Query("place_id"); raw != "" {
		placeID, err := uuid.Parse(raw)
		if err != nil {
			h.fail(c, apperr.BadRequest("invalid_id", "place_id 不是合法的 UUID"))
			return
		}
		query = query.Where("place_id = ?", placeID)
	}

	var notes []model.Note
	if err := query.
		Order("happened_on DESC NULLS LAST, created_at DESC").
		Limit(queryInt(c, "limit", 200, 1, 1000)).
		Find(&notes).Error; err != nil {
		h.fail(c, err)
		return
	}

	out := make([]dto.NoteOut, 0, len(notes))
	for i := range notes {
		out = append(out, dto.NewNoteOut(&notes[i]))
	}
	c.JSON(http.StatusOK, out)
}

// CreateNote 写一篇笔记。
func (h *Handler) CreateNote(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	var req dto.NoteCreate
	if !h.bind(c, &req) {
		return
	}

	// 确认目标地点属于当前用户
	var place model.Place
	if err := h.DB.WithContext(ctx(c)).First(&place, "id = ? AND user_id = ?", req.PlaceID, user.ID).Error; err != nil {
		h.fail(c, apperr.NotFound("place_not_found", "地点不存在"))
		return
	}

	note := model.Note{
		UserID:     user.ID,
		PlaceID:    req.PlaceID,
		Title:      strings.TrimSpace(req.Title),
		BodyMD:     req.BodyMD,
		HappenedOn: req.HappenedOn.PtrTime(),
	}
	if err := h.DB.WithContext(ctx(c)).Create(&note).Error; err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.NewNoteOut(&note))
}

// UpdateNote 更新笔记。
func (h *Handler) UpdateNote(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	note, ok := h.ownedNote(c, user.ID)
	if !ok {
		return
	}
	var req dto.NoteUpdate
	if !h.bind(c, &req) {
		return
	}

	if req.Title != nil {
		note.Title = *req.Title
	}
	if req.BodyMD != nil {
		note.BodyMD = *req.BodyMD
	}
	if req.HappenedOn != nil {
		note.HappenedOn = req.HappenedOn.PtrTime()
	}

	if err := h.DB.WithContext(ctx(c)).Save(note).Error; err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewNoteOut(note))
}

// DeleteNote 删除笔记。
func (h *Handler) DeleteNote(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	note, ok := h.ownedNote(c, user.ID)
	if !ok {
		return
	}
	if err := h.DB.WithContext(ctx(c)).Where("id = ?", note.ID).Delete(&model.Note{}).Error; err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ownedNote(c *gin.Context, userID uuid.UUID) (*model.Note, bool) {
	id, ok := h.pathUUID(c, "note_id")
	if !ok {
		return nil, false
	}
	var note model.Note
	err := h.DB.WithContext(ctx(c)).First(&note, "id = ? AND user_id = ?", id, userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		h.fail(c, apperr.NotFound("note_not_found", "笔记不存在"))
		return nil, false
	}
	if err != nil {
		h.fail(c, err)
		return nil, false
	}
	return &note, true
}
