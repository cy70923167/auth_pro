package handler

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestResolveLicenseSiteTarget(t *testing.T) {
	tests := []struct {
		name       string
		domain     string
		serverIP   string
		wantType   string
		wantTarget string
		wantIP     string
		wantErr    error
	}{
		{
			name:       "domain takes precedence and is normalized",
			domain:     "HTTPS://Example.COM./install/path",
			serverIP:   "192.0.2.10",
			wantType:   "domain",
			wantTarget: "example.com",
			wantIP:     "192.0.2.10",
		},
		{
			name:       "server ip is fallback target",
			serverIP:   "2001:0db8:0:0:0:0:0:1",
			wantType:   "ip",
			wantTarget: "2001:db8::1",
			wantIP:     "2001:db8::1",
		},
		{
			name:     "invalid domain is rejected",
			domain:   "bad domain.example",
			serverIP: "192.0.2.10",
			wantErr:  errLicenseSiteInvalidDomain,
		},
		{
			name:     "invalid server ip is rejected",
			domain:   "example.com",
			serverIP: "not-an-ip",
			wantErr:  errLicenseSiteInvalidIP,
		},
		{
			name:    "empty target is rejected",
			wantErr: errLicenseSiteEmpty,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target, err := resolveLicenseSiteTarget(test.domain, test.serverIP)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("resolveLicenseSiteTarget() error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr != nil {
				return
			}
			if target.Type != test.wantType || target.Value != test.wantTarget || target.ServerIP != test.wantIP {
				t.Fatalf("resolveLicenseSiteTarget() = %#v", target)
			}
		})
	}
}

func TestLicenseSiteIdentityIgnoresServerIPForDomain(t *testing.T) {
	first, err := resolveLicenseSiteTarget("example.com", "192.0.2.10")
	if err != nil {
		t.Fatal(err)
	}
	second, err := resolveLicenseSiteTarget("EXAMPLE.COM.", "192.0.2.11")
	if err != nil {
		t.Fatal(err)
	}
	if first.Type != second.Type || first.Value != second.Value {
		t.Fatalf("same domain produced different identities: %#v %#v", first, second)
	}
	if first.ServerIP == second.ServerIP {
		t.Fatalf("test inputs did not preserve different last-seen IPs: %#v %#v", first, second)
	}
}

func TestLicenseVerifyV2SignatureProtectsTargetFields(t *testing.T) {
	secret := "app-secret"
	base := licenseVerifyRequest{
		AppKey:      "demo-app",
		LicenseKey:  "license-key",
		Domain:      "Example.COM.",
		ServerIP:    "2001:0db8:0:0:0:0:0:1",
		Timestamp:   1700000000,
		SignVersion: licenseSignVersionV2,
	}
	base.Sign = licenseVerifyV2Sign(base, secret)
	if version, valid := licenseVerifySignValid(base, secret, base.Domain, base.ServerIP, base.LicenseKey); !valid || version != licenseSignVersionV2 {
		t.Fatal("valid v2 signature was rejected")
	}

	tampered := []licenseVerifyRequest{
		base,
		base,
		base,
		base,
	}
	tampered[0].LicenseKey = "other-key"
	tampered[1].Domain = "other.example.com"
	tampered[2].ServerIP = "2001:db8::2"
	tampered[3].Timestamp++
	for index, request := range tampered {
		if _, valid := licenseVerifySignValid(request, secret, request.Domain, request.ServerIP, request.LicenseKey); valid {
			t.Fatalf("tampered v2 request %d was accepted", index)
		}
	}
}

func TestLicenseVerifyV1CompatibilitySignature(t *testing.T) {
	secret := "app-secret"
	req := licenseVerifyRequest{
		AppKey:     "demo-app",
		LicenseKey: "license-key",
		Domain:     "example.com",
		ServerIP:   "192.0.2.10",
		Timestamp:  1700000000,
	}
	req.Sign = licenseVerifyMD5(req.AppKey + req.LicenseKey + int64ToString(req.Timestamp) + secret)
	if version, valid := licenseVerifySignValid(req, secret, req.Domain, req.ServerIP, req.LicenseKey); !valid || version != licenseSignVersionV1 {
		t.Fatal("valid v1 compatibility signature was rejected")
	}
}

func TestAppVersionV2SignatureProtectsSite(t *testing.T) {
	secret := "app-secret"
	req := appVersionCheckRequest{
		AppKey:         "demo-app",
		CurrentVersion: "1.2.3",
		LicenseKey:     "license-key",
		Domain:         "example.com",
		ServerIP:       "192.0.2.10",
		Timestamp:      1700000000,
		SignVersion:    licenseSignVersionV2,
	}
	canonical := appVersionCheckV2Canonical(req)
	req.Sign = appVersionCheckV2Sign(canonical, secret)
	if version, valid := appVersionCheckSignValid(req, secret); !valid || version != licenseSignVersionV2 {
		t.Fatal("valid app-version v2 signature was rejected")
	}
	req.Domain = "other.example.com"
	if _, valid := appVersionCheckSignValid(req, secret); valid {
		t.Fatal("tampered app-version site was accepted")
	}
}

