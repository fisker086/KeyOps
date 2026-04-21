package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/auth"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	pkgconfig "github.com/fisker086/keyops/pkg/config"
	"github.com/fisker086/keyops/pkg/sshkey"
	"github.com/fisker086/keyops/pkg/twofactor"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// JWT Claims
type Claims struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type AuthService struct {
	repo             *repository.UserRepository
	settingRepo      *repository.SettingRepository
	TwoFactorSvc     *twofactor.TwoFactorService
	jwtSecret        []byte // JWT签名密钥
	aesKey           []byte // AES-256加密密钥（32字节）
	adminWhitelist []string // 小写邮箱列表，来自 security.admin_whitelist 或 ADMIN_WHITELIST
}

// NewAuthService 创建认证服务
// jwtSecret: JWT签名密钥（建议64字节或更长，更安全）
// adminWhitelistRaw: 管理员白名单
// AES-256加密密钥会自动从此密钥提取前32字节用于加密SSH私钥等敏感数据
// 认证方式覆盖见 pkg/config.SecurityAuthMethodOverride()（AUTH_METHOD / security.auth_method）
func NewAuthService(repo *repository.UserRepository, settingRepo *repository.SettingRepository, jwtSecret string, adminWhitelistRaw string) *AuthService {
	// 处理JWT密钥
	jwtKey := []byte(jwtSecret)
	if len(jwtKey) == 0 {
		// 如果没有配置，使用默认值（64字节，仅用于开发环境）
		jwtKey = []byte("DdzI7wyean0JDT86fIEY+XEPKa+swZRkAlDUojBhnUQUta4KY/EG3JnnI6mDSrxV")
	}

	// 从jwt_secret提取32字节用于AES-256加密
	// - 如果jwt_secret >= 32字节：取前32字节（推荐，JWT密钥应该更长更安全）
	// - 如果jwt_secret < 32字节：使用SHA256哈希转换为32字节
	aesKey := extract32BytesForAES(jwtKey)

	// 验证AES密钥长度（必须是32字节）
	if len(aesKey) != 32 {
		// 如果长度不对，使用默认值（仅用于开发环境）
		aesKey = []byte("zjump-aes-key-32bytes-needed!!!!")
	}

	return &AuthService{
		repo:           repo,
		settingRepo:    settingRepo,
		TwoFactorSvc:   twofactor.NewTwoFactorService("ZJump"),
		jwtSecret:      jwtKey,
		aesKey:         aesKey,
		adminWhitelist: parseAdminWhitelist(adminWhitelistRaw),
	}
}

// parseAdminWhitelist 将逗号分隔的邮箱拆成小写条目；不含 @ 的项忽略（避免误配用户名）。
func parseAdminWhitelist(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && strings.Contains(p, "@") {
			out = append(out, strings.ToLower(p))
		}
	}
	return out
}

func (s *AuthService) isAdminEmailWhitelisted(email string) bool {
	if len(s.adminWhitelist) == 0 {
		return false
	}
	e := strings.ToLower(strings.TrimSpace(email))
	if e == "" {
		return false
	}
	for _, entry := range s.adminWhitelist {
		if entry == e {
			return true
		}
	}
	return false
}

// applyAdminWhitelistOnLogin 密码/LDAP 登录成功后：邮箱在白名单则提升为 admin（仅提升，不写库失败则回滚内存角色）
func (s *AuthService) applyAdminWhitelistOnLogin(user *model.User) {
	if user == nil || !s.isAdminEmailWhitelisted(user.Email) || user.Role == "admin" {
		return
	}
	prev := user.Role
	user.Role = "admin"
	if err := s.repo.UpdateUser(user); err != nil {
		fmt.Printf(" [Login] admin_whitelist 更新角色失败: %v\n", err)
		user.Role = prev
	}
}

// extract32BytesForAES 从JWT密钥提取32字节用于AES-256加密
// 策略：
//   - 如果密钥 >= 32字节：取前32字节（推荐，JWT密钥应该更长更安全）
//   - 如果密钥 < 32字节：使用SHA256哈希转换为32字节
func extract32BytesForAES(key []byte) []byte {
	if len(key) >= 32 {
		// 如果密钥长度 >= 32字节，取前32字节
		// 这样即使JWT密钥是64字节或更长，也能安全地提取前32字节用于AES
		return key[:32]
	}

	// 如果长度不足32字节，使用SHA256哈希转换为32字节
	hash := sha256.Sum256(key)
	return hash[:]
}

// Register 用户注册
func (s *AuthService) Register(req *model.RegisterRequest) (*model.User, error) {
	// 检查用户名是否已存在
	if _, err := s.repo.FindUserByUsername(req.Username); err != nil {
		// 如果是记录不存在错误，说明用户名可用，继续
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("检查用户名失败: %w", err)
		}
	} else {
		// 用户已存在
		return nil, errors.New("用户名已存在")
	}

	// 检查邮箱是否已存在
	if req.Email != "" {
		if _, err := s.repo.FindUserByEmail(req.Email); err != nil {
			// 如果是记录不存在错误，说明邮箱可用，继续
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("检查邮箱失败: %w", err)
			}
		} else {
			// 邮箱已被使用
			return nil, errors.New("邮箱已被使用")
		}
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	// 创建用户
	user := &model.User{
		ID:       uuid.New().String(),
		Username: req.Username,
		Password: string(hashedPassword),
		Email:    req.Email,
		FullName: req.FullName,
		Role:     "user", // 默认角色
		Status:   "active",
	}

	if err := s.repo.CreateUser(user); err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	return user, nil
}

