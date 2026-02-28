package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ApprovalCallbackHandler 审批回调处理器
type ApprovalCallbackHandler struct {
	db *gorm.DB
}

// NewApprovalCallbackHandler 创建审批回调处理器
func NewApprovalCallbackHandler(db *gorm.DB) *ApprovalCallbackHandler {
	return &ApprovalCallbackHandler{
		db: db,
	}
}

// FeishuCallbackRequest 飞书回调请求结构
type FeishuCallbackRequest struct {
	Type      string `json:"type"`
	AppID     string `json:"app_id"`
	TenantKey string `json:"tenant_key"`
	Token     string `json:"token"`
	Challenge string `json:"challenge"`
	Header    struct {
		EventType  string `json:"event_type"`
		Token      string `json:"token"`
		CreateTime string `json:"create_time"`
		EventID    string `json:"event_id"`
		AppID      string `json:"app_id"`
		TenantKey  string `json:"tenant_key"`
	} `json:"header"`
	Event struct {
		AppID        string `json:"app_id"`
		TenantKey    string `json:"tenant_key"`
		Type         string `json:"type"`
		InstanceCode string `json:"instance_code"`
		UserID       string `json:"user_id"`
		OpenID       string `json:"open_id"`
		StartTime    int64  `json:"start_time"`
		EndTime      int64  `json:"end_time"`
		Status       string `json:"status"`
		Form         string `json:"form"`
		TaskID       string `json:"task_id"`
		Comment      string `json:"comment"`
		OperateTime  int64  `json:"operate_time"`
		Operator     struct {
			UserID string `json:"user_id"`
			OpenID string `json:"open_id"`
		} `json:"operator"`
	} `json:"event"`
}

