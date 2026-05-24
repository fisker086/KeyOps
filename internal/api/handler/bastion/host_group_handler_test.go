package bastion

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

var errTest = errors.New("test error")

type mockGroupRepo struct {
	findAllFunc                       func() ([]model.HostGroup, error)
	findAllWithStatsFunc              func() ([]model.HostGroup, error)
	findByIDFunc                      func(id string) (*model.HostGroup, error)
	createFunc                        func(group *model.HostGroup) error
	updateFunc                        func(group *model.HostGroup) error
	deleteFunc                        func(id string) error
	searchHostsInGroupFunc            func(groupID, search string) ([]model.Host, error)
	getHostsByGroupIDWithPaginationFunc func(groupID string, page, pageSize int) ([]model.Host, int64, error)
	addHostsToGroupFunc               func(groupID string, hostIDs []string, addedBy string) error
	removeHostsFromGroupFunc          func(groupID string, hostIDs []string) error
	getGroupsByHostIDFunc             func(hostID string) ([]model.HostGroup, error)
	getGroupStatisticsFunc            func(groupID string) (*model.HostGroupStatistics, error)
}

func (m *mockGroupRepo) Create(group *model.HostGroup) error {
	if m.createFunc != nil {
		return m.createFunc(group)
	}
	return nil
}

func (m *mockGroupRepo) FindByID(id string) (*model.HostGroup, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(id)
	}
	return nil, errTest
}

func (m *mockGroupRepo) FindByName(name string) (*model.HostGroup, error) {
	return nil, nil
}

func (m *mockGroupRepo) FindAll() ([]model.HostGroup, error) {
	if m.findAllFunc != nil {
		return m.findAllFunc()
	}
	return nil, nil
}

func (m *mockGroupRepo) FindAllWithStats() ([]model.HostGroup, error) {
	if m.findAllWithStatsFunc != nil {
		return m.findAllWithStatsFunc()
	}
	return nil, nil
}

func (m *mockGroupRepo) Update(group *model.HostGroup) error {
	if m.updateFunc != nil {
		return m.updateFunc(group)
	}
	return nil
}

func (m *mockGroupRepo) Delete(id string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(id)
	}
	return nil
}

func (m *mockGroupRepo) AddHostToGroup(groupID, hostID, addedBy string) error {
	return nil
}

func (m *mockGroupRepo) RemoveHostFromGroup(groupID, hostID string) error {
	return nil
}

func (m *mockGroupRepo) AddHostsToGroup(groupID string, hostIDs []string, addedBy string) error {
	if m.addHostsToGroupFunc != nil {
		return m.addHostsToGroupFunc(groupID, hostIDs, addedBy)
	}
	return nil
}

func (m *mockGroupRepo) RemoveHostsFromGroup(groupID string, hostIDs []string) error {
	if m.removeHostsFromGroupFunc != nil {
		return m.removeHostsFromGroupFunc(groupID, hostIDs)
	}
	return nil
}

func (m *mockGroupRepo) GetHostsByGroupID(groupID string) ([]model.Host, error) {
	return nil, nil
}

func (m *mockGroupRepo) GetHostsByGroupIDWithPagination(groupID string, page, pageSize int) ([]model.Host, int64, error) {
	if m.getHostsByGroupIDWithPaginationFunc != nil {
		return m.getHostsByGroupIDWithPaginationFunc(groupID, page, pageSize)
	}
	return nil, 0, nil
}

func (m *mockGroupRepo) GetGroupsByHostID(hostID string) ([]model.HostGroup, error) {
	if m.getGroupsByHostIDFunc != nil {
		return m.getGroupsByHostIDFunc(hostID)
	}
	return nil, nil
}

func (m *mockGroupRepo) IsHostInGroup(groupID, hostID string) (bool, error) {
	return false, nil
}

func (m *mockGroupRepo) GetGroupStatistics(groupID string) (*model.HostGroupStatistics, error) {
	if m.getGroupStatisticsFunc != nil {
		return m.getGroupStatisticsFunc(groupID)
	}
	return nil, nil
}

func (m *mockGroupRepo) SearchHostsInGroup(groupID, keyword string) ([]model.Host, error) {
	if m.searchHostsInGroupFunc != nil {
		return m.searchHostsInGroupFunc(groupID, keyword)
	}
	return nil, nil
}

