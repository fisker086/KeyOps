package approval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"gorm.io/gorm"
)

// FeishuProvider 飞书审批提供者
type FeishuProvider struct {
	config   *model.ApprovalConfig
	baseURL  string
	client   *http.Client
	db       *gorm.DB
	hostRepo *repository.HostRepository
}

// NewFeishuProvider 创建飞书审批提供者
func NewFeishuProvider(config *model.ApprovalConfig, db *gorm.DB) *FeishuProvider {
	// 使用用户配置的API基础URL，如果没有配置则使用默认值
	baseURL := config.APIBaseURL
	if baseURL == "" {
		baseURL = "https://open.larksuite.com/open-apis" // 默认值
	}

	return &FeishuProvider{
		config:  config,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		db:       db,
		hostRepo: repository.NewHostRepository(db),
	}
}

// GetName 获取平台名称
func (p *FeishuProvider) GetName() string {
	return "feishu"
}

// getTenantAccessToken 获取租户访问令牌
func (p *FeishuProvider) getTenantAccessToken(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/auth/v3/tenant_access_token/internal", p.baseURL)

	reqBody := map[string]string{
		"app_id":     p.config.AppID,
		"app_secret": p.config.AppSecret,
	}

	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
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
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("获取飞书访问令牌失败 [%d]: %s", result.Code, result.Msg)
	}

	return result.TenantAccessToken, nil
}

// CreateApproval 创建审批单
func (p *FeishuProvider) CreateApproval(ctx context.Context, approval *model.Approval) (string, error) {
	// 检查审批代码是否存在
	if p.config.ApprovalCode == "" {
		return "", fmt.Errorf("审批代码未配置")
	}

	// 获取访问令牌
	token, err := p.getTenantAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("获取访问令牌失败: %v", err)
	}

	// 构建审批表单
	formContent, err := p.buildFormContent(approval)
	if err != nil {
		return "", fmt.Errorf("构建表单内容失败: %v", err)
	}

	// 直接使用HTTP请求，因为SDK可能有兼容性问题
	return p.createApprovalViaHTTP(ctx, token, formContent, approval)
}

// CreateApprovalWithFormData 使用预构建的表单数据创建审批单
func (p *FeishuProvider) CreateApprovalWithFormData(ctx context.Context, approvalCode string, formData string, approval *model.Approval) (string, error) {
	// 确定使用的审批代码
	code := approvalCode
	if code == "" {
		code = p.config.ApprovalCode
	}
	if code == "" {
		return "", fmt.Errorf("审批代码未配置")
	}

	// 获取访问令牌
	token, err := p.getTenantAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("获取访问令牌失败: %v", err)
	}

	// 使用预构建的表单数据创建审批
	return p.createApprovalViaHTTPWithCode(ctx, token, code, formData, approval)
}

// buildFormContent 构建表单内容
func (p *FeishuProvider) buildFormContent(approval *model.Approval) (string, error) {
	// 如果配置中有自定义表单字段，使用配置的字段
	if p.config.FormFields != "" {
		// 尝试从 ExternalData 中获取字段映射配置（工单模板的审批配置）
		var fieldMappings []map[string]interface{}
		if approval.ExternalData != "" {
			var externalData map[string]interface{}
			if err := json.Unmarshal([]byte(approval.ExternalData), &externalData); err == nil {
				// 支持新格式：field_mappings 是数组
				if mappingsArray, ok := externalData["field_mappings"].([]interface{}); ok {
					fieldMappings = make([]map[string]interface{}, 0, len(mappingsArray))
					for _, m := range mappingsArray {
						if mapping, ok := m.(map[string]interface{}); ok {
							fieldMappings = append(fieldMappings, mapping)
						}
					}
				} else if mappings, ok := externalData["field_mappings"].(map[string]interface{}); ok {
					// 兼容旧格式：field_mappings 是对象（字段名称到关键字的映射）
					fieldMappings = make([]map[string]interface{}, 0, len(mappings))
					for k, v := range mappings {
						if str, ok := v.(string); ok {
							fieldMappings = append(fieldMappings, map[string]interface{}{
								"fieldName": k, // 使用 fieldName 作为键，兼容旧格式
								"keyword":   str,
							})
						}
					}
				}
			}
		}
		return p.buildFormContentFromConfig(approval, fieldMappings)
	}

	// 如果没有配置表单字段，返回错误
	return "", fmt.Errorf("未配置表单字段，请在工单审批配置中设置form_fields")
}

