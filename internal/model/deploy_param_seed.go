package model

import (
	"github.com/fisker086/keyops/pkg/logger"
	"gorm.io/gorm"
)

// deployParamSeed 初始化的参数数据
type deployParamSeed struct {
	Name           string
	DataType       string
	ChineseName    string
	ValidationRule string
	Description    string
	GroupName      string
	SortOrder      int
	Category       string
}

var deployParamSeeds = []deployParamSeed{
	// ===== 基础配置 =====
	{Name: "docker_registry", DataType: "string", ChineseName: "镜像仓库地址", ValidationRule: "^[a-z0-9]+([-.]{1}[a-z0-9]+)*.[a-z]{2,7}$", Description: "不带HTTP前缀", GroupName: "基础配置", SortOrder: 1, Category: "basic"},
	{Name: "docker_registry_credential", DataType: "string", ChineseName: "镜像仓库凭据", ValidationRule: "^(qingcloud|aliyun|huawei)_registry$", Description: "凭据保存在Jenkins", GroupName: "基础配置", SortOrder: 2, Category: "basic"},
	{Name: "permission_verify", DataType: "bool", ChineseName: "开启权限验证", GroupName: "基础配置", SortOrder: 3, Category: "basic"},
	{Name: "permission_admin", DataType: "string", ChineseName: "管理员用户", Description: "多个用户以逗号分割", GroupName: "基础配置", SortOrder: 4, Category: "basic"},
	{Name: "permission_users", DataType: "string", ChineseName: "普通用户", Description: "多个用户以逗号分割", GroupName: "基础配置", SortOrder: 5, Category: "basic"},
	{Name: "cluster", DataType: "string", ChineseName: "部署集群", Description: "集群名称", GroupName: "基础配置", SortOrder: 6, Category: "basic"},
	{Name: "apiserver", DataType: "string", ChineseName: "集群地址", Description: "集群apiserver地址，无需在配置中心配置，jenkinsfile中处理", GroupName: "基础配置", SortOrder: 7, Category: "basic"},
	{Name: "namespace", DataType: "string", ChineseName: "命名空间", Description: "默认由项目组和环境拼接", GroupName: "基础配置", SortOrder: 8, Category: "basic"},
	{Name: "kube_timeout", DataType: "uint", ChineseName: "执行kubectl超时时间", GroupName: "基础配置", SortOrder: 9, Category: "basic"},

	// ===== 控制器 =====
	{Name: "k8s.controller.enabled", DataType: "bool", ChineseName: "启用部署", Description: "部署无状态或者有状态应用，默认为Deployment", GroupName: "控制器", SortOrder: 10, Category: "k8s"},
	{Name: "k8s.controller.kind", DataType: "string", ChineseName: "部署类型", ValidationRule: "^(Deployment|StatefulSet)$", GroupName: "控制器", SortOrder: 11, Category: "k8s"},
	{Name: "k8s.replicaCount", DataType: "uint", ChineseName: "Pod副本数", GroupName: "控制器", SortOrder: 12, Category: "k8s"},
	{Name: "k8s.minReadySeconds", DataType: "uint", ChineseName: "最小等待时间", GroupName: "控制器", SortOrder: 13, Category: "k8s"},
	{Name: "k8s.revisionHistoryLimit", DataType: "uint", ChineseName: "保存最大历史部署版本", GroupName: "控制器", SortOrder: 14, Category: "k8s"},
	{Name: "k8s.podManagementPolicy", DataType: "string", ChineseName: "Pod创建策略", ValidationRule: "^(OrderedReady|Parallel)$", Description: "仅限StatefuleSet", GroupName: "控制器", SortOrder: 15, Category: "k8s"},
	{Name: "k8s.strategy.type", DataType: "string", ChineseName: "部署更新类型", ValidationRule: "^(RollingUpdate|Recreate)$", Description: "默认为RollingUpdate，Deployment支持RollingUpdate和Recreate，StatefuleSet支持OnDelete 和RollingUpdate", GroupName: "控制器", SortOrder: 16, Category: "k8s"},
	{Name: "k8s.strategy.maxSurge", DataType: "string", ChineseName: "更新最大峰值", Description: "可以是具体数值或百分比，默认为25%，仅限Deployment", GroupName: "控制器", SortOrder: 17, Category: "k8s"},
	{Name: "k8s.strategy.maxUnavailable", DataType: "string", ChineseName: "更新最大不可用", Description: "可以是具体数值或百分比，默认为25%，仅限Deployment", GroupName: "控制器", SortOrder: 18, Category: "k8s"},
	{Name: "k8s.strategy.partition", DataType: "string", ChineseName: "更新分区序号", Description: "仅限StatefuleSet", GroupName: "控制器", SortOrder: 19, Category: "k8s"},

	// ===== 亲和性 =====
	{Name: "k8s.affinity.enabled", DataType: "bool", ChineseName: "启用亲和性调度", Description: "是否开启亲和性调度，默认为true", GroupName: "亲和性", SortOrder: 20, Category: "k8s"},
	{Name: "k8s.affinity.nodeAffinity.enabled", DataType: "bool", ChineseName: "启用节点亲和性调度", GroupName: "亲和性", SortOrder: 21, Category: "k8s"},
	{Name: "k8s.affinity.nodeAffinity.type", DataType: "string", ChineseName: "节点亲和性调度类型", ValidationRule: "^(soft|hard)$", Description: "节点亲和性类型。软亲和（soft）还是硬亲和（hard）", GroupName: "亲和性", SortOrder: 22, Category: "k8s"},
	{Name: "k8s.affinity.nodeAffinity.key", DataType: "string", ChineseName: "节点亲和性KEY", Description: "节点亲和性默认KEY是role", GroupName: "亲和性", SortOrder: 23, Category: "k8s"},
	{Name: "k8s.affinity.nodeAffinity.values[0]", DataType: "string", ChineseName: "节点亲和性VALUE", Description: "写多个表示需满足key=value1或key=value2", GroupName: "亲和性", SortOrder: 24, Category: "k8s"},
	{Name: "k8s.affinity.nodeAffinity.values[1]", DataType: "string", ChineseName: "节点亲和性VALUE", Description: "写多个表示需满足key=value1或key=value2", GroupName: "亲和性", SortOrder: 25, Category: "k8s"},
	{Name: "k8s.affinity.podAffinity.enabled", DataType: "bool", ChineseName: "启用Pod亲和性调度", Description: "是否开启Pod亲和性调度，默认为false", GroupName: "亲和性", SortOrder: 26, Category: "k8s"},
	{Name: "k8s.affinity.podAffinity.type", DataType: "string", ChineseName: "Pod亲和性调度类型", ValidationRule: "^(soft|hard)$", Description: "Pod亲和性类型。软亲和（soft）还是硬亲和（hard）", GroupName: "亲和性", SortOrder: 27, Category: "k8s"},
	{Name: "k8s.affinity.podAntiAffinity.enabled", DataType: "bool", ChineseName: "启用Pod反亲和性调度", Description: "是否开启Pod反亲和性调度，默认为false", GroupName: "亲和性", SortOrder: 28, Category: "k8s"},
	{Name: "k8s.affinity.podAntiAffinity.type", DataType: "string", ChineseName: "Pod反亲和性调度类型", ValidationRule: "^(soft|hard)$", Description: "Pod亲和性类型。软亲和（soft）还是硬亲和（hard）", GroupName: "亲和性", SortOrder: 29, Category: "k8s"},

	// ===== 污点容忍 =====
	{Name: "k8s.tolerations[0].key", DataType: "string", ChineseName: "容忍KEY", GroupName: "污点容忍", SortOrder: 30, Category: "k8s"},
	{Name: "k8s.tolerations[0].operator", DataType: "string", ChineseName: "容忍操作", ValidationRule: "^(Exists|Equal)$", GroupName: "污点容忍", SortOrder: 31, Category: "k8s"},
	{Name: "k8s.tolerations[0].value", DataType: "string", ChineseName: "容忍VALUE", GroupName: "污点容忍", SortOrder: 32, Category: "k8s"},
	{Name: "k8s.tolerations[0].effect", DataType: "string", ChineseName: "容忍结果", ValidationRule: "^(NoSchedule|PreferNoSchedule|NoExecute)$", Description: "为空表示匹配所有污点结果", GroupName: "污点容忍", SortOrder: 33, Category: "k8s"},
	{Name: "k8s.tolerations[1].key", DataType: "string", ChineseName: "容忍KEY", GroupName: "污点容忍", SortOrder: 34, Category: "k8s"},
	{Name: "k8s.tolerations[1].operator", DataType: "string", ChineseName: "容忍操作", ValidationRule: "^(Exists|Equal)$", GroupName: "污点容忍", SortOrder: 35, Category: "k8s"},
	{Name: "k8s.tolerations[1].value", DataType: "string", ChineseName: "容忍VALUE", GroupName: "污点容忍", SortOrder: 36, Category: "k8s"},
	{Name: "k8s.tolerations[1].effect", DataType: "string", ChineseName: "容忍结果", ValidationRule: "^(NoSchedule|PreferNoSchedule|NoExecute)$", Description: "为空表示匹配所有污点结果", GroupName: "污点容忍", SortOrder: 37, Category: "k8s"},

	// ===== 主机网络 =====
	{Name: "k8s.hostnetwork.enabled", DataType: "bool", ChineseName: "启用主机网络", Description: "是否启用主机网络，默认为false", GroupName: "主机网络", SortOrder: 38, Category: "k8s"},
	{Name: "k8s.hostnetwork.dnsPolicy", DataType: "string", ChineseName: "DNS策略", ValidationRule: "^(ClusterFirst|ClusterFirstWithHostNet|Default|None)$", Description: "默认为ClusterFirst", GroupName: "主机网络", SortOrder: 39, Category: "k8s"},
	{Name: "k8s.terminationGracePeriodSeconds", DataType: "string", ChineseName: "优停止宽限期", GroupName: "主机网络", SortOrder: 40, Category: "k8s"},

	// ===== 镜像 =====
	{Name: "k8s.imagePullSecrets[0].name", DataType: "string", ChineseName: "镜像拉取密钥", Description: "多个表示引用多个secret", GroupName: "镜像", SortOrder: 41, Category: "k8s"},
	{Name: "k8s.imagePullSecrets[1].name", DataType: "string", ChineseName: "镜像拉取密钥", Description: "多个表示引用多个secret", GroupName: "镜像", SortOrder: 42, Category: "k8s"},
	{Name: "k8s.image.repository", DataType: "string", ChineseName: "镜像地址", Description: "配置中心无需配置，在部署时拼接", GroupName: "镜像", SortOrder: 43, Category: "k8s"},
	{Name: "k8s.image.tag", DataType: "string", ChineseName: "镜像Tag", Description: "配置中心无需配置，在部署时拼接", GroupName: "镜像", SortOrder: 44, Category: "k8s"},
	{Name: "k8s.image.pullPolicy", DataType: "string", ChineseName: "镜像拉取策略", ValidationRule: "^(IfNotPresent|Always|Never)$", Description: "默认为Always", GroupName: "镜像", SortOrder: 45, Category: "k8s"},

	// ===== 安全上下文 =====
	{Name: "k8s.securityContext.runAsUser", DataType: "string", ChineseName: "容器运行用户", Description: "容器运行用户，建议使用普通用户运行，如65534", GroupName: "安全上下文", SortOrder: 46, Category: "k8s"},
	{Name: "k8s.securityContext.runAsNonRoot", DataType: "bool", ChineseName: "以非root用户运行", GroupName: "安全上下文", SortOrder: 47, Category: "k8s"},

	// ===== 环境变量 =====
	{Name: "k8s.env[0].name", DataType: "string", ChineseName: "变量名称", GroupName: "环境变量", SortOrder: 48, Category: "k8s"},
	{Name: "k8s.env[0].value", DataType: "string", ChineseName: "变量值", GroupName: "环境变量", SortOrder: 49, Category: "k8s"},
	{Name: "k8s.env[1].name", DataType: "string", ChineseName: "变量名称", GroupName: "环境变量", SortOrder: 50, Category: "k8s"},
	{Name: "k8s.env[1].value", DataType: "string", ChineseName: "变量值", GroupName: "环境变量", SortOrder: 51, Category: "k8s"},
	{Name: "k8s.env[2].name", DataType: "string", ChineseName: "变量名称", GroupName: "环境变量", SortOrder: 52, Category: "k8s"},
	{Name: "k8s.env[2].value", DataType: "string", ChineseName: "变量值", GroupName: "环境变量", SortOrder: 53, Category: "k8s"},
	{Name: "k8s.env[3].name", DataType: "string", ChineseName: "变量名称", GroupName: "环境变量", SortOrder: 54, Category: "k8s"},
	{Name: "k8s.env[3].value", DataType: "string", ChineseName: "变量值", GroupName: "环境变量", SortOrder: 55, Category: "k8s"},
	{Name: "k8s.env[4].name", DataType: "string", ChineseName: "变量名称", GroupName: "环境变量", SortOrder: 56, Category: "k8s"},
	{Name: "k8s.env[4].value", DataType: "string", ChineseName: "变量值", GroupName: "环境变量", SortOrder: 57, Category: "k8s"},
	{Name: "k8s.env[5].name", DataType: "string", ChineseName: "变量名称", GroupName: "环境变量", SortOrder: 58, Category: "k8s"},
	{Name: "k8s.env[5].value", DataType: "string", ChineseName: "变量值", GroupName: "环境变量", SortOrder: 59, Category: "k8s"},
	{Name: "k8s.env[6].name", DataType: "string", ChineseName: "变量名称", GroupName: "环境变量", SortOrder: 60, Category: "k8s"},
	{Name: "k8s.env[6].value", DataType: "string", ChineseName: "变量值", GroupName: "环境变量", SortOrder: 61, Category: "k8s"},
	{Name: "k8s.env[7].name", DataType: "string", ChineseName: "变量名称", GroupName: "环境变量", SortOrder: 62, Category: "k8s"},
	{Name: "k8s.env[7].value", DataType: "string", ChineseName: "变量值", GroupName: "环境变量", SortOrder: 63, Category: "k8s"},
	{Name: "k8s.env[8].name", DataType: "string", ChineseName: "变量名称", GroupName: "环境变量", SortOrder: 64, Category: "k8s"},
	{Name: "k8s.env[8].value", DataType: "string", ChineseName: "变量值", GroupName: "环境变量", SortOrder: 65, Category: "k8s"},

	// ===== 变量引用 =====
	{Name: "k8s.envFrom.enabled", DataType: "bool", ChineseName: "开启变量引入", Description: "引入configmap或者secret资源对象内容作为变量", GroupName: "变量引用", SortOrder: 66, Category: "k8s"},
	{Name: "k8s.envFrom.envFrom[0].type", DataType: "string", ChineseName: "引入对象类型", ValidationRule: "^(configMapRef|secretRef)$", Description: "引入configmap或者secret资源对象内容作为变量", GroupName: "变量引用", SortOrder: 67, Category: "k8s"},
	{Name: "k8s.envFrom.envFrom[0].name", DataType: "string", ChineseName: "引入对象名称", GroupName: "变量引用", SortOrder: 68, Category: "k8s"},
	{Name: "k8s.envFrom.envFrom[1].type", DataType: "string", ChineseName: "引入对象类型", ValidationRule: "^(configMapRef|secretRef)$", Description: "引入configmap或者secret资源对象内容作为变量", GroupName: "变量引用", SortOrder: 69, Category: "k8s"},
	{Name: "k8s.envFrom.envFrom[1].name", DataType: "string", ChineseName: "引入对象名称", GroupName: "变量引用", SortOrder: 70, Category: "k8s"},

	// ===== 资源限制 =====
	{Name: "k8s.resources.enabled", DataType: "bool", ChineseName: "启用资源限制", Description: "是否开启资源限制，默认为true", GroupName: "资源限制", SortOrder: 71, Category: "k8s"},
	{Name: "k8s.resources.resources.requests.cpu", DataType: "string", ChineseName: "初始CPU资源", ValidationRule: "^[1-9](|\\d)$|^0\\.[1-9]$|^[1-9]\\d{2}m$", Description: "初始CPU资源，调度时满足此值即可调度", GroupName: "资源限制", SortOrder: 72, Category: "k8s"},
	{Name: "k8s.resources.resources.requests.memory", DataType: "string", ChineseName: "初始MEM资源", ValidationRule: "^[1-9]\\d{0,2}(|\\.\\d)Gi$|^[1-9]\\d{2,5}Mi$", Description: "初始MEM资源，调度时满足此值即可调度", GroupName: "资源限制", SortOrder: 73, Category: "k8s"},
	{Name: "k8s.resources.resources.requests.ephemeral-storage", DataType: "string", ChineseName: "初始本地临时性存储", ValidationRule: "^[1-9]\\d{0,2}(|\\.\\d)Gi$|^[1-9]\\d{2,5}Mi$", GroupName: "资源限制", SortOrder: 74, Category: "k8s"},
	{Name: "k8s.resources.resources.requests.'nvidia\\.com/gpu'", DataType: "string", ChineseName: "使用显卡数", GroupName: "资源限制", SortOrder: 75, Category: "k8s"},
	{Name: "k8s.resources.resources.limits.cpu", DataType: "string", ChineseName: "最大CPU资源", ValidationRule: "^[1-9](|\\d)$|^0\\.[1-9]$|^[1-9]\\d{2}m$", Description: "最大CPU资源", GroupName: "资源限制", SortOrder: 76, Category: "k8s"},
	{Name: "k8s.resources.resources.limits.memory", DataType: "string", ChineseName: "最大MEM资源", ValidationRule: "^[1-9]\\d{0,2}(|\\.\\d)Gi$|^[1-9]\\d{2,5}Mi$", Description: "最大MEM资源，不可超过此值，超过会出现OOM", GroupName: "资源限制", SortOrder: 77, Category: "k8s"},
	{Name: "k8s.resources.resources.limits.ephemeral-storage", DataType: "string", ChineseName: "最大本地临时性存储", ValidationRule: "^[1-9]\\d{0,2}(|\\.\\d)Gi$|^[1-9]\\d{2,5}Mi$", GroupName: "资源限制", SortOrder: 78, Category: "k8s"},
	{Name: "k8s.resources.resources.limits.'nvidia\\.com/gpu'", DataType: "string", ChineseName: "使用显卡数", GroupName: "资源限制", SortOrder: 79, Category: "k8s"},

	// ===== 启动检查 =====
	{Name: "k8s.startupProbe.enabled", DataType: "bool", ChineseName: "开启启动检查", Description: "默认为true", GroupName: "启动检查", SortOrder: 80, Category: "probe"},
	{Name: "k8s.startupProbe.type", DataType: "string", ChineseName: "启动检查类型", ValidationRule: "^(httpGet|tcpSocket)$", Description: "默认为httpGet", GroupName: "启动检查", SortOrder: 81, Category: "probe"},
	{Name: "k8s.startupProbe.tcpSocket.port", DataType: "string", ChineseName: "启动检查tcpSocket端口", Description: "默认为8080", GroupName: "启动检查", SortOrder: 82, Category: "probe"},
	{Name: "k8s.startupProbe.httpGet.path", DataType: "string", ChineseName: "启动检查路径", Description: "默认为/actuator/health", GroupName: "启动检查", SortOrder: 83, Category: "probe"},
	{Name: "k8s.startupProbe.httpGet.port", DataType: "string", ChineseName: "启动检查httpGet端口", Description: "默认为8080", GroupName: "启动检查", SortOrder: 84, Category: "probe"},
	{Name: "k8s.startupProbe.initialDelaySeconds", DataType: "uint", ChineseName: "启动检查首次探测时间", Description: "默认为45", GroupName: "启动检查", SortOrder: 85, Category: "probe"},
	{Name: "k8s.startupProbe.timeoutSeconds", DataType: "uint", ChineseName: "启动检查响应超时时间", Description: "默认为5", GroupName: "启动检查", SortOrder: 86, Category: "probe"},
	{Name: "k8s.startupProbe.periodSeconds", DataType: "uint", ChineseName: "启动检查定期探测时间", Description: "默认为10", GroupName: "启动检查", SortOrder: 87, Category: "probe"},
	{Name: "k8s.startupProbe.successThreshold", DataType: "uint", ChineseName: "失败后检查最小成功次数", Description: "默认为1", GroupName: "启动检查", SortOrder: 88, Category: "probe"},
	{Name: "k8s.startupProbe.failureThreshold", DataType: "uint", ChineseName: "启动成功后检查失败次数", Description: "默认为3", GroupName: "启动检查", SortOrder: 89, Category: "probe"},

	// ===== 健康检查 =====
	{Name: "k8s.livenessProbe.enabled", DataType: "bool", ChineseName: "开启健康检查", Description: "默认为true", GroupName: "健康检查", SortOrder: 90, Category: "probe"},
	{Name: "k8s.livenessProbe.type", DataType: "string", ChineseName: "健康检查类型", ValidationRule: "^(httpGet|tcpSocket)$", Description: "默认为httpGet", GroupName: "健康检查", SortOrder: 91, Category: "probe"},
	{Name: "k8s.livenessProbe.tcpSocket.port", DataType: "string", ChineseName: "健康检查tcpSocket端口", Description: "默认为8080", GroupName: "健康检查", SortOrder: 92, Category: "probe"},
	{Name: "k8s.livenessProbe.httpGet.path", DataType: "string", ChineseName: "健康检查路径", Description: "默认为/actuator/health/liveness", GroupName: "健康检查", SortOrder: 93, Category: "probe"},
	{Name: "k8s.livenessProbe.httpGet.port", DataType: "string", ChineseName: "健康检查httpGet端口", Description: "默认为8080", GroupName: "健康检查", SortOrder: 94, Category: "probe"},
	{Name: "k8s.livenessProbe.initialDelaySeconds", DataType: "uint", ChineseName: "健康检查首次探测时间", Description: "默认为30", GroupName: "健康检查", SortOrder: 95, Category: "probe"},
	{Name: "k8s.livenessProbe.timeoutSeconds", DataType: "uint", ChineseName: "健康检查响应超时时间", Description: "默认为5", GroupName: "健康检查", SortOrder: 96, Category: "probe"},
	{Name: "k8s.livenessProbe.periodSeconds", DataType: "uint", ChineseName: "健康检查定期探测时间", Description: "默认为10", GroupName: "健康检查", SortOrder: 97, Category: "probe"},
	{Name: "k8s.livenessProbe.successThreshold", DataType: "uint", ChineseName: "失败后检查最小成功次数", Description: "默认为1", GroupName: "健康检查", SortOrder: 98, Category: "probe"},
	{Name: "k8s.livenessProbe.failureThreshold", DataType: "uint", ChineseName: "启动成功后检查失败次数", Description: "默认为3", GroupName: "健康检查", SortOrder: 99, Category: "probe"},

	// ===== 就绪检查 =====
	{Name: "k8s.readinessProbe.enabled", DataType: "bool", ChineseName: "开启就绪检查", Description: "默认为true", GroupName: "就绪检查", SortOrder: 100, Category: "probe"},
	{Name: "k8s.readinessProbe.type", DataType: "string", ChineseName: "就绪检查类型", ValidationRule: "^(httpGet|tcpSocket)$", Description: "默认为httpGet", GroupName: "就绪检查", SortOrder: 101, Category: "probe"},
	{Name: "k8s.readinessProbe.tcpSocket.port", DataType: "string", ChineseName: "就绪检查tcpSocket端口", Description: "默认为8080", GroupName: "就绪检查", SortOrder: 102, Category: "probe"},
	{Name: "k8s.readinessProbe.httpGet.path", DataType: "string", ChineseName: "就绪检查路径", Description: "默认为/actuator/health/readiness", GroupName: "就绪检查", SortOrder: 103, Category: "probe"},
	{Name: "k8s.readinessProbe.httpGet.port", DataType: "string", ChineseName: "就绪检查httpGet端口", Description: "默认为8080", GroupName: "就绪检查", SortOrder: 104, Category: "probe"},
	{Name: "k8s.readinessProbe.initialDelaySeconds", DataType: "uint", ChineseName: "就绪检查首次探测时间", Description: "默认为30", GroupName: "就绪检查", SortOrder: 105, Category: "probe"},
	{Name: "k8s.readinessProbe.timeoutSeconds", DataType: "uint", ChineseName: "就绪检查响应超时时间", Description: "默认为5", GroupName: "就绪检查", SortOrder: 106, Category: "probe"},
	{Name: "k8s.readinessProbe.periodSeconds", DataType: "uint", ChineseName: "就绪检查定期探测时间", Description: "默认为10", GroupName: "就绪检查", SortOrder: 107, Category: "probe"},
	{Name: "k8s.readinessProbe.successThreshold", DataType: "uint", ChineseName: "失败后检查最小成功次数", Description: "默认为1", GroupName: "就绪检查", SortOrder: 108, Category: "probe"},
	{Name: "k8s.readinessProbe.failureThreshold", DataType: "uint", ChineseName: "启动成功后检查失败次数", Description: "默认为3", GroupName: "就绪检查", SortOrder: 109, Category: "probe"},

	// ===== 服务 =====
	{Name: "k8s.service.enabled", DataType: "bool", ChineseName: "开启服务", GroupName: "服务", SortOrder: 110, Category: "network"},
	{Name: "k8s.service.headless", DataType: "bool", ChineseName: "使用无头服务", ValidationRule: "^(true|false)$", Description: "默认为false", GroupName: "服务", SortOrder: 111, Category: "network"},
	{Name: "k8s.service.type", DataType: "string", ChineseName: "服务类型", ValidationRule: "^(ClusterIP|NodePort|LoadBalancer)$", Description: "负载均衡均衡方式目前仅支持公有云场景,默认为ClusterIP", GroupName: "服务", SortOrder: 112, Category: "network"},
	{Name: "k8s.service.annotations.'service\\.beta\\.kubernetes\\.io/alibaba-cloud-loadbalancer-address-type'", DataType: "string", ChineseName: "负载均衡类型", ValidationRule: "^(internet|intranet)$", Description: "内网LB或外网LB", GroupName: "服务", SortOrder: 113, Category: "network"},
	{Name: "k8s.service.annotations.'service\\.beta\\.kubernetes\\.io/alibaba-cloud-loadbalancer-force-override-listeners'", DataType: "string", ChineseName: "是否覆盖已存在监听", ValidationRule: "^(true|false)$", Description: "true或false", GroupName: "服务", SortOrder: 114, Category: "network"},
	{Name: "k8s.service.annotations.'service\\.beta\\.kubernetes\\.io/alibaba-cloud-loadbalancer-id'", DataType: "string", ChineseName: "负载均衡实例ID", GroupName: "服务", SortOrder: 115, Category: "network"},
	{Name: "k8s.service.ports[0].name", DataType: "string", ChineseName: "服务端口名称", GroupName: "服务", SortOrder: 116, Category: "network"},
	{Name: "k8s.service.ports[0].port", DataType: "uint", ChineseName: "服务端口号", GroupName: "服务", SortOrder: 117, Category: "network"},
	{Name: "k8s.service.ports[0].protocol", DataType: "string", ChineseName: "服务端口协议", ValidationRule: "^(TCP|UDP|SCTP)$", Description: "默认为TCP", GroupName: "服务", SortOrder: 118, Category: "network"},
	{Name: "k8s.service.ports[0].nodeport", DataType: "string", ChineseName: "服务节点端口", GroupName: "服务", SortOrder: 119, Category: "network"},
	{Name: "k8s.service.ports[1].name", DataType: "string", ChineseName: "服务端口名称", GroupName: "服务", SortOrder: 120, Category: "network"},
	{Name: "k8s.service.ports[1].port", DataType: "uint", ChineseName: "服务端口号", GroupName: "服务", SortOrder: 121, Category: "network"},
	{Name: "k8s.service.ports[1].protocol", DataType: "string", ChineseName: "服务端口协议", ValidationRule: "^(TCP|UDP|SCTP)$", Description: "默认为TCP", GroupName: "服务", SortOrder: 122, Category: "network"},
	{Name: "k8s.service.ports[1].nodeport", DataType: "string", ChineseName: "服务节点端口", GroupName: "服务", SortOrder: 123, Category: "network"},

	// ===== 域名 =====
	{Name: "k8s.ingress.enabled", DataType: "bool", ChineseName: "开启域名配置", GroupName: "域名", SortOrder: 124, Category: "network"},
	{Name: "k8s.ingress.className", DataType: "string", ChineseName: "Ingress类名称", ValidationRule: "^(nginx|kong)$", Description: "默认为nginx", GroupName: "域名", SortOrder: 125, Category: "network"},
	{Name: "k8s.ingress.annotations.'nginx\\.ingress\\.kubernetes\\.io/ssl-redirect'", DataType: "string", ChineseName: "启用HTTP跳转HTTPS", ValidationRule: "^(true|false)$", GroupName: "域名", SortOrder: 126, Category: "network"},
	{Name: "k8s.ingress.annotations.'nginx\\.org/websocket-services'", DataType: "string", ChineseName: "开启WEBSOCKET", ValidationRule: "^(true|false)$", GroupName: "域名", SortOrder: 127, Category: "network"},
	{Name: "k8s.ingress.annotations.'nginx\\.ingress\\.kubernetes\\.io/auth-tls-verify-client'", DataType: "string", ChineseName: "开启客户端验证", ValidationRule: "^(on|off)$", GroupName: "域名", SortOrder: 128, Category: "network"},
	{Name: "k8s.ingress.annotations.'nginx\\.ingress\\.kubernetes\\.io/auth-tls-verify-depth'", DataType: "string", ChineseName: "验证证书链的深度", GroupName: "域名", SortOrder: 129, Category: "network"},
	{Name: "k8s.ingress.annotations.'nginx\\.ingress\\.kubernetes\\.io/auth-tls-secret'", DataType: "string", ChineseName: "指定CA证书Secret", GroupName: "域名", SortOrder: 130, Category: "network"},
	{Name: "k8s.ingress.annotations.'nginx\\.ingress\\.kubernetes\\.io/auth-tls-pass-certificate-to-upstream'", DataType: "string", ChineseName: "是否将证书传递给后端服务", ValidationRule: "^(true|false)$", GroupName: "域名", SortOrder: 131, Category: "network"},
	{Name: "k8s.ingress.annotations.'nginx\\.ingress\\.kubernetes\\.io/proxy-body-size'", DataType: "string", ChineseName: "修改bodysize", GroupName: "域名", SortOrder: 132, Category: "network"},
	{Name: "k8s.ingress.hosts[0].host", DataType: "string", ChineseName: "Ingress域名", GroupName: "域名", SortOrder: 133, Category: "network"},
	{Name: "k8s.ingress.hosts[0].paths[0].path", DataType: "string", ChineseName: "Ingress路径", GroupName: "域名", SortOrder: 134, Category: "network"},
	{Name: "k8s.ingress.hosts[0].paths[0].pathType", DataType: "string", ChineseName: "Ingress路径类型", ValidationRule: "^(Prefix|ImplementationSpecific)$", Description: "默认为Prefix", GroupName: "域名", SortOrder: 135, Category: "network"},
	{Name: "k8s.ingress.hosts[0].paths[0].servicePort", DataType: "uint", ChineseName: "Ingress映射Service端口", GroupName: "域名", SortOrder: 136, Category: "network"},
	{Name: "k8s.ingress.hosts[0].paths[1].path", DataType: "string", ChineseName: "Ingress路径", GroupName: "域名", SortOrder: 137, Category: "network"},
	{Name: "k8s.ingress.hosts[0].paths[1].pathType", DataType: "string", ChineseName: "Ingress路径类型", ValidationRule: "^(Prefix|ImplementationSpecific)$", Description: "默认为Prefix", GroupName: "域名", SortOrder: 138, Category: "network"},
	{Name: "k8s.ingress.hosts[0].paths[1].servicePort", DataType: "uint", ChineseName: "Ingress映射Service端口", GroupName: "域名", SortOrder: 139, Category: "network"},
	{Name: "k8s.ingress.hosts[0].paths[2].path", DataType: "string", ChineseName: "Ingress路径", GroupName: "域名", SortOrder: 140, Category: "network"},
	{Name: "k8s.ingress.hosts[0].paths[2].pathType", DataType: "string", ChineseName: "Ingress路径类型", ValidationRule: "^(Prefix|ImplementationSpecific)$", Description: "默认为Prefix", GroupName: "域名", SortOrder: 141, Category: "network"},
	{Name: "k8s.ingress.hosts[0].paths[2].servicePort", DataType: "uint", ChineseName: "Ingress映射Service端口", GroupName: "域名", SortOrder: 142, Category: "network"},
	{Name: "k8s.ingress.hosts[1].host", DataType: "string", ChineseName: "Ingress域名", GroupName: "域名", SortOrder: 143, Category: "network"},
	{Name: "k8s.ingress.hosts[1].paths[0].path", DataType: "string", ChineseName: "Ingress路径", GroupName: "域名", SortOrder: 144, Category: "network"},
	{Name: "k8s.ingress.hosts[1].paths[0].pathType", DataType: "string", ChineseName: "Ingress路径类型", ValidationRule: "^(Prefix|ImplementationSpecific)$", Description: "默认为Prefix", GroupName: "域名", SortOrder: 145, Category: "network"},
	{Name: "k8s.ingress.hosts[1].paths[0].servicePort", DataType: "uint", ChineseName: "Ingress映射Service端口", GroupName: "域名", SortOrder: 146, Category: "network"},
	{Name: "k8s.ingress.hosts[1].paths[1].path", DataType: "string", ChineseName: "Ingress路径", GroupName: "域名", SortOrder: 147, Category: "network"},
	{Name: "k8s.ingress.hosts[1].paths[1].pathType", DataType: "string", ChineseName: "Ingress路径类型", ValidationRule: "^(Prefix|ImplementationSpecific)$", Description: "默认为Prefix", GroupName: "域名", SortOrder: 148, Category: "network"},
	{Name: "k8s.ingress.hosts[1].paths[1].servicePort", DataType: "uint", ChineseName: "Ingress映射Service端口", GroupName: "域名", SortOrder: 149, Category: "network"},
	{Name: "k8s.ingress.hosts[1].paths[2].path", DataType: "string", ChineseName: "Ingress路径", GroupName: "域名", SortOrder: 150, Category: "network"},
	{Name: "k8s.ingress.hosts[1].paths[2].pathType", DataType: "string", ChineseName: "Ingress路径类型", ValidationRule: "^(Prefix|ImplementationSpecific)$", Description: "默认为Prefix", GroupName: "域名", SortOrder: 151, Category: "network"},
	{Name: "k8s.ingress.hosts[1].paths[2].servicePort", DataType: "uint", ChineseName: "Ingress映射Service端口", GroupName: "域名", SortOrder: 152, Category: "network"},
	{Name: "k8s.ingress.hosts[2].host", DataType: "string", ChineseName: "Ingress域名", GroupName: "域名", SortOrder: 153, Category: "network"},
	{Name: "k8s.ingress.hosts[2].paths[0].path", DataType: "string", ChineseName: "Ingress路径", GroupName: "域名", SortOrder: 154, Category: "network"},
	{Name: "k8s.ingress.hosts[2].paths[0].pathType", DataType: "string", ChineseName: "Ingress路径类型", ValidationRule: "^(Prefix|ImplementationSpecific)$", Description: "默认为Prefix", GroupName: "域名", SortOrder: 155, Category: "network"},
	{Name: "k8s.ingress.hosts[2].paths[0].servicePort", DataType: "uint", ChineseName: "Ingress映射Service端口", GroupName: "域名", SortOrder: 156, Category: "network"},
	{Name: "k8s.ingress.hosts[2].paths[1].path", DataType: "string", ChineseName: "Ingress路径", GroupName: "域名", SortOrder: 157, Category: "network"},
	{Name: "k8s.ingress.hosts[2].paths[1].pathType", DataType: "string", ChineseName: "Ingress路径类型", ValidationRule: "^(Prefix|ImplementationSpecific)$", Description: "默认为Prefix", GroupName: "域名", SortOrder: 158, Category: "network"},
	{Name: "k8s.ingress.hosts[2].paths[1].servicePort", DataType: "uint", ChineseName: "Ingress映射Service端口", GroupName: "域名", SortOrder: 159, Category: "network"},
	{Name: "k8s.ingress.hosts[2].paths[2].path", DataType: "string", ChineseName: "Ingress路径", GroupName: "域名", SortOrder: 160, Category: "network"},
	{Name: "k8s.ingress.hosts[2].paths[2].pathType", DataType: "string", ChineseName: "Ingress路径类型", ValidationRule: "^(Prefix|ImplementationSpecific)$", Description: "默认为Prefix", GroupName: "域名", SortOrder: 161, Category: "network"},
	{Name: "k8s.ingress.hosts[2].paths[2].servicePort", DataType: "uint", ChineseName: "Ingress映射Service端口", GroupName: "域名", SortOrder: 162, Category: "network"},
	{Name: "k8s.ingress.tls[0].secretName", DataType: "string", ChineseName: "域名证书Secret名称", GroupName: "域名", SortOrder: 163, Category: "network"},
	{Name: "k8s.ingress.tls[0].hosts[0]", DataType: "string", ChineseName: "启用TLS的域名", GroupName: "域名", SortOrder: 164, Category: "network"},
	{Name: "k8s.ingress.tls[0].hosts[1]", DataType: "string", ChineseName: "启用TLS的域名", GroupName: "域名", SortOrder: 165, Category: "network"},
	{Name: "k8s.ingress.tls[1].secretName", DataType: "string", ChineseName: "域名证书Secret名称", GroupName: "域名", SortOrder: 166, Category: "network"},
	{Name: "k8s.ingress.tls[1].hosts[0]", DataType: "string", ChineseName: "启用TLS的域名", GroupName: "域名", SortOrder: 167, Category: "network"},
	{Name: "k8s.ingress.tls[1].hosts[1]", DataType: "string", ChineseName: "启用TLS的域名", GroupName: "域名", SortOrder: 168, Category: "network"},

	// ===== 自动扩缩容 =====
	{Name: "k8s.autoscaling.enabled", DataType: "bool", ChineseName: "开启自动扩缩容", GroupName: "自动扩缩容", SortOrder: 169, Category: "autoscaling"},
	{Name: "k8s.autoscaling.minReplicas", DataType: "uint", ChineseName: "最小副本数", GroupName: "自动扩缩容", SortOrder: 170, Category: "autoscaling"},
	{Name: "k8s.autoscaling.maxReplicas", DataType: "uint", ChineseName: "最大副本数", GroupName: "自动扩缩容", SortOrder: 171, Category: "autoscaling"},
	{Name: "k8s.autoscaling.targetCPUUtilizationPercentage", DataType: "uint", ChineseName: "CPU百分比", GroupName: "自动扩缩容", SortOrder: 172, Category: "autoscaling"},
	{Name: "k8s.autoscaling.targetMemoryUtilizationPercentage", DataType: "uint", ChineseName: "内存百分比", GroupName: "自动扩缩容", SortOrder: 173, Category: "autoscaling"},

	// ===== 监控 =====
	{Name: "k8s.serviceMonitor.enabled", DataType: "bool", ChineseName: "开启监控接入", GroupName: "监控", SortOrder: 174, Category: "monitor"},
	{Name: "k8s.serviceMonitor.namespace", DataType: "string", ChineseName: "监控服务命名空间", Description: "默认为monitoring", GroupName: "监控", SortOrder: 175, Category: "monitor"},
	{Name: "k8s.serviceMonitor.interval", DataType: "string", ChineseName: "采集指标间隔", Description: "默认为30s", GroupName: "监控", SortOrder: 176, Category: "monitor"},
	{Name: "k8s.serviceMonitor.scrapeTimeout", DataType: "string", ChineseName: "采集指标超时时间", Description: "默认为10s", GroupName: "监控", SortOrder: 177, Category: "monitor"},
	{Name: "k8s.serviceMonitor.path", DataType: "string", ChineseName: "采集指标路径", Description: "默认为/actuator/prometheus", GroupName: "监控", SortOrder: 178, Category: "monitor"},

	// ===== 卷挂载 =====
	{Name: "k8s.volumes.enabled", DataType: "bool", ChineseName: "开启卷挂载", GroupName: "卷挂载", SortOrder: 179, Category: "storage"},
	{Name: "k8s.volumes.volumes[0].name", DataType: "string", ChineseName: "挂载卷名称", Description: "根据实际类型配置对应挂载参数", GroupName: "卷挂载", SortOrder: 180, Category: "storage"},
	{Name: "k8s.volumes.volumes[0].type", DataType: "string", ChineseName: "挂载卷类型", ValidationRule: "^(hostPath|nfs|configMap|emptyDir|persistentVolumeClaim|secret)$", Description: "根据实际类型配置对应挂载参数", GroupName: "卷挂载", SortOrder: 181, Category: "storage"},
	{Name: "k8s.volumes.volumes[0].parameter.path", DataType: "string", ChineseName: "挂载卷路径", Description: "根据实际类型配置对应挂载参数", GroupName: "卷挂载", SortOrder: 182, Category: "storage"},
	{Name: "k8s.volumes.volumes[0].parameter.server", DataType: "string", ChineseName: "挂载卷NFS服务", Description: "根据实际类型配置对应挂载参数", GroupName: "卷挂载", SortOrder: 183, Category: "storage"},
	{Name: "k8s.volumes.volumes[0].parameter.name", DataType: "string", ChineseName: "挂载对象名称", GroupName: "卷挂载", SortOrder: 184, Category: "storage"},
	{Name: "k8s.volumes.volumes[0].mountPath", DataType: "string", ChineseName: "挂载到容器路径", GroupName: "卷挂载", SortOrder: 185, Category: "storage"},
	{Name: "k8s.volumes.volumes[1].name", DataType: "string", ChineseName: "挂载卷名称", Description: "根据实际类型配置对应挂载参数", GroupName: "卷挂载", SortOrder: 186, Category: "storage"},
	{Name: "k8s.volumes.volumes[1].type", DataType: "string", ChineseName: "挂载卷类型", ValidationRule: "^(hostPath|nfs|configMap|emptyDir|persistentVolumeClaim|secret)$", Description: "根据实际类型配置对应挂载参数", GroupName: "卷挂载", SortOrder: 187, Category: "storage"},
	{Name: "k8s.volumes.volumes[1].parameter.server", DataType: "string", ChineseName: "挂载卷NFS服务", Description: "根据实际类型配置对应挂载参数", GroupName: "卷挂载", SortOrder: 188, Category: "storage"},
	{Name: "k8s.volumes.volumes[1].parameter.path", DataType: "string", ChineseName: "挂载卷路径", Description: "根据实际类型配置对应挂载参数", GroupName: "卷挂载", SortOrder: 189, Category: "storage"},
	{Name: "k8s.volumes.volumes[1].parameter.name", DataType: "string", ChineseName: "挂载对象名称", GroupName: "卷挂载", SortOrder: 190, Category: "storage"},
	{Name: "k8s.volumes.volumes[1].mountPath", DataType: "string", ChineseName: "挂载到容器路径", GroupName: "卷挂载", SortOrder: 191, Category: "storage"},

	// ===== 定时任务 =====
	{Name: "k8s.cronjob.enabled", DataType: "bool", ChineseName: "开启定时任务", GroupName: "定时任务", SortOrder: 192, Category: "cronjob"},
	{Name: "k8s.cronjob.restartPolicy", DataType: "string", ChineseName: "重启策略", ValidationRule: "^(OnFailure|Never)$", Description: "默认为OnFailure", GroupName: "定时任务", SortOrder: 193, Category: "cronjob"},
	{Name: "k8s.cronjob.concurrencyPolicy", DataType: "string", ChineseName: "并发策略", ValidationRule: "^(Allow|Forbid|Replace)$", Description: "默认为Allow", GroupName: "定时任务", SortOrder: 194, Category: "cronjob"},
	{Name: "k8s.cronjob.failedJobsHistoryLimit", DataType: "uint", ChineseName: "保留失败作业数", Description: "默认为1", GroupName: "定时任务", SortOrder: 195, Category: "cronjob"},
	{Name: "k8s.cronjob.successfulJobsHistoryLimit", DataType: "uint", ChineseName: "保留成功完成作业数", Description: "默认为3", GroupName: "定时任务", SortOrder: 196, Category: "cronjob"},
	{Name: "k8s.cronjob.schedule", DataType: "string", ChineseName: "定时任务执行策略", GroupName: "定时任务", SortOrder: 197, Category: "cronjob"},
	{Name: "k8s.cronjob.command", DataType: "string", ChineseName: "定时任务执行命令", GroupName: "定时任务", SortOrder: 198, Category: "cronjob"},

	// ===== 普通任务 =====
	{Name: "k8s.job.enabled", DataType: "bool", ChineseName: "开启普通任务", GroupName: "普通任务", SortOrder: 199, Category: "cronjob"},
	{Name: "k8s.job.restartPolicy", DataType: "string", ChineseName: "重启策略", ValidationRule: "^(OnFailure|Never)$", Description: "默认为OnFailure", GroupName: "普通任务", SortOrder: 200, Category: "cronjob"},
	{Name: "k8s.job.backoffLimit", DataType: "uint", ChineseName: "失败之前重试次数", Description: "默认为6", GroupName: "普通任务", SortOrder: 201, Category: "cronjob"},
	{Name: "k8s.job.parallelism", DataType: "uint", ChineseName: "作业运行最大Pod数", Description: "默认为1", GroupName: "普通任务", SortOrder: 202, Category: "cronjob"},

	// ===== 其他 =====
	{Name: "verify_enabled", DataType: "bool", ChineseName: "开启服务验证", GroupName: "服务验证", SortOrder: 203, Category: "other"},
	{Name: "verify_timeout", DataType: "uint", ChineseName: "服务验证超时时间", GroupName: "服务验证", SortOrder: 204, Category: "other"},
	{Name: "notice_enabled", DataType: "bool", ChineseName: "开启部署通知", GroupName: "通知", SortOrder: 205, Category: "other"},
	{Name: "webhook_url", DataType: "string", ChineseName: "webhook地址", ValidationRule: "^((https)?:\\/\\/)[^\\s]+", GroupName: "通知", SortOrder: 206, Category: "other"},
	{Name: "webhook_type", DataType: "string", ChineseName: "webhook类型", ValidationRule: "^(wechat|feishu)$", GroupName: "通知", SortOrder: 207, Category: "other"},
	{Name: "webhook_token", DataType: "string", ChineseName: "webhook密钥", GroupName: "通知", SortOrder: 208, Category: "other"},
	{Name: "clean_workspace", DataType: "bool", ChineseName: "清理工作空间", GroupName: "其他", SortOrder: 209, Category: "other"},

	// Helm 发布（发版中心使用：定位 chart 与 release 元信息，与 k8s.* values 配合）
	{Name: "helm.chart_name", DataType: "string", ChineseName: "Helm Chart名称", Description: "Chart 名称（仓库内名称或本地路径）", GroupName: "Helm发布", SortOrder: 210, Category: "helm"},
	{Name: "helm.chart_repo", DataType: "string", ChineseName: "Helm仓库地址", Description: "Chart 仓库 URL，留空表示使用本地 chart 路径", GroupName: "Helm发布", SortOrder: 211, Category: "helm"},
	{Name: "helm.chart_version", DataType: "string", ChineseName: "Chart版本", Description: "留空表示使用仓库最新版本", GroupName: "Helm发布", SortOrder: 212, Category: "helm"},
	{Name: "helm.release_name", DataType: "string", ChineseName: "Release名称", Description: "留空默认使用应用名称", GroupName: "Helm发布", SortOrder: 213, Category: "helm"},
}

