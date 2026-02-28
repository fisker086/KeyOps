package model

import (
	"time"

	"gorm.io/datatypes"
)

// AlertRuleGroup 告警规则组
type AlertRuleGroup struct {
	ID           uint    `gorm:"primaryKey" json:"id"`
	GroupName    string  `gorm:"type:varchar(100);uniqueIndex;not null" json:"group_name" binding:"required"`
	Description  string  `gorm:"type:text" json:"description"`
	DepartmentID *string `gorm:"type:varchar(36);index" json:"department_id"` // NULL表示全局规则组
	SourceID     uint    `gorm:"index" json:"source_id"`                      // 关联的数据源ID
	File         string  `gorm:"type:varchar(500)" json:"file"`               // 规则文件路径（相对于规则目录）
	Enabled      bool    `gorm:"default:true;index" json:"enabled"`
	BaseModel
}

func (AlertRuleGroup) TableName() string {
	return "alert_rule_groups"
}

// AlertRuleSource 告警规则数据源
type AlertRuleSource struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	SourceName   string `gorm:"type:varchar(100);uniqueIndex:idx_source_name;not null" json:"source_name" binding:"required"`
	SourceType   string `gorm:"type:varchar(100);not null" json:"source_type" binding:"required"` // prometheus, victoriametrics, etc.
	Address      string `gorm:"type:varchar(500);not null" json:"address" binding:"required"`
	APIKey       string `gorm:"type:varchar(200);index:idx_api_key" json:"api_key"` // API密钥，用于webhook认证
	AutoSync     bool   `gorm:"default:true" json:"auto_sync"`
	SyncInterval int    `gorm:"default:10" json:"sync_interval"` // 同步间隔（分钟），默认10分钟
	BaseModel
}

func (AlertRuleSource) TableName() string {
	return "alert_rule_sources"
}

// AlertRule 告警规则
type AlertRule struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"type:varchar(500);uniqueIndex:idx_name_group_source" json:"name" binding:"required"`
	GroupID     *uint          `gorm:"index;uniqueIndex:idx_name_group_source" json:"group_id"` // 规则组ID
	Group       string         `gorm:"type:varchar(100);index" json:"group"`                    // 规则组名称（兼容字段）
	Expr        string         `gorm:"type:text;not null" json:"expr" binding:"required"`       // PromQL表达式
	Duration    int            `gorm:"default:0" json:"duration"`                               // 持续时间（秒）
	Labels      datatypes.JSON `gorm:"type:json" json:"labels"`                                 // 标签
	Annotations datatypes.JSON `gorm:"type:json" json:"annotations"`                            // 注解
	Health      string         `gorm:"type:varchar(100);default:'unknown'" json:"health"`       // unknown, ok, error
	SourceID    uint           `gorm:"index;uniqueIndex:idx_name_group_source" json:"source_id"`
	Enabled     bool           `gorm:"default:true" json:"enabled"`
	BaseModel
}

func (AlertRule) TableName() string {
	return "alert_rules"
}

// AlertEvent 告警事件
type AlertEvent struct {
	ID               uint64         `gorm:"primaryKey" json:"id"`
	AlertTitle       string         `gorm:"type:varchar(200);index" json:"alert_title"`
	SourceID         uint           `gorm:"index;not null" json:"source_id"`
	Description      string         `gorm:"type:varchar(500)" json:"description"`
	Level            int            `gorm:"index;not null" json:"level"` // 告警等级
	FirstTriggerTime *time.Time     `gorm:"type:datetime" json:"first_trigger_time"`
	FirstAckTime     *time.Time     `gorm:"type:datetime" json:"first_ack_time"`
	TriggerTime      *time.Time     `gorm:"type:datetime;index" json:"trigger_time"`
	RecoverTime      *time.Time     `gorm:"type:datetime" json:"recover_time"`
	Annotations      datatypes.JSON `gorm:"type:json" json:"annotations"`
	IsRecovered      bool           `gorm:"default:false" json:"is_recovered"`
	Progress         int            `gorm:"default:1;index" json:"progress"`   // 1未认领 2已认领 3已关闭
	UID              string         `gorm:"type:varchar(36);index" json:"uid"` // 处理人
	Tags             datatypes.JSON `gorm:"type:json" json:"tags"`
	FingerPrint      string         `gorm:"type:varchar(100);index" json:"finger_print"` // 指纹去重
	SourceIP         string         `gorm:"type:varchar(50)" json:"source_ip"`
	DepartmentID     string         `gorm:"type:varchar(36);index" json:"department_id"`
	IntegrationID    uint           `gorm:"index" json:"integration_id"`
	BaseModel
}

func (AlertEvent) TableName() string {
	return "alert_events"
}

// AlertLog 告警日志
type AlertLog struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	AlertID uint64 `gorm:"index;not null" json:"alert_id"`
	Action  string `gorm:"type:varchar(20)" json:"action"` // claim, closed, cancel_claim, opened
	UID     string `gorm:"type:varchar(36);index" json:"uid"`
	BaseModel
}

func (AlertLog) TableName() string {
	return "alert_logs"
}

// AlertStrategy 告警策略
type AlertStrategy struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	StrategyName string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"strategy_name" binding:"required"`
	DepartmentID string         `gorm:"type:varchar(36);index" json:"department_id"`
	TemplateID   uint           `gorm:"index" json:"template_id"`
	Status       string         `gorm:"type:varchar(20);default:'enabled';index" json:"status"` // enabled, disabled, deleted
	Weight       int            `gorm:"default:0" json:"weight"`
	Continuous   bool           `gorm:"default:false" json:"continuous"`
	Delay        int            `gorm:"default:0" json:"delay"` // 延迟通知（秒）
	TimeSlot     datatypes.JSON `gorm:"type:json" json:"time_slot"`
	Filters      datatypes.JSON `gorm:"type:json" json:"filters"`
	StrategySet  datatypes.JSON `gorm:"type:json" json:"strategy_set"`
	UID          uint           `gorm:"index" json:"uid"`
	BaseModel
}

