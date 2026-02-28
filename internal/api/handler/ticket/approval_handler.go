package ticket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/fisker086/keyops/internal/approval"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/service/release"
)

// ApprovalHandler 审批处理器
type ApprovalHandler struct {
	db         *gorm.DB
	factory    *approval.Factory
	releaseSvc *release.Service
}

// NewApprovalHandler 创建审批处理器
func NewApprovalHandler(db *gorm.DB, factory *approval.Factory, releaseSvc *release.Service) *ApprovalHandler {
	return &ApprovalHandler{
		db:         db,
		factory:    factory,
		releaseSvc: releaseSvc,
	}
}

// ListApprovals 获取审批列表
func (h *ApprovalHandler) ListApprovals(c *gin.Context) {
	userID := c.Query("user_id")
	status := c.Query("status")
	approvalType := c.Query("type")
	role := c.Query("role") // my: 我的申请, approve: 待我审批, all: 全部

	query := h.db.Model(&model.Approval{})

	// 根据角色过滤
	if role == "my" && userID != "" {
		// 我的申请：显示所有状态的工单（包括已结束的）
		query = query.Where("applicant_id = ?", userID)
	} else if role == "approve" && userID != "" {
		// 查询待当前用户审批的工单 (MySQL兼容方式)
		// approver_ids 存储为 JSON 数组格式，如: ["id1","id2"]
		query = query.Where("status = ? AND JSON_CONTAINS(approver_ids, ?)", model.ApprovalStatusPending, fmt.Sprintf(`"%s"`, userID))
	} else if role == "all" && userID != "" {
		// 全部工单：显示所有与我相关的工单（申请人或审批人）
		query = query.Where("applicant_id = ? OR JSON_CONTAINS(approver_ids, ?)", userID, fmt.Sprintf(`"%s"`, userID))
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if approvalType != "" {
		query = query.Where("type = ?", approvalType)
	}

	var approvals []model.Approval
	if err := query.Order("created_at DESC").Preload("Applicant").Find(&approvals).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取审批列表失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"approvals": approvals,
			"total":     len(approvals),
		},
	})
}

// GetApproval 获取审批详情
func (h *ApprovalHandler) GetApproval(c *gin.Context) {
	id := c.Param("id")

	var approval model.Approval
	if err := h.db.Preload("Applicant").First(&approval, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "审批不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取审批详情失败",
			"error":   err.Error(),
		})
		return
	}

	// 获取审批历史
	var comments []model.ApprovalComment
	h.db.Where("approval_id = ?", id).Order("created_at ASC").Find(&comments)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"approval": approval,
			"comments": comments,
		},
	})
}