// helmDeployParamNames 发版中心专用的 helm 元信息参数名（按前缀分流时排除出 chart values）
var helmDeployParamNames = []string{
	"helm.chart_name",
	"helm.chart_repo",
	"helm.chart_version",
	"helm.release_name",
}

// SeedDeployParams 初始化部署参数数据，启动时自动调用
func SeedDeployParams(db *gorm.DB) {
	if db == nil {
		return
	}
	count := int64(0)
	db.Model(&DeployParam{}).Count(&count)
	if count > 0 {
		return // 已有数据，跳过
	}
	logger.Infof("Seeding %d deploy params...", len(deployParamSeeds))
	for _, s := range deployParamSeeds {
		p := DeployParam{
			Name:           s.Name,
			DataType:       s.DataType,
			ChineseName:    s.ChineseName,
			ValidationRule: s.ValidationRule,
			Description:    s.Description,
			GroupName:      s.GroupName,
			SortOrder:      s.SortOrder,
			Category:       s.Category,
		}
		if err := db.Where("name = ?", s.Name).FirstOrCreate(&p).Error; err != nil {
			logger.Warnf("Failed to seed deploy param %s: %v", s.Name, err)
		}
	}
	logger.Infof("Deploy params seeded successfully")
}

// SeedHelmDeployParams 幂等补种 helm 分类参数。
// SeedDeployParams 在表非空时会整体跳过，导致已部署环境拿不到后续新增的参数定义；
// 本函数对每个 helm 参数单独 FirstOrCreate，确保老环境也能补齐，启动时调用。
func SeedHelmDeployParams(db *gorm.DB) {
	if db == nil {
		return
	}
	helmNames := map[string]bool{}
	for _, n := range helmDeployParamNames {
		helmNames[n] = true
	}
	for _, s := range deployParamSeeds {
		if s.Category != "helm" && !helmNames[s.Name] {
			continue
		}
		p := DeployParam{
			Name:           s.Name,
			DataType:       s.DataType,
			ChineseName:    s.ChineseName,
			ValidationRule: s.ValidationRule,
			Description:    s.Description,
			GroupName:      s.GroupName,
			SortOrder:      s.SortOrder,
			Category:       s.Category,
		}
		if err := db.Where("name = ?", s.Name).FirstOrCreate(&p).Error; err != nil {
			logger.Warnf("Failed to seed helm deploy param %s: %v", s.Name, err)
		}
	}
}

// SeedDeployParamMenu 已废弃：发布中心菜单改由 sql/remove_release_center_menu.sql 一次性清理，
// 不再在启动时自动创建 menu-release / menu-deploy-params。
func SeedDeployParamMenu(_ *gorm.DB) {}
