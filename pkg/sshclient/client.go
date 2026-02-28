package sshclient

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/fisker086/keyops/pkg/logger"
	"golang.org/x/crypto/ssh"
)

type SSHClient struct {
	client  *ssh.Client
	session *ssh.Session
}

type SSHConfig struct {
	Host       string
	Port       int
	Username   string
	Password   string
	PrivateKey string
	Passphrase string // 私钥密码
	AuthType   string // 认证类型: "password"、"key" 或 "both"（同时支持密码和密钥）
	Timeout    time.Duration
}

func NewSSHClient(cfg SSHConfig) (*SSHClient, error) {
	var authMethods []ssh.AuthMethod

	// 根据认证类型配置认证方法
	switch cfg.AuthType {
	case "key":
		// 只使用密钥认证
		if cfg.PrivateKey == "" {
			return nil, fmt.Errorf("private key is required for key authentication")
		}

		var signer ssh.Signer
		var err error

		if cfg.Passphrase != "" {
			// 带密码的私钥
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(cfg.PrivateKey), []byte(cfg.Passphrase))
		} else {
			// 无密码的私钥
			signer, err = ssh.ParsePrivateKey([]byte(cfg.PrivateKey))
		}

		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))

	case "password":
		// 只使用密码认证
		if cfg.Password == "" {
			return nil, fmt.Errorf("password is required for password authentication")
		}
		authMethods = append(authMethods, ssh.Password(cfg.Password))

	case "both":
		// 同时支持密码和密钥认证（优先尝试密钥，然后密码）
		// 先添加密钥认证（如果提供）
		if cfg.PrivateKey != "" {
			var signer ssh.Signer
			var err error

			if cfg.Passphrase != "" {
				signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(cfg.PrivateKey), []byte(cfg.Passphrase))
			} else {
				signer, err = ssh.ParsePrivateKey([]byte(cfg.PrivateKey))
			}

			if err != nil {
				logger.Warnf("[SSHClient] failed to parse private key, will try password only: %v", err)
			} else {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}

		// 再添加密码认证（如果提供）
		if cfg.Password != "" {
			authMethods = append(authMethods, ssh.Password(cfg.Password))
		}

		if len(authMethods) == 0 {
			return nil, fmt.Errorf("both authentication type requires at least password or private key")
		}

	default:
		// 兼容旧代码：如果没有指定 AuthType 或为空，根据提供的认证信息自动判断
		// 如果两者都提供，优先使用私钥，但也支持同时使用
		if cfg.PrivateKey != "" {
			var signer ssh.Signer
			var err error

			if cfg.Passphrase != "" {
				signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(cfg.PrivateKey), []byte(cfg.Passphrase))
			} else {
				signer, err = ssh.ParsePrivateKey([]byte(cfg.PrivateKey))
			}

			if err != nil {
				logger.Warnf("[SSHClient] failed to parse private key: %v", err)
			} else {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}

		// 如果提供了密码，也添加密码认证（支持同时使用）
		if cfg.Password != "" {
			authMethods = append(authMethods, ssh.Password(cfg.Password))
		}
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication method provided")
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 生产环境应该验证 host key
		Timeout:         cfg.Timeout,
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	// 记录认证方法详情（不记录敏感信息）
	authMethodNames := make([]string, 0)
	if cfg.PrivateKey != "" {
		authMethodNames = append(authMethodNames, "publickey")
	}
	if cfg.Password != "" {
		authMethodNames = append(authMethodNames, "password")
	}
	if len(authMethodNames) == 0 {
		authMethodNames = append(authMethodNames, "unknown")
	}
	logger.Infof("[SSHClient] Dialing %s as user '%s' with auth methods: [%s] (authType: %s), timeout: %v",
		addr, cfg.Username, strings.Join(authMethodNames, ", "), cfg.AuthType, cfg.Timeout)

	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		logger.Errorf("[SSHClient] Failed to dial %s as user '%s' with auth type '%s': %v",
			addr, cfg.Username, cfg.AuthType, err)
		return nil, fmt.Errorf("failed to dial %s as %s: %w", addr, cfg.Username, err)
	}
	logger.Infof("[SSHClient] Successfully connected to %s as user '%s'", addr, cfg.Username)

	return &SSHClient{client: client}, nil
}

func (c *SSHClient) NewSession() (*ssh.Session, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	c.session = session
	return session, nil
}

func (c *SSHClient) Close() error {
	if c.session != nil {
		c.session.Close()
	}
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func (c *SSHClient) Client() *ssh.Client {
	return c.client
}

// TestConnection 测试 SSH 连接
func TestConnection(cfg SSHConfig) error {
	client, err := NewSSHClient(cfg)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	// 执行简单命令测试
	_, err = session.CombinedOutput("echo test")
	return err
}

// GetHostKey 获取主机指纹
func GetHostKey(host string, port int) (string, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	sshConn, _, _, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: 5 * time.Second,
	})
	if sshConn != nil {
		sshConn.Close()
	}

	return "", nil
}
