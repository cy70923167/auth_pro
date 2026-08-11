package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDSN(t *testing.T) {
	tests := []struct {
		name string
		cfg  DBConfig
		want string
	}{
		{
			name: "tcp",
			cfg:  DBConfig{Host: "127.0.0.1", Port: "3306", Database: "auto_pro", Username: "root", Password: "secret"},
			want: "root:secret@tcp(127.0.0.1:3306)/auto_pro?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true",
		},
		{
			name: "unix socket",
			cfg:  DBConfig{Host: "unix:/tmp/mysql.sock", Database: "auto_pro", Username: "root"},
			want: "root:@unix(/tmp/mysql.sock)/auto_pro?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetDSN(&tt.cfg); got != tt.want {
				t.Fatalf("GetDSN() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadDBConfigFromEnv(t *testing.T) {
	t.Setenv("AUTO_PRO_DB_HOST", "unix:/tmp/auto-pro-test.sock")
	t.Setenv("AUTO_PRO_DB_PORT", "")
	t.Setenv("AUTO_PRO_DB_NAME", "auto_pro_test")
	t.Setenv("AUTO_PRO_DB_USER", "test_user")
	t.Setenv("AUTO_PRO_DB_PASSWORD", "test_password")

	cfg, err := LoadDBConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "unix:/tmp/auto-pro-test.sock" || cfg.Port != "" || cfg.Database != "auto_pro_test" || cfg.Username != "test_user" || cfg.Password != "test_password" {
		t.Fatalf("unexpected environment database config: %#v", cfg)
	}
}

func TestDefaultUpdateManifestURL(t *testing.T) {
	t.Setenv("AUTO_PRO_UPDATE_URL", "")
	if got := GetUpdateManifestURL(); got != "https://e.91ani.cn/latest.json" {
		t.Fatalf("GetUpdateManifestURL() = %q", got)
	}
}

func TestResolveFrontendDirForWebsiteRoot(t *testing.T) {
	root := t.TempDir()
	backendDir := filepath.Join(root, "backend")
	if err := os.MkdirAll(backendDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}

	got := resolveFrontendDir(backendDir, filepath.Join(backendDir, "auth_pro"))
	if got != root {
		t.Fatalf("resolveFrontendDir() = %q, want %q", got, root)
	}
}

func TestGetUpdateManifestURL(t *testing.T) {
	t.Setenv("AUTO_PRO_UPDATE_URL", "")
	if got := GetUpdateManifestURL(); got != "https://e.91ani.cn/latest.json" {
		t.Fatalf("GetUpdateManifestURL() = %q", got)
	}

	t.Setenv("AUTO_PRO_UPDATE_URL", "https://mirror.example.com/latest.json")
	if got := GetUpdateManifestURL(); got != "https://mirror.example.com/latest.json" {
		t.Fatalf("GetUpdateManifestURL() override = %q", got)
	}
}

func TestGetFrontendDirOverride(t *testing.T) {
	frontendDir := t.TempDir()
	t.Setenv("AUTO_PRO_FRONTEND_DIR", frontendDir)
	if got := GetFrontendDir(); got != frontendDir {
		t.Fatalf("GetFrontendDir() = %q, want %q", got, frontendDir)
	}
}
