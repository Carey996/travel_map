// Package model 定义 GORM 模型。
//
// 表结构与 Python/SQLAlchemy 版完全一致，可以直接对接已有数据库。
// 所有列名都显式声明：GORM 的自动蛇形转换对 SummaryMD / BodyMD 这类连续大写缩写
// 并不总是可靠（MD 不在 GORM 的 commonInitialisms 名单里），显式声明彻底避免这类隐患。
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NewID 生成时间有序的 UUID v7；无状态服务可本地生成主键，无需向数据库申请序列。
func NewID() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.New() // 理论上不会失败；退化为 v4 也好过 panic
	}
	return id
}

// User 账号。
type User struct {
	ID           uuid.UUID `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	Email        string    `gorm:"column:email;size:320;uniqueIndex;not null" json:"email"`
	DisplayName  string    `gorm:"column:display_name;size:80;not null;default:''" json:"display_name"`
	PasswordHash string    `gorm:"column:password_hash;size:255;not null" json:"-"`
	IsActive     bool      `gorm:"column:is_active;not null;default:true" json:"is_active"`
	CreatedAt    time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

// TableName 固定表名，避免依赖 GORM 的复数化推断。
func (User) TableName() string { return "users" }

// BeforeCreate 在入库前补上 UUID v7 主键。
func (u *User) BeforeCreate(*gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = NewID()
	}
	return nil
}

// Place 地点：地图上的一个大头针，可以是城市也可以是具体景点。
type Place struct {
	ID        uuid.UUID  `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID  `gorm:"column:user_id;type:uuid;not null;index" json:"-"`
	TripID    *uuid.UUID `gorm:"column:trip_id;type:uuid;index" json:"trip_id"`
	Name      string     `gorm:"column:name;size:160;not null;default:''" json:"name"`
	Kind      string     `gorm:"column:kind;size:16;not null;default:'spot'" json:"kind"`
	Lat       float64    `gorm:"column:lat;not null" json:"lat"`
	Lng       float64    `gorm:"column:lng;not null" json:"lng"`
	SummaryMD string     `gorm:"column:summary_md;type:text;not null;default:''" json:"summary_md"`
	Tags      StringList `gorm:"column:tags;type:jsonb;not null" json:"tags"`
	Rating    int        `gorm:"column:rating;not null;default:0" json:"rating"`
	DateFrom  *time.Time `gorm:"column:date_from;type:date" json:"date_from"`
	DateTo    *time.Time `gorm:"column:date_to;type:date" json:"date_to"`
	Address   string     `gorm:"column:address;size:400;not null;default:''" json:"address"`
	Geocoded  bool       `gorm:"column:geocoded;not null;default:false" json:"geocoded"`
	CreatedAt time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Place) TableName() string { return "places" }

func (p *Place) BeforeCreate(*gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = NewID()
	}
	return nil
}

