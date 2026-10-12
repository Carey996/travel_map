package dto

import (
	"bytes"
	"strings"
	"time"
)

// DateOnly 是只到日期的可空字段。
//
// 需要它是因为两端口径不同：浏览器 el-date-picker 提交的是 2025-04-01 这种纯日期串，
// 而 Go 的 time.Time 默认按 RFC3339 解析。这里两种都接受，输出统一为 2006-01-02。
type DateOnly struct {
	Time  time.Time
	Valid bool
}

// NewDateOnly 由 time.Time 构造（会截断到日）。
func NewDateOnly(t time.Time) DateOnly {
	utc := t.UTC()
	return DateOnly{Time: time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
}

// PtrDateOnly 由可空 time.Time 构造。
func PtrDateOnly(t *time.Time) *DateOnly {
	if t == nil {
		return nil
	}
	d := NewDateOnly(*t)
	return &d
}

// PtrTime 转回可空 time.Time。
func (d *DateOnly) PtrTime() *time.Time {
	if d == nil || !d.Valid {
		return nil
	}
	return &d.Time
}

// UnmarshalJSON 接受 2025-04-01、RFC3339 与 null。
func (d *DateOnly) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == quote+quote || trimmed == "" {
		d.Valid = false
		return nil
	}

	unquoted := strings.Trim(trimmed, quote)
	for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05", "2006/01/02"} {
		if parsed, err := time.Parse(layout, unquoted); err == nil {
			*d = NewDateOnly(parsed)
			return nil
		}
	}
	return &time.ParseError{
		Layout:  "2006-01-02",
		Value:   unquoted,
		Message: "无法解析日期，需为 YYYY-MM-DD 或 RFC3339",
	}
}

// MarshalJSON 统一输出 2006-01-02 或 null。
func (d DateOnly) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte("null"), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('"')
	buf.WriteString(d.Time.Format("2006-01-02"))
	buf.WriteByte('"')
	return buf.Bytes(), nil
}

// quote 是双引号字符，避免在源码里到处写转义。
const quote = "\""
