package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// Crypto 加密工具
type Crypto struct {
	aesKey []byte // AES-256加密密钥（32字节）
}

// NewCrypto 创建加密工具实例
// jwtSecret: JWT签名密钥（建议64字节或更长）
// AES-256加密密钥会自动从此密钥提取前32字节用于加密敏感数据
func NewCrypto(jwtSecret string) *Crypto {
	// 处理JWT密钥
	jwtKey := []byte(jwtSecret)
	if len(jwtKey) == 0 {
		// 如果没有配置，使用默认值（64字节，仅用于开发环境）
		jwtKey = []byte("DdzI7wyean0JDT86fIEY+XEPKa+swZRkAlDUojBhnUQUta4KY/EG3JnnI6mDSrxV")
	}

	// 从jwt_secret提取32字节用于AES-256加密
	aesKey := extract32BytesForAES(jwtKey)

	// 验证AES密钥长度（必须是32字节）
	if len(aesKey) != 32 {
		// 如果长度不对，使用默认值（仅用于开发环境）
		aesKey = []byte("zjump-aes-key-32bytes-needed!!!!")
	}

	return &Crypto{
		aesKey: aesKey,
	}
}

// extract32BytesForAES 从JWT密钥提取32字节用于AES-256加密
// 策略：
//   - 如果密钥 >= 32字节：取前32字节（推荐，JWT密钥应该更长更安全）
//   - 如果密钥 < 32字节：使用SHA256哈希转换为32字节
func extract32BytesForAES(key []byte) []byte {
	if len(key) >= 32 {
		// 如果密钥长度 >= 32字节，取前32字节
		return key[:32]
	}

	// 如果长度不足32字节，使用SHA256哈希转换为32字节
	hash := sha256.Sum256(key)
	return hash[:]
}

// Encrypt 加密数据
func (c *Crypto) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		return "", err
	}

	// 使用GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 生成nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// 加密
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Base64编码
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密数据
func (c *Crypto) Decrypt(encryptedText string) (string, error) {
	// Base64解码
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// IsEncrypted 检查字符串是否是加密后的格式（Base64编码的AES-GCM密文）
func (c *Crypto) IsEncrypted(text string) bool {
	// 尝试Base64解码
	decoded, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return false
	}

	// 检查长度是否足够（至少包含nonce）
	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		return false
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return false
	}

	nonceSize := gcm.NonceSize()
	return len(decoded) > nonceSize
}