// CreateApproval 创建审批
func (h *ApprovalHandler) CreateApproval(c *gin.Context) {
	var req model.Approval

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 验证必填字段
	if req.Title == "" || req.Type == "" || req.ApplicantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "标题、类型、申请人不能为空",
		})
		return
	}

	// 生成ID和时间戳
	req.ID = uuid.New().String()
	now := time.Now()
	req.CreatedAt = now
	req.UpdatedAt = now
	req.Status = model.ApprovalStatusPending

	// 如果没有指定平台，使用内部审批系统
	if req.Platform == "" {
		req.Platform = model.ApprovalPlatformInternal
	}

	// 计算过期时间
	if req.Duration > 0 {
		expiresAt := now.Add(time.Duration(req.Duration) * time.Hour)
		req.ExpiresAt = &expiresAt
	}

	// 处理内部审批系统
	if req.Platform == model.ApprovalPlatformInternal {
		// 从工单配置中读取审批人信息（内部审批系统也需要审批人）
		var config model.ApprovalConfig
		if err := h.db.Where("type = ? AND enabled = ?", string(req.Platform), true).First(&config).Error; err == nil {
			// 解析审批人ID列表
			if config.ApproverUserIDs != "" {
				var approverIDs []string
				if err := json.Unmarshal([]byte(config.ApproverUserIDs), &approverIDs); err == nil {
					req.ApproverIDs = approverIDs
				}
			}
		}

		// 内部审批系统：直接保存到数据库，无需第三方平台
		if err := h.db.Create(&req).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "创建审批申请失败",
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "审批申请创建成功",
			"data":    req,
		})
		return
	}

	// 创建第三方审批单
	provider, ok := h.factory.GetProvider(req.Platform)
	if !ok {
		// 第三方审批平台功能开发中
		var platformName string
		switch req.Platform {
		case model.ApprovalPlatformFeishu:
			platformName = "飞书"
		case model.ApprovalPlatformDingTalk:
			platformName = "钉钉"
		case model.ApprovalPlatformWeChat:
			platformName = "企业微信"
		default:
			platformName = string(req.Platform)
		}

		c.JSON(http.StatusNotImplemented, gin.H{
			"code":    501,
			"message": fmt.Sprintf("%s审批集成功能开发中，敬请期待！", platformName),
			"detail":  fmt.Sprintf("目前 %s 审批平台的集成功能正在开发中，请联系管理员或等待后续版本更新", platformName),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	externalID, err := provider.CreateApproval(ctx, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建外部审批单失败",
			"error":   err.Error(),
		})
		return
	}

	req.ExternalID = externalID

	// 使用默认的外部链接模板
	req.ExternalURL = h.getDefaultExternalURL(req.Platform, externalID)

	// 从工单配置中读取审批人信息
	var config model.ApprovalConfig
	if err := h.db.Where("type = ? AND enabled = ?", string(req.Platform), true).First(&config).Error; err == nil {
		// 解析审批人ID列表
		if config.ApproverUserIDs != "" {
			var approverIDs []string
			if err := json.Unmarshal([]byte(config.ApproverUserIDs), &approverIDs); err == nil {
				req.ApproverIDs = approverIDs
			}
		}
	}

	// 保存到数据库
	if err := h.db.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建审批失败",
			"error":   err.Error(),
		})
		return
	}

	// 添加提交记录
	comment := model.ApprovalComment{
		ID:         uuid.New().String(),
		ApprovalID: req.ID,
		UserID:     req.ApplicantID,
		UserName:   req.ApplicantName,
		Action:     "submit",
		Comment:    "提交审批申请",
		CreatedAt:  now,
	}
	h.db.Create(&comment)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data":    req,
	})
}

