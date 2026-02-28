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

// WeChatProvider 企业微信审批提供者
type WeChatProvider struct {
	config  *model.ApprovalConfig
	baseURL string
	client  *http.Client
	db      *gorm.DB
}

// NewWeChatProvider 创建企业微信审批提供者
func NewWeChatProvider(config *model.ApprovalConfig, db *gorm.DB) *WeChatProvider {
	// 使用用户配置的API基础URL，如果没有配置则使用默认值
	baseURL := config.APIBaseURL
	if baseURL == "" {
		baseURL = "https://qyapi.weixin.qq.com" // 默认值
	}

	return &WeChatProvider{
		config:  config,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		db: db,
	}
}

// CreateApproval 创建企业微信审批
func (p *WeChatProvider) CreateApproval(ctx context.Context, approval *model.Approval) (string, error) {
	// 检查审批代码是否存在
	if p.config.TemplateID == "" {
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
func (p *WeChatProvider) CreateApprovalWithFormData(ctx context.Context, approvalCode string, formData string, approval *model.Approval) (string, error) {
	// 确定使用的审批代码（企业微信使用 template_id）
	code := approvalCode
	if code == "" {
		code = p.config.TemplateID
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

// getAccessToken 获取企业微信访问令牌
func (p *WeChatProvider) getAccessToken(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/cgi-bin/gettoken?corpid=%s&corpsecret=%s", p.baseURL, p.config.AppID, p.config.AppSecret)

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
		return "", fmt.Errorf("企业微信API错误 [%d]: %s", result.ErrCode, result.ErrMsg)
	}

	return result.Token, nil
}

// buildFormContent 构建表单内容
func (p *WeChatProvider) buildFormContent(approval *model.Approval) string {
	formData := []map[string]interface{}{
		{
			"title": "工单标题",
			"value": approval.Title,
		},
		{
			"title": "详细描述",
			"value": approval.Description,
		},
		{
			"title": "申请理由",
			"value": approval.Reason,
		},
		{
			"title": "申请资源",
			"value": fmt.Sprintf("%v", approval.ResourceNames),
		},
		{
			"title": "权限时长",
			"value": fmt.Sprintf("%d小时", approval.Duration),
		},
	}

	jsonData, _ := json.Marshal(formData)
	return string(jsonData)
}

// createApprovalInstance 创建审批实例
func (p *WeChatProvider) createApprovalInstance(ctx context.Context, token, formContent string, approval *model.Approval) (string, error) {
	return p.createApprovalInstanceWithCode(ctx, token, p.config.TemplateID, formContent, approval)
}

// createApprovalInstanceWithCode 使用指定的审批代码创建审批实例
func (p *WeChatProvider) createApprovalInstanceWithCode(ctx context.Context, token, templateID, formContent string, approval *model.Approval) (string, error) {
	// 使用数据库配置中的API路径，如果没有配置则使用默认路径
	apiPath := p.config.APIPath
	if apiPath == "" {
		apiPath = "/cgi-bin/oa/applyevent" // 默认路径
	}
	url := fmt.Sprintf("%s%s?access_token=%s", p.baseURL, apiPath, token)

	// 企业微信API需要的是用户名（userid），而不是系统内部的UUID
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

	// 构建审批人列表
	approverList := []map[string]interface{}{}
	if len(approverIDs) > 0 {
		for _, approverID := range approverIDs {
			approverList = append(approverList, map[string]interface{}{
				"userid": approverID,
			})
		}
	} else {
		// 如果没有配置审批人，使用申请人作为审批人
		approverList = append(approverList, map[string]interface{}{
			"userid": userID,
		})
	}

	// 解析表单数据（formContent 是 JSON 字符串，需要解析为数组）
	// 飞书格式: [{"id": "widgetId", "type": "input", "value": "xxx"}]
	// 企业微信格式: [{"title": "字段标题", "value": "xxx"}] 或 [{"control": "字段ID", "value": "xxx"}]
	var formData []map[string]interface{}
	if formContent != "" {
		var feishuFormData []map[string]interface{}
		if err := json.Unmarshal([]byte(formContent), &feishuFormData); err != nil {
			return "", fmt.Errorf("解析表单数据失败: %v", err)
		}
		// 转换为企业微信格式：使用 id 作为 control（字段ID）
		for _, item := range feishuFormData {
			wechatItem := map[string]interface{}{
				"value": item["value"],
			}
			// 企业微信使用 control 字段作为字段ID
			if id, ok := item["id"].(string); ok && id != "" {
				wechatItem["control"] = id
			}
			formData = append(formData, wechatItem)
		}
	}

	reqBody := map[string]interface{}{
		"creator_userid":        userID,
		"template_id":           templateID,
		"use_template_approver": 0, // 不使用模板审批人，使用自定义审批人
		"approver":              approverList,
		"form_data":             formData,
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
		SpNo    string `json:"sp_no"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("企业微信API错误 [%d]: %s", result.ErrCode, result.ErrMsg)
	}

	return result.SpNo, nil
}

// GetApprovalFormDetail 获取审批表单详情
func (p *WeChatProvider) GetApprovalFormDetail(ctx context.Context, templateID string) ([]map[string]interface{}, error) {
	// 企业微信获取表单详情需要调用不同的 API
	// 这里暂时返回错误，因为企业微信的 API 结构不同
	return nil, fmt.Errorf("企业微信平台暂不支持自动获取表单详情，请手动配置表单字段映射")
}

// GetName 获取平台名称
func (p *WeChatProvider) GetName() string {
	return "企业微信"
}

// GetApprovalStatus 获取审批状态
func (p *WeChatProvider) GetApprovalStatus(ctx context.Context, externalID string) (*ApprovalStatusResponse, error) {
	// 获取访问令牌
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取访问令牌失败: %v", err)
	}

	// 查询审批实例详情
	// 使用数据库配置中的API路径，如果没有配置则使用默认路径
	apiPathGet := p.config.APIPathGet
	if apiPathGet == "" {
		apiPathGet = "/cgi-bin/oa/getapprovaldetail" // 默认路径
	}
	url := fmt.Sprintf("%s%s?access_token=%s", p.baseURL, apiPathGet, token)

	reqBody := map[string]interface{}{
		"sp_no": externalID,
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
		Info    struct {
			Status   string `json:"status"`
			SpStatus string `json:"sp_status"`
		} `json:"info"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		return nil, fmt.Errorf("企业微信API错误 [%d]: %s", result.ErrCode, result.ErrMsg)
	}

	// 映射状态
	status := p.mapStatus(result.Info.SpStatus)

	return &ApprovalStatusResponse{
		Status: status,
	}, nil
}

// CancelApproval 取消审批
func (p *WeChatProvider) CancelApproval(ctx context.Context, externalID string) error {
	// 获取访问令牌
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("获取访问令牌失败: %v", err)
	}

	// 取消审批实例
	// 使用数据库配置中的API路径，如果没有配置则使用默认路径
	apiPathCancel := p.config.APIPathCancel
	if apiPathCancel == "" {
		apiPathCancel = "/cgi-bin/oa/applyevent/cancel" // 默认路径
	}
	url := fmt.Sprintf("%s%s?access_token=%s", p.baseURL, apiPathCancel, token)

	reqBody := map[string]interface{}{
		"sp_no":  externalID,
		"userid": p.config.AppID, // 使用应用ID作为操作人
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
		return fmt.Errorf("企业微信API错误 [%d]: %s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

// HandleCallback 处理回调
func (p *WeChatProvider) HandleCallback(ctx context.Context, data interface{}) (*CallbackResult, error) {
	// 企业微信回调处理逻辑
	return nil, fmt.Errorf("企业微信回调处理未实现")
}

// ValidateConfig 验证配置
func (p *WeChatProvider) ValidateConfig(config map[string]interface{}) error {
	// 验证企业微信配置
	return nil
}

// mapStatus 映射企业微信状态到系统状态
func (p *WeChatProvider) mapStatus(weChatStatus string) model.ApprovalStatus {
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
		return model.ApprovalStatusPending
	}
}
