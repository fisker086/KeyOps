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

type mockSessionService struct {
	createSessionFunc          func(hostID string, userID string) (*model.SessionResponse, error)
	getLoginRecordsByUserFunc  func(page, pageSize int, hostID, userID string) ([]model.LoginRecordWithType, int64, error)
	getSessionRecordingsFunc   func(page, pageSize int, search string) ([]model.SessionRecording, int64, error)
	getSessionRecordingFunc    func(sessionID string) (*model.SessionRecording, error)
	createSessionRecordingFunc func(recording *model.SessionRecording) error
	getCommandRecordsFunc      func(page, pageSize int, search, hostFilter string) ([]model.CommandRecord, int64, error)
	createCommandRecordFunc    func(record *model.CommandRecord) error
	getCommandsBySessionFunc   func(sessionID string) ([]model.CommandRecord, error)
	terminateSessionFunc       func(sessionID string) error
}

func (m *mockSessionService) CreateSession(hostID string, userID string) (*model.SessionResponse, error) {
	if m.createSessionFunc != nil {
		return m.createSessionFunc(hostID, userID)
	}
	return nil, nil
}

func (m *mockSessionService) GetLoginRecordsByUser(page, pageSize int, hostID, userID string) ([]model.LoginRecordWithType, int64, error) {
	if m.getLoginRecordsByUserFunc != nil {
		return m.getLoginRecordsByUserFunc(page, pageSize, hostID, userID)
	}
	return nil, 0, nil
}

func (m *mockSessionService) GetSessionRecordings(page, pageSize int, search string) ([]model.SessionRecording, int64, error) {
	if m.getSessionRecordingsFunc != nil {
		return m.getSessionRecordingsFunc(page, pageSize, search)
	}
	return nil, 0, nil
}

func (m *mockSessionService) GetSessionRecording(sessionID string) (*model.SessionRecording, error) {
	if m.getSessionRecordingFunc != nil {
		return m.getSessionRecordingFunc(sessionID)
	}
	return nil, nil
}

func (m *mockSessionService) CreateSessionRecording(recording *model.SessionRecording) error {
	if m.createSessionRecordingFunc != nil {
		return m.createSessionRecordingFunc(recording)
	}
	return nil
}

func (m *mockSessionService) GetCommandRecords(page, pageSize int, search, hostFilter string) ([]model.CommandRecord, int64, error) {
	if m.getCommandRecordsFunc != nil {
		return m.getCommandRecordsFunc(page, pageSize, search, hostFilter)
	}
	return nil, 0, nil
}

func (m *mockSessionService) CreateCommandRecord(record *model.CommandRecord) error {
	if m.createCommandRecordFunc != nil {
		return m.createCommandRecordFunc(record)
	}
	return nil
}

func (m *mockSessionService) GetCommandsBySession(sessionID string) ([]model.CommandRecord, error) {
	if m.getCommandsBySessionFunc != nil {
		return m.getCommandsBySessionFunc(sessionID)
	}
	return nil, nil
}

func (m *mockSessionService) TerminateSession(sessionID string) error {
	if m.terminateSessionFunc != nil {
		return m.terminateSessionFunc(sessionID)
	}
	return nil
}

func setupSessionTest() (*gin.Engine, *mockSessionService, *SessionHandler) {
	gin.SetMode(gin.TestMode)
	mockSvc := &mockSessionService{}
	handler := NewSessionHandler(mockSvc)
	r := gin.New()
	return r, mockSvc, handler
}