// CreateThirdPartyApproval 创建第三方审批实例（从工单创建）
func (h *ApprovalHandler) CreateThirdPartyApproval(c *gin.Context) {
	var req struct {
		TicketID         uint   `json:"ticket_id" binding:"required"`
		ApprovalConfigID string `json:"approval_config_id" binding:"required"`
		ApprovalCode     string `json:"approval_code"`                // 可选，如果提供则覆盖配置中的审批代码
		Platform         string `json:"platform" binding:"required"`  // feishu, dingtalk, wechat
		FormData         string `json:"form_data" binding:"required"` // JSON数组字符串
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 获取当前用户信息
	userID, _ := c.Get("user_id")
	userName, _ := c.Get("username")
	applicantID := ""
	applicantName := ""
	if userID != nil {
		applicantID = fmt.Sprintf("%v", userID)
	}
	if userName != nil {
		applicantName = fmt.Sprintf("%v", userName)
	}

	// 获取工单信息
	var ticket model.Ticket
	if err := h.db.First(&ticket, req.TicketID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "工单不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取工单信息失败",
			"error":   err.Error(),
		})
		return
	}

	// 获取审批配置
	var config model.ApprovalConfig
	if err := h.db.First(&config, "id = ?", req.ApprovalConfigID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "审批配置不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取审批配置失败",
			"error":   err.Error(),
		})
		return
	}

	// 检查配置是否启用
	if !config.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "审批配置未启用",
		})
		return
	}

	// 从工单模板的审批配置中读取 platform、api_base_url 和 callback_url（如果存在则覆盖系统配置）
	// 优先使用模板配置中的 platform，如果没有则使用请求参数中的 platform（兼容旧数据）
	actualPlatform := req.Platform
	if ticket.TemplateID != nil && *ticket.TemplateID > 0 {
		var template model.FormTemplate
		if err := h.db.First(&template, *ticket.TemplateID).Error; err == nil {
			if len(template.ApprovalConfig) > 0 {
				var templateApprovalConfig map[string]interface{}
				if err := json.Unmarshal(template.ApprovalConfig, &templateApprovalConfig); err == nil {
					// 如果模板配置中有 platform，优先使用模板配置的 platform
					if templatePlatform, ok := templateApprovalConfig["platform"].(string); ok && templatePlatform != "" {
						actualPlatform = templatePlatform
					}
					// 如果模板配置中有 api_base_url，则覆盖系统配置
					if apiBaseURL, ok := templateApprovalConfig["api_base_url"].(string); ok && apiBaseURL != "" {
						config.APIBaseURL = apiBaseURL
					}
					// 如果模板配置中有 callback_url，则覆盖系统配置
					if callbackURL, ok := templateApprovalConfig["callback_url"].(string); ok && callbackURL != "" {
						config.CallbackURL = callbackURL
					}
				}
			}
		}
	}

	// 检查平台类型是否匹配（使用实际使用的平台类型）
	if config.Type != actualPlatform {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": fmt.Sprintf("审批配置的平台类型(%s)与模板配置的平台类型(%s)不匹配", config.Type, actualPlatform),
		})
		return
	}

	// 确定平台类型
	platform := model.ApprovalPlatform(actualPlatform)

	// 构建审批对象
	now := time.Now()
	approvalRecord := &model.Approval{
		ID:            uuid.New().String(),
		Title:         ticket.Title,
		Description:   fmt.Sprintf("工单编号: %s", ticket.TicketNumber),
		Type:          model.ApprovalTypeDeployment, // 默认类型，可以根据工单类型调整
		Status:        model.ApprovalStatusPending,
		Platform:      platform,
		ApplicantID:   applicantID,
		ApplicantName: applicantName,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// 如果申请人信息为空，使用工单的申请人信息
	if approvalRecord.ApplicantID == "" {
		approvalRecord.ApplicantID = ticket.ApplicantID
	}
	if approvalRecord.ApplicantName == "" {
		approvalRecord.ApplicantName = ticket.ApplicantName
	}

	// 从配置中读取审批人信息
	var approverNames []string
	if config.ApproverUserIDs != "" {
		var approverIDs []string
		if err := json.Unmarshal([]byte(config.ApproverUserIDs), &approverIDs); err == nil {
			approvalRecord.ApproverIDs = approverIDs
			// 将审批人ID转换为名称
			for _, approverID := range approverIDs {
				var user model.User
				if err := h.db.Where("id = ?", approverID).First(&user).Error; err == nil {
					approverNames = append(approverNames, user.Username)
				} else {
					// 如果查询不到用户，使用ID作为名称（兼容）
					approverNames = append(approverNames, approverID)
				}
			}
			approvalRecord.ApproverNames = approverNames
		}
	}

	// 创建 provider 实例（需要重新创建，因为配置可能已更新）
	var providerInstance approval.Provider
	switch platform {
	case model.ApprovalPlatformFeishu:
		providerInstance = approval.NewFeishuProvider(&config, h.db)
	case model.ApprovalPlatformDingTalk:
		providerInstance = approval.NewDingTalkProvider(&config, h.db)
	case model.ApprovalPlatformWeChat:
		providerInstance = approval.NewWeChatProvider(&config, h.db)
	default:
		c.JSON(http.StatusNotImplemented, gin.H{
			"code":    501,
			"message": fmt.Sprintf("不支持的审批平台: %s", req.Platform),
		})
		return
	}

	// 调用 provider 创建审批实例
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	externalID, err := providerInstance.CreateApprovalWithFormData(ctx, req.ApprovalCode, req.FormData, approvalRecord)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建第三方审批实例失败",
			"error":   err.Error(),
		})
		return
	}

	// 更新审批对象
	approvalRecord.ExternalID = externalID
	approvalRecord.ExternalURL = h.getDefaultExternalURL(platform, externalID)

	// 保存到数据库
	if err := h.db.Create(approvalRecord).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "保存审批记录失败",
			"error":   err.Error(),
		})
		return
	}

	// 更新工单的审批信息
	ticket.ApprovalPlatform = req.Platform
	ticket.ApprovalInstanceID = externalID
	ticket.ApprovalURL = approvalRecord.ExternalURL
	// 同步审批人信息到工单
	if len(approverNames) > 0 {
		approversJSON, _ := json.Marshal(approverNames)
		ticket.Approvers = datatypes.JSON(approversJSON)
		// 构建初始审批步骤
		var steps []map[string]interface{}
		for i, approverName := range approverNames {
			step := map[string]interface{}{
				"step":     i + 1,
				"approver": approverName,
				"status":   "pending",
			}
			steps = append(steps, step)
		}
		stepsJSON, _ := json.Marshal(steps)
		ticket.ApprovalSteps = datatypes.JSON(stepsJSON)
	}
	if err := h.db.Save(&ticket).Error; err != nil {
		// 记录错误但不影响返回结果
		fmt.Printf("更新工单审批信息失败: %v\n", err)
	}

	// 添加提交记录
	comment := model.ApprovalComment{
		ID:         uuid.New().String(),
		ApprovalID: approvalRecord.ID,
		UserID:     approvalRecord.ApplicantID,
		UserName:   approvalRecord.ApplicantName,
		Action:     "submit",
		Comment:    fmt.Sprintf("从工单 #%s 创建第三方审批", ticket.TicketNumber),
		CreatedAt:  now,
	}
	h.db.Create(&comment)

	// 返回结果
	var instanceCode string
	if platform == model.ApprovalPlatformFeishu {
		instanceCode = externalID
	} else if platform == model.ApprovalPlatformDingTalk {
		instanceCode = externalID
	} else if platform == model.ApprovalPlatformWeChat {
		instanceCode = externalID
	}

	c.JSON(http.StatusOK, gin.H{
		"code":          0,
		"message":       "创建第三方审批实例成功",
		"instance_code": instanceCode,
		"instance_id":   externalID,
		"data":          approvalRecord,
	})
}

