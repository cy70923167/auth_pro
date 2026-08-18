package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type licenseOwnerScanFunc func(dest ...any) error

func (fn licenseOwnerScanFunc) Scan(dest ...any) error {
	return fn(dest...)
}

func TestValidateLicenseOwner(t *testing.T) {
	databaseErr := errors.New("database unavailable")

	tests := []struct {
		name           string
		ownerType      string
		ownerID        int64
		enabled        bool
		scanErr        error
		wantMessage    string
		wantErr        error
		wantTable      string
		wantQueryCalls int
	}{
		{name: "enabled user", ownerType: "user", ownerID: 12, enabled: true, wantTable: "users", wantQueryCalls: 1},
		{name: "enabled agent", ownerType: "agent", ownerID: 34, enabled: true, wantTable: "agents", wantQueryCalls: 1},
		{name: "disabled user", ownerType: "user", ownerID: 12, wantMessage: "指定用户不存在或已禁用", wantTable: "users", wantQueryCalls: 1},
		{name: "missing agent", ownerType: "agent", ownerID: 34, scanErr: sql.ErrNoRows, wantMessage: "指定代理不存在或已禁用", wantTable: "agents", wantQueryCalls: 1},
		{name: "invalid type", ownerType: "admin", ownerID: 1, wantMessage: "授权归属类型不正确"},
		{name: "invalid id", ownerType: "user", ownerID: 0, wantMessage: "请选择授权归属账号"},
		{name: "database error", ownerType: "user", ownerID: 12, scanErr: databaseErr, wantErr: databaseErr, wantTable: "users", wantQueryCalls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryCalls := 0
			message, err := validateLicenseOwner(func(query string, args ...any) licenseOwnerRowScanner {
				queryCalls++
				if tt.wantTable != "" && !strings.Contains(query, "FROM "+tt.wantTable) {
					t.Fatalf("query = %q, want table %q", query, tt.wantTable)
				}
				if !strings.HasSuffix(query, "FOR UPDATE") {
					t.Fatalf("query = %q, want row lock", query)
				}
				if len(args) != 1 || args[0] != tt.ownerID {
					t.Fatalf("args = %#v, want owner ID %d", args, tt.ownerID)
				}
				return licenseOwnerScanFunc(func(dest ...any) error {
					if tt.scanErr != nil {
						return tt.scanErr
					}
					*(dest[0].(*bool)) = tt.enabled
					return nil
				})
			}, tt.ownerType, tt.ownerID)

			if message != tt.wantMessage {
				t.Fatalf("message = %q, want %q", message, tt.wantMessage)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if queryCalls != tt.wantQueryCalls {
				t.Fatalf("query calls = %d, want %d", queryCalls, tt.wantQueryCalls)
			}
		})
	}
}

func TestLoadAdminLicensePlan(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	tests := []struct {
		name         string
		scanErr      error
		durationDays int
		price        float64
		wantMessage  string
		wantErr      error
	}{
		{name: "enabled plan", durationDays: 365, price: 199},
		{name: "missing or mismatched plan", scanErr: sql.ErrNoRows, wantMessage: "套餐不存在、已禁用或不属于所选应用"},
		{name: "database error", scanErr: databaseErr, wantErr: databaseErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, message, err := loadAdminLicensePlan(func(query string, args ...any) licensePlanRowScanner {
				if !strings.Contains(query, "p.id = ? AND p.app_id = ?") ||
					!strings.Contains(query, "p.enabled = 1 AND a.enabled = 1") ||
					!strings.Contains(query, "FOR UPDATE") {
					t.Fatalf("query does not enforce plan ownership and enabled status: %q", query)
				}
				if len(args) != 2 || args[0] != int64(9) || args[1] != int64(3) {
					t.Fatalf("args = %#v, want plan ID 9 and app ID 3", args)
				}
				return licenseOwnerScanFunc(func(dest ...any) error {
					if tt.scanErr != nil {
						return tt.scanErr
					}
					*(dest[0].(*int)) = tt.durationDays
					*(dest[1].(*float64)) = tt.price
					return nil
				})
			}, 3, 9)

			if message != tt.wantMessage {
				t.Fatalf("message = %q, want %q", message, tt.wantMessage)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if plan.DurationDays != tt.durationDays || plan.OriginalPrice != tt.price {
				t.Fatalf("plan = %#v, want duration %d and price %.2f", plan, tt.durationDays, tt.price)
			}
		})
	}
}

func TestLicenseCreateRequiresPlan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/license/create", bytes.NewBufferString(
		`{"appId":1,"type":"domain","ownerType":"user","ownerId":12,"domain":"example.com"}`,
	))
	ctx.Request.Header.Set("Content-Type", "application/json")

	LicenseCreate(ctx)

	var response struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != 400 {
		t.Fatalf("response code = %d, want 400; body = %s", response.Code, recorder.Body.String())
	}
}

func TestLicenseCreateRequiresOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		payload string
	}{
		{name: "missing owner type", payload: `{"appId":1,"planId":1,"type":"domain","ownerId":12,"domain":"example.com"}`},
		{name: "missing owner id", payload: `{"appId":1,"planId":1,"type":"domain","ownerType":"user","domain":"example.com"}`},
		{name: "invalid owner type", payload: `{"appId":1,"planId":1,"type":"domain","ownerType":"admin","ownerId":1,"domain":"example.com"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/license/create", bytes.NewBufferString(tt.payload))
			ctx.Request.Header.Set("Content-Type", "application/json")

			LicenseCreate(ctx)

			var response struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Code != 400 {
				t.Fatalf("response code = %d, want 400; body = %s", response.Code, recorder.Body.String())
			}
		})
	}
}
