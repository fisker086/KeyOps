package approval

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
	"gorm.io/gorm"
)

// DingTalkProvider 钉钉审批提供者
type DingTalkProvider struct {
	config  *model.ApprovalConfig
	baseURL string
	client  *http.Client
	db      *gorm.DB
}

// NewDingTalkProvider 创建钉钉审批提供者
func NewDingTalkProvider(config *model.ApprovalConfig, db *gorm.DB) *DingTalkProvider {
	// 使用用户配置的API基础URL，如果没有配置则使用默认值
	baseURL := config.APIBaseURL
	if baseURL == "" {
		baseURL = "https://oapi.dingtalk.com" // 默认值
	}

	return &DingTalkProvider{
		config:  config,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		db: db,
	}
}

// CreateApproval 创建钉钉审批
func (p *DingTalkProvider) CreateApproval(ctx context.Context, approval *model.Approval) (string, error) {
	// 检查审批代码是否存在
	if p.config.ProcessCode == "" {
		return "", fmt.Errorf("审批代码未配置")
	}

	// 获取访问令牌
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("获取访问令牌失败: %v", err)
	}

	// 构建审批表单
	formContent := p.buildFormContent(approval)

	// 创建审批实例
	return p.createApprovalInstance(ctx, token, formContent, approval)
}

// CreateApprovalWithFormData 使用预构建的表单数据创建审批单
func (p *DingTalkProvider) CreateApprovalWithFormData(ctx context.Context, approvalCode string, formData string, approval *model.Approval) (string, error) {
	// 确定使用的审批代码（钉钉使用 process_code）
	code := approvalCode
	if code == "" {
		code = p.config.ProcessCode
	}
	if code == "" {
		return "", fmt.Errorf("审批代码未配置")
	}

	// 获取访问令牌
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("获取访问令牌失败: %v", err)
	}

	// 使用预构建的表单数据创建审批
	return p.createApprovalInstanceWithCode(ctx, token, code, formData, approval)
}

// getAccessToken 获取钉钉访问令牌
func (p *DingTalkProvider) getAccessToken(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/gettoken?appkey=%s&appsecret=%s", p.baseURL, p.config.AppID, p.config.AppSecret)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Token   string `json:"access_token"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("钉钉API错误 [%d]: %s", result.ErrCode, result.ErrMsg)
	}

	return result.Token, nil
}

// buildFormContent 构建表单内容
func (p *DingTalkProvider) buildFormContent(approval *model.Approval) string {
	formData := []map[string]interface{}{
		{
			"name":  "title",
			"value": approval.Title,
		},
		{
			"name":  "description",
			"value": approval.Description,
		},
		{
			"name":  "reason",
			"value": approval.Reason,
		},
		{
			"name":  "resources",
			"value": fmt.Sprintf("%v", approval.ResourceNames),
		},
		{
			"name":  "duration",
			"value": fmt.Sprintf("%d", approval.Duration),
		},
	}

	jsonData, _ := json.Marshal(formData)
	return string(jsonData)
}

// createApprovalInstance 创建审批实例
func (p *DingTalkProvider) createApprovalInstance(ctx context.Context, token, formContent string, approval *model.Approval) (string, error) {
	return p.createApprovalInstanceWithCode(ctx, token, p.config.ProcessCode, formContent, approval)
}

