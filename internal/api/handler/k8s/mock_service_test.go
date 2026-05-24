package k8s

import (
	k8sService "github.com/fisker086/keyops/internal/service/k8s"
)

type mockK8sService struct {
	GetBaseInfoFunc               func(clusterID, clusterName string, nodeID, envID uint, namespace string) (*k8sService.BaseInfo, error)
	GetNodeListFunc               func(clusterID, clusterName string, nodeID, envID uint) ([]*k8sService.Node, error)
	GetEventListFunc              func(clusterID, clusterName string, nodeID, envID uint, namespace, objectName, objectKind string) ([]*k8sService.Event, error)
	ScaleReplicaFunc              func(clusterID, clusterName string, nodeID, envID uint, namespace, deploymentName string, desiredReplicas uint) (*k8sService.ReplicaCounts, error)
	GetNamespaceListFunc          func(clusterID, clusterName string) ([]*k8sService.Namespace, error)
	GetReplicaFunc                func(clusterID, clusterName string, nodeID, envID uint, namespace string) (*k8sService.ReplicaCounts, error)
	GetPodListFunc                func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Pod, error)
	GetPodDetailFunc              func(clusterID, clusterName, namespace, podName string) (*k8sService.PodDetail, error)
	GetContainersListFunc         func(clusterID, clusterName string, nodeID, envID uint, namespace, podName string) ([]*k8sService.Container, error)
	RestartPodFunc                func(clusterID, clusterName string, nodeID, envID uint, namespace, podName string) error
	GetPodMetricsFunc             func(clusterID, clusterName, namespace, podName, metricsName string, lastTime, step uint) (interface{}, error)
	DownloadContainerLogsFunc     func(clusterID, clusterName string, nodeID, envID uint, namespace, podName, container string, limitBytes, sinceSecond int) (string, error)
	GetConfigMapListFunc          func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.ConfigMap, error)
	GetSecretListFunc             func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Secret, error)
	GetPVListFunc                 func(clusterID, clusterName string, nodeID, envID uint) ([]*k8sService.PV, error)
	GetStorageClassListFunc       func(clusterID, clusterName string, nodeID, envID uint) ([]*k8sService.StorageClass, error)
	GetPVCListFunc                func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.PVC, error)
	GetResourceYamlFunc           func(clusterID, clusterName, namespace, resourceType, resourceName string) (string, error)
	UpdateResourceYamlFunc        func(clusterID, clusterName, namespace, resourceType, resourceName, yaml string) error
	DeleteResourceFunc            func(clusterID, clusterName, namespace, resourceType, resourceName string) error
	DryRunResourceYamlFunc        func(clusterID, clusterName, namespace, resourceType, resourceName, yaml string) (string, error)
	GetServiceListFunc            func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Service, error)
	GetIngressListFunc            func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Ingress, error)
	GetHPAListFunc                func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.HPA, error)
	GetDeploymentListFunc         func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Deployment, error)
	GetDeploymentDetailFunc       func(clusterID, clusterName, namespace, deploymentName string) (*k8sService.DeploymentDetail, error)
	GetDeploymentRevisionsFunc    func(clusterID, clusterName, namespace, deploymentName string) ([]*k8sService.DeploymentRevision, int64, error)
	RollbackDeploymentFunc        func(clusterID, clusterName, namespace, deploymentName string, toRevision int64) error
	GetDeploymentMetricsFunc      func(clusterID, clusterName, namespace, deploymentName string, lastTime, step uint) (interface{}, error)
	GetDaemonSetListFunc          func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.DaemonSet, error)
	GetDaemonSetDetailFunc        func(clusterID, clusterName, namespace, daemonSetName string) (*k8sService.DeploymentDetail, error)
	GetDaemonSetMetricsFunc       func(clusterID, clusterName, namespace, daemonSetName string, lastTime, step uint) (interface{}, error)
	GetDaemonSetRevisionsFunc     func(clusterID, clusterName, namespace, daemonSetName string) ([]*k8sService.DaemonSetRevision, int64, error)
	RollbackDaemonSetFunc         func(clusterID, clusterName, namespace, daemonSetName string, toRevision int64) error
	GetStatefulSetListFunc        func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.StatefulSet, error)
	GetStatefulSetDetailFunc      func(clusterID, clusterName, namespace, statefulSetName string) (*k8sService.DeploymentDetail, error)
	GetStatefulSetMetricsFunc     func(clusterID, clusterName, namespace, statefulSetName string, lastTime, step uint) (interface{}, error)
	GetStatefulSetRevisionsFunc   func(clusterID, clusterName, namespace, statefulSetName string) ([]*k8sService.StatefulSetRevision, int64, error)
	RollbackStatefulSetFunc       func(clusterID, clusterName, namespace, statefulSetName string, toRevision int64) error
	GetCronJobListFunc            func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.CronJob, error)
	GetCronJobDetailFunc          func(clusterID, clusterName, namespace, cronJobName string) (*k8sService.CronJobDetail, error)
	GetJobListFunc                func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Job, error)
	GetJobDetailFunc              func(clusterID, clusterName, namespace, jobName string) (*k8sService.JobDetail, error)
	GetNodeMetricsListFunc        func(clusterID, clusterName string) ([]k8sService.NodeMetrics, error)
}