func (m *mockGroupRepo) GetDB() *gorm.DB {
	return nil
}

func (m *mockGroupRepo) BatchUpdateSortOrder(updates map[string]int) error {
	return nil
}

func (m *mockGroupRepo) MoveHostsBetweenGroups(fromGroupID, toGroupID string, hostIDs []string, movedBy string) error {
	return nil
}

type mockUserRepo struct {
	getUsersInGroupFunc func(groupID string) ([]model.User, error)
}

func (m *mockUserRepo) CreateUser(user *model.User) error {
	return nil
}

func (m *mockUserRepo) FindUserByUsername(username string) (*model.User, error) {
	return nil, nil
}

func (m *mockUserRepo) FindUserByEmail(email string) (*model.User, error) {
	return nil, nil
}

func (m *mockUserRepo) FindUserByID(id string) (*model.User, error) {
	return nil, nil
}

func (m *mockUserRepo) UpdateUser(user *model.User) error {
	return nil
}

func (m *mockUserRepo) UpdateUserLastLogin(userID string, loginTime time.Time, loginIP string) error {
	return nil
}

func (m *mockUserRepo) FindAllUsers() ([]model.User, error) {
	return nil, nil
}

func (m *mockUserRepo) CreatePlatformLoginRecord(record *model.PlatformLoginRecord) error {
	return nil
}

func (m *mockUserRepo) FindPlatformLoginRecords(page, pageSize int, userID string) ([]model.PlatformLoginRecord, int64, error) {
	return nil, 0, nil
}

func (m *mockUserRepo) UpdatePlatformLoginRecordLogout(recordID string) error {
	return nil
}

func (m *mockUserRepo) UpdatePlatformLoginRecordLogoutByUser(userID string) error {
	return nil
}

func (m *mockUserRepo) GetDB() *gorm.DB {
	return nil
}

func (m *mockUserRepo) FindAllUsersWithPagination(page, pageSize int, keyword string) ([]model.User, int64, error) {
	return nil, 0, nil
}

func (m *mockUserRepo) DeleteUser(userID string) error {
	return nil
}

func (m *mockUserRepo) UpdateUserRole(userID, role string) error {
	return nil
}

func (m *mockUserRepo) UpdateUserStatus(userID, status string) error {
	return nil
}

func (m *mockUserRepo) AssignRolesToUser(userID string, roleIDs []string, createdBy string) error {
	return nil
}

func (m *mockUserRepo) GetUserRoles(userID string) ([]string, error) {
	return nil, nil
}

func (m *mockUserRepo) GetUserWithGroups(userID string) (*model.UserWithGroups, error) {
	return nil, nil
}

func (m *mockUserRepo) FindAllUsersWithGroups(page, pageSize int, keyword string) ([]model.UserWithGroups, int64, error) {
	return nil, 0, nil
}

func (m *mockUserRepo) RemoveUserFromGroup(userID, groupID string) error {
	return nil
}

func (m *mockUserRepo) AddUserToGroup(userID, groupID, createdBy string) error {
	return nil
}

func (m *mockUserRepo) GetUsersInGroup(groupID string) ([]model.User, error) {
	if m.getUsersInGroupFunc != nil {
		return m.getUsersInGroupFunc(groupID)
	}
	return nil, nil
}

func (m *mockUserRepo) AssignHostsToUser(userID string, hostIDs []string, createdBy string) error {
	return nil
}

func (m *mockUserRepo) GetUserHosts(userID string) ([]string, error) {
	return nil, nil
}

func (m *mockUserRepo) GetUserHostGroupIDs(userID string) ([]string, error) {
	return nil, nil
}

func (m *mockUserRepo) AddUserToHost(userID, hostID, createdBy string) error {
	return nil
}

func (m *mockUserRepo) RemoveUserFromHost(userID, hostID string) error {
	return nil
}

func (m *mockUserRepo) GetUserWithGroupsAndHosts(userID string) (*model.UserWithGroups, error) {
	return nil, nil
}

func (m *mockUserRepo) FindPlatformLoginRecordByID(recordID string) (*model.PlatformLoginRecord, error) {
	return nil, nil
}

