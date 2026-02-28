package model

import "time"

// K8sCluster K8s集群模型
type K8sCluster struct {
	ID               string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Name             string     `json:"name" gorm:"type:varchar(100);uniqueIndex;not null"` // 集群名称（唯一）
	Description      string     `json:"description" gorm:"type:text"`                        // 描述
	
	// 连接配置
	APIServer        string     `json:"apiServer" gorm:"type:varchar(255);not null"`         // API Server URL
	Token            string     `json:"-" gorm:"type:text"`                                  // Bearer Token（不在JSON中暴露）
	Kubeconfig       string     `json:"-" gorm:"type:text"`                                   // Kubeconfig内容（不在JSON中暴露）
	AuthType         string     `json:"authType" gorm:"type:varchar(20);default:'token'"`     // 认证类型: token, kubeconfig
	
	// 集群信息
	Version          string     `json:"version" gorm:"type:varchar(50)"`                      // Kubernetes版本
	Region           string     `json:"region" gorm:"type:varchar(100)"`                    // 区域
	Environment      string     `json:"environment" gorm:"type:varchar(50)"`                 // 环境: dev, test, prod
	
	// 状态和设置
	Status           string     `json:"status" gorm:"type:varchar(20);default:'active';index"` // 状态: active, inactive, error
	DefaultNamespace string     `json:"defaultNamespace" gorm:"type:varchar(100)"`            // 默认命名空间
	
	// 审计和元数据
	CreatedBy        string     `json:"createdBy,omitempty" gorm:"type:varchar(36)"`
	CreatedAt        time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
	LastCheckedAt    *time.Time `json:"lastCheckedAt,omitempty" gorm:"type:timestamp"`        // 最后健康检查时间
}

func (K8sCluster) TableName() string {
	return "k8s_clusters"
}

