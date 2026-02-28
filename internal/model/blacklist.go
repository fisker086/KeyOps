package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// StringArray 字符串数组类型，用于存储 JSON 数组
type StringArray []string

// Scan 实现 sql.Scanner 接口
func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = []string{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	return json.Unmarshal(bytes, s)
}

// Value 实现 driver.Valuer 接口
func (s StringArray) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "[]", nil
	}
	return json.Marshal(s)
}

// BlacklistRule 黑名单规则
type BlacklistRule struct {
	ID          string      `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Command     string      `json:"command" gorm:"type:varchar(255);not null;comment:命令名称"`
	Pattern     string      `json:"pattern" gorm:"type:varchar(512);not null;comment:匹配模式"`
	Description string      `json:"description" gorm:"type:text;comment:说明"`
	Scope       string      `json:"scope" gorm:"type:varchar(20);default:global;comment:作用范围:global/user"`
	Users       StringArray `json:"users" gorm:"type:json;comment:限制用户列表"`
	Enabled     bool        `json:"enabled" gorm:"default:true;comment:是否启用"`
	CreatedAt   time.Time   `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt   time.Time   `json:"updatedAt" gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (BlacklistRule) TableName() string {
	return "blacklist_rules"
}