// GetApprovalFormDetail 获取审批表单详情
func (h *ApprovalHandler) GetApprovalFormDetail(c *gin.Context) {
	var req struct {
		ApprovalConfigID string `json:"approval_config_id" binding:"required"`
		ApprovalCode     string `json:"approval_code" binding:"required"`
		Platform         string `json:"platform" binding:"required"` // feishu, dingtalk, wechat
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 获取审批配置
	var config model.ApprovalConfig
	if err := h.db.First(&config, "id = ?", req.ApprovalConfigID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "审批配置不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取审批配置失败",
			"error":   err.Error(),
		})
		return
	}

	// 检查平台类型是否匹配
	if config.Type != req.Platform {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": fmt.Sprintf("审批配置的平台类型(%s)与请求的平台类型(%s)不匹配", config.Type, req.Platform),
		})
		return
	}

	// 创建 provider 实例
	platform := model.ApprovalPlatform(req.Platform)
	var providerInstance approval.Provider
	switch platform {
	case model.ApprovalPlatformFeishu:
		providerInstance = approval.NewFeishuProvider(&config, h.db)
	case model.ApprovalPlatformDingTalk:
		providerInstance = approval.NewDingTalkProvider(&config, h.db)
	case model.ApprovalPlatformWeChat:
		providerInstance = approval.NewWeChatProvider(&config, h.db)
	default:
		c.JSON(http.StatusNotImplemented, gin.H{
			"code":    501,
			"message": fmt.Sprintf("不支持的审批平台: %s", req.Platform),
		})
		return
	}

	// 调用 provider 获取表单详情
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	formFields, err := providerInstance.GetApprovalFormDetail(ctx, req.ApprovalCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取审批表单详情失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"form_fields": formFields,
		},
	})
}

// ApproveApproval 批准审批
func (h *ApprovalHandler) ApproveApproval(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		ApproverID   string `json:"approver_id" binding:"required"`
		ApproverName string `json:"approver_name"`
		Comment      string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	var approval model.Approval
	if err := h.db.First(&approval, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "审批不存在",
		})
		return
	}

	if approval.Status != model.ApprovalStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "审批已处理，无法再次批准",
		})
		return
	}

	// 更新审批状态
	now := time.Now()
	approval.Status = model.ApprovalStatusApproved
	approval.ApprovedAt = &now
	approval.ApprovalNote = req.Comment
	approval.CurrentApprover = req.ApproverName
	approval.UpdatedAt = now

	if err := h.db.Save(&approval).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "批准失败",
			"error":   err.Error(),
		})
		return
	}

	// 发布类审批：批准后自动触发生产发布
	if approval.Type == model.ApprovalTypeDeployment && approval.DeployConfig != "" && h.releaseSvc != nil {
		if err := h.releaseSvc.ExecuteApprovedDeployment(&approval); err != nil {
			// 记录日志但不影响审批成功状态，可后续在工单详情展示
			_ = err
		}
	}

	// 添加批准记录
	comment := model.ApprovalComment{
		ID:         uuid.New().String(),
		ApprovalID: approval.ID,
		UserID:     req.ApproverID,
		UserName:   req.ApproverName,
		Action:     "approve",
		Comment:    req.Comment,
		CreatedAt:  now,
	}
	h.db.Create(&comment)

	// TODO: 执行权限授予操作
	// h.grantPermissions(&approval)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "批准成功",
		"data":    approval,
	})
}