// Login 用户登录（支持账户密码、LDAP、SSO）
// 优先尝试数据库用户认证，如果失败再根据authMethod配置尝试LDAP或SSO
// 这样即使选择了LDAP或SSO作为主要认证方式，数据库管理员账户仍然可以登录
func (s *AuthService) Login(req *model.LoginRequest, loginIP, userAgent string) (*model.LoginResponse, error) {
	// 获取认证配置（从auth category读取）
	authSettings, _ := s.settingRepo.GetByCategory("auth")

	// 与 GetAuthMethods 一致：config/环境覆盖优先，否则数据库 authMethod
	authMethod := s.getSettingValue(authSettings, "authMethod", "password")
	if o := pkgconfig.SecurityAuthMethodOverride(); o != "" {
		authMethod = o
	}

	var user *model.User
	var err error

	// 优先尝试数据库用户认证（确保管理员账户始终可用）
	user, err = s.authenticateWithPassword(req.Username, req.Password)
	if err == nil && user != nil {
		// 数据库用户认证成功
	} else {
		// 数据库用户认证失败，根据authMethod配置尝试其他认证方式
		switch authMethod {
		case "ldap":
			// LDAP认证
			user, err = s.authenticateWithLDAP(req.Username, req.Password, authSettings)
			if err != nil {
				return nil, fmt.Errorf("LDAP认证失败: %w", err)
			}
		case "sso":
			// SSO认证（不支持密码登录，需要通过 OAuth2 流程）
			return nil, errors.New("SSO认证需要通过授权流程，请使用SSO登录按钮。数据库用户请使用数据库账户登录")
		default:
			// password模式，数据库用户认证失败就直接返回错误
			if err != nil {
				return nil, err
			}
			return nil, errors.New("用户名或密码错误")
		}
	}

	// 与 SSO 一致：admin_whitelist 按邮箱提升管理员（密码登录、LDAP 登录均生效）
	s.applyAdminWhitelistOnLogin(user)

	// 检查用户是否过期
	if user.ExpiresAt != nil && user.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("账号已过期，请联系管理员")
	}

	// 检查全局2FA配置
	var globalConfig model.TwoFactorConfig
	if err := s.repo.GetDB().First(&globalConfig).Error; err == nil && globalConfig.Enabled {
		// 全局2FA已启用，检查用户是否已设置2FA
		if !user.TwoFactorEnabled {
			// 用户未设置2FA，允许登录但标记需要设置2FA
			// 生成临时token，让用户能够进入系统设置2FA
			token, err := s.GenerateToken(user)
			if err != nil {
				return nil, fmt.Errorf("生成Token失败: %w", err)
			}

			// 更新最后登录时间
			now := time.Now()
			if err := s.repo.UpdateUserLastLogin(user.ID, now, loginIP); err != nil {
				fmt.Printf("更新最后登录时间失败: %v\n", err)
			}

			// 创建平台登录记录
			loginRecord := &model.PlatformLoginRecord{
				ID:        uuid.New().String(),
				UserID:    user.ID,
				Username:  user.Username,
				LoginIP:   loginIP,
				UserAgent: userAgent,
				LoginTime: now,
				Status:    "active",
			}
			if err := s.repo.CreatePlatformLoginRecord(loginRecord); err != nil {
				fmt.Printf("创建平台登录记录失败: %v\n", err)
			}

			return &model.LoginResponse{
				Token:               token,
				User:                *user,
				RequiresTwoFactor:   false, // 允许登录
				TwoFactorEnabled:    false,
				NeedsTwoFactorSetup: true, // 标记需要设置2FA
			}, nil
		}

		// 用户已启用2FA，需要验证2FA代码
		if req.TwoFactorCode == "" && req.BackupCode == "" {
			return &model.LoginResponse{
				RequiresTwoFactor: true,
				TwoFactorEnabled:  true,
				User:              *user,
			}, nil
		}

		// 检查2FA配置是否完整
		if user.TwoFactorSecret == "" && user.TwoFactorBackupCodes == "" {
			// 2FA已启用但配置不完整，允许登录但标记需要重新设置2FA
			token, err := s.GenerateToken(user)
			if err != nil {
				return nil, fmt.Errorf("生成Token失败: %w", err)
			}

			now := time.Now()
			if err := s.repo.UpdateUserLastLogin(user.ID, now, loginIP); err != nil {
				fmt.Printf("更新最后登录时间失败: %v\n", err)
			}

			loginRecord := &model.PlatformLoginRecord{
				ID:        uuid.New().String(),
				UserID:    user.ID,
				Username:  user.Username,
				LoginIP:   loginIP,
				UserAgent: userAgent,
				LoginTime: now,
				Status:    "active",
			}
			if err := s.repo.CreatePlatformLoginRecord(loginRecord); err != nil {
				fmt.Printf("创建平台登录记录失败: %v\n", err)
			}

			return &model.LoginResponse{
				Token:               token,
				User:                *user,
				RequiresTwoFactor:   false,
				TwoFactorEnabled:    true,
				NeedsTwoFactorSetup: true, // 标记需要重新设置2FA
			}, nil
		}

		// 验证2FA代码
		if !s.validateTwoFactorCode(user, req.TwoFactorCode, req.BackupCode) {
			return nil, errors.New("2FA验证失败")
		}
	} else if user.TwoFactorEnabled {
		// 用户个人启用了2FA，需要验证
		if req.TwoFactorCode == "" && req.BackupCode == "" {
			return &model.LoginResponse{
				RequiresTwoFactor: true,
				TwoFactorEnabled:  true,
				User:              *user,
			}, nil
		}

		// 检查2FA配置是否完整
		if user.TwoFactorSecret == "" && user.TwoFactorBackupCodes == "" {
			// 2FA已启用但配置不完整，允许登录但标记需要重新设置2FA
			token, err := s.GenerateToken(user)
			if err != nil {
				return nil, fmt.Errorf("生成Token失败: %w", err)
			}

			now := time.Now()
			if err := s.repo.UpdateUserLastLogin(user.ID, now, loginIP); err != nil {
				fmt.Printf("更新最后登录时间失败: %v\n", err)
			}

			loginRecord := &model.PlatformLoginRecord{
				ID:        uuid.New().String(),
				UserID:    user.ID,
				Username:  user.Username,
				LoginIP:   loginIP,
				UserAgent: userAgent,
				LoginTime: now,
				Status:    "active",
			}
			if err := s.repo.CreatePlatformLoginRecord(loginRecord); err != nil {
				fmt.Printf("创建平台登录记录失败: %v\n", err)
			}

			return &model.LoginResponse{
				Token:               token,
				User:                *user,
				RequiresTwoFactor:   false,
				TwoFactorEnabled:    true,
				NeedsTwoFactorSetup: true, // 标记需要重新设置2FA
			}, nil
		}

		// 验证2FA代码
		if !s.validateTwoFactorCode(user, req.TwoFactorCode, req.BackupCode) {
			return nil, errors.New("2FA验证失败")
		}
	}

	// 生成 JWT Token
	token, err := s.GenerateToken(user)
	if err != nil {
		return nil, fmt.Errorf("生成Token失败: %w", err)
	}

	// 更新最后登录时间和IP
	now := time.Now()
	if err := s.repo.UpdateUserLastLogin(user.ID, now, loginIP); err != nil {
		// 记录错误但不影响登录
		fmt.Printf("更新最后登录时间失败: %v\n", err)
	}

	// 创建平台登录记录（记录用户登录堡垒机平台，不是连接虚拟机）
	loginRecord := &model.PlatformLoginRecord{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Username:  user.Username,
		LoginIP:   loginIP,
		UserAgent: userAgent,
		LoginTime: now,
		Status:    "active",
	}
	if err := s.repo.CreatePlatformLoginRecord(loginRecord); err != nil {
		// 记录错误但不影响登录
		fmt.Printf(" [Login] 创建平台登录记录失败: %v\n", err)
	}

	return &model.LoginResponse{
		Token: token,
		User:  *user,
	}, nil
}

