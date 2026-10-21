package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/Carey996/travel_map/api/internal/model"
)

// TripCreate 新建行程。
type TripCreate struct {
	Title     string    `json:"title" binding:"max=200"`
	SummaryMD string    `json:"summary_md"`
	DateFrom  *DateOnly `json:"date_from"`
	DateTo    *DateOnly `json:"date_to"`
}

// TripUpdate 更新行程；字段用指针以区分「未传」与「传了零值」。
type TripUpdate struct {
	Title     *string   `json:"title" binding:"omitempty,max=200"`
	SummaryMD *string   `json:"summary_md"`
	DateFrom  *DateOnly `json:"date_from"`
	DateTo    *DateOnly `json:"date_to"`
}

// TripOut 是行程的对外表示。
//
// slug 不对外暴露：它是给将来的公开分享页用的稳定标识，界面上用 id 就够。
// 两个计数给列表与地图用——地图上要按照片数决定色块的强调程度。
type TripOut struct {
	ID         uuid.UUID `json:"id"`
	Title      string    `json:"title"`
	SummaryMD  string    `json:"summary_md"`
	DateFrom   *DateOnly `json:"date_from"`
	DateTo     *DateOnly `json:"date_to"`
	PlaceCount int64     `json:"place_count"`
	PhotoCount int64     `json:"photo_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// NewTripOut 由模型构造响应。
func NewTripOut(t *model.Trip) TripOut {
	return TripOut{
		ID: t.ID, Title: t.Title, SummaryMD: t.SummaryMD,
		DateFrom: PtrDateOnly(t.DateFrom), DateTo: PtrDateOnly(t.DateTo),
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

// TripPlacesRequest 批量把地点加入行程。
type TripPlacesRequest struct {
	PlaceIDs []uuid.UUID `json:"place_ids" binding:"required,min=1,max=1000"`
}
