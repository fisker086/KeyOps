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

func TestGetConfigMapList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		mockConfigMaps []*k8sService.ConfigMap
		mockErr        error
		wantStatus     int
		wantCode       int
	}{
		{
			name:           "success",
			query:          "?cluster_id=test-cluster&namespace=default",
			mockConfigMaps: []*k8sService.ConfigMap{{Name: "cm-1", Namespace: "default"}},
			wantStatus:     http.StatusOK,
			wantCode:       0,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster",
			mockErr:    errors.New("internal error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{
				GetConfigMapListFunc: func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.ConfigMap, error) {
					return tt.mockConfigMaps, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetConfigMapList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetSecretList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		query       string
		mockSecrets []*k8sService.Secret
		mockErr     error
		wantStatus  int
		wantCode    int
	}{
		{
			name:        "success",
			query:       "?cluster_id=test-cluster&namespace=default",
			mockSecrets: []*k8sService.Secret{{Name: "secret-1", Namespace: "default"}},
			wantStatus:  http.StatusOK,
			wantCode:    0,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster",
			mockErr:    errors.New("internal error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{
				GetSecretListFunc: func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Secret, error) {
					return tt.mockSecrets, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetSecretList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}
