package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"auto_pro/config"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name       string
		left       string
		right      string
		wantResult int
	}{
		{name: "equal with missing zeros", left: "1.2", right: "1.2.0", wantResult: 0},
		{name: "numeric segment", left: "1.10.0", right: "1.9.9", wantResult: 1},
		{name: "leading v", left: "v2.0.0", right: "1.99.0", wantResult: 1},
		{name: "prerelease before release", left: "2.0.0-rc.1", right: "2.0.0", wantResult: -1},
		{name: "prerelease numeric", left: "2.0.0-rc.10", right: "2.0.0-rc.2", wantResult: 1},
		{name: "build metadata ignored", left: "1.0.0+build.2", right: "1.0.0+build.1", wantResult: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareVersions(tt.left, tt.right)
			if got != tt.wantResult {
				t.Fatalf("compareVersions(%q, %q) = %d, want %d", tt.left, tt.right, got, tt.wantResult)
			}
		})
	}
}

func TestShouldForceUpdate(t *testing.T) {
	tests := []struct {
		name       string
		current    string
		latest     appVersionRecord
		candidates []appVersionRecord
		want       bool
	}{
		{
			name: "downstream release forces update", current: "1.0.0",
			latest: appVersionRecord{Version: "1.2.0"},
			candidates: []appVersionRecord{
				{Version: "1.1.0"}, {Version: "1.2.0", ForceUpdate: true},
			},
			want: true,
		},
		{
			name: "below latest release minimum", current: "1.4.9",
			latest: appVersionRecord{Version: "2.1.0", MinVersion: "2.0.0"}, want: true,
		},
		{
			name: "latest release can lower minimum", current: "1.5.0",
			latest: appVersionRecord{Version: "2.1.0", MinVersion: "1.5.0"},
			candidates: []appVersionRecord{
				{Version: "2.0.0", MinVersion: "2.0.0"}, {Version: "2.1.0", MinVersion: "1.5.0"},
			},
			want: false,
		},
		{
			name: "current release force flag is ignored", current: "1.0.0",
			latest: appVersionRecord{Version: "1.1.0"},
			candidates: []appVersionRecord{
				{Version: "1.0.0", ForceUpdate: true}, {Version: "1.1.0"},
			},
			want: false,
		},
		{name: "optional update", current: "1.0.0", latest: appVersionRecord{Version: "1.1.0"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldForceUpdate(tt.current, tt.latest, tt.candidates); got != tt.want {
				t.Fatalf("shouldForceUpdate(%q, %#v, %#v) = %v, want %v", tt.current, tt.latest, tt.candidates, got, tt.want)
			}
		})
	}
}

