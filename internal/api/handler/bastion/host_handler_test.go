package bastion

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fisker086/keyops/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// mockHostService implements HostService for testing
type mockHostService struct {
	checkDuplicateFunc func(ip string, port int, excludeID string) error
	createHostFunc     func(host *model.Host) error
	getHostFunc        func(id string) (*model.Host, error)
	updateHostFunc     func(id string, host *model.Host) error
	deleteHostFunc     func(id string) error
	listHostsFunc      func(page, pageSize int, search string, tags []string) ([]model.Host, int64, error)
	listByPermsFunc    func(page, pageSize int, search string, tags []string, userID string) ([]model.Host, int64, error)
}

func (m *mockHostService) CheckIPAndPortDuplicate(ip string, port int, excludeID string) error {
	if m.checkDuplicateFunc != nil {
		return m.checkDuplicateFunc(ip, port, excludeID)
	}
	return nil
}

func (m *mockHostService) CreateHost(host *model.Host) error {
	if m.createHostFunc != nil {
		return m.createHostFunc(host)
	}
	return nil
}

func (m *mockHostService) GetHost(id string) (*model.Host, error) {
	if m.getHostFunc != nil {
		return m.getHostFunc(id)
	}
	return nil, errors.New("not found")
}

func (m *mockHostService) UpdateHost(id string, host *model.Host) error {
	if m.updateHostFunc != nil {
		return m.updateHostFunc(id, host)
	}
	return nil
}

func (m *mockHostService) DeleteHost(id string) error {
	if m.deleteHostFunc != nil {
		return m.deleteHostFunc(id)
	}
	return nil
}

func (m *mockHostService) ListHosts(page, pageSize int, search string, tags []string) ([]model.Host, int64, error) {
	if m.listHostsFunc != nil {
		return m.listHostsFunc(page, pageSize, search, tags)
	}
	return nil, 0, nil
}

func (m *mockHostService) ListHostsByPermissions(page, pageSize int, search string, tags []string, userID string) ([]model.Host, int64, error) {
	if m.listByPermsFunc != nil {
		return m.listByPermsFunc(page, pageSize, search, tags, userID)
	}
	return nil, 0, nil
}

func setupTest() (*gin.Engine, *mockHostService) {
	gin.SetMode(gin.TestMode)
	mockSvc := &mockHostService{}
	handler := NewHostHandler(mockSvc)
	r := gin.New()
	r.POST("/api/v1/host", handler.CreateHost)
	return r, mockSvc
}

func parseResponse(t *testing.T, body []byte) model.Response {
	var resp model.Response
	err := json.Unmarshal(body, &resp)
	assert.NoError(t, err)
	return resp
}

func TestCreateHost(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		mockDuplicate  func(ip string, port int, excludeID string) error
		mockCreate     func(host *model.Host) error
		expectedCode   int
		expectedStatus int
		checkResponse  func(t *testing.T, resp model.Response)
	}{
		{
			name:           "successful creation",
			requestBody:    `{"name":"test-host","ip":"192.168.1.1","port":22,"os":"linux"}`,
			expectedCode:   0,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp model.Response) {
				data, _ := json.Marshal(resp.Data)
				var host model.Host
				err := json.Unmarshal(data, &host)
				assert.NoError(t, err)
				assert.Equal(t, "test-host", host.Name)
				assert.Equal(t, "192.168.1.1", host.IP)
				assert.Equal(t, 22, host.Port)
				assert.Equal(t, "unknown", host.Status)    // default status override
				assert.Equal(t, "server", host.DeviceType) // default device type
			},
		},
		{
			name:           "default port when port is zero",
			requestBody:    `{"name":"test-host","ip":"10.0.0.1","port":0,"os":"linux"}`,
			expectedCode:   0,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp model.Response) {
				data, _ := json.Marshal(resp.Data)
				var host model.Host
				json.Unmarshal(data, &host)
				assert.Equal(t, 22, host.Port, "port should default to 22")
			},
		},
		{
			name:           "default device type when empty",
			requestBody:    `{"name":"test-host","ip":"10.0.0.2","deviceType":""}`,
			expectedCode:   0,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp model.Response) {
				data, _ := json.Marshal(resp.Data)
				var host model.Host
				json.Unmarshal(data, &host)
				assert.Equal(t, "server", host.DeviceType, "deviceType should default to server")
			},
		},
		{
			name:           "invalid JSON body",
			requestBody:    `{bad json`,
			expectedCode:   400,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Contains(t, resp.Message, "invalid character")
			},
		},
		{
			name:        "duplicate IP and port",
			requestBody: `{"name":"dup-host","ip":"192.168.1.1","port":22}`,
			mockDuplicate: func(ip string, port int, excludeID string) error {
				return errors.New("IP地址 192.168.1.1 和端口 22 的组合已被主机 'other' 使用")
			},
			expectedCode:   400,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Contains(t, resp.Message, "已被主机")
			},
		},
		{
			name:           "invalid device type",
			requestBody:    `{"name":"bad-device","ip":"10.0.0.3","deviceType":"invalid_type"}`,
			expectedCode:   400,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Contains(t, resp.Message, "invalid device type")
			},
		},
		{
			name:        "service create fails",
			requestBody: `{"name":"fail-host","ip":"10.0.0.4","port":22,"os":"linux"}`,
			mockCreate: func(host *model.Host) error {
				return errors.New("database error")
			},
			expectedCode:   500,
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Contains(t, resp.Message, "database error")
			},
		},
		{
			name:           "missing name",
			requestBody:    `{"ip":"10.0.0.5","port":22}`,
			expectedCode:   0,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp model.Response) {
				data, _ := json.Marshal(resp.Data)
				var host model.Host
				json.Unmarshal(data, &host)
				assert.Equal(t, "10.0.0.5", host.IP)
				assert.Equal(t, "unknown", host.Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockSvc := setupTest()
			mockSvc.checkDuplicateFunc = tt.mockDuplicate
			mockSvc.createHostFunc = tt.mockCreate

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/host",
				strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			resp := parseResponse(t, w.Body.Bytes())
			assert.Equal(t, tt.expectedCode, resp.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}
