package bastion

import (
	"testing"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func setupSessionSvc(t *testing.T) *SessionService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Host{}, &model.LoginRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	hostRepo := repository.NewHostRepository(db)
	sessionRepo := repository.NewSessionRepository(db, nil, nil, nil)
	return NewSessionService(sessionRepo, hostRepo)
}

func TestCreateSessionSuccess(t *testing.T) {
	svc := setupSessionSvc(t)

	db := svc.repo.GetDB()
	hostID := uuid.NewString()
	if err := db.Create(&model.Host{ID: hostID, Name: "h1", IP: "10.0.0.1", Port: 22}).Error; err != nil {
		t.Fatalf("create host: %v", err)
	}

	resp, err := svc.CreateSession(hostID, "u-1")
	if err != nil {
		t.Fatalf("CreateSession error: %v", err)
	}
	if resp.SessionID == "" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestCreateSessionHostNotFound(t *testing.T) {
	svc := setupSessionSvc(t)
	if _, err := svc.CreateSession("missing-host", "u-1"); err == nil {
		t.Fatal("expected error for missing host")
	}
}