func (m *mockUserRepo) SearchUsersByGroup(groupID, keyword string) ([]model.User, error) {
	return nil, nil
}

func (m *mockUserRepo) GetUsersInGroupWithPagination(groupID string, page, pageSize int) ([]model.User, int64, error) {
	return nil, 0, nil
}

func (m *mockUserRepo) BatchAddUsersToGroup(groupID string, userIDs []string, createdBy string) error {
	return nil
}

func (m *mockUserRepo) BatchRemoveUsersFromGroup(groupID string, userIDs []string) error {
	return nil
}

func (m *mockUserRepo) FindAllUsersWithGroupsAndHosts(page, pageSize int, keyword string) ([]model.UserWithGroups, int64, error) {
	return nil, 0, nil
}

type mockHostRepo struct{}

func (m *mockHostRepo) Create(host *model.Host) error { return nil }
func (m *mockHostRepo) FindByID(id string) (*model.Host, error) { return nil, nil }
func (m *mockHostRepo) FindByIP(ip string) (*model.Host, error) { return nil, nil }
func (m *mockHostRepo) FindByIPAndPort(ip string, port int) (*model.Host, error) { return nil, nil }
func (m *mockHostRepo) Update(host *model.Host) error { return nil }
func (m *mockHostRepo) Delete(id string) error { return nil }
func (m *mockHostRepo) FindAll(page, pageSize int, search string, tags []string) ([]model.Host, int64, error) {
	return nil, 0, nil
}
func (m *mockHostRepo) FindByUser(page, pageSize int, search string, tags []string, userID string) ([]model.Host, int64, error) {
	return nil, 0, nil
}
func (m *mockHostRepo) CountByStatus() (total, online, offline int64, err error) { return 0, 0, 0, nil }
func (m *mockHostRepo) CountByStatusForUser(userID string) (total, online, offline int64, err error) {
	return 0, 0, 0, nil
}
func (m *mockHostRepo) IncrementLoginCount(id string) error { return nil }
func (m *mockHostRepo) UpdateLastLoginTime(id string) error { return nil }
func (m *mockHostRepo) GetHostsWithUserLoginCount(page, pageSize int, search string, tags []string, userID string) ([]model.Host, int64, error) {
	return nil, 0, nil
}
func (m *mockHostRepo) GetUserFrequentHosts(userID string, limit int) ([]model.Host, error) { return nil, nil }
func (m *mockHostRepo) UpdateStatus(id string, status string) error { return nil }
func (m *mockHostRepo) FindAllWithPagination(page, pageSize int, search string, tags []string) ([]model.Host, int64, error) {
	return nil, 0, nil
}
func (m *mockHostRepo) FindByUserPermissions(page, pageSize int, search string, tags []string, userID string) ([]model.Host, int64, error) {
	return nil, 0, nil
}
func (m *mockHostRepo) GetAccessibleHostIDsForUser(userID string) ([]string, error) { return nil, nil }
func (m *mockHostRepo) FindAllWithFilters(filters map[string]interface{}, page, pageSize int, search string, tags []string) ([]model.Host, int64, error) {
	return nil, 0, nil
}
func (m *mockHostRepo) FindByIDs(ids []string) ([]model.Host, error) { return nil, nil }
func (m *mockHostRepo) GetDB() *gorm.DB { return nil }
func (m *mockHostRepo) BatchUpdateTags(updates map[string]string) error { return nil }
func (m *mockHostRepo) MoveHostsToGroup(hostIDs []string, groupID string) error { return nil }
func (m *mockHostRepo) CheckIPAndPortDuplicate(ip string, port int, excludeID string) (bool, error) {
	return false, nil
}
func (m *mockHostRepo) FindHostsByUserID(userID string) ([]model.Host, error) { return nil, nil }

type testResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func setupGroupTest() (*gin.Engine, *mockGroupRepo, *mockUserRepo) {
	gin.SetMode(gin.TestMode)
	mockGroup := &mockGroupRepo{}
	mockHost := &mockHostRepo{}
	mockUser := &mockUserRepo{}
	handler := NewHostGroupHandler(mockGroup, mockHost, mockUser)
	r := gin.New()

	r.GET("/api/host-groups", handler.ListGroups)
	r.GET("/api/host-groups/:id", handler.GetGroup)
	r.POST("/api/host-groups", handler.CreateGroup)
	r.PUT("/api/host-groups/:id", handler.UpdateGroup)
	r.DELETE("/api/host-groups/:id", handler.DeleteGroup)
	r.GET("/api/host-groups/:id/hosts", handler.GetGroupHosts)
	r.POST("/api/host-groups/:id/hosts", handler.AddHostsToGroup)
	r.DELETE("/api/host-groups/:id/hosts", handler.RemoveHostsFromGroup)
	r.GET("/api/hosts/:id/groups", handler.GetHostGroups)
	r.GET("/api/host-groups/:id/statistics", handler.GetGroupStatistics)
	r.GET("/api/host-groups/:id/users", handler.GetGroupUsers)

	return r, mockGroup, mockUser
}

func parseGroupResponse(t *testing.T, body []byte) testResponse {
	var resp testResponse
	err := json.Unmarshal(body, &resp)
	assert.NoError(t, err)
	return resp
}

func TestListGroups(t *testing.T) {
	groups := []model.HostGroup{
		{ID: uuid.New().String(), Name: "group1"},
		{ID: uuid.New().String(), Name: "group2"},
	}

	tests := []struct {
		name         string
		query        string
		mockFindAll  func() ([]model.HostGroup, error)
		expectedCode int
		expectedLen  int
	}{
		{
			name:  "success",
			query: "",
			mockFindAll: func() ([]model.HostGroup, error) {
				return groups, nil
			},
			expectedCode: 0,
			expectedLen:  2,
		},
		{
			name:  "success with stats",
			query: "?stats=true",
			mockFindAll: func() ([]model.HostGroup, error) {
				groups[0].HostCount = 5
				groups[1].HostCount = 3
				return groups, nil
			},
			expectedCode: 0,
			expectedLen:  2,
		},
		{
			name:  "service error",
			query: "",
			mockFindAll: func() ([]model.HostGroup, error) {
				return nil, errTest
			},
			expectedCode: -1,
			expectedLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockGroup, _ := setupGroupTest()
			mockGroup.findAllFunc = tt.mockFindAll
			mockGroup.findAllWithStatsFunc = tt.mockFindAll

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/host-groups"+tt.query, nil)
			r.ServeHTTP(w, req)

			if tt.expectedCode == 0 {
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.Equal(t, http.StatusInternalServerError, w.Code)
			}

			resp := parseGroupResponse(t, w.Body.Bytes())
			assert.Equal(t, tt.expectedCode, resp.Code)
		})
	}
}

func TestGetGroup(t *testing.T) {
	groupID := uuid.New().String()
	group := &model.HostGroup{ID: groupID, Name: "test-group"}
	stats := &model.HostGroupStatistics{
		GroupID:    groupID,
		GroupName:  "test-group",
		TotalHosts: 10,
		OnlineHosts: 5,
	}

	tests := []struct {
		name           string
		groupID        string
		mockFindByID   func(id string) (*model.HostGroup, error)
		mockStats      func(id string) (*model.HostGroupStatistics, error)
		expectedCode   int
		expectedStatus int
	}{
		{
			name:    "success",
			groupID: groupID,
			mockFindByID: func(id string) (*model.HostGroup, error) {
				return group, nil
			},
			mockStats: func(id string) (*model.HostGroupStatistics, error) {
				return stats, nil
			},
			expectedCode:   0,
			expectedStatus: http.StatusOK,
		},
		{
			name:    "not found",
			groupID: "nonexistent",
			mockFindByID: func(id string) (*model.HostGroup, error) {
				return nil, errTest
			},
			expectedCode:   -1,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockGroup, _ := setupGroupTest()
			mockGroup.findByIDFunc = tt.mockFindByID
			mockGroup.getGroupStatisticsFunc = tt.mockStats

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/host-groups/"+tt.groupID, nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			resp := parseGroupResponse(t, w.Body.Bytes())
			assert.Equal(t, tt.expectedCode, resp.Code)
		})
	}
}

