package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringList 把 []string 映射到 Postgres 的 jsonb 列。
//
// 用 jsonb 而不是 text[] 是为了与 Python 版保持一致——
// 当时选它是因为 SQLite 也能用，便于零依赖跑测试。
type StringList []string

// Value 实现 driver.Valuer。
func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]string(s))
	if err != nil {
		return nil, fmt.Errorf("序列化 StringList 失败: %w", err)
	}
	return string(b), nil
}

// Scan 实现 sql.Scanner。
func (s *StringList) Scan(src any) error {
	if src == nil {
		*s = StringList{}
		return nil
	}
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("无法把 %T 扫描为 StringList", src)
	}
	if len(raw) == 0 {
		*s = StringList{}
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("解析 StringList 失败: %w", err)
	}
	*s = out
	return nil
}

// GormDataType 告诉 GORM 该字段的数据库类型。
func (StringList) GormDataType() string { return "jsonb" }
