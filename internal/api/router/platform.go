package router

import (
	"github.com/fisker086/keyops/internal/aiassistant"
	"github.com/fisker086/keyops/internal/api/handler"
	"github.com/fisker086/keyops/internal/api/middleware"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/internal/service"
	"github.com/gin-gonic/gin"
)

func registerPlatform(
	authenticated *gin.RouterGroup,
	k8sHandler *handler.K8sHandler,
	k8sClusterHandler *handler.K8sClusterHandler,
	k8sPermissionHandler *handler.K8sPermissionHandler,
	k8sSearchHandler *handler.K8sSearchHandler,
	deploymentHandler *handler.DeploymentHandler,
	releaseHandler *handler.ReleaseHandler,
	buildMasterHandler *handler.BuildMasterHandler,
	registryHandler *handler.RegistryHandler,
	monitorHandler *handler.MonitorHandler,
	jenkinsHandler *handler.JenkinsHandler,
	auditHandler *handler.AuditHandler,
	dmsInstanceHandler *handler.DMSInstanceHandler,
	dmsQueryHandler *handler.DMSQueryHandler,
	dmsQueryLogHandler *handler.DMSQueryLogHandler,
	dmsPermissionHandler *handler.DMSPermissionHandler,
	aiAssistantHandler *aiassistant.Handler,
	k8sPermissionService *service.K8sPermissionService,
	roleRepo repository.RoleRepository,
) {
	k8sClusters := authenticated.Group("/k8s/clusters")
	k8sClusters.Use(middleware.OperationLogMiddleware())
	{
		k8sClusters.GET("", k8sClusterHandler.ListClusters)
		k8sClusters.GET("/summary", k8sClusterHandler.GetAllClustersSummary)
		k8sClusters.GET("/dashboard/statistics", k8sClusterHandler.GetDashboardStatistics)
		k8sClusters.POST("", k8sClusterHandler.CreateCluster)
		k8sClusters.GET("/:id", k8sClusterHandler.GetCluster)
		k8sClusters.GET("/:id/summary", k8sClusterHandler.GetClusterSummary)
		k8sClusters.GET("/:id/permissions", k8sPermissionHandler.GetClusterPermissions)
		k8sClusters.GET("/:id/permitted-namespaces", k8sPermissionHandler.GetPermittedNamespaces)
		k8sClusters.GET("/:id/effective-action", k8sPermissionHandler.GetEffectiveAction)
		k8sClusters.PUT("/:id", k8sClusterHandler.UpdateCluster)
		k8sClusters.DELETE("/:id", k8sClusterHandler.DeleteCluster)
	}

	k8sPermissions := authenticated.Group("/k8s/permissions")
	k8sPermissions.Use(middleware.OperationLogMiddleware())
	{
		k8sPermissions.GET("", k8sPermissionHandler.GetPermissions)
		k8sPermissions.POST("", k8sPermissionHandler.AddPermission)
		k8sPermissions.PUT("", k8sPermissionHandler.UpdatePermission)
		k8sPermissions.DELETE("", k8sPermissionHandler.RemovePermission)
		k8sPermissions.POST("/check", k8sPermissionHandler.CheckPermission)
	}

	k8sSearch := authenticated.Group("/k8s/search")
	{
		k8sSearch.GET("", k8sSearchHandler.GlobalSearch)
	}

	k8s := authenticated.Group("/v1/kube")
	k8s.Use(middleware.OperationLogMiddleware())
	k8s.Use(middleware.K8sPermissionMiddleware(k8sPermissionService, roleRepo))
	{
		k8s.GET("/base", k8sHandler.GetBaseInfo)
		k8s.GET("/namespace", k8sHandler.GetNamespaceList)
		k8s.GET("/pod", k8sHandler.GetPodList)
		k8s.GET("/pod/detail", k8sHandler.GetPodDetail)
		k8s.GET("/service", k8sHandler.GetServiceList)
		k8s.GET("/ingress", k8sHandler.GetIngressList)
		k8s.GET("/hpa", k8sHandler.GetHPAList)
		k8s.GET("/event", k8sHandler.GetEventList)
		k8s.GET("/deployment", k8sHandler.GetDeploymentList)
		k8s.GET("/deployment/:deployment_name", k8sHandler.GetDeploymentDetail)
		k8s.GET("/daemonset", k8sHandler.GetDaemonSetList)
		k8s.GET("/daemonset/:daemonset_name", k8sHandler.GetDaemonSetDetail)
		k8s.GET("/statefulset", k8sHandler.GetStatefulSetList)
		k8s.GET("/statefulset/:statefulset_name", k8sHandler.GetStatefulSetDetail)
		k8s.GET("/cronjob", k8sHandler.GetCronJobList)
		k8s.GET("/cronjob/:cronjob_name", k8sHandler.GetCronJobDetail)
		k8s.GET("/job", k8sHandler.GetJobList)
		k8s.GET("/job/:job_name", k8sHandler.GetJobDetail)
		k8s.GET("/node", k8sHandler.GetNodeList)
		k8s.GET("/pv", k8sHandler.GetPVList)
		k8s.GET("/pvc", k8sHandler.GetPVCList)
		k8s.GET("/storageclass", k8sHandler.GetStorageClassList)
		k8s.GET("/configmap", k8sHandler.GetConfigMapList)
		k8s.GET("/secret", k8sHandler.GetSecretList)
		k8s.GET("/containers", k8sHandler.GetContainersList)
		k8s.GET("/scale", k8sHandler.GetReplica)
		k8s.POST("/scale", k8sHandler.ScaleReplica)
		k8s.DELETE("/pod", k8sHandler.RestartPod)
		k8s.GET("/pod/down_logs", k8sHandler.DownloadContainerLogs)
		k8s.GET("/pod/metrics", k8sHandler.GetPodMetrics)
		k8s.GET("/pod/ws/logs", k8sHandler.StreamPodLogs)
		k8s.GET("/pod/ws/terminal", k8sHandler.ConnectPodTerminal)
		k8s.GET("/yaml", k8sHandler.GetResourceYaml)
		k8s.PUT("/yaml", k8sHandler.UpdateResourceYaml)
		k8s.DELETE("/yaml", k8sHandler.DeleteResource)
		k8s.POST("/yaml/dry-run", k8sHandler.DryRunResourceYaml)
		k8s.GET("/deployment/:deployment_name/revisions", k8sHandler.GetDeploymentRevisions)
		k8s.POST("/deployment/:deployment_name/rollback", k8sHandler.RollbackDeployment)
		k8s.GET("/deployment/:deployment_name/metrics", k8sHandler.GetDeploymentMetrics)
		k8s.GET("/daemonset/:daemonset_name/revisions", k8sHandler.GetDaemonSetRevisions)
		k8s.POST("/daemonset/:daemonset_name/rollback", k8sHandler.RollbackDaemonSet)
		k8s.GET("/daemonset/:daemonset_name/metrics", k8sHandler.GetDaemonSetMetrics)
		k8s.GET("/statefulset/:statefulset_name/revisions", k8sHandler.GetStatefulSetRevisions)
		k8s.POST("/statefulset/:statefulset_name/rollback", k8sHandler.RollbackStatefulSet)
		k8s.GET("/statefulset/:statefulset_name/metrics", k8sHandler.GetStatefulSetMetrics)
	}

	deployments := authenticated.Group("/deployments")
	{
		deployments.GET("", deploymentHandler.ListDeployments)
		deployments.POST("", deploymentHandler.CreateDeployment)
		deployments.GET("/:id", deploymentHandler.GetDeployment)
		deployments.POST("/:id/execute", deploymentHandler.ExecuteK8sDeployment)
		deployments.PUT("/:id/status", deploymentHandler.UpdateDeploymentStatus)
		deployments.DELETE("/:id", deploymentHandler.DeleteDeployment)
	}

	release := authenticated.Group("/release")
	{
		release.GET("/runs", releaseHandler.ListRuns)
		release.GET("/runs/:id", releaseHandler.GetRun)
		release.POST("/runs", releaseHandler.CreateRun)
		release.POST("/runs/:id/execute", releaseHandler.ExecuteRun)
		release.POST("/runs/:id/status", releaseHandler.UpdateRunStatus)
		release.GET("/applications/:id/last-prod-run", releaseHandler.GetLastProdRun)
		release.POST("/rollback", releaseHandler.RollbackProd)
		release.GET("/pipelines", releaseHandler.ListPipelines)
		release.GET("/pipeline", releaseHandler.GetPipeline)
		release.PUT("/pipeline", releaseHandler.SavePipeline)
		release.DELETE("/pipeline/:id", releaseHandler.DeletePipeline)
	}

	buildMaster := authenticated.Group("/build-master")
	{
		buildMaster.GET("/lists", buildMasterHandler.List)
		buildMaster.GET("/lists/:id", buildMasterHandler.Get)
		buildMaster.POST("/lists", buildMasterHandler.Create)
		buildMaster.PATCH("/lists/:id", buildMasterHandler.Update)
		buildMaster.GET("/records", buildMasterHandler.RecordsByQuery)
	}

	registry := authenticated.Group("/registry")
	{
		registry.GET("/applications/:appId/versions", registryHandler.GetApplicationVersions)
		registry.GET("/test", registryHandler.TestConnection)
	}

	monitors := authenticated.Group("/monitors")
	{
		monitors.GET("/prom", monitorHandler.ListMonitors)
		monitors.GET("/prom/count", monitorHandler.CountMonitors)
		monitors.POST("/prom", monitorHandler.CreateMonitor)
		monitors.GET("/prom/:id", monitorHandler.GetMonitor)
		monitors.PUT("/prom/:id", monitorHandler.UpdateMonitor)
		monitors.DELETE("/prom/:id", monitorHandler.DeleteMonitor)
		monitors.GET("/probe", monitorHandler.GetProbe)
	}

	jenkins := authenticated.Group("/jenkins")
	{
		jenkins.GET("/servers", jenkinsHandler.GetJenkinsServers)
		jenkins.POST("/servers", jenkinsHandler.CreateJenkinsServer)
		jenkins.GET("/servers/:id", jenkinsHandler.GetJenkinsServerDetail)
		jenkins.PUT("/servers/:id", jenkinsHandler.UpdateJenkinsServer)
		jenkins.DELETE("/servers/:id", jenkinsHandler.DeleteJenkinsServer)
		jenkins.POST("/test-connection", jenkinsHandler.TestJenkinsConnection)
		jenkins.GET("/:serverId/jobs", jenkinsHandler.GetJobs)
		jenkins.GET("/:serverId/jobs/search", jenkinsHandler.SearchJobs)
		jenkins.GET("/:serverId/jobs/:jobName", jenkinsHandler.GetJobDetail)
		jenkins.POST("/:serverId/jobs/:jobName/start", jenkinsHandler.StartJob)
		jenkins.GET("/:serverId/jobs/:jobName/builds/:buildNumber", jenkinsHandler.GetBuildDetail)
		jenkins.POST("/:serverId/jobs/:jobName/builds/:buildNumber/stop", jenkinsHandler.StopBuild)
		jenkins.GET("/:serverId/jobs/:jobName/builds/:buildNumber/log", jenkinsHandler.GetBuildLog)
		jenkins.GET("/:serverId/system-info", jenkinsHandler.GetSystemInfo)
		jenkins.GET("/:serverId/queue", jenkinsHandler.GetQueueInfo)
	}

	audit := authenticated.Group("/v1/audit")
	{
		audit.GET("/operation-logs", auditHandler.GetOperationLogs)
		audit.GET("/operation-logs/:id", auditHandler.GetOperationLogDetail)
		audit.DELETE("/operation-logs/:id", auditHandler.DeleteOperationLog)
		audit.DELETE("/operation-logs/batch", auditHandler.BatchDeleteOperationLogs)
		audit.GET("/pod-commands", auditHandler.GetPodCommandLogs)
	}

	dms := authenticated.Group("/dms")
	{
		dms.GET("/instances", dmsInstanceHandler.ListInstances)
		dms.POST("/instances", dmsInstanceHandler.CreateInstance)
		dms.POST("/instances/test-connection", dmsInstanceHandler.TestConnectionWithBody)
		dms.GET("/instances/:id", dmsInstanceHandler.GetInstance)
		dms.PUT("/instances/:id", dmsInstanceHandler.UpdateInstance)
		dms.DELETE("/instances/:id", dmsInstanceHandler.DeleteInstance)
		dms.POST("/instances/:id/test", dmsInstanceHandler.TestConnection)
		dms.POST("/query/execute", dmsQueryHandler.ExecuteQuery)
		dms.GET("/query/databases", dmsQueryHandler.GetDatabases)
		dms.GET("/query/tables", dmsQueryHandler.GetTables)
		dms.GET("/logs/queries", dmsQueryLogHandler.ListQueryLogs)
		dms.GET("/logs/queries/:id", dmsQueryLogHandler.GetQueryLog)
		dms.GET("/permissions", dmsPermissionHandler.GetUserPermissions)
		dms.GET("/permissions/my", dmsPermissionHandler.GetMyPermissions)
		dms.POST("/permissions", dmsPermissionHandler.GrantPermission)
		dms.POST("/permissions/batch", dmsPermissionHandler.BatchGrantPermissions)
		dms.PUT("/permissions", dmsPermissionHandler.UpdatePermission)
		dms.PUT("/permissions/resource", dmsPermissionHandler.UpdatePermissionResource)
		dms.DELETE("/permissions", dmsPermissionHandler.RevokePermission)
	}

	if aiAssistantHandler != nil {
		aiAssistantHandler.RegisterRoutes(authenticated)
	}
}
