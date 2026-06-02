package k8s

import (
	"bytes"
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

func init() {
	gin.SetMode(gin.TestMode)
}

// ------------------------------------------------------------
// Deployment tests
// ------------------------------------------------------------

func TestGetDeploymentList(t *testing.T) {
	tests := []struct {
		name            string
		query           string
		mockDeployments []*k8sService.Deployment
		mockErr         error
		wantStatus      int
		wantCode        int
	}{
		{
			name:            "success",
			query:           "?cluster_id=test-cluster&namespace=default",
			mockDeployments: []*k8sService.Deployment{{Name: "deploy-1", Namespace: "default"}},
			wantStatus:      http.StatusOK,
			wantCode:        0,
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
				GetDeploymentListFunc: func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Deployment, error) {
					return tt.mockDeployments, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetDeploymentList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetDeploymentRevisions(t *testing.T) {
	tests := []struct {
		name                string
		query               string
		params              []gin.Param
		mockRevisions       []*k8sService.DeploymentRevision
		mockCurrentRevision int64
		mockErr             error
		wantStatus          int
		wantCode            int
	}{
		{
			name:                "success",
			query:               "?cluster_id=test-cluster&namespace=default",
			params:              []gin.Param{{Key: "deployment_name", Value: "test-deployment"}},
			mockRevisions:       []*k8sService.DeploymentRevision{{Revision: 1}},
			mockCurrentRevision: 1,
			wantStatus:          http.StatusOK,
			wantCode:            0,
		},
		{
			name:       "missing params",
			query:      "?cluster_id=test-cluster",
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "deployment_name", Value: "test-deployment"}},
			mockErr:    errors.New("not found"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{}
			if tt.mockRevisions != nil || tt.mockErr != nil {
				mock.GetDeploymentRevisionsFunc = func(clusterID, clusterName, namespace, deploymentName string) ([]*k8sService.DeploymentRevision, int64, error) {
					return tt.mockRevisions, tt.mockCurrentRevision, tt.mockErr
				}
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)
			c.Params = tt.params

			h.GetDeploymentRevisions(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestRollbackDeployment(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		params     []gin.Param
		body       string
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "deployment_name", Value: "test-deployment"}},
			body:       `{"to_revision": 3}`,
			wantStatus: http.StatusOK,
			wantCode:   0,
		},
		{
			name:       "invalid JSON",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "deployment_name", Value: "test-deployment"}},
			body:       `{bad}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "missing params",
			query:      "?cluster_id=test-cluster",
			body:       `{"to_revision": 3}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "deployment_name", Value: "test-deployment"}},
			body:       `{"to_revision": 3}`,
			mockErr:    errors.New("rollback failed"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{}
			if tt.mockErr != nil {
				mock.RollbackDeploymentFunc = func(clusterID, clusterName, namespace, deploymentName string, toRevision int64) error {
					return tt.mockErr
				}
			}
			if tt.mockErr == nil && tt.body == `{"to_revision": 3}` && tt.params != nil {
				mock.RollbackDeploymentFunc = func(clusterID, clusterName, namespace, deploymentName string, toRevision int64) error {
					return nil
				}
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodPost, tt.query, bytes.NewBufferString(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Params = tt.params

			h.RollbackDeployment(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetDeploymentMetrics(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		params      []gin.Param
		mockMetrics interface{}
		mockErr     error
		wantStatus  int
		wantCode    int
	}{
		{
			name:        "success",
			query:       "?cluster_id=test-cluster&namespace=default&last_time=3600&step=300",
			params:      []gin.Param{{Key: "deployment_name", Value: "test-deployment"}},
			mockMetrics: map[string]interface{}{"cpu": 0.5},
			wantStatus:  http.StatusOK,
			wantCode:    0,
		},
		{
			name:       "missing params",
			query:      "?cluster_id=test-cluster",
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "deployment_name", Value: "test-deployment"}},
			mockErr:    errors.New("metrics error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{}
			if tt.mockMetrics != nil || tt.mockErr != nil {
				mock.GetDeploymentMetricsFunc = func(clusterID, clusterName, namespace, deploymentName string, lastTime, step uint) (interface{}, error) {
					return tt.mockMetrics, tt.mockErr
				}
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)
			c.Params = tt.params

			h.GetDeploymentMetrics(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetDeploymentDetail(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		params     []gin.Param
		mockDetail *k8sService.DeploymentDetail
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "deployment_name", Value: "test-deployment"}},
			mockDetail: &k8sService.DeploymentDetail{},
			wantStatus: http.StatusOK,
			wantCode:   0,
		},
		{
			name:       "missing namespace",
			query:      "?cluster_id=test-cluster",
			params:     []gin.Param{{Key: "deployment_name", Value: "test-deployment"}},
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "missing deployment_name",
			query:      "?cluster_id=test-cluster&namespace=default",
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "deployment_name", Value: "test-deployment"}},
			mockErr:    errors.New("not found"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{}
			if tt.mockDetail != nil || tt.mockErr != nil {
				mock.GetDeploymentDetailFunc = func(clusterID, clusterName, namespace, deploymentName string) (*k8sService.DeploymentDetail, error) {
					return tt.mockDetail, tt.mockErr
				}
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)
			c.Params = tt.params

			h.GetDeploymentDetail(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

// ------------------------------------------------------------
// DaemonSet tests
// ------------------------------------------------------------

func TestGetDaemonSetList(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		mockDaemonSets []*k8sService.DaemonSet
		mockErr        error
		wantStatus     int
		wantCode       int
	}{
		{
			name:           "success",
			query:          "?cluster_id=test-cluster&namespace=default",
			mockDaemonSets: []*k8sService.DaemonSet{{Name: "ds-1", Namespace: "default"}},
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
				GetDaemonSetListFunc: func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.DaemonSet, error) {
					return tt.mockDaemonSets, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetDaemonSetList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetDaemonSetMetrics(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		params      []gin.Param
		mockMetrics interface{}
		mockErr     error
		wantStatus  int
		wantCode    int
	}{
		{
			name:        "success",
			query:       "?cluster_id=test-cluster&namespace=default&last_time=3600&step=300",
			params:      []gin.Param{{Key: "daemonset_name", Value: "test-daemonset"}},
			mockMetrics: map[string]interface{}{"cpu": 0.5},
			wantStatus:  http.StatusOK,
			wantCode:    0,
		},
		{
			name:       "missing params",
			query:      "?cluster_id=test-cluster",
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "daemonset_name", Value: "test-daemonset"}},
			mockErr:    errors.New("metrics error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{}
			if tt.mockMetrics != nil || tt.mockErr != nil {
				mock.GetDaemonSetMetricsFunc = func(clusterID, clusterName, namespace, daemonSetName string, lastTime, step uint) (interface{}, error) {
					return tt.mockMetrics, tt.mockErr
				}
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)
			c.Params = tt.params

			h.GetDaemonSetMetrics(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetDaemonSetRevisions(t *testing.T) {
	tests := []struct {
		name                string
		query               string
		params              []gin.Param
		mockRevisions       []*k8sService.DaemonSetRevision
		mockCurrentRevision int64
		mockErr             error
		wantStatus          int
		wantCode            int
	}{
		{
			name:                "success",
			query:               "?cluster_id=test-cluster&namespace=default",
			params:              []gin.Param{{Key: "daemonset_name", Value: "test-daemonset"}},
			mockRevisions:       []*k8sService.DaemonSetRevision{{Revision: 1}},
			mockCurrentRevision: 1,
			wantStatus:          http.StatusOK,
			wantCode:            0,
		},
		{
			name:       "missing params",
			query:      "?cluster_id=test-cluster",
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "daemonset_name", Value: "test-daemonset"}},
			mockErr:    errors.New("not found"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{}
			if tt.mockRevisions != nil || tt.mockErr != nil {
				mock.GetDaemonSetRevisionsFunc = func(clusterID, clusterName, namespace, daemonSetName string) ([]*k8sService.DaemonSetRevision, int64, error) {
					return tt.mockRevisions, tt.mockCurrentRevision, tt.mockErr
				}
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)
			c.Params = tt.params

			h.GetDaemonSetRevisions(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestRollbackDaemonSet(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		params     []gin.Param
		body       string
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "daemonset_name", Value: "test-daemonset"}},
			body:       `{"to_revision": 3}`,
			wantStatus: http.StatusOK,
			wantCode:   0,
		},
		{
			name:       "invalid JSON",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "daemonset_name", Value: "test-daemonset"}},
			body:       `{bad}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "missing params",
			query:      "?cluster_id=test-cluster",
			body:       `{"to_revision": 3}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "daemonset_name", Value: "test-daemonset"}},
			body:       `{"to_revision": 3}`,
			mockErr:    errors.New("rollback failed"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{}
			if tt.mockErr != nil || (tt.params != nil && tt.body == `{"to_revision": 3}`) {
				mock.RollbackDaemonSetFunc = func(clusterID, clusterName, namespace, daemonSetName string, toRevision int64) error {
					return tt.mockErr
				}
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodPost, tt.query, bytes.NewBufferString(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Params = tt.params

			h.RollbackDaemonSet(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetDaemonSetDetail(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		params     []gin.Param
		mockDetail *k8sService.DeploymentDetail
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "daemonset_name", Value: "test-daemonset"}},
			mockDetail: &k8sService.DeploymentDetail{},
			wantStatus: http.StatusOK,
			wantCode:   0,
		},
		{
			name:       "missing namespace",
			query:      "?cluster_id=test-cluster",
			params:     []gin.Param{{Key: "daemonset_name", Value: "test-daemonset"}},
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "missing daemonset_name",
			query:      "?cluster_id=test-cluster&namespace=default",
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "daemonset_name", Value: "test-daemonset"}},
			mockErr:    errors.New("not found"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{}
			if tt.mockDetail != nil || tt.mockErr != nil {
				mock.GetDaemonSetDetailFunc = func(clusterID, clusterName, namespace, daemonSetName string) (*k8sService.DeploymentDetail, error) {
					return tt.mockDetail, tt.mockErr
				}
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)
			c.Params = tt.params

			h.GetDaemonSetDetail(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

// ------------------------------------------------------------
// StatefulSet tests
// ------------------------------------------------------------

func TestGetStatefulSetList(t *testing.T) {
	tests := []struct {
		name             string
		query            string
		mockStatefulSets []*k8sService.StatefulSet
		mockErr          error
		wantStatus       int
		wantCode         int
	}{
		{
			name:             "success",
			query:            "?cluster_id=test-cluster&namespace=default",
			mockStatefulSets: []*k8sService.StatefulSet{{Name: "ss-1", Namespace: "default"}},
			wantStatus:       http.StatusOK,
			wantCode:         0,
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
				GetStatefulSetListFunc: func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.StatefulSet, error) {
					return tt.mockStatefulSets, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetStatefulSetList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetStatefulSetDetail(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		params     []gin.Param
		mockDetail *k8sService.DeploymentDetail
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "statefulset_name", Value: "test-statefulset"}},
			mockDetail: &k8sService.DeploymentDetail{},
			wantStatus: http.StatusOK,
			wantCode:   0,
		},
		{
			name:       "missing namespace",
			query:      "?cluster_id=test-cluster",
			params:     []gin.Param{{Key: "statefulset_name", Value: "test-statefulset"}},
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "missing statefulset_name",
			query:      "?cluster_id=test-cluster&namespace=default",
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "statefulset_name", Value: "test-statefulset"}},
			mockErr:    errors.New("not found"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{}
			if tt.mockDetail != nil || tt.mockErr != nil {
				mock.GetStatefulSetDetailFunc = func(clusterID, clusterName, namespace, statefulSetName string) (*k8sService.DeploymentDetail, error) {
					return tt.mockDetail, tt.mockErr
				}
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)
			c.Params = tt.params

			h.GetStatefulSetDetail(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetStatefulSetMetrics(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		params      []gin.Param
		mockMetrics interface{}
		mockErr     error
		wantStatus  int
		wantCode    int
	}{
		{
			name:        "success",
			query:       "?cluster_id=test-cluster&namespace=default&last_time=3600&step=300",
			params:      []gin.Param{{Key: "statefulset_name", Value: "test-statefulset"}},
			mockMetrics: map[string]interface{}{"cpu": 0.5},
			wantStatus:  http.StatusOK,
			wantCode:    0,
		},
		{
			name:       "missing params",
			query:      "?cluster_id=test-cluster",
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "statefulset_name", Value: "test-statefulset"}},
			mockErr:    errors.New("metrics error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{}
			if tt.mockMetrics != nil || tt.mockErr != nil {
				mock.GetStatefulSetMetricsFunc = func(clusterID, clusterName, namespace, statefulSetName string, lastTime, step uint) (interface{}, error) {
					return tt.mockMetrics, tt.mockErr
				}
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)
			c.Params = tt.params

			h.GetStatefulSetMetrics(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetStatefulSetRevisions(t *testing.T) {
	tests := []struct {
		name                string
		query               string
		params              []gin.Param
		mockRevisions       []*k8sService.StatefulSetRevision
		mockCurrentRevision int64
		mockErr             error
		wantStatus          int
		wantCode            int
	}{
		{
			name:                "success",
			query:               "?cluster_id=test-cluster&namespace=default",
			params:              []gin.Param{{Key: "statefulset_name", Value: "test-statefulset"}},
			mockRevisions:       []*k8sService.StatefulSetRevision{{Revision: 1}},
			mockCurrentRevision: 1,
			wantStatus:          http.StatusOK,
			wantCode:            0,
		},
		{
			name:       "missing params",
			query:      "?cluster_id=test-cluster",
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "statefulset_name", Value: "test-statefulset"}},
			mockErr:    errors.New("not found"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{}
			if tt.mockRevisions != nil || tt.mockErr != nil {
				mock.GetStatefulSetRevisionsFunc = func(clusterID, clusterName, namespace, statefulSetName string) ([]*k8sService.StatefulSetRevision, int64, error) {
					return tt.mockRevisions, tt.mockCurrentRevision, tt.mockErr
				}
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)
			c.Params = tt.params

			h.GetStatefulSetRevisions(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestRollbackStatefulSet(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		params     []gin.Param
		body       string
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "statefulset_name", Value: "test-statefulset"}},
			body:       `{"to_revision": 3}`,
			wantStatus: http.StatusOK,
			wantCode:   0,
		},
		{
			name:       "invalid JSON",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "statefulset_name", Value: "test-statefulset"}},
			body:       `{bad}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "missing params",
			query:      "?cluster_id=test-cluster",
			body:       `{"to_revision": 3}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "statefulset_name", Value: "test-statefulset"}},
			body:       `{"to_revision": 3}`,
			mockErr:    errors.New("rollback failed"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{}
			if tt.mockErr != nil || (tt.params != nil && tt.body == `{"to_revision": 3}`) {
				mock.RollbackStatefulSetFunc = func(clusterID, clusterName, namespace, statefulSetName string, toRevision int64) error {
					return tt.mockErr
				}
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodPost, tt.query, bytes.NewBufferString(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Params = tt.params

			h.RollbackStatefulSet(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

// ------------------------------------------------------------
// CronJob / Job tests
// ------------------------------------------------------------

func TestGetCronJobList(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		mockCronJobs []*k8sService.CronJob
		mockErr      error
		wantStatus   int
		wantCode     int
	}{
		{
			name:         "success",
			query:        "?cluster_id=test-cluster&namespace=default",
			mockCronJobs: []*k8sService.CronJob{{Name: "cj-1", Namespace: "default"}},
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
				GetCronJobListFunc: func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.CronJob, error) {
					return tt.mockCronJobs, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetCronJobList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetCronJobDetail(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		params     []gin.Param
		mockDetail *k8sService.CronJobDetail
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "cronjob_name", Value: "test-cronjob"}},
			mockDetail: &k8sService.CronJobDetail{},
			wantStatus: http.StatusOK,
			wantCode:   0,
		},
		{
			name:       "missing params",
			query:      "?cluster_id=test-cluster",
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "cronjob_name", Value: "test-cronjob"}},
			mockErr:    errors.New("not found"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{}
			if tt.mockDetail != nil || tt.mockErr != nil {
				mock.GetCronJobDetailFunc = func(clusterID, clusterName, namespace, cronJobName string) (*k8sService.CronJobDetail, error) {
					return tt.mockDetail, tt.mockErr
				}
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)
			c.Params = tt.params

			h.GetCronJobDetail(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetJobList(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		mockJobs   []*k8sService.Job
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster&namespace=default",
			mockJobs:   []*k8sService.Job{{Name: "job-1", Namespace: "default"}},
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
				GetJobListFunc: func(clusterID, clusterName string, nodeID, envID uint, namespace string) ([]*k8sService.Job, error) {
					return tt.mockJobs, tt.mockErr
				},
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)

			h.GetJobList(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestGetJobDetail(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		params     []gin.Param
		mockDetail *k8sService.JobDetail
		mockErr    error
		wantStatus int
		wantCode   int
	}{
		{
			name:       "success",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "job_name", Value: "test-job"}},
			mockDetail: &k8sService.JobDetail{},
			wantStatus: http.StatusOK,
			wantCode:   0,
		},
		{
			name:       "missing params",
			query:      "?cluster_id=test-cluster",
			wantStatus: http.StatusBadRequest,
			wantCode:   400,
		},
		{
			name:       "service error",
			query:      "?cluster_id=test-cluster&namespace=default",
			params:     []gin.Param{{Key: "job_name", Value: "test-job"}},
			mockErr:    errors.New("not found"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockK8sService{}
			if tt.mockDetail != nil || tt.mockErr != nil {
				mock.GetJobDetailFunc = func(clusterID, clusterName, namespace, jobName string) (*k8sService.JobDetail, error) {
					return tt.mockDetail, tt.mockErr
				}
			}
			h := &K8sHandler{service: mock}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, tt.query, nil)
			c.Params = tt.params

			h.GetJobDetail(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp model.Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}
