package middleware

import (
	"sync"
	"time"
)

// 登录防爆破策略：
//   - 同一 IP+账号：窗口内连续失败 5 次锁定 10 分钟
//   - 同一 IP（任意账号）：窗口内失败 20 次锁定 10 分钟，防止横向喷洒
const (
	maxLoginFailuresPerAccount = 5
	maxLoginFailuresPerIP      = 20
	loginLockDuration          = 10 * time.Minute
	loginAttemptWindow         = 10 * time.Minute
	loginGuardMaxEntries       = 10000
)

type loginAttempt struct {
	failures    int
	windowStart time.Time
	lockedUntil time.Time
}

type loginGuard struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt
}

var guard = &loginGuard{attempts: make(map[string]*loginAttempt)}

func loginAccountKey(ip, account string) string { return "acct|" + ip + "|" + account }
func loginIPKey(ip string) string               { return "ip|" + ip }

func (g *loginGuard) attemptFor(key string, now time.Time) *loginAttempt {
	a, ok := g.attempts[key]
	if !ok {
		a = &loginAttempt{windowStart: now}
		g.attempts[key] = a
		return a
	}
	// 窗口过期且不在锁定期内则重新计数
	if now.Sub(a.windowStart) > loginAttemptWindow && now.After(a.lockedUntil) {
		a.failures = 0
		a.windowStart = now
	}
	return a
}

// LoginLockRemaining 返回账号或 IP 的剩余锁定时间；0 表示未锁定。
func LoginLockRemaining(ip, account string) time.Duration {
	now := time.Now()
	guard.mu.Lock()
	defer guard.mu.Unlock()

	var remaining time.Duration
	for _, key := range []string{loginAccountKey(ip, account), loginIPKey(ip)} {
		if a, ok := guard.attempts[key]; ok && now.Before(a.lockedUntil) {
			if d := a.lockedUntil.Sub(now); d > remaining {
				remaining = d
			}
		}
	}
	return remaining
}

// RecordLoginFailure 记录一次登录失败，达到阈值后锁定。
func RecordLoginFailure(ip, account string) {
	now := time.Now()
	guard.mu.Lock()
	defer guard.mu.Unlock()
	guard.evictIfNeeded(now)

	limits := []struct {
		key string
		max int
	}{
		{loginAccountKey(ip, account), maxLoginFailuresPerAccount},
		{loginIPKey(ip), maxLoginFailuresPerIP},
	}
	for _, l := range limits {
		a := guard.attemptFor(l.key, now)
		a.failures++
		if a.failures >= l.max {
			a.lockedUntil = now.Add(loginLockDuration)
			a.failures = 0
			a.windowStart = now
		}
	}
}

// RecordLoginSuccess 登录成功后清除该账号的失败计数。
func RecordLoginSuccess(ip, account string) {
	guard.mu.Lock()
	defer guard.mu.Unlock()
	delete(guard.attempts, loginAccountKey(ip, account))
}

// evictIfNeeded 在条目过多时清理已过期的记录，避免内存无界增长。
func (g *loginGuard) evictIfNeeded(now time.Time) {
	if len(g.attempts) < loginGuardMaxEntries {
		return
	}
	for k, a := range g.attempts {
		if now.After(a.lockedUntil) && now.Sub(a.windowStart) > loginAttemptWindow {
			delete(g.attempts, k)
		}
	}
}
