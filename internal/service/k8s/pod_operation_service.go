package k8s

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// RestartPod 重启 Pod
func (s *K8sService) RestartPod(clusterID string, clusterName string, nodeID uint, envID uint, namespace string, podName string) error {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return err
	}

	if podName == "" {
		return fmt.Errorf("pod_name 参数必填")
	}

	ns := s.getNamespace(cluster, namespace)

	// 重启 Pod 通过删除 Pod 实现（Deployment/StatefulSet 会自动重建）
	podURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/pods/" + podName
	
	// 创建 DELETE 请求
	httpReq, err := http.NewRequest("DELETE", podURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// 设置认证
	var client *http.Client
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

	client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: 10 * time.Second,
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("API请求失败: %s, 响应: %s", resp.Status, string(body))
	}

	return nil
}

// DownloadContainerLogs 下载容器日志
func (s *K8sService) DownloadContainerLogs(clusterID string, clusterName string, nodeID uint, envID uint, namespace string, podName, container string, limitBytes, sinceSecond int) (string, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return "", fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return "", err
	}

	if podName == "" || container == "" {
		return "", fmt.Errorf("pod_name 和 container 参数必填")
	}

	ns := s.getNamespace(cluster, namespace)

	// 构建日志 URL
	logsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/pods/" + podName + "/log"
	params := url.Values{}
	params.Set("container", container)
	if limitBytes > 0 {
		params.Set("limitBytes", strconv.Itoa(limitBytes))
	}
	if sinceSecond > 0 {
		// sinceSecond 转换为时间戳
		sinceTime := time.Now().Add(-time.Duration(sinceSecond) * time.Second)
		params.Set("sinceTime", sinceTime.Format(time.RFC3339))
	}
	if len(params) > 0 {
		logsURL += "?" + params.Encode()
	}

	httpReq, client, err := s.createK8sHTTPClient(cluster, logsURL)
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

	return string(body), nil
}

