package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fisker086/keyops/internal/model"
	authService "github.com/fisker086/keyops/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type mockAuthService struct {
	registerFunc                   func(*model.RegisterRequest) (*model.User, error)
	loginFunc                      func(*model.LoginRequest, string, string) (*model.LoginResponse, string, error)
	logoutFunc                     func(string, string) error
	validateRefreshTokenFunc       func(string) (*authService.RefreshClaims, error)
	getUserByIDFunc                func(string) (*model.User, error)
	generateTokenPairFunc          func(*model.User) (string, string, error)
	enforceSessionLimitFunc        func(string) error
	getAllUsersFunc                func() ([]model.User, error)
	getUsersWithPaginationFunc     func(int, int, string) ([]model.User, int64, error)
	getUserWithGroupsAndHostsFunc  func(string) (*model.UserWithGroups, error)
	getPlatformLoginRecordsFunc    func(int, int, string) ([]model.PlatformLoginRecord, int64, error)
	createUserFunc                 func(*model.RegisterRequest, string, string, *string) (*model.User, error)
	updateUserInfoFunc             func(string, string, string, *string) error
	updateUserExpirationFunc       func(string, *string, *bool) error
	updateUserRoleFunc             func(string, string) error
	updateUserStatusFunc           func(string, string) error
	deleteUserFunc                 func(string) error
	resetUserPasswordFunc          func(string, string) error
	loginWithSSOFunc               func(string, string, string) (*model.LoginResponse, string, error)
	refreshTokenPairFunc           func(string) (string, string, error)
	assignRolesToUserFunc          func(string, []string, string) error
	getUserRolesFunc               func(string) ([]string, error)
	getUserWithGroupsFunc          func(string) (*model.UserWithGroups, error)
	getUsersWithGroupsFunc         func(int, int, string) ([]model.UserWithGroups, int64, error)
	assignHostsToUserFunc          func(string, []string, string) error
	getUserHostsFunc               func(string) ([]string, error)
	getUsersWithGroupsAndHostsFunc func(int, int, string) ([]model.UserWithGroups, int64, error)
	generateSSHKeyFunc             func(string) error
	deleteSSHKeyFunc               func(string) error
	getSSHPrivateKeyFunc           func(string) (string, string, error)
	updateUserAuthMethodFunc       func(string, string) error
}

func (m *mockAuthService) Register(req *model.RegisterRequest) (*model.User, error) {
	if m.registerFunc != nil {
		return m.registerFunc(req)
	}
	return nil, nil
}

func (m *mockAuthService) Login(req *model.LoginRequest, loginIP, userAgent string) (*model.LoginResponse, string, error) {
	if m.loginFunc != nil {
		return m.loginFunc(req, loginIP, userAgent)
	}
	return nil, "", nil
}

func (m *mockAuthService) Logout(userID, refreshJTI string) error {
	if m.logoutFunc != nil {
		return m.logoutFunc(userID, refreshJTI)
	}
	return nil
}

func (m *mockAuthService) ValidateRefreshToken(tokenString string) (*authService.RefreshClaims, error) {
	if m.validateRefreshTokenFunc != nil {
		return m.validateRefreshTokenFunc(tokenString)
	}
	return nil, nil
}

func (m *mockAuthService) GetUserByID(userID string) (*model.User, error) {
	if m.getUserByIDFunc != nil {
		return m.getUserByIDFunc(userID)
	}
	return nil, errors.New("not found")
}

func (m *mockAuthService) GenerateTokenPair(user *model.User) (string, string, error) {
	if m.generateTokenPairFunc != nil {
		return m.generateTokenPairFunc(user)
	}
	return "", "", nil
}

func (m *mockAuthService) EnforceSessionLimitForUser(userID string) error {
	if m.enforceSessionLimitFunc != nil {
		return m.enforceSessionLimitFunc(userID)
	}
	return nil
}

func (m *mockAuthService) GetAllUsers() ([]model.User, error) {
	if m.getAllUsersFunc != nil {
		return m.getAllUsersFunc()
	}
	return nil, nil
}

