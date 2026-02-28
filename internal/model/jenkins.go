package model

import (
	"time"
)

// JenkinsServer Jenkins服务器配置
type JenkinsServer struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Alias       string    `gorm:"type:varchar(255);not null;index" json:"alias"` // 别名(服务器名称)
	URL         string    `gorm:"type:varchar(500);not null" json:"url"`         // Jenkins服务器URL（格式：http://host:port 或 https://host:port）
	Username    string    `gorm:"type:varchar(100);not null" json:"username"`    // 用户名
	Password    string    `gorm:"type:varchar(500);not null" json:"-"`           // 密码或API Token（加密存储）
	Description string    `gorm:"type:text" json:"description"`                  // 描述
	Enabled     bool      `gorm:"default:true;index" json:"enabled"`             // 是否启用
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 指定表名
func (JenkinsServer) TableName() string {
	return "jenkins_servers"
}

// ================== 请求和响应结构体 ==================

// JenkinsServerInfo Jenkins服务器信息（用于API响应）
type JenkinsServerInfo struct {
	ID          uint   `json:"id"`
	Alias       string `json:"alias"`
	URL         string `json:"url"`
	Username    string `json:"username"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// JenkinsServerListResponse Jenkins服务器列表响应
type JenkinsServerListResponse struct {
	List  []JenkinsServerInfo `json:"list"`
	Total int64               `json:"total"`
}

// CreateJenkinsServerRequest 创建Jenkins服务器请求
type CreateJenkinsServerRequest struct {
	Alias       string `json:"alias" binding:"required"`
	URL         string `json:"url" binding:"required"` // Jenkins服务器URL（格式：http://host:port 或 https://host:port）
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"` // 密码或API Token
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// UpdateJenkinsServerRequest 更新Jenkins服务器请求
type UpdateJenkinsServerRequest struct {
	Alias       string `json:"alias"`
	URL         string `json:"url"` // Jenkins服务器URL（格式：http://host:port 或 https://host:port）
	Username    string `json:"username"`
	Password    string `json:"password"` // 密码或API Token（如果为空则不更新）
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
}

// TestJenkinsConnectionRequest 测试Jenkins连接请求
type TestJenkinsConnectionRequest struct {
	URL      string `json:"url" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"` // 密码或API Token
}

// TestJenkinsConnectionResponse 测试Jenkins连接响应
type TestJenkinsConnectionResponse struct {
	Success    bool               `json:"success"`
	Message    string             `json:"message"`
	SystemInfo *JenkinsSystemInfo `json:"systemInfo,omitempty"`
	Error      string             `json:"error,omitempty"`
}

// JenkinsJob Jenkins任务信息
type JenkinsJob struct {
	Name                string        `json:"name"`
	DisplayName         string        `json:"displayName"`
	Description         string        `json:"description"`
	URL                 string        `json:"url"`
	Buildable           bool          `json:"buildable"`
	Color               string        `json:"color"`
	Class               string        `json:"_class"`
	LastBuild           *JenkinsBuild `json:"lastBuild"`
	LastStableBuild     *JenkinsBuild `json:"lastStableBuild"`
	LastSuccessfulBuild *JenkinsBuild `json:"lastSuccessfulBuild"`
	LastFailedBuild     *JenkinsBuild `json:"lastFailedBuild"`
}

// JenkinsBuild Jenkins构建信息
type JenkinsBuild struct {
	Number            int    `json:"number"`
	URL               string `json:"url"`
	DisplayName       string `json:"displayName"`
	FullDisplayName   string `json:"fullDisplayName"`
	Description       string `json:"description"`
	Result            string `json:"result"` // SUCCESS/FAILURE/UNSTABLE/ABORTED
	Building          bool   `json:"building"`
	Duration          int64  `json:"duration"`
	EstimatedDuration int64  `json:"estimatedDuration"`
	Timestamp         int64  `json:"timestamp"`
	KeepLog           bool   `json:"keepLog"`
	QueueId           int    `json:"queueId"`
}

// JenkinsJobListResponse Jenkins任务列表响应
type JenkinsJobListResponse struct {
	Jobs   []JenkinsJob `json:"jobs"`
	Total  int          `json:"total"`
	Server string       `json:"server"`
}

// JenkinsJobDetailResponse Jenkins任务详情响应
type JenkinsJobDetailResponse struct {
	Job    JenkinsJob     `json:"job"`
	Builds []JenkinsBuild `json:"builds"`
	Server string         `json:"server"`
}

// StartJobRequest 启动任务请求
type StartJobRequest struct {
	Parameters map[string]string `json:"parameters"`
	Reason     string            `json:"reason"`
}

// StartJobResponse 启动任务响应
type StartJobResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	QueueId     int    `json:"queueId"`
	JobName     string `json:"jobName"`
	Server      string `json:"server"`
	BuildNumber int    `json:"buildNumber"`
}

// StopBuildRequest 停止构建请求
type StopBuildRequest struct {
	Reason string `json:"reason"`
}

