package dto

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Carey996/travel_map/api/internal/model"
)

// URLSigner 由对象存储实现，用于把对象键换成预签名直链。
type URLSigner interface {
	PresignGet(ctx context.Context, key string) (string, error)
}

// UploadVariant 描述一个派生版本（缩略图 / 预览图）的直传信息。
type UploadVariant struct {
	ContentType string `json:"content_type" binding:"max=120"`
	Suffix      string `json:"suffix" binding:"max=16"`
}

// UploadURLRequest 是申请直传 URL 的请求。
type UploadURLRequest struct {
	Filename    string         `json:"filename" binding:"required,max=400"`
	ContentType string         `json:"content_type" binding:"max=120"`
	ContentHash string         `json:"content_hash" binding:"required,min=8,max=64"`
	ByteSize    int64          `json:"byte_size" binding:"gte=0"`
	Preview     *UploadVariant `json:"preview"`
	Thumb       *UploadVariant `json:"thumb"`
}

// SignedUpload 是一条预签名直传指令。
type SignedUpload struct {
	Key     string            `json:"key"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

// UploadURLResponse 返回各版本的直传地址。
//
// Duplicate 非空时表示该哈希已存在，客户端应当**跳过整个上传**，直接用这条记录。
type UploadURLResponse struct {
	Duplicate *PhotoOut     `json:"duplicate"`
	Original  SignedUpload  `json:"original"`
	Preview   *SignedUpload `json:"preview"`
	Thumb     *SignedUpload `json:"thumb"`
}

// PhotoCreate 登记照片元数据（EXIF 已由前端解析）。
type PhotoCreate struct {
	ContentHash  string     `json:"content_hash" binding:"required,min=8,max=64"`
	StorageKey   string     `json:"storage_key" binding:"required,max=512"`
	PreviewKey   *string    `json:"preview_key" binding:"omitempty,max=512"`
	ThumbKey     *string    `json:"thumb_key" binding:"omitempty,max=512"`
	OriginalName string     `json:"original_name" binding:"max=400"`
	Mime         string     `json:"mime" binding:"max=120"`
	Bytes        int64      `json:"bytes" binding:"gte=0"`
	Width        *int       `json:"width"`
	Height       *int       `json:"height"`
	Lat          *float64   `json:"lat" binding:"omitempty,gte=-90,lte=90"`
	Lng          *float64   `json:"lng" binding:"omitempty,gte=-180,lte=180"`
	Altitude     *float64   `json:"altitude"`
	TakenAt      *time.Time `json:"taken_at"`
	CameraMake   string     `json:"camera_make" binding:"max=120"`
	CameraModel  string     `json:"camera_model" binding:"max=120"`
	Lens         string     `json:"lens" binding:"max=160"`
	ISO          *int       `json:"iso"`
	FNumber      *float64   `json:"f_number"`
	Exposure     string     `json:"exposure" binding:"max=40"`
	FocalLength  *float64   `json:"focal_length"`
	Orientation  *int       `json:"orientation"`
	PlaceID      *uuid.UUID `json:"place_id"`
}

// PhotoUpdate 更新照片：说明、归属与坐标修正。
type PhotoUpdate struct {
	Caption    *string    `json:"caption"`
	PlaceID    *uuid.UUID `json:"place_id"`
	ClearPlace bool       `json:"clear_place"`
	Lat        *float64   `json:"lat" binding:"omitempty,gte=-90,lte=90"`
	Lng        *float64   `json:"lng" binding:"omitempty,gte=-180,lte=180"`
	TakenAt    *time.Time `json:"taken_at"`
}

// PhotoOut 是照片的对外表示。
//
// 注意这里没有 storage_key：对象键只在服务端使用，前端拿到的是三个预签名直链。
type PhotoOut struct {
	ID           uuid.UUID  `json:"id"`
	PlaceID      *uuid.UUID `json:"place_id"`
	Caption      string     `json:"caption"`
	OriginalName string     `json:"original_name"`
	Mime         string     `json:"mime"`
	Bytes        int64      `json:"bytes"`
	Width        *int       `json:"width"`
	Height       *int       `json:"height"`
	Lat          *float64   `json:"lat"`
	Lng          *float64   `json:"lng"`
	Altitude     *float64   `json:"altitude"`
	TakenAt      *time.Time `json:"taken_at"`
	CameraMake   string     `json:"camera_make"`
	CameraModel  string     `json:"camera_model"`
	Lens         string     `json:"lens"`
	ISO          *int       `json:"iso"`
	FNumber      *float64   `json:"f_number"`
	Exposure     string     `json:"exposure"`
	FocalLength  *float64   `json:"focal_length"`
	CreatedAt    time.Time  `json:"created_at"`
	URL          string     `json:"url"`
	PreviewURL   string     `json:"preview_url"`
	ThumbURL     string     `json:"thumb_url"`
}

// NewPhotoOut 把 ORM 对象转成带预签名直链的响应。
//
// 这是唯一一处把对象键换成 URL 的地方——直接返回模型会泄露存储键，
// 且前端拿不到可用的图片地址。签名是纯本地计算，所以每个请求现签即可。
func NewPhotoOut(ctx context.Context, p *model.Photo, signer URLSigner) (*PhotoOut, error) {
	original, err := signer.PresignGet(ctx, p.StorageKey)
	if err != nil {
		return nil, err
	}

	previewKey := p.StorageKey
	if p.PreviewKey != nil && *p.PreviewKey != "" {
		previewKey = *p.PreviewKey
	}
	preview, err := signer.PresignGet(ctx, previewKey)
	if err != nil {
		return nil, err
	}

	thumbKey := previewKey
	if p.ThumbKey != nil && *p.ThumbKey != "" {
		thumbKey = *p.ThumbKey
	}
	thumb, err := signer.PresignGet(ctx, thumbKey)
	if err != nil {
		return nil, err
	}

	return &PhotoOut{
		ID: p.ID, PlaceID: p.PlaceID, Caption: p.Caption, OriginalName: p.OriginalName,
		Mime: p.Mime, Bytes: p.Bytes, Width: p.Width, Height: p.Height,
		Lat: p.Lat, Lng: p.Lng, Altitude: p.Altitude, TakenAt: p.TakenAt,
		CameraMake: p.CameraMake, CameraModel: p.CameraModel, Lens: p.Lens,
		ISO: p.ISO, FNumber: p.FNumber, Exposure: p.Exposure, FocalLength: p.FocalLength,
		CreatedAt: p.CreatedAt,
		URL:       original, PreviewURL: preview, ThumbURL: thumb,
	}, nil
}