func (m *mockAuthService) GetUsersWithPagination(page, pageSize int, keyword string) ([]model.User, int64, error) {
	if m.getUsersWithPaginationFunc != nil {
		return m.getUsersWithPaginationFunc(page, pageSize, keyword)
	}
	return nil, 0, nil
}

func (m *mockAuthService) GetUserWithGroupsAndHosts(userID string) (*model.UserWithGroups, error) {
	if m.getUserWithGroupsAndHostsFunc != nil {
		return m.getUserWithGroupsAndHostsFunc(userID)
	}
	return nil, nil
}

func (m *mockAuthService) GetPlatformLoginRecords(page, pageSize int, userID string) ([]model.PlatformLoginRecord, int64, error) {
	if m.getPlatformLoginRecordsFunc != nil {
		return m.getPlatformLoginRecordsFunc(page, pageSize, userID)
	}
	return nil, 0, nil
}

func (m *mockAuthService) CreateUser(req *model.RegisterRequest, role string, authMethod string, organizationID *string) (*model.User, error) {
	if m.createUserFunc != nil {
		return m.createUserFunc(req, role, authMethod, organizationID)
	}
	return nil, nil
}

func (m *mockAuthService) UpdateUserInfo(userID, fullName, email string, organizationID *string) error {
	if m.updateUserInfoFunc != nil {
		return m.updateUserInfoFunc(userID, fullName, email, organizationID)
	}
	return nil
}

func (m *mockAuthService) UpdateUserExpiration(userID string, expiresAt *string, autoDisableOnExpiry *bool) error {
	if m.updateUserExpirationFunc != nil {
		return m.updateUserExpirationFunc(userID, expiresAt, autoDisableOnExpiry)
	}
	return nil
}

func (m *mockAuthService) UpdateUserRole(userID, role string) error {
	if m.updateUserRoleFunc != nil {
		return m.updateUserRoleFunc(userID, role)
	}
	return nil
}

func (m *mockAuthService) UpdateUserStatus(userID, status string) error {
	if m.updateUserStatusFunc != nil {
		return m.updateUserStatusFunc(userID, status)
	}
	return nil
}

func (m *mockAuthService) DeleteUser(userID string) error {
	if m.deleteUserFunc != nil {
		return m.deleteUserFunc(userID)
	}
	return nil
}

func (m *mockAuthService) ResetUserPassword(userID, newPassword string) error {
	if m.resetUserPasswordFunc != nil {
		return m.resetUserPasswordFunc(userID, newPassword)
	}
	return nil
}

func (m *mockAuthService) LoginWithSSO(code, loginIP, userAgent string) (*model.LoginResponse, string, error) {
	if m.loginWithSSOFunc != nil {
		return m.loginWithSSOFunc(code, loginIP, userAgent)
	}
	return nil, "", nil
}

func (m *mockAuthService) RefreshTokenPair(refreshTokenString string) (string, string, error) {
	if m.refreshTokenPairFunc != nil {
		return m.refreshTokenPairFunc(refreshTokenString)
	}
	return "", "", nil
}

func (m *mockAuthService) AssignRolesToUser(userID string, roleIDs []string, adminID string) error {
	if m.assignRolesToUserFunc != nil {
		return m.assignRolesToUserFunc(userID, roleIDs, adminID)
	}
	return nil
}

func (m *mockAuthService) GetUserRoles(userID string) ([]string, error) {
	if m.getUserRolesFunc != nil {
		return m.getUserRolesFunc(userID)
	}
	return nil, nil
}

func (m *mockAuthService) GetUserWithGroups(userID string) (*model.UserWithGroups, error) {
	if m.getUserWithGroupsFunc != nil {
		return m.getUserWithGroupsFunc(userID)
	}
	return nil, nil
}

func (m *mockAuthService) GetUsersWithGroups(page, pageSize int, keyword string) ([]model.UserWithGroups, int64, error) {
	if m.getUsersWithGroupsFunc != nil {
		return m.getUsersWithGroupsFunc(page, pageSize, keyword)
	}
	return nil, 0, nil
}

