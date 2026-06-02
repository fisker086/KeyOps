package release

import (
	"testing"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type mockReleaseRunRepo struct {
	run *model.ReleaseRun
}

func (m *mockReleaseRunRepo) Create(run *model.ReleaseRun) error           { return nil }
func (m *mockReleaseRunRepo) GetByID(id string) (*model.ReleaseRun, error) { return m.run, nil }
func (m *mockReleaseRunRepo) Update(run *model.ReleaseRun) error           { return nil }
func (m *mockReleaseRunRepo) UpdateStatus(id string, status string, startedAt, completedAt *time.Time) error {
	return nil
}
func (m *mockReleaseRunRepo) UpdateStatusAndDeployedEnv(id string, status string, deployedEnv string, startedAt, completedAt *time.Time) error {
	return nil
}
func (m *mockReleaseRunRepo) GetLastSuccessfulProdRun(applicationID string) (*model.ReleaseRun, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockReleaseRunRepo) List(repoURL, branch, status string, page, pageSize int) ([]model.ReleaseRun, int64, error) {
	return nil, 0, nil
}

var _ repository.ReleaseRunRepository = (*mockReleaseRunRepo)(nil)

func setupReleaseSvc(t *testing.T, run *model.ReleaseRun) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Approval{}, &model.RoleMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := &mockReleaseRunRepo{run: run}
	svc := NewService(repo)
	svc.db = db
	return svc, db
}

func TestExecuteRunProdCreatesApproval(t *testing.T) {
	run := &model.ReleaseRun{
		ID:            "r1",
		ApplicationID: "app1",
		RepoURL:       "https://git.example.com/repo.git",
		Branch:        "main",
		CommitSHA:     "1234567890abcdef",
		Status:        model.ReleaseRunStatusPending,
	}
	svc, db := setupReleaseSvc(t, run)

	if err := db.Create(&model.RoleMember{RoleID: "role:admin", UserID: "u-admin"}).Error; err != nil {
		t.Fatalf("create role member: %v", err)
	}

	created, approvalID, err := svc.ExecuteRun("r1", EnvironmentProd, "u1", "alice")
	if err != nil {
		t.Fatalf("ExecuteRun error: %v", err)
	}
	if !created || approvalID == "" {
		t.Fatalf("expected approval created, got created=%v approvalID=%q", created, approvalID)
	}

	var approval model.Approval
	if err := db.Where("id = ?", approvalID).First(&approval).Error; err != nil {
		t.Fatalf("approval not found: %v", err)
	}
	if approval.Type != model.ApprovalTypeDeployment || approval.Status != model.ApprovalStatusPending {
		t.Fatalf("unexpected approval: %#v", approval)
	}
}

func TestExecuteRunRejectsNonPending(t *testing.T) {
	run := &model.ReleaseRun{ID: "r1", Status: model.ReleaseRunStatusRunning}
	svc, _ := setupReleaseSvc(t, run)
	if _, _, err := svc.ExecuteRun("r1", EnvironmentProd, "u1", "alice"); err == nil {
		t.Fatal("expected error for non-pending run")
	}
}