// GetApprovalStatus 获取审批单状态
func (p *FeishuProvider) GetApprovalStatus(ctx context.Context, externalID string) (*ApprovalStatusResponse, error) {
	token, err := p.getTenantAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	// 使用数据库配置中的获取审批API路径
	apiPathGet := p.config.APIPathGet
	if apiPathGet == "" {
		apiPathGet = "/approval/v4/instances" // 默认路径
	}
	url := fmt.Sprintf("%s%s/%s", p.baseURL, apiPathGet, externalID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Status       string `json:"status"`
			ApproverName string `json:"approver_name"`
			UpdateTime   int64  `json:"update_time"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("get approval status failed: %s", result.Msg)
	}

	status := p.convertStatus(result.Data.Status)

	return &ApprovalStatusResponse{
		Status:       status,
		ApproverName: result.Data.ApproverName,
		ApprovedAt:   time.Unix(result.Data.UpdateTime, 0).Format(time.RFC3339),
	}, nil
}

// convertStatus 转换状态
func (p *FeishuProvider) convertStatus(feishuStatus string) model.ApprovalStatus {
	switch feishuStatus {
	case "APPROVED":
		return model.ApprovalStatusApproved
	case "REJECTED":
		return model.ApprovalStatusRejected
	case "CANCELED":
		return model.ApprovalStatusCanceled
	case "DELETED":
		return model.ApprovalStatusCanceled
	default:
		return model.ApprovalStatusPending
	}
}

// CancelApproval 取消审批单
func (p *FeishuProvider) CancelApproval(ctx context.Context, externalID string) error {
	token, err := p.getTenantAccessToken(ctx)
	if err != nil {
		return err
	}

	// 使用数据库配置中的取消审批API路径
	apiPathCancel := p.config.APIPathCancel
	if apiPathCancel == "" {
		apiPathCancel = "/approval/v4/instances/cancel" // 默认路径
	}
	url := fmt.Sprintf("%s%s", p.baseURL, apiPathCancel)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return err
	}

	if result.Code != 0 {
		return fmt.Errorf("cancel approval failed: %s", result.Msg)
	}

	return nil
}

// HandleCallback 处理审批回调
func (p *FeishuProvider) HandleCallback(ctx context.Context, data interface{}) (*CallbackResult, error) {
	// 飞书回调数据解析
	callbackData, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid callback data")
	}

	instanceCode, _ := callbackData["instance_code"].(string)
	status, _ := callbackData["status"].(string)
	approverName, _ := callbackData["approver_name"].(string)
	comment, _ := callbackData["comment"].(string)

	return &CallbackResult{
		ApprovalID:   instanceCode,
		Status:       p.convertStatus(status),
		ApproverName: approverName,
		Comment:      comment,
	}, nil
}

// ValidateConfig 验证配置
func (p *FeishuProvider) ValidateConfig(config map[string]interface{}) error {
	appID, ok := config["app_id"].(string)
	if !ok || appID == "" {
		return fmt.Errorf("app_id is required")
	}

	appSecret, ok := config["app_secret"].(string)
	if !ok || appSecret == "" {
		return fmt.Errorf("app_secret is required")
	}

	approvalCode, ok := config["approval_code"].(string)
	if !ok || approvalCode == "" {
		return fmt.Errorf("approval_code is required")
	}

	return nil
}

// getHostIPByName 根据主机名获取IP地址
func (p *FeishuProvider) getHostIPByName(hostName string) string {
	if p.hostRepo == nil {
		return ""
	}

	// 通过主机名查询主机信息
	hosts, _, err := p.hostRepo.FindAll(1, 1, hostName, nil)
	if err != nil || len(hosts) == 0 {
		return ""
	}

	return hosts[0].IP
}

// createApprovalViaHTTP 使用HTTP请求创建审批
func (p *FeishuProvider) createApprovalViaHTTP(ctx context.Context, token, formContent string, approval *model.Approval) (string, error) {
	return p.createApprovalViaHTTPWithCode(ctx, token, p.config.ApprovalCode, formContent, approval)
}

// createApprovalViaHTTPWithCode 使用指定的审批代码创建审批
func (p *FeishuProvider) createApprovalViaHTTPWithCode(ctx context.Context, token, approvalCode, formContent string, approval *model.Approval) (string, error) {
	// 使用数据库配置中的API路径，如果没有配置则使用默认路径
	apiPath := p.config.APIPath
	if apiPath == "" {
		apiPath = "/approval/v4/instances" // 默认路径
	}
	url := fmt.Sprintf("%s%s", p.baseURL, apiPath)

	// 飞书API需要的是用户名（username），而不是系统内部的UUID
	// ApplicantName 应该已经是用户名了（在创建审批记录时已设置）
	userID := approval.ApplicantName
	if userID == "" {
		userID = approval.ApplicantID // 如果ApplicantName为空，fallback到ApplicantID
		fmt.Printf("飞书审批 - 警告：ApplicantName为空，使用ApplicantID: %s\n", userID)
	}
	
	// 如果userID看起来像UUID（36个字符且包含连字符），尝试查询用户名
	if len(userID) == 36 && strings.Contains(userID, "-") {
		fmt.Printf("飞书审批 - 警告：userID是UUID格式，尝试查询用户名: %s\n", userID)
		var user model.User
		if err := p.db.Where("id = ?", userID).First(&user).Error; err == nil {
			userID = user.Username
			fmt.Printf("飞书审批 - 查询到用户名: %s\n", userID)
		} else {
			fmt.Printf("飞书审批 - 错误：userID是UUID格式但查询用户名失败: %v，将使用UUID（可能失败）\n", err)
		}
	}
	
	fmt.Printf("飞书审批 - 最终使用的userID: %s (原始ApplicantID: %s, 原始ApplicantName: %s)\n", userID, approval.ApplicantID, approval.ApplicantName)
	openID := userID // 对于飞书，OpenID通常与UserID相同

	reqBody := map[string]interface{}{
		"approval_code": approvalCode,
		"user_id":       userID,
		"open_id":       openID,
		"form":          formContent,
	}

	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			InstanceCode string `json:"instance_code"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	if result.Code != 0 {
		return "", fmt.Errorf("HTTP请求失败 [%d]: %s", result.Code, result.Msg)
	}

	return result.Data.InstanceCode, nil
}

// GetApprovalFormDetail 获取审批表单详情
func (p *FeishuProvider) GetApprovalFormDetail(ctx context.Context, approvalCode string) ([]map[string]interface{}, error) {
	if approvalCode == "" {
		return nil, fmt.Errorf("审批代码不能为空")
	}

	// 获取访问令牌
	token, err := p.getTenantAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取访问令牌失败: %v", err)
	}

	// 调用飞书 API 获取审批详情
	// API: GET /approval/v4/approvals/{approval_code}
	url := fmt.Sprintf("%s/approval/v4/approvals/%s", p.baseURL, approvalCode)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Form string `json:"form"` // JSON 字符串
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("获取审批表单详情失败 [%d]: %s", result.Code, result.Msg)
	}

	// 解析 form JSON 字符串
	var formFields []map[string]interface{}
	if err := json.Unmarshal([]byte(result.Data.Form), &formFields); err != nil {
		return nil, fmt.Errorf("解析表单字段失败: %v", err)
	}

	return formFields, nil
}

