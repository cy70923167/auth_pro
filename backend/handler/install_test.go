package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
)

func TestInstallStatusUsesPersistentLock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("AUTO_PRO_DATA_DIR", t.TempDir())
	if err := config.CreateLockFile(); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	InstallStatus(ctx)

	var response struct {
		Installed bool `json:"installed"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Installed {
		t.Fatal("installed system was reported as not installed")
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func TestInstallStatusRequiresPersistentLock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("AUTO_PRO_DATA_DIR", t.TempDir())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	InstallStatus(ctx)

	var response struct {
		Installed bool `json:"installed"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Installed {
		t.Fatal("system without install.lock was reported as installed")
	}
}
