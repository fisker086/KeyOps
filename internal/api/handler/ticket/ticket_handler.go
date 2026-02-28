package ticket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/datatypes"

	"github.com/fisker086/keyops/internal/approval"
	"github.com/fisker086/keyops/internal/model"
)

// TicketHandler 工单处理器
type TicketHandler struct {
	db *gorm.DB
}

// NewTicketHandler 创建工单处理器
func NewTicketHandler(db *gorm.DB) *TicketHandler {
	return &TicketHandler{
		db: db,
	}
}

// generateTicketNumber 生成工单编号
func generateTicketNumber() string {
	return fmt.Sprintf("TKT-%s", time.Now().Format("20060102150405")+uuid.New().String()[:8])
}

// DeploymentWithJenkinsInfo 包含Jenkins服务器信息的部署记录
type DeploymentWithJenkinsInfo struct {
	model.Deployment
	JenkinsServerURL string `json:"jenkins_server_url,omitempty"` // Jenkins服务器URL（如果部署类型是Jenkins）
}

// buildDeploymentWithJenkinsInfo 构建包含Jenkins服务器信息的部署记录
func buildDeploymentWithJenkinsInfo(db *gorm.DB, deployment model.Deployment) DeploymentWithJenkinsInfo {
	deploymentWithInfo := DeploymentWithJenkinsInfo{
		Deployment: deployment,
	}

	// 如果是Jenkins部署，查询Jenkins服务器URL
	if deployment.DeployType == "jenkins" && deployment.DeployConfig != "" {
		var deployConfig map[string]interface{}
		if err := json.Unmarshal([]byte(deployment.DeployConfig), &deployConfig); err == nil {
			if serverID, ok := deployConfig["jenkins_server_id"].(float64); ok {
				var jenkinsServer model.JenkinsServer
				if err := db.First(&jenkinsServer, uint(serverID)).Error; err == nil {
					// 直接使用 URL 字段
					deploymentWithInfo.JenkinsServerURL = jenkinsServer.URL
				}
			}
		}
	}

	return deploymentWithInfo
}