func TestCreateGroup(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		setUserID      bool
		mockCreate     func(group *model.HostGroup) error
		expectedCode   int
		expectedStatus int
		checkResponse  func(t *testing.T, resp testResponse)
	}{
		{
			name:           "success",
			requestBody:    `{"name":"new-group","description":"desc"}`,
			setUserID:      false,
			expectedCode:   0,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp testResponse) {
				assert.Contains(t, string(resp.Data), `"name":"new-group"`)
			},
		},
		{
			name:           "invalid JSON",
			requestBody:    `{bad json`,
			expectedCode:   -1,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:        "service error",
			requestBody: `{"name":"fail-group"}`,
			mockCreate: func(group *model.HostGroup) error {
				return errTest
			},
			expectedCode:   -1,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockGroup, _ := setupGroupTest()
			mockGroup.createFunc = tt.mockCreate

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/host-groups",
				strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			if tt.setUserID {
				// Not needed for current tests as handler handles missing userID gracefully
			}

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			resp := parseGroupResponse(t, w.Body.Bytes())
			assert.Equal(t, tt.expectedCode, resp.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestUpdateGroup(t *testing.T) {
	groupID := uuid.New().String()
	existingGroup := &model.HostGroup{ID: groupID, Name: "old-name", Description: "old-desc"}

	tests := []struct {
		name           string
		groupID        string
		requestBody    string
		mockFindByID   func(id string) (*model.HostGroup, error)
		mockUpdate     func(group *model.HostGroup) error
		expectedCode   int
		expectedStatus int
	}{
		{
			name:        "success",
			groupID:     groupID,
			requestBody: `{"name":"new-name","description":"new-desc"}`,
			mockFindByID: func(id string) (*model.HostGroup, error) {
				return existingGroup, nil
			},
			expectedCode:   0,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid JSON",
			groupID:        groupID,
			requestBody:    `{bad json`,
			expectedCode:   -1,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:        "not found",
			groupID:     "nonexistent",
			requestBody: `{"name":"new-name"}`,
			mockFindByID: func(id string) (*model.HostGroup, error) {
				return nil, errTest
			},
			expectedCode:   -1,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:        "empty name",
			groupID:     groupID,
			requestBody: `{"name":""}`,
			mockFindByID: func(id string) (*model.HostGroup, error) {
				return existingGroup, nil
			},
			expectedCode:   -1,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:        "service error",
			groupID:     groupID,
			requestBody: `{"name":"new-name"}`,
			mockFindByID: func(id string) (*model.HostGroup, error) {
				return existingGroup, nil
			},
			mockUpdate: func(group *model.HostGroup) error {
				return errTest
			},
			expectedCode:   -1,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockGroup, _ := setupGroupTest()
			mockGroup.findByIDFunc = tt.mockFindByID
			mockGroup.updateFunc = tt.mockUpdate

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", "/api/host-groups/"+tt.groupID,
				strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			resp := parseGroupResponse(t, w.Body.Bytes())
			assert.Equal(t, tt.expectedCode, resp.Code)
		})
	}
}

func TestDeleteGroup(t *testing.T) {
	tests := []struct {
		name           string
		groupID        string
		mockDelete     func(id string) error
		expectedCode   int
		expectedStatus int
	}{
		{
			name:    "success",
			groupID: uuid.New().String(),
			expectedCode:   0,
			expectedStatus: http.StatusOK,
		},
		{
			name:    "service error",
			groupID: uuid.New().String(),
			mockDelete: func(id string) error {
				return errTest
			},
			expectedCode:   -1,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockGroup, _ := setupGroupTest()
			mockGroup.deleteFunc = tt.mockDelete

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("DELETE", "/api/host-groups/"+tt.groupID, nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			resp := parseGroupResponse(t, w.Body.Bytes())
			assert.Equal(t, tt.expectedCode, resp.Code)
		})
	}
}

func TestGetGroupHosts(t *testing.T) {
	hosts := []model.Host{
		{ID: uuid.New().String(), Name: "host1", IP: "10.0.0.1"},
		{ID: uuid.New().String(), Name: "host2", IP: "10.0.0.2"},
	}

	tests := []struct {
		name         string
		groupID      string
		query        string
		mockSearch   func(groupID, search string) ([]model.Host, error)
		mockPagination func(groupID string, page, pageSize int) ([]model.Host, int64, error)
		expectedCode int
	}{
		{
			name:    "success",
			groupID: uuid.New().String(),
			query:   "",
			mockPagination: func(groupID string, page, pageSize int) ([]model.Host, int64, error) {
				return hosts, 2, nil
			},
			expectedCode: 0,
		},
		{
			name:    "success with search",
			groupID: uuid.New().String(),
			query:   "?search=host1",
			mockSearch: func(groupID, search string) ([]model.Host, error) {
				return hosts[:1], nil
			},
			expectedCode: 0,
		},
		{
			name:    "service error",
			groupID: uuid.New().String(),
			query:   "",
			mockPagination: func(groupID string, page, pageSize int) ([]model.Host, int64, error) {
				return nil, 0, errTest
			},
			expectedCode: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockGroup, _ := setupGroupTest()
			mockGroup.searchHostsInGroupFunc = tt.mockSearch
			mockGroup.getHostsByGroupIDWithPaginationFunc = tt.mockPagination

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/host-groups/"+tt.groupID+"/hosts"+tt.query, nil)
			r.ServeHTTP(w, req)

			if tt.expectedCode == 0 {
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.Equal(t, http.StatusInternalServerError, w.Code)
			}

			resp := parseGroupResponse(t, w.Body.Bytes())
			assert.Equal(t, tt.expectedCode, resp.Code)
		})
	}
}

