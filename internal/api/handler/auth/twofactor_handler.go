package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/twofactor"
)

// TwoFactorHandler 2FA处理器
type TwoFactorHandler struct {
	db           *gorm.DB
	twoFactorSvc *twofactor.TwoFactorService
}

// NewTwoFactorHandler 创建2FA处理器
func NewTwoFactorHandler(db *gorm.DB, twoFactorSvc *twofactor.TwoFactorService) *TwoFactorHandler {
	return &TwoFactorHandler{
		db:           db,
		twoFactorSvc: twoFactorSvc,
	}
}

// GetGlobalConfig 获取全局2FA配置
func (h *TwoFactorHandler) GetGlobalConfig(c *gin.Context) {
	var config model.TwoFactorConfig
	if err := h.db.First(&config).Error; err != nil {
		// 如果不存在，创建默认配置
		config = model.TwoFactorConfig{
			Enabled: false,
			Issuer:  "ZJump",
		}
		h.db.Create(&config)
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    config,
	})
}

// GetGlobalStatus 获取全局2FA状态（普通用户可访问，只读）
func (h *TwoFactorHandler) GetGlobalStatus(c *gin.Context) {
	var config model.TwoFactorConfig
	if err := h.db.First(&config).Error; err != nil {
		// 如果不存在，返回默认配置
		config = model.TwoFactorConfig{
			Enabled: false,
			Issuer:  "ZJump",
		}
	}

	// 只返回状态信息，不包含敏感配置
	status := struct {
		Enabled bool `json:"enabled"`
	}{
		Enabled: config.Enabled,
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    status,
	})
}