// ListTickets 获取工单列表
func (h *TicketHandler) ListTickets(c *gin.Context) {
	var tickets []model.Ticket
	query := h.db.Model(&model.Ticket{})

	// 预加载模板（如果有的话）
	query = query.Preload("Template")

	// 权限检查：如果查询全部工单（没有传 applicant_id 且 exclude_status=draft），需要管理员权限
	applicantID := c.Query("applicant_id")
	excludeStatus := c.Query("exclude_status")
	if applicantID == "" && excludeStatus == "draft" {
		// 查询全部工单，需要管理员权限
		role, exists := c.Get("role")
		if !exists || role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "需要管理员权限才能查看全部工单",
			})
			return
		}
	}

	// 过滤条件
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	// 排除特定状态（例如：排除草稿）
	if excludeStatus != "" {
		query = query.Where("status != ?", excludeStatus)
	}
	if templateID := c.Query("template_id"); templateID != "" {
		query = query.Where("template_id = ?", templateID)
	}
	if applicantID != "" {
		query = query.Where("applicant_id = ?", applicantID)
	} else {
		// 如果没有传 applicant_id，但也不是查询全部工单（没有 exclude_status=draft），则默认查询当前用户的工单
		// 这是为了安全：普通用户查询时，如果没有传 applicant_id，自动限制为自己的工单
		if excludeStatus != "draft" {
			if userID, exists := c.Get("user_id"); exists {
				query = query.Where("applicant_id = ?", userID)
			}
		}
	}
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("title LIKE ? OR ticket_number LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	// 分页
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	var total int64
	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&tickets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取工单列表失败",
			"error":   err.Error(),
		})
		return
	}

	// 为每个工单查询关联的审批和部署信息
	type TicketWithStatus struct {
		model.Ticket
		ApplicantUsername string `json:"applicant_username"` // 申请人用户名
		ApprovalStatus    string `json:"approval_status"`    // pending, approved, rejected
		DeploymentID      string `json:"deployment_id"`
		DeploymentStatus  string `json:"deployment_status"` // pending, running, success, failed
	}

	ticketsWithStatus := make([]TicketWithStatus, len(tickets))
	for i, ticket := range tickets {
		ticketsWithStatus[i] = TicketWithStatus{
			Ticket: ticket,
		}

		// 查询申请人的用户名
		if ticket.ApplicantID != "" {
			var user model.User
			if err := h.db.Where("id = ?", ticket.ApplicantID).First(&user).Error; err == nil {
				ticketsWithStatus[i].ApplicantUsername = user.Username
			} else {
				// 如果查询不到用户，使用 applicant_name 作为 fallback
				ticketsWithStatus[i].ApplicantUsername = ticket.ApplicantName
			}
		} else {
			ticketsWithStatus[i].ApplicantUsername = ticket.ApplicantName
		}

		// 如果工单有审批实例ID，查询审批状态
		if ticket.ApprovalInstanceID != "" {
			var approval model.Approval
			// 尝试通过 external_id 或 id 查找审批
			if err := h.db.Where("external_id = ? OR id = ?", ticket.ApprovalInstanceID, ticket.ApprovalInstanceID).First(&approval).Error; err == nil {
				ticketsWithStatus[i].ApprovalStatus = string(approval.Status)
				ticketsWithStatus[i].DeploymentID = approval.DeploymentID

				// 如果审批有部署ID，查询部署状态
				if approval.DeploymentID != "" {
					var deployment model.Deployment
					if err := h.db.Where("id = ?", approval.DeploymentID).First(&deployment).Error; err == nil {
						ticketsWithStatus[i].DeploymentStatus = deployment.Status
					}
				}

				// 如果工单的审批人为空，尝试从审批记录中同步
				var currentApprovers []string
				if len(ticket.Approvers) > 0 {
					if err := json.Unmarshal(ticket.Approvers, &currentApprovers); err != nil {
						currentApprovers = []string{}
					}
				}
				if len(currentApprovers) == 0 {
					var approverNamesFromApproval []string
					if len(approval.ApproverNames) > 0 {
						approverNamesFromApproval = approval.ApproverNames
					} else if len(approval.ApproverIDs) > 0 {
						// 如果审批记录只有ID没有名称，尝试转换为名称
						for _, approverID := range approval.ApproverIDs {
							var user model.User
							if err := h.db.Where("id = ?", approverID).First(&user).Error; err == nil {
								approverNamesFromApproval = append(approverNamesFromApproval, user.Username)
							} else {
								approverNamesFromApproval = append(approverNamesFromApproval, approverID)
							}
						}
					}
					if len(approverNamesFromApproval) > 0 {
						approversJSON, _ := json.Marshal(approverNamesFromApproval)
						ticketsWithStatus[i].Approvers = datatypes.JSON(approversJSON)
					}
				}
			}
		}

		// 如果工单的审批人仍然为空，尝试从模板的审批配置中读取
		var currentApproversAfterApproval []string
		if len(ticketsWithStatus[i].Approvers) > 0 {
			if err := json.Unmarshal(ticketsWithStatus[i].Approvers, &currentApproversAfterApproval); err != nil {
				currentApproversAfterApproval = []string{}
			}
		}
		if len(currentApproversAfterApproval) == 0 && ticket.TemplateID != nil && ticket.Template.ID > 0 {
			// 从模板的审批配置中读取
			if len(ticket.Template.ApprovalConfig) > 0 {
				var approvalConfig map[string]interface{}
				if err := json.Unmarshal(ticket.Template.ApprovalConfig, &approvalConfig); err == nil {
					if approverUserIDs, ok := approvalConfig["approver_user_ids"].([]interface{}); ok {
						var approverNamesFromConfig []string
						for _, approverIDInterface := range approverUserIDs {
							approverID := fmt.Sprintf("%v", approverIDInterface)
							var user model.User
							if err := h.db.Where("id = ?", approverID).First(&user).Error; err == nil {
								approverNamesFromConfig = append(approverNamesFromConfig, user.Username)
							} else {
								approverNamesFromConfig = append(approverNamesFromConfig, approverID)
							}
						}
						if len(approverNamesFromConfig) > 0 {
							approversJSON, _ := json.Marshal(approverNamesFromConfig)
							ticketsWithStatus[i].Approvers = datatypes.JSON(approversJSON)
							currentApproversAfterApproval = approverNamesFromConfig
						}
					}
				}
			}
		}

		// 如果工单有审批平台但没有审批实例ID，也尝试从审批配置中读取
		if len(currentApproversAfterApproval) == 0 && ticket.ApprovalPlatform != "" && ticket.ApprovalInstanceID == "" {
			var config model.ApprovalConfig
			if err := h.db.Where("type = ? AND enabled = ?", ticket.ApprovalPlatform, true).First(&config).Error; err == nil {
				if config.ApproverUserIDs != "" {
					var approverIDs []string
					if err := json.Unmarshal([]byte(config.ApproverUserIDs), &approverIDs); err == nil {
						var approverNamesFromConfig []string
						for _, approverID := range approverIDs {
							var user model.User
							if err := h.db.Where("id = ?", approverID).First(&user).Error; err == nil {
								approverNamesFromConfig = append(approverNamesFromConfig, user.Username)
							} else {
								approverNamesFromConfig = append(approverNamesFromConfig, approverID)
							}
						}
						if len(approverNamesFromConfig) > 0 {
							approversJSON, _ := json.Marshal(approverNamesFromConfig)
							ticketsWithStatus[i].Approvers = datatypes.JSON(approversJSON)
						}
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    ticketsWithStatus,
		"total":   total,
	})
}

// GetTicket 获取工单详情
func (h *TicketHandler) GetTicket(c *gin.Context) {
	id := c.Param("id")

	var ticket model.Ticket
	if err := h.db.Preload("Template").First(&ticket, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "工单不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取工单详情失败",
			"error":   err.Error(),
		})
		return
	}

	// 查询申请人的用户名
	applicantUsername := ticket.ApplicantName
	if ticket.ApplicantID != "" {
		var user model.User
		if err := h.db.Where("id = ?", ticket.ApplicantID).First(&user).Error; err == nil {
			applicantUsername = user.Username
		}
	}

	// 查询关联的审批和部署信息
	type TicketDetailResponse struct {
		model.Ticket
		ApplicantUsername string                      `json:"applicant_username"` // 申请人用户名
		ApprovalStatus    string                      `json:"approval_status"`
		DeploymentID      string                      `json:"deployment_id"`
		DeploymentStatus  string                      `json:"deployment_status"`
		Deployments       []DeploymentWithJenkinsInfo `json:"deployments"` // 关联的部署列表
	}

	response := TicketDetailResponse{
		Ticket:           ticket,
		ApplicantUsername: applicantUsername,
		Deployments:      []DeploymentWithJenkinsInfo{}, // 初始化为空数组
	}

	// 查询关联的审批和部署信息
	var foundDeployments []DeploymentWithJenkinsInfo

	// 方式1: 如果工单有审批实例ID，通过审批实例ID查找
	if ticket.ApprovalInstanceID != "" {
		var approval model.Approval
		if err := h.db.Where("external_id = ? OR id = ?", ticket.ApprovalInstanceID, ticket.ApprovalInstanceID).First(&approval).Error; err == nil {
			response.ApprovalStatus = string(approval.Status)
			response.DeploymentID = approval.DeploymentID

			// 如果工单的审批信息不完整，从审批记录中同步
			if ticket.CurrentApprover == "" && approval.CurrentApprover != "" {
				ticket.CurrentApprover = approval.CurrentApprover
			}
			// 优先使用审批记录中的审批人信息（如果工单的审批人为空）
			var approverNamesFromApproval []string
			if len(approval.ApproverNames) > 0 {
				approverNamesFromApproval = approval.ApproverNames
			} else if len(approval.ApproverIDs) > 0 {
				// 如果审批记录只有ID没有名称，尝试转换为名称
				for _, approverID := range approval.ApproverIDs {
					var user model.User
					if err := h.db.Where("id = ?", approverID).First(&user).Error; err == nil {
						approverNamesFromApproval = append(approverNamesFromApproval, user.Username)
					} else {
						approverNamesFromApproval = append(approverNamesFromApproval, approverID)
					}
				}
			}
			// 如果工单的审批人为空或为空数组，使用审批记录中的审批人
			var currentApprovers []string
			if len(ticket.Approvers) > 0 {
				if err := json.Unmarshal(ticket.Approvers, &currentApprovers); err != nil {
					currentApprovers = []string{}
				}
			}
			if len(currentApprovers) == 0 && len(approverNamesFromApproval) > 0 {
				approversJSON, _ := json.Marshal(approverNamesFromApproval)
				ticket.Approvers = datatypes.JSON(approversJSON)
				currentApprovers = approverNamesFromApproval
			}
			// 如果还是没有审批人信息，尝试从审批配置中读取（作为最后的备选方案）
			if len(currentApprovers) == 0 && ticket.ApprovalPlatform != "" {
				var config model.ApprovalConfig
				if err := h.db.Where("type = ? AND enabled = ?", ticket.ApprovalPlatform, true).First(&config).Error; err == nil {
					if config.ApproverUserIDs != "" {
						var approverIDs []string
						if err := json.Unmarshal([]byte(config.ApproverUserIDs), &approverIDs); err == nil {
							var approverNamesFromConfig []string
							for _, approverID := range approverIDs {
								var user model.User
								if err := h.db.Where("id = ?", approverID).First(&user).Error; err == nil {
									approverNamesFromConfig = append(approverNamesFromConfig, user.Username)
								} else {
									approverNamesFromConfig = append(approverNamesFromConfig, approverID)
								}
							}
							if len(approverNamesFromConfig) > 0 {
								approversJSON, _ := json.Marshal(approverNamesFromConfig)
								ticket.Approvers = datatypes.JSON(approversJSON)
								currentApprovers = approverNamesFromConfig
							}
						}
					}
				}
			}
			// 如果审批步骤为空，尝试从审批记录构建审批步骤
			var currentApprovalSteps []map[string]interface{}
			if len(ticket.ApprovalSteps) > 0 {
				if err := json.Unmarshal(ticket.ApprovalSteps, &currentApprovalSteps); err != nil {
					currentApprovalSteps = []map[string]interface{}{}
				}
			}
			if len(currentApprovalSteps) == 0 {
				// 使用之前获取的审批人列表
				var approverNamesForSteps []string
				if len(currentApprovers) > 0 {
					approverNamesForSteps = currentApprovers
				} else if len(approverNamesFromApproval) > 0 {
					approverNamesForSteps = approverNamesFromApproval
				} else if len(approval.ApproverNames) > 0 {
					approverNamesForSteps = approval.ApproverNames
				} else if len(approval.ApproverIDs) > 0 {
					// 如果只有ID，转换为名称
					for _, approverID := range approval.ApproverIDs {
						var user model.User
						if err := h.db.Where("id = ?", approverID).First(&user).Error; err == nil {
							approverNamesForSteps = append(approverNamesForSteps, user.Username)
						} else {
							approverNamesForSteps = append(approverNamesForSteps, approverID)
						}
					}
				}
				// 如果还是没有审批人信息，尝试从审批配置中读取（作为最后的备选方案）
				if len(approverNamesForSteps) == 0 && ticket.ApprovalPlatform != "" {
					var config model.ApprovalConfig
					if err := h.db.Where("type = ? AND enabled = ?", ticket.ApprovalPlatform, true).First(&config).Error; err == nil {
						if config.ApproverUserIDs != "" {
							var approverIDs []string
							if err := json.Unmarshal([]byte(config.ApproverUserIDs), &approverIDs); err == nil {
								for _, approverID := range approverIDs {
									var user model.User
									if err := h.db.Where("id = ?", approverID).First(&user).Error; err == nil {
										approverNamesForSteps = append(approverNamesForSteps, user.Username)
									} else {
										approverNamesForSteps = append(approverNamesForSteps, approverID)
									}
								}
							}
						}
					}
				}
				if len(approverNamesForSteps) > 0 {
					var steps []map[string]interface{}
					for i, approverName := range approverNamesForSteps {
						step := map[string]interface{}{
							"step":     i + 1,
							"approver": approverName,
							"status":   "pending",
						}
						// 如果当前审批人是这个审批人，标记为当前步骤
						if approval.CurrentApprover == approverName {
							step["status"] = "pending"
						}
						steps = append(steps, step)
					}
					stepsJSON, _ := json.Marshal(steps)
					ticket.ApprovalSteps = datatypes.JSON(stepsJSON)
					// 同时更新工单的审批人列表
					if len(currentApprovers) == 0 {
						approversJSON, _ := json.Marshal(approverNamesForSteps)
						ticket.Approvers = datatypes.JSON(approversJSON)
					}
				}
			}

			// 如果审批有部署ID，查询部署状态和Jenkins服务器信息
			if approval.DeploymentID != "" {
				var deployment model.Deployment
				if err := h.db.Where("id = ?", approval.DeploymentID).First(&deployment).Error; err == nil {
					response.DeploymentStatus = deployment.Status
					deploymentWithInfo := buildDeploymentWithJenkinsInfo(h.db, deployment)
					foundDeployments = append(foundDeployments, deploymentWithInfo)
				}
			}
		}
	}

	// 方式2: 如果没有找到，尝试通过工单申请人查找类型为deployment的审批记录
	if len(foundDeployments) == 0 {
		var approvals []model.Approval
		// 查找申请人是工单申请人，类型为deployment的审批记录（时间范围扩大到工单创建时间前后24小时内）
		timeRange := ticket.CreatedAt.Add(-24 * time.Hour)
		if err := h.db.Where("applicant_id = ? AND type = ? AND created_at >= ?",
			ticket.ApplicantID, "deployment", timeRange).Order("created_at DESC").Find(&approvals).Error; err == nil {
			for _, approval := range approvals {
				if approval.DeploymentID != "" {
					var deployment model.Deployment
					if err := h.db.Where("id = ?", approval.DeploymentID).First(&deployment).Error; err == nil {
						deploymentWithInfo := buildDeploymentWithJenkinsInfo(h.db, deployment)
						foundDeployments = append(foundDeployments, deploymentWithInfo)
					}
				}
			}
		}
	}

	// 方式3: 如果还是没有找到，尝试通过工单申请人查找最近创建的部署记录（作为备选）
	if len(foundDeployments) == 0 {
		var deployments []model.Deployment
		// 查找创建人是工单申请人的部署记录（时间范围扩大到工单创建时间前后24小时内）
		timeRange := ticket.CreatedAt.Add(-24 * time.Hour)
		if err := h.db.Where("created_by = ? AND created_at >= ?",
			ticket.ApplicantID, timeRange).Order("created_at DESC").Limit(20).Find(&deployments).Error; err == nil {
			for _, deployment := range deployments {
				deploymentWithInfo := buildDeploymentWithJenkinsInfo(h.db, deployment)
				foundDeployments = append(foundDeployments, deploymentWithInfo)
			}
		}
	}

	// 方式4: 如果工单有描述或标题中包含项目名称，尝试通过项目名称查找部署记录
	if len(foundDeployments) == 0 && len(ticket.FormData) > 0 {
		var formData map[string]interface{}
		if err := json.Unmarshal(ticket.FormData, &formData); err == nil {
			// 尝试从表单数据中提取项目名称或服务名称
			var projectName string
			if name, ok := formData["project_name"].(string); ok && name != "" {
				projectName = name
			} else if name, ok := formData["service_name"].(string); ok && name != "" {
				projectName = name
			} else if name, ok := formData["application_name"].(string); ok && name != "" {
				projectName = name
			}

			if projectName != "" {
				var deployments []model.Deployment
				timeRange := ticket.CreatedAt.Add(-24 * time.Hour)
				if err := h.db.Where("project_name = ? AND created_at >= ?",
					projectName, timeRange).Order("created_at DESC").Limit(10).Find(&deployments).Error; err == nil {
					for _, deployment := range deployments {
						deploymentWithInfo := buildDeploymentWithJenkinsInfo(h.db, deployment)
						foundDeployments = append(foundDeployments, deploymentWithInfo)
					}
				}
			}
		}
	}

	response.Deployments = foundDeployments

	// 如果工单的审批人仍然为空，尝试从审批配置中读取（作为最后的备选方案）
	var finalApprovers []string
	if len(ticket.Approvers) > 0 {
		if err := json.Unmarshal(ticket.Approvers, &finalApprovers); err != nil {
			finalApprovers = []string{}
		}
	}
	if len(finalApprovers) == 0 && ticket.ApprovalPlatform != "" {
		var config model.ApprovalConfig
		if err := h.db.Where("type = ? AND enabled = ?", ticket.ApprovalPlatform, true).First(&config).Error; err == nil {
			if config.ApproverUserIDs != "" {
				var approverIDs []string
				if err := json.Unmarshal([]byte(config.ApproverUserIDs), &approverIDs); err == nil {
					var approverNamesFromConfig []string
					for _, approverID := range approverIDs {
						var user model.User
						if err := h.db.Where("id = ?", approverID).First(&user).Error; err == nil {
							approverNamesFromConfig = append(approverNamesFromConfig, user.Username)
						} else {
							approverNamesFromConfig = append(approverNamesFromConfig, approverID)
						}
					}
					if len(approverNamesFromConfig) > 0 {
						approversJSON, _ := json.Marshal(approverNamesFromConfig)
						ticket.Approvers = datatypes.JSON(approversJSON)
						finalApprovers = approverNamesFromConfig
						// 同时构建审批步骤
						if len(ticket.ApprovalSteps) == 0 {
							var steps []map[string]interface{}
							for i, approverName := range approverNamesFromConfig {
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
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    response,
	})
}

// CreateTicket 创建工单
func (h *TicketHandler) CreateTicket(c *gin.Context) {
	var ticket model.Ticket
	if err := c.ShouldBindJSON(&ticket); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 生成工单编号
	ticket.TicketNumber = generateTicketNumber()

	// 设置默认值
	if ticket.Status == "" {
		ticket.Status = "draft"
	}
	if ticket.Priority == "" {
		ticket.Priority = "normal"
	}
	// 设置工单类型：如果有 template_id，则为日常工单；否则为发布工单
	if ticket.Type == "" {
		if ticket.TemplateID != nil && *ticket.TemplateID > 0 {
			ticket.Type = "daily"
		} else {
			ticket.Type = "deployment"
		}
	}

	// 如果工单状态是已提交（非草稿），需要验证三方审批配置
	if ticket.Status != "draft" && ticket.TemplateID != nil && *ticket.TemplateID > 0 {
		var template model.FormTemplate
		if err := h.db.First(&template, *ticket.TemplateID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    400,
					"message": "工单模板不存在",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "获取工单模板失败",
				"error":   err.Error(),
			})
			return
		}

		// 检查三方审批配置
		if len(template.ApprovalConfig) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "该工单模板未配置三方审批，请先在\"设计工单\"页面完善三方审批配置后再提交工单",
			})
			return
		}

		// 解析审批配置
		var approvalConfig map[string]interface{}
		if err := json.Unmarshal(template.ApprovalConfig, &approvalConfig); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "工单模板的三方审批配置格式错误",
				"error":   err.Error(),
			})
			return
		}

		// 获取平台类型（如果模板配置中有 platform，使用模板配置的，否则兼容旧数据）
		platform := ""
		if templatePlatform, ok := approvalConfig["platform"].(string); ok && templatePlatform != "" {
			platform = templatePlatform
		}

		// 验证审批代码是否存在（根据平台类型使用不同的字段名）
		var approvalCode string
		var hasApprovalCode bool
		if platform == "dingtalk" {
			// 钉钉使用 process_code（虽然模板配置中可能还是叫 approval_code，但这里做兼容）
			if code, ok := approvalConfig["approval_code"].(string); ok && code != "" {
				approvalCode = code
				hasApprovalCode = true
			}
		} else if platform == "wechat" {
			// 企业微信使用 template_id（虽然模板配置中可能还是叫 approval_code，但这里做兼容）
			if code, ok := approvalConfig["approval_code"].(string); ok && code != "" {
				approvalCode = code
				hasApprovalCode = true
			}
		} else {
			// 飞书或其他平台使用 approval_code
			approvalCode, hasApprovalCode = approvalConfig["approval_code"].(string)
		}

		if !hasApprovalCode || approvalCode == "" {
			platformName := "三方审批"
			if platform == "feishu" {
				platformName = "飞书审批"
			} else if platform == "dingtalk" {
				platformName = "钉钉审批"
			} else if platform == "wechat" {
				platformName = "企业微信审批"
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": fmt.Sprintf("该工单模板未配置%s代码，请先在\"设计工单\"页面完善三方审批配置后再提交工单", platformName),
			})
			return
		}

		// 验证 app_id 和 app_secret 是否配置
		appID, hasAppID := approvalConfig["app_id"].(string)
		appSecret, hasAppSecret := approvalConfig["app_secret"].(string)
		if !hasAppID || appID == "" || !hasAppSecret || appSecret == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "该工单模板的三方审批配置中缺少应用ID(App ID)或应用密钥(App Secret)，请先在\"设计工单\"页面完善三方审批配置后再提交工单",
			})
			return
		}
	}

	// 从上下文获取当前用户信息（如果中间件设置了）
	if userID, exists := c.Get("user_id"); exists {
		if ticket.ApplicantID == "" {
			ticket.ApplicantID = userID.(string)
		}
	}
	if userName, exists := c.Get("username"); exists {
		if ticket.ApplicantName == "" {
			ticket.ApplicantName = userName.(string)
		}
	}

	if err := h.db.Create(&ticket).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建工单失败",
			"error":   err.Error(),
		})
		return
	}

	// 重新加载以获取关联数据（如果 template_id 不为空）
	if ticket.TemplateID != nil && *ticket.TemplateID > 0 {
		if ticket.TemplateID != nil && *ticket.TemplateID > 0 {
			h.db.Preload("Template").First(&ticket, ticket.ID)
		} else {
			h.db.First(&ticket, ticket.ID)
		}
	} else {
		h.db.First(&ticket, ticket.ID)
	}

	// 如果工单有模板且模板配置了第三方审批，自动创建审批实例
	if ticket.TemplateID != nil && *ticket.TemplateID > 0 && ticket.Status == "submitted" {
		go h.autoCreateThirdPartyApproval(&ticket, c)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    ticket,
	})
}