func TestCreateSession(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		mockCreate     func(hostID, userID string) (*model.SessionResponse, error)
		expectedStatus int
		expectedCode   int
		checkResponse  func(t *testing.T, resp model.Response)
		userID         string
	}{
		{
			name:        "success",
			requestBody: `{"hostId":"host-1"}`,
		mockCreate: func(hostID, userID string) (*model.SessionResponse, error) {
			return &model.SessionResponse{
				SessionID: "session-1",
			}, nil
		},
		expectedStatus: http.StatusOK,
		expectedCode:   0,
		checkResponse: func(t *testing.T, resp model.Response) {
			data, _ := json.Marshal(resp.Data)
			var s model.SessionResponse
			err := json.Unmarshal(data, &s)
			assert.NoError(t, err)
			assert.Equal(t, "session-1", s.SessionID)
		},
			userID: "user-1",
		},
		{
			name:           "bad request - invalid JSON",
			requestBody:    `{bad json`,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   400,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Contains(t, resp.Message, "invalid character")
			},
		},
		{
			name:           "unauthorized - no userID",
			requestBody:    `{"hostId":"host-1"}`,
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   401,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Contains(t, resp.Message, "未找到用户信息")
			},
		},
		{
			name:        "service error",
			requestBody: `{"hostId":"host-1"}`,
			mockCreate: func(hostID, userID string) (*model.SessionResponse, error) {
				return nil, errors.New("internal error")
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   500,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Contains(t, resp.Message, "internal error")
			},
			userID: "user-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockSvc, handler := setupSessionTest()
			mockSvc.createSessionFunc = tt.mockCreate

			if tt.userID != "" {
				r.Use(func(c *gin.Context) {
					c.Set("userID", tt.userID)
				})
			}
			r.POST("/api/v1/session", handler.CreateSession)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/session",
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

func TestGetLoginRecords(t *testing.T) {
	tests := []struct {
		name             string
		mockRecords      func(page, pageSize int, hostID, userID string) ([]model.LoginRecordWithType, int64, error)
		expectedStatus   int
		expectedCode     int
	}{
		{
			name: "success",
			mockRecords: func(page, pageSize int, hostID, userID string) ([]model.LoginRecordWithType, int64, error) {
				return []model.LoginRecordWithType{
					{LoginRecord: model.LoginRecord{ID: "rec-1", HostID: "host-1"}},
				}, 1, nil
			},
			expectedStatus: http.StatusOK,
			expectedCode:   0,
		},
		{
			name: "service error",
			mockRecords: func(page, pageSize int, hostID, userID string) ([]model.LoginRecordWithType, int64, error) {
				return nil, 0, errors.New("db error")
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockSvc, handler := setupSessionTest()
			mockSvc.getLoginRecordsByUserFunc = tt.mockRecords

			r.Use(func(c *gin.Context) {
				c.Set("userID", "user-1")
			})
			r.GET("/api/v1/session/login-records", handler.GetLoginRecords)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/session/login-records", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			resp := parseResponse(t, w.Body.Bytes())
			assert.Equal(t, tt.expectedCode, resp.Code)
		})
	}
}

func TestGetSessionRecordings(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		mockRecordings func(page, pageSize int, search string) ([]model.SessionRecording, int64, error)
		expectedStatus int
		expectedCode   int
	}{
		{
			name: "success - admin role",
			role: "admin",
			mockRecordings: func(page, pageSize int, search string) ([]model.SessionRecording, int64, error) {
				return []model.SessionRecording{
					{SessionID: "sess-1", HostID: "host-1"},
				}, 1, nil
			},
			expectedStatus: http.StatusOK,
			expectedCode:   0,
		},
		{
			name:           "forbidden - non-admin role",
			role:           "user",
			expectedStatus: http.StatusForbidden,
			expectedCode:   403,
		},
		{
			name: "service error",
			role: "admin",
			mockRecordings: func(page, pageSize int, search string) ([]model.SessionRecording, int64, error) {
				return nil, 0, errors.New("db error")
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockSvc, handler := setupSessionTest()
			mockSvc.getSessionRecordingsFunc = tt.mockRecordings

			r.Use(func(c *gin.Context) {
				c.Set("role", tt.role)
			})
			r.GET("/api/v1/session/recordings", handler.GetSessionRecordings)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/session/recordings", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			resp := parseResponse(t, w.Body.Bytes())
			assert.Equal(t, tt.expectedCode, resp.Code)
		})
	}
}

