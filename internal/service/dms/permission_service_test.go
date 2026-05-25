package dms

import "testing"

func TestBuildResourcePath(t *testing.T) {
	svc := &PermissionService{}
	if got := svc.buildResourcePath(1, "", ""); got != "dms:instance:1" {
		t.Fatalf("unexpected instance path: %s", got)
	}
	if got := svc.buildResourcePath(2, "app", ""); got != "dms:instance:2:db:app" {
		t.Fatalf("unexpected db path: %s", got)
	}
	if got := svc.buildResourcePath(3, "app", "orders"); got != "dms:instance:3:db:app:table:orders" {
		t.Fatalf("unexpected table path: %s", got)
	}
}

func TestParsePermission(t *testing.T) {
	svc := &PermissionService{}
	perm := svc.parsePermission("dms:instance:7:db:testdb:table:users", "read")
	if perm == nil {
		t.Fatal("expected permission parsed")
	}
	if perm.InstanceID != 7 || perm.DatabaseName != "testdb" || perm.Table != "users" || perm.PermissionType != "read" {
		t.Fatalf("unexpected permission parsed: %#v", perm)
	}

	if svc.parsePermission("invalid:path", "read") != nil {
		t.Fatal("expected nil for invalid path")
	}
}
