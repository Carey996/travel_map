package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/Carey996/travel_map/api/internal/model"
)

// NoteCreate 新建笔记。
type NoteCreate struct {
	PlaceID    uuid.UUID `json:"place_id" binding:"required"`
	Title      string    `json:"title" binding:"max=200"`
	BodyMD     string    `json:"body_md"`
	HappenedOn *DateOnly `json:"happened_on"`
}

// NoteUpdate 更新笔记。
type NoteUpdate struct {
	Title      *string   `json:"title" binding:"omitempty,max=200"`
	BodyMD     *string   `json:"body_md"`
	HappenedOn *DateOnly `json:"happened_on"`
}

// NoteOut 是笔记的对外表示。
type NoteOut struct {
	ID         uuid.UUID `json:"id"`
	PlaceID    uuid.UUID `json:"place_id"`
	Title      string    `json:"title"`
	BodyMD     string    `json:"body_md"`
	HappenedOn *DateOnly `json:"happened_on"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// NewNoteOut 由模型构造响应。
func NewNoteOut(n *model.Note) NoteOut {
	return NoteOut{
		ID: n.ID, PlaceID: n.PlaceID, Title: n.Title, BodyMD: n.BodyMD,
		HappenedOn: PtrDateOnly(n.HappenedOn), CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt,
	}
}