// createApprovalInstanceWithCode 使用指定的审批代码创建审批实例
func (p *DingTalkProvider) createApprovalInstanceWithCode(ctx context.Context, token, processCode, formContent string, approval *model.Approval) (string, error) {
	// 使用数据库配置中的API路径，如果没有配置则使用默认路径
	apiPath := p.config.APIPath
	if apiPath == "" {
		apiPath = "/topapi/processinstance/create" // 默认路径
	}
	url := fmt.Sprintf("%s%s?access_token=%s", p.baseURL, apiPath, token)

	// 钉钉API需要的是用户名（userid），而不是系统内部的UUID
	// ApplicantName 应该已经是用户名了（在创建审批记录时已设置）
	userID := approval.ApplicantName
	if userID == "" {
		userID = approval.ApplicantID // 如果ApplicantName为空，fallback到ApplicantID
	}
	
	// 如果userID看起来像UUID（36个字符且包含连字符），尝试查询用户名
	if len(userID) == 36 && strings.Contains(userID, "-") {
		var user model.User
		if err := p.db.Where("id = ?", userID).First(&user).Error; err == nil {
			userID = user.Username
		}
	}
	
	deptID := "1" // 钉钉默认部门ID，可以根据需要调整

	// 从配置中读取审批人列表，并转换为用户名
	var approverIDs []string
	if p.config.ApproverUserIDs != "" {
		var approverUserIDs []string
		if err := json.Unmarshal([]byte(p.config.ApproverUserIDs), &approverUserIDs); err == nil {
			// 将审批人ID转换为用户名
			for _, approverID := range approverUserIDs {
				// 如果是UUID格式，查询用户名
				if len(approverID) == 36 && strings.Contains(approverID, "-") {
					var user model.User
					if err := p.db.Where("id = ?", approverID).First(&user).Error; err == nil {
						approverIDs = append(approverIDs, user.Username)
					} else {
						// 如果查询失败，使用原ID（可能是用户名）
						approverIDs = append(approverIDs, approverID)
					}
				} else {
					// 不是UUID，直接使用（可能是用户名）
					approverIDs = append(approverIDs, approverID)
				}
			}
		}
	}

	// 解析表单数据（formContent 是 JSON 字符串，需要解析为数组）
	// 飞书格式: [{"id": "widgetId", "type": "input", "value": "xxx"}]
	// 钉钉格式: [{"name": "fieldName", "value": "xxx"}]
	var formComponentValues []map[string]interface{}
	if formContent != "" {
		var feishuFormData []map[string]interface{}
		if err := json.Unmarshal([]byte(formContent), &feishuFormData); err != nil {
			return "", fmt.Errorf("解析表单数据失败: %v", err)
		}
		// 转换为钉钉格式：使用 id 作为 name（如果字段映射中有 fieldName，优先使用 fieldName）
		for _, item := range feishuFormData {
			dingtalkItem := map[string]interface{}{
				"value": item["value"],
			}
			// 优先使用 id 作为 name（钉钉的字段名称）
			if id, ok := item["id"].(string); ok && id != "" {
				dingtalkItem["name"] = id
			} else {
				// 如果没有 id，尝试使用其他字段
				dingtalkItem["name"] = ""
			}
			formComponentValues = append(formComponentValues, dingtalkItem)
		}
	}

	reqBody := map[string]interface{}{
		"process_code":          processCode,
		"originator_user_id":    userID,
		"form_component_values": formComponentValues,
		"dept_id":               deptID,
	}

	// 如果配置了审批人，添加到请求体中
	if len(approverIDs) > 0 {
		reqBody["approvers"] = approverIDs
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
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  struct {
			ProcessInstanceID string `json:"process_instance_id"`
		} `json:"result"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("钉钉API错误 [%d]: %s", result.ErrCode, result.ErrMsg)
	}

	return result.Result.ProcessInstanceID, nil
}

// GetApprovalFormDetail 获取审批表单详情
func (p *DingTalkProvider) GetApprovalFormDetail(ctx context.Context, processCode string) ([]map[string]interface{}, error) {
	// 钉钉获取表单详情需要调用不同的 API
	// 这里暂时返回错误，因为钉钉的 API 结构不同
	// 如果需要实现，需要调用 /topapi/process/form/get 接口
	return nil, fmt.Errorf("钉钉平台暂不支持自动获取表单详情，请手动配置表单字段映射")
}

// GetName 获取平台名称
func (p *DingTalkProvider) GetName() string {
	return "钉钉"
}

// GetApprovalStatus 获取审批状态
func (p *DingTalkProvider) GetApprovalStatus(ctx context.Context, externalID string) (*ApprovalStatusResponse, error) {
	// 获取访问令牌
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取访问令牌失败: %v", err)
	}

	// 查询审批实例详情
	// 使用数据库配置中的API路径，如果没有配置则使用默认路径
	apiPathGet := p.config.APIPathGet
	if apiPathGet == "" {
		apiPathGet = "/topapi/processinstance/get" // 默认路径
	}
	url := fmt.Sprintf("%s%s?access_token=%s", p.baseURL, apiPathGet, token)

	reqBody := map[string]interface{}{
		"process_instance_id": externalID,
	}

	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

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
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  struct {
			Status string `json:"status"`
			Result string `json:"result"`
		} `json:"result"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		return nil, fmt.Errorf("钉钉API错误 [%d]: %s", result.ErrCode, result.ErrMsg)
	}

	// 映射状态
	status := p.mapStatus(result.Result.Status)

	return &ApprovalStatusResponse{
		Status:  status,
		Comment: result.Result.Result,
	}, nil
}

// CancelApproval 取消审批
func (p *DingTalkProvider) CancelApproval(ctx context.Context, externalID string) error {
	// 获取访问令牌
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("获取访问令牌失败: %v", err)
	}

	// 取消审批实例
	// 使用数据库配置中的API路径，如果没有配置则使用默认路径
	apiPathCancel := p.config.APIPathCancel
	if apiPathCancel == "" {
		apiPathCancel = "/topapi/processinstance/cancel" // 默认路径
	}
	url := fmt.Sprintf("%s%s?access_token=%s", p.baseURL, apiPathCancel, token)

	reqBody := map[string]interface{}{
		"process_instance_id": externalID,
		"operator_userid":     p.config.AppID, // 使用应用ID作为操作人
	}

	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

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
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("钉钉API错误 [%d]: %s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

// HandleCallback 处理回调
func (p *DingTalkProvider) HandleCallback(ctx context.Context, data interface{}) (*CallbackResult, error) {
	// 钉钉回调处理逻辑
	return nil, fmt.Errorf("钉钉回调处理未实现")
}

// ValidateConfig 验证配置
func (p *DingTalkProvider) ValidateConfig(config map[string]interface{}) error {
	// 验证钉钉配置
	return nil
}

// mapStatus 映射钉钉状态到系统状态
func (p *DingTalkProvider) mapStatus(dingTalkStatus string) model.ApprovalStatus {
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
		return model.ApprovalStatusPending
	}
}