// authenticateWithPassword 使用密码认证（默认方式）
func (s *AuthService) authenticateWithPassword(username, password string) (*model.User, error) {
	// 查找用户
	user, err := s.repo.FindUserByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户名或密码错误")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 检查用户状态
	if user.Status != "active" {
		return nil, errors.New("用户已被禁用")
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	return user, nil
}

// validateTwoFactorCode 验证2FA代码
func (s *AuthService) validateTwoFactorCode(user *model.User, totpCode, backupCode string) bool {
	// 验证TOTP代码
	if totpCode != "" && s.TwoFactorSvc.ValidateCode(user.TwoFactorSecret, totpCode) {
		return true
	}

	// 验证备用码
	if backupCode != "" && user.TwoFactorBackupCodes != "" {
		backupCodes, err := s.TwoFactorSvc.DeserializeBackupCodes(user.TwoFactorBackupCodes)
		if err == nil && s.TwoFactorSvc.ValidateBackupCode(backupCodes, backupCode) {
			return true
		}
	}

	return false
}

// ValidateTwoFactorCode 公开的2FA验证方法
func (s *AuthService) ValidateTwoFactorCode(user *model.User, totpCode, backupCode string) bool {
	return s.validateTwoFactorCode(user, totpCode, backupCode)
}

// ValidatePassword 验证用户密码
func (s *AuthService) ValidatePassword(user *model.User, password string) error {
	// 检查用户状态
	if user.Status != "active" {
		return errors.New("用户已被禁用")
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return errors.New("密码错误")
	}

	return nil
}

// authenticateWithLDAP 使用 LDAP 认证
func (s *AuthService) authenticateWithLDAP(username, password string, settings []model.Setting) (*model.User, error) {
	// 获取LDAP配置
	ldapServer := s.getSettingValue(settings, "ldapServer", "")
	ldapPortStr := s.getSettingValue(settings, "ldapPort", "389")
	bindDn := s.getSettingValue(settings, "ldapBindDn", "")
	bindPassword := s.getSettingValue(settings, "ldapBindPassword", "")
	baseDn := s.getSettingValue(settings, "ldapBaseDn", "")
	userFilter := s.getSettingValue(settings, "ldapUserFilter", "(uid={username})")
	useTLSStr := s.getSettingValue(settings, "ldapUseTLS", "false")
	skipTLSVerifyStr := s.getSettingValue(settings, "ldapSkipTLSVerify", "false")
	adminGroup := s.getSettingValue(settings, "ldapAdminGroup", "")

	// 检查LDAP是否启用
	ldapEnabledStr := s.getSettingValue(settings, "ldapEnabled", "false")
	if ldapEnabledStr != "true" {
		return nil, fmt.Errorf("LDAP未启用")
	}

	if ldapServer == "" || bindDn == "" || baseDn == "" || bindPassword == "" {
		return nil, errors.New("LDAP配置不完整，请在系统设置中完成LDAP配置")
	}

	// 解析端口
	ldapPort, err := strconv.Atoi(ldapPortStr)
	if err != nil {
		ldapPort = 389 // 默认端口
	}

	// 解析TLS配置
	useTLS := useTLSStr == "true"
	skipTLSVerify := skipTLSVerifyStr == "true"

	// 构建LDAP配置
	ldapConfig := &auth.LDAPConfig{
		Enabled:       true,
		Host:          ldapServer,
		Port:          ldapPort,
		UseSSL:        useTLS,
		BindDN:        bindDn,
		BindPassword:  bindPassword,
		BaseDN:        baseDn,
		UserFilter:    userFilter,
		AdminGroup:    adminGroup,
		SkipTLSVerify: skipTLSVerify,
		AttributeMapping: auth.AttributeMapping{
			UsernameAttribute: s.getSettingValue(settings, "ldapUsernameAttribute", ""),
			EmailAttribute:    s.getSettingValue(settings, "ldapEmailAttribute", ""),
			FullNameAttribute: s.getSettingValue(settings, "ldapFullNameAttribute", ""),
			MemberOfAttribute: s.getSettingValue(settings, "ldapMemberOfAttribute", ""),
		},
	}

	// 创建LDAP认证器
	ldapAuth := auth.NewLDAPAuthenticator(ldapConfig)

	// 执行LDAP认证
	ldapUser, err := ldapAuth.Authenticate(username, password)
	if err != nil {
		return nil, fmt.Errorf("LDAP认证失败: %w", err)
	}

	// LDAP认证成功，创建或更新本地用户
	return s.createOrUpdateUserFromLDAP(ldapUser)
}

// createOrUpdateUserFromLDAP 从LDAP用户信息创建或更新本地用户
func (s *AuthService) createOrUpdateUserFromLDAP(ldapUser *auth.LDAPUser) (*model.User, error) {
	// 查找本地用户（优先通过用户名，其次通过邮箱）
	var user *model.User
	var err error

	// 先通过用户名查找
	user, err = s.repo.FindUserByUsername(ldapUser.Username)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 如果通过用户名没找到，且LDAP用户有邮箱，尝试通过邮箱查找
	if user == nil && ldapUser.Email != "" {
		user, err = s.repo.FindUserByEmail(ldapUser.Email)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("查询用户失败: %w", err)
		}
	}

	// 用户存在，更新信息
	if user != nil {
		// 更新用户信息（保留2FA相关设置）
		if ldapUser.Email != "" {
			user.Email = ldapUser.Email
		}
		if ldapUser.FullName != "" {
			user.FullName = ldapUser.FullName
		}
		// 如果LDAP用户是管理员，更新角色（但保留原有角色，除非LDAP明确标记为管理员）
		if ldapUser.IsAdmin {
			user.Role = "admin"
		}
		// 确保用户状态为active
		if user.Status != "active" {
			user.Status = "active"
		}
		// 注意：不更新TwoFactorEnabled、TwoFactorSecret、TwoFactorBackupCodes等2FA相关字段
		// 这些字段应该由用户自己设置，LDAP同步不应该覆盖

		if err := s.repo.UpdateUser(user); err != nil {
			return nil, fmt.Errorf("更新用户失败: %w", err)
		}

		return user, nil
	}

	// 用户不存在，创建新用户
	// LDAP用户不需要密码（通过LDAP认证），生成一个随机密码
	randomPassword := uuid.New().String()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(randomPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	// 确定用户角色
	role := "user"
	if ldapUser.IsAdmin {
		role = "admin"
	}

	user = &model.User{
		ID:       uuid.New().String(),
		Username: ldapUser.Username,
		Password: string(hashedPassword), // LDAP用户不需要使用这个密码
		Email:    ldapUser.Email,
		FullName: ldapUser.FullName,
		Role:     role,
		Status:   "active",
	}

	if err := s.repo.CreateUser(user); err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	return user, nil
}

// getSettingValue 获取配置值，支持默认值
func (s *AuthService) getSettingValue(settings []model.Setting, key, defaultValue string) string {
	for _, setting := range settings {
		if repository.LogicalSettingKey(setting.Category, setting.Key) == key {
			return setting.Value
		}
	}
	return defaultValue
}

// isAuthMethodEnabled 检查认证方式是否启用（已废弃，保留兼容性）
func (s *AuthService) isAuthMethodEnabled(settings []model.Setting, key string) bool {
	for _, setting := range settings {
		if repository.LogicalSettingKey(setting.Category, setting.Key) == key {
			return setting.Value == "true"
		}
	}
	return false
}

// Logout 用户登出
func (s *AuthService) Logout(userID string) error {
	return s.repo.UpdatePlatformLoginRecordLogoutByUser(userID)
}

// GenerateToken 生成 JWT Token
func (s *AuthService) GenerateToken(user *model.User) (string, error) {
	// 设置过期时间为7天（168小时），适合堡垒机场景
	// 用户一般需要长时间操作服务器，不应频繁重新登录
	expirationTime := time.Now().Add(7 * 24 * time.Hour)

	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "zjump",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// ValidateToken 验证 JWT Token
func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("无效的Token")
}

// GetPlatformLoginRecords 获取平台登录记录
func (s *AuthService) GetPlatformLoginRecords(page, pageSize int, userID string) ([]model.PlatformLoginRecord, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	return s.repo.FindPlatformLoginRecords(page, pageSize, userID)
}

// GetUserByID 根据ID获取用户
func (s *AuthService) GetUserByID(userID string) (*model.User, error) {
	return s.repo.FindUserByID(userID)
}

// GetUserByUsername 根据用户名获取用户
func (s *AuthService) GetUserByUsername(username string) (*model.User, error) {
	return s.repo.FindUserByUsername(username)
}

// GetAllUsers 获取所有用户列表（用于黑名单选择）
func (s *AuthService) GetAllUsers() ([]model.User, error) {
	return s.repo.FindAllUsers()
}

// ===== User Management Methods =====

// GetUsersWithPagination 分页获取用户列表
func (s *AuthService) GetUsersWithPagination(page, pageSize int, keyword string) ([]model.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	return s.repo.FindAllUsersWithPagination(page, pageSize, keyword)
}

// CreateUser 创建新用户（管理员功能）
func (s *AuthService) CreateUser(req *model.RegisterRequest, role string, authMethod string, organizationID *string) (*model.User, error) {
	// 检查用户名是否已存在
	if _, err := s.repo.FindUserByUsername(req.Username); err != nil {
		// 如果是记录不存在错误，说明用户名可用，继续
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("检查用户名失败: %w", err)
		}
	} else {
		// 用户已存在
		return nil, errors.New("用户名已存在")
	}

	// 检查邮箱是否已存在
	if req.Email != "" {
		if _, err := s.repo.FindUserByEmail(req.Email); err != nil {
			// 如果是记录不存在错误，说明邮箱可用，继续
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("检查邮箱失败: %w", err)
			}
		} else {
			// 邮箱已被使用
			return nil, errors.New("邮箱已被使用")
		}
	}

	// 验证部门ID是否存在（如果提供了）
	if organizationID != nil && *organizationID != "" {
		// 这里可以添加验证部门是否存在的逻辑
		// 暂时先不验证，允许后续通过外键约束来保证数据完整性
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	// 验证角色
	if role != "admin" && role != "user" {
		role = "user"
	}

	// 验证认证方式
	if authMethod == "" || (authMethod != "password" && authMethod != "publickey") {
		authMethod = "password"
	}

	// 创建用户
	user := &model.User{
		ID:             uuid.New().String(),
		Username:       req.Username,
		Password:       string(hashedPassword),
		Email:          req.Email,
		FullName:       req.FullName,
		Role:           role,
		Status:         "active",
		AuthMethod:     authMethod,
		OrganizationID: organizationID,
	}

	if err := s.repo.CreateUser(user); err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	return user, nil
}

