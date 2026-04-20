package auth

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	authService "github.com/fisker086/keyops/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthHandler struct {
	service       *authService.AuthService
	settingRepo   *repository.SettingRepository
	roleRepo *repository.RoleRepository
}

func NewAuthHandler(service *authService.AuthService, settingRepo *repository.SettingRepository, roleRepo *repository.RoleRepository) *AuthHandler {
	return &AuthHandler{
		service:     service,
		settingRepo: settingRepo,
		roleRepo:    roleRepo,
	}
}

// Register 用户注册
func (h *AuthHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	user, err := h.service.Register(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(user))
}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 获取客户端IP和UserAgent
	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	resp, err := h.service.Login(&req, clientIP, userAgent)
	if err != nil {
		c.JSON(http.StatusUnauthorized, model.Error(401, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(resp))
}

// Logout 用户登出
func (h *AuthHandler) Logout(c *gin.Context) {
	// 从上下文获取用户ID（由中间件设置）
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Error(401, "未登录"))
		return
	}

	if err := h.service.Logout(userID.(string)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"message": "登出成功",
	}))
}

// GetCurrentUser 获取当前登录用户信息
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	// 从上下文获取用户ID（由中间件设置）
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Error(401, "未登录"))
		return
	}

	user, err := h.service.GetUserByID(userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "用户不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(user))
}

// GetMyPermissions 返回当前登录用户的分组与主机权限（Web 终端等场景；不暴露他人数据）
func (h *AuthHandler) GetMyPermissions(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Error(401, "未登录"))
		return
	}

	userWithPermissions, err := h.service.GetUserWithGroupsAndHosts(userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "用户不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(userWithPermissions))
}

// GetPlatformLoginRecords 获取平台登录记录
func (h *AuthHandler) GetPlatformLoginRecords(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	userID := c.Query("userId") // 管理员可以查看所有用户的登录记录

	// 非管理员只能查看自己的记录
	role, _ := c.Get("role")
	if role != "admin" {
		currentUserID, _ := c.Get("userID")
		userID = currentUserID.(string)
	}

	records, total, err := h.service.GetPlatformLoginRecords(page, pageSize, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(model.PlatformLoginRecordsResponse{
		Records: records,
		Total:   total,
	}))
}

// GetUsers 获取用户列表（用于黑名单选择）
func (h *AuthHandler) GetUsers(c *gin.Context) {
	users, err := h.service.GetAllUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取用户列表失败"))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"users": users,
	}))
}

// ===== User Management Methods (Admin Only) =====

// GetUsersWithPagination 分页获取用户列表（管理员功能）
func (h *AuthHandler) GetUsersWithPagination(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	keyword := c.Query("keyword")

	users, total, err := h.service.GetUsersWithPagination(page, pageSize, keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取用户列表失败"))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"users":    users,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	}))
}

// GetUsersWithUserGroups 分页获取用户列表及其所属用户组（管理员功能）
func (h *AuthHandler) GetUsersWithRoles(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	keyword := c.Query("keyword")

	users, total, err := h.service.GetUsersWithPagination(page, pageSize, keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取用户列表失败"))
		return
	}

	// 为每个用户获取其所属的角色
	type UserWithRoles struct {
		model.User
		Roles []model.Role `json:"roles"`
	}

	usersWithRoles := make([]UserWithRoles, 0, len(users))
	for _, user := range users {
		// 获取用户所属的角色
		roles, err := h.roleRepo.GetRolesByUserID(user.ID)
		if err != nil {
			// 如果获取失败，返回空数组
			roles = []model.Role{}
		}

		usersWithRoles = append(usersWithRoles, UserWithRoles{
			User:  user,
			Roles: roles,
		})
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"users":    usersWithRoles,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	}))
}

