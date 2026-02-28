package k8s

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"gopkg.in/yaml.v3"
)

// GetResourceYaml 获取 K8s 资源的 YAML 内容
func (s *K8sService) GetResourceYaml(clusterID string, clusterName string, namespace string, resourceType string, resourceName string) (string, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return "", fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return "", err
	}

	// 根据资源类型构建 API 路径
	// 注意：PV 和 StorageClass 是集群级别的资源，不需要 namespace
	apiPath, err := s.getResourceAPIPath(resourceType, namespace, resourceName, cluster)
	if err != nil {
		return "", err
	}

	resourceURL := strings.TrimSuffix(cluster.APIServer, "/") + apiPath
	httpReq, client, err := s.createK8sHTTPClient(cluster, resourceURL)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API请求失败: %s, 响应: %s", resp.Status, string(body))
	}

	// 解析 JSON 响应
	var resourceObj map[string]interface{}
	if err := json.Unmarshal(body, &resourceObj); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	// 清理系统字段，但保留必要的元数据
	if metadata, ok := resourceObj["metadata"].(map[string]interface{}); ok {
		// 删除系统生成的字段
		delete(metadata, "managedFields")
		delete(metadata, "resourceVersion")
		delete(metadata, "uid")
		delete(metadata, "selfLink")
		delete(metadata, "generation")
		delete(metadata, "creationTimestamp")
	}

	// 删除状态字段（通常不需要在 YAML 中）
	delete(resourceObj, "status")

	// 转换为 YAML
	yamlBytes, err := yaml.Marshal(resourceObj)
	if err != nil {
		return "", fmt.Errorf("序列化YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

// DeleteResource 删除 K8s 资源
func (s *K8sService) DeleteResource(clusterID string, clusterName string, namespace string, resourceType string, resourceName string) error {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return err
	}

	apiPath, err := s.getResourceAPIPath(resourceType, namespace, resourceName, cluster)
	if err != nil {
		return err
	}

	resourceURL := strings.TrimSuffix(cluster.APIServer, "/") + apiPath
	httpReq, client, err := s.createK8sHTTPClient(cluster, resourceURL)
	if err != nil {
		return err
	}
	httpReq.Method = "DELETE"
	httpReq.Body = nil
	httpReq.GetBody = nil
	httpReq.ContentLength = 0

	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("删除资源失败: %s, 响应: %s", resp.Status, string(body))
	}
	return nil
}