// RejectApproval 拒绝审批
func (h *ApprovalHandler) RejectApproval(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		ApproverID   string `json:"approver_id" binding:"required"`
		ApproverName string `json:"approver_name"`
		Reason       string `json:"reason" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	var approval model.Approval
	if err := h.db.First(&approval, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "审批不存在",
		})
		return
	}

	if approval.Status != model.ApprovalStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "审批已处理，无法再次拒绝",
		})
		return
	}

	// 更新审批状态
	now := time.Now()
	approval.Status = model.ApprovalStatusRejected
	approval.RejectedAt = &now
	approval.RejectReason = req.Reason
	approval.CurrentApprover = req.ApproverName
	approval.UpdatedAt = now

	if err := h.db.Save(&approval).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "拒绝失败",
			"error":   err.Error(),
		})
		return
	}

	// 添加拒绝记录
	comment := model.ApprovalComment{
		ID:         uuid.New().String(),
		ApprovalID: approval.ID,
		UserID:     req.ApproverID,
		UserName:   req.ApproverName,
		Action:     "reject",
		Comment:    req.Reason,
		CreatedAt:  now,
	}
	h.db.Create(&comment)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "已拒绝",
		"data":    approval,
	})
}

// CancelApproval 取消审批
func (h *ApprovalHandler) CancelApproval(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		UserID string `json:"user_id" binding:"required"`
		Reason string `json:"reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	var approval model.Approval
	if err := h.db.First(&approval, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "审批不存在",
		})
		return
	}

	// 只有申请人可以取消
	if approval.ApplicantID != req.UserID {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "无权取消此审批",
		})
		return
	}

	if approval.Status != model.ApprovalStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "只能取消待审批的工单",
		})
		return
	}

	// 取消第三方审批工单
	if approval.ExternalID != "" {
		provider, ok := h.factory.GetProvider(approval.Platform)
		if ok {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			provider.CancelApproval(ctx, approval.ExternalID)
		}
	}

	// 更新状态
	now := time.Now()
	approval.Status = model.ApprovalStatusCanceled
	approval.UpdatedAt = now

	if err := h.db.Save(&approval).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "取消失败",
			"error":   err.Error(),
		})
		return
	}

	// 添加取消记录
	comment := model.ApprovalComment{
		ID:         uuid.New().String(),
		ApprovalID: approval.ID,
		UserID:     req.UserID,
		Action:     "cancel",
		Comment:    req.Reason,
		CreatedAt:  now,
	}
	h.db.Create(&comment)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "已取消",
		"data":    approval,
	})
}

// UpdateApproval 更新审批（用于标记已发布等）
// @Summary 更新审批
// @Description 更新审批信息，主要用于标记发布状态
// @Tags 审批管理
// @Accept json
// @Produce json
// @Param id path string true "审批ID"
// @Param request body object true "更新请求" SchemaExample({"deployment_id": "xxx", "deployed": true})
// @Success 200 {object} model.Response{data=model.Approval}
// @Router /api/approvals/{id} [put]
func (h *ApprovalHandler) UpdateApproval(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		DeploymentID string `json:"deployment_id"`
		Deployed     *bool  `json:"deployed"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	var approval model.Approval
	if err := h.db.First(&approval, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "审批不存在",
		})
		return
	}

	// 更新字段
	if req.DeploymentID != "" {
		approval.DeploymentID = req.DeploymentID
	}
	if req.Deployed != nil {
		approval.Deployed = *req.Deployed
	}
	approval.UpdatedAt = time.Now()

	if err := h.db.Save(&approval).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
		"data":    approval,
	})
}

// AddComment 添加评论
func (h *ApprovalHandler) AddComment(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		UserID   string `json:"user_id" binding:"required"`
		UserName string `json:"user_name"`
		Comment  string `json:"comment" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 检查审批是否存在
	var approval model.Approval
	if err := h.db.First(&approval, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "审批不存在",
		})
		return
	}

	// 添加评论
	comment := model.ApprovalComment{
		ID:         uuid.New().String(),
		ApprovalID: id,
		UserID:     req.UserID,
		UserName:   req.UserName,
		Action:     "comment",
		Comment:    req.Comment,
		CreatedAt:  time.Now(),
	}

	if err := h.db.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "添加评论失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "评论成功",
		"data":    comment,
	})
}