// autoCreateThirdPartyApproval 自动创建第三方审批实例（异步执行，不阻塞工单创建）
func (h *TicketHandler) autoCreateThirdPartyApproval(ticket *model.Ticket, c *gin.Context) {
	// 获取模板信息
	var template model.FormTemplate
	if err := h.db.First(&template, ticket.TemplateID).Error; err != nil {
		fmt.Printf("自动创建审批失败：获取模板失败: %v\n", err)
		return
	}

	// 检查模板是否有审批配置
	if len(template.ApprovalConfig) == 0 {
		return // 没有配置审批，直接返回
	}

	// 解析审批配置
	var approvalConfig map[string]interface{}
	if err := json.Unmarshal(template.ApprovalConfig, &approvalConfig); err != nil {
		fmt.Printf("自动创建审批失败：解析模板审批配置失败: %v\n", err)
		return
	}

	// 获取平台类型
	platform := ""
	if templatePlatform, ok := approvalConfig["platform"].(string); ok && templatePlatform != "" {
		platform = templatePlatform
	} else {
		return // 没有配置平台，直接返回
	}

	// 获取审批代码
	var approvalCode string
	var hasApprovalCode bool
	if platform == "dingtalk" || platform == "wechat" {
		if code, ok := approvalConfig["approval_code"].(string); ok && code != "" {
			approvalCode = code
			hasApprovalCode = true
		}
	} else {
		approvalCode, hasApprovalCode = approvalConfig["approval_code"].(string)
	}

	if !hasApprovalCode || approvalCode == "" {
		fmt.Printf("自动创建审批失败：模板未配置审批代码\n")
		return
	}

	// 构建审批配置：必须从模板配置中读取 app_id 和 app_secret
	var config model.ApprovalConfig
	
	// 检查模板配置中是否包含 app_id 和 app_secret（提交时已验证，这里再次检查以确保安全）
	templateAppID, hasTemplateAppID := approvalConfig["app_id"].(string)
	templateAppSecret, hasTemplateAppSecret := approvalConfig["app_secret"].(string)
	
	if !hasTemplateAppID || templateAppID == "" || !hasTemplateAppSecret || templateAppSecret == "" {
		platformName := "三方审批"
		if platform == "feishu" {
			platformName = "飞书审批"
		} else if platform == "dingtalk" {
			platformName = "钉钉审批"
		} else if platform == "wechat" {
			platformName = "企业微信审批"
		}
		fmt.Printf("工单 %s 自动创建%s失败：模板配置中缺少app_id或app_secret，请先在\"设计工单\"页面完善三方审批配置\n", 
			ticket.TicketNumber, platformName)
		return
	}
	
	// 从模板配置构建 ApprovalConfig
	config.Type = platform
	config.AppID = templateAppID
	config.AppSecret = templateAppSecret
	config.ApprovalCode = approvalCode
	
	// 从模板配置中读取其他字段
	if apiBaseURL, ok := approvalConfig["api_base_url"].(string); ok && apiBaseURL != "" {
		config.APIBaseURL = apiBaseURL
	}
	if callbackURL, ok := approvalConfig["callback_url"].(string); ok && callbackURL != "" {
		config.CallbackURL = callbackURL
	}
	
	// 从模板配置中读取审批人信息
	if approverUserIDs, ok := approvalConfig["approver_user_ids"].([]interface{}); ok && len(approverUserIDs) > 0 {
		approverIDsJSON, _ := json.Marshal(approverUserIDs)
		config.ApproverUserIDs = string(approverIDsJSON)
	}

	// 获取字段映射配置
	var fieldMappings []map[string]interface{}
	if mappingsArray, ok := approvalConfig["field_mappings"].([]interface{}); ok {
		fieldMappings = make([]map[string]interface{}, 0, len(mappingsArray))
		for _, m := range mappingsArray {
			if mapping, ok := m.(map[string]interface{}); ok {
				fieldMappings = append(fieldMappings, mapping)
			}
		}
	}
	// 如果配置中没有FormFields，尝试从飞书API获取表单详情（仅飞书平台）
	var formFieldsFromAPI []map[string]interface{}
	if config.FormFields == "" && platform == "feishu" {
		// 创建临时的provider实例来获取表单详情
		platformType := model.ApprovalPlatform(platform)
		var tempProvider approval.Provider
		switch platformType {
		case model.ApprovalPlatformFeishu:
			tempProvider = approval.NewFeishuProvider(&config, h.db)
		default:
			tempProvider = nil
		}
		if tempProvider != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			formFields, err := tempProvider.GetApprovalFormDetail(ctx, approvalCode)
			cancel()
			if err != nil {
				fmt.Printf("工单 %s 自动创建审批失败：从飞书API获取表单详情失败: %v\n", ticket.TicketNumber, err)
			} else {
				formFieldsFromAPI = formFields
				// 将表单详情保存到config中，供后续使用
				if formFieldsJSON, err := json.Marshal(formFieldsFromAPI); err == nil {
					config.FormFields = string(formFieldsJSON)
				}
			}
		}
	}

	// 构建表单数据
	formData := h.buildFormDataFromTicket(ticket, fieldMappings, &config)
	if formData == "" {
		fmt.Printf("工单 %s 自动创建审批失败：构建表单数据失败 (平台: %s, 审批代码: %s)\n", ticket.TicketNumber, platform, approvalCode)
		return
	}

	// 获取当前用户信息
	userID, _ := c.Get("user_id")
	userName, _ := c.Get("username")
	applicantID := ticket.ApplicantID
	applicantName := ticket.ApplicantName
	
	// 查询用户的用户名（Username），用于飞书等第三方平台
	var applicantUsername string
	if applicantID != "" {
		var user model.User
		if err := h.db.Where("id = ?", applicantID).First(&user).Error; err == nil {
			applicantUsername = user.Username
		}
	}
	
	// 如果查询不到用户名，使用context中的username或ApplicantName
	if applicantUsername == "" {
		if userName != nil {
			applicantUsername = fmt.Sprintf("%v", userName)
		} else if applicantName != "" {
			// 如果ApplicantName不是UUID格式，可能是用户名，直接使用
			// 如果是UUID格式，尝试查询
			if len(applicantName) == 36 && strings.Contains(applicantName, "-") {
				// 可能是UUID，尝试查询
				var user model.User
				if err := h.db.Where("id = ?", applicantName).First(&user).Error; err == nil {
					applicantUsername = user.Username
				} else {
					applicantUsername = applicantName
				}
			} else {
				// 不是UUID，可能是用户名，直接使用
				applicantUsername = applicantName
			}
		} else {
			fmt.Printf("工单 %s 自动创建审批失败：无法获取申请人用户名\n", ticket.TicketNumber)
			return
		}
	}
	
	if userID != nil && applicantID == "" {
		applicantID = fmt.Sprintf("%v", userID)
	}
	if userName != nil && applicantName == "" {
		applicantName = fmt.Sprintf("%v", userName)
	}

	// 创建 provider 实例
	platformType := model.ApprovalPlatform(platform)
	var providerInstance approval.Provider
	switch platformType {
	case model.ApprovalPlatformFeishu:
		providerInstance = approval.NewFeishuProvider(&config, h.db)
	case model.ApprovalPlatformDingTalk:
		providerInstance = approval.NewDingTalkProvider(&config, h.db)
	case model.ApprovalPlatformWeChat:
		providerInstance = approval.NewWeChatProvider(&config, h.db)
	default:
		fmt.Printf("自动创建审批失败：不支持的审批平台: %s\n", platform)
		return
	}

	// 构建审批对象
	now := time.Now()
	approvalRecord := &model.Approval{
		ID:            uuid.New().String(),
		Title:         ticket.Title,
		Description:   fmt.Sprintf("工单编号: %s", ticket.TicketNumber),
		Type:          model.ApprovalTypeDeployment,
		Status:        model.ApprovalStatusPending,
		Platform:      platformType,
		ApplicantID:   applicantID,
		ApplicantName: applicantUsername, // 使用用户名而不是fullName，用于第三方平台
		CreatedAt:     now,
		UpdatedAt:     now,
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

	// 保存工单表单数据到 ExternalData（供 provider 使用）
	externalData := map[string]interface{}{
		"ticket_form_data": ticket.FormData,
		"field_mappings":   fieldMappings,
	}
	externalDataJSON, _ := json.Marshal(externalData)
	approvalRecord.ExternalData = string(externalDataJSON)

	// 调用 provider 创建审批实例
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	externalID, err := providerInstance.CreateApprovalWithFormData(ctx, approvalCode, formData, approvalRecord)
	if err != nil {
		fmt.Printf("自动创建审批失败：创建第三方审批实例失败: %v\n", err)
		return
	}

	// 更新审批对象
	approvalRecord.ExternalID = externalID
	approvalRecord.ExternalURL = h.getDefaultExternalURL(platformType, externalID)

	// 保存到数据库
	if err := h.db.Create(approvalRecord).Error; err != nil {
		fmt.Printf("自动创建审批失败：保存审批记录失败: %v\n", err)
		return
	}

	// 更新工单的审批信息
	ticket.ApprovalPlatform = platform
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
	if err := h.db.Save(ticket).Error; err != nil {
		fmt.Printf("自动创建审批失败：更新工单审批信息失败: %v\n", err)
		return
	}
}