// UpdateUserInfo 更新用户信息（管理员功能）
func (s *AuthService) UpdateUserInfo(userID string, fullName, email string, organizationID *string) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}

	// 检查邮箱是否被其他用户使用
	if email != "" && email != user.Email {
		if existingUser, err := s.repo.FindUserByEmail(email); err == nil && existingUser.ID != userID {
			return errors.New("邮箱已被使用")
		}
	}

	user.FullName = fullName
	user.Email = email
	// 允许清空部门（organizationID为nil时设置为nil）
	user.OrganizationID = organizationID

	return s.repo.UpdateUser(user)
}

// UpdateUserExpiration 更新用户过期信息（管理员功能）
func (s *AuthService) UpdateUserExpiration(userID string, expiresAt *string, autoDisableOnExpiry *bool) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}

	// 更新过期时间
	if expiresAt != nil {
		if *expiresAt == "" {
			// 空字符串表示永不过期
			user.ExpiresAt = nil
			user.ExpirationWarningSent = false // 重置警告标记
		} else {
			// 解析时间字符串
			t, err := time.Parse(time.RFC3339, *expiresAt)
			if err != nil {
				return fmt.Errorf("无效的时间格式: %v", err)
			}
			user.ExpiresAt = &t
			user.ExpirationWarningSent = false // 重置警告标记
		}
	}

	// 更新自动禁用设置
	if autoDisableOnExpiry != nil {
		user.AutoDisableOnExpiry = *autoDisableOnExpiry
	}

	return s.repo.UpdateUser(user)
}

