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

func TestGetServiceList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		query        string
		mockServices []*k8sService.Service
		mockErr      error
		wantStatus   int
		wantCode     int
	}{
		{
			name:         "success",
			query:        "?cluster_id=test-cluster&namespace=default",
			mockServices: []*k8sService.Service{{Name: "svc-1", Namespace: "default"}},
			wantStatus:   http.StatusOK,
			wantCode:     0,
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
				GetServiceListFunc: func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Service, error) {
					return tt.mockServices, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetServiceList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetIngressList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		query         string
		mockIngresses []*k8sService.Ingress
		mockErr       error
		wantStatus    int
		wantCode      int
	}{
		{
			name:          "success",
			query:         "?cluster_id=test-cluster&namespace=default",
			mockIngresses: []*k8sService.Ingress{{Name: "ing-1", Namespace: "default"}},
			wantStatus:    http.StatusOK,
			wantCode:      0,
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
				GetIngressListFunc: func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Ingress, error) {
					return tt.mockIngresses, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetIngressList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetHPAList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      string
		mockHPAs   []*k8sService.HPA
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster&namespace=default",
			mockHPAs:   []*k8sService.HPA{{Name: "hpa-1", Namespace: "default"}},
			wantStatus: http.StatusOK,
			wantCode:   0,
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
				GetHPAListFunc: func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.HPA, error) {
					return tt.mockHPAs, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetHPAList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}
