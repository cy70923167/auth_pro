package handler

import (
	"regexp"
	"testing"
)

func TestLicenseVerifySuccessDataIncludesPlan(t *testing.T) {
	data := licenseVerifySuccessData("Demo App", matchedLicense{
		PlanID:   12,
		PlanName: "专业版",
		Type:     "key",
	})

	if data["planId"] != int64(12) || data["planName"] != "专业版" {
		t.Fatalf("unexpected plan data: planId=%v planName=%v", data["planId"], data["planName"])
	}
}

func TestGenerateRandomLicenseKey(t *testing.T) {
	seen := make(map[string]struct{}, 128)
	valid := regexp.MustCompile(`^[A-Za-z0-9]{16}$`)

	for i := 0; i < 128; i++ {
		key, err := generateRandomLicenseKey()
		if err != nil {
			t.Fatalf("generateRandomLicenseKey() error = %v", err)
		}
		if !valid.MatchString(key) {
			t.Fatalf("generated key %q is not a 16-character alphanumeric key", key)
		}
		if _, exists := seen[key]; exists {
			t.Fatalf("generated duplicate key %q", key)
		}
		seen[key] = struct{}{}
	}
}