// UpdateGlobalConfig 更新全局2FA配置
func (h *TwoFactorHandler) UpdateGlobalConfig(c *gin.Context) {
	var req struct {
		Enabled bool   `json:"enabled"`
		Issuer  string `json:"issuer"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "请求参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// 添加调试日志
	fmt.Printf("收到2FA配置更新请求: enabled=%v, issuer=%s\n", req.Enabled, req.Issuer)

	var config model.TwoFactorConfig
	if err := h.db.First(&config).Error; err != nil {
		// 如果不存在，创建新配置
		config = model.TwoFactorConfig{
			Issuer: req.Issuer,
		}
	}

	config.Enabled = req.Enabled
	if req.Issuer != "" {
		config.Issuer = req.Issuer
	}

	if err := h.db.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "更新配置失败",
			Data:    nil,
		})
		return
	}

	// 注意：关闭全局2FA不再清空用户个人2FA设置，避免意外重置

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "配置更新成功",
		Data:    config,
	})
}

// GetUserStatus 获取用户2FA状态
func (h *TwoFactorHandler) GetUserStatus(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Response{
			Code:    http.StatusUnauthorized,
			Message: "未授权",
			Data:    nil,
		})
		return
	}

	var user model.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Code:    http.StatusNotFound,
			Message: "用户不存在",
			Data:    nil,
		})
		return
	}

	status := model.TwoFactorStatus{
		Enabled:    user.TwoFactorEnabled,
		VerifiedAt: user.TwoFactorVerifiedAt,
	}

	// 如果用户已启用2FA，返回备用码（仅首次设置时）
	if user.TwoFactorEnabled && user.TwoFactorBackupCodes != "" {
		backupCodes, err := h.twoFactorSvc.DeserializeBackupCodes(user.TwoFactorBackupCodes)
		if err == nil {
			status.BackupCodes = backupCodes
		}
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    status,
	})
}

// SetupTwoFactor 设置2FA
func (h *TwoFactorHandler) SetupTwoFactor(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Response{
			Code:    http.StatusUnauthorized,
			Message: "未授权",
			Data:    nil,
		})
		return
	}

	var user model.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Code:    http.StatusNotFound,
			Message: "用户不存在",
			Data:    nil,
		})
		return
	}

	// 允许任意已认证用户为自己的账户设置2FA

	// 如果已经启用2FA，返回错误
	if user.TwoFactorEnabled {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "2FA已启用",
			Data:    nil,
		})
		return
	}

	// 生成密钥
	secret, err := h.twoFactorSvc.GenerateSecret(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "生成密钥失败",
			Data:    nil,
		})
		return
	}

	// 生成二维码
	qrCode, err := h.twoFactorSvc.GenerateQRCode(user.Username, secret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "生成二维码失败",
			Data:    nil,
		})
		return
	}

	// 生成二维码URL
	_, err = h.twoFactorSvc.GetQRCodeURL(user.Username, secret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "生成二维码失败",
			Data:    nil,
		})
		return
	}

	// 生成备用码
	backupCodes, err := h.twoFactorSvc.GenerateBackupCodes(10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "生成备用码失败",
			Data:    nil,
		})
		return
	}

	response := model.TwoFactorSetupResponse{
		QRCode:      qrCode,
		Secret:      secret,
		BackupCodes: backupCodes,
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    response,
	})
}

// VerifyTwoFactor 验证并启用2FA
func (h *TwoFactorHandler) VerifyTwoFactor(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Response{
			Code:    http.StatusUnauthorized,
			Message: "未授权",
			Data:    nil,
		})
		return
	}

	var req model.TwoFactorSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "请求参数错误",
			Data:    nil,
		})
		return
	}

	var user model.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Code:    http.StatusNotFound,
			Message: "用户不存在",
			Data:    nil,
		})
		return
	}

	// 允许任意已认证用户为自己的账户启用2FA

	// 验证TOTP代码
	valid := h.twoFactorSvc.ValidateCode(req.Secret, req.Code)
	if !valid {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "验证码错误",
			Data:    nil,
		})
		return
	}

	// 生成备用码
	backupCodes, err := h.twoFactorSvc.GenerateBackupCodes(10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "生成备用码失败",
			Data:    nil,
		})
		return
	}

	// 序列化备用码
	backupCodesStr, err := h.twoFactorSvc.SerializeBackupCodes(backupCodes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "序列化备用码失败",
			Data:    nil,
		})
		return
	}

	// 更新用户2FA设置
	now := time.Now()
	user.TwoFactorEnabled = true
	user.TwoFactorSecret = req.Secret
	user.TwoFactorBackupCodes = backupCodesStr
	user.TwoFactorVerifiedAt = &now

	if err := h.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "启用2FA失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "2FA启用成功",
		Data: model.TwoFactorSetupResponse{
			BackupCodes: backupCodes,
		},
	})
}

// DisableTwoFactor 禁用2FA
func (h *TwoFactorHandler) DisableTwoFactor(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Response{
			Code:    http.StatusUnauthorized,
			Message: "未授权",
			Data:    nil,
		})
		return
	}

	var user model.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Code:    http.StatusNotFound,
			Message: "用户不存在",
			Data:    nil,
		})
		return
	}

	// 检查用户权限，只有管理员可以禁用2FA
	if user.Role != "admin" {
		c.JSON(http.StatusForbidden, model.Response{
			Code:    http.StatusForbidden,
			Message: "只有管理员可以禁用2FA",
			Data:    nil,
		})
		return
	}

	user.TwoFactorEnabled = false
	user.TwoFactorSecret = ""
	user.TwoFactorBackupCodes = ""
	user.TwoFactorVerifiedAt = nil

	if err := h.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "禁用2FA失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "2FA已禁用",
		Data:    nil,
	})
}

// ResetUserTwoFactor 重置用户2FA（管理员功能）
func (h *TwoFactorHandler) ResetUserTwoFactor(c *gin.Context) {
	// 检查当前用户是否为管理员
	currentUserID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Response{
			Code:    http.StatusUnauthorized,
			Message: "未授权",
			Data:    nil,
		})
		return
	}

	var currentUser model.User
	if err := h.db.First(&currentUser, "id = ?", currentUserID).Error; err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Code:    http.StatusNotFound,
			Message: "用户不存在",
			Data:    nil,
		})
		return
	}

	if currentUser.Role != "admin" {
		c.JSON(http.StatusForbidden, model.Response{
			Code:    http.StatusForbidden,
			Message: "只有管理员可以重置用户2FA",
			Data:    nil,
		})
		return
	}

	// 获取要重置的用户ID
	targetUserID := c.Param("userId")
	if targetUserID == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "用户ID不能为空",
			Data:    nil,
		})
		return
	}

	// 查找目标用户
	var targetUser model.User
	if err := h.db.First(&targetUser, "id = ?", targetUserID).Error; err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Code:    http.StatusNotFound,
			Message: "目标用户不存在",
			Data:    nil,
		})
		return
	}

	// 重置用户的2FA设置
	targetUser.TwoFactorEnabled = false
	targetUser.TwoFactorSecret = ""
	targetUser.TwoFactorBackupCodes = ""
	targetUser.TwoFactorVerifiedAt = nil

	if err := h.db.Save(&targetUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "重置2FA失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "用户2FA已重置",
		Data:    nil,
	})
}

// VerifyCode 验证2FA代码
func (h *TwoFactorHandler) VerifyCode(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Response{
			Code:    http.StatusUnauthorized,
			Message: "未授权",
			Data:    nil,
		})
		return
	}

	var req model.TwoFactorVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "请求参数错误",
			Data:    nil,
		})
		return
	}

	var user model.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Code:    http.StatusNotFound,
			Message: "用户不存在",
			Data:    nil,
		})
		return
	}

	if !user.TwoFactorEnabled {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "2FA未启用",
			Data:    nil,
		})
		return
	}

	// 验证TOTP代码
	if req.Code != "" && h.twoFactorSvc.ValidateCode(user.TwoFactorSecret, req.Code) {
		c.JSON(http.StatusOK, model.Response{
			Code:    http.StatusOK,
			Message: "验证成功",
			Data:    nil,
		})
		return
	}

	// 验证备用码
	if req.BackupCode != "" && user.TwoFactorBackupCodes != "" {
		backupCodes, err := h.twoFactorSvc.DeserializeBackupCodes(user.TwoFactorBackupCodes)
		if err == nil && h.twoFactorSvc.ValidateBackupCode(backupCodes, req.BackupCode) {
			// 备用码使用后需要移除
			// 这里简化处理，实际应该移除已使用的备用码
			c.JSON(http.StatusOK, model.Response{
				Code:    http.StatusOK,
				Message: "验证成功",
				Data:    nil,
			})
			return
		}
	}

	c.JSON(http.StatusBadRequest, model.Response{
		Code:    http.StatusBadRequest,
		Message: "验证失败",
		Data:    nil,
	})
}

// GetBackupCodes 获取备用码
func (h *TwoFactorHandler) GetBackupCodes(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Response{
			Code:    http.StatusUnauthorized,
			Message: "未授权",
			Data:    nil,
		})
		return
	}

	var user model.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Code:    http.StatusNotFound,
			Message: "用户不存在",
			Data:    nil,
		})
		return
	}

	if !user.TwoFactorEnabled || user.TwoFactorBackupCodes == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "2FA未启用或备用码不存在",
			Data:    nil,
		})
		return
	}

	backupCodes, err := h.twoFactorSvc.DeserializeBackupCodes(user.TwoFactorBackupCodes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "获取备用码失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data: gin.H{
			"backupCodes": backupCodes,
		},
	})
}

// RegenerateBackupCodes 重新生成备用码
func (h *TwoFactorHandler) RegenerateBackupCodes(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Response{
			Code:    http.StatusUnauthorized,
			Message: "未授权",
			Data:    nil,
		})
		return
	}

	var user model.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Code:    http.StatusNotFound,
			Message: "用户不存在",
			Data:    nil,
		})
		return
	}

	if !user.TwoFactorEnabled {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "2FA未启用",
			Data:    nil,
		})
		return
	}

	// 生成新的备用码
	backupCodes, err := h.twoFactorSvc.GenerateBackupCodes(10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "生成备用码失败",
			Data:    nil,
		})
		return
	}

	// 序列化备用码
	backupCodesStr, err := h.twoFactorSvc.SerializeBackupCodes(backupCodes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "序列化备用码失败",
			Data:    nil,
		})
		return
	}

	// 更新用户备用码
	user.TwoFactorBackupCodes = backupCodesStr
	if err := h.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "更新备用码失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "备用码重新生成成功",
		Data: gin.H{
			"backupCodes": backupCodes,
		},
	})
}