// buildFormContentFromConfig 从配置中构建表单内容
// fieldMappings: Widget ID 到关键字的映射数组，每个映射包含 widgetId 和 keyword
// TODO: 当前仅实现飞书平台，后续需要为钉钉和企微实现类似的逻辑
func (p *FeishuProvider) buildFormContentFromConfig(approval *model.Approval, fieldMappings []map[string]interface{}) (string, error) {
	var formFields []map[string]interface{}
	if err := json.Unmarshal([]byte(p.config.FormFields), &formFields); err != nil {
		return "", fmt.Errorf("解析表单字段配置失败: %v", err)
	}

	// 尝试从 ExternalData 中获取工单数据
	var ticketFormData map[string]interface{}
	if approval.ExternalData != "" {
		var externalData map[string]interface{}
		if err := json.Unmarshal([]byte(approval.ExternalData), &externalData); err == nil {
			if formData, ok := externalData["ticket_form_data"].(map[string]interface{}); ok {
				ticketFormData = formData
			}
		}
	}

	// 构建 Widget ID 到字段名称（关键字）的映射表
	// 新格式：{ fieldName: "字段名", widgetId: "widgetId" }，字段名称就是关键字
	// 旧格式：{ fieldName: "字段名", keyword: "关键字" } 或 { widgetId: "widgetId", keyword: "关键字" }
	widgetIdToKeyword := make(map[string]string)
	fieldNameToKeyword := make(map[string]string) // 用于兼容旧格式：通过字段名称匹配
	
	if len(fieldMappings) > 0 {
		for _, mapping := range fieldMappings {
			var widgetId, fieldName, keyword string
			
			// 获取字段名称
			if fName, ok := mapping["fieldName"].(string); ok && fName != "" {
				fieldName = fName
			}
			
			// 获取 Widget ID
			if wId, ok := mapping["widgetId"].(string); ok && wId != "" {
				widgetId = wId
			}
			
			// 获取关键字（新格式：字段名称就是关键字；旧格式：有单独的 keyword 字段）
			if k, ok := mapping["keyword"].(string); ok && k != "" {
				keyword = k
			} else if fieldName != "" {
				// 新格式：如果没有 keyword，使用字段名称作为关键字
				keyword = fieldName
			}
			
			if widgetId != "" && keyword != "" {
				widgetIdToKeyword[widgetId] = keyword
			}
			
			// 兼容旧格式：通过字段名称匹配
			if fieldName != "" && keyword != "" {
				fieldNameToKeyword[fieldName] = keyword
			}
		}
	}

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

		// 获取字段名称（用于兼容旧格式）
		fieldName := ""
		if name, ok := field["name"].(string); ok {
			fieldName = name
		}

		// 如果提供了字段映射配置，使用配置的关键字来匹配数据
		if len(widgetIdToKeyword) > 0 || len(fieldNameToKeyword) > 0 {
			var keyword string
			var exists bool
			
			// 优先使用 Widget ID 匹配（新格式）
			if widgetId != "" {
				keyword, exists = widgetIdToKeyword[widgetId]
			}
			
			// 如果 Widget ID 匹配失败，尝试使用字段名称匹配（兼容旧格式）
			if !exists && fieldName != "" {
				keyword, exists = fieldNameToKeyword[fieldName]
				// 如果字段名称也没有匹配到，但字段名称存在，使用字段名称作为关键字（新格式）
				if !exists {
					keyword = fieldName
					exists = true
				}
			}
			
			if exists && keyword != "" {
				// 使用关键字从工单数据或审批对象中获取值
				formItem["value"] = p.getValueByKeyword(approval, ticketFormData, keyword, field)
			} else {
				// 如果没有配置映射，使用空值
				formItem["value"] = ""
			}
		} else {
			// 如果没有提供字段映射配置，使用默认的硬编码匹配（向后兼容）
			formItem["value"] = p.getDefaultValueByFieldName(approval, fieldName, field)
		}

		formData = append(formData, formItem)
	}

	jsonData, _ := json.Marshal(formData)
	return string(jsonData), nil
}