// UpdateUserRole 更新用户角色（管理员功能）
func (s *AuthService) UpdateUserRole(userID, role string) error {
	// 验证角色
	if role != "admin" && role != "user" {
		return errors.New("无效的角色")
	}

	// 检查用户是否存在
	if _, err := s.repo.FindUserByID(userID); err != nil {
		return errors.New("用户不存在")
	}

	return s.repo.UpdateUserRole(userID, role)
}

// UpdateUserStatus 更新用户状态（管理员功能）
func (s *AuthService) UpdateUserStatus(userID, status string) error {
	// 验证状态
	if status != "active" && status != "inactive" {
		return errors.New("无效的状态")
	}

	// 检查用户是否存在
	if _, err := s.repo.FindUserByID(userID); err != nil {
		return errors.New("用户不存在")
	}

	return s.repo.UpdateUserStatus(userID, status)
}

// DeleteUser 删除用户（管理员功能）
func (s *AuthService) DeleteUser(userID string) error {
	// 检查用户是否存在
	if _, err := s.repo.FindUserByID(userID); err != nil {
		return errors.New("用户不存在")
	}

	return s.repo.DeleteUser(userID)
}

// ResetUserPassword 重置用户密码（管理员功能）
func (s *AuthService) ResetUserPassword(userID, newPassword string) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}

	user.Password = string(hashedPassword)
	return s.repo.UpdateUser(user)
}