func (m *mockK8sService) GetBaseInfo(clusterID, clusterName string, nodeID, envID uint, namespace string) (*k8sService.BaseInfo, error) {
	return m.GetBaseInfoFunc(clusterID, clusterName, nodeID, envID, namespace)
}

func (m *mockK8sService) GetNodeList(clusterID, clusterName string, nodeID, envID uint) ([]*k8sService.Node, error) {
	return m.GetNodeListFunc(clusterID, clusterName, nodeID, envID)
}

func (m *mockK8sService) GetEventList(clusterID, clusterName string, nodeID, envID uint, namespace, objectName, objectKind string) ([]*k8sService.Event, error) {
	return m.GetEventListFunc(clusterID, clusterName, nodeID, envID, namespace, objectName, objectKind)
}

func (m *mockK8sService) ScaleReplica(clusterID, clusterName string, nodeID, envID uint, namespace, deploymentName string, desiredReplicas uint) (*k8sService.ReplicaCounts, error) {
	return m.ScaleReplicaFunc(clusterID, clusterName, nodeID, envID, namespace, deploymentName, desiredReplicas)
}

func (m *mockK8sService) GetNamespaceList(clusterID, clusterName string) ([]*k8sService.Namespace, error) {
	return m.GetNamespaceListFunc(clusterID, clusterName)
}

func (m *mockK8sService) GetReplica(clusterID, clusterName string, nodeID, envID uint, namespace string) (*k8sService.ReplicaCounts, error) {
	return m.GetReplicaFunc(clusterID, clusterName, nodeID, envID, namespace)
}

func (m *mockK8sService) GetPodList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Pod, error) {
	return m.GetPodListFunc(clusterID, clusterName, nodeID, envID, namespace)
}

func (m *mockK8sService) GetPodDetail(clusterID, clusterName, namespace, podName string) (*k8sService.PodDetail, error) {
	return m.GetPodDetailFunc(clusterID, clusterName, namespace, podName)
}

func (m *mockK8sService) GetContainersList(clusterID, clusterName string, nodeID, envID uint, namespace, podName string) ([]*k8sService.Container, error) {
	return m.GetContainersListFunc(clusterID, clusterName, nodeID, envID, namespace, podName)
}

func (m *mockK8sService) RestartPod(clusterID, clusterName string, nodeID, envID uint, namespace, podName string) error {
	return m.RestartPodFunc(clusterID, clusterName, nodeID, envID, namespace, podName)
}

