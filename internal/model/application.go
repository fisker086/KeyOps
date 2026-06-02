package model

import (
	"time"
)

// Application 应用服务模型
type Application struct {
	ID          string      `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Org         string      `json:"org" gorm:"type:varchar(100);index"`                                   // 事业部（关联到 organizations.unit_code）
	LineOfBiz   string      `json:"lineOfBiz" gorm:"type:varchar(100);index"`                             // 业务线
	Name        string      `json:"name" gorm:"type:varchar(255);not null;index"`                         // 应用名称
	IsCritical  bool        `json:"isCritical" gorm:"type:boolean;default:false;index"`                   // 是否核心应用
	SrvType     string      `json:"srvType" gorm:"type:varchar(50);not null;index"`                       // 应用类型：SERVER、WEB、MIDDLEWARE、DATAWARE、MOBILE、DATABASE、MICROSERVICE、BATCH、SCHEDULER、GATEWAY、CACHE、MESSAGE_QUEUE、BACKEND（API已合并到BACKEND）
	VirtualTech string      `json:"virtualTech" gorm:"type:varchar(50);index"`                            // 虚拟化技术类型：K8S、EC2、ECS、GCE
	DevLanguage string      `json:"devLanguage" gorm:"column:dev_language;type:varchar(50);index"`        // 开发语言：java/nodejs/golang/python/php 等
	Status      string      `json:"status" gorm:"type:varchar(50);not null;default:'Initializing';index"` // 应用状态：Initializing、Running、Stopped
	Site        string      `json:"site" gorm:"type:varchar(50);index"`                                   // 应用站点（扩展字段，可留空）
	Department  string      `json:"department" gorm:"type:varchar(100);index"`                            // 部门（关联到 organizations.unit_code）
	Description string      `json:"description" gorm:"type:text"`                                         // 应用功能用途描述和备注信息
	OnlineAt    *time.Time  `json:"onlineAt" gorm:"type:datetime;index"`                                  // 应用上线时间
	OfflineAt   *time.Time  `json:"offlineAt" gorm:"type:datetime"`                                       // 应用下线时间
	GitURL      string      `json:"gitUrl" gorm:"type:varchar(500)"`                                      // Git地址
	OpsOwners   StringArray `json:"opsOwners" gorm:"type:json"`                                           // 运维负责人(多选)
	TestOwners  StringArray `json:"testOwners" gorm:"type:json"`                                          // 测试负责人(多选)
	DevOwners   StringArray `json:"devOwners" gorm:"type:json"`                                           // 研发负责人(多选)
	CreatedAt   time.Time   `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt   time.Time   `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (Application) TableName() string {
	return "applications"
}

// CreateApplicationRequest 创建应用请求
type CreateApplicationRequest struct {
	Org         string      `json:"org"`
	LineOfBiz   string      `json:"lineOfBiz"`
	Name        string      `json:"name" binding:"required"`
	IsCritical  bool        `json:"isCritical"`
	SrvType     string      `json:"srvType" binding:"required"`
	VirtualTech string      `json:"virtualTech"`
	DevLanguage string      `json:"devLanguage"`
	Status      string      `json:"status"`
	Site        string      `json:"site"`
	Department  string      `json:"department"`
	Description string      `json:"description"`
	OnlineAt    *time.Time  `json:"onlineAt"`
	OfflineAt   *time.Time  `json:"offlineAt"`
	GitURL      string      `json:"gitUrl"`
	OpsOwners   StringArray `json:"opsOwners"`
	TestOwners  StringArray `json:"testOwners"`
	DevOwners   StringArray `json:"devOwners"`
}

// UpdateApplicationRequest 更新应用请求
type UpdateApplicationRequest struct {
	Org         string      `json:"org"`
	LineOfBiz   string      `json:"lineOfBiz"`
	Name        string      `json:"name" binding:"required"`
	IsCritical  bool        `json:"isCritical"`
	SrvType     string      `json:"srvType" binding:"required"`
	VirtualTech string      `json:"virtualTech"`
	DevLanguage string      `json:"devLanguage"`
	Status      string      `json:"status"`
	Site        string      `json:"site"`
	Department  string      `json:"department"`
	Description string      `json:"description"`
	OnlineAt    *time.Time  `json:"onlineAt"`
	OfflineAt   *time.Time  `json:"offlineAt"`
	GitURL      string      `json:"gitUrl"`
	OpsOwners   StringArray `json:"opsOwners"`
	TestOwners  StringArray `json:"testOwners"`
	DevOwners   StringArray `json:"devOwners"`
}

// ParamTemplate 参数模板（按语言）
type ParamTemplate struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Language    string    `json:"language" gorm:"column:language;type:varchar(50);index:idx_lang_ver,unique;not null"`
	VersionName string    `json:"versionName" gorm:"column:version_name;type:varchar(100);index:idx_lang_ver,unique;not null"`
	Description string    `json:"description" gorm:"column:description;type:varchar(255);default:''"`
	IsDefault   bool      `json:"isDefault" gorm:"column:is_default;type:boolean;default:false;index"`
	Content     string    `json:"content" gorm:"column:content;type:text;not null"` // JSON 对象字符串 {key:value}
	CreatedAt   time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (ParamTemplate) TableName() string {
	return "param_templates"
}