// getValueByKeyword 根据关键字从审批对象或工单数据中获取值
func (p *FeishuProvider) getValueByKeyword(approval *model.Approval, ticketFormData map[string]interface{}, keyword string, field map[string]interface{}) interface{} {
	// 特殊关键字处理
	switch keyword {
	case "工单编号", "ticket_number":
		if approval.ExternalData != "" {
			var externalData map[string]interface{}
			if err := json.Unmarshal([]byte(approval.ExternalData), &externalData); err == nil {
				if ticketNumber, ok := externalData["ticket_number"].(string); ok {
					return ticketNumber
				}
			}
		}
		return ""
	case "工单标题", "title":
		return approval.Title
	case "详细描述", "description":
		return approval.Description
	case "申请理由", "reason":
		return approval.Reason
	case "申请资源", "resources":
		if len(approval.ResourceNames) > 0 {
			var resources []string
			for _, name := range approval.ResourceNames {
				ip := p.getHostIPByName(name)
				if ip != "" {
					resources = append(resources, fmt.Sprintf("%s (IP: %s)", name, ip))
				} else {
					resources = append(resources, fmt.Sprintf("%s (IP: 未找到)", name))
				}
			}
			return strings.Join(resources, "\n")
		}
		return approval.ResourceType
	case "申请类型", "type":
		return p.getApprovalTypeValue(approval.Type, field)
	case "权限时长", "duration":
		return fmt.Sprintf("%d", approval.Duration)
	}

	// 从工单数据中查找（支持嵌套路径，如 "details.0.project_name"）
	if ticketFormData != nil {
		value := p.findValueByKeyword(ticketFormData, keyword)
		if value != nil {
			return value
		}
	}

	// 如果找不到，返回空值
	return ""
}

// findValueByKeyword 通过关键字在数据中查找值（支持嵌套路径）
func (p *FeishuProvider) findValueByKeyword(data map[string]interface{}, keyword string) interface{} {
	if data == nil || keyword == "" {
		return nil
	}

	// 直接匹配
	if value, ok := data[keyword]; ok {
		return value
	}

	// 支持嵌套路径，如 "details.0.project_name"
	parts := strings.Split(keyword, ".")
	current := interface{}(data)
	for _, part := range parts {
		if current == nil {
			return nil
		}
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		case []interface{}:
			if index, err := strconv.Atoi(part); err == nil && index >= 0 && index < len(v) {
				current = v[index]
			} else {
				return nil
			}
		default:
			return nil
		}
	}
	return current
}

