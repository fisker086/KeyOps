package twofactor

import (
	"bytes"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"math/big"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/pquerna/otp/totp"
)

// TwoFactorService 双因素认证服务
type TwoFactorService struct {
	issuer string
}

// NewTwoFactorService 创建2FA服务
func NewTwoFactorService(issuer string) *TwoFactorService {
	return &TwoFactorService{
		issuer: issuer,
	}
}

// GenerateSecret 生成2FA密钥
func (s *TwoFactorService) GenerateSecret(username string) (string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.issuer,
		AccountName: username,
		// 使用默认设置：30秒周期，6位数字，SHA1算法
	})
	if err != nil {
		return "", err
	}
	return key.Secret(), nil
}

// GenerateQRCode 生成二维码数据URL
func (s *TwoFactorService) GenerateQRCode(username, secret string) (string, error) {
	// 将 base32 编码的密钥转换为字节数组
	secretBytes, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", err
	}

	// 使用已生成的密钥创建TOTP对象
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.issuer,
		AccountName: username,
		Secret:      secretBytes, // 使用解码后的字节数组
		// 使用默认设置：30秒周期，6位数字，SHA1算法
	})
	if err != nil {
		return "", err
	}

	// 生成二维码图片
	qrCode, err := qr.Encode(key.URL(), qr.M, qr.Auto)
	if err != nil {
		return "", err
	}

	// 缩放二维码以提高清晰度
	qrCode, err = barcode.Scale(qrCode, 200, 200)
	if err != nil {
		return "", err
	}

	// 将二维码转换为PNG图片
	var buf bytes.Buffer
	err = png.Encode(&buf, qrCode)
	if err != nil {
		return "", err
	}

	// 转换为base64数据URL
	base64Str := base64.StdEncoding.EncodeToString(buf.Bytes())
	return "data:image/png;base64," + base64Str, nil
}

// ValidateCode 验证TOTP代码
func (s *TwoFactorService) ValidateCode(secret, code string) bool {
	// 使用最简单的标准验证，与Google Authenticator等应用完全兼容
	return totp.Validate(code, secret)
}

// GenerateCurrentCode 生成当前时间的TOTP代码（用于调试）
func (s *TwoFactorService) GenerateCurrentCode(secret string) (string, error) {
	return totp.GenerateCode(secret, time.Now())
}

// GenerateCodeForTime 生成指定时间的TOTP代码（用于调试）
func (s *TwoFactorService) GenerateCodeForTime(secret string, t time.Time) (string, error) {
	return totp.GenerateCode(secret, t)
}

// GetQRCodeURL 获取二维码的原始URL（用于调试）
func (s *TwoFactorService) GetQRCodeURL(username, secret string) (string, error) {
	// 将 base32 编码的密钥转换为字节数组
	secretBytes, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", err
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.issuer,
		AccountName: username,
		Secret:      secretBytes, // 使用解码后的字节数组
		// 使用默认设置
	})
	if err != nil {
		return "", err
	}
	return key.URL(), nil
}

// GenerateBackupCodes 生成备用码
func (s *TwoFactorService) GenerateBackupCodes(count int) ([]string, error) {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		code, err := s.generateRandomCode(8)
		if err != nil {
			return nil, err
		}
		codes[i] = code
	}
	return codes, nil
}

// ValidateBackupCode 验证备用码
func (s *TwoFactorService) ValidateBackupCode(backupCodes []string, code string) bool {
	for _, backupCode := range backupCodes {
		if backupCode == code {
			return true
		}
	}
	return false
}

// generateRandomCode 生成随机码
func (s *TwoFactorService) generateRandomCode(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}
	return string(result), nil
}

// SerializeBackupCodes 序列化备用码
func (s *TwoFactorService) SerializeBackupCodes(codes []string) (string, error) {
	data, err := json.Marshal(codes)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DeserializeBackupCodes 反序列化备用码
func (s *TwoFactorService) DeserializeBackupCodes(data string) ([]string, error) {
	var codes []string
	err := json.Unmarshal([]byte(data), &codes)
	if err != nil {
		return nil, err
	}
	return codes, nil
}

// GetTimeWindow 获取时间窗口（用于防止重放攻击）
func (s *TwoFactorService) GetTimeWindow() int64 {
	return time.Now().Unix() / 30
}

// IsCodeRecentlyUsed 检查代码是否最近使用过（简单的重放攻击防护）
func (s *TwoFactorService) IsCodeRecentlyUsed(usedCodes map[string]int64, code string, window int64) bool {
	if lastUsed, exists := usedCodes[code]; exists {
		// 如果代码在最近30秒内使用过，拒绝
		return window-lastUsed < 1
	}
	return false
}

// MarkCodeAsUsed 标记代码为已使用
func (s *TwoFactorService) MarkCodeAsUsed(usedCodes map[string]int64, code string, window int64) {
	usedCodes[code] = window
}