// getResourceAPIPath 根据资源类型获取 API 路径
func (s *K8sService) getResourceAPIPath(resourceType string, namespace string, resourceName string, cluster *model.K8sCluster) (string, error) {
	resourceType = strings.ToLower(resourceType)
	
	// 获取命名空间（如果需要）
	var ns string
	if namespace != "" {
		// 如果传入了 cluster，使用 getNamespace 方法获取默认命名空间
		if cluster != nil {
			ns = s.getNamespace(cluster, namespace)
		} else {
			ns = namespace
		}
	}
	
	switch resourceType {
	case "pod":
		if ns == "" {
			return "", fmt.Errorf("pod 资源需要 namespace")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/pods/%s", ns, resourceName), nil
	case "service":
		if ns == "" {
			return "", fmt.Errorf("service 资源需要 namespace")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/services/%s", ns, resourceName), nil
	case "ingress":
		if ns == "" {
			return "", fmt.Errorf("ingress 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/networking.k8s.io/v1/namespaces/%s/ingresses/%s", ns, resourceName), nil
	case "deployment":
		if ns == "" {
			return "", fmt.Errorf("deployment 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", ns, resourceName), nil
	case "daemonset":
		if ns == "" {
			return "", fmt.Errorf("daemonset 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/apps/v1/namespaces/%s/daemonsets/%s", ns, resourceName), nil
	case "statefulset":
		if ns == "" {
			return "", fmt.Errorf("statefulset 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/apps/v1/namespaces/%s/statefulsets/%s", ns, resourceName), nil
	case "job":
		if ns == "" {
			return "", fmt.Errorf("job 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/batch/v1/namespaces/%s/jobs/%s", ns, resourceName), nil
	case "cronjob":
		if ns == "" {
			return "", fmt.Errorf("cronjob 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/batch/v1/namespaces/%s/cronjobs/%s", ns, resourceName), nil
	// 存储相关资源
	case "pv":
		// PV 是集群级别的资源，不需要 namespace
		return fmt.Sprintf("/api/v1/persistentvolumes/%s", resourceName), nil
	case "pvc":
		if ns == "" {
			return "", fmt.Errorf("pvc 资源需要 namespace")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/persistentvolumeclaims/%s", ns, resourceName), nil
	case "storageclass", "sc":
		// StorageClass 是集群级别的资源，不需要 namespace
		return fmt.Sprintf("/apis/storage.k8s.io/v1/storageclasses/%s", resourceName), nil
	case "configmap", "cm":
		if ns == "" {
			return "", fmt.Errorf("configmap 资源需要 namespace")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/configmaps/%s", ns, resourceName), nil
	case "secret":
		if ns == "" {
			return "", fmt.Errorf("secret 资源需要 namespace")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", ns, resourceName), nil
	// Istio 资源
	case "destinationrule", "destination-rule":
		if ns == "" {
			return "", fmt.Errorf("destinationrule 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/networking.istio.io/v1beta1/namespaces/%s/destinationrules/%s", ns, resourceName), nil
	case "gateway":
		if ns == "" {
			return "", fmt.Errorf("gateway 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/networking.istio.io/v1beta1/namespaces/%s/gateways/%s", ns, resourceName), nil
	case "virtualservice", "virtual-service":
		if ns == "" {
			return "", fmt.Errorf("virtualservice 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/networking.istio.io/v1beta1/namespaces/%s/virtualservices/%s", ns, resourceName), nil
	default:
		return "", fmt.Errorf("不支持的资源类型: %s", resourceType)
	}
}

// UpdateResourceYaml 更新 K8s 资源的 YAML 内容
func (s *K8sService) UpdateResourceYaml(clusterID string, clusterName string, namespace string, resourceType string, resourceName string, yamlContent string) error {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return err
	}

	// 解析 YAML 为 JSON
	var resourceObj map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &resourceObj); err != nil {
		return fmt.Errorf("解析YAML失败: %v", err)
	}

	// 验证和设置资源名称和命名空间
	if metadata, ok := resourceObj["metadata"].(map[string]interface{}); ok {
		if name, ok := metadata["name"].(string); ok && name != resourceName {
			return fmt.Errorf("YAML中的资源名称(%s)与请求参数(%s)不匹配", name, resourceName)
		}
		// 确保资源名称正确
		metadata["name"] = resourceName
		
		// 对于需要namespace的资源，确保namespace正确设置
		resourceTypeLower := strings.ToLower(resourceType)
		clusterLevelResources := map[string]bool{
			"pv":           true,
			"storageclass": true,
			"sc":           true,
		}
		if !clusterLevelResources[resourceTypeLower] && namespace != "" {
			metadata["namespace"] = namespace
		}
	}

	// 获取 API 路径
	apiPath, err := s.getResourceAPIPath(resourceType, namespace, resourceName, cluster)
	if err != nil {
		return err
	}

	resourceURL := strings.TrimSuffix(cluster.APIServer, "/") + apiPath

	// 转换为 JSON
	jsonData, err := json.Marshal(resourceObj)
	if err != nil {
		return fmt.Errorf("序列化JSON失败: %v", err)
	}

	// 创建 PUT 请求
	httpReq, err := http.NewRequest("PUT", resourceURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// 设置认证
	var tlsConfig *tls.Config
	if cluster.AuthType == "token" && cluster.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cluster.Token)
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	} else if cluster.AuthType == "kubeconfig" && cluster.Kubeconfig != "" {
		clusterService := NewK8sClusterService(s.clusterRepo)
		authInfo, err := clusterService.parseKubeconfigAuth(cluster.Kubeconfig)
		if err != nil {
			return fmt.Errorf("解析Kubeconfig失败: %v", err)
		}

		if authInfo.Token != "" {
			httpReq.Header.Set("Authorization", "Bearer "+authInfo.Token)
			tlsConfig = &tls.Config{InsecureSkipVerify: true}
		} else if authInfo.ClientCert != "" && authInfo.ClientKey != "" {
			cert, err := tls.X509KeyPair([]byte(authInfo.ClientCert), []byte(authInfo.ClientKey))
			if err != nil {
				return fmt.Errorf("解析客户端证书失败: %v", err)
			}
			tlsConfig = &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: true,
			}
		} else {
			return fmt.Errorf("Kubeconfig中未找到有效的认证信息")
		}
	} else {
		return fmt.Errorf("缺少认证信息，无法连接集群")
	}

	// 创建 HTTP 客户端
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API请求失败: %s, 响应: %s", resp.Status, string(body))
	}

	return nil
}

// DryRunResourceYaml Dry-run 预览 K8s 资源变更
func (s *K8sService) DryRunResourceYaml(clusterID string, clusterName string, namespace string, resourceType string, resourceName string, yamlContent string) (string, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return "", fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return "", err
	}

	// 解析 YAML 为 JSON
	var resourceObj map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &resourceObj); err != nil {
		return "", fmt.Errorf("解析YAML失败: %v", err)
	}

	// 验证和设置资源名称和命名空间
	// 对于 dry-run，允许 YAML 中的名称与请求参数不一致（因为 dry-run 可以用于创建新资源或重命名）
	var actualResourceName string
	if metadata, ok := resourceObj["metadata"].(map[string]interface{}); ok {
		// 如果 YAML 中有名称，使用 YAML 中的名称；否则使用请求参数中的名称
		if yamlName, ok := metadata["name"].(string); ok && yamlName != "" {
			actualResourceName = yamlName
		} else {
			actualResourceName = resourceName
			metadata["name"] = resourceName
		}
		
		// 对于需要namespace的资源，确保namespace正确设置
		resourceTypeLower := strings.ToLower(resourceType)
		clusterLevelResources := map[string]bool{
			"pv":           true,
			"storageclass": true,
			"sc":           true,
		}
		if !clusterLevelResources[resourceTypeLower] && namespace != "" {
			metadata["namespace"] = namespace
		}
	} else {
		actualResourceName = resourceName
	}

	// 转换为 JSON
	jsonData, err := json.Marshal(resourceObj)
	if err != nil {
		return "", fmt.Errorf("序列化JSON失败: %v", err)
	}

	// 设置认证信息（需要在创建请求前设置）
	var tlsConfig *tls.Config
	var authHeader string
	
	if cluster.AuthType == "token" && cluster.Token != "" {
		authHeader = "Bearer " + cluster.Token
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	} else if cluster.AuthType == "kubeconfig" && cluster.Kubeconfig != "" {
		clusterService := NewK8sClusterService(s.clusterRepo)
		authInfo, err := clusterService.parseKubeconfigAuth(cluster.Kubeconfig)
		if err != nil {
			return "", fmt.Errorf("解析Kubeconfig失败: %v", err)
		}

		if authInfo.Token != "" {
			authHeader = "Bearer " + authInfo.Token
			tlsConfig = &tls.Config{InsecureSkipVerify: true}
		} else if authInfo.ClientCert != "" && authInfo.ClientKey != "" {
			cert, err := tls.X509KeyPair([]byte(authInfo.ClientCert), []byte(authInfo.ClientKey))
			if err != nil {
				return "", fmt.Errorf("解析客户端证书失败: %v", err)
			}
			tlsConfig = &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: true,
			}
		} else {
			return "", fmt.Errorf("Kubeconfig中未找到有效的认证信息")
		}
	} else {
		return "", fmt.Errorf("缺少认证信息，无法连接集群")
	}

	// 创建 HTTP 客户端
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: 30 * time.Second,
	}

	// 先尝试 POST（创建），如果资源已存在（409），则使用 PUT（更新）
	// 获取集合端点路径（用于 POST）
	collectionPath, err := s.getResourceAPIPathForDryRun(resourceType, namespace, cluster)
	if err != nil {
		return "", err
	}
	collectionURL := strings.TrimSuffix(cluster.APIServer, "/") + collectionPath + "?dryRun=All"

	// 创建 POST 请求
	httpReq, err := http.NewRequest("POST", collectionURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		httpReq.Header.Set("Authorization", authHeader)
	}

	// 先尝试 POST（创建资源）
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	// 如果返回 409（资源已存在），尝试使用 PUT（更新资源）
	if resp.StatusCode == http.StatusConflict {
		// 获取具体资源端点路径（用于 PUT）
		resourcePath, err := s.getResourceAPIPath(resourceType, namespace, actualResourceName, cluster)
		if err != nil {
			return "", fmt.Errorf("获取资源路径失败: %v", err)
		}
		resourceURL := strings.TrimSuffix(cluster.APIServer, "/") + resourcePath + "?dryRun=All"

		// 创建 PUT 请求
		putReq, err := http.NewRequest("PUT", resourceURL, strings.NewReader(string(jsonData)))
		if err != nil {
			return "", fmt.Errorf("创建PUT请求失败: %v", err)
		}
		putReq.Header.Set("Content-Type", "application/json")
		if authHeader != "" {
			putReq.Header.Set("Authorization", authHeader)
		}

		// 执行 PUT 请求
		resp, err = client.Do(putReq)
		if err != nil {
			return "", fmt.Errorf("PUT请求失败: %v", err)
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("读取PUT响应失败: %v", err)
		}
	}

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("API请求失败: %s, 响应: %s", resp.Status, string(body))
	}

	// 解析响应并转换为 YAML
	var responseObj map[string]interface{}
	if err := json.Unmarshal(body, &responseObj); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	// 清理系统字段
	if metadata, ok := responseObj["metadata"].(map[string]interface{}); ok {
		delete(metadata, "managedFields")
		delete(metadata, "resourceVersion")
		delete(metadata, "uid")
		delete(metadata, "selfLink")
		delete(metadata, "generation")
		delete(metadata, "creationTimestamp")
	}

	// 转换为 YAML
	yamlBytes, err := yaml.Marshal(responseObj)
	if err != nil {
		return "", fmt.Errorf("序列化YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

// getResourceAPIPathForDryRun 获取用于 dry-run 的资源 API 路径（集合端点）
func (s *K8sService) getResourceAPIPathForDryRun(resourceType string, namespace string, cluster *model.K8sCluster) (string, error) {
	resourceType = strings.ToLower(resourceType)
	
	var ns string
	if namespace != "" {
		if cluster != nil {
			ns = s.getNamespace(cluster, namespace)
		} else {
			ns = namespace
		}
	}
	
	switch resourceType {
	case "pod":
		if ns == "" {
			return "", fmt.Errorf("pod 资源需要 namespace")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/pods", ns), nil
	case "service":
		if ns == "" {
			return "", fmt.Errorf("service 资源需要 namespace")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/services", ns), nil
	case "ingress":
		if ns == "" {
			return "", fmt.Errorf("ingress 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/networking.k8s.io/v1/namespaces/%s/ingresses", ns), nil
	case "deployment":
		if ns == "" {
			return "", fmt.Errorf("deployment 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments", ns), nil
	case "daemonset":
		if ns == "" {
			return "", fmt.Errorf("daemonset 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/apps/v1/namespaces/%s/daemonsets", ns), nil
	case "statefulset":
		if ns == "" {
			return "", fmt.Errorf("statefulset 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/apps/v1/namespaces/%s/statefulsets", ns), nil
	case "job":
		if ns == "" {
			return "", fmt.Errorf("job 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/batch/v1/namespaces/%s/jobs", ns), nil
	case "cronjob":
		if ns == "" {
			return "", fmt.Errorf("cronjob 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/batch/v1/namespaces/%s/cronjobs", ns), nil
	case "pv":
		return "/api/v1/persistentvolumes", nil
	case "pvc":
		if ns == "" {
			return "", fmt.Errorf("pvc 资源需要 namespace")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/persistentvolumeclaims", ns), nil
	case "storageclass", "sc":
		return "/apis/storage.k8s.io/v1/storageclasses", nil
	case "configmap", "cm":
		if ns == "" {
			return "", fmt.Errorf("configmap 资源需要 namespace")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/configmaps", ns), nil
	case "secret":
		if ns == "" {
			return "", fmt.Errorf("secret 资源需要 namespace")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/secrets", ns), nil
	case "destinationrule", "destination-rule":
		if ns == "" {
			return "", fmt.Errorf("destinationrule 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/networking.istio.io/v1beta1/namespaces/%s/destinationrules", ns), nil
	case "gateway":
		if ns == "" {
			return "", fmt.Errorf("gateway 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/networking.istio.io/v1beta1/namespaces/%s/gateways", ns), nil
	case "virtualservice", "virtual-service":
		if ns == "" {
			return "", fmt.Errorf("virtualservice 资源需要 namespace")
		}
		return fmt.Sprintf("/apis/networking.istio.io/v1beta1/namespaces/%s/virtualservices", ns), nil
	default:
		return "", fmt.Errorf("不支持的资源类型: %s", resourceType)
	}
}
