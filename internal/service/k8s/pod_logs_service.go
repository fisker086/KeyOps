package k8s

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// PodLogsService Pod 日志服务
type PodLogsService struct {
	*K8sService
}

// NewPodLogsService 创建 Pod 日志服务
func NewPodLogsService(k8sService *K8sService) *PodLogsService {
	return &PodLogsService{K8sService: k8sService}
}

// StreamPodLogs 流式传输 Pod 日志到 WebSocket
func (s *PodLogsService) StreamPodLogs(clusterID, clusterName, namespace, podName, container string, follow bool, tailLines int, ws *websocket.Conn) error {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil {
		return fmt.Errorf("获取集群配置失败: %v", err)
	}

	if podName == "" {
		return fmt.Errorf("pod_name 参数必填")
	}

	ns := s.getNamespace(cluster, namespace)

	// 如果没有指定容器，使用第一个容器
	if container == "" {
		// 获取 Pod 信息以获取容器列表
		podURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/pods/" + podName
		httpReq, client, err := s.createK8sHTTPClient(cluster, podURL)
		if err != nil {
			return err
		}

		resp, err := client.Do(httpReq)
		if err != nil {
			return fmt.Errorf("获取 Pod 信息失败: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("获取 Pod 信息失败: %s, 响应: %s", resp.Status, string(body))
		}

		// 解析 Pod 响应获取第一个容器名称
		var podResponse struct {
			Spec struct {
				Containers []struct {
					Name string `json:"name"`
				} `json:"containers"`
			} `json:"spec"`
		}
		body, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(body, &podResponse); err == nil {
			if len(podResponse.Spec.Containers) > 0 {
				container = podResponse.Spec.Containers[0].Name
			}
		}
	}

	if container == "" {
		return fmt.Errorf("未找到容器")
	}

	// 设置默认 tailLines
	if tailLines <= 0 {
		tailLines = 100
	}

	// 构建日志 URL
	logsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/pods/" + podName + "/log"
	params := url.Values{}
	params.Set("container", container)
	if follow {
		params.Set("follow", "true")
	}
	if tailLines > 0 {
		params.Set("tailLines", strconv.Itoa(tailLines))
	}
	logsURL += "?" + params.Encode()

	// 创建 HTTP 请求
	httpReq, err := http.NewRequest("GET", logsURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

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
		Timeout: 0, // 不设置超时，保持长连接
	}

	// 执行请求
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("执行请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("获取日志失败: %s, 响应: %s", resp.Status, string(body))
	}

	// 流式读取日志并发送到 WebSocket
	reader := bufio.NewReader(resp.Body)
	buffer := make([]byte, 4096)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 goroutine 监听 WebSocket 关闭
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, _, err := ws.ReadMessage()
				if err != nil {
					cancel()
					// 关闭 HTTP 响应，停止读取日志
					resp.Body.Close()
					return
				}
			}
		}
	}()

	// 读取并发送日志
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			n, err := reader.Read(buffer)
			if n > 0 {
				// 将数据发送到 WebSocket
				// 日志数据按行发送，保持格式
				data := buffer[:n]
				if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
					cancel()
					return fmt.Errorf("发送日志到 WebSocket 失败: %v", err)
				}
			}
			if err == io.EOF {
				if !follow {
					// 非 follow 模式，读取完成后退出
					return nil
				}
				// follow 模式，等待新数据
				// 检查连接是否仍然有效
				select {
				case <-ctx.Done():
					return nil
				default:
					time.Sleep(100 * time.Millisecond)
					continue
				}
			}
			if err != nil {
				cancel()
				return fmt.Errorf("读取日志失败: %v", err)
			}
		}
	}
}