const licenseSiteTestDriverName = "license-site-test"

var (
	registerLicenseSiteTestDriver sync.Once
	licenseSiteTestStates         sync.Map
)

type licenseSiteTestState struct {
	mu sync.Mutex

	licenseType string
	maxSites    int

	bindings map[string]string // identity key -> server IP

	insertFailures int // 剩余触发唯一键冲突的插入次数
	insertCount    int
	updateCount    int
}

type licenseSiteTestDriver struct{}
type licenseSiteTestConn struct{ state *licenseSiteTestState }
type licenseSiteTestTx struct{ state *licenseSiteTestState }
type licenseSiteTestResult struct{ id int64 }
type licenseSiteTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func openLicenseSiteTestDB(t *testing.T, state *licenseSiteTestState) *sql.DB {
	t.Helper()
	registerLicenseSiteTestDriver.Do(func() {
		sql.Register(licenseSiteTestDriverName, licenseSiteTestDriver{})
	})
	name := fmt.Sprintf("license-site-%d", lenOfSyncMap(&licenseSiteTestStates))
	licenseSiteTestStates.Store(name, state)
	t.Cleanup(func() { licenseSiteTestStates.Delete(name) })
	db, err := sql.Open(licenseSiteTestDriverName, name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func lenOfSyncMap(m *sync.Map) int {
	count := 0
	m.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

func (licenseSiteTestDriver) Open(name string) (driver.Conn, error) {
	value, ok := licenseSiteTestStates.Load(name)
	if !ok {
		return nil, errors.New("license site test state not found")
	}
	return &licenseSiteTestConn{state: value.(*licenseSiteTestState)}, nil
}

func (conn *licenseSiteTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}
func (conn *licenseSiteTestConn) Close() error { return nil }
func (conn *licenseSiteTestConn) Begin() (driver.Tx, error) {
	return &licenseSiteTestTx{state: conn.state}, nil
}
func (conn *licenseSiteTestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return conn.Begin()
}

func licenseSiteIdentityKey(licenseID int64, targetType, target string) string {
	return fmt.Sprintf("%d\x00%s\x00%s", licenseID, targetType, target)
}

func (conn *licenseSiteTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	conn.state.mu.Lock()
	defer conn.state.mu.Unlock()

	switch {
	case strings.Contains(query, "SELECT type, COALESCE(max_domains, 0)"):
		if len(args) != 1 {
			return nil, errors.New("license lock query missing ID")
		}
		return &licenseSiteTestRows{
			columns: []string{"type", "max_domains"},
			values:  [][]driver.Value{{conn.state.licenseType, int64(conn.state.maxSites)}},
		}, nil
	case strings.Contains(query, "SELECT id FROM license_domains"):
		if len(args) != 3 {
			return nil, errors.New("binding lookup missing arguments")
		}
		licenseID, _ := args[0].Value.(int64)
		targetType, _ := args[1].Value.(string)
		target, _ := args[2].Value.(string)
		key := licenseSiteIdentityKey(licenseID, targetType, target)
		if _, exists := conn.state.bindings[key]; !exists {
			return &licenseSiteTestRows{columns: []string{"id"}}, nil
		}
		return &licenseSiteTestRows{columns: []string{"id"}, values: [][]driver.Value{{int64(1)}}}, nil
	case strings.Contains(query, "SELECT COUNT(*) FROM license_domains"):
		return &licenseSiteTestRows{
			columns: []string{"count"},
			values:  [][]driver.Value{{int64(len(conn.state.bindings))}},
		}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

func (conn *licenseSiteTestConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	conn.state.mu.Lock()
	defer conn.state.mu.Unlock()

	switch {
	case strings.Contains(query, "UPDATE license_domains"):
		conn.state.updateCount++
		return licenseSiteTestResult{id: 1}, nil
	case strings.Contains(query, "INSERT INTO license_domains"):
		if len(args) != 4 {
			return nil, errors.New("binding insert missing arguments")
		}
		if conn.state.insertFailures > 0 {
			conn.state.insertFailures--
			return nil, &mysql.MySQLError{Number: 1062, Message: "duplicate key"}
		}
		licenseID, _ := args[0].Value.(int64)
		targetType, _ := args[1].Value.(string)
		target, _ := args[2].Value.(string)
		serverIP, _ := args[3].Value.(string)
		conn.state.bindings[licenseSiteIdentityKey(licenseID, targetType, target)] = serverIP
		conn.state.insertCount++
		return licenseSiteTestResult{id: int64(conn.state.insertCount)}, nil
	default:
		return nil, errors.New("unexpected exec: " + query)
	}
}

func (tx *licenseSiteTestTx) Commit() error   { return nil }
func (tx *licenseSiteTestTx) Rollback() error { return nil }
func (result licenseSiteTestResult) LastInsertId() (int64, error) {
	return result.id, nil
}
func (licenseSiteTestResult) RowsAffected() (int64, error) { return 1, nil }
func (rows licenseSiteTestRows) Columns() []string         { return rows.columns }
func (licenseSiteTestRows) Close() error                   { return nil }
func (rows *licenseSiteTestRows) Next(dest []driver.Value) error {
	if rows.index >= len(rows.values) {
		return io.EOF
	}
	copy(dest, rows.values[rows.index])
	rows.index++
	return nil
}

func newLicenseSiteTestState() *licenseSiteTestState {
	return &licenseSiteTestState{
		licenseType: "key",
		maxSites:    2,
		bindings:    make(map[string]string),
	}
}

func TestCheckKeyLicenseSiteCreatesAndReusesBinding(t *testing.T) {
	state := newLicenseSiteTestState()
	db := openLicenseSiteTestDB(t, state)

	if err := checkKeyLicenseSite(db, 1, "HTTPS://Example.COM/install", "192.0.2.10", true); err != nil {
		t.Fatalf("first binding failed: %v", err)
	}
	if err := checkKeyLicenseSite(db, 1, "EXAMPLE.COM.", "192.0.2.11", true); err != nil {
		t.Fatalf("repeat binding failed: %v", err)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.insertCount != 1 {
		t.Fatalf("insert count = %d, want 1", state.insertCount)
	}
	if state.updateCount != 1 {
		t.Fatalf("update count = %d, want 1 for the reused binding", state.updateCount)
	}
	if len(state.bindings) != 1 {
		t.Fatalf("binding count = %d, want 1", len(state.bindings))
	}
}

func TestCheckKeyLicenseSiteIPFallback(t *testing.T) {
	state := newLicenseSiteTestState()
	db := openLicenseSiteTestDB(t, state)

	if err := checkKeyLicenseSite(db, 1, "", "2001:0db8:0:0:0:0:0:1", true); err != nil {
		t.Fatalf("ip-only binding failed: %v", err)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	key := licenseSiteIdentityKey(1, "ip", "2001:db8::1")
	if _, exists := state.bindings[key]; !exists {
		t.Fatalf("ip binding not stored: %#v", state.bindings)
	}
}

func TestCheckKeyLicenseSiteLimitRejectsNewTargetOnly(t *testing.T) {
	state := newLicenseSiteTestState()
	state.maxSites = 1
	db := openLicenseSiteTestDB(t, state)

	if err := checkKeyLicenseSite(db, 1, "a.example.com", "192.0.2.10", true); err != nil {
		t.Fatalf("first binding failed: %v", err)
	}
	if err := checkKeyLicenseSite(db, 1, "b.example.com", "192.0.2.11", true); !errors.Is(err, errLicenseSiteLimitReached) {
		t.Fatalf("second binding error = %v, want site_limit_exceeded", err)
	}
	if err := checkKeyLicenseSite(db, 1, "a.example.com", "192.0.2.12", true); err != nil {
		t.Fatalf("existing binding rejected after limit: %v", err)
	}
}

func TestCheckKeyLicenseSiteV1CannotCreateNewBinding(t *testing.T) {
	state := newLicenseSiteTestState()
	db := openLicenseSiteTestDB(t, state)

	if err := checkKeyLicenseSite(db, 1, "new.example.com", "192.0.2.10", false); !errors.Is(err, errLicenseSiteNotBound) {
		t.Fatalf("v1 unknown target error = %v, want site_not_bound", err)
	}
	if err := checkKeyLicenseSite(db, 1, "existing.example.com", "192.0.2.11", true); err != nil {
		t.Fatalf("create existing binding failed: %v", err)
	}
	if err := checkKeyLicenseSite(db, 1, "existing.example.com", "192.0.2.12", false); err != nil {
		t.Fatalf("v1 known target rejected: %v", err)
	}
}

func TestCheckKeyLicenseSiteUnbindThenRebind(t *testing.T) {
	state := newLicenseSiteTestState()
	state.maxSites = 1
	db := openLicenseSiteTestDB(t, state)

	if err := checkKeyLicenseSite(db, 1, "a.example.com", "192.0.2.10", true); err != nil {
		t.Fatalf("first binding failed: %v", err)
	}
	state.mu.Lock()
	state.bindings = make(map[string]string)
	state.mu.Unlock()
	if err := checkKeyLicenseSite(db, 1, "b.example.com", "192.0.2.11", true); err != nil {
		t.Fatalf("rebind after unbind failed: %v", err)
	}
}

func TestCheckKeyLicenseSiteConcurrentDuplicateInsertRetries(t *testing.T) {
	state := newLicenseSiteTestState()
	state.insertFailures = 1
	db := openLicenseSiteTestDB(t, state)

	if err := checkKeyLicenseSite(db, 1, "race.example.com", "192.0.2.10", true); err != nil {
		t.Fatalf("duplicate-key retry failed: %v", err)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.insertCount != 1 {
		t.Fatalf("insert count = %d, want 1 after retry", state.insertCount)
	}
}