func TestValidateVersionFields(t *testing.T) {
	if err := validateVersionFields("1.2.0", "稳定版本", "修复问题", "1.0.0"); err != nil {
		t.Fatalf("valid fields returned error: %v", err)
	}
	if err := validateVersionFields("latest", "稳定版本", "修复问题", ""); err == nil {
		t.Fatal("invalid version should return an error")
	}
	if err := validateVersionFields("1.2.0", "稳定版本", "修复问题", "2.0.0"); err == nil {
		t.Fatal("minimum version above release should return an error")
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := map[string]string{
		"v1.2.0":          "1.2",
		"1.02.000":        "1.2",
		"1.2.0+build.10":  "1.2",
		"V2.0.0-RC.01":    "2-rc.1",
		"1_10_0-alpha.01": "1.10-alpha.1",
	}
	for input, want := range tests {
		if got := normalizeVersion(input); got != want {
			t.Fatalf("normalizeVersion(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestValidVersion(t *testing.T) {
	valid := []string{"1", "1.2.0", "v2.0.0-rc.1", "1_2_0+build.7"}
	for _, version := range valid {
		if !validVersion(version) {
			t.Fatalf("validVersion(%q) = false, want true", version)
		}
	}

	invalid := []string{"", "latest", "1.a.0", "1.2.beta", "1.2.0-", "1.2.0+"}
	for _, version := range invalid {
		if validVersion(version) {
			t.Fatalf("validVersion(%q) = true, want false", version)
		}
	}
}

func TestNormalizedVersionConflict(t *testing.T) {
	tests := []struct {
		name  string
		items []appVersionNormItem
		want  bool
	}{
		{
			name: "versions can be published independently",
			items: []appVersionNormItem{
				{ID: 1, AppID: 1, VersionNorm: "1.1"},
				{ID: 2, AppID: 1, VersionNorm: "1.2"},
				{ID: 3, AppID: 2, VersionNorm: "1.1"},
			},
		},
		{
			name: "duplicate semantic target",
			items: []appVersionNormItem{
				{ID: 1, AppID: 1, VersionNorm: "1.2"},
				{ID: 2, AppID: 1, VersionNorm: "1.2"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizedVersionConflict(tt.items) != nil; got != tt.want {
				t.Fatalf("normalizedVersionConflict() error = %v, want error %v", got, tt.want)
			}
		})
	}
}

func TestAppVersionCheckSignValid(t *testing.T) {
	req := appVersionCheckRequest{
		AppKey: "demo-app", CurrentVersion: "1.2.0", LicenseKey: "license-key", Timestamp: 1234567890,
	}
	canonical := strings.Join([]string{req.AppKey, req.CurrentVersion, req.LicenseKey, "1234567890"}, "\n")
	mac := hmac.New(sha256.New, []byte("app-secret"))
	_, _ = mac.Write([]byte(canonical))
	req.Sign = hex.EncodeToString(mac.Sum(nil))
	if !appVersionCheckSignValid(req, "app-secret") {
		t.Fatal("valid version check signature was rejected")
	}
	req.CurrentVersion = "1.2.1"
	if appVersionCheckSignValid(req, "app-secret") {
		t.Fatal("tampered currentVersion should invalidate signature")
	}
}

func TestAppVersionDownloadToken(t *testing.T) {
	token, err := createAppVersionDownloadToken(12, 34, 56, "license")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	claims, err := parseAppVersionDownloadToken(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.VersionID != 12 || claims.AppID != 34 || claims.LicenseID != 56 || claims.Scope != "license" {
		t.Fatalf("unexpected claims: %#v", claims)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[1] == "" {
		t.Fatalf("unexpected token format: %q", token)
	}
	replacement := byte('A')
	if parts[1][0] == replacement {
		replacement = 'B'
	}
	parts[1] = string(replacement) + parts[1][1:]
	tampered := strings.Join(parts, ".")
	if _, err := parseAppVersionDownloadToken(tampered); err == nil {
		t.Fatal("tampered token should fail")
	}
}

func TestExpiredAppVersionDownloadToken(t *testing.T) {
	payload := strings.Join([]string{
		"12", "34", "56", "license",
		strconv.FormatInt(time.Now().Add(-time.Second).Unix(), 10),
		base64.RawURLEncoding.EncodeToString([]byte("expired-token-nonce")),
	}, ".")
	secret, err := config.LoadOrCreateJWTSecret()
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	token := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if _, err := parseAppVersionDownloadToken(token); err == nil {
		t.Fatal("expired token should fail")
	}
}

func TestResolveReleasePackagePath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AUTO_PRO_APP_RELEASE_DIR", root)

	got, err := resolveReleasePackagePath("app-12/release-package.zip")
	if err != nil {
		t.Fatalf("resolve valid package path: %v", err)
	}
	want := filepath.Join(root, "app-12", "release-package.zip")
	if got != want {
		t.Fatalf("resolved path = %q, want %q", got, want)
	}

	invalid := []string{"", "../secret", "app-12/../../secret", filepath.Join(root, "outside.zip")}
	for _, packagePath := range invalid {
		if _, err := resolveReleasePackagePath(packagePath); err == nil {
			t.Fatalf("resolveReleasePackagePath(%q) should fail", packagePath)
		}
	}
}

func TestValidateDownloadURL(t *testing.T) {
	valid := []string{
		"https://example.com/releases/app.zip",
		"http://downloads.example.com/app.zip?token=public",
	}
	for _, rawURL := range valid {
		if err := validateDownloadURL(rawURL); err != nil {
			t.Fatalf("validateDownloadURL(%q) returned error: %v", rawURL, err)
		}
	}

	invalid := []string{
		"javascript:alert(1)",
		"/relative/app.zip",
		"https://user:password@example.com/app.zip",
	}
	for _, rawURL := range invalid {
		if err := validateDownloadURL(rawURL); err == nil {
			t.Fatalf("validateDownloadURL(%q) should fail", rawURL)
		}
	}
}