// Photo 照片元数据；二进制在对象存储，这里只保存对象键与 EXIF 解析结果。
type Photo struct {
	ID      uuid.UUID  `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	UserID  uuid.UUID  `gorm:"column:user_id;type:uuid;not null;index;uniqueIndex:uq_photos_user_content,priority:1" json:"-"`
	PlaceID *uuid.UUID `gorm:"column:place_id;type:uuid;index" json:"place_id"`

	// 对象键只在服务端使用，绝不返回给前端（响应里是预签名直链）
	StorageKey string  `gorm:"column:storage_key;size:512;not null" json:"-"`
	PreviewKey *string `gorm:"column:preview_key;size:512" json:"-"`
	ThumbKey   *string `gorm:"column:thumb_key;size:512" json:"-"`

	OriginalName string `gorm:"column:original_name;size:400;not null;default:''" json:"original_name"`
	Mime         string `gorm:"column:mime;size:120;not null;default:'image/jpeg'" json:"mime"`
	Bytes        int64  `gorm:"column:bytes;not null;default:0" json:"bytes"`
	Width        *int   `gorm:"column:width" json:"width"`
	Height       *int   `gorm:"column:height" json:"height"`

	Lat      *float64   `gorm:"column:lat" json:"lat"`
	Lng      *float64   `gorm:"column:lng" json:"lng"`
	Altitude *float64   `gorm:"column:altitude" json:"altitude"`
	TakenAt  *time.Time `gorm:"column:taken_at;index" json:"taken_at"`

	CameraMake  string   `gorm:"column:camera_make;size:120;not null;default:''" json:"camera_make"`
	CameraModel string   `gorm:"column:camera_model;size:120;not null;default:''" json:"camera_model"`
	Lens        string   `gorm:"column:lens;size:160;not null;default:''" json:"lens"`
	ISO         *int     `gorm:"column:iso" json:"iso"`
	FNumber     *float64 `gorm:"column:f_number" json:"f_number"`
	Exposure    string   `gorm:"column:exposure;size:40;not null;default:''" json:"exposure"`
	FocalLength *float64 `gorm:"column:focal_length" json:"focal_length"`
	Orientation *int     `gorm:"column:orientation" json:"orientation"`

	Caption string `gorm:"column:caption;type:text;not null;default:''" json:"caption"`

	// 注意是「每个用户唯一」而非全局唯一：两个用户拍到同一张图不该互相冲突
	ContentHash string `gorm:"column:content_hash;size:64;not null;uniqueIndex:uq_photos_user_content,priority:2" json:"-"`

	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Photo) TableName() string { return "photos" }

func (p *Photo) BeforeCreate(*gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = NewID()
	}
	return nil
}

// Note 旅行笔记：挂在地点下，一个地点可以按天写多篇。
type Note struct {
	ID         uuid.UUID  `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	UserID     uuid.UUID  `gorm:"column:user_id;type:uuid;not null;index" json:"-"`
	PlaceID    uuid.UUID  `gorm:"column:place_id;type:uuid;not null;index" json:"place_id"`
	Title      string     `gorm:"column:title;size:200;not null;default:''" json:"title"`
	BodyMD     string     `gorm:"column:body_md;type:text;not null;default:''" json:"body_md"`
	HappenedOn *time.Time `gorm:"column:happened_on;type:date;index" json:"happened_on"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Note) TableName() string { return "notes" }

func (n *Note) BeforeCreate(*gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = NewID()
	}
	return nil
}

// Trip 行程：把多个地点串成一次旅行（表结构已就绪，界面待启用）。
type Trip struct {
	ID        uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	UserID    uuid.UUID  `gorm:"column:user_id;type:uuid;not null;index"`
	Title     string     `gorm:"column:title;size:200;not null;default:''"`
	Slug      string     `gorm:"column:slug;size:200;not null;uniqueIndex"`
	SummaryMD string     `gorm:"column:summary_md;type:text;not null;default:''"`
	DateFrom  *time.Time `gorm:"column:date_from;type:date"`
	DateTo    *time.Time `gorm:"column:date_to;type:date"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt time.Time  `gorm:"column:updated_at;not null"`
}

func (Trip) TableName() string { return "trips" }

func (t *Trip) BeforeCreate(*gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = NewID()
	}
	return nil
}

// TripPlace 行程与地点的连接表，带顺序与第几天。
type TripPlace struct {
	TripID   uuid.UUID `gorm:"column:trip_id;type:uuid;primaryKey"`
	PlaceID  uuid.UUID `gorm:"column:place_id;type:uuid;primaryKey"`
	Position int       `gorm:"column:position;not null;default:0"`
	DayIndex *int      `gorm:"column:day_index"`
}

func (TripPlace) TableName() string { return "trip_places" }

// Share 公开只读分享链接（表结构已就绪）。
type Share struct {
	ID         uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	UserID     uuid.UUID  `gorm:"column:user_id;type:uuid;not null;index"`
	Slug       string     `gorm:"column:slug;size:120;not null;uniqueIndex"`
	TargetType string     `gorm:"column:target_type;size:16;not null"`
	TargetID   uuid.UUID  `gorm:"column:target_id;type:uuid;not null"`
	Title      string     `gorm:"column:title;size:200;not null;default:''"`
	IsPublic   bool       `gorm:"column:is_public;not null;default:true"`
	ExpiresAt  *time.Time `gorm:"column:expires_at"`
	ViewCount  int        `gorm:"column:view_count;not null;default:0"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;not null"`
}

func (Share) TableName() string { return "shares" }

func (s *Share) BeforeCreate(*gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = NewID()
	}
	return nil
}

// UserSession 会话记录，仅当 SESSION_BACKEND=db 时使用。
type UserSession struct {
	ID        uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	UserID    uuid.UUID  `gorm:"column:user_id;type:uuid;not null;index"`
	TokenHash string     `gorm:"column:token_hash;size:64;not null;uniqueIndex"`
	UserAgent string     `gorm:"column:user_agent;size:400;not null;default:''"`
	IP        string     `gorm:"column:ip;size:64;not null;default:''"`
	ExpiresAt time.Time  `gorm:"column:expires_at;not null;index"`
	RevokedAt *time.Time `gorm:"column:revoked_at"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
}

func (UserSession) TableName() string { return "user_sessions" }

func (s *UserSession) BeforeCreate(*gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = NewID()
	}
	return nil
}

// AllModels 供自动迁移使用，顺序即建表顺序。
func AllModels() []any {
	return []any{&User{}, &Trip{}, &Place{}, &Photo{}, &Note{}, &TripPlace{}, &Share{}, &UserSession{}}
}