// ===== User-Group Permission Methods =====

// AssignRolesToUser 给用户分配角色
func (s *AuthService) AssignRolesToUser(userID string, roleIDs []string, createdBy string) error {
	// 检查用户是否存在
	if _, err := s.repo.FindUserByID(userID); err != nil {
		return errors.New("用户不存在")
	}

	return s.repo.AssignRolesToUser(userID, roleIDs, createdBy)
}

// GetUserRoles 获取用户有权限访问的角色ID列表
func (s *AuthService) GetUserRoles(userID string) ([]string, error) {
	return s.repo.GetUserRoles(userID)
}

// GetUserWithGroups 获取用户及其分组信息
func (s *AuthService) GetUserWithGroups(userID string) (*model.UserWithGroups, error) {
	return s.repo.GetUserWithGroups(userID)
}

// GetUsersWithGroups 获取所有用户及其分组信息（分页）
func (s *AuthService) GetUsersWithGroups(page, pageSize int, keyword string) ([]model.UserWithGroups, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	return s.repo.FindAllUsersWithGroups(page, pageSize, keyword)
}

// ===== User-Host Permission Methods =====

// AssignHostsToUser 给用户分配单个主机权限
func (s *AuthService) AssignHostsToUser(userID string, hostIDs []string, createdBy string) error {
	// 检查用户是否存在
	if _, err := s.repo.FindUserByID(userID); err != nil {
		return errors.New("用户不存在")
	}

	return s.repo.AssignHostsToUser(userID, hostIDs, createdBy)
}

// GetUserHosts 获取用户有权限访问的主机ID列表
func (s *AuthService) GetUserHosts(userID string) ([]string, error) {
	return s.repo.GetUserHosts(userID)
}

// GetUserWithGroupsAndHosts 获取用户及其分组和主机权限信息
func (s *AuthService) GetUserWithGroupsAndHosts(userID string) (*model.UserWithGroups, error) {
	return s.repo.GetUserWithGroupsAndHosts(userID)
}

// GetUsersWithGroupsAndHosts 获取所有用户及其分组和主机信息（分页）
func (s *AuthService) GetUsersWithGroupsAndHosts(page, pageSize int, keyword string) ([]model.UserWithGroups, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	return s.repo.FindAllUsersWithGroupsAndHosts(page, pageSize, keyword)
}

// ===== SSO OAuth2 Methods =====

// SSOUserInfo SSO 用户信息结构（通用格式）
type SSOUserInfo struct {
	Sub       string `json:"sub"`        // 用户唯一标识
	Email     string `json:"email"`      // 邮箱
	Name      string `json:"name"`       // 姓名
	Username  string `json:"username"`   // 用户名
	OpenID    string `json:"open_id"`    // 飞书 OpenID
	UnionID   string `json:"union_id"`   // 飞书 UnionID
	Mobile    string `json:"mobile"`     // 手机号
	AvatarURL string `json:"avatar_url"` // 头像
}



// ExchangeCodeForToken 使用授权码换取访问令牌（入口，分发到各 provider）
func (s *AuthService) ExchangeCodeForToken(code, provider, clientID, clientSecret, tokenURL, redirectURL string) (string, error) {
	p := normalizeSSOProvider(provider)
	fmt.Printf(" [SSO] 开始换取 Token: provider=%s (normalized=%s)\n", provider, p)

	switch p {
	case SSOProviderFeishu, SSOProviderLark:
		return s.exchangeFeishuToken(code, clientID, clientSecret, tokenURL)
	case SSOProviderDingTalk:
		return s.exchangeDingTalkToken(code, clientID, clientSecret, tokenURL)
	case SSOProviderWeCom:
		return s.exchangeWeComCorpToken(clientID, clientSecret, tokenURL)
	}
	return s.exchangeOIDCToken(code, clientID, clientSecret, tokenURL, redirectURL)
}

// GetSSOUserInfo 获取 SSO 用户信息（入口，分发到各 provider）
func (s *AuthService) GetSSOUserInfo(accessToken, provider, userInfoURL, oauthCode, agentID string) (*SSOUserInfo, error) {
	p := normalizeSSOProvider(provider)
	fmt.Printf(" [SSO] 获取用户信息: provider=%s (normalized=%s)\n", provider, p)

	switch p {
	case SSOProviderFeishu, SSOProviderLark:
		return s.getFeishuUserInfo(accessToken, userInfoURL)
	case SSOProviderDingTalk:
		return s.getDingTalkUserInfo(accessToken, userInfoURL)
	case SSOProviderWeCom:
		return s.getWeComUserInfo(accessToken, oauthCode, agentID)
	}
	return s.getOIDCUserInfo(accessToken, userInfoURL)
}

