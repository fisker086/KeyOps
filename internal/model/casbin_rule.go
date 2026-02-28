package model

// CasbinRule Casbin权限规则表
type CasbinRule struct {
	ID    uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Ptype string `gorm:"type:varchar(100);not null;index" json:"ptype"` // 策略类型：p(策略)或g(角色继承)
	V0    string `gorm:"type:varchar(100);index" json:"v0"`             // subject（用户ID或用户组ID）
	V1    string `gorm:"type:varchar(100);index" json:"v1"`             // object（资源路径）
	V2    string `gorm:"type:varchar(100)" json:"v2"`                   // action（操作：HTTP方法）
	V3    string `gorm:"type:varchar(100);default:''" json:"v3"`
	V4    string `gorm:"type:varchar(100);default:''" json:"v4"`
	V5    string `gorm:"type:varchar(100);default:''" json:"v5"`
}

func (CasbinRule) TableName() string {
	return "casbin_rule"
}

