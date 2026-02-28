package jenkins

import (
	"net/http"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	jenkinsService "github.com/fisker086/keyops/internal/service/jenkins"
	"github.com/gin-gonic/gin"
)

type JenkinsHandler struct {
	jenkinsService *jenkinsService.JenkinsService
}

func NewJenkinsHandler(jenkinsService *jenkinsService.JenkinsService) *JenkinsHandler {
	return &JenkinsHandler{
		jenkinsService: jenkinsService,
	}
}

// GetJenkinsServers 获取Jenkins服务器列表
// @Summary 获取Jenkins服务器列表
// @Description 获取所有配置的Jenkins服务器
// @Tags Jenkins
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} model.Response{data=model.JenkinsServerListResponse}
// @Router /api/jenkins/servers [get]
func (h *JenkinsHandler) GetJenkinsServers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	servers, total, err := h.jenkinsService.GetJenkinsServers(page, pageSize)
	if err != nil {
	c.JSON(http.StatusInternalServerError, model.Error(http.StatusInternalServerError, "获取Jenkins服务器列表失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(model.JenkinsServerListResponse{
		List:  servers,
		Total: total,
	}))
}

// GetJenkinsServerDetail 获取Jenkins服务器详情
// @Summary 获取Jenkins服务器详情
// @Description 根据服务器ID获取Jenkins服务器详情
// @Tags Jenkins
// @Accept json
// @Produce json
// @Param id path int true "服务器ID"
// @Success 200 {object} model.Response{data=model.JenkinsServerInfo}
// @Router /api/jenkins/servers/{id} [get]
func (h *JenkinsHandler) GetJenkinsServerDetail(c *gin.Context) {
	serverIDStr := c.Param("id")
	serverID, err := strconv.ParseUint(serverIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(http.StatusBadRequest, "无效的服务器ID"))
		return
	}

	serverInfo, err := h.jenkinsService.GetJenkinsServerDetail(uint(serverID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(http.StatusInternalServerError, "获取Jenkins服务器详情失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(serverInfo))
}

// CreateJenkinsServer 创建Jenkins服务器
// @Summary 创建Jenkins服务器
// @Description 创建新的Jenkins服务器配置
// @Tags Jenkins
// @Accept json
// @Produce json
// @Param server body model.CreateJenkinsServerRequest true "服务器信息"
// @Success 200 {object} model.Response{data=model.JenkinsServer}
// @Router /api/jenkins/servers [post]
func (h *JenkinsHandler) CreateJenkinsServer(c *gin.Context) {
	var req model.CreateJenkinsServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(http.StatusBadRequest, "请求参数错误: "+err.Error()))
		return
	}

	server, err := h.jenkinsService.CreateJenkinsServer(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(http.StatusInternalServerError, "创建Jenkins服务器失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(server))
}

// UpdateJenkinsServer 更新Jenkins服务器
// @Summary 更新Jenkins服务器
// @Description 更新Jenkins服务器配置
// @Tags Jenkins
// @Accept json
// @Produce json
// @Param id path int true "服务器ID"
// @Param server body model.UpdateJenkinsServerRequest true "服务器信息"
// @Success 200 {object} model.Response
// @Router /api/jenkins/servers/{id} [put]
func (h *JenkinsHandler) UpdateJenkinsServer(c *gin.Context) {
	serverIDStr := c.Param("id")
	serverID, err := strconv.ParseUint(serverIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(http.StatusBadRequest, "无效的服务器ID"))
		return
	}

	var req model.UpdateJenkinsServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(http.StatusBadRequest, "请求参数错误: "+err.Error()))
		return
	}

	if err := h.jenkinsService.UpdateJenkinsServer(uint(serverID), &req); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(http.StatusInternalServerError, "更新Jenkins服务器失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// DeleteJenkinsServer 删除Jenkins服务器
// @Summary 删除Jenkins服务器
// @Description 删除Jenkins服务器配置
// @Tags Jenkins
// @Accept json
// @Produce json
// @Param id path int true "服务器ID"
// @Success 200 {object} model.Response
// @Router /api/jenkins/servers/{id} [delete]
func (h *JenkinsHandler) DeleteJenkinsServer(c *gin.Context) {
	serverIDStr := c.Param("id")
	serverID, err := strconv.ParseUint(serverIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "无效的服务器ID",
		})
		return
	}

	if err := h.jenkinsService.DeleteJenkinsServer(uint(serverID)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(http.StatusInternalServerError, "删除Jenkins服务器失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// TestJenkinsConnection 测试Jenkins连接
// @Summary 测试Jenkins连接
// @Description 测试Jenkins服务器连接是否正常
// @Tags Jenkins
// @Accept json
// @Produce json
// @Param request body model.TestJenkinsConnectionRequest true "连接测试请求"
// @Success 200 {object} model.Response{data=model.TestJenkinsConnectionResponse}
// @Router /api/jenkins/test-connection [post]
func (h *JenkinsHandler) TestJenkinsConnection(c *gin.Context) {
	var req model.TestJenkinsConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(http.StatusBadRequest, "请求参数错误: "+err.Error()))
		return
	}

	response, err := h.jenkinsService.TestJenkinsConnection(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(http.StatusInternalServerError, "测试连接失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(response))
}

// GetJobs 获取Jenkins任务列表
// @Summary 获取Jenkins任务列表
// @Description 获取指定Jenkins服务器的所有任务
// @Tags Jenkins
// @Accept json
// @Produce json
// @Param serverId path int true "服务器ID"
// @Success 200 {object} model.Response{data=model.JenkinsJobListResponse}
// @Router /api/jenkins/{serverId}/jobs [get]
func (h *JenkinsHandler) GetJobs(c *gin.Context) {
	serverIDStr := c.Param("serverId")
	serverID, err := strconv.ParseUint(serverIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(http.StatusBadRequest, "无效的服务器ID"))
		return
	}

	response, err := h.jenkinsService.GetJobs(uint(serverID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(http.StatusInternalServerError, "获取任务列表失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(response))
}

// SearchJobs 搜索Jenkins任务
// @Summary 搜索Jenkins任务
// @Description 根据关键词模糊搜索指定Jenkins服务器的任务
// @Tags Jenkins
// @Accept json
// @Produce json
// @Param serverId path int true "服务器ID"
// @Param keyword query string true "搜索关键词"
// @Success 200 {object} model.Response{data=model.JenkinsJobListResponse}
// @Router /api/jenkins/{serverId}/jobs/search [get]
func (h *JenkinsHandler) SearchJobs(c *gin.Context) {
	serverIDStr := c.Param("serverId")
	serverID, err := strconv.ParseUint(serverIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "无效的服务器ID",
		})
		return
	}

	keyword := c.Query("keyword")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, model.Error(http.StatusBadRequest, "搜索关键词不能为空"))
		return
	}

	response, err := h.jenkinsService.SearchJobs(uint(serverID), keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(http.StatusInternalServerError, "搜索任务失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(response))
}

// GetJobDetail 获取Jenkins任务详情
// @Summary 获取Jenkins任务详情
// @Description 获取指定任务的详细信息和构建历史
// @Tags Jenkins
// @Accept json
// @Produce json
// @Param serverId path int true "服务器ID"
// @Param jobName path string true "任务名称"
// @Success 200 {object} model.Response{data=model.JenkinsJobDetailResponse}
// @Router /api/jenkins/{serverId}/jobs/{jobName} [get]
func (h *JenkinsHandler) GetJobDetail(c *gin.Context) {
	serverIDStr := c.Param("serverId")
	serverID, err := strconv.ParseUint(serverIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "无效的服务器ID",
		})
		return
	}

	jobName := c.Param("jobName")
	if jobName == "" {
		c.JSON(http.StatusBadRequest, model.Error(http.StatusBadRequest, "任务名称不能为空"))
		return
	}

	response, err := h.jenkinsService.GetJobDetail(uint(serverID), jobName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(http.StatusInternalServerError, "获取任务详情失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(response))
}

// StartJob 启动Jenkins任务
// @Summary 启动Jenkins任务
// @Description 启动指定的Jenkins任务，支持带参数构建
// @Tags Jenkins
// @Accept json
// @Produce json
// @Param serverId path int true "服务器ID"
// @Param jobName path string true "任务名称"
// @Param request body model.StartJobRequest false "启动任务请求"
// @Success 200 {object} model.Response{data=model.StartJobResponse}
// @Router /api/jenkins/{serverId}/jobs/{jobName}/start [post]
func (h *JenkinsHandler) StartJob(c *gin.Context) {
	serverIDStr := c.Param("serverId")
	serverID, err := strconv.ParseUint(serverIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "无效的服务器ID",
		})
		return
	}

	jobName := c.Param("jobName")
	if jobName == "" {
		c.JSON(http.StatusBadRequest, model.Error(http.StatusBadRequest, "任务名称不能为空"))
		return
	}

	var req model.StartJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 如果没有提供请求体，使用空的请求
		req = model.StartJobRequest{}
	}

	response, err := h.jenkinsService.StartJob(uint(serverID), jobName, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(http.StatusInternalServerError, "启动任务失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(response))
}

// StopBuild 停止Jenkins构建
// @Summary 停止Jenkins构建
// @Description 停止指定的Jenkins构建任务
// @Tags Jenkins
// @Accept json
// @Produce json
// @Param serverId path int true "服务器ID"
// @Param jobName path string true "任务名称"
// @Param buildNumber path int true "构建编号"
// @Success 200 {object} model.Response{data=model.StopBuildResponse}
// @Router /api/jenkins/{serverId}/jobs/{jobName}/builds/{buildNumber}/stop [post]
func (h *JenkinsHandler) StopBuild(c *gin.Context) {
	serverIDStr := c.Param("serverId")
	serverID, err := strconv.ParseUint(serverIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "无效的服务器ID",
		})
		return
	}

	jobName := c.Param("jobName")
	if jobName == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "任务名称不能为空",
		})
		return
	}

	buildNumberStr := c.Param("buildNumber")
	buildNumber, err := strconv.Atoi(buildNumberStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(http.StatusBadRequest, "无效的构建编号"))
		return
	}

	response, err := h.jenkinsService.StopBuild(uint(serverID), jobName, buildNumber)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(http.StatusInternalServerError, "停止构建失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(response))
}

