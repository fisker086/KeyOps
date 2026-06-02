package router

import (
	bmhandler "github.com/fisker086/keyops/internal/api/handler/buildmaster"
	"github.com/gin-gonic/gin"
)

func buildMasterCallback(h *bmhandler.BuildMasterHandler, platform string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Params = append(c.Params, gin.Param{Key: "platform", Value: platform})
		h.ApprovalCallback(c)
	}
}

func registerCallbacks(api *gin.RouterGroup, d Deps) {
	h := d.Handlers

	// 按业务 scope 隔离的回调（推荐）：/api/approvals/callback/:source/:platform/:scope_id
	// scope_id 通常为 form_templates.uuid，每个工单模板可在飞书/钉钉等单独配置 webhook
	api.POST("/approvals/callback/:source/:platform/:scope_id", h.ApprovalCallback.HandleScopedCallback)

	// 通用回调（兼容旧配置）：/api/approvals/callback/:source/:platform
	api.POST("/approvals/callback/:source/feishu", h.ApprovalCallback.HandleFeishuCallback)
	api.POST("/approvals/callback/:source/lark", h.ApprovalCallback.HandleFeishuCallback)
	api.POST("/approvals/callback/:source/dingtalk", h.ApprovalCallback.HandleDingTalkCallback)
	api.POST("/approvals/callback/:source/wechat", h.ApprovalCallback.HandleWeChatCallback)

	// BuildMaster 第三方审批回调（无认证，webhook）
	api.POST("/build-master/callback/feishu", buildMasterCallback(h.BuildMaster, "feishu"))
	api.POST("/build-master/callback/lark", buildMasterCallback(h.BuildMaster, "lark"))
	api.POST("/build-master/callback/dingtalk", buildMasterCallback(h.BuildMaster, "dingtalk"))
	api.POST("/build-master/callback/wechat", buildMasterCallback(h.BuildMaster, "wechat"))
}