// encodeBasicAuth 编码 Basic Auth（使用 Base64）
func encodeBasicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// CreateOrUpdateSSOUser 创建或更新 SSO 用户
func (s *AuthService) CreateOrUpdateSSOUser(ssoUserInfo *SSOUserInfo) (*model.User, error) {
	fmt.Printf(" [SSO] 创建或更新用户: username=%s, email=%s\n", ssoUserInfo.Username, ssoUserInfo.Email)

	var user *model.User
	var err error

	// 优先通过邮箱查找用户
	if ssoUserInfo.Email != "" {
		user, err = s.repo.FindUserByEmail(ssoUserInfo.Email)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("查询用户失败: %w", err)
		}
	}

	// 如果通过邮箱没找到，尝试通过用户名查找
	if user == nil && ssoUserInfo.Username != "" {
		user, err = s.repo.FindUserByUsername(ssoUserInfo.Username)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("查询用户失败: %w", err)
		}
	}

	// 用户存在，更新信息
	if user != nil {
		fmt.Printf(" [SSO] 用户已存在，更新信息: userID=%s\n", user.ID)

		// 更新用户信息
		if ssoUserInfo.Name != "" {
			user.FullName = ssoUserInfo.Name
		}
		if ssoUserInfo.Email != "" && user.Email == "" {
			user.Email = ssoUserInfo.Email
		}

		// 白名单仅提升为 admin，不因不在名单而降级
		if s.isAdminEmailWhitelisted(ssoUserInfo.Email) && user.Role != "admin" {
			user.Role = "admin"
		}

		if err := s.repo.UpdateUser(user); err != nil {
			return nil, fmt.Errorf("更新用户失败: %w", err)
		}

		return user, nil
	}

	// 用户不存在，创建新用户
	fmt.Printf(" [SSO] 创建新用户\n")

	// 生成随机密码（SSO用户不使用密码登录）
	randomPassword := uuid.New().String()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(randomPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	role := "user"
	if s.isAdminEmailWhitelisted(ssoUserInfo.Email) {
		role = "admin"
	}

	user = &model.User{
		ID:       uuid.New().String(),
		Username: ssoUserInfo.Username,
		Password: string(hashedPassword),
		Email:    ssoUserInfo.Email,
		FullName: ssoUserInfo.Name,
		Role:     role,
		Status:   "active",
	}

	if err := s.repo.CreateUser(user); err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	fmt.Printf(" [SSO] 新用户创建成功: userID=%s, username=%s\n", user.ID, user.Username)
	return user, nil
}

// LoginWithSSO SSO 登录主流程
func (s *AuthService) LoginWithSSO(code, loginIP, userAgent string) (*model.LoginResponse, error) {
	fmt.Printf(" [SSO] 开始 SSO 登录流程\n")

	// 获取 SSO 配置（从 auth category 读取）
	authSettings, err := s.settingRepo.GetByCategory("auth")
	if err != nil {
		return nil, fmt.Errorf("获取SSO配置失败: %w", err)
	}

	// 解析配置（字段名有 sso 前缀）
	provider := s.getSettingValue(authSettings, "ssoProvider", "")
	clientID := s.getSettingValue(authSettings, "ssoClientId", "")
	clientSecret := s.getSettingValue(authSettings, "ssoClientSecret", "")
	tokenURL := s.getSettingValue(authSettings, "ssoTokenUrl", "")
	userInfoURL := s.getSettingValue(authSettings, "ssoUserInfoUrl", "")
	redirectURL := s.getSettingValue(authSettings, "ssoRedirectUrl", "")
	agentID := s.getSettingValue(authSettings, "ssoAgentId", "")

	p := normalizeSSOProvider(provider)
	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return nil, errors.New("SSO配置不完整")
	}
	switch p {
	case SSOProviderWeCom:
		if tokenURL == "" {
			tokenURL = defaultWeComGetTokenURL
		}
	case SSOProviderDingTalk:
		if tokenURL == "" {
			tokenURL = defaultDingTalkTokenURL
		}
		if userInfoURL == "" {
			userInfoURL = defaultDingTalkUserInfoURL
		}
	default:
		if tokenURL == "" || userInfoURL == "" {
			return nil, errors.New("SSO配置不完整")
		}
	}

	// 1. 使用授权码换取访问令牌
	accessToken, err := s.ExchangeCodeForToken(code, provider, clientID, clientSecret, tokenURL, redirectURL)
	if err != nil {
		return nil, fmt.Errorf("获取访问令牌失败: %w", err)
	}

	// 2. 使用访问令牌获取用户信息
	ssoUserInfo, err := s.GetSSOUserInfo(accessToken, provider, userInfoURL, code, agentID)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %w", err)
	}

	// 3. 创建或更新本地用户
	user, err := s.CreateOrUpdateSSOUser(ssoUserInfo)
	if err != nil {
		return nil, fmt.Errorf("创建或更新用户失败: %w", err)
	}

	// 检查用户是否过期
	if user.ExpiresAt != nil && user.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("账号已过期，请联系管理员")
	}

	// 4. 生成 JWT Token
	token, err := s.GenerateToken(user)
	if err != nil {
		return nil, fmt.Errorf("生成Token失败: %w", err)
	}

	// 5. 更新最后登录时间
	now := time.Now()
	if err := s.repo.UpdateUserLastLogin(user.ID, now, loginIP); err != nil {
		fmt.Printf(" [SSO] 更新最后登录时间失败: %v\n", err)
	}

	// 6. 创建平台登录记录
	loginRecord := &model.PlatformLoginRecord{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Username:  user.Username,
		LoginIP:   loginIP,
		UserAgent: userAgent,
		LoginTime: now,
		Status:    "active",
	}
	if err := s.repo.CreatePlatformLoginRecord(loginRecord); err != nil {
		fmt.Printf(" [SSO] 创建平台登录记录失败: %v\n", err)
	}

	fmt.Printf(" [SSO] 登录成功: userID=%s, username=%s\n", user.ID, user.Username)

	return &model.LoginResponse{
		Token: token,
		User:  *user,
	}, nil
}