func (m *mockK8sService) GetPodMetrics(clusterID, clusterName, namespace, podName, metricsName string, lastTime, step uint) (interface{}, error) {
	return m.GetPodMetricsFunc(clusterID, clusterName, namespace, podName, metricsName, lastTime, step)
}

func (m *mockK8sService) DownloadContainerLogs(clusterID, clusterName string, nodeID, envID uint, namespace, podName, container string, limitBytes, sinceSecond int) (string, error) {
	return m.DownloadContainerLogsFunc(clusterID, clusterName, nodeID, envID, namespace, podName, container, limitBytes, sinceSecond)
}

func (m *mockK8sService) GetConfigMapList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.ConfigMap, error) {
	return m.GetConfigMapListFunc(clusterID, clusterName, nodeID, envID, namespace)
}

func (m *mockK8sService) GetSecretList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Secret, error) {
	return m.GetSecretListFunc(clusterID, clusterName, nodeID, envID, namespace)
}

func (m *mockK8sService) GetPVList(clusterID, clusterName string, nodeID, envID uint) ([]*k8sService.PV, error) {
	return m.GetPVListFunc(clusterID, clusterName, nodeID, envID)
}

func (m *mockK8sService) GetStorageClassList(clusterID, clusterName string, nodeID, envID uint) ([]*k8sService.StorageClass, error) {
	return m.GetStorageClassListFunc(clusterID, clusterName, nodeID, envID)
}

func (m *mockK8sService) GetPVCList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.PVC, error) {
	return m.GetPVCListFunc(clusterID, clusterName, nodeID, envID, namespace)
}

func (m *mockK8sService) GetResourceYaml(clusterID, clusterName, namespace, resourceType, resourceName string) (string, error) {
	return m.GetResourceYamlFunc(clusterID, clusterName, namespace, resourceType, resourceName)
}

func (m *mockK8sService) UpdateResourceYaml(clusterID, clusterName, namespace, resourceType, resourceName, yaml string) error {
	return m.UpdateResourceYamlFunc(clusterID, clusterName, namespace, resourceType, resourceName, yaml)
}

func (m *mockK8sService) DeleteResource(clusterID, clusterName, namespace, resourceType, resourceName string) error {
	return m.DeleteResourceFunc(clusterID, clusterName, namespace, resourceType, resourceName)
}

func (m *mockK8sService) DryRunResourceYaml(clusterID, clusterName, namespace, resourceType, resourceName, yaml string) (string, error) {
	return m.DryRunResourceYamlFunc(clusterID, clusterName, namespace, resourceType, resourceName, yaml)
}

func (m *mockK8sService) GetServiceList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Service, error) {
	return m.GetServiceListFunc(clusterID, clusterName, nodeID, envID, namespace)
}

func (m *mockK8sService) GetIngressList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Ingress, error) {
	return m.GetIngressListFunc(clusterID, clusterName, nodeID, envID, namespace)
}

func (m *mockK8sService) GetHPAList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.HPA, error) {
	return m.GetHPAListFunc(clusterID, clusterName, nodeID, envID, namespace)
}

func (m *mockK8sService) GetDeploymentList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Deployment, error) {
	return m.GetDeploymentListFunc(clusterID, clusterName, nodeID, envID, namespace)
}

func (m *mockK8sService) GetDeploymentDetail(clusterID, clusterName, namespace, deploymentName string) (*k8sService.DeploymentDetail, error) {
	return m.GetDeploymentDetailFunc(clusterID, clusterName, namespace, deploymentName)
}

func (m *mockK8sService) GetDeploymentRevisions(clusterID, clusterName, namespace, deploymentName string) ([]*k8sService.DeploymentRevision, int64, error) {
	return m.GetDeploymentRevisionsFunc(clusterID, clusterName, namespace, deploymentName)
}

func (m *mockK8sService) RollbackDeployment(clusterID, clusterName, namespace, deploymentName string, toRevision int64) error {
	return m.RollbackDeploymentFunc(clusterID, clusterName, namespace, deploymentName, toRevision)
}