// CreateUserByAdmin 创建新用户（管理员功能）
func (h *AuthHandler) CreateUserByAdmin(c *gin.Context) {
	var req struct {
		model.RegisterRequest
		Role           string  `json:"role" binding:"required"`
		AuthMethod     string  `json:"authMethod"`
		OrganizationID *string `json:"organizationId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 默认认证方式为password
	if req.AuthMethod == "" {
		req.AuthMethod = "password"
	}

	// 验证认证方式
	if req.AuthMethod != "password" && req.AuthMethod != "publickey" {
		c.JSON(http.StatusBadRequest, model.Error(400, "认证方式必须是: password 或 publickey"))
		return
	}

	user, err := h.service.CreateUser(&req.RegisterRequest, req.Role, req.AuthMethod, req.OrganizationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(user))
}

// UpdateUserByAdmin 更新用户信息（管理员功能）
func (h *AuthHandler) UpdateUserByAdmin(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		FullName            string  `json:"fullName"`
		Email               string  `json:"email"`
		ExpiresAt           *string `json:"expiresAt"` // ISO 8601 格式的时间字符串
		AutoDisableOnExpiry *bool   `json:"autoDisableOnExpiry"`
		OrganizationID      *string `json:"organizationId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 基本信息更新
	if err := h.service.UpdateUserInfo(userID, req.FullName, req.Email, req.OrganizationID); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 更新过期相关字段
	if err := h.service.UpdateUserExpiration(userID, req.ExpiresAt, req.AutoDisableOnExpiry); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"message": "更新成功",
	}))
}

// UpdateUserRole 更新用户角色（管理员功能）
func (h *AuthHandler) UpdateUserRole(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		Role string `json:"role" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	if err := h.service.UpdateUserRole(userID, req.Role); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"message": "更新角色成功",
	}))
}

// UpdateUserStatus 更新用户状态（管理员功能）
func (h *AuthHandler) UpdateUserStatus(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	if err := h.service.UpdateUserStatus(userID, req.Status); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"message": "更新状态成功",
	}))
}

// DeleteUser 删除用户（管理员功能）
func (h *AuthHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("id")

	// 不允许删除自己
	currentUserID, _ := c.Get("userID")
	if currentUserID.(string) == userID {
		c.JSON(http.StatusBadRequest, model.Error(400, "不能删除自己"))
		return
	}

	if err := h.service.DeleteUser(userID); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"message": "删除成功",
	}))
}

// ResetUserPassword 重置用户密码（管理员功能）
func (h *AuthHandler) ResetUserPassword(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		Password string `json:"password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	if err := h.service.ResetUserPassword(userID, req.Password); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"message": "重置密码成功",
	}))
}

// InitiateSSO 发起 SSO 登录
func (h *AuthHandler) InitiateSSO(c *gin.Context) {
	// 获取 SSO 配置（从 auth category 读取）
	authSettings, err := h.settingRepo.GetByCategory("auth")
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取SSO配置失败"))
		return
	}

	// 检查 SSO 是否启用
	enabled := false
	var authUrl, clientId, redirectUrl, scopes string

	for _, setting := range authSettings {
		switch setting.Key {
		case "ssoEnabled":
			enabled = (setting.Value == "true")
		case "ssoAuthUrl":
			authUrl = setting.Value
		case "ssoClientId":
			clientId = setting.Value
		case "ssoRedirectUrl":
			redirectUrl = setting.Value
		case "ssoScopes":
			scopes = setting.Value
		}
	}

	if !enabled {
		c.JSON(http.StatusBadRequest, model.Error(400, "SSO 未启用"))
		return
	}

	if authUrl == "" || clientId == "" {
		c.JSON(http.StatusInternalServerError, model.Error(500, "SSO 配置不完整"))
		return
	}

	if scopes == "" {
		scopes = "openid profile email"
	}

	// 生成 state 参数用于防止 CSRF 攻击
	state := uuid.New().String()

	// 构造授权 URL（使用 url.Values 自动处理编码）
	params := url.Values{}
	params.Set("client_id", clientId)
	params.Set("redirect_uri", redirectUrl)
	params.Set("response_type", "code")
	params.Set("state", state)
	if scopes != "" {
		params.Set("scope", scopes)
	}

	authorizationURL := authUrl
	if strings.Contains(authUrl, "?") {
		authorizationURL += "&" + params.Encode()
	} else {
		authorizationURL += "?" + params.Encode()
	}

	fmt.Printf(" [SSO] 生成授权 URL: %s\n", authorizationURL)

	c.JSON(http.StatusOK, model.Success(gin.H{
		"authUrl": authorizationURL,
		"state":   state,
	}))
}