// DingTalkCallbackRequest 钉钉回调请求结构
type DingTalkCallbackRequest struct {
	EventType             string `json:"eventType"`
	ProcessInstanceID     string `json:"processInstanceId"`
	CorpID                string `json:"corpId"`
	EventTime             int64  `json:"eventTime"`
	Type                  string `json:"type"`
	Title                 string `json:"title"`
	Content               string `json:"content"`
	URL                   string `json:"url"`
	ProcessCode           string `json:"processCode"`
	BizCategoryID         string `json:"bizCategoryId"`
	BizType               string `json:"bizType"`
	TitleInEnglish        string `json:"titleInEnglish"`
	BizNumber             string `json:"bizNumber"`
	MainProcessInstanceID string `json:"mainProcessInstanceId"`
	Data                  struct {
		ProcessInstanceID   string `json:"processInstanceId"`
		OriginatorUserID    string `json:"originatorUserId"`
		OriginatorDeptID    string `json:"originatorDeptId"`
		Title               string `json:"title"`
		CreateTime          string `json:"createTime"`
		FinishTime          string `json:"finishTime"`
		Status              string `json:"status"`
		Result              string `json:"result"`
		BusinessID          string `json:"businessId"`
		FormComponentValues []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"formComponentValues"`
		OperationRecords []struct {
			UserID          string `json:"userId"`
			Date            string `json:"date"`
			OperationType   string `json:"operationType"`
			OperationResult string `json:"operationResult"`
			Remark          string `json:"remark"`
		} `json:"operationRecords"`
		Tasks []struct {
			UserID     string `json:"userId"`
			TaskID     string `json:"taskId"`
			CreateTime string `json:"createTime"`
			FinishTime string `json:"finishTime"`
			TaskResult string `json:"taskResult"`
			TaskStatus string `json:"taskStatus"`
		} `json:"tasks"`
	} `json:"data"`
}

// WeChatCallbackRequest 企业微信回调请求结构
type WeChatCallbackRequest struct {
	ToUserName   string `json:"ToUserName"`
	FromUserName string `json:"FromUserName"`
	CreateTime   int64  `json:"CreateTime"`
	MsgType      string `json:"MsgType"`
	Event        string `json:"Event"`
	EventKey     string `json:"EventKey"`
	AgentID      int    `json:"AgentID"`
	Content      struct {
		ProcessInstanceID string `json:"process_instance_id"`
		ProcessCode       string `json:"process_code"`
		ProcessType       string `json:"process_type"`
		Title             string `json:"title"`
		CreateTime        int64  `json:"create_time"`
		FinishTime        int64  `json:"finish_time"`
		Status            string `json:"status"`
		Result            string `json:"result"`
		FormData          []struct {
			Title string `json:"title"`
			Value string `json:"value"`
		} `json:"form_data"`
		ApproverInfo []struct {
			UserID string `json:"userid"`
			Status string `json:"status"`
		} `json:"approver_info"`
	} `json:"content"`
}

// HandleFeishuCallback 处理飞书审批回调
func (h *ApprovalCallbackHandler) HandleFeishuCallback(c *gin.Context) {
	var req FeishuCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// 处理URL验证
	if req.Type == "url_verification" {
		c.JSON(http.StatusOK, gin.H{"challenge": req.Challenge})
		return
	}

	// 处理审批事件
	if req.Header.EventType == "approval_instance" {
		err := h.handleFeishuApprovalEvent(&req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

// HandleDingTalkCallback 处理钉钉审批回调
func (h *ApprovalCallbackHandler) HandleDingTalkCallback(c *gin.Context) {
	var req DingTalkCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// 处理审批事件
	if req.EventType == "bpms_task_change" || req.EventType == "bpms_instance_change" {
		err := h.handleDingTalkApprovalEvent(&req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

// HandleWeChatCallback 处理企业微信审批回调
func (h *ApprovalCallbackHandler) HandleWeChatCallback(c *gin.Context) {
	var req WeChatCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// 处理审批事件
	if req.Event == "sys_approval_change" {
		err := h.handleWeChatApprovalEvent(&req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

// getFeishuInstanceDetail 获取飞书审批实例详情（包含 task_list）
func (h *ApprovalCallbackHandler) getFeishuInstanceDetail(instanceCode string) (map[string]interface{}, error) {
	// 查找审批记录以获取配置信息
	var approval model.Approval
	if err := h.db.Where("external_id = ?", instanceCode).First(&approval).Error; err != nil {
		return nil, fmt.Errorf("未找到审批记录: %v", err)
	}

	// 查找审批配置
	var config model.ApprovalConfig
	if err := h.db.Where("type = ? AND enabled = ?", string(approval.Platform), true).First(&config).Error; err != nil {
		return nil, fmt.Errorf("未找到审批配置: %v", err)
	}

	// 获取 token
	token, err := h.getFeishuToken(&config)
	if err != nil {
		return nil, fmt.Errorf("获取token失败: %v", err)
	}

	// 构建 API URL
	baseURL := config.APIBaseURL
	if baseURL == "" {
		baseURL = "https://open.larksuite.com/open-apis"
	}
	apiPath := config.APIPathGet
	if apiPath == "" {
		apiPath = "/approval/v4/instances"
	}
	url := fmt.Sprintf("%s%s/%s", baseURL, apiPath, instanceCode)

	// 发起请求
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code int                    `json:"code"`
		Msg  string                 `json:"msg"`
		Data map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("获取实例详情失败: code=%d, msg=%s", result.Code, result.Msg)
	}

	return result.Data, nil
}

// getFeishuToken 获取飞书访问 token
func (h *ApprovalCallbackHandler) getFeishuToken(config *model.ApprovalConfig) (string, error) {
	baseURL := config.APIBaseURL
	if baseURL == "" {
		baseURL = "https://open.larksuite.com/open-apis"
	}
	url := fmt.Sprintf("%s/auth/v3/tenant_access_token/internal", baseURL)

	reqBody := map[string]string{
		"app_id":     config.AppID,
		"app_secret": config.AppSecret,
	}

	body, _ := json.Marshal(reqBody)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("获取token失败: code=%d, msg=%s", result.Code, result.Msg)
	}

	return result.TenantAccessToken, nil
}

// convertUserIDToUsername 将用户ID转换为用户名（如果是UUID则查询数据库）
func (h *ApprovalCallbackHandler) convertUserIDToUsername(userID string) string {
	if userID == "" {
		return ""
	}
	// 如果是UUID格式，查询用户名
	if len(userID) == 36 && strings.Contains(userID, "-") {
		var user model.User
		if err := h.db.Where("id = ?", userID).First(&user).Error; err == nil {
			return user.Username
		}
	}
	// 如果不是UUID或查询失败，直接返回（可能是用户名）
	return userID
}

// updateApprovalRecordStatus 更新审批记录的状态
func (h *ApprovalCallbackHandler) updateApprovalRecordStatus(approval *model.Approval, status model.ApprovalStatus, comment string, finishTime *time.Time, currentApproverName string) {
	approval.Status = status
	approval.UpdatedAt = time.Now()
	
	if currentApproverName != "" {
		approval.CurrentApprover = currentApproverName
	}

	switch status {
	case model.ApprovalStatusApproved:
		if finishTime != nil {
			approval.ApprovedAt = finishTime
		} else {
			now := time.Now()
			approval.ApprovedAt = &now
		}
		approval.ApprovalNote = comment
	case model.ApprovalStatusRejected:
		if finishTime != nil {
			approval.RejectedAt = finishTime
		} else {
			now := time.Now()
			approval.RejectedAt = &now
		}
		approval.RejectReason = comment
	default:
		fmt.Printf("警告: 未预期的审批状态 %s，不需要设置 ApprovedAt 或 RejectedAt\n", status)
	}
}

// updateTicketStatus 更新工单状态
func (h *ApprovalCallbackHandler) updateTicketStatus(ticket *model.Ticket, status model.ApprovalStatus, comment string, currentApproverName string) {
	switch status {
	case model.ApprovalStatusApproved:
		ticket.Status = "approved"
		ticket.ApprovalResult = "approved"
		ticket.ApprovalComment = comment
	case model.ApprovalStatusRejected:
		ticket.Status = "rejected"
		ticket.ApprovalResult = "rejected"
		ticket.ApprovalComment = comment
	case model.ApprovalStatusCanceled:
		ticket.Status = "cancelled"
		ticket.ApprovalResult = "canceled"
		ticket.ApprovalComment = comment
	default:
		fmt.Printf("警告: 未预期的审批状态 %s，不更新工单状态\n", status)
	}
	if currentApproverName != "" {
		ticket.CurrentApprover = currentApproverName
	}
}

// updateTicketFromApproval 根据审批状态更新关联的工单
func (h *ApprovalCallbackHandler) updateTicketFromApproval(externalID string, status model.ApprovalStatus, comment string, currentApproverName string, approverStatuses map[string]string) error {
	var ticket model.Ticket
	if err := h.db.Where("approval_instance_id = ?", externalID).First(&ticket).Error; err != nil {
		// 如果找不到工单，不返回错误（可能工单已被删除）
		return nil
	}

	// 更新工单状态
	h.updateTicketStatus(&ticket, status, comment, currentApproverName)

	// 更新审批步骤（只有在有状态信息时才更新）
	if len(approverStatuses) > 0 {
		if err := h.updateTicketApprovalSteps(&ticket, approverStatuses); err != nil {
			// 记录错误但不中断流程
			fmt.Printf("更新工单审批步骤失败: %v\n", err)
		}
	}

	return h.db.Save(&ticket).Error
}

// updateTicketApprovalSteps 更新工单的审批步骤
func (h *ApprovalCallbackHandler) updateTicketApprovalSteps(ticket *model.Ticket, approverStatuses map[string]string) error {
	// 解析现有的审批步骤
	var stepsInterface []interface{}
	if len(ticket.ApprovalSteps) > 0 {
		if err := json.Unmarshal(ticket.ApprovalSteps, &stepsInterface); err != nil {
			stepsInterface = []interface{}{}
		}
	}

	var steps []map[string]interface{}
	// 如果没有审批步骤，从审批人列表构建
	if len(stepsInterface) == 0 {
		var approvers []string
		if len(ticket.Approvers) > 0 {
			if err := json.Unmarshal(ticket.Approvers, &approvers); err == nil {
				for _, approver := range approvers {
					status := approverStatuses[approver]
					if status == "" {
						status = "pending"
					}
					comment := approverStatuses[approver+"_comment"]
					steps = append(steps, map[string]interface{}{
						"approver": approver,
						"status":   status,
						"comment":  comment,
					})
				}
			}
		}
	} else {
		// 转换并更新现有步骤的状态
		for _, stepInterface := range stepsInterface {
			if stepMap, ok := stepInterface.(map[string]interface{}); ok {
				approver, _ := stepMap["approver"].(string)
				if approver != "" {
					// 如果approverStatuses中有对应的状态，更新；否则保留原有状态
					if status, exists := approverStatuses[approver]; exists {
						stepMap["status"] = status
					}
					// 如果approverStatuses中有对应的审批意见，更新；否则保留原有意见
					if comment, exists := approverStatuses[approver+"_comment"]; exists {
						stepMap["comment"] = comment
					}
				}
				steps = append(steps, stepMap)
			}
		}
	}

	// 保存更新后的审批步骤
	stepsJSON, err := json.Marshal(steps)
	if err != nil {
		return err
	}
	ticket.ApprovalSteps = datatypes.JSON(stepsJSON)
	return nil
}

// handleFeishuApprovalEvent 处理飞书审批事件
func (h *ApprovalCallbackHandler) handleFeishuApprovalEvent(req *FeishuCallbackRequest) error {
	// 查找对应的审批记录
	var approval model.Approval
	err := h.db.Where("external_id = ?", req.Event.InstanceCode).First(&approval).Error
	if err != nil {
		return fmt.Errorf("未找到对应的审批记录: %v", err)
	}

	// 获取实例详情（包含 task_list）
	instanceDetail, err := h.getFeishuInstanceDetail(req.Event.InstanceCode)
	if err != nil {
		// 如果获取详情失败，仍然处理基本状态更新
		fmt.Printf("获取实例详情失败，仅更新基本状态: %v\n", err)
	} else {
		// 从 task_list 中提取审批人信息
		if taskList, ok := instanceDetail["task_list"].([]interface{}); ok {
			var allApproverIDs []string
			for _, task := range taskList {
				if taskMap, ok := task.(map[string]interface{}); ok {
					if userID, _ := taskMap["user_id"].(string); userID != "" {
						allApproverIDs = append(allApproverIDs, userID)
					}
				}
			}
			// 更新审批人信息
			if len(allApproverIDs) > 0 {
				approval.ApproverIDs = allApproverIDs
			}
		}
	}

	// 更新审批状态
	status := h.mapFeishuStatus(req.Event.Status)
	
	// 更新当前审批人（转换为用户名）
	currentApproverName := ""
	if req.Event.Operator.UserID != "" {
		currentApproverName = h.convertUserIDToUsername(req.Event.Operator.UserID)
	}

	// 计算完成时间
	var finishTime *time.Time
	if req.Event.EndTime > 0 {
		ft := time.Unix(req.Event.EndTime/1000, 0)
		finishTime = &ft
	}

	// 更新审批记录状态
	h.updateApprovalRecordStatus(&approval, status, req.Event.Comment, finishTime, currentApproverName)

	// 保存审批记录更新
	if err := h.db.Save(&approval).Error; err != nil {
		return err
	}

	// 构建审批步骤状态映射
	approverStatuses := make(map[string]string)
	if instanceDetail != nil {
		if taskList, ok := instanceDetail["task_list"].([]interface{}); ok {
			for _, task := range taskList {
				if taskMap, ok := task.(map[string]interface{}); ok {
					userID, _ := taskMap["user_id"].(string)
					taskStatus, _ := taskMap["status"].(string)
					comment, _ := taskMap["comment"].(string)
					if userID != "" {
						approverName := h.convertUserIDToUsername(userID)
						if approverName != "" {
							// 映射飞书状态到系统状态
							var stepStatus string
							switch taskStatus {
							case "APPROVED":
								stepStatus = "approved"
							case "REJECTED":
								stepStatus = "rejected"
							case "CANCELLED", "CANCELED":
								stepStatus = "canceled"
							default:
								fmt.Printf("警告: 未预期的飞书任务状态 %s，默认为 pending\n", taskStatus)
								stepStatus = "pending"
							}
							approverStatuses[approverName] = stepStatus
							if comment != "" {
								approverStatuses[approverName+"_comment"] = comment
							}
						}
					}
				}
			}
		}
	}
	// 如果当前操作人有状态，也更新（优先使用task_list中的状态，如果没有则使用整体状态）
	if currentApproverName != "" {
		if _, exists := approverStatuses[currentApproverName]; !exists {
			switch status {
			case model.ApprovalStatusApproved:
				approverStatuses[currentApproverName] = "approved"
			case model.ApprovalStatusRejected:
				approverStatuses[currentApproverName] = "rejected"
			case model.ApprovalStatusCanceled:
				approverStatuses[currentApproverName] = "canceled"
			default:
				fmt.Printf("警告: 未预期的审批状态 %s，不设置审批人状态\n", status)
			}
		}
		// 如果当前操作人有审批意见，更新（优先使用task_list中的comment）
		if req.Event.Comment != "" {
			commentKey := currentApproverName + "_comment"
			if _, exists := approverStatuses[commentKey]; !exists {
				approverStatuses[commentKey] = req.Event.Comment
			}
		}
	}

	// 更新关联的工单状态
	h.updateTicketFromApproval(req.Event.InstanceCode, status, req.Event.Comment, currentApproverName, approverStatuses)

	return nil
}

// handleDingTalkApprovalEvent 处理钉钉审批事件
func (h *ApprovalCallbackHandler) handleDingTalkApprovalEvent(req *DingTalkCallbackRequest) error {
	// 查找对应的审批记录
	var approval model.Approval
	err := h.db.Where("external_id = ?", req.ProcessInstanceID).First(&approval).Error
	if err != nil {
		return fmt.Errorf("未找到对应的审批记录: %v", err)
	}

	// 从 Tasks 中提取审批人信息
	var allApproverIDs []string
	if len(req.Data.Tasks) > 0 {
		for _, task := range req.Data.Tasks {
			if task.UserID != "" {
				allApproverIDs = append(allApproverIDs, task.UserID)
			}
		}
		// 更新审批人列表
		if len(allApproverIDs) > 0 {
			approval.ApproverIDs = allApproverIDs
		}
	}

	// 更新审批状态
	status := h.mapDingTalkStatus(req.Data.Status)

	// 提取审批意见（从操作记录中获取最新的）
	var comment string
	var currentApproverUserID string
	if len(req.Data.OperationRecords) > 0 {
		lastRecord := req.Data.OperationRecords[len(req.Data.OperationRecords)-1]
		comment = lastRecord.Remark
		currentApproverUserID = lastRecord.UserID
	}

	// 更新当前审批人（转换为用户名）
	currentApproverName := ""
	if currentApproverUserID != "" {
		currentApproverName = h.convertUserIDToUsername(currentApproverUserID)
	}

	// 计算完成时间
	var finishTime *time.Time
	if req.Data.FinishTime != "" {
		if ft, err := time.Parse("2006-01-02 15:04:05", req.Data.FinishTime); err == nil {
			finishTime = &ft
		}
	}

	// 更新审批记录状态
	h.updateApprovalRecordStatus(&approval, status, comment, finishTime, currentApproverName)

	// 保存审批记录更新
	if err := h.db.Save(&approval).Error; err != nil {
		return err
	}

	// 构建审批步骤状态映射
	approverStatuses := make(map[string]string)
	for _, task := range req.Data.Tasks {
		if task.UserID != "" {
			approverName := h.convertUserIDToUsername(task.UserID)
			if approverName != "" {
				var stepStatus string
				if task.TaskStatus == "COMPLETED" {
					// 钉钉的TaskResult: "agree"表示同意，"refuse"表示拒绝
					switch task.TaskResult {
					case "agree":
						stepStatus = "approved"
					case "refuse":
						stepStatus = "rejected"
					default:
						fmt.Printf("警告: 未预期的钉钉任务结果 %s，默认为 pending\n", task.TaskResult)
						stepStatus = "pending"
					}
				} else {
					stepStatus = "pending"
				}
				approverStatuses[approverName] = stepStatus
			}
		}
	}
	// 从操作记录中获取审批意见（按时间顺序，后面的覆盖前面的）
	if len(req.Data.OperationRecords) > 0 {
		for _, record := range req.Data.OperationRecords {
			if record.UserID != "" && record.Remark != "" {
				approverName := h.convertUserIDToUsername(record.UserID)
				if approverName != "" {
					approverStatuses[approverName+"_comment"] = record.Remark
				}
			}
		}
	}

	// 更新关联的工单状态
	h.updateTicketFromApproval(req.ProcessInstanceID, status, comment, currentApproverName, approverStatuses)

	return nil
}

// handleWeChatApprovalEvent 处理企业微信审批事件
func (h *ApprovalCallbackHandler) handleWeChatApprovalEvent(req *WeChatCallbackRequest) error {
	// 查找对应的审批记录
	var approval model.Approval
	err := h.db.Where("external_id = ?", req.Content.ProcessInstanceID).First(&approval).Error
	if err != nil {
		return fmt.Errorf("未找到对应的审批记录: %v", err)
	}

	// 从 ApproverInfo 中提取审批人信息
	var allApproverIDs []string
	var currentApproverUserID string
	if len(req.Content.ApproverInfo) > 0 {
		for _, approver := range req.Content.ApproverInfo {
			if approver.UserID != "" {
				allApproverIDs = append(allApproverIDs, approver.UserID)
			}
		}
		// 更新审批人列表
		if len(allApproverIDs) > 0 {
			approval.ApproverIDs = allApproverIDs
		}
		// 获取最后一个审批人作为当前审批人
		currentApproverUserID = req.Content.ApproverInfo[len(req.Content.ApproverInfo)-1].UserID
	}

	// 更新审批状态
	status := h.mapWeChatStatus(req.Content.Status)

	// 更新当前审批人（转换为用户名）
	currentApproverName := ""
	if currentApproverUserID != "" {
		currentApproverName = h.convertUserIDToUsername(currentApproverUserID)
	}

	// 计算完成时间
	var finishTime *time.Time
	if req.Content.FinishTime > 0 {
		ft := time.Unix(req.Content.FinishTime, 0)
		finishTime = &ft
	}

	// 更新审批记录状态
	h.updateApprovalRecordStatus(&approval, status, req.Content.Result, finishTime, currentApproverName)

	// 保存审批记录更新
	if err := h.db.Save(&approval).Error; err != nil {
		return err
	}

	// 构建审批步骤状态映射
	approverStatuses := make(map[string]string)
	for _, approver := range req.Content.ApproverInfo {
		if approver.UserID != "" {
			approverName := h.convertUserIDToUsername(approver.UserID)
			if approverName != "" {
				var stepStatus string
				switch approver.Status {
				case "1": // 已通过
					stepStatus = "approved"
				case "2": // 已驳回
					stepStatus = "rejected"
				default:
					fmt.Printf("警告: 未预期的企业微信审批人状态 %s，默认为 pending\n", approver.Status)
					stepStatus = "pending"
				}
				approverStatuses[approverName] = stepStatus
				// 如果当前审批人有审批意见，添加
				if req.Content.Result != "" && approverName == currentApproverName {
					approverStatuses[approverName+"_comment"] = req.Content.Result
				}
			}
		}
	}
	// 如果当前审批人不在ApproverInfo中，也添加状态
	if currentApproverName != "" {
		if _, exists := approverStatuses[currentApproverName]; !exists {
			switch status {
			case model.ApprovalStatusApproved:
				approverStatuses[currentApproverName] = "approved"
			case model.ApprovalStatusRejected:
				approverStatuses[currentApproverName] = "rejected"
			case model.ApprovalStatusCanceled:
				approverStatuses[currentApproverName] = "canceled"
			default:
				fmt.Printf("警告: 未预期的审批状态 %s，不设置审批人状态\n", status)
			}
			if req.Content.Result != "" {
				approverStatuses[currentApproverName+"_comment"] = req.Content.Result
			}
		}
	}

	// 更新关联的工单状态
	h.updateTicketFromApproval(req.Content.ProcessInstanceID, status, req.Content.Result, currentApproverName, approverStatuses)

	return nil
}

// mapFeishuStatus 映射飞书状态到系统状态
func (h *ApprovalCallbackHandler) mapFeishuStatus(feishuStatus string) model.ApprovalStatus {
	switch feishuStatus {
	case "PENDING":
		return model.ApprovalStatusPending
	case "APPROVED":
		return model.ApprovalStatusApproved
	case "REJECTED":
		return model.ApprovalStatusRejected
	case "CANCELLED":
		return model.ApprovalStatusCanceled
	default:
		fmt.Printf("警告: 未预期的飞书状态 %s，默认为 pending\n", feishuStatus)
		return model.ApprovalStatusPending
	}
}

// mapDingTalkStatus 映射钉钉状态到系统状态
func (h *ApprovalCallbackHandler) mapDingTalkStatus(dingTalkStatus string) model.ApprovalStatus {
	switch dingTalkStatus {
	case "RUNNING":
		return model.ApprovalStatusPending
	case "COMPLETED":
		return model.ApprovalStatusApproved
	case "TERMINATED":
		return model.ApprovalStatusRejected
	case "CANCELED":
		return model.ApprovalStatusCanceled
	default:
		fmt.Printf("警告: 未预期的钉钉状态 %s，默认为 pending\n", dingTalkStatus)
		return model.ApprovalStatusPending
	}
}

// mapWeChatStatus 映射企业微信状态到系统状态
func (h *ApprovalCallbackHandler) mapWeChatStatus(weChatStatus string) model.ApprovalStatus {
	switch weChatStatus {
	case "1": // 审批中
		return model.ApprovalStatusPending
	case "2": // 已通过
		return model.ApprovalStatusApproved
	case "3": // 已驳回
		return model.ApprovalStatusRejected
	case "4": // 已撤销
		return model.ApprovalStatusCanceled
	default:
		fmt.Printf("警告: 未预期的企业微信状态 %s，默认为 pending\n", weChatStatus)
		return model.ApprovalStatusPending
	}
}
