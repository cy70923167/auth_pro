package middleware

import (
	"testing"
	"time"
)

func resetGuard() {
	guard.mu.Lock()
	defer guard.mu.Unlock()
	guard.attempts = make(map[string]*loginAttempt)
}

func TestAccountLocksAfterMaxFailures(t *testing.T) {
	resetGuard()
	ip, account := "1.2.3.4", "admin"

	for i := 0; i < maxLoginFailuresPerAccount-1; i++ {
		RecordLoginFailure(ip, account)
		if got := LoginLockRemaining(ip, account); got != 0 {
			t.Fatalf("failure %d: expected unlocked, got %v", i+1, got)
		}
	}

	RecordLoginFailure(ip, account)
	if got := LoginLockRemaining(ip, account); got <= 0 {
		t.Fatal("expected account to be locked after reaching threshold")
	}
}

func TestOtherAccountSameIPLockedByIPLimit(t *testing.T) {
	resetGuard()
	ip := "5.6.7.8"

	for i := 0; i < maxLoginFailuresPerIP; i++ {
		RecordLoginFailure(ip, "user"+string(rune('a'+i%26))+string(rune('a'+(i/26)%26)))
	}

	if got := LoginLockRemaining(ip, "brand_new_account"); got <= 0 {
		t.Fatal("expected IP-level lock to affect new accounts from the same IP")
	}
	if got := LoginLockRemaining("9.9.9.9", "brand_new_account"); got != 0 {
		t.Fatal("expected other IPs to remain unlocked")
	}
}

func TestSuccessClearsAccountCounter(t *testing.T) {
	resetGuard()
	ip, account := "1.2.3.4", "admin"

	for i := 0; i < maxLoginFailuresPerAccount-1; i++ {
		RecordLoginFailure(ip, account)
	}
	RecordLoginSuccess(ip, account)

	for i := 0; i < maxLoginFailuresPerAccount-1; i++ {
		RecordLoginFailure(ip, account)
		if got := LoginLockRemaining(ip, account); got != 0 {
			t.Fatalf("expected counter reset after success, locked with %v remaining", got)
		}
	}
}

func TestLockExpires(t *testing.T) {
	resetGuard()
	ip, account := "1.2.3.4", "admin"

	for i := 0; i < maxLoginFailuresPerAccount; i++ {
		RecordLoginFailure(ip, account)
	}

	// 手动把锁定结束时间改到过去，模拟锁定期结束
	guard.mu.Lock()
	guard.attempts[loginAccountKey(ip, account)].lockedUntil = time.Now().Add(-time.Second)
	guard.mu.Unlock()

	if got := LoginLockRemaining(ip, account); got != 0 {
		t.Fatalf("expected lock to expire, got %v", got)
	}
}