func TestAddHostsToGroup(t *testing.T) {
	groupID := uuid.New().String()

	tests := []struct {
		name           string
		groupID        string
		requestBody    string
		mockFindByID   func(id string) (*model.HostGroup, error)
		mockAddHosts   func(groupID string, hostIDs []string, addedBy string) error
		expectedCode   int
		expectedStatus int
	}{
		{
			name:        "success",
			groupID:     groupID,
			requestBody: `{"hostIds":["host1","host2"]}`,
			mockFindByID: func(id string) (*model.HostGroup, error) {
				return &model.HostGroup{ID: groupID}, nil
			},
			expectedCode:   0,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid JSON",
			groupID:        groupID,
			requestBody:    `{bad json`,
			expectedCode:   -1,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:        "not found",
			groupID:     "nonexistent",
			requestBody: `{"hostIds":["host1"]}`,
			mockFindByID: func(id string) (*model.HostGroup, error) {
				return nil, errTest
			},
			expectedCode:   -1,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:        "service error",
			groupID:     groupID,
			requestBody: `{"hostIds":["host1"]}`,
			mockFindByID: func(id string) (*model.HostGroup, error) {
				return &model.HostGroup{ID: groupID}, nil
			},
			mockAddHosts: func(groupID string, hostIDs []string, addedBy string) error {
				return errTest
			},
			expectedCode:   -1,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockGroup, _ := setupGroupTest()
			mockGroup.findByIDFunc = tt.mockFindByID
			mockGroup.addHostsToGroupFunc = tt.mockAddHosts

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/host-groups/"+tt.groupID+"/hosts",
				strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			resp := parseGroupResponse(t, w.Body.Bytes())
			assert.Equal(t, tt.expectedCode, resp.Code)
		})
	}
}

func TestRemoveHostsFromGroup(t *testing.T) {
	groupID := uuid.New().String()

	tests := []struct {
		name           string
		groupID        string
		requestBody    string
		mockFindByID   func(id string) (*model.HostGroup, error)
		mockRemoveHosts func(groupID string, hostIDs []string) error
		expectedCode   int
		expectedStatus int
	}{
		{
			name:        "success",
			groupID:     groupID,
			requestBody: `{"hostIds":["host1"]}`,
			mockFindByID: func(id string) (*model.HostGroup, error) {
				return &model.HostGroup{ID: groupID}, nil
			},
			expectedCode:   0,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid JSON",
			groupID:        groupID,
			requestBody:    `{bad json`,
			expectedCode:   -1,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:        "not found",
			groupID:     "nonexistent",
			requestBody: `{"hostIds":["host1"]}`,
			mockFindByID: func(id string) (*model.HostGroup, error) {
				return nil, errTest
			},
			expectedCode:   -1,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:        "service error",
			groupID:     groupID,
			requestBody: `{"hostIds":["host1"]}`,
			mockFindByID: func(id string) (*model.HostGroup, error) {
				return &model.HostGroup{ID: groupID}, nil
			},
			mockRemoveHosts: func(groupID string, hostIDs []string) error {
				return errTest
			},
			expectedCode:   -1,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockGroup, _ := setupGroupTest()
			mockGroup.findByIDFunc = tt.mockFindByID
			mockGroup.removeHostsFromGroupFunc = tt.mockRemoveHosts

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("DELETE", "/api/host-groups/"+tt.groupID+"/hosts",
				strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			resp := parseGroupResponse(t, w.Body.Bytes())
			assert.Equal(t, tt.expectedCode, resp.Code)
		})
	}
}