// SSOCallback 处理 SSO 回调
func (h *AuthHandler) SSOCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	errorParam := c.Query("error")

	// 检查是否有错误参数
	if errorParam != "" {
		errorDesc := c.Query("error_description")
		fmt.Printf(" [SSO] 授权失败: %s - %s\n", errorParam, errorDesc)
		// 重定向到登录页并显示错误
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/?error=%s&error_description=%s", errorParam, errorDesc))
		return
	}

	if code == "" {
		fmt.Printf(" [SSO] 缺少授权码\n")
		c.Redirect(http.StatusTemporaryRedirect, "/?error=missing_code")
		return
	}

	// TODO: 验证 state 参数防止 CSRF 攻击
	// 目前先跳过 state 验证，后续可以通过 Redis 或内存缓存实现
	if state == "" {
		fmt.Printf(" [SSO] 缺少 state 参数（安全警告）\n")
	}

	// 获取客户端信息（用于记录登录IP和UserAgent）
	loginIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	// 调用 Service 完成 SSO 登录流程
	loginResp, err := h.service.LoginWithSSO(code, loginIP, userAgent)
	if err != nil {
		fmt.Printf(" [SSO] 登录失败: %v\n", err)
		// 重定向到登录页并显示错误
		errorMsg := url.QueryEscape(err.Error())
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/?error=sso_login_failed&error_description=%s", errorMsg))
		return
	}

	fmt.Printf(" [SSO] 登录成功，重定向到前端\n")

	// 重定向到前端首页，并在 URL 中传递 token
	// 前端需要从 URL 中获取 token 并保存到 localStorage
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/?sso_token=%s", loginResp.Token))
}

// ===== User-Group Permission Management =====

// AssignRolesToUser 给用户分配角色
func (h *AuthHandler) AssignRolesToUser(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		RoleIDs []string `json:"roleIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 获取当前管理员ID
	adminID, _ := c.Get("userID")

	if err := h.service.AssignRolesToUser(userID, req.RoleIDs, adminID.(string)); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"message": "分配角色成功",
	}))
}

// GetUserRoles 获取用户的角色列表
func (h *AuthHandler) GetUserRoles(c *gin.Context) {
	userID := c.Param("id")

	roleIDs, err := h.service.GetUserRoles(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取用户分组失败"))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"roleIds": roleIDs,
	}))
}

// GetUserWithGroups 获取用户及其分组信息
func (h *AuthHandler) GetUserWithGroups(c *gin.Context) {
	userID := c.Param("id")

	userWithGroups, err := h.service.GetUserWithGroups(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "用户不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(userWithGroups))
}

// GetUsersWithGroups 获取所有用户及其分组信息（分页）
func (h *AuthHandler) GetUsersWithGroups(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	keyword := c.Query("keyword")

	users, total, err := h.service.GetUsersWithGroups(page, pageSize, keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取用户列表失败"))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"users":    users,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	}))
}

// ===== User-Host Permission Management =====

// AssignHostsToUser 给用户分配单个主机权限
func (h *AuthHandler) AssignHostsToUser(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		HostIDs []string `json:"hostIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 获取当前管理员ID
	adminID, _ := c.Get("userID")

	if err := h.service.AssignHostsToUser(userID, req.HostIDs, adminID.(string)); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"message": "分配主机权限成功",
	}))
}

// GetUserHosts 获取用户的主机权限列表
func (h *AuthHandler) GetUserHosts(c *gin.Context) {
	userID := c.Param("id")

	hostIDs, err := h.service.GetUserHosts(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取用户主机失败"))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"hostIds": hostIDs,
	}))
}