func (m *mockK8sService) GetDeploymentMetrics(clusterID, clusterName, namespace, deploymentName string, lastTime, step uint) (interface{}, error) {
	return m.GetDeploymentMetricsFunc(clusterID, clusterName, namespace, deploymentName, lastTime, step)
}

func (m *mockK8sService) GetDaemonSetList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.DaemonSet, error) {
	return m.GetDaemonSetListFunc(clusterID, clusterName, nodeID, envID, namespace)
}

func (m *mockK8sService) GetDaemonSetDetail(clusterID, clusterName, namespace, daemonSetName string) (*k8sService.DeploymentDetail, error) {
	return m.GetDaemonSetDetailFunc(clusterID, clusterName, namespace, daemonSetName)
}

func (m *mockK8sService) GetDaemonSetMetrics(clusterID, clusterName, namespace, daemonSetName string, lastTime, step uint) (interface{}, error) {
	return m.GetDaemonSetMetricsFunc(clusterID, clusterName, namespace, daemonSetName, lastTime, step)
}

func (m *mockK8sService) GetDaemonSetRevisions(clusterID, clusterName, namespace, daemonSetName string) ([]*k8sService.DaemonSetRevision, int64, error) {
	return m.GetDaemonSetRevisionsFunc(clusterID, clusterName, namespace, daemonSetName)
}

func (m *mockK8sService) RollbackDaemonSet(clusterID, clusterName, namespace, daemonSetName string, toRevision int64) error {
	return m.RollbackDaemonSetFunc(clusterID, clusterName, namespace, daemonSetName, toRevision)
}

func (m *mockK8sService) GetStatefulSetList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.StatefulSet, error) {
	return m.GetStatefulSetListFunc(clusterID, clusterName, nodeID, envID, namespace)
}

func (m *mockK8sService) GetStatefulSetDetail(clusterID, clusterName, namespace, statefulSetName string) (*k8sService.DeploymentDetail, error) {
	return m.GetStatefulSetDetailFunc(clusterID, clusterName, namespace, statefulSetName)
}

func (m *mockK8sService) GetStatefulSetMetrics(clusterID, clusterName, namespace, statefulSetName string, lastTime, step uint) (interface{}, error) {
	return m.GetStatefulSetMetricsFunc(clusterID, clusterName, namespace, statefulSetName, lastTime, step)
}

func (m *mockK8sService) GetStatefulSetRevisions(clusterID, clusterName, namespace, statefulSetName string) ([]*k8sService.StatefulSetRevision, int64, error) {
	return m.GetStatefulSetRevisionsFunc(clusterID, clusterName, namespace, statefulSetName)
}

func (m *mockK8sService) RollbackStatefulSet(clusterID, clusterName, namespace, statefulSetName string, toRevision int64) error {
	return m.RollbackStatefulSetFunc(clusterID, clusterName, namespace, statefulSetName, toRevision)
}

func (m *mockK8sService) GetCronJobList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.CronJob, error) {
	return m.GetCronJobListFunc(clusterID, clusterName, nodeID, envID, namespace)
}

func (m *mockK8sService) GetCronJobDetail(clusterID, clusterName, namespace, cronJobName string) (*k8sService.CronJobDetail, error) {
	return m.GetCronJobDetailFunc(clusterID, clusterName, namespace, cronJobName)
}

func (m *mockK8sService) GetJobList(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Job, error) {
	return m.GetJobListFunc(clusterID, clusterName, nodeID, envID, namespace)
}

func (m *mockK8sService) GetJobDetail(clusterID, clusterName, namespace, jobName string) (*k8sService.JobDetail, error) {
	return m.GetJobDetailFunc(clusterID, clusterName, namespace, jobName)
}

func (m *mockK8sService) GetNodeMetricsList(clusterID, clusterName string) ([]k8sService.NodeMetrics, error) {
	return m.GetNodeMetricsListFunc(clusterID, clusterName)
}
