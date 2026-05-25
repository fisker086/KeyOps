package router

import (
	"github.com/gin-gonic/gin"
)

func registerCallbacks(api *gin.RouterGroup, d Deps) {
	h := d.Handlers

	api.POST("/approvals/callback/feishu", h.ApprovalCallback.HandleFeishuCallback)
	api.POST("/approvals/callback/dingtalk", h.ApprovalCallback.HandleDingTalkCallback)
	api.POST("/approvals/callback/wechat", h.ApprovalCallback.HandleWeChatCallback)
}