func (m *mockAuthService) AssignHostsToUser(userID string, hostIDs []string, adminID string) error {
	if m.assignHostsToUserFunc != nil {
		return m.assignHostsToUserFunc(userID, hostIDs, adminID)
	}
	return nil
}

func (m *mockAuthService) GetUserHosts(userID string) ([]string, error) {
	if m.getUserHostsFunc != nil {
		return m.getUserHostsFunc(userID)
	}
	return nil, nil
}

func (m *mockAuthService) GetUsersWithGroupsAndHosts(page, pageSize int, keyword string) ([]model.UserWithGroups, int64, error) {
	if m.getUsersWithGroupsAndHostsFunc != nil {
		return m.getUsersWithGroupsAndHostsFunc(page, pageSize, keyword)
	}
	return nil, 0, nil
}

func (m *mockAuthService) GenerateSSHKey(userID string) error {
	if m.generateSSHKeyFunc != nil {
		return m.generateSSHKeyFunc(userID)
	}
	return nil
}

func (m *mockAuthService) DeleteSSHKey(userID string) error {
	if m.deleteSSHKeyFunc != nil {
		return m.deleteSSHKeyFunc(userID)
	}
	return nil
}

func (m *mockAuthService) GetSSHPrivateKey(userID string) (string, string, error) {
	if m.getSSHPrivateKeyFunc != nil {
		return m.getSSHPrivateKeyFunc(userID)
	}
	return "", "", nil
}

func (m *mockAuthService) UpdateUserAuthMethod(userID, authMethod string) error {
	if m.updateUserAuthMethodFunc != nil {
		return m.updateUserAuthMethodFunc(userID, authMethod)
	}
	return nil
}

func setupAuthTest() (*gin.Engine, *mockAuthService) {
	gin.SetMode(gin.TestMode)
	mockSvc := &mockAuthService{}
	handler := NewAuthHandler(mockSvc, nil, nil)
	r := gin.New()
	r.POST("/api/auth/register", handler.Register)
	r.POST("/api/auth/login", handler.Login)
	r.GET("/api/auth/me", func(c *gin.Context) {
		c.Set("userID", "test-user-id")
		handler.GetCurrentUser(c)
	})
	r.GET("/api/auth/me-unauthorized", handler.GetCurrentUser)
	return r, mockSvc
}