func TestGetHostGroups(t *testing.T) {
	groups := []model.HostGroup{
		{ID: uuid.New().String(), Name: "group1"},
	}

	tests := []struct {
		name              string
		hostID            string
		mockGetGroupsByHostID func(hostID string) ([]model.HostGroup, error)
		expectedCode      int
	}{
		{
			name:   "success",
			hostID: uuid.New().String(),
			mockGetGroupsByHostID: func(hostID string) ([]model.HostGroup, error) {
				return groups, nil
			},
			expectedCode: 0,
		},
		{
			name:   "service error",
			hostID: uuid.New().String(),
			mockGetGroupsByHostID: func(hostID string) ([]model.HostGroup, error) {
				return nil, errTest
			},
			expectedCode: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockGroup, _ := setupGroupTest()
			mockGroup.getGroupsByHostIDFunc = tt.mockGetGroupsByHostID

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/hosts/"+tt.hostID+"/groups", nil)
			r.ServeHTTP(w, req)

			if tt.expectedCode == 0 {
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.Equal(t, http.StatusInternalServerError, w.Code)
			}

			resp := parseGroupResponse(t, w.Body.Bytes())
			assert.Equal(t, tt.expectedCode, resp.Code)
		})
	}
}

func TestGetGroupStatistics(t *testing.T) {
	stats := &model.HostGroupStatistics{
		GroupID:    uuid.New().String(),
		GroupName:  "test-group",
		TotalHosts: 10,
		OnlineHosts: 5,
		OfflineHosts: 5,
	}

	tests := []struct {
		name         string
		groupID      string
		mockStats    func(id string) (*model.HostGroupStatistics, error)
		expectedCode int
	}{
		{
			name:    "success",
			groupID: uuid.New().String(),
			mockStats: func(id string) (*model.HostGroupStatistics, error) {
				return stats, nil
			},
			expectedCode: 0,
		},
		{
			name:    "service error",
			groupID: uuid.New().String(),
			mockStats: func(id string) (*model.HostGroupStatistics, error) {
				return nil, errTest
			},
			expectedCode: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockGroup, _ := setupGroupTest()
			mockGroup.getGroupStatisticsFunc = tt.mockStats

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/host-groups/"+tt.groupID+"/statistics", nil)
			r.ServeHTTP(w, req)

			if tt.expectedCode == 0 {
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.Equal(t, http.StatusInternalServerError, w.Code)
			}

			resp := parseGroupResponse(t, w.Body.Bytes())
			assert.Equal(t, tt.expectedCode, resp.Code)
		})
	}
}

func TestGetGroupUsers(t *testing.T) {
	users := []model.User{
		{ID: uuid.New().String(), Username: "user1"},
		{ID: uuid.New().String(), Username: "user2"},
	}

	tests := []struct {
		name           string
		groupID        string
		mockGetUsers   func(groupID string) ([]model.User, error)
		expectedCode   int
		expectedStatus int
	}{
		{
			name:    "success",
			groupID: uuid.New().String(),
			mockGetUsers: func(groupID string) ([]model.User, error) {
				return users, nil
			},
			expectedCode:   0,
			expectedStatus: http.StatusOK,
		},
		{
			name:    "service error",
			groupID: uuid.New().String(),
			mockGetUsers: func(groupID string) ([]model.User, error) {
				return nil, errTest
			},
			expectedCode:   -1,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _, mockUser := setupGroupTest()
			mockUser.getUsersInGroupFunc = tt.mockGetUsers

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/host-groups/"+tt.groupID+"/users", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			resp := parseGroupResponse(t, w.Body.Bytes())
			assert.Equal(t, tt.expectedCode, resp.Code)
		})
	}
}
