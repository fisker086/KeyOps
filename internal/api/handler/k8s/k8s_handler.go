package k8s

import (
	"net/http"

	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/internal/service"
	k8sService "github.com/fisker086/keyops/internal/service/k8s"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// K8sService defines the interface for K8s operations used by handlers.
type K8sService interface {
	GetBaseInfo(clusterID, clusterName string, nodeID, envID uint, namespace string) (*k8sService.BaseInfo, error)
	GetNodeList(clusterID, clusterName string, nodeID, envID uint) ([]*k8sService.Node, error)
	GetEventList(clusterID, clusterName string, nodeID, envID uint, namespace, objectName, objectKind string) ([]*k8sService.Event, error)
	ScaleReplica(clusterID, clusterName string, nodeID, envID uint, namespace, deploymentName string, desiredReplicas uint) (*k8sService.ReplicaCounts, error)
	GetNamespaceList(clusterID, clusterName string) ([]*k8sService.Namespace, error)
	GetReplica(clusterID, clusterName string, nodeID, envID uint, namespace string) (*k8sService.ReplicaCounts, error)
	GetPodList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Pod, error)
	GetPodDetail(clusterID, clusterName, namespace, podName string) (*k8sService.PodDetail, error)
	GetContainersList(clusterID, clusterName string, nodeID, envID uint, namespace, podName string) ([]*k8sService.Container, error)
	RestartPod(clusterID, clusterName string, nodeID, envID uint, namespace, podName string) error
	GetPodMetrics(clusterID, clusterName, namespace, podName, metricsName string, lastTime, step uint) (interface{}, error)
	DownloadContainerLogs(clusterID, clusterName string, nodeID, envID uint, namespace, podName, container string, limitBytes, sinceSecond int) (string, error)
	GetConfigMapList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.ConfigMap, error)
	GetSecretList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Secret, error)
	GetPVList(clusterID, clusterName string, nodeID, envID uint) ([]*k8sService.PV, error)
	GetStorageClassList(clusterID, clusterName string, nodeID, envID uint) ([]*k8sService.StorageClass, error)
	GetPVCList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.PVC, error)
	GetResourceYaml(clusterID, clusterName, namespace, resourceType, resourceName string) (string, error)
	UpdateResourceYaml(clusterID, clusterName, namespace, resourceType, resourceName, yaml string) error
	DeleteResource(clusterID, clusterName, namespace, resourceType, resourceName string) error
	DryRunResourceYaml(clusterID, clusterName, namespace, resourceType, resourceName, yaml string) (string, error)
	GetServiceList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Service, error)
	GetIngressList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Ingress, error)
	GetHPAList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.HPA, error)
	GetDeploymentList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Deployment, error)
	GetDeploymentDetail(clusterID, clusterName, namespace, deploymentName string) (*k8sService.DeploymentDetail, error)
	GetDeploymentRevisions(clusterID, clusterName, namespace, deploymentName string) ([]*k8sService.DeploymentRevision, int64, error)
	RollbackDeployment(clusterID, clusterName, namespace, deploymentName string, toRevision int64) error
	GetDeploymentMetrics(clusterID, clusterName, namespace, deploymentName string, lastTime, step uint) (interface{}, error)
	GetDaemonSetList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.DaemonSet, error)
	GetDaemonSetDetail(clusterID, clusterName, namespace, daemonSetName string) (*k8sService.DeploymentDetail, error)
	GetDaemonSetMetrics(clusterID, clusterName, namespace, daemonSetName string, lastTime, step uint) (interface{}, error)
	GetDaemonSetRevisions(clusterID, clusterName, namespace, daemonSetName string) ([]*k8sService.DaemonSetRevision, int64, error)
	RollbackDaemonSet(clusterID, clusterName, namespace, daemonSetName string, toRevision int64) error
	GetStatefulSetList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.StatefulSet, error)
	GetStatefulSetDetail(clusterID, clusterName, namespace, statefulSetName string) (*k8sService.DeploymentDetail, error)
	GetStatefulSetMetrics(clusterID, clusterName, namespace, statefulSetName string, lastTime, step uint) (interface{}, error)
	GetStatefulSetRevisions(clusterID, clusterName, namespace, statefulSetName string) ([]*k8sService.StatefulSetRevision, int64, error)
	RollbackStatefulSet(clusterID, clusterName, namespace, statefulSetName string, toRevision int64) error
	GetCronJobList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.CronJob, error)
	GetCronJobDetail(clusterID, clusterName, namespace, cronJobName string) (*k8sService.CronJobDetail, error)
	GetJobList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Job, error)
	GetJobDetail(clusterID, clusterName, namespace, jobName string) (*k8sService.JobDetail, error)
	GetNodeMetricsList(clusterID, clusterName string) ([]k8sService.NodeMetrics, error)
}

var _ K8sService = (*k8sService.K8sService)(nil)

type K8sHandler struct {
	service           K8sService
	permissionService *k8sService.K8sPermissionService
	roleRepo          repository.RoleRepository
	authService       *service.AuthService
}

func NewK8sHandler(service *k8sService.K8sService, permissionService *k8sService.K8sPermissionService, roleRepo repository.RoleRepository, authService *service.AuthService) *K8sHandler {
	return &K8sHandler{
		service:           service,
		permissionService: permissionService,
		roleRepo:          roleRepo,
		authService:       authService,
	}
}