// buildFormDataFromTicket 从工单数据构建审批表单数据
func (h *TicketHandler) buildFormDataFromTicket(ticket *model.Ticket, fieldMappings []map[string]interface{}, config *model.ApprovalConfig) string {
	// 解析配置的表单字段
	var formFields []map[string]interface{}
	if config.FormFields != "" {
		if err := json.Unmarshal([]byte(config.FormFields), &formFields); err != nil {
			fmt.Printf("构建表单数据失败：解析表单字段配置失败: %v\n", err)
			return ""
		}
	} else {
		// 如果 FormFields 为空，使用 field_mappings 直接构建表单数据
		if len(fieldMappings) == 0 {
			fmt.Printf("构建表单数据失败：配置FormFields为空且字段映射为空，无法构建表单数据\n")
			return ""
		}
		// 从字段映射构建表单字段结构（用于后续处理）
		formFields = make([]map[string]interface{}, 0, len(fieldMappings))
		for _, mapping := range fieldMappings {
			widgetId := ""
			if wId, ok := mapping["widgetId"].(string); ok && wId != "" {
				widgetId = wId
			}
			if widgetId != "" {
				// 构建一个简单的字段结构，type 默认为 "input"（飞书审批的默认类型）
				formFields = append(formFields, map[string]interface{}{
					"id":   widgetId,
					"type": "input", // 默认类型，飞书审批会根据实际字段类型自动处理
				})
			}
		}
	}

	// 构建 Widget ID 到关键字的映射
	widgetIdToKeyword := make(map[string]string)
	fieldNameToKeyword := make(map[string]string)

	if len(fieldMappings) > 0 {
		for _, mapping := range fieldMappings {
			var widgetId, fieldName, keyword string

			if fName, ok := mapping["fieldName"].(string); ok && fName != "" {
				fieldName = fName
			}
			if wId, ok := mapping["widgetId"].(string); ok && wId != "" {
				widgetId = wId
			}
			if k, ok := mapping["keyword"].(string); ok && k != "" {
				keyword = k
			} else if fieldName != "" {
				keyword = fieldName
			}

			if widgetId != "" && keyword != "" {
				widgetIdToKeyword[widgetId] = keyword
			}
			if fieldName != "" && keyword != "" {
				fieldNameToKeyword[fieldName] = keyword
			}
		}
	}

	// 解析工单表单数据
	ticketFormData := make(map[string]interface{})
	if ticket.FormData != nil {
		if formDataBytes, err := json.Marshal(ticket.FormData); err == nil {
			if err := json.Unmarshal(formDataBytes, &ticketFormData); err != nil {
				fmt.Printf("构建表单数据失败：解析工单表单数据失败: %v\n", err)
			}
		}
	}

	// 构建表单数据数组
	var formData []map[string]interface{}
	for _, field := range formFields {
		formItem := map[string]interface{}{
			"id":   field["id"],
			"type": field["type"],
		}

		widgetId := ""
		if id, ok := field["id"].(string); ok {
			widgetId = id
		}

		fieldName := ""
		if name, ok := field["name"].(string); ok {
			fieldName = name
		}

		// 根据映射配置获取值
		var keyword string
		var exists bool

		if widgetId != "" {
			keyword, exists = widgetIdToKeyword[widgetId]
		}

		if !exists && fieldName != "" {
			keyword, exists = fieldNameToKeyword[fieldName]
			if !exists {
				keyword = fieldName
				exists = true
			}
		}

		if exists && keyword != "" {
			// 从工单数据中获取值
			var value interface{}
			var found bool
			
			// 首先尝试直接匹配关键字
			if v, ok := ticketFormData[keyword]; ok {
				value = v
				found = true
			} else {
				// 如果关键字包含路径分隔符（如 td_GENTtvMGgJ.td_OcuEwMrgCw），尝试提取最后一部分
				if strings.Contains(keyword, ".") {
					parts := strings.Split(keyword, ".")
					if len(parts) > 0 {
						lastPart := parts[len(parts)-1]
						if v, ok := ticketFormData[lastPart]; ok {
							value = v
							found = true
						}
					}
				}
			}
			
			if found {
				formItem["value"] = value
			} else {
				formItem["value"] = ""
			}
		} else {
			formItem["value"] = ""
		}

		formData = append(formData, formItem)
	}

	jsonData, err := json.Marshal(formData)
	if err != nil {
		fmt.Printf("构建表单数据失败：序列化表单数据失败: %v\n", err)
		return ""
	}
	return string(jsonData)
}

