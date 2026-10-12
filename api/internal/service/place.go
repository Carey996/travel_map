// Package service 存放可复用的业务逻辑，不依赖 Gin。
package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Carey996/travel_map/api/internal/geo"
	"github.com/Carey996/travel_map/api/internal/model"
)

// AutoAssignRadiusM 是自动归入同一地点的半径。
//
// 同一个地点内的照片通常相距很近；1.5 km 能覆盖一座小城中心或一个景区。
const AutoAssignRadiusM = 1500.0

// NearestPlace 找出离给定坐标最近的地点。
//
// 只有落在 maxDistanceM 之内才算命中；否则返回 nil 与实际最近距离。
func NearestPlace(ctx context.Context, gdb *gorm.DB, userID uuid.UUID, lat, lng, maxDistanceM float64) (*model.Place, float64, error) {
	var places []model.Place
	if err := gdb.WithContext(ctx).Where("user_id = ?", userID).Find(&places).Error; err != nil {
		return nil, 0, fmt.Errorf("查询地点失败: %w", err)
	}

	var best *model.Place
	bestDist := math.Inf(1)
	for i := range places {
		d := geo.Haversine(lat, lng, places[i].Lat, places[i].Lng)
		if d < bestDist {
			best, bestDist = &places[i], d
		}
	}
	if best != nil && bestDist <= maxDistanceM {
		return best, bestDist, nil
	}
	return nil, bestDist, nil
}

// AssignPhoto 把照片归入地点；1.5 km 内没有已有地点时就地新建一个待命名的地点。
//
// 返回最终归属的地点（照片无 GPS 时为 nil）。调用方负责提交事务。
func AssignPhoto(ctx context.Context, gdb *gorm.DB, userID uuid.UUID, photo *model.Photo, radiusM float64) (*model.Place, error) {
	if photo.Lat == nil || photo.Lng == nil {
		return nil, nil
	}

	if photo.PlaceID != nil {
		var place model.Place
		if err := gdb.WithContext(ctx).First(&place, "id = ?", *photo.PlaceID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, nil
			}
			return nil, fmt.Errorf("查询地点失败: %w", err)
		}
		if err := extendDateRange(ctx, gdb, &place, photo); err != nil {
			return nil, err
		}
		return &place, nil
	}

	place, _, err := NearestPlace(ctx, gdb, userID, *photo.Lat, *photo.Lng, radiusM)
	if err != nil {
		return nil, err
	}
	if place == nil {
		place = &model.Place{
			UserID:   userID,
			Kind:     "spot",
			Lat:      *photo.Lat,
			Lng:      *photo.Lng,
			Geocoded: false,
			Tags:     model.StringList{},
		}
		if err := gdb.WithContext(ctx).Create(place).Error; err != nil {
			return nil, fmt.Errorf("新建地点失败: %w", err)
		}
	}

	photo.PlaceID = &place.ID
	if err := extendDateRange(ctx, gdb, place, photo); err != nil {
		return nil, err
	}
	return place, nil
}

// extendDateRange 让地点的时间范围自动覆盖其照片的拍摄日期，
// 前端地点列表因此能显示真实的行程日期，而不是「今天创建」。
func extendDateRange(ctx context.Context, gdb *gorm.DB, place *model.Place, photo *model.Photo) error {
	if photo.TakenAt == nil {
		return nil
	}
	taken := truncateToDate(*photo.TakenAt)

	changed := false
	if place.DateFrom == nil || taken.Before(*place.DateFrom) {
		place.DateFrom = &taken
		changed = true
	}
	if place.DateTo == nil || taken.After(*place.DateTo) {
		place.DateTo = &taken
		changed = true
	}
	if !changed {
		return nil
	}
	if err := gdb.WithContext(ctx).Model(place).
		Updates(map[string]any{"date_from": place.DateFrom, "date_to": place.DateTo}).Error; err != nil {
		return fmt.Errorf("更新地点日期范围失败: %w", err)
	}
	return nil
}

func truncateToDate(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}
