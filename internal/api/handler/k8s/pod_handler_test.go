package k8s

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fisker086/keyops/internal/model"
	k8sService "github.com/fisker086/keyops/internal/service/k8s"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetPodList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      string
		mockPods   []*k8sService.Pod
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster&namespace=default",
			mockPods:   []*k8sService.Pod{{Name: "pod-1", Namespace: "default"}},
			wantStatus: http.StatusOK,
			wantCode:   0,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster",
			mockPods:   nil,
			mockErr:    errors.New("internal error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{
				GetPodListFunc: func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Pod, error) {
					return tt.mockPods, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetPodList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetPodDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      string
		mockDetail *k8sService.PodDetail
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster&namespace=default&pod_name=test-pod",
			mockDetail: &k8sService.PodDetail{},
			wantStatus: http.StatusOK,
			wantCode:   0,
		},
		{
			name:       "missing required params",
			query:      "?cluster_id=test-cluster",
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster&namespace=default&pod_name=test-pod",
			mockDetail: nil,
			mockErr:    errors.New("not found"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{
				GetPodDetailFunc: func(clusterID, clusterName, namespace, podName string) (*k8sService.PodDetail, error) {
					return tt.mockDetail, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetPodDetail(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetContainersList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		mockContainers []*k8sService.Container
		mockErr        error
		wantStatus     int
		wantCode       int
	}{
		{
			name:           "success",
			query:          "?cluster_id=test-cluster&namespace=default&pod_name=test-pod",
			mockContainers: []*k8sService.Container{{Name: "container-1"}},
			wantStatus:     http.StatusOK,
			wantCode:       0,
		},
		{
			name:       "missing pod_name",
			query:      "?cluster_id=test-cluster&namespace=default",
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster&namespace=default&pod_name=test-pod",
			mockErr:    errors.New("error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{
				GetContainersListFunc: func(clusterID, clusterName string, nodeID, envID uint, namespace, podName string) ([]*k8sService.Container, error) {
					return tt.mockContainers, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetContainersList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}