// getDefaultExternalURL 获取默认的外部审批链接
func (h *TicketHandler) getDefaultExternalURL(platform model.ApprovalPlatform, externalID string) string {
	switch platform {
	case model.ApprovalPlatformFeishu:
		return fmt.Sprintf("https://www.feishu.cn/approval/open/approval_detail?instance_code=%s", externalID)
	case model.ApprovalPlatformDingTalk:
		return fmt.Sprintf("https://oa.dingtalk.com/approval/detail?processInstanceId=%s", externalID)
	case model.ApprovalPlatformWeChat:
		return fmt.Sprintf("https://work.weixin.qq.com/wework_admin/frame#approval/detail?spname=approval&instanceid=%s", externalID)
	default:
		return ""
	}
}

// UpdateTicket 更新工单
func (h *TicketHandler) UpdateTicket(c *gin.Context) {
	id := c.Param("id")

	var ticket model.Ticket
	if err := h.db.First(&ticket, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "工单不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取工单失败",
			"error":   err.Error(),
		})
		return
	}

	var updateData model.Ticket
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 更新字段
	if updateData.Title != "" {
		ticket.Title = updateData.Title
	}
	if updateData.FormData != nil {
		ticket.FormData = updateData.FormData
	}
	if updateData.Status != "" {
		ticket.Status = updateData.Status
	}
	if updateData.Priority != "" {
		ticket.Priority = updateData.Priority
	}

	if err := h.db.Save(&ticket).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新工单失败",
			"error":   err.Error(),
		})
		return
	}

	if ticket.TemplateID != nil && *ticket.TemplateID > 0 {
		h.db.Preload("Template").First(&ticket, ticket.ID)
	} else {
		h.db.First(&ticket, ticket.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    ticket,
	})
}

