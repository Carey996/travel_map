package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/Carey996/travel_map/api/internal/model"
)

// PlaceCreate 新建地点的请求。
type PlaceCreate struct {
	Name    string  `json:"name" binding:"max=160"`
	Kind    string  `json:"kind" binding:"omitempty,oneof=city spot region other"`
	Lat     float64 `json:"lat" binding:"required,gte=-90,lte=90"`
	Lng     float64 `json:"lng" binding:"required,gte=-180,lte=180"`
	Address string  `json:"address" binding:"max=400"`
}

// PlaceUpdate 更新地点；字段用指针以区分「未传」与「传了零值」。
type PlaceUpdate struct {
	Name      *string    `json:"name" binding:"omitempty,max=160"`
	Kind      *string    `json:"kind" binding:"omitempty,oneof=city spot region other"`
	Lat       *float64   `json:"lat" binding:"omitempty,gte=-90,lte=90"`
	Lng       *float64   `json:"lng" binding:"omitempty,gte=-180,lte=180"`
	SummaryMD *string    `json:"summary_md"`
	Tags      *[]string  `json:"tags"`
	Rating    *int       `json:"rating" binding:"omitempty,gte=0,lte=5"`
	DateFrom  *DateOnly  `json:"date_from"`
	DateTo    *DateOnly  `json:"date_to"`
	TripID    *uuid.UUID `json:"trip_id"`
}

// PlaceOut 是地点的对外表示。
type PlaceOut struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Kind      string     `json:"kind"`
	Lat       float64    `json:"lat"`
	Lng       float64    `json:"lng"`
	Address   string     `json:"address"`
	Geocoded  bool       `json:"geocoded"`
	SummaryMD string     `json:"summary_md"`
	Tags      []string   `json:"tags"`
	Rating    int        `json:"rating"`
	DateFrom  *DateOnly  `json:"date_from"`
	DateTo    *DateOnly  `json:"date_to"`
	TripID    *uuid.UUID `json:"trip_id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// PlaceDetailOut 在列表里额外带上照片与笔记数量。
type PlaceDetailOut struct {
	PlaceOut
	PhotoCount int64 `json:"photo_count"`
	NoteCount  int64 `json:"note_count"`
}

// NewPlaceOut 由模型构造响应。
func NewPlaceOut(p *model.Place) PlaceOut {
	tags := []string(p.Tags)
	if tags == nil {
		tags = []string{}
	}
	return PlaceOut{
		ID: p.ID, Name: p.Name, Kind: p.Kind, Lat: p.Lat, Lng: p.Lng,
		Address: p.Address, Geocoded: p.Geocoded, SummaryMD: p.SummaryMD,
		Tags: tags, Rating: p.Rating,
		DateFrom: PtrDateOnly(p.DateFrom), DateTo: PtrDateOnly(p.DateTo),
		TripID: p.TripID, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// AssignPhotosRequest 把照片移入某地点。
type AssignPhotosRequest struct {
	PhotoIDs         []uuid.UUID `json:"photo_ids" binding:"required,min=1,max=1000"`
	PruneEmptyPlaces bool        `json:"prune_empty_places"`
}

// MergePlaceRequest 把该地点并入另一个地点。
type MergePlaceRequest struct {
	TargetID uuid.UUID `json:"target_id" binding:"required"`
}
