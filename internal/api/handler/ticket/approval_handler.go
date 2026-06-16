package ticket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/fisker086/keyops/internal/approval"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/internal/service/release"
	"github.com/fisker086/keyops/pkg/logger"
)

// ApprovalHandler 审批处理器
type ApprovalHandler struct {
	db                 *gorm.DB
	factory            *approval.Factory
	releaseSvc         *release.Service
	permissionRuleRepo repository.PermissionRuleRepository
}

// NewApprovalHandler 创建审批处理器
func NewApprovalHandler(db *gorm.DB, factory *approval.Factory, releaseSvc *release.Service, permissionRuleRepo repository.PermissionRuleRepository) *ApprovalHandler {
	return &ApprovalHandler{
		db:                 db,
		factory:            factory,
		releaseSvc:         releaseSvc,
		permissionRuleRepo: permissionRuleRepo,
	}
}

var itemNameCN = map[string]string{
	"WEB":           "前端",
	"MIDDLEWARE":    "中间件",
	"BACKEND":       "后端",
	"FRONTEND":      "前端",
	"SERVER":        "服务端",
	"DATAWARE":      "数据仓库",
	"MOBILE":        "移动端",
	"DATABASE":      "数据库",
	"MICROSERVICE":  "微服务",
	"BATCH":         "批量",
	"SCHEDULER":     "定时任务",
	"GATEWAY":       "网关",
	"CACHE":         "缓存",
	"MESSAGE_QUEUE": "消息队列",
}

func translateItemNames(names string) string {
	if names == "" {
		return ""
	}
	parts := strings.Split(names, ", ")
	translated := make([]string, 0, len(parts))
	for _, p := range parts {
		if cn, ok := itemNameCN[strings.ToUpper(p)]; ok {
			translated = append(translated, cn)
		} else {
			translated = append(translated, p)
		}
	}
	return strings.Join(translated, ", ")
}

// 仅展示已提交审批的发版单：审核中/发版中/已完成，或已有审批记录/第三方实例
const buildMasterSubmittedForApprovalSQL = `(bl.status >= ? OR (bl.approval_instance IS NOT NULL AND bl.approval_instance != '') OR EXISTS (SELECT 1 FROM build_master_approvals ba_sub WHERE ba_sub.list_id = bl.id))`

func buildMasterDisplayType(typeVal int) string {
	if typeVal == model.BuildMasterTypeUrgent {
		return "urgent"
	}
	return "normal"
}

func buildMasterTypeFilterInt(approvalType string) (int, bool) {
	switch approvalType {
	case "urgent", "deployment":
		return model.BuildMasterTypeUrgent, true
	case "normal", "daily":
		return model.BuildMasterTypeNormal, true
	default:
		return 0, false
	}
}

