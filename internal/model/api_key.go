package model

import "time"

type ApiKey struct {
	ID          string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Name        string     `json:"name" gorm:"type:varchar(100);not null"`
	Key         string     `json:"-" gorm:"uniqueIndex;type:varchar(255);not null"`
	KeyPrefix   string     `json:"keyPrefix" gorm:"type:varchar(20);not null"`
	UserID      string     `json:"userId" gorm:"type:varchar(36);not null;index"`
	Permissions []string   `json:"permissions" gorm:"type:json;serializer:json"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty" gorm:"type:timestamp"`
	IsActive    bool       `json:"isActive" gorm:"type:boolean;default:true;index"`
	TrustLevel  string     `json:"trustLevel" gorm:"type:varchar(20);default:'LOW';index"`
	LastUsedAt  *time.Time `json:"lastUsedAt,omitempty" gorm:"type:timestamp"`
	CreatedAt   time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`

	User User `json:"-" gorm:"foreignKey:UserID;references:ID"`
}

func (ApiKey) TableName() string {
	return "api_keys"
}
