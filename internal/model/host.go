package model

import (
	"fmt"
	"time"
)

// DeviceType 设备类型常量
const (
	DeviceTypeLinux   = "linux"
	DeviceTypeWindows = "windows"
	DeviceTypeNetwork = "network"
	DeviceTypeVMware  = "vmware"
	DeviceTypeDocker  = "docker"
	// 新增设备类型（与前端统一）
	DeviceTypeServer   = "server"
	DeviceTypeSwitch   = "switch"
	DeviceTypeRouter   = "router"
	DeviceTypeFirewall = "firewall"
	DeviceTypeOther    = "other"
)

// ConnectionMode 连接模式常量
const (
	ConnectionModeAuto   = "auto"   // 自动探测
	ConnectionModeDirect = "direct" // 强制直连
	ConnectionModeProxy  = "proxy"  // 强制通过代理
)

type Host struct {
	ID          string `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Name        string `json:"name" gorm:"type:varchar(255);not null"`
	IP          string `json:"ip" gorm:"type:varchar(45);not null;index"`
	Port        int    `json:"port" gorm:"default:22"`
	Status      string `json:"status" gorm:"type:varchar(20);default:'unknown';index"`
	OS          string `json:"os" gorm:"type:varchar(100)"`
	CPU         string `json:"cpu" gorm:"type:varchar(100)"`
	Memory      string `json:"memory" gorm:"type:varchar(50)"`
	Tags        string `json:"tags" gorm:"type:text"` // JSON array
	Description string `json:"description" gorm:"type:text"`
	DeviceType  string `json:"deviceType" gorm:"type:varchar(20);default:'linux';index"` // linux, windows, network, vmware, docker

	// 连接路由相关字段
	ConnectionMode string `json:"connectionMode" gorm:"type:varchar(20);default:'auto';index"` // auto, direct, proxy - 连接模式
	ProxyID        string `json:"proxyId,omitempty" gorm:"type:varchar(128);index"`            // 指定的代理ID（当connectionMode=proxy时使用）
	NetworkZone    string `json:"networkZone,omitempty" gorm:"type:varchar(50);index"`         // 网络区域标识（用于路由规则匹配）

	// 注意：认证信息（用户名、密码、密钥）已移至 SystemUser 模型
	// 通过 PermissionRule 关联主机和系统用户，实现更灵活的权限管理

	LastLoginTime *time.Time `json:"lastLoginTime" gorm:"type:timestamp"`
	LoginCount    int        `json:"loginCount" gorm:"default:0"`
	CreatedAt     time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (Host) TableName() string {
	return "hosts"
}

// ValidateDeviceType 验证设备类型是否有效
func (h *Host) ValidateDeviceType() error {
	// 如果设备类型为空，使用默认值
	if h.DeviceType == "" {
		h.DeviceType = DeviceTypeServer
		return nil
	}

	validTypes := []string{
		DeviceTypeLinux, DeviceTypeWindows, DeviceTypeNetwork,
		DeviceTypeVMware, DeviceTypeDocker,
		DeviceTypeServer, DeviceTypeSwitch, DeviceTypeRouter,
		DeviceTypeFirewall, DeviceTypeOther,
	}
	for _, t := range validTypes {
		if h.DeviceType == t {
			return nil
		}
	}
	return fmt.Errorf("invalid device type: %s", h.DeviceType)
}

// RoutingDecision 路由决策结果
type RoutingDecision struct {
	Mode     string `json:"mode"`     // direct, proxy
	Direct   bool   `json:"direct"`   // 是否直连
	ProxyID  string `json:"proxyId"`  // 代理ID（如果使用代理）
	ProxyURL string `json:"proxyUrl"` // 代理URL（如果使用代理）
	Reason   string `json:"reason"`   // 决策原因
}