// GetBuildDetail 获取Jenkins构建详情
// @Summary 获取Jenkins构建详情
// @Description 获取指定构建的详细信息
// @Tags Jenkins
// @Accept json
// @Produce json
// @Param serverId path int true "服务器ID"
// @Param jobName path string true "任务名称"
// @Param buildNumber path int true "构建编号"
// @Success 200 {object} model.Response{data=model.JenkinsBuildDetailResponse}
// @Router /api/jenkins/{serverId}/jobs/{jobName}/builds/{buildNumber} [get]
func (h *JenkinsHandler) GetBuildDetail(c *gin.Context) {
	serverIDStr := c.Param("serverId")
	serverID, err := strconv.ParseUint(serverIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "无效的服务器ID",
		})
		return
	}

	jobName := c.Param("jobName")
	if jobName == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "任务名称不能为空",
		})
		return
	}

	buildNumberStr := c.Param("buildNumber")
	buildNumber, err := strconv.Atoi(buildNumberStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(http.StatusBadRequest, "无效的构建编号"))
		return
	}

	response, err := h.jenkinsService.GetBuildDetail(uint(serverID), jobName, buildNumber)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(http.StatusInternalServerError, "获取构建详情失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(response))
}

// GetBuildLog 获取Jenkins构建日志
// @Summary 获取Jenkins构建日志
// @Description 获取指定构建的日志信息，支持分页获取
// @Tags Jenkins
// @Accept json
// @Produce json
// @Param serverId path int true "服务器ID"
// @Param jobName path string true "任务名称"
// @Param buildNumber path int true "构建编号"
// @Param start query int false "开始位置" default(0)
// @Success 200 {object} model.Response{data=model.GetBuildLogResponse}
// @Router /api/jenkins/{serverId}/jobs/{jobName}/builds/{buildNumber}/log [get]
func (h *JenkinsHandler) GetBuildLog(c *gin.Context) {
	serverIDStr := c.Param("serverId")
	serverID, err := strconv.ParseUint(serverIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "无效的服务器ID",
		})
		return
	}

	jobName := c.Param("jobName")
	if jobName == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "任务名称不能为空",
		})
		return
	}

	buildNumberStr := c.Param("buildNumber")
	buildNumber, err := strconv.Atoi(buildNumberStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "无效的构建编号",
		})
		return
	}

	start, _ := strconv.Atoi(c.DefaultQuery("start", "0"))

	response, err := h.jenkinsService.GetBuildLog(uint(serverID), jobName, buildNumber, start)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(http.StatusInternalServerError, "获取构建日志失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(response))
}

