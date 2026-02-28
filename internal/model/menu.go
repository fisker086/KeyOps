package model

import (
	"time"
)

// Menu 菜单模型
type Menu struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	ParentID  string    `json:"parentId" gorm:"type:varchar(36);index;default:''"` // 父菜单ID，空字符串表示顶级菜单
	Path      string    `json:"path" gorm:"type:varchar(255);not null"`            // 路由路径
	Name      string    `json:"name" gorm:"type:varchar(100);not null"`            // 路由名称（唯一标识）
	Component string    `json:"component,omitempty" gorm:"type:varchar(255)"`     // 前端组件路径
	Hidden    bool      `json:"hidden" gorm:"default:false"`                      // 是否隐藏
	Sort      int       `json:"sort" gorm:"default:0;index"`                      // 排序
	Meta      MenuMeta  `json:"meta" gorm:"embedded"`                             // 菜单元数据
	Children  []Menu    `json:"children,omitempty" gorm:"-"`                       // 子菜单（不存储）
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (Menu) TableName() string {
	return "menus"
}

// MenuMeta 菜单元数据
type MenuMeta struct {
	Title       string `json:"title" gorm:"type:varchar(100);not null"`       // 菜单标题
	Icon        string `json:"icon,omitempty" gorm:"type:varchar(50)"`       // 菜单图标
	KeepAlive   bool   `json:"keepAlive" gorm:"default:false"`                // 是否缓存
	ActiveName  string `json:"activeName,omitempty" gorm:"type:varchar(100)"` // 激活菜单名称
	CloseTab    bool   `json:"closeTab" gorm:"default:false"`                 // 是否自动关闭标签页
	DefaultMenu bool   `json:"defaultMenu" gorm:"default:false"`              // 是否是默认菜单
}

// MenuPermission 菜单权限关联（角色和菜单的关联）
type MenuPermission struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	RoleID    string    `json:"roleId" gorm:"type:varchar(100);not null;index"` // 角色ID（可以是role:admin、role:user或角色ID）
	MenuID    string    `json:"menuId" gorm:"type:varchar(36);not null;index"`
	CreatedBy string    `json:"createdBy,omitempty" gorm:"type:varchar(36)"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
}

func (MenuPermission) TableName() string {
	return "menu_permissions"
}

// API API模型（用于API权限管理）
type API struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Path        string    `json:"path" gorm:"type:varchar(255);not null;index"`        // API路径
	Method      string    `json:"method" gorm:"type:varchar(20);not null;index"`       // HTTP方法
	Group       string    `json:"group" gorm:"type:varchar(100);not null;index"`       // API分组
	Description string    `json:"description" gorm:"type:varchar(255)"`               // API描述
	CreatedAt   time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (API) TableName() string {
	return "apis"
}

// MenuWithPermission 带权限信息的菜单
type MenuWithPermission struct {
	Menu
	HasPermission bool `json:"hasPermission" gorm:"-"` // 用户是否有权限访问此菜单
}

