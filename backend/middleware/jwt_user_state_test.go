package middleware

import (
	"database/sql"
	"testing"
)

func TestActiveUserRejection(t *testing.T) {
	tests := []struct {
		name             string
		enabled          bool
		accountStatus    string
		convertedAgentID sql.NullInt64
		wantMessage      string
		wantConverted    bool
	}{
		{name: "active user", enabled: true, accountStatus: "active"},
		{
			name:             "converted status invalidates old token",
			enabled:          false,
			accountStatus:    "converted",
			convertedAgentID: sql.NullInt64{Int64: 501, Valid: true},
			wantMessage:      "该账号已升级为代理，请前往代理端登录",
			wantConverted:    true,
		},
		{
			name:             "conversion link invalidates stale active status",
			enabled:          true,
			accountStatus:    "active",
			convertedAgentID: sql.NullInt64{Int64: 501, Valid: true},
			wantMessage:      "该账号已升级为代理，请前往代理端登录",
			wantConverted:    true,
		},
		{name: "disabled user", enabled: false, accountStatus: "active", wantMessage: "用户账户已禁用"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, converted := activeUserRejection(tt.enabled, tt.accountStatus, tt.convertedAgentID)
			if message != tt.wantMessage || converted != tt.wantConverted {
				t.Fatalf("activeUserRejection() = (%q, %v), want (%q, %v)", message, converted, tt.wantMessage, tt.wantConverted)
			}
		})
	}
}
