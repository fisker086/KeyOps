package model

import (
	"time"
)

// Application 应用服务模型
type Application struct {
	ID              string      `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Org             string      `json:"org" gorm:"type:varchar(100);index"`                                   // 事业部（关联到 organizations.unit_code）
	LineOfBiz       string      `json:"lineOfBiz" gorm:"type:varchar(100);index"`                             // 业务线
	Name            string      `json:"name" gorm:"type:varchar(255);not null;index"`                         // 应用名称
	IsCritical      bool        `json:"isCritical" gorm:"type:boolean;default:false;index"`                   // 是否核心应用
	SrvType         string      `json:"srvType" gorm:"type:varchar(50);not null;index"`                       // 应用类型：SERVER、WEB、MIDDLEWARE、DATAWARE、MOBILE、DATABASE、MICROSERVICE、BATCH、SCHEDULER、GATEWAY、CACHE、MESSAGE_QUEUE、BACKEND（API已合并到BACKEND）
	VirtualTech     string      `json:"virtualTech" gorm:"type:varchar(50);index"`                            // 虚拟化技术类型：K8S、EC2、ECS、GCE
	Status          string      `json:"status" gorm:"type:varchar(50);not null;default:'Initializing';index"` // 应用状态：Initializing、Running、Stopped
	Site            string      `json:"site" gorm:"type:varchar(50);index"`                                   // 应用站点（扩展字段，可留空），示例值：大陆、香港、北美、欧洲等，可根据实际需求填写
	Department      string      `json:"department" gorm:"type:varchar(100);index"`                            // 部门（关联到 organizations.unit_code）
	Description     string      `json:"description" gorm:"type:text"`                                         // 应用功能用途描述和备注信息
	OnlineAt        *time.Time  `json:"onlineAt" gorm:"type:datetime;index"`                                  // 应用上线时间
	OfflineAt       *time.Time  `json:"offlineAt" gorm:"type:datetime"`                                       // 应用下线时间
	JenkinsServerID *uint       `json:"jenkinsServerId" gorm:"type:int;index"`                                // Jenkins服务器ID（关联到jenkins_servers.id）
	JenkinsJobName  string      `json:"jenkinsJobName" gorm:"type:varchar(255);index"`                        // Jenkins Job名称
	GitURL          string      `json:"gitUrl" gorm:"type:varchar(500)"`                                      // Git地址
	OpsOwners       StringArray `json:"opsOwners" gorm:"type:json"`                                           // 运维负责人(多选)
	TestOwners      StringArray `json:"testOwners" gorm:"type:json"`                                          // 测试负责人(多选)
	DevOwners       StringArray `json:"devOwners" gorm:"type:json"`                                           // 研发负责人(多选)
	CreatedAt       time.Time   `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt       time.Time   `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (Application) TableName() string {
	return "applications"
}

// CreateApplicationRequest 创建应用请求
type CreateApplicationRequest struct {
	Org             string      `json:"org"`
	LineOfBiz       string      `json:"lineOfBiz"`
	Name            string      `json:"name" binding:"required"`
	IsCritical      bool        `json:"isCritical"`
	SrvType         string      `json:"srvType" binding:"required"`
	VirtualTech     string      `json:"virtualTech"`
	Status          string      `json:"status"`
	Site            string      `json:"site"`
	Department      string      `json:"department"`
	Description     string      `json:"description"`
	OnlineAt        *time.Time  `json:"onlineAt"`
	OfflineAt       *time.Time  `json:"offlineAt"`
	JenkinsServerID *uint       `json:"jenkinsServerId"`
	JenkinsJobName  string      `json:"jenkinsJobName"`
	GitURL          string      `json:"gitUrl"`
	OpsOwners       StringArray `json:"opsOwners"`
	TestOwners      StringArray `json:"testOwners"`
	DevOwners       StringArray `json:"devOwners"`
}

// UpdateApplicationRequest 更新应用请求
type UpdateApplicationRequest struct {
	Org             string      `json:"org"`
	LineOfBiz       string      `json:"lineOfBiz"`
	Name            string      `json:"name" binding:"required"`
	IsCritical      bool        `json:"isCritical"`
	SrvType         string      `json:"srvType" binding:"required"`
	VirtualTech     string      `json:"virtualTech"`
	Status          string      `json:"status"`
	Site            string      `json:"site"`
	Department      string      `json:"department"`
	Description     string      `json:"description"`
	OnlineAt        *time.Time  `json:"onlineAt"`
	OfflineAt       *time.Time  `json:"offlineAt"`
	JenkinsServerID *uint       `json:"jenkinsServerId"`
	JenkinsJobName  string      `json:"jenkinsJobName"`
	GitURL          string      `json:"gitUrl"`
	OpsOwners       StringArray `json:"opsOwners"`
	TestOwners      StringArray `json:"testOwners"`
	DevOwners       StringArray `json:"devOwners"`
}
