package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// PV PersistentVolume 信息
type PV struct {
	Name          string   `json:"name"`
	Capacity      string   `json:"capacity"`
	AccessModes   []string `json:"accessModes"`
	ReclaimPolicy string   `json:"reclaimPolicy"`
	Status        string   `json:"status"`
	Claim         string   `json:"claim"`
	StorageClass  string   `json:"storageClass"`
	Age           string   `json:"age"`
}

// PVC PersistentVolumeClaim 信息
type PVC struct {
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	Status       string   `json:"status"`
	Volume       string   `json:"volume"`
	Capacity     string   `json:"capacity"`
	AccessModes  []string `json:"accessModes"`
	StorageClass string   `json:"storageClass"`
	Age          string   `json:"age"`
}

// StorageClass 存储类信息
type StorageClass struct {
	Name                string `json:"name"`
	Provisioner         string `json:"provisioner"`
	ReclaimPolicy       string `json:"reclaimPolicy"`
	VolumeBindingMode   string `json:"volumeBindingMode"`
	AllowVolumeExpansion bool   `json:"allowVolumeExpansion"`
	Age                 string `json:"age"`
}

// GetPVList 获取 PV 列表（集群级别资源）
func (s *K8sService) GetPVList(clusterID string, clusterName string, nodeID uint, envID uint) ([]*PV, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	pvsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/persistentvolumes"
	httpReq, client, err := s.createK8sHTTPClient(cluster, pvsURL)
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

	var pvListResponse struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				Capacity              map[string]string `json:"capacity"`
				AccessModes           []string          `json:"accessModes"`
				PersistentVolumeReclaimPolicy string `json:"persistentVolumeReclaimPolicy"`
				StorageClassName      string   `json:"storageClassName"`
				ClaimRef              *struct {
					Namespace string `json:"namespace"`
					Name      string `json:"name"`
				} `json:"claimRef"`
			} `json:"spec"`
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &pvListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	pvs := make([]*PV, 0, len(pvListResponse.Items))
	for _, item := range pvListResponse.Items {
		capacity := "-"
		if item.Spec.Capacity != nil {
			if c, ok := item.Spec.Capacity["storage"]; ok {
				capacity = c
			}
		}

		claim := "-"
		if item.Spec.ClaimRef != nil {
			claim = item.Spec.ClaimRef.Namespace + "/" + item.Spec.ClaimRef.Name
		}

		reclaimPolicy := item.Spec.PersistentVolumeReclaimPolicy
		if reclaimPolicy == "" {
			reclaimPolicy = "Retain"
		}

		accessModes := item.Spec.AccessModes
		if accessModes == nil {
			accessModes = []string{}
		}

		pvs = append(pvs, &PV{
			Name:          item.Metadata.Name,
			Capacity:     capacity,
			AccessModes:  accessModes,
			ReclaimPolicy: reclaimPolicy,
			Status:        item.Status.Phase,
			Claim:         claim,
			StorageClass:  item.Spec.StorageClassName,
			Age:           formatAge(item.Metadata.CreationTimestamp),
		})
	}

	return pvs, nil
}

// GetStorageClassList 获取 StorageClass 列表（集群级别资源）
func (s *K8sService) GetStorageClassList(clusterID string, clusterName string, nodeID uint, envID uint) ([]*StorageClass, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	scURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/storage.k8s.io/v1/storageclasses"
	httpReq, client, err := s.createK8sHTTPClient(cluster, scURL)
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

	var scListResponse struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Provisioner       string `json:"provisioner"`
			ReclaimPolicy     string `json:"reclaimPolicy"`
			VolumeBindingMode string `json:"volumeBindingMode"`
			AllowVolumeExpansion *bool `json:"allowVolumeExpansion"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &scListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	storageClasses := make([]*StorageClass, 0, len(scListResponse.Items))
	for _, item := range scListResponse.Items {
		reclaimPolicy := item.ReclaimPolicy
		if reclaimPolicy == "" {
			reclaimPolicy = "Delete"
		}

		volumeBindingMode := item.VolumeBindingMode
		if volumeBindingMode == "" {
			volumeBindingMode = "Immediate"
		}

		allowExpansion := false
		if item.AllowVolumeExpansion != nil {
			allowExpansion = *item.AllowVolumeExpansion
		}

		storageClasses = append(storageClasses, &StorageClass{
			Name:                item.Metadata.Name,
			Provisioner:        item.Provisioner,
			ReclaimPolicy:       reclaimPolicy,
			VolumeBindingMode:  volumeBindingMode,
			AllowVolumeExpansion: allowExpansion,
			Age:                formatAge(item.Metadata.CreationTimestamp),
		})
	}

	return storageClasses, nil
}

// GetPVCList 获取 PVC 列表（命名空间级别）
func (s *K8sService) GetPVCList(clusterID string, clusterName string, nodeID uint, envID uint, namespace string) ([]*PVC, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	ns := s.getNamespace(cluster, namespace)

	pvcsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/persistentvolumeclaims"
	httpReq, client, err := s.createK8sHTTPClient(cluster, pvcsURL)
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

	var pvcListResponse struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				VolumeName       string   `json:"volumeName"`
				StorageClassName *string  `json:"storageClassName"`
				AccessModes      []string `json:"accessModes"`
				Resources        struct {
					Requests map[string]string `json:"requests"`
				} `json:"resources"`
			} `json:"spec"`
			Status struct {
				Phase  string `json:"phase"`
				Capacity *map[string]string `json:"capacity"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &pvcListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	pvcs := make([]*PVC, 0, len(pvcListResponse.Items))
	for _, item := range pvcListResponse.Items {
		capacity := "-"
		if item.Status.Capacity != nil {
			if c, ok := (*item.Status.Capacity)["storage"]; ok {
				capacity = c
			}
		}
		if capacity == "-" && item.Spec.Resources.Requests != nil {
			if r, ok := item.Spec.Resources.Requests["storage"]; ok {
				capacity = r + " (requested)"
			}
		}

		volume := item.Spec.VolumeName
		if volume == "" {
			volume = "-"
		}

		storageClass := ""
		if item.Spec.StorageClassName != nil {
			storageClass = *item.Spec.StorageClassName
		}

		accessModes := item.Spec.AccessModes
		if accessModes == nil {
			accessModes = []string{}
		}

		pvcs = append(pvcs, &PVC{
			Name:         item.Metadata.Name,
			Namespace:    item.Metadata.Namespace,
			Status:       item.Status.Phase,
			Volume:       volume,
			Capacity:     capacity,
			AccessModes:  accessModes,
			StorageClass: storageClass,
			Age:          formatAge(item.Metadata.CreationTimestamp),
		})
	}

	return pvcs, nil
}
