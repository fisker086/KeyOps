package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// GetIngressList 获取 Ingress 列表
func (s *K8sService) GetIngressList(clusterID string, clusterName string, nodeID uint, envID uint, namespace string) ([]*Ingress, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	ns := s.getNamespace(cluster, namespace)

	ingressURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/networking.k8s.io/v1/namespaces/" + ns + "/ingresses"
	httpReq, client, err := s.createK8sHTTPClient(cluster, ingressURL)
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

	var ingressListResponse struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				Rules []struct {
					Host string `json:"host"`
				} `json:"rules"`
				TLS []struct {
					Hosts []string `json:"hosts"`
				} `json:"tls"`
			} `json:"spec"`
			Status struct {
				LoadBalancer struct {
					Ingress []struct {
						IP       string `json:"ip"`
						Hostname string `json:"hostname"`
					} `json:"ingress"`
				} `json:"loadBalancer"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &ingressListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	ingresses := make([]*Ingress, 0, len(ingressListResponse.Items))
	for _, item := range ingressListResponse.Items {
		ingress := &Ingress{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Age:       formatAge(item.Metadata.CreationTimestamp),
		}

		// 提取主机：从 spec.rules[].host 和 spec.tls[].hosts 合并（去重）
		hostSet := make(map[string]bool)
		for _, rule := range item.Spec.Rules {
			if rule.Host != "" {
				hostSet[rule.Host] = true
			}
		}
		for _, tls := range item.Spec.TLS {
			for _, h := range tls.Hosts {
				if h != "" {
					hostSet[h] = true
				}
			}
		}
		var hosts []string
		for h := range hostSet {
			hosts = append(hosts, h)
		}
		if len(hosts) == 0 && len(item.Spec.Rules) > 0 {
			// 无 host 的 rule 表示匹配任意 host（路径匹配）
			hosts = []string{"*"}
		}
		sort.Strings(hosts)
		ingress.Hosts = strings.Join(hosts, ",")

		// 提取地址：优先 status.loadBalancer，为空时用 hosts 作为访问入口
		var addresses []string
		for _, lb := range item.Status.LoadBalancer.Ingress {
			if lb.IP != "" {
				addresses = append(addresses, lb.IP)
			} else if lb.Hostname != "" {
				addresses = append(addresses, lb.Hostname)
			}
		}
		if len(addresses) > 0 {
			ingress.Address = strings.Join(addresses, ",")
		} else if ingress.Hosts != "" {
			ingress.Address = ingress.Hosts
		} else {
			ingress.Address = "-"
		}

		ingresses = append(ingresses, ingress)
	}

	return ingresses, nil
}

// GetIngressesForServiceNames 获取引用了指定 Service 的 Ingress 列表（用于工作负载详情的关联资源展示）
func (s *K8sService) GetIngressesForServiceNames(clusterID string, clusterName string, nodeID uint, envID uint, namespace string, serviceNames []string) ([]*Ingress, error) {
	if len(serviceNames) == 0 {
		return nil, nil
	}
	serviceNameSet := make(map[string]bool)
	for _, n := range serviceNames {
		serviceNameSet[n] = true
	}

	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}
	ns := s.getNamespace(cluster, namespace)

	ingressURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/networking.k8s.io/v1/namespaces/" + ns + "/ingresses"
	httpReq, client, err := s.createK8sHTTPClient(cluster, ingressURL)
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

	var ingressListResponse struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				DefaultBackend *struct {
					Service *struct {
						Name string `json:"name"`
					} `json:"service"`
				} `json:"defaultBackend"`
				Rules []struct {
					Host string `json:"host"`
					HTTP *struct {
						Paths []struct {
							Backend struct {
								Service *struct {
									Name string `json:"name"`
								} `json:"service"`
							} `json:"backend"`
						} `json:"paths"`
					} `json:"http"`
				} `json:"rules"`
				TLS []struct {
					Hosts []string `json:"hosts"`
				} `json:"tls"`
			} `json:"spec"`
			Status struct {
				LoadBalancer struct {
					Ingress []struct {
						IP       string `json:"ip"`
						Hostname string `json:"hostname"`
					} `json:"ingress"`
				} `json:"loadBalancer"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &ingressListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	ingresses := make([]*Ingress, 0)
	for _, item := range ingressListResponse.Items {
		referencesOurService := false
		if item.Spec.DefaultBackend != nil && item.Spec.DefaultBackend.Service != nil &&
			serviceNameSet[item.Spec.DefaultBackend.Service.Name] {
			referencesOurService = true
		}
		for _, rule := range item.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}
			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service != nil && serviceNameSet[path.Backend.Service.Name] {
					referencesOurService = true
					break
				}
			}
			if referencesOurService {
				break
			}
		}
		if !referencesOurService {
			continue
		}
		ingress := &Ingress{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Age:       formatAge(item.Metadata.CreationTimestamp),
		}
		// 提取主机：从 spec.rules[].host 和 spec.tls[].hosts 合并（去重）
		hostSet := make(map[string]bool)
		for _, rule := range item.Spec.Rules {
			if rule.Host != "" {
				hostSet[rule.Host] = true
			}
		}
		for _, tls := range item.Spec.TLS {
			for _, h := range tls.Hosts {
				if h != "" {
					hostSet[h] = true
				}
			}
		}
		var hosts []string
		for h := range hostSet {
			hosts = append(hosts, h)
		}
		if len(hosts) == 0 && len(item.Spec.Rules) > 0 {
			// 无 host 的 rule 表示匹配任意 host（路径匹配）
			hosts = []string{"*"}
		}
		sort.Strings(hosts)
		ingress.Hosts = strings.Join(hosts, ",")

		// 提取地址：优先 status.loadBalancer，为空时用 hosts 作为访问入口
		var addresses []string
		for _, lb := range item.Status.LoadBalancer.Ingress {
			if lb.IP != "" {
				addresses = append(addresses, lb.IP)
			} else if lb.Hostname != "" {
				addresses = append(addresses, lb.Hostname)
			}
		}
		if len(addresses) > 0 {
			ingress.Address = strings.Join(addresses, ",")
		} else if ingress.Hosts != "" {
			ingress.Address = ingress.Hosts
		} else {
			ingress.Address = "-"
		}
		ingresses = append(ingresses, ingress)
	}
	return ingresses, nil
}