// GetUserWithGroupsAndHosts 获取用户及其分组和主机信息
func (h *AuthHandler) GetUserWithGroupsAndHosts(c *gin.Context) {
	userID := c.Param("id")

	userWithPermissions, err := h.service.GetUserWithGroupsAndHosts(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "用户不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(userWithPermissions))
}

// GetUsersWithGroupsAndHosts 获取所有用户及其分组和主机信息（分页）
func (h *AuthHandler) GetUsersWithGroupsAndHosts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	keyword := c.Query("keyword")

	users, total, err := h.service.GetUsersWithGroupsAndHosts(page, pageSize, keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取用户列表失败"))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"users":    users,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	}))
}

// GetSSOConfig 获取SSO配置状态（公开接口，不需要登录）
// GET /api/auth/sso/config
func (h *AuthHandler) GetSSOConfig(c *gin.Context) {
	// 获取认证配置
	authSettings, err := h.settingRepo.GetByCategory("auth")
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取配置失败"))
		return
	}

	authMethod := ""
	for _, setting := range authSettings {
		if setting.Key == "authMethod" {
			authMethod = setting.Value
			break
		}
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"enabled":    authMethod == "sso",
		"authMethod": authMethod,
	}))
}

// GetAuthMethod 获取当前启用的认证方式（公开接口，不需要登录）
// GET /api/auth/method
func (h *AuthHandler) GetAuthMethod(c *gin.Context) {
	// 获取认证配置
	authSettings, err := h.settingRepo.GetByCategory("auth")
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取配置失败"))
		return
	}

	authMethod := "password" // 默认
	for _, setting := range authSettings {
		if setting.Key == "authMethod" {
			authMethod = setting.Value
			break
		}
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"authMethod": authMethod,
	}))
}

// ===== SSH Key Management =====

// GenerateSSHKey 为用户生成SSH密钥对（管理员或用户本人）
func (h *AuthHandler) GenerateSSHKey(c *gin.Context) {
	userID := c.Param("id")

	// 检查权限：管理员或用户本人
	currentUserID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "admin" && currentUserID.(string) != userID {
		c.JSON(http.StatusForbidden, model.Error(403, "无权操作"))
		return
	}

	err := h.service.GenerateSSHKey(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	// 获取更新后的用户信息
	user, err := h.service.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取用户信息失败"))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"message":     "SSH密钥生成成功",
		"fingerprint": user.SSHKeyFingerprint,
		"publicKey":   user.SSHPublicKey,
		"generatedAt": user.SSHKeyGeneratedAt,
	}))
}

// DeleteSSHKey 删除用户的SSH密钥（管理员或用户本人）
func (h *AuthHandler) DeleteSSHKey(c *gin.Context) {
	userID := c.Param("id")

	// 检查权限：管理员或用户本人
	currentUserID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "admin" && currentUserID.(string) != userID {
		c.JSON(http.StatusForbidden, model.Error(403, "无权操作"))
		return
	}

	err := h.service.DeleteSSHKey(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"message": "SSH密钥已删除",
	}))
}

// DownloadSSHPrivateKey 下载SSH私钥（管理员或用户本人，仅一次）
func (h *AuthHandler) DownloadSSHPrivateKey(c *gin.Context) {
	userID := c.Param("id")

	// 检查权限：管理员或用户本人
	currentUserID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "admin" && currentUserID.(string) != userID {
		c.JSON(http.StatusForbidden, model.Error(403, "无权操作"))
		return
	}

	privateKey, username, err := h.service.GetSSHPrivateKey(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	// 设置下载响应头
	filename := fmt.Sprintf("%s_zjump_ssh_key", username)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/octet-stream")
	c.String(http.StatusOK, privateKey)
}

// UpdateUserAuthMethod 更新用户的SSH认证方式（管理员或用户本人）
func (h *AuthHandler) UpdateUserAuthMethod(c *gin.Context) {
	userID := c.Param("id")

	// 检查权限：管理员或用户本人
	currentUserID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "admin" && currentUserID.(string) != userID {
		c.JSON(http.StatusForbidden, model.Error(403, "无权操作"))
		return
	}

	var req struct {
		AuthMethod string `json:"authMethod" binding:"required,oneof=password publickey"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "认证方式必须是: password 或 publickey"))
		return
	}

	err := h.service.UpdateUserAuthMethod(userID, req.AuthMethod)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"message": "认证方式更新成功",
	}))
}