// getDefaultValueByFieldName 根据字段名称获取默认值（向后兼容）
func (p *FeishuProvider) getDefaultValueByFieldName(approval *model.Approval, fieldName string, field map[string]interface{}) interface{} {
	switch fieldName {
	case "工单标题", "title":
		return approval.Title
	case "详细描述", "description":
		return approval.Description
	case "申请理由", "reason":
		return approval.Reason
	case "申请资源", "resources":
		if len(approval.ResourceNames) > 0 {
			var resources []string
			for _, name := range approval.ResourceNames {
				ip := p.getHostIPByName(name)
				if ip != "" {
					resources = append(resources, fmt.Sprintf("%s (IP: %s)", name, ip))
				} else {
					resources = append(resources, fmt.Sprintf("%s (IP: 未找到)", name))
				}
			}
			return strings.Join(resources, "\n")
		}
		return approval.ResourceType
	case "申请类型", "type":
		return p.getApprovalTypeValue(approval.Type, field)
	case "权限时长", "duration":
		return fmt.Sprintf("%d", approval.Duration)
	default:
		return ""
	}
}

// getApprovalTypeValue 根据审批类型和字段配置获取对应的选项值
func (p *FeishuProvider) getApprovalTypeValue(approvalType model.ApprovalType, field map[string]interface{}) string {
	// 如果字段配置中有选项列表，尝试匹配
	if options, ok := field["option"].([]interface{}); ok {
		for _, option := range options {
			if optMap, ok := option.(map[string]interface{}); ok {
				text := ""
				value := ""
				if t, exists := optMap["text"].(string); exists {
					text = t
				}
				if v, exists := optMap["value"].(string); exists {
					value = v
				}

				// 根据审批类型匹配文本
				switch approvalType {
				case model.ApprovalTypeHostAccess:
					if text == "host_access" || text == "主机访问权限" {
						return value
					}
				case model.ApprovalTypeHostGroupAccess:
					if text == "host_group_access" || text == "主机组访问权限" {
						return value
					}
				}
			}
		}
	}

	// 如果没有找到匹配的选项，返回默认值
	switch approvalType {
	case model.ApprovalTypeHostAccess:
		return "mh22s1w3-nkqsak2eis-0"
	case model.ApprovalTypeHostGroupAccess:
		return "mh22s1w3-dipw4j04vb-0"
	default:
		return "mh22s1w3-nkqsak2eis-0"
	}
}

// BuildReleaseFormData 按字段名称构建发布审批表单数据（与 feishu 审批定义字段名一致）
func (p *FeishuProvider) BuildReleaseFormData(ctx context.Context, approvalCode string, approval *model.Approval, baseURL string) (string, error) {
	formFields, err := p.GetApprovalFormDetail(ctx, approvalCode)
	if err != nil {
		return "", fmt.Errorf("获取审批表单详情: %w", err)
	}
	var formData []map[string]interface{}
	for _, field := range formFields {
		formItem := map[string]interface{}{
			"id":   field["id"],
			"type": field["type"],
		}
		fieldName, _ := field["name"].(string)
		formItem["value"] = p.getReleaseFormValueByFieldName(approval, fieldName, baseURL)
		formData = append(formData, formItem)
	}
	jsonData, _ := json.Marshal(formData)
	return string(jsonData), nil
}

func (p *FeishuProvider) getReleaseFormValueByFieldName(approval *model.Approval, fieldName string, baseURL string) interface{} {
	switch fieldName {
	case "工单编号":
		if len(approval.ID) >= 8 {
			return "REL-" + approval.ID[:8]
		}
		return "REL-" + approval.ID
	case "工单标题", "title":
		return approval.Title
	case "工单描述", "详细描述", "description":
		return approval.Description
	case "优先级", "priority":
		if approval.Priority != "" {
			return approval.Priority
		}
		return "normal"
	case "明细":
		var cfg struct {
			ReleaseRunID  string `json:"release_run_id"`
			ApplicationID string `json:"application_id"`
		}
		_ = json.Unmarshal([]byte(approval.DeployConfig), &cfg)
		if cfg.ReleaseRunID != "" || cfg.ApplicationID != "" {
			return fmt.Sprintf("应用ID: %s, Release Run: %s", cfg.ApplicationID, cfg.ReleaseRunID)
		}
		return approval.Description
	case "工单链接":
		if baseURL != "" {
			return fmt.Sprintf("%s/approvals", strings.TrimSuffix(baseURL, "/"))
		}
		return ""
	default:
		return ""
	}
}
