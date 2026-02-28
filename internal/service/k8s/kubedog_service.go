package k8s

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fisker086/keyops/pkg/config"
	"github.com/fisker086/keyops/pkg/logger"
)

// KubeDogService kubedog 服务
type KubeDogService struct {
	kubedogURL   string
	kubedogPath  string
	downloadPath string
}

// NewKubeDogService 创建 kubedog 服务
func NewKubeDogService(cfg *config.Config) *KubeDogService {
	// 默认 kubedog 下载地址（可以从配置中读取）
	kubedogURL := os.Getenv("KUBEDOG_URL")
	if kubedogURL == "" {
		kubedogURL = "https://github.com/flant/kubedog/releases/download/v0.11.0/kubedog-linux-amd64"
	}

	// kubedog 可执行文件路径
	kubedogPath := "/tmp/kubedog"
	downloadPath := "/tmp"

	return &KubeDogService{
		kubedogURL:   kubedogURL,
		kubedogPath:  kubedogPath,
		downloadPath: downloadPath,
	}
}

// RunKubeDog 运行 kubedog 监听部署状态
// deployID: 部署记录ID
// clusterHost: K8s 集群地址
// clusterToken: K8s 集群 Token
// kind: K8s 资源类型 (Deployment, StatefulSet, DaemonSet等)
// resourceName: 资源名称
// namespace: 命名空间
// timeout: 超时时间（秒）
// logPath: 日志文件路径（可选）
func (s *KubeDogService) RunKubeDog(
	deployID string,
	clusterHost string,
	clusterToken string,
	kind string,
	resourceName string,
	namespace string,
	timeout int,
	logPath string,
) error {
	// 生成 kubeconfig base64
	kubeConfigBase64, err := s.generateKubeConfig(clusterHost, clusterToken)
	if err != nil {
		return fmt.Errorf("生成 kubeconfig 失败: %v", err)
	}

	// 确保 kubedog 可执行文件存在
	if err := s.ensureKubeDogExists(); err != nil {
		return fmt.Errorf("确保 kubedog 存在失败: %v", err)
	}

	// 构建 kubedog 命令
	timeoutStr := fmt.Sprintf("%ds", timeout)
	cmd := exec.Command(s.kubedogPath, "rollout", "track", strings.ToLower(kind), resourceName, "-n", namespace, "-t", timeoutStr)
	cmd.Env = []string{"KUBEDOG_KUBE_CONFIG_BASE64=" + kubeConfigBase64}

	// 设置日志文件路径
	if logPath == "" {
		logPath = filepath.Join("/tmp", deployID+".log")
	}

	// 记录开始信息
	logContent := fmt.Sprintf("\nStarting App %s Status Verification:\n", resourceName)
	if err := s.appendLog(logPath, logContent); err != nil {
		logger.Warnf("写入日志失败: %v", err)
	}

	logContent = fmt.Sprintf("kubedog command: %s\n", cmd.String())
	if err := s.appendLog(logPath, logContent); err != nil {
		logger.Warnf("写入日志失败: %v", err)
	}

	logger.Infof("kubedog going, kubedog command: [%s]", cmd.String())

	// 设置 stdout 和 stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建 stdout pipe 失败: %v", err)
	}
	cmd.Stderr = cmd.Stdout

	// 启动命令
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 kubedog 失败: %v", err)
	}

	// 读取输出并写入日志
	buffer := make([]byte, 1024)
	for {
		n, err := stdout.Read(buffer)
		if n > 0 {
			// 替换 null 字节为空格
			cleanBytes := bytes.Replace(buffer[:n], []byte{0}, []byte{32}, -1)
			logContent := strings.TrimSpace(string(cleanBytes))
			if logContent != "" {
				if err := s.appendLog(logPath, logContent+"\n"); err != nil {
					logger.Warnf("写入日志失败: %v", err)
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Warnf("读取 kubedog 输出失败: %v", err)
			break
		}
	}

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		errorLog := fmt.Sprintf("\nkubedog verification failed: %v\n", err)
		if err := s.appendLog(logPath, errorLog); err != nil {
			logger.Warnf("写入日志失败: %v", err)
		}
		return fmt.Errorf("kubedog 验证失败: %v", err)
	}

	successLog := fmt.Sprintf("\nApp %s Status Verification completed successfully\n", resourceName)
	if err := s.appendLog(logPath, successLog); err != nil {
		logger.Warnf("写入日志失败: %v", err)
	}

	return nil
}

// generateKubeConfig 生成 kubeconfig base64 编码
// 使用简单的 YAML 格式生成 kubeconfig，避免依赖 k8s.io/client-go
func (s *KubeDogService) generateKubeConfig(host, token string) (string, error) {
	// 生成简单的 kubeconfig YAML
	kubeConfigYAML := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: %s
    insecure-skip-tls-verify: true
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: kubernetes-admin
  name: kubernetes
current-context: kubernetes
users:
- name: kubernetes-admin
  user:
    token: %s
`, host, token)

	// 编码为 base64
	return base64.StdEncoding.EncodeToString([]byte(kubeConfigYAML)), nil
}

// ensureKubeDogExists 确保 kubedog 可执行文件存在
func (s *KubeDogService) ensureKubeDogExists() error {
	// 检查文件是否存在且可执行
	if info, err := os.Stat(s.kubedogPath); err == nil {
		// 检查是否可执行
		if info.Mode().Perm()&0111 != 0 {
			return nil
		}
		// 文件存在但不可执行，设置权限
		if err := os.Chmod(s.kubedogPath, 0755); err != nil {
			return fmt.Errorf("设置 kubedog 权限失败: %v", err)
		}
		return nil
	}

	// 文件不存在，下载
	logger.Infof("kubedog 不存在，开始下载: %s", s.kubedogURL)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", s.kubedogURL, nil)
	if err != nil {
		return fmt.Errorf("创建下载请求失败: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载 kubedog 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载 kubedog 失败，状态码: %d", resp.StatusCode)
	}

	// 创建文件
	file, err := os.Create(s.kubedogPath)
	if err != nil {
		return fmt.Errorf("创建 kubedog 文件失败: %v", err)
	}
	defer file.Close()

	// 写入文件
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("写入 kubedog 文件失败: %v", err)
	}

	// 设置可执行权限
	if err := os.Chmod(s.kubedogPath, 0755); err != nil {
		return fmt.Errorf("设置 kubedog 权限失败: %v", err)
	}

	logger.Infof("kubedog 下载成功: %s", s.kubedogPath)
	return nil
}

// appendLog 追加日志到文件
func (s *KubeDogService) appendLog(logPath, content string) error {
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}