// GetSystemInfo 获取Jenkins系统信息
// @Summary 获取Jenkins系统信息
// @Description 获取指定Jenkins服务器的系统信息
// @Tags Jenkins
// @Accept json
// @Produce json
// @Param serverId path int true "服务器ID"
// @Success 200 {object} model.Response{data=model.JenkinsSystemInfo}
// @Router /api/jenkins/{serverId}/system-info [get]
func (h *JenkinsHandler) GetSystemInfo(c *gin.Context) {
	serverIDStr := c.Param("serverId")
	serverID, err := strconv.ParseUint(serverIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "无效的服务器ID",
		})
		return
	}

	systemInfo, err := h.jenkinsService.GetSystemInfo(uint(serverID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(http.StatusInternalServerError, "获取系统信息失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(systemInfo))
}

// GetQueueInfo 获取Jenkins队列信息
// @Summary 获取Jenkins队列信息
// @Description 获取指定Jenkins服务器的构建队列信息
// @Tags Jenkins
// @Accept json
// @Produce json
// @Param serverId path int true "服务器ID"
// @Success 200 {object} model.Response{data=model.JenkinsQueue}
// @Router /api/jenkins/{serverId}/queue [get]
func (h *JenkinsHandler) GetQueueInfo(c *gin.Context) {
	serverIDStr := c.Param("serverId")
	serverID, err := strconv.ParseUint(serverIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "无效的服务器ID",
		})
		return
	}

	queueInfo, err := h.jenkinsService.GetQueueInfo(uint(serverID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(http.StatusInternalServerError, "获取队列信息失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(queueInfo))
}