func (AlertStrategy) TableName() string {
	return "alert_strategies"
}

// AlertLevel 告警等级
type AlertLevel struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	LevelName string `gorm:"type:varchar(50);not null" json:"level_name"`
	Color     string `gorm:"type:varchar(50);not null" json:"color"`
	IsDefault bool   `gorm:"default:true" json:"is_default"`
	LevelDesc string `gorm:"type:text" json:"level_desc"`
	BaseModel
}

func (AlertLevel) TableName() string {
	return "alert_levels"
}

// AlertGroup 告警组（用于通知配置）
type AlertGroup struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	GroupName    string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"group_name" binding:"required"`
	Description  string         `gorm:"type:text" json:"description"`
	DepartmentID *string        `gorm:"type:varchar(36);index" json:"department_id"` // NULL表示全局告警组
	Members      datatypes.JSON `gorm:"type:json" json:"members"`                    // 成员ID列表（UUID字符串数组）["uuid1", "uuid2"]
	UID          uint           `gorm:"index" json:"uid"`                            // 创建人
	BaseModel
}

func (AlertGroup) TableName() string {
	return "alert_groups"
}

// AlertAggregation 告警聚合
type AlertAggregation struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	AggregationName string         `gorm:"type:varchar(100)" json:"aggregation_name"`
	AggregationDesc string         `gorm:"type:text" json:"aggregation_desc"`
	LevelDimension  bool           `gorm:"default:false" json:"level_dimension"`
	TagsDimension   datatypes.JSON `gorm:"type:json" json:"tags_dimension"`
	TitleDimension  bool           `gorm:"default:false" json:"title_dimension"`
	Windows         int            `gorm:"default:0" json:"windows"` // 聚合窗口（秒）
	Storm           int            `gorm:"default:0" json:"storm"`   // 风暴预警阈值
	UID             uint           `gorm:"index" json:"uid"`
	Status          string         `gorm:"type:varchar(20);default:'enabled';index" json:"status"` // enabled, disabled, deleted
	BaseModel
}

func (AlertAggregation) TableName() string {
	return "alert_aggregations"
}

// AlertSilence 告警静默
type AlertSilence struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	DepartmentID string         `gorm:"type:varchar(36);index" json:"department_id"`
	SilenceName  string         `gorm:"type:varchar(100)" json:"silence_name"`
	SilenceDesc  string         `gorm:"type:text" json:"silence_desc"`
	SilenceType  string         `gorm:"type:varchar(20)" json:"silence_type"` // once, period
	SilenceTime  datatypes.JSON `gorm:"type:json" json:"silence_time"`
	Filters      datatypes.JSON `gorm:"type:json" json:"filters"`
	UID          uint           `gorm:"index" json:"uid"`
	BaseModel
}

func (AlertSilence) TableName() string {
	return "alert_silences"
}

// AlertRestrain 告警抑制
type AlertRestrain struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	RestrainType   string         `gorm:"type:varchar(20)" json:"restrain_type"`
	Fields         datatypes.JSON `gorm:"type:json" json:"fields"`
	CumulativeTime int            `gorm:"type:int" json:"cumulative_time"` // 抑制时长（秒）
	UID            uint           `gorm:"index" json:"uid"`
	BaseModel
}

func (AlertRestrain) TableName() string {
	return "alert_restrains"
}

// AlertTemplate 告警模板
type AlertTemplate struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	TemplateName string         `gorm:"type:varchar(100)" json:"template_name"`
	TemplateDesc string         `gorm:"type:text" json:"template_desc"`
	Channels     datatypes.JSON `gorm:"type:json" json:"channels"`
	Members      datatypes.JSON `gorm:"type:json" json:"members"`      // 通知成员ID列表（UUID字符串数组）["uuid1", "uuid2"]
	AlertGroups  datatypes.JSON `gorm:"type:json" json:"alert_groups"` // 告警组ID列表（数字数组）[1, 2, 3]
	Enable       bool           `gorm:"default:true" json:"enable"`
	BaseModel
}

func (AlertTemplate) TableName() string {
	return "alert_templates"
}

// AlertChannel 告警渠道
type AlertChannel struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	ChannelName  string `gorm:"type:varchar(100);not null" json:"channel_name"`
	ChannelType  string `gorm:"type:varchar(50);not null" json:"channel_type"`
	ChannelSign  string `gorm:"type:varchar(500);not null" json:"channel_sign"`
	ChannelGroup string `gorm:"type:varchar(100)" json:"channel_group"`
	BaseModel
}

func (AlertChannel) TableName() string {
	return "alert_channels"
}

// StrategyLog 策略日志
type StrategyLog struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	AlertID         uint64         `gorm:"index" json:"alert_id"`
	UID             uint           `gorm:"index" json:"uid"`
	StrategyContent datatypes.JSON `gorm:"type:json" json:"strategy_content"`
	StrategyID      uint           `gorm:"index" json:"strategy_id"`
	Channels        datatypes.JSON `gorm:"type:json" json:"channels"`
	IsNotify        bool           `gorm:"default:false" json:"is_notify"`
	ErrMessage      string         `gorm:"type:text" json:"err_message"`
	NotifyType      int            `gorm:"default:1" json:"notify_type"` // 1告警 2恢复
	BaseModel
}

func (StrategyLog) TableName() string {
	return "strategy_logs"
}

// PrometheusAlert Webhook接收的Prometheus告警格式
type PrometheusAlert struct {
	Version           string                `json:"version"`
	Receiver          string                `json:"receiver"`
	Status            string                `json:"status"`
	Alerts            []PrometheusAlertItem `json:"alerts"`
	GroupLabels       map[string]string     `json:"groupLabels"`
	CommonLabels      map[string]string     `json:"commonLabels"`
	CommonAnnotations map[string]string     `json:"commonAnnotations"`
	ExternalURL       string                `json:"externalURL"`
}

type PrometheusAlertItem struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// OnCallSchedule 值班排班表
type OnCallSchedule struct {
	ID                  uint   `gorm:"primaryKey" json:"id"`
	ScheduleName        string `gorm:"type:varchar(100);not null" json:"schedule_name"`
	Description         string `gorm:"type:text" json:"description"`
	DepartmentID        string `gorm:"type:varchar(36);index" json:"department_id"` // 部门ID（关联组织架构）
	Enabled             bool   `gorm:"default:true;index" json:"enabled"`
	UID                 uint   `gorm:"index" json:"uid"`                              // 创建人
	NotificationWebhook string `gorm:"type:varchar(500)" json:"notification_webhook"` // 群组机器人Webhook URL（用于发送值班开始通知）
	BaseModel
}

func (OnCallSchedule) TableName() string {
	return "on_call_schedules"
}

// OnCallShift 值班班次表
type OnCallShift struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ScheduleID uint      `gorm:"index;not null" json:"schedule_id"`
	UserID     string    `gorm:"type:varchar(36);index;not null" json:"user_id"` // 关联 users 表
	StartTime  time.Time `gorm:"type:datetime;index;not null" json:"start_time"`
	EndTime    time.Time `gorm:"type:datetime;index;not null" json:"end_time"`
	ShiftType  string    `gorm:"type:varchar(20);default:'manual'" json:"shift_type"`   // manual, daily, weekly, monthly
	RepeatRule string    `gorm:"type:varchar(100)" json:"repeat_rule"`                  // 重复规则（如：每天、每周一、每月1号）
	Status     string    `gorm:"type:varchar(20);default:'active';index" json:"status"` // active, cancelled
	BaseModel
}

func (OnCallShift) TableName() string {
	return "on_call_shifts"
}

// OnCallShiftWithUser 值班班次表（包含用户名）
type OnCallShiftWithUser struct {
	OnCallShift
	Username string `json:"username" gorm:"-"`
}

// OnCallAssignment 告警分配表
type OnCallAssignment struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	AlertID      uint64    `gorm:"index;not null" json:"alert_id"`
	UserID       string    `gorm:"type:varchar(36);index;not null" json:"user_id"`
	ShiftID      *uint     `gorm:"index" json:"shift_id"`
	AssignedAt   time.Time `gorm:"type:datetime;not null" json:"assigned_at"`
	AssignedBy   string    `gorm:"type:varchar(36)" json:"assigned_by"` // 分配人（如果是自动分配则为空）
	AutoAssigned bool      `gorm:"default:false" json:"auto_assigned"`
	BaseModel
}

func (OnCallAssignment) TableName() string {
	return "on_call_assignments"
}
