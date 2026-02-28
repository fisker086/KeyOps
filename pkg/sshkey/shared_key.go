package sshkey

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/fisker086/keyops/pkg/logger"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

// SSHHostKey 数据库中存储的SSH主机密钥模型
type SSHHostKey struct {
	ID          uint      `gorm:"primaryKey;autoIncrement"`
	KeyType     string    `gorm:"type:varchar(20);default:'rsa'"`
	KeyName     string    `gorm:"type:varchar(50);default:'default'"`
	PrivateKey  string    `gorm:"type:text;not null"`
	PublicKey   string    `gorm:"type:text;not null"`
	Fingerprint string    `gorm:"type:varchar(255);not null"`
	KeySize     int       `gorm:"default:2048"`
	Comment     string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (SSHHostKey) TableName() string {
	return "ssh_host_keys"
}

// GetOrGenerateSharedHostKey 从数据库获取或生成共享的SSH host key
// 这个函数确保所有实例使用相同的SSH主机密钥，避免客户端警告
func GetOrGenerateSharedHostKey(db *gorm.DB, keyType, keyName string) (ssh.Signer, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	if keyType == "" {
		keyType = "rsa"
	}
	if keyName == "" {
		keyName = "default"
	}

	// 尝试从数据库获取现有密钥
	// 使用 Find 而不是 First，避免在记录不存在时输出 "record not found" 日志
	var hostKey SSHHostKey
	result := db.Where("key_type = ? AND key_name = ?", keyType, keyName).Find(&hostKey)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to query host key from database: %w", result.Error)
	}

	// 如果找到了密钥，解析并返回
	if result.RowsAffected > 0 {
		signer, err := ssh.ParsePrivateKey([]byte(hostKey.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse shared host key from database: %w", err)
		}
		return signer, nil
	}

	// 密钥不存在，自动生成新密钥（这是正常行为，首次启动时会执行）
	logger.Infof("SSH host key not found in database (key_type=%s, key_name=%s), generating new key...", keyType, keyName)

	// 生成新密钥
	signer, privateKeyPEM, publicKeyStr, fingerprint, err := generateSSHKeyPair(keyType)
	if err != nil {
		return nil, fmt.Errorf("failed to generate SSH key pair: %w", err)
	}

	// 保存到数据库
	newHostKey := SSHHostKey{
		KeyType:     keyType,
		KeyName:     keyName,
		PrivateKey:  privateKeyPEM,
		PublicKey:   publicKeyStr,
		Fingerprint: fingerprint,
		KeySize:     2048,
		Comment:     fmt.Sprintf("Shared SSH host key for multi-instance deployment - Generated at %s", time.Now().Format(time.RFC3339)),
	}

	if err := db.Create(&newHostKey).Error; err != nil {
		return nil, fmt.Errorf("failed to save host key to database: %w", err)
	}

	return signer, nil
}

// generateSSHKeyPair 生成SSH密钥对
func generateSSHKeyPair(keyType string) (ssh.Signer, string, string, string, error) {
	var privateKey *rsa.PrivateKey
	var err error

	switch keyType {
	case "rsa":
		privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, "", "", "", fmt.Errorf("failed to generate RSA key: %w", err)
		}
	default:
		return nil, "", "", "", fmt.Errorf("unsupported key type: %s (currently only 'rsa' is supported)", keyType)
	}

	// 转换为PEM格式
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	privateKeyBytes := pem.EncodeToMemory(privateKeyPEM)

	// 解析为SSH格式
	signer, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("failed to parse private key: %w", err)
	}

	// 获取公钥和指纹
	publicKey := signer.PublicKey()
	publicKeyStr := string(ssh.MarshalAuthorizedKey(publicKey))
	fingerprint := ssh.FingerprintSHA256(publicKey)

	return signer, string(privateKeyBytes), publicKeyStr, fingerprint, nil
}

// DeleteHostKey 删除指定的主机密钥
func DeleteHostKey(db *gorm.DB, keyType, keyName string) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	result := db.Where("key_type = ? AND key_name = ?", keyType, keyName).Delete(&SSHHostKey{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete host key: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("host key not found")
	}

	return nil
}

// ListHostKeys 列出所有主机密钥
func ListHostKeys(db *gorm.DB) ([]SSHHostKey, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	var keys []SSHHostKey
	if err := db.Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("failed to list host keys: %w", err)
	}

	// 不返回私钥内容（安全考虑）
	for i := range keys {
		keys[i].PrivateKey = "[REDACTED]"
	}

	return keys, nil
}

// GetHostKeyFingerprint 获取指定密钥的指纹
func GetHostKeyFingerprint(db *gorm.DB, keyType, keyName string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("database connection is nil")
	}

	var hostKey SSHHostKey
	result := db.Select("fingerprint").Where("key_type = ? AND key_name = ?", keyType, keyName).First(&hostKey)
	if result.Error != nil {
		return "", fmt.Errorf("failed to get host key fingerprint: %w", result.Error)
	}

	return hostKey.Fingerprint, nil
}
