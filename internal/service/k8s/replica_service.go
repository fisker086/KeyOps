package k8s

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GetReplica 获取副本数
// 注意：此方法需要 deployment_name 参数，但当前接口没有提供
// 可以通过 node_id 查找对应的 deployment，或者需要修改接口添加 deployment_name 参数
// 暂时通过获取 Deployment 列表，然后使用第一个 Deployment（简化实现）
func (s *K8sService) GetReplica(clusterID string, clusterName string, nodeID uint, envID uint, namespace string) (*ReplicaCounts, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	ns := s.getNamespace(cluster, namespace)

	// 如果没有指定 deployment_name，尝试获取 Deployment 列表并使用第一个
	// 这是一个简化实现，理想情况下应该通过 node_id/env_id 查找对应的 deployment
	deployments, err := s.GetDeploymentList(clusterID, clusterName, nodeID, envID, namespace)
	if err != nil {
		return nil, fmt.Errorf("获取 Deployment 列表失败: %v", err)
	}

	if len(deployments) == 0 {
		return nil, fmt.Errorf("命名空间 %s 中没有找到 Deployment", ns)
	}

	// 使用第一个 Deployment（简化实现）
	deploymentName := deployments[0].Name

	// 获取 Deployment 详情以获取副本数
	deploymentURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/apps/v1/namespaces/" + ns + "/deployments/" + deploymentName
	httpReq, client, err := s.createK8sHTTPClient(cluster, deploymentURL)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API请求失败: %s, 响应: %s", resp.Status, string(body))
	}

	var deploymentResponse struct {
		Spec struct {
			Replicas *int32 `json:"replicas"`
		} `json:"spec"`
		Status struct {
			Replicas          int32 `json:"replicas"`
			ReadyReplicas     int32 `json:"readyReplicas"`
			UpdatedReplicas   int32 `json:"updatedReplicas"`
			AvailableReplicas int32 `json:"availableReplicas"`
		} `json:"status"`
	}

	if err := json.Unmarshal(body, &deploymentResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	desiredReplicas := int32(0)
	if deploymentResponse.Spec.Replicas != nil {
		desiredReplicas = *deploymentResponse.Spec.Replicas
	}

	return &ReplicaCounts{
		DesiredReplicas: desiredReplicas,
		ActualReplicas:  deploymentResponse.Status.Replicas,
		Desired:         desiredReplicas,
		Current:         deploymentResponse.Status.Replicas,
		Ready:           deploymentResponse.Status.ReadyReplicas,
		Available:       deploymentResponse.Status.AvailableReplicas,
	}, nil
}

// ScaleReplica 扩缩容
func (s *K8sService) ScaleReplica(clusterID string, clusterName string, nodeID uint, envID uint, namespace string, deploymentName string, desiredReplicas uint) (*ReplicaCounts, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	ns := s.getNamespace(cluster, namespace)

	// 如果没有指定 deployment_name，尝试获取 Deployment 列表并使用第一个（向后兼容）
	if deploymentName == "" {
	deployments, err := s.GetDeploymentList(clusterID, clusterName, nodeID, envID, namespace)
	if err != nil {
		return nil, fmt.Errorf("获取 Deployment 列表失败: %v", err)
	}

	if len(deployments) == 0 {
		return nil, fmt.Errorf("命名空间 %s 中没有找到 Deployment", ns)
	}

		// 使用第一个 Deployment（向后兼容）
		deploymentName = deployments[0].Name
	}

	// 获取当前的 Deployment
	deploymentURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/apps/v1/namespaces/" + ns + "/deployments/" + deploymentName
	httpReq, client, err := s.createK8sHTTPClient(cluster, deploymentURL)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API请求失败: %s, 响应: %s", resp.Status, string(body))
	}

	var deploymentResponse struct {
		Spec struct {
			Replicas *int32 `json:"replicas"`
		} `json:"spec"`
		Status struct {
			Replicas          int32 `json:"replicas"`
			ReadyReplicas     int32 `json:"readyReplicas"`
			UpdatedReplicas   int32 `json:"updatedReplicas"`
			AvailableReplicas int32 `json:"availableReplicas"`
		} `json:"status"`
	}

	if err := json.Unmarshal(body, &deploymentResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 更新副本数
	desiredReplicasInt32 := int32(desiredReplicas)
	deploymentResponse.Spec.Replicas = &desiredReplicasInt32

	// 构建更新请求
	updateBody, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": desiredReplicasInt32,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("构建请求体失败: %v", err)
	}

	// 创建 PATCH 请求
	patchReq, err := http.NewRequest("PATCH", deploymentURL, bytes.NewBuffer(updateBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}
	patchReq.Header.Set("Content-Type", "application/merge-patch+json")

	// 设置认证
	var tlsConfig *tls.Config
	if cluster.AuthType == "token" && cluster.Token != "" {
		patchReq.Header.Set("Authorization", "Bearer "+cluster.Token)
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	} else if cluster.AuthType == "kubeconfig" && cluster.Kubeconfig != "" {
		clusterService := NewK8sClusterService(s.clusterRepo)
		authInfo, err := clusterService.parseKubeconfigAuth(cluster.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("解析Kubeconfig失败: %v", err)
		}

		if authInfo.Token != "" {
			patchReq.Header.Set("Authorization", "Bearer "+authInfo.Token)
			tlsConfig = &tls.Config{InsecureSkipVerify: true}
		} else if authInfo.ClientCert != "" && authInfo.ClientKey != "" {
			cert, err := tls.X509KeyPair([]byte(authInfo.ClientCert), []byte(authInfo.ClientKey))
			if err != nil {
				return nil, fmt.Errorf("解析客户端证书失败: %v", err)
			}
			tlsConfig = &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: true,
			}
		} else {
			return nil, fmt.Errorf("Kubeconfig中未找到有效的认证信息")
		}
	} else {
		return nil, fmt.Errorf("缺少认证信息，无法连接集群")
	}

	patchClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: 10 * time.Second,
	}

	patchResp, err := patchClient.Do(patchReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer patchResp.Body.Close()

	patchBody, err := io.ReadAll(patchResp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	if patchResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API请求失败: %s, 响应: %s", patchResp.Status, string(patchBody))
	}

	// 解析更新后的响应
	var updatedDeployment struct {
		Status struct {
			Replicas          int32 `json:"replicas"`
			ReadyReplicas     int32 `json:"readyReplicas"`
			UpdatedReplicas   int32 `json:"updatedReplicas"`
			AvailableReplicas int32 `json:"availableReplicas"`
		} `json:"status"`
	}

	if err := json.Unmarshal(patchBody, &updatedDeployment); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return &ReplicaCounts{
		DesiredReplicas: desiredReplicasInt32,
		ActualReplicas:  updatedDeployment.Status.Replicas,
		Desired:         desiredReplicasInt32,
		Current:         updatedDeployment.Status.Replicas,
		Ready:           updatedDeployment.Status.ReadyReplicas,
		Available:       updatedDeployment.Status.AvailableReplicas,
	}, nil
}

