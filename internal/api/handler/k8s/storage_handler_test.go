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

func TestGetPVList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      string
		mockPVs    []*k8sService.PV
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster",
			mockPVs:    []*k8sService.PV{{Name: "pv-1"}},
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
				GetPVListFunc: func(clusterID, clusterName string, nodeID, envID uint) ([]*k8sService.PV, error) {
					return tt.mockPVs, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetPVList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetStorageClassList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name               string
		query              string
		mockStorageClasses []*k8sService.StorageClass
		mockErr            error
		wantStatus         int
		wantCode           int
	}{
		{
			name:               "success",
			query:              "?cluster_id=test-cluster",
			mockStorageClasses: []*k8sService.StorageClass{{Name: "sc-1"}},
			wantStatus:         http.StatusOK,
			wantCode:           0,
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
				GetStorageClassListFunc: func(clusterID, clusterName string, nodeID, envID uint) ([]*k8sService.StorageClass, error) {
					return tt.mockStorageClasses, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetStorageClassList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetPVCList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      string
		mockPVCs   []*k8sService.PVC
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster&namespace=default",
			mockPVCs:   []*k8sService.PVC{{Name: "pvc-1", Namespace: "default"}},
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
				GetPVCListFunc: func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.PVC, error) {
					return tt.mockPVCs, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetPVCList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}