// SubmitTicket 提交工单
func (h *TicketHandler) SubmitTicket(c *gin.Context) {
	id := c.Param("id")

	var ticket model.Ticket
	if err := h.db.First(&ticket, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "工单不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取工单失败",
			"error":   err.Error(),
		})
		return
	}

	if ticket.Status != "draft" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "只能提交草稿状态的工单",
		})
		return
	}

	ticket.Status = "submitted"
	if err := h.db.Save(&ticket).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交工单失败",
			"error":   err.Error(),
		})
		return
	}

	if ticket.TemplateID != nil && *ticket.TemplateID > 0 {
		h.db.Preload("Template").First(&ticket, ticket.ID)
	} else {
		h.db.First(&ticket, ticket.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    ticket,
	})
}

// CancelTicket 取消工单
func (h *TicketHandler) CancelTicket(c *gin.Context) {
	id := c.Param("id")

	var ticket model.Ticket
	if err := h.db.First(&ticket, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "工单不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取工单失败",
			"error":   err.Error(),
		})
		return
	}

	if ticket.Status == "approved" || ticket.Status == "rejected" || ticket.Status == "cancelled" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "该状态的工单无法取消",
		})
		return
	}

	ticket.Status = "cancelled"
	if err := h.db.Save(&ticket).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "取消工单失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    ticket,
	})
}

// GetRenderForm 获取渲染表单（用于编辑）
func (h *TicketHandler) GetRenderForm(c *gin.Context) {
	id := c.Param("id")

	var ticket model.Ticket
	// 先加载工单，然后根据是否有 template_id 决定是否加载模板
	if err := h.db.First(&ticket, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "工单不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取工单失败",
			"error":   err.Error(),
		})
		return
	}

	// 如果工单有 template_id，加载模板
	if ticket.TemplateID != nil && *ticket.TemplateID > 0 {
		h.db.Preload("Template").First(&ticket, ticket.ID)
	}

	// 如果工单没有模板，返回空 schema
	var schema interface{}
	if ticket.TemplateID != nil && *ticket.TemplateID > 0 && ticket.Template.ID > 0 {
		schema = ticket.Template.Schema
	} else {
		schema = nil
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"schema":    schema,
			"form_data": ticket.FormData,
		},
	})
}
