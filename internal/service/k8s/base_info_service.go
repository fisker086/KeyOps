package k8s

import (
	"fmt"

	"github.com/fisker086/keyops/internal/model"
)

// GetBaseInfo 获取基础信息
// 支持两种方式：
// 1. 使用 cluster_id 或 cluster_name（推荐，多集群场景）
// 2. 使用 node_id 和 env_id（兼容旧方式）
func (s *K8sService) GetBaseInfo(clusterID string, clusterName string, nodeID uint, envID uint, namespace string) (*BaseInfo, error) {
	var cluster *model.K8sCluster
	var err error

	// 优先使用 cluster_id/cluster_name
	if clusterID != "" || clusterName != "" {
		cluster, err = s.GetClusterConfig(clusterID, clusterName)
		if err != nil {
			return nil, err
		}
	} else if nodeID > 0 && envID > 0 {
		// 兼容旧方式：通过 node_id 和 env_id 获取集群
		// TODO: 实现从 node_containers 表获取集群信息
		return nil, fmt.Errorf("请使用 cluster_id 或 cluster_name 参数")
	} else {
		return nil, fmt.Errorf("必须提供 cluster_id/cluster_name 或 node_id/env_id")
	}

	// 使用命名空间
	ns := namespace
	if ns == "" {
		ns = cluster.DefaultNamespace
	}
	if ns == "" {
		ns = "default"
	}

	// 获取节点数量
	nodes, err := s.GetNodeList(clusterID, clusterName, 0, 0)
	nodeCount := 0
	if err == nil {
		nodeCount = len(nodes)
	}

	// 获取 Pod 数量
	pods, err := s.GetPodList(clusterID, clusterName, 0, 0, ns)
	podCount := 0
	if err == nil {
		podCount = len(pods)
	}

	// 获取 Service 数量
	services, err := s.GetServiceList(clusterID, clusterName, 0, 0, ns)
	serviceCount := 0
	if err == nil {
		serviceCount = len(services)
	}

	// 获取 Ingress 数量
	ingresses, err := s.GetIngressList(clusterID, clusterName, 0, 0, ns)
	ingressCount := 0
	if err == nil {
		ingressCount = len(ingresses)
	}

	return &BaseInfo{
		Cluster:      cluster.Name,
		Namespace:    ns,
		NodeCount:    nodeCount,
		PodCount:     podCount,
		ServiceCount: serviceCount,
		IngressCount: ingressCount,
	}, nil
}