// SearchUsers 搜索用户（用于审批人选择）
func (h *ApprovalHandler) SearchUsers(c *gin.Context) {
	keyword := c.Query("keyword")

	// 允许空关键字，返回空列表
	if keyword == "" {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"users": []model.User{},
			},
		})
		return
	}

	var users []model.User
	query := h.db.Model(&model.User{}).Where("status = ?", "active")

	// 搜索用户名、邮箱、全名
	keyword = "%" + keyword + "%"
	query = query.Where("username LIKE ? OR email LIKE ? OR full_name LIKE ?",
		keyword, keyword, keyword)

	// 限制返回数量，按用户名排序
	query = query.Order("username ASC").Limit(50)

	if err := query.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "搜索用户失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"users": users,
		},
	})
}

// SearchHosts 搜索主机（用于资源选择）
func (h *ApprovalHandler) SearchHosts(c *gin.Context) {
	keyword := c.Query("keyword")

	// 允许空关键字，返回空列表
	if keyword == "" {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"hosts": []model.Host{},
			},
		})
		return
	}

	var hosts []model.Host
	query := h.db.Model(&model.Host{})

	// 搜索主机名或IP地址
	keyword = "%" + keyword + "%"
	query = query.Where("name LIKE ? OR ip LIKE ?", keyword, keyword)

	// 限制返回数量，按名称排序
	query = query.Order("name ASC").Limit(50)

	if err := query.Find(&hosts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "搜索主机失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"hosts": hosts,
		},
	})
}

// GetApprovalStats 获取审批统计
func (h *ApprovalHandler) GetApprovalStats(c *gin.Context) {
	userID := c.Query("user_id")

	stats := gin.H{}

	// 我的申请统计
	if userID != "" {
		var myStats struct {
			Total    int64 `json:"total"`
			Pending  int64 `json:"pending"`
			Approved int64 `json:"approved"`
			Rejected int64 `json:"rejected"`
		}

		h.db.Model(&model.Approval{}).Where("applicant_id = ?", userID).Count(&myStats.Total)
		h.db.Model(&model.Approval{}).Where("applicant_id = ? AND status = ?", userID, model.ApprovalStatusPending).Count(&myStats.Pending)
		h.db.Model(&model.Approval{}).Where("applicant_id = ? AND status = ?", userID, model.ApprovalStatusApproved).Count(&myStats.Approved)
		h.db.Model(&model.Approval{}).Where("applicant_id = ? AND status = ?", userID, model.ApprovalStatusRejected).Count(&myStats.Rejected)

		stats["my_approvals"] = myStats

		// 待我审批统计 (MySQL兼容方式)
		var pendingCount int64
		h.db.Model(&model.Approval{}).Where("status = ? AND JSON_CONTAINS(approver_ids, ?)", model.ApprovalStatusPending, fmt.Sprintf(`"%s"`, userID)).Count(&pendingCount)
		stats["pending_approvals"] = pendingCount
	}

	// 全局统计（管理员）
	var globalStats struct {
		Total    int64 `json:"total"`
		Pending  int64 `json:"pending"`
		Approved int64 `json:"approved"`
		Rejected int64 `json:"rejected"`
	}

	h.db.Model(&model.Approval{}).Count(&globalStats.Total)
	h.db.Model(&model.Approval{}).Where("status = ?", model.ApprovalStatusPending).Count(&globalStats.Pending)
	h.db.Model(&model.Approval{}).Where("status = ?", model.ApprovalStatusApproved).Count(&globalStats.Approved)
	h.db.Model(&model.Approval{}).Where("status = ?", model.ApprovalStatusRejected).Count(&globalStats.Rejected)

	stats["global"] = globalStats

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// HandleCallback 处理第三方平台回调
func (h *ApprovalHandler) HandleCallback(c *gin.Context) {
	platform := c.Param("platform")

	provider, ok := h.factory.GetProvider(model.ApprovalPlatform(platform))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "不支持的审批平台",
		})
		return
	}

	var callbackData map[string]interface{}
	if err := c.ShouldBindJSON(&callbackData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "回调数据格式错误",
			"error":   err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := provider.HandleCallback(ctx, callbackData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "处理回调失败",
			"error":   err.Error(),
		})
		return
	}

	// 更新审批状态
	var approval model.Approval
	if err := h.db.Where("external_id = ?", result.ApprovalID).First(&approval).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "审批不存在",
		})
		return
	}

	now := time.Now()
	approval.Status = result.Status
	approval.CurrentApprover = result.ApproverName
	approval.UpdatedAt = now

	if result.Status == model.ApprovalStatusApproved {
		approval.ApprovedAt = &now
		approval.ApprovalNote = result.Comment
	} else if result.Status == model.ApprovalStatusRejected {
		approval.RejectedAt = &now
		approval.RejectReason = result.Comment
	}

	h.db.Save(&approval)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "处理成功",
	})
}

