package aiassistant

// InspectionReportSender 巡检报告发送器：将巡检结果发送到告警渠道（飞书/钉钉/企业微信等）
type InspectionReportSender interface {
	// SendInspectionReport 发送巡检报告到指定渠道
	// channelIDs: 告警渠道 ID 列表；scheduleName: 定时任务名称；sessionID: 会话 ID；status: completed/error；summary: 报告摘要（如结论+步骤概览）
	SendInspectionReport(channelIDs []uint, scheduleName, sessionID, status, summary string) error
}
