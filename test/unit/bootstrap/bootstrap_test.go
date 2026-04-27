package bootstrap

import (
	"testing"

	appbootstrap "github.com/hcd233/aris-proxy-api/internal/bootstrap"
)

func TestBuildServer(t *testing.T) {
	server, err := appbootstrap.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}
	if server == nil {
		t.Fatal("BuildServer() returned nil server")
	}
	if server.App == nil {
		t.Fatal("BuildServer() returned nil Fiber app")
	}
	if server.HumaAPI == nil {
		t.Fatal("BuildServer() returned nil Huma API")
	}
}

func TestRegisterRoutes(t *testing.T) {
	server, err := appbootstrap.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}
	if err := appbootstrap.RegisterRoutes(server); err != nil {
		t.Fatalf("RegisterRoutes() error = %v", err)
	}
}