func parseResponse(t *testing.T, body []byte) model.Response {
	var resp model.Response
	err := json.Unmarshal(body, &resp)
	assert.NoError(t, err)
	return resp
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name          string
		requestBody   string
		mockRegister  func(*model.RegisterRequest) (*model.User, error)
		expectedCode  int
		checkResponse func(t *testing.T, resp model.Response)
	}{
		{
			name:        "successful registration",
			requestBody: `{"username":"testuser","password":"password123","email":"test@example.com","fullName":"Test User"}`,
			mockRegister: func(req *model.RegisterRequest) (*model.User, error) {
				return &model.User{
					ID:       "user-1",
					Username: req.Username,
					Email:    req.Email,
					FullName: req.FullName,
					Role:     "user",
					Status:   "active",
				}, nil
			},
			expectedCode: http.StatusOK,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Equal(t, 0, resp.Code)
				data, _ := json.Marshal(resp.Data)
				var user model.User
				err := json.Unmarshal(data, &user)
				assert.NoError(t, err)
				assert.Equal(t, "testuser", user.Username)
				assert.Equal(t, "test@example.com", user.Email)
				assert.Equal(t, "user", user.Role)
				assert.Equal(t, "active", user.Status)
			},
		},
		{
			name:         "invalid JSON body",
			requestBody:  `{bad json`,
			expectedCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Equal(t, 400, resp.Code)
				assert.Contains(t, resp.Message, "invalid")
			},
		},
		{
			name:        "service returns error",
			requestBody: `{"username":"existing","password":"password123","email":"existing@example.com","fullName":"Existing"}`,
			mockRegister: func(req *model.RegisterRequest) (*model.User, error) {
				return nil, errors.New("用户名已存在")
			},
			expectedCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Equal(t, 400, resp.Code)
				assert.Contains(t, resp.Message, "用户名已存在")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockSvc := setupAuthTest()
			mockSvc.registerFunc = tt.mockRegister

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/auth/register",
				strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			resp := parseResponse(t, w.Body.Bytes())
			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name          string
		requestBody   string
		mockLogin     func(*model.LoginRequest, string, string) (*model.LoginResponse, string, error)
		expectedCode  int
		checkResponse func(t *testing.T, resp model.Response)
	}{
		{
			name:        "successful login",
			requestBody: `{"username":"testuser","password":"password123"}`,
			mockLogin: func(req *model.LoginRequest, loginIP, userAgent string) (*model.LoginResponse, string, error) {
				return &model.LoginResponse{
					AccessToken: "access-token-123",
					User: model.User{
						ID:       "user-1",
						Username: req.Username,
						Role:     "user",
						Status:   "active",
					},
				}, "refresh-token-123", nil
			},
			expectedCode: http.StatusOK,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Equal(t, 0, resp.Code)
				assert.Equal(t, "success", resp.Message)
				data, _ := json.Marshal(resp.Data)
				var loginResp model.LoginResponse
				err := json.Unmarshal(data, &loginResp)
				assert.NoError(t, err)
				assert.Equal(t, "access-token-123", loginResp.AccessToken)
				assert.Equal(t, "testuser", loginResp.User.Username)
			},
		},
		{
			name:         "invalid JSON body",
			requestBody:  `{bad json`,
			expectedCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Equal(t, 400, resp.Code)
				assert.Contains(t, resp.Message, "invalid")
			},
		},
		{
			name:        "invalid credentials",
			requestBody: `{"username":"baduser","password":"wrongpass"}`,
			mockLogin: func(req *model.LoginRequest, loginIP, userAgent string) (*model.LoginResponse, string, error) {
				return nil, "", errors.New("用户名或密码错误")
			},
			expectedCode: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Equal(t, 401, resp.Code)
				assert.Contains(t, resp.Message, "用户名或密码错误")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockSvc := setupAuthTest()
			mockSvc.loginFunc = tt.mockLogin

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/auth/login",
				strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			resp := parseResponse(t, w.Body.Bytes())
			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestGetCurrentUser(t *testing.T) {
	tests := []struct {
		name            string
		path            string
		mockGetUserByID func(string) (*model.User, error)
		expectedCode    int
		checkResponse   func(t *testing.T, resp model.Response)
	}{
		{
			name: "success",
			path: "/api/auth/me",
			mockGetUserByID: func(userID string) (*model.User, error) {
				return &model.User{
					ID:       userID,
					Username: "testuser",
					Email:    "test@example.com",
					FullName: "Test User",
					Role:     "user",
					Status:   "active",
				}, nil
			},
			expectedCode: http.StatusOK,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Equal(t, 0, resp.Code)
				data, _ := json.Marshal(resp.Data)
				var user model.User
				err := json.Unmarshal(data, &user)
				assert.NoError(t, err)
				assert.Equal(t, "testuser", user.Username)
				assert.Equal(t, "test@example.com", user.Email)
				assert.Equal(t, "user", user.Role)
			},
		},
		{
			name:         "unauthorized - no userID in context",
			path:         "/api/auth/me-unauthorized",
			expectedCode: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Equal(t, 401, resp.Code)
				assert.Contains(t, resp.Message, "未登录")
			},
		},
		{
			name: "not found",
			path: "/api/auth/me",
			mockGetUserByID: func(userID string) (*model.User, error) {
				return nil, errors.New("用户不存在")
			},
			expectedCode: http.StatusNotFound,
			checkResponse: func(t *testing.T, resp model.Response) {
				assert.Equal(t, 404, resp.Code)
				assert.Contains(t, resp.Message, "用户不存在")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockSvc := setupAuthTest()
			mockSvc.getUserByIDFunc = tt.mockGetUserByID

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.path, nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			resp := parseResponse(t, w.Body.Bytes())
			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}