// StopBuildResponse 停止构建响应
type StopBuildResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	JobName     string `json:"jobName"`
	BuildNumber int    `json:"buildNumber"`
	Server      string `json:"server"`
}

// GetBuildLogRequest 获取构建日志请求
type GetBuildLogRequest struct {
	Start int  `json:"start"`
	Html  bool `json:"html"`
}

// GetBuildLogResponse 获取构建日志响应
type GetBuildLogResponse struct {
	Log         string `json:"log"`
	HasMore     bool   `json:"hasMore"`
	TextSize    int    `json:"textSize"`
	MoreData    bool   `json:"moreData"`
	JobName     string `json:"jobName"`
	BuildNumber int    `json:"buildNumber"`
	Server      string `json:"server"`
}

// JenkinsBuildDetailResponse Jenkins构建详情响应
type JenkinsBuildDetailResponse struct {
	Build  JenkinsBuild `json:"build"`
	Server string       `json:"server"`
}

// JenkinsSystemInfo Jenkins系统信息
type JenkinsSystemInfo struct {
	Version         string            `json:"version"`
	Mode            string            `json:"mode"`
	NodeDescription string            `json:"nodeDescription"`
	NodeName        string            `json:"nodeName"`
	NumExecutors    int               `json:"numExecutors"`
	UseCrumbs       bool              `json:"useCrumbs"`
	UseSecurity     bool              `json:"useSecurity"`
	Views           []JenkinsView     `json:"views"`
	PrimaryView     *JenkinsView      `json:"primaryView"`
	UnlabeledLoad   map[string]int    `json:"unlabeledLoad,omitempty"`
	AssignedLabels  []JenkinsLabel    `json:"assignedLabels,omitempty"`
	OverallLoad     map[string]int    `json:"overallLoad,omitempty"`
	Computers       []JenkinsComputer `json:"computers"`
}

// JenkinsView Jenkins视图
type JenkinsView struct {
	Name        string       `json:"name"`
	URL         string       `json:"url"`
	Description string       `json:"description"`
	Jobs        []JenkinsJob `json:"jobs"`
}

// JenkinsLabel Jenkins标签
type JenkinsLabel struct {
	Name string `json:"name"`
}

// JenkinsComputer Jenkins计算机(节点)
type JenkinsComputer struct {
	DisplayName         string                 `json:"displayName"`
	Executors           []JenkinsExecutor      `json:"executors,omitempty"`
	Icon                string                 `json:"icon,omitempty"`
	IconClassName       string                 `json:"iconClassName,omitempty"`
	Idle                bool                   `json:"idle,omitempty"`
	JnlpAgent           bool                   `json:"jnlpAgent,omitempty"`
	LaunchSupported     bool                   `json:"launchSupported,omitempty"`
	LoadStatistics      JenkinsLoadStatistics  `json:"loadStatistics,omitempty"`
	ManualLaunchAllowed bool                   `json:"manualLaunchAllowed,omitempty"`
	MonitorData         map[string]interface{} `json:"monitorData,omitempty"`
	NumExecutors        int                    `json:"numExecutors"`
	Offline             bool                   `json:"offline"`
	OfflineCause        interface{}            `json:"offlineCause,omitempty"`
	OneOffExecutors     []JenkinsExecutor      `json:"oneOffExecutors,omitempty"`
	TemporarilyOffline  bool                   `json:"temporarilyOffline,omitempty"`
}

// JenkinsExecutor Jenkins执行器
type JenkinsExecutor struct {
	CurrentExecutable interface{} `json:"currentExecutable,omitempty"`
	CurrentWorkUnit   interface{} `json:"currentWorkUnit,omitempty"`
	Idle              bool        `json:"idle"`
	LikelyStuck       bool        `json:"likelyStuck,omitempty"`
	Number            int         `json:"number"`
	Progress          int         `json:"progress,omitempty"`
}

// JenkinsLoadStatistics Jenkins负载统计
type JenkinsLoadStatistics struct {
	BusyExecutors  int `json:"busyExecutors"`
	IdleExecutors  int `json:"idleExecutors"`
	TotalExecutors int `json:"totalExecutors"`
	QueueLength    int `json:"queueLength"`
}

// JenkinsQueue Jenkins队列信息
type JenkinsQueue struct {
	Items []JenkinsQueueItem `json:"items"`
}

// JenkinsQueueItem Jenkins队列项目
type JenkinsQueueItem struct {
	Actions                    []interface{} `json:"actions,omitempty"`
	Blocked                    bool          `json:"blocked"`
	Buildable                  bool          `json:"buildable"`
	Id                         int           `json:"id"`
	InQueueSince               int64         `json:"inQueueSince"`
	Params                     string        `json:"params,omitempty"`
	Stuck                      bool          `json:"stuck"`
	Task                       JenkinsTask   `json:"task"`
	URL                        string        `json:"url,omitempty"`
	Why                        string        `json:"why,omitempty"`
	BuildableStartMilliseconds int64         `json:"buildableStartMilliseconds,omitempty"`
}

// JenkinsTask Jenkins任务
type JenkinsTask struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Color string `json:"color"`
}