// ===== SSH Key Management =====

// GenerateSSHKey 为用户生成SSH密钥对
func (s *AuthService) GenerateSSHKey(userID string) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}

	// 使用sshkey包生成密钥对
	keyPair, err := generateSSHKeyPair(2048)
	if err != nil {
		return fmt.Errorf("生成SSH密钥失败: %w", err)
	}

	// 加密私钥（使用AES加密）
	encryptedPrivateKey, err := s.encryptPrivateKey(keyPair.PrivateKey)
	if err != nil {
		return fmt.Errorf("加密私钥失败: %w", err)
	}

	// 更新用户记录
	now := time.Now()
	user.SSHPublicKey = keyPair.PublicKey
	user.SSHPrivateKeyEncrypted = encryptedPrivateKey
	user.SSHKeyFingerprint = keyPair.Fingerprint
	user.SSHKeyGeneratedAt = &now

	if err := s.repo.UpdateUser(user); err != nil {
		return fmt.Errorf("更新用户SSH密钥失败: %w", err)
	}

	return nil
}

// DeleteSSHKey 删除用户的SSH密钥
func (s *AuthService) DeleteSSHKey(userID string) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}

	// 清空SSH密钥相关字段
	user.SSHPublicKey = ""
	user.SSHPrivateKeyEncrypted = ""
	user.SSHKeyFingerprint = ""
	user.SSHKeyGeneratedAt = nil

	// 如果认证方式是publickey，改回password
	if user.AuthMethod == "publickey" {
		user.AuthMethod = "password"
	}

	if err := s.repo.UpdateUser(user); err != nil {
		return fmt.Errorf("删除SSH密钥失败: %w", err)
	}

	return nil
}

// GetSSHPrivateKey 获取用户的SSH私钥（解密后）
func (s *AuthService) GetSSHPrivateKey(userID string) (string, string, error) {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return "", "", fmt.Errorf("用户不存在: %w", err)
	}

	if user.SSHPrivateKeyEncrypted == "" {
		return "", "", errors.New("用户没有SSH私钥")
	}

	// 解密私钥
	privateKey, err := s.decryptPrivateKey(user.SSHPrivateKeyEncrypted)
	if err != nil {
		return "", "", fmt.Errorf("解密私钥失败: %w", err)
	}

	return privateKey, user.Username, nil
}

// UpdateUserAuthMethod 更新用户的认证方式
func (s *AuthService) UpdateUserAuthMethod(userID, authMethod string) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}

	// 如果选择publickey，但没有密钥，返回错误
	if authMethod == "publickey" && user.SSHPublicKey == "" {
		return errors.New("请先生成SSH密钥")
	}

	// 验证认证方式
	if authMethod != "password" && authMethod != "publickey" {
		return errors.New("认证方式必须是: password 或 publickey")
	}

	user.AuthMethod = authMethod

	if err := s.repo.UpdateUser(user); err != nil {
		return fmt.Errorf("更新认证方式失败: %w", err)
	}

	return nil
}

// GetUserPublicKey 获取用户的公钥（用于SSH认证）
func (s *AuthService) GetUserPublicKey(username string) (string, error) {
	user, err := s.repo.FindUserByUsername(username)
	if err != nil {
		return "", fmt.Errorf("用户不存在: %w", err)
	}

	if user.SSHPublicKey == "" {
		return "", errors.New("用户没有配置SSH公钥")
	}

	// 检查认证方式
	if user.AuthMethod != "publickey" && user.AuthMethod != "both" {
		return "", errors.New("用户未启用公钥认证")
	}

	return user.SSHPublicKey, nil
}

// ===== Helper Functions =====

// generateSSHKeyPair 生成SSH密钥对
func generateSSHKeyPair(bitSize int) (*sshkey.KeyPair, error) {
	return sshkey.GenerateRSAKeyPair(bitSize)
}

// encryptPrivateKey 加密私钥
func (s *AuthService) encryptPrivateKey(privateKey string) (string, error) {
	block, err := aes.NewCipher(s.aesKey)
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
	ciphertext := gcm.Seal(nonce, nonce, []byte(privateKey), nil)

	// Base64编码
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptPrivateKey 解密私钥
func (s *AuthService) decryptPrivateKey(encryptedKey string) (string, error) {
	// Base64解码
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedKey)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.aesKey)
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
