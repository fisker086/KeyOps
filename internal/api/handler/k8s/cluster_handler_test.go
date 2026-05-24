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

func TestGetBaseInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      string
		mockInfo   *k8sService.BaseInfo
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster",
			mockInfo:   &k8sService.BaseInfo{Cluster: "test", PodCount: 10},
			wantStatus: http.StatusOK,
			wantCode:   0,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster",
			mockErr:    errors.New("error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{
				GetBaseInfoFunc: func(clusterID, clusterName string, nodeID, envID uint, namespace string) (*k8sService.BaseInfo, error) {
					return tt.mockInfo, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetBaseInfo(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetNodeList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      string
		mockNodes  []*k8sService.Node
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster",
			mockNodes:  []*k8sService.Node{{Name: "node-1"}},
			wantStatus: http.StatusOK,
			wantCode:   0,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster",
			mockErr:    errors.New("error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{
				GetNodeListFunc: func(clusterID, clusterName string, nodeID, envID uint) ([]*k8sService.Node, error) {
					return tt.mockNodes, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetNodeList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetEventList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      string
		mockEvents []*k8sService.Event
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster",
			mockEvents: []*k8sService.Event{{Type: "Normal"}},
			wantStatus: http.StatusOK,
			wantCode:   0,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster",
			mockErr:    errors.New("error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{
				GetEventListFunc: func(clusterID, clusterName string, nodeID, envID uint, namespace, objectName, objectKind string) ([]*k8sService.Event, error) {
					return tt.mockEvents, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetEventList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetNamespaceList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		mockNamespaces []*k8sService.Namespace
		mockErr        error
		wantStatus     int
		wantCode       int
	}{
		{
			name:           "success",
			query:          "?cluster_id=test-cluster",
			mockNamespaces: []*k8sService.Namespace{{Name: "default"}},
			wantStatus:     http.StatusOK,
			wantCode:       0,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster",
			mockErr:    errors.New("error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{
				GetNamespaceListFunc: func(clusterID, clusterName string) ([]*k8sService.Namespace, error) {
					return tt.mockNamespaces, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetNamespaceList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}