func TestGetSessionRecording(t *testing.T) {
	tests := []struct {
		name           string
		sessionID      string
		mockRecording  func(id string) (*model.SessionRecording, error)
		expectedStatus int
		expectedCode   int
	}{
		{
			name:      "success",
			sessionID: "sess-1",
			mockRecording: func(id string) (*model.SessionRecording, error) {
				return &model.SessionRecording{
					SessionID:      "sess-1",
					ConnectionType: "ssh",
				}, nil
			},
			expectedStatus: http.StatusOK,
			expectedCode:   0,
		},
		{
			name:      "not found - nil recording",
			sessionID: "sess-missing",
			mockRecording: func(id string) (*model.SessionRecording, error) {
				return nil, nil
			},
			expectedStatus: http.StatusNotFound,
			expectedCode:   404,
		},
		{
			name:      "service error",
			sessionID: "sess-err",
			mockRecording: func(id string) (*model.SessionRecording, error) {
				return nil, errors.New("not found")
			},
			expectedStatus: http.StatusNotFound,
			expectedCode:   404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockSvc, handler := setupSessionTest()
			mockSvc.getSessionRecordingFunc = tt.mockRecording

			r.GET("/api/v1/session/recordings/:sessionId", handler.GetSessionRecording)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/session/recordings/"+tt.sessionID, nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			resp := parseResponse(t, w.Body.Bytes())
			assert.Equal(t, tt.expectedCode, resp.Code)
		})
	}
}

func TestCreateCommandRecord(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		mockCreate     func(record *model.CommandRecord) error
		expectedStatus int
		expectedCode   int
		checkResponse  func(t *testing.T, resp model.Response)
	}{
		{
			name:        "success",
			requestBody: `{"sessionId":"sess-1","command":"ls -la"}`,
			mockCreate: func(record *model.CommandRecord) error {
				return nil
			},
			expectedStatus: http.StatusOK,
			expectedCode:   0,
			checkResponse: func(t *testing.T, resp model.Response) {
				data, _ := json.Marshal(resp.Data)
				var r model.CommandRecord
				err := json.Unmarshal(data, &r)
				assert.NoError(t, err)
				assert.Equal(t, "sess-1", r.SessionID)
				assert.Equal(t, "ls -la", r.Command)
			},
		},
		{
			name:           "bad request - invalid JSON",
			requestBody:    `{bad json`,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   400,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Contains(t, resp.Message, "invalid character")
			},
		},
		{
			name:        "service error",
			requestBody: `{"sessionId":"sess-1","command":"ls -la"}`,
			mockCreate: func(record *model.CommandRecord) error {
				return errors.New("db error")
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   500,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Contains(t, resp.Message, "db error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockSvc, handler := setupSessionTest()
			mockSvc.createCommandRecordFunc = tt.mockCreate

			r.POST("/api/v1/session/command-records", handler.CreateCommandRecord)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/session/command-records",
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

func TestTerminateSession(t *testing.T) {
	tests := []struct {
		name           string
		sessionID      string
		mockTerminate  func(id string) error
		expectedStatus int
		expectedCode   int
	}{
		{
			name:      "success",
			sessionID: "sess-1",
			mockTerminate: func(id string) error {
				return nil
			},
			expectedStatus: http.StatusOK,
			expectedCode:   0,
		},
		{
			name:      "service error",
			sessionID: "sess-err",
			mockTerminate: func(id string) error {
				return errors.New("session not found")
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockSvc, handler := setupSessionTest()
			mockSvc.terminateSessionFunc = tt.mockTerminate

			r.POST("/api/v1/session/terminal/:sessionId", handler.TerminateSession)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/session/terminal/"+tt.sessionID, nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			resp := parseResponse(t, w.Body.Bytes())
			assert.Equal(t, tt.expectedCode, resp.Code)
		})
	}
}