// ListApprovals 获取工单列表（由build_master_lists左连build_master_approvals）
func (h *ApprovalHandler) ListApprovals(c *gin.Context) {
	userID := c.Query("user_id")
	statusFilter := c.Query("status")
	approvalType := c.Query("type")
	role := c.Query("role") // related: 与我相关, all: 全部
	isAdmin := c.GetString("role") == "admin"

	query := h.db.Table("build_master_lists bl").
		Select(`bl.*,
				agg.approval_result,
				agg.approver_ids,
				agg.approver_names,
				agg.approval_note,
				agg.reject_reason,
				inames.item_names,
				det.items_json`).
		Joins(`LEFT JOIN (
			SELECT
				ba.list_id,
				CASE
					WHEN SUM(CASE WHEN ba.result = 'rejected' THEN 1 ELSE 0 END) > 0 THEN 'rejected'
					WHEN SUM(CASE WHEN ba.result = 'pending' THEN 1 ELSE 0 END) > 0 THEN 'pending'
					WHEN SUM(CASE WHEN ba.result = 'approved' THEN 1 ELSE 0 END) > 0
						AND SUM(CASE WHEN ba.result = 'pending' THEN 1 ELSE 0 END) = 0 THEN 'approved'
					ELSE NULL
				END AS approval_result,
				GROUP_CONCAT(DISTINCT ba.approver_id SEPARATOR ',') AS approver_ids,
				GROUP_CONCAT(DISTINCT ba.approver_name SEPARATOR ',') AS approver_names,
				GROUP_CONCAT(CASE WHEN ba.result = 'approved' THEN ba.comment ELSE NULL END SEPARATOR '; ') AS approval_note,
				GROUP_CONCAT(CASE WHEN ba.result = 'rejected' THEN ba.comment ELSE NULL END SEPARATOR '; ') AS reject_reason
			FROM build_master_approvals ba
			GROUP BY ba.list_id
		) agg ON agg.list_id = bl.id`).
		Joins(`LEFT JOIN (
			SELECT bi.list_id, GROUP_CONCAT(DISTINCT bi.name SEPARATOR ', ') AS item_names
			FROM build_master_items bi
			GROUP BY bi.list_id
		) inames ON inames.list_id = bl.id`).
		Joins(`LEFT JOIN (
			SELECT bi.list_id,
				JSON_ARRAYAGG(
					JSON_OBJECT('name', bi.name, 'app_name', COALESCE(bid.app_name, ''), 'tag', COALESCE(bid.tag, ''), 'operate', COALESCE(bid.operate, ''), 'sub_type', COALESCE(bid.sub_type, ''))
				) AS items_json
			FROM build_master_items bi
			JOIN build_master_item_details bid ON bid.item_id = bi.id
			GROUP BY bi.list_id
		) det ON det.list_id = bl.id`).
		Where(buildMasterSubmittedForApprovalSQL, model.BuildMasterStatusApproving)

	// 与我相关：申请人是本人，或我是审批人
	if role == "related" && userID != "" {
		query = query.Where("bl.owner_id = ? OR EXISTS (SELECT 1 FROM build_master_approvals ba2 WHERE ba2.list_id = bl.id AND ba2.approver_id = ?)", userID, userID)
	} else if role == "all" && userID != "" && !isAdmin {
		query = query.Where("bl.owner_id = ? OR EXISTS (SELECT 1 FROM build_master_approvals ba2 WHERE ba2.list_id = bl.id AND ba2.approver_id = ?)", userID, userID)
	}

	if statusFilter != "" {
		query = query.Where("bl.status = ?", statusFilter)
	}

	if approvalType != "" {
		if typeInt, ok := buildMasterTypeFilterInt(approvalType); ok {
			query = query.Where("bl.type = ?", typeInt)
		}
	}

	type buildMasterRow struct {
		model.BuildMasterList
		ApprovalResult *string `gorm:"column:approval_result"`
		ApproverIDsStr *string `gorm:"column:approver_ids"`
		ApproverNames  *string `gorm:"column:approver_names"`
		ApprovalNote   *string `gorm:"column:approval_note"`
		RejectReason   *string `gorm:"column:reject_reason"`
		ItemNames      *string `gorm:"column:item_names"`
		ItemsJSON      *string `gorm:"column:items_json"`
	}

	var rows []buildMasterRow
	if err := query.Order("bl.created_at DESC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取审批列表失败",
			"error":   err.Error(),
		})
		return
	}

	// 映射为前端需要的格式
	type approvalItem struct {
		ID              string   `json:"id"`
		Title           string   `json:"title"`
		Type            string   `json:"type"`
		Status          string   `json:"status"`
		ApplicantID     string   `json:"applicant_id"`
		ApplicantName   string   `json:"applicant_name"`
		ApproverIDs     []string `json:"approver_ids"`
		CurrentApprover string   `json:"current_approver"`
		Priority        string   `json:"priority"`
		TicketID        uint     `json:"ticket_id"`
		TicketNumber    string   `json:"ticket_number"`
		Site            string   `json:"site"`
		OrderNum        int      `json:"order_num"`
		CreatedAt       string   `json:"created_at"`
		UpdatedAt       string   `json:"updated_at"`
		ApprovalNote    string   `json:"approval_note,omitempty"`
		RejectReason    string   `json:"reject_reason,omitempty"`
		ItemNames       string   `json:"item_names,omitempty"`
		ItemsJSON       string   `json:"items_json,omitempty"`
	}

	approvals := make([]approvalItem, 0, len(rows))
	for _, row := range rows {
		// 显示状态：发版完成后始终显示 completed，其余按审批结果或列表状态
		dispStatus := "pending"
		if row.BuildMasterList.Status == 4 {
			dispStatus = "completed"
		} else if row.ApprovalResult != nil && *row.ApprovalResult != "" {
			dispStatus = *row.ApprovalResult
		} else {
			switch row.BuildMasterList.Status {
			case 3:
				dispStatus = "approved"
			case 2:
				dispStatus = "pending"
			default:
				dispStatus = "pending"
			}
		}

		dispType := buildMasterDisplayType(row.BuildMasterList.Type)

		// 优先级（与发版类型一致）
		priority := "normal"
		if row.BuildMasterList.Type == model.BuildMasterTypeUrgent {
			priority = "urgent"
		}

		// 构建标题
		title := row.BuildMasterList.Site + " " + row.BuildMasterList.PublishDate
		if row.ItemNames != nil && *row.ItemNames != "" {
			title += " (" + translateItemNames(*row.ItemNames) + ")"
		}
		if row.BuildMasterList.OrderDescribe != "" {
			title = row.BuildMasterList.OrderDescribe + " - " + title
		}

		// 解析审批人ID列表
		var approverIDs []string
		if row.ApproverIDsStr != nil && *row.ApproverIDsStr != "" {
			approverIDs = strings.Split(*row.ApproverIDsStr, ",")
		}

		// 当前审批人（取第一个pending的审批人）
		currentApprover := ""
		var approverName string
		h.db.Table("build_master_approvals").
			Where("list_id = ? AND result = 'pending'", row.BuildMasterList.ID).
			Order("order_num ASC").
			Select("approver_name").
			Limit(1).
			Scan(&approverName)
		currentApprover = approverName

		item := approvalItem{
			ID:              row.BuildMasterList.ID,
			Title:           title,
			Type:            dispType,
			Status:          dispStatus,
			ApplicantID:     row.BuildMasterList.OwnerID,
			ApplicantName:   row.BuildMasterList.OwnerName,
			ApproverIDs:     approverIDs,
			CurrentApprover: currentApprover,
			Priority:        priority,
			TicketID:        0,
			TicketNumber:    fmt.Sprintf("WF%d-%s", row.BuildMasterList.CreatedAt.Unix(), row.BuildMasterList.ID[:8]),
			Site:            row.BuildMasterList.Site,
			OrderNum:        row.BuildMasterList.OrderNum,
			CreatedAt:       row.BuildMasterList.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:       row.BuildMasterList.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if row.ItemNames != nil {
			item.ItemNames = translateItemNames(*row.ItemNames)
		}
		if row.ItemsJSON != nil {
			item.ItemsJSON = *row.ItemsJSON
		}

		if row.ApprovalNote != nil {
			item.ApprovalNote = *row.ApprovalNote
		}
		if row.RejectReason != nil {
			item.RejectReason = *row.RejectReason
		}

		approvals = append(approvals, item)
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

	// 1. 尝试旧 approvals 表
	var approval model.Approval
	if err := h.db.Preload("Applicant").First(&approval, "id = ?", id).Error; err == nil {
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
		return
	}

	// 2. 尝试 build_master_lists（新审批系统）
	type bmDetail struct {
		model.BuildMasterList
		ApprovalResult *string `gorm:"column:approval_result"`
		ApprovalNote   *string `gorm:"column:approval_note"`
		RejectReason   *string `gorm:"column:reject_reason"`
		ItemNames      *string `gorm:"column:item_names"`
		ItemsJSON      *string `gorm:"column:items_json"`
	}
	var row bmDetail
	subQuery := h.db.Table("build_master_approvals").
		Select(`list_id,
			CASE
				WHEN SUM(CASE WHEN result = 'rejected' THEN 1 ELSE 0 END) > 0 THEN 'rejected'
				WHEN SUM(CASE WHEN result = 'pending' THEN 1 ELSE 0 END) > 0 THEN 'pending'
				WHEN SUM(CASE WHEN result = 'approved' THEN 1 ELSE 0 END) > 0 THEN 'approved'
				ELSE NULL
			END AS approval_result,
			GROUP_CONCAT(CASE WHEN result = 'approved' THEN comment ELSE NULL END SEPARATOR '; ') AS approval_note,
			GROUP_CONCAT(CASE WHEN result = 'rejected' THEN comment ELSE NULL END SEPARATOR '; ') AS reject_reason`).
		Group("list_id")
	if err := h.db.Table("build_master_lists bl").
		Select("bl.*, agg.approval_result, agg.approval_note, agg.reject_reason, inames.item_names, det.items_json").
		Joins("LEFT JOIN (?) agg ON agg.list_id = bl.id", subQuery).
		Joins(`LEFT JOIN (
			SELECT bi.list_id, GROUP_CONCAT(DISTINCT bi.name SEPARATOR ', ') AS item_names
			FROM build_master_items bi
			GROUP BY bi.list_id
		) inames ON inames.list_id = bl.id`).
		Joins(`LEFT JOIN (
			SELECT bi.list_id,
				JSON_ARRAYAGG(
					JSON_OBJECT('name', bi.name, 'app_name', COALESCE(bid.app_name, ''), 'tag', COALESCE(bid.tag, ''), 'operate', COALESCE(bid.operate, ''), 'sub_type', COALESCE(bid.sub_type, ''))
				) AS items_json
			FROM build_master_items bi
			JOIN build_master_item_details bid ON bid.item_id = bi.id
			GROUP BY bi.list_id
		) det ON det.list_id = bl.id`).
		Where("bl.id = ?", id).
		First(&row).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "审批不存在",
		})
		return
	}

	// 状态：发版完成后始终显示 completed，其余按审批结果或列表状态
	dispStatus := "pending"
	if row.BuildMasterList.Status == 4 {
		dispStatus = "completed"
	} else if row.ApprovalResult != nil && *row.ApprovalResult != "" {
		dispStatus = *row.ApprovalResult
	} else {
		switch row.BuildMasterList.Status {
		case 3:
			dispStatus = "approved"
		}
	}

	dispType := buildMasterDisplayType(row.BuildMasterList.Type)

	// 标题
	title := row.BuildMasterList.Site + " " + row.BuildMasterList.PublishDate
	if row.ItemNames != nil && *row.ItemNames != "" {
		title += " (" + translateItemNames(*row.ItemNames) + ")"
	}
	if row.BuildMasterList.OrderDescribe != "" {
		title = row.BuildMasterList.OrderDescribe + " - " + title
	}

	// 构建返回结果
	result := model.Approval{
		ID:            row.BuildMasterList.ID,
		Title:         title,
		Type:          model.ApprovalType(dispType),
		Status:        model.ApprovalStatus(dispStatus),
		ApplicantID:   row.BuildMasterList.OwnerID,
		ApplicantName: row.BuildMasterList.OwnerName,
		TicketNumber:  fmt.Sprintf("WF%d-%s", row.BuildMasterList.CreatedAt.Unix(), row.BuildMasterList.ID[:8]),
		CreatedAt:     row.BuildMasterList.CreatedAt,
		UpdatedAt:     row.BuildMasterList.UpdatedAt,
	}
	if row.ApprovalNote != nil {
		result.ApprovalNote = *row.ApprovalNote
	}
	if row.RejectReason != nil {
		result.RejectReason = *row.RejectReason
	}

	// 从 build_master_approvals 构建审批历史
	type bmComment struct {
		ID        string    `json:"id"`
		UserID    string    `json:"user_id"`
		UserName  string    `json:"user_name"`
		Action    string    `json:"action"`
		Comment   string    `json:"comment"`
		CreatedAt time.Time `json:"created_at"`
	}
	var bmApprovals []model.BuildMasterApproval
	h.db.Where("list_id = ?", id).Order("order_num ASC").Find(&bmApprovals)
	comments := make([]bmComment, 0, len(bmApprovals))
	for _, ba := range bmApprovals {
		action := ba.Result
		if action == "pending" {
			action = "comment"
		}
		comments = append(comments, bmComment{
			ID:        fmt.Sprintf("%d", ba.ID),
			UserID:    ba.ApproverID,
			UserName:  ba.ApproverName,
			Action:    action,
			Comment:   ba.Comment,
			CreatedAt: ba.CreatedAt,
		})
	}

	// 从 build_master_operation_logs 补充审批历史
	var logs []model.BuildMasterOperationLog
	h.db.Where("list_id = ?", id).Order("created_at ASC").Find(&logs)
	logCommentIDs := make(map[string]bool)
	for _, c := range comments {
		logCommentIDs[c.ID] = true
	}
	for _, log := range logs {
		var action string
		var userName string
		var comment string
		switch log.Method {
		case "submit":
			action = "submit"
			userName = log.OperatorName
			comment = "提交审批申请"
		case "approval_submit":
			action = "submit"
			userName = log.OperatorName
			comment = "提交第三方审批"
		case "approval_approved":
			action = "approve"
			userName = log.OperatorName
			comment = "审批通过（第三方）"
		case "approval_rejected":
			action = "reject"
			userName = log.OperatorName
			comment = "审批拒绝（第三方）"
		case "approve":
			userName = log.OperatorName
			var changes []struct {
				Name string `json:"name"`
				New  string `json:"new"`
			}
			if err := json.Unmarshal([]byte(log.Body), &changes); err == nil {
				for _, change := range changes {
					if change.Name == "approve_result" {
						if change.New == "approved" {
							action = "approve"
						} else if change.New == "rejected" {
							action = "reject"
						}
						break
					}
				}
			}
		case "deploy_helm":
			userName = log.OperatorName
			action = "deploy"
			var changes []struct {
				Name string `json:"name"`
				New  string `json:"new"`
			}
			if err := json.Unmarshal([]byte(log.Body), &changes); err == nil {
				deployIDs := make([]string, 0)
				fails := make([]string, 0)
				for _, c := range changes {
					if c.Name == "deployment_id" && c.New != "" {
						deployIDs = append(deployIDs, c.New)
					}
					if c.Name == "failed_item" && c.New != "" {
						fails = append(fails, c.New)
					}
				}
				parts := make([]string, 0)
				if len(deployIDs) > 0 {
					parts = append(parts, fmt.Sprintf("触发 %d 个服务部署", len(deployIDs)))
				}
				if len(fails) > 0 {
					parts = append(parts, fmt.Sprintf("%d 个失败: %s", len(fails), strings.Join(fails, "; ")))
				}
				if len(parts) > 0 {
					comment = strings.Join(parts, "，")
				}
			}
			if comment == "" {
				comment = "触发一键发版"
			}
		default:
			continue
		}
		logID := fmt.Sprintf("log_%d", log.ID)
		if !logCommentIDs[logID] {
			comments = append(comments, bmComment{
				ID:        logID,
				UserID:    log.OperatorID,
				UserName:  userName,
				Action:    action,
				Comment:   comment,
				CreatedAt: log.CreatedAt,
			})
			logCommentIDs[logID] = true
		}
	}
	sort.Slice(comments, func(i, j int) bool {
		return comments[i].CreatedAt.Before(comments[j].CreatedAt)
	})

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"approval": result,
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

	// 生成回调令牌，嵌入审批表单数据中，回调时可直接匹配
	callbackToken := uuid.New().String()

	// 构建审批对象
	now := time.Now()
	approvalRecord := &model.Approval{
		ID:             uuid.New().String(),
		Title:          ticket.Title,
		Description:    fmt.Sprintf("工单编号: %s", ticket.TicketNumber),
		Type:           model.ApprovalTypeDeployment,
		Status:         model.ApprovalStatusPending,
		Platform:       platform,
		ApplicantID:    applicantID,
		ApplicantName:  applicantName,
		CallbackSource: "ticket",
		CallbackToken:  callbackToken,
		TicketID:       ticket.ID,
		TicketNumber:   ticket.TicketNumber,
		CreatedAt:      now,
		UpdatedAt:      now,
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
				if err := h.db.Where("email = ?", approverID).First(&user).Error; err == nil {
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
		providerInstance = approval.NewFeishuProvider(&config, h.db, model.ApprovalPlatformFeishu)
	case model.ApprovalPlatformLark:
		providerInstance = approval.NewFeishuProvider(&config, h.db, model.ApprovalPlatformLark)
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

	// 将回调令牌注入审批表单数据
	var formDataList []map[string]interface{}
	if err := json.Unmarshal([]byte(req.FormData), &formDataList); err == nil && callbackToken != "" {
		formDataList = append(formDataList, map[string]interface{}{
			"id":    "_callback_token",
			"type":  "input",
			"value": callbackToken,
		})
		injected, _ := json.Marshal(formDataList)
		req.FormData = string(injected)
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
		logger.Warnf("更新工单审批信息失败: %v", err)
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
		providerInstance = approval.NewFeishuProvider(&config, h.db, model.ApprovalPlatformFeishu)
	case model.ApprovalPlatformLark:
		providerInstance = approval.NewFeishuProvider(&config, h.db, model.ApprovalPlatformLark)
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

// grantPermissions 审批通过后执行权限授予
func (h *ApprovalHandler) grantPermissions(approval *model.Approval, performedBy string) error {
	if approval.Type != model.ApprovalTypeHostAccess && approval.Type != model.ApprovalTypeHostGroupAccess {
		return nil
	}

	now := time.Now()
	var expiresAt *time.Time
	if approval.Duration > 0 {
		t := now.Add(time.Duration(approval.Duration) * time.Hour)
		expiresAt = &t
	} else if approval.ExpiresAt != nil {
		expiresAt = approval.ExpiresAt
	}

	// 查找申请人角色
	var roleMember model.RoleMember
	if err := h.db.Where("user_id = ?", approval.ApplicantID).First(&roleMember).Error; err != nil {
		return fmt.Errorf("applicant has no role: %w", err)
	}

	systemUserIDs := make([]string, 0)
	for _, p := range approval.Permissions {
		if p != "" {
			systemUserIDs = append(systemUserIDs, p)
		}
	}

	rule := &model.PermissionRule{
		ID:          uuid.New().String(),
		Name:        fmt.Sprintf("审批授权 - %s", approval.Title),
		RoleID:      roleMember.RoleID,
		ValidFrom:   &now,
		ValidTo:     expiresAt,
		Enabled:     true,
		Description: approval.Reason,
		CreatedBy:   performedBy,
	}

	switch approval.Type {
	case model.ApprovalTypeHostGroupAccess:
		if len(approval.ResourceIDs) > 0 {
			rule.HostGroupID = &approval.ResourceIDs[0]
		}
	case model.ApprovalTypeHostAccess:
		if len(approval.ResourceIDs) > 0 {
			hostIDs, _ := json.Marshal(approval.ResourceIDs)
			rule.HostIDs = string(hostIDs)
		}
	}

	hostGroupIDs := make([]string, 0)
	if rule.HostGroupID != nil {
		hostGroupIDs = append(hostGroupIDs, *rule.HostGroupID)
	}

	return h.permissionRuleRepo.CreateWithRelations(rule, systemUserIDs, hostGroupIDs)
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

	// 执行权限授予（主机/主机组访问类审批）
	if err := h.grantPermissions(&approval, req.ApproverID); err != nil {
		logger.Warnf("[Approval] Permission grant failed for approval %s: %v", approval.ID, err)
	}

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

	// 1. 尝试旧 approvals 表
	var approval model.Approval
	if err := h.db.First(&approval, "id = ?", id).Error; err == nil {
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
		return
	}

	// 2. 尝试 build_master_lists（新审批系统）
	var bmList model.BuildMasterList
	if err := h.db.First(&bmList, "id = ?", id).Error; err == nil {
		// 获取下一个序号
		var maxOrder int
		h.db.Model(&model.BuildMasterApproval{}).
			Where("list_id = ?", id).
			Select("COALESCE(MAX(order_num), 0)").
			Scan(&maxOrder)

		bmComment := model.BuildMasterApproval{
			ListID:       id,
			ApproverID:   req.UserID,
			ApproverName: req.UserName,
			Result:       model.BuildMasterApprovalPending,
			Comment:      req.Comment,
			OrderNum:     maxOrder + 1,
		}
		if err := h.db.Create(&bmComment).Error; err != nil {
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
			"data":    bmComment,
		})
		return
	}

	// 3. 两表均未找到
	c.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "审批不存在",
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

// BuildMasterDeployApproval 一键发版 Build Master 审批单
func (h *ApprovalHandler) BuildMasterDeployApproval(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "approval id required"})
		return
	}
	userID, _ := c.Get("user_id")
	userName, _ := c.Get("username")
	uid, _ := userID.(string)
	uname, _ := userName.(string)

	if h.releaseSvc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "release service not available"})
		return
	}
	result, err := h.releaseSvc.DeployBuildMaster(id, uid, uname)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	// 写操作日志记录部署结果
	bodyChanges := []map[string]string{
		{"name": "deploy_status", "old": "", "new": "deploying"},
	}
	for _, fid := range result.DeploymentIDs {
		bodyChanges = append(bodyChanges, map[string]string{"name": "deployment_id", "old": "", "new": fid})
	}
	for _, fi := range result.FailedItems {
		bodyChanges = append(bodyChanges, map[string]string{"name": "failed_item", "old": "", "new": fi.AppName + ": " + fi.Error})
	}
	bodyBytes, _ := json.Marshal(bodyChanges)
	_ = h.db.Create(&model.BuildMasterOperationLog{
		ListID:       id,
		OperatorID:   uid,
		OperatorName: uname,
		Method:       "deploy_helm",
		Body:         string(bodyBytes),
	})

	// 更新 approvals 表的 deployment_id 字段
	h.db.Model(&model.Approval{}).
		Where("description = ?", "build_master_list_id:"+id).
		Update("deployment_id", strings.Join(result.DeploymentIDs, ","))

	msg := fmt.Sprintf("发版已触发，共 %d 个服务", len(result.DeploymentIDs))
	if len(result.FailedItems) > 0 {
		msg += fmt.Sprintf("，%d 个服务部署失败", len(result.FailedItems))
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": msg,
		"data": gin.H{
			"deployment_ids": result.DeploymentIDs,
			"failed_items":   result.FailedItems,
		},
	})
}