// GetApprovalConfig 获取审批配置
func (h *ApprovalHandler) GetApprovalConfig(c *gin.Context) {
	var configs []model.ApprovalConfig

	if err := h.db.Order("created_at DESC").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取配置失败",
			"error":   err.Error(),
		})
		return
	}

	// 支持的平台列表
	platforms := []string{"feishu", "dingtalk", "wechat"}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"configs":   configs,
			"platforms": platforms,
		},
	})
}

// UpdateApprovalConfig 更新审批配置
func (h *ApprovalHandler) UpdateApprovalConfig(c *gin.Context) {
	id := c.Param("id")

	var req model.ApprovalConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 验证必填字段
	if req.Name == "" || req.Type == "" || req.AppID == "" || req.AppSecret == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "配置名称、平台类型、应用ID和应用密钥不能为空",
		})
		return
	}

	// 验证平台类型
	if req.Type != "feishu" && req.Type != "dingtalk" && req.Type != "wechat" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "不支持的平台类型",
		})
		return
	}

	// 验证表单字段格式
	if req.FormFields != "" {
		var fields interface{}
		if err := json.Unmarshal([]byte(req.FormFields), &fields); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "表单字段格式错误，必须是有效的JSON",
				"error":   err.Error(),
			})
			return
		}
	}

	if id == "" {
		// 创建新配置
		req.ID = uuid.New().String()
		req.CreatedAt = time.Now()
		req.UpdatedAt = time.Now()

		if err := h.db.Create(&req).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "创建配置失败",
				"error":   err.Error(),
			})
			return
		}
	} else {
		// 更新配置
		// 先查询现有记录，保留原有的 CreatedAt 值
		var existingConfig model.ApprovalConfig
		if err := h.db.Where("id = ?", id).First(&existingConfig).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "配置不存在",
			})
			return
		}

		// 保留原有的 CreatedAt 值
		req.CreatedAt = existingConfig.CreatedAt
		req.UpdatedAt = time.Now()
		req.ID = id

		if err := h.db.Save(&req).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "更新配置失败",
				"error":   err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "保存成功",
		"data":    req,
	})
}

// DeleteApprovalConfig 删除审批配置
func (h *ApprovalHandler) DeleteApprovalConfig(c *gin.Context) {
	id := c.Param("id")

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "配置ID不能为空",
		})
		return
	}

	// 检查配置是否存在
	var config model.ApprovalConfig
	if err := h.db.First(&config, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "配置不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询配置失败",
			"error":   err.Error(),
		})
		return
	}

	// 删除配置
	if err := h.db.Delete(&model.ApprovalConfig{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除配置失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// getDefaultExternalURL 获取默认的外部链接URL
func (h *ApprovalHandler) getDefaultExternalURL(platform model.ApprovalPlatform, externalID string) string {
	switch platform {
	case model.ApprovalPlatformFeishu:
		return fmt.Sprintf("https://www.feishu.cn/approval/instance/%s", externalID)
	case model.ApprovalPlatformDingTalk:
		return fmt.Sprintf("https://aflow.dingtalk.com/dingtalk/mobile/homepage.htm?corpid=&lwp_as=1&procInstId=%s", externalID)
	case model.ApprovalPlatformWeChat:
		return fmt.Sprintf("https://work.weixin.qq.com/wework_admin/frame#/approval/detail/%s", externalID)
	default:
		return ""
	}
}
