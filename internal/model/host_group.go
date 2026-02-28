package model

import "time"

// HostGroup 主机分组模型
type HostGroup struct {
	ID          string    `json:"id" gorm:"column:id;primaryKey;type:varchar(36)"`
	Name        string    `json:"name" gorm:"column:name;type:varchar(100);not null"`
	Description string    `json:"description,omitempty" gorm:"column:description;type:text"`
	Color       string    `json:"color,omitempty" gorm:"column:color;type:varchar(20)"`
	Icon        string    `json:"icon,omitempty" gorm:"column:icon;type:varchar(50)"`
	SortOrder   int       `json:"sortOrder" gorm:"column:sort_order;default:0"`
	CreatedBy   string    `json:"createdBy,omitempty" gorm:"column:created_by;type:varchar(36)"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`

	// 关联字段（不存储在数据库）
	HostCount   int `json:"hostCount" gorm:"-"`
	OnlineCount int `json:"onlineCount" gorm:"-"`
}

func (HostGroup) TableName() string {
	return "host_groups"
}

// HostGroupMember 主机-分组关联模型
type HostGroupMember struct {
	ID      uint      `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	GroupID string    `json:"groupId" gorm:"column:group_id;type:varchar(36);not null;index"`
	HostID  string    `json:"hostId" gorm:"column:host_id;type:varchar(36);not null;index"`
	AddedBy string    `json:"addedBy,omitempty" gorm:"column:added_by;type:varchar(36)"`
	AddedAt time.Time `json:"addedAt" gorm:"column:added_at;autoCreateTime"`
}

func (HostGroupMember) TableName() string {
	return "host_group_members"
}

// HostGroupWithHosts 带主机列表的分组
type HostGroupWithHosts struct {
	HostGroup
	Hosts []Host `json:"hosts"`
}

// HostGroupStatistics 分组统计信息
type HostGroupStatistics struct {
	GroupID      string `json:"groupId"`
	GroupName    string `json:"groupName"`
	TotalHosts   int    `json:"totalHosts"`
	OnlineHosts  int    `json:"onlineHosts"`
	OfflineHosts int    `json:"offlineHosts"`
}