// HelmReleaseDeploy 独立的 Helm 一键部署接口
// POST /api/helm/deploy  { "app_name": "...", "environment": "...", "version": "..." }
func (h *ApprovalHandler) HelmReleaseDeploy(c *gin.Context) {
	var req struct {
		AppName     string `json:"app_name"`
		AppID       string `json:"app_id"`
		Environment string `json:"environment"`
		Version     string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if req.AppName == "" && req.AppID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "app_name or app_id required"})
		return
	}
	if h.releaseSvc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "release service not available"})
		return
	}
	userID, _ := c.Get("user_id")
	userName, _ := c.Get("username")
	uid, _ := userID.(string)
	uname, _ := userName.(string)

	deployID, err := h.releaseSvc.DeployHelmRelease(&release.HelmDeployRequest{
		AppName:     req.AppName,
		AppID:       req.AppID,
		Environment: req.Environment,
		Version:     req.Version,
		UserID:      uid,
		UserName:    uname,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "部署已触发",
		"data": gin.H{
			"deployment_id": deployID,
			"status":        "running",
			"query_url":     "/api/deployments/" + deployID,
		},
	})
}

// GetApprovalStats 获取审批统计（与ListApprovals一致，基于build_master_lists+approvals）
func (h *ApprovalHandler) GetApprovalStats(c *gin.Context) {
	userID := c.Query("user_id")

	stats := gin.H{}

	// 我的申请统计（与我相关）
	if userID != "" {
		baseQuery := h.db.Table("build_master_lists bl").
			Joins(`LEFT JOIN (
				SELECT ba.list_id,
					CASE
						WHEN SUM(CASE WHEN ba.result = 'rejected' THEN 1 ELSE 0 END) > 0 THEN 'rejected'
						WHEN SUM(CASE WHEN ba.result = 'pending' THEN 1 ELSE 0 END) > 0 THEN 'pending'
						WHEN SUM(CASE WHEN ba.result = 'approved' THEN 1 ELSE 0 END) > 0
							AND SUM(CASE WHEN ba.result = 'pending' THEN 1 ELSE 0 END) = 0 THEN 'approved'
						ELSE NULL
					END AS approval_result
				FROM build_master_approvals ba
				GROUP BY ba.list_id
			) agg ON agg.list_id = bl.id`).
			Where(buildMasterSubmittedForApprovalSQL, model.BuildMasterStatusApproving)

		filtered := baseQuery.Where("bl.owner_id = ? OR EXISTS (SELECT 1 FROM build_master_approvals ba2 WHERE ba2.list_id = bl.id AND ba2.approver_id = ?)", userID, userID)

		var myStats struct {
			Total    int64 `json:"total"`
			Pending  int64 `json:"pending"`
			Approved int64 `json:"approved"`
			Rejected int64 `json:"rejected"`
		}

		filtered.Select(`
			COUNT(*) AS total,
			SUM(CASE WHEN COALESCE(agg.approval_result, CASE WHEN bl.status IN (3,4) THEN 'approved' ELSE 'pending' END) = 'pending' THEN 1 ELSE 0 END) AS pending,
			SUM(CASE WHEN COALESCE(agg.approval_result, CASE WHEN bl.status IN (3,4) THEN 'approved' ELSE 'pending' END) = 'approved' THEN 1 ELSE 0 END) AS approved,
			SUM(CASE WHEN COALESCE(agg.approval_result, CASE WHEN bl.status IN (3,4) THEN 'approved' ELSE 'pending' END) = 'rejected' THEN 1 ELSE 0 END) AS rejected
		`).Scan(&myStats)

		stats["my_approvals"] = myStats

		// 待我审批统计
		var pendingCount int64
		h.db.Table("build_master_lists bl").
			Joins(`LEFT JOIN (
				SELECT ba.list_id,
					CASE
						WHEN SUM(CASE WHEN ba.result = 'rejected' THEN 1 ELSE 0 END) > 0 THEN 'rejected'
						WHEN SUM(CASE WHEN ba.result = 'pending' THEN 1 ELSE 0 END) > 0 THEN 'pending'
						WHEN SUM(CASE WHEN ba.result = 'approved' THEN 1 ELSE 0 END) > 0
							AND SUM(CASE WHEN ba.result = 'pending' THEN 1 ELSE 0 END) = 0 THEN 'approved'
						ELSE NULL
					END AS approval_result
				FROM build_master_approvals ba
				GROUP BY ba.list_id
			) agg ON agg.list_id = bl.id`).
			Where(buildMasterSubmittedForApprovalSQL, model.BuildMasterStatusApproving).
			Where("EXISTS (SELECT 1 FROM build_master_approvals ba3 WHERE ba3.list_id = bl.id AND ba3.approver_id = ? AND ba3.result = 'pending')", userID).
			Where("COALESCE(agg.approval_result, CASE WHEN bl.status IN (3,4) THEN 'approved' ELSE 'pending' END) = 'pending'").
			Count(&pendingCount)
		stats["pending_approvals"] = pendingCount
	}

	// 全局统计（仅管理员）
	isAdmin := c.GetString("role") == "admin"
	if isAdmin {
		var globalStats struct {
			Total    int64 `json:"total"`
			Pending  int64 `json:"pending"`
			Approved int64 `json:"approved"`
			Rejected int64 `json:"rejected"`
		}

		h.db.Table("build_master_lists bl").
			Joins(`LEFT JOIN (
				SELECT ba.list_id,
					CASE
						WHEN SUM(CASE WHEN ba.result = 'rejected' THEN 1 ELSE 0 END) > 0 THEN 'rejected'
						WHEN SUM(CASE WHEN ba.result = 'pending' THEN 1 ELSE 0 END) > 0 THEN 'pending'
						WHEN SUM(CASE WHEN ba.result = 'approved' THEN 1 ELSE 0 END) > 0
							AND SUM(CASE WHEN ba.result = 'pending' THEN 1 ELSE 0 END) = 0 THEN 'approved'
						ELSE NULL
					END AS approval_result
				FROM build_master_approvals ba
				GROUP BY ba.list_id
			) agg ON agg.list_id = bl.id`).
			Where(buildMasterSubmittedForApprovalSQL, model.BuildMasterStatusApproving).
			Select(`
				COUNT(*) AS total,
				SUM(CASE WHEN COALESCE(agg.approval_result, CASE WHEN bl.status IN (3,4) THEN 'approved' ELSE 'pending' END) = 'pending' THEN 1 ELSE 0 END) AS pending,
				SUM(CASE WHEN COALESCE(agg.approval_result, CASE WHEN bl.status IN (3,4) THEN 'approved' ELSE 'pending' END) = 'approved' THEN 1 ELSE 0 END) AS approved,
				SUM(CASE WHEN COALESCE(agg.approval_result, CASE WHEN bl.status IN (3,4) THEN 'approved' ELSE 'pending' END) = 'rejected' THEN 1 ELSE 0 END) AS rejected
			`).Scan(&globalStats)

		stats["global"] = globalStats
	}

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
	platforms := approval.ListPlatformTypes()

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
	if !approval.IsSupportedPlatform(model.ApprovalPlatform(req.Type)) {
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
	case model.ApprovalPlatformLark:
		return fmt.Sprintf("https://www.larksuite.com/approval/instance/%s", externalID)
	case model.ApprovalPlatformDingTalk:
		return fmt.Sprintf("https://oa.dingtalk.com/approval/detail?processInstanceId=%s", externalID)
	case model.ApprovalPlatformWeChat:
		return fmt.Sprintf("https://work.weixin.qq.com/wework_admin/frame#/approval/detail/%s", externalID)
	default:
		return ""
	}
}
