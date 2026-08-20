package handler

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const licenseCardTestDriverName = "license-card-test"

var (
	registerLicenseCardTestDriver sync.Once
	licenseCardTestStates         sync.Map
	licenseCardTestSequence       atomic.Uint64
)

type licenseCardTestState struct {
	rowMu sync.Mutex
	mu    sync.Mutex

	cardStatus     string
	redeemedType   string
	redeemedID     int64
	licenseID      int64
	batchStatus    string
	appEnabled     bool
	userEnabled    map[int64]bool
	agentEnabled   map[int64]bool
	appName        string
	planName       string
	licenseType    string
	durationDays   int
	maxSites       int
	price          float64
	licenseInserts int
	license        licenseCardTestLicense
	adminCards     []licenseCardTestAdminCard
}

type licenseCardTestAdminCard struct {
	id           int64
	code         string
	status       string
	ownerType    string
	ownerAccount string
	licenseID    int64
	redeemedAt   *time.Time
	createdAt    time.Time
}

type licenseCardTestLicense struct {
	id           int64
	licenseNo    string
	appID        int64
	planID       int64
	price        float64
	licenseType  string
	ownerType    string
	ownerID      int64
	durationDays int
	maxSites     int
	startedAt    time.Time
	expiredAt    *time.Time
	licenseKey   string
}

type licenseCardTestDriver struct{}
type licenseCardTestConn struct {
	state     *licenseCardTestState
	rowLocked bool
}
type licenseCardTestTx struct{ conn *licenseCardTestConn }
type licenseCardTestResult struct {
	id       int64
	affected int64
}
type licenseCardTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func newLicenseCardTestState(licenseType string) *licenseCardTestState {
	return &licenseCardTestState{
		cardStatus:   licenseCardStatusUnused,
		batchStatus:  licenseCardBatchActive,
		appEnabled:   true,
		userEnabled:  map[int64]bool{11: true, 12: true},
		agentEnabled: map[int64]bool{21: true, 22: true},
		appName:      "快照应用",
		planName:     "快照套餐",
		licenseType:  licenseType,
		durationDays: 30,
		price:        88.50,
	}
}

func (licenseCardTestDriver) Open(name string) (driver.Conn, error) {
	value, ok := licenseCardTestStates.Load(name)
	if !ok {
		return nil, errors.New("license card test state not found")
	}
	return &licenseCardTestConn{state: value.(*licenseCardTestState)}, nil
}

func (conn *licenseCardTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}
func (conn *licenseCardTestConn) Close() error { return nil }
func (conn *licenseCardTestConn) Begin() (driver.Tx, error) {
	return &licenseCardTestTx{conn: conn}, nil
}
func (conn *licenseCardTestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return conn.Begin()
}

func (conn *licenseCardTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "FROM license_cards c") && strings.Contains(query, "FOR UPDATE"):
		conn.state.rowMu.Lock()
		conn.rowLocked = true
		conn.state.mu.Lock()
		defer conn.state.mu.Unlock()
		redeemedID := driver.Value(nil)
		if conn.state.redeemedID != 0 {
			redeemedID = conn.state.redeemedID
		}
		licenseID := driver.Value(nil)
		if conn.state.licenseID != 0 {
			licenseID = conn.state.licenseID
		}
		return &licenseCardTestRows{
			columns: []string{"card_id", "batch_id", "card_status", "redeemed_type", "redeemed_id", "license_id", "batch_status", "app_id", "plan_id", "app_name", "plan_name", "duration_days", "max_sites_snapshot", "price", "license_type", "app_enabled"},
			values: [][]driver.Value{{
				int64(1), int64(2), conn.state.cardStatus, conn.state.redeemedType, redeemedID,
				licenseID, conn.state.batchStatus, int64(3), int64(4), conn.state.appName,
				conn.state.planName, int64(conn.state.durationDays), int64(conn.state.maxSites), conn.state.price,
				conn.state.licenseType, conn.state.appEnabled,
			}},
		}, nil
	case strings.Contains(query, "SELECT COUNT(*) FROM license_cards c"):
		return conn.adminCardCountRows(args)
	case strings.Contains(query, "LEFT JOIN users u ON") && strings.Contains(query, "LEFT JOIN agents a ON"):
		return conn.adminCardListRows(args)
	case strings.Contains(query, "SELECT enabled FROM users"):
		return conn.ownerRows("user", args)
	case strings.Contains(query, "SELECT enabled FROM agents"):
		return conn.ownerRows("agent", args)
	case strings.Contains(query, "SELECT license_no, type, license_key, expired_at FROM licenses"):
		conn.state.mu.Lock()
		defer conn.state.mu.Unlock()
		if conn.state.license.id == 0 {
			return &licenseCardTestRows{columns: []string{"license_no", "type", "license_key", "expired_at"}}, nil
		}
		expiredAt := driver.Value(nil)
		if conn.state.license.expiredAt != nil {
			expiredAt = *conn.state.license.expiredAt
		}
		return &licenseCardTestRows{
			columns: []string{"license_no", "type", "license_key", "expired_at"},
			values:  [][]driver.Value{{conn.state.license.licenseNo, conn.state.license.licenseType, conn.state.license.licenseKey, expiredAt}},
		}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

func (conn *licenseCardTestConn) adminCardCountRows(args []driver.NamedValue) (driver.Rows, error) {
	status := adminCardStatusArg(args)
	conn.state.mu.Lock()
	defer conn.state.mu.Unlock()
	var total int64
	for _, card := range conn.state.adminCards {
		if status == "" || card.status == status {
			total++
		}
	}
	return &licenseCardTestRows{columns: []string{"count"}, values: [][]driver.Value{{total}}}, nil
}

func (conn *licenseCardTestConn) adminCardListRows(args []driver.NamedValue) (driver.Rows, error) {
	status := adminCardStatusArg(args)
	conn.state.mu.Lock()
	defer conn.state.mu.Unlock()
	values := make([][]driver.Value, 0, len(conn.state.adminCards))
	for _, card := range conn.state.adminCards {
		if status != "" && card.status != status {
			continue
		}
		ownerType := driver.Value(nil)
		if card.ownerType != "" {
			ownerType = card.ownerType
		}
		licenseID := driver.Value(nil)
		if card.licenseID != 0 {
			licenseID = card.licenseID
		}
		redeemedAt := driver.Value(nil)
		if card.redeemedAt != nil {
			redeemedAt = *card.redeemedAt
		}
		values = append(values, []driver.Value{
			card.id, card.code, card.status, ownerType, card.ownerAccount,
			licenseID, redeemedAt, card.createdAt,
		})
	}
	return &licenseCardTestRows{
		columns: []string{"id", "card_code", "status", "redeemed_by_type", "redeemed_by_account", "license_id", "redeemed_at", "created_at"},
		values:  values,
	}, nil
}

func adminCardStatusArg(args []driver.NamedValue) string {
	if len(args) >= 2 {
		if status, ok := args[1].Value.(string); ok {
			return status
		}
	}
	return ""
}

func (conn *licenseCardTestConn) ownerRows(ownerType string, args []driver.NamedValue) (driver.Rows, error) {
	if len(args) != 1 {
		return nil, errors.New("owner query missing ID")
	}
	ownerID, ok := args[0].Value.(int64)
	if !ok {
		return nil, errors.New("owner ID is not int64")
	}
	conn.state.mu.Lock()
	defer conn.state.mu.Unlock()
	enabled := conn.state.userEnabled[ownerID]
	if ownerType == "agent" {
		enabled = conn.state.agentEnabled[ownerID]
	}
	if !enabled {
		return &licenseCardTestRows{columns: []string{"enabled"}}, nil
	}
	return &licenseCardTestRows{columns: []string{"enabled"}, values: [][]driver.Value{{true}}}, nil
}

func (conn *licenseCardTestConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	conn.state.mu.Lock()
	defer conn.state.mu.Unlock()

	switch {
	case strings.Contains(query, "INSERT INTO licenses"):
		if len(args) != 13 {
			return nil, errors.New("unexpected license argument count")
		}
		conn.state.licenseInserts++
		conn.state.licenseID = 101
		license := licenseCardTestLicense{
			id:           101,
			licenseNo:    args[0].Value.(string),
			appID:        args[1].Value.(int64),
			planID:       args[2].Value.(int64),
			price:        asFloat64(args[3].Value),
			licenseType:  args[4].Value.(string),
			ownerType:    args[5].Value.(string),
			ownerID:      args[6].Value.(int64),
			durationDays: int(args[7].Value.(int64)),
			startedAt:    args[8].Value.(time.Time),
			licenseKey:   args[10].Value.(string),
			maxSites:     int(args[11].Value.(int64)),
		}
		if value, ok := args[9].Value.(time.Time); ok {
			license.expiredAt = &value
		}
		conn.state.license = license
		return licenseCardTestResult{id: 101, affected: 1}, nil
	case strings.Contains(query, "UPDATE license_cards SET status = 'redeemed'"):
		if conn.state.cardStatus != licenseCardStatusUnused {
			return licenseCardTestResult{affected: 0}, nil
		}
		conn.state.cardStatus = licenseCardStatusRedeemed
		conn.state.redeemedType = args[0].Value.(string)
		conn.state.redeemedID = args[1].Value.(int64)
		conn.state.licenseID = args[2].Value.(int64)
		return licenseCardTestResult{affected: 1}, nil
	case strings.Contains(query, "INSERT INTO operation_logs"):
		return licenseCardTestResult{id: 1, affected: 1}, nil
	default:
		return nil, errors.New("unexpected exec: " + query)
	}
}

func asFloat64(value any) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case int64:
		return float64(number)
	default:
		return 0
	}
}

func (tx *licenseCardTestTx) Commit() error {
	tx.conn.unlockRow()
	return nil
}
func (tx *licenseCardTestTx) Rollback() error {
	tx.conn.unlockRow()
	return nil
}
func (conn *licenseCardTestConn) unlockRow() {
	if conn.rowLocked {
		conn.rowLocked = false
		conn.state.rowMu.Unlock()
	}
}
func (result licenseCardTestResult) LastInsertId() (int64, error) { return result.id, nil }
func (result licenseCardTestResult) RowsAffected() (int64, error) { return result.affected, nil }
func (rows licenseCardTestRows) Columns() []string                { return rows.columns }
func (licenseCardTestRows) Close() error                          { return nil }
func (rows *licenseCardTestRows) Next(dest []driver.Value) error {
	if rows.index >= len(rows.values) {
		return io.EOF
	}
	copy(dest, rows.values[rows.index])
	rows.index++
	return nil
}

func openLicenseCardTestDB(t *testing.T, state *licenseCardTestState) *sql.DB {
	t.Helper()
	registerLicenseCardTestDriver.Do(func() {
		sql.Register(licenseCardTestDriverName, licenseCardTestDriver{})
	})
	name := strings.ReplaceAll(t.Name(), "/", "-") + "-" + string(rune(licenseCardTestSequence.Add(1)))
	licenseCardTestStates.Store(name, state)
	t.Cleanup(func() { licenseCardTestStates.Delete(name) })
	db, err := sql.Open(licenseCardTestDriverName, name)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(8)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestGenerateLicenseCardCodeIsUniqueAndWellFormed(t *testing.T) {
	pattern := regexp.MustCompile(`^AUTH-[A-HJ-NP-Z2-9]{4}(?:-[A-HJ-NP-Z2-9]{4}){3}$`)
	codes := make(map[string]struct{}, 2000)
	for index := 0; index < 2000; index++ {
		code, err := generateLicenseCardCode()
		if err != nil {
			t.Fatal(err)
		}
		if !pattern.MatchString(code) {
			t.Fatalf("unexpected card format: %q", code)
		}
		if _, exists := codes[code]; exists {
			t.Fatalf("duplicate card generated: %q", code)
		}
		codes[code] = struct{}{}
	}
}

func TestRedeemLicenseCardCreatesSnapshotLicense(t *testing.T) {
	tests := []struct {
		name               string
		ownerType          string
		ownerID            int64
		licenseType        string
		wantBindingPending bool
		wantKey            bool
	}{
		{name: "user domain waits for binding", ownerType: "user", ownerID: 11, licenseType: "domain", wantBindingPending: true},
		{name: "agent wildcard waits for binding", ownerType: "agent", ownerID: 21, licenseType: "wildcard", wantBindingPending: true},
		{name: "user ip waits for binding", ownerType: "user", ownerID: 11, licenseType: "ip", wantBindingPending: true},
		{name: "agent key is immediately usable", ownerType: "agent", ownerID: 21, licenseType: "key", wantKey: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := newLicenseCardTestState(test.licenseType)
			db := openLicenseCardTestDB(t, state)

			result, message, err := redeemLicenseCard(db, " auth-abcd-efgh-jkmn-pqrs ", test.ownerType, test.ownerID)
			if err != nil || message != "" {
				t.Fatalf("redeem error=%v message=%q", err, message)
			}
			if result.AppName != state.appName || result.PlanName != state.planName || result.Type != test.licenseType {
				t.Fatalf("redemption did not use batch snapshot: %#v", result)
			}
			if result.BindingPending != test.wantBindingPending {
				t.Fatalf("bindingPending=%v, want %v", result.BindingPending, test.wantBindingPending)
			}
			if (result.LicenseKey != "") != test.wantKey {
				t.Fatalf("licenseKey=%q, want key=%v", result.LicenseKey, test.wantKey)
			}

			state.mu.Lock()
			license := state.license
			inserts := state.licenseInserts
			state.mu.Unlock()
			if inserts != 1 || license.ownerType != test.ownerType || license.ownerID != test.ownerID {
				t.Fatalf("license ownership/inserts invalid: inserts=%d license=%#v", inserts, license)
			}
			if license.appID != 3 || license.planID != 4 || license.price != state.price || license.durationDays != state.durationDays || license.maxSites != state.maxSites {
				t.Fatalf("license snapshot invalid: %#v", license)
			}
			if license.expiredAt == nil {
				t.Fatal("finite plan did not create expiration")
			}
			if got := license.expiredAt.Sub(license.startedAt); got < 29*24*time.Hour || got > 31*24*time.Hour {
				t.Fatalf("expiration duration=%v", got)
			}
		})
	}
}

func TestRedeemLicenseCardConcurrentSingleIssueAndIdempotency(t *testing.T) {
	state := newLicenseCardTestState("key")
	db := openLicenseCardTestDB(t, state)
	type outcome struct {
		result  licenseCardRedemption
		message string
		err     error
	}
	start := make(chan struct{})
	outcomes := make(chan outcome, 2)
	for index := 0; index < 2; index++ {
		go func() {
			<-start
			result, message, err := redeemLicenseCard(db, "AUTH-ABCD-EFGH-JKMN-PQRS", "user", 11)
			outcomes <- outcome{result: result, message: message, err: err}
		}()
	}
	close(start)
	first := <-outcomes
	second := <-outcomes
	for _, current := range []outcome{first, second} {
		if current.err != nil || current.message != "" || current.result.LicenseID != 101 {
			t.Fatalf("concurrent outcome=%#v", current)
		}
	}
	if first.result.Idempotent == second.result.Idempotent {
		t.Fatalf("want one initial result and one idempotent result: first=%v second=%v", first.result.Idempotent, second.result.Idempotent)
	}
	if first.result.LicenseKey == "" || first.result.LicenseKey != second.result.LicenseKey {
		t.Fatalf("idempotent retry returned a different key: %q / %q", first.result.LicenseKey, second.result.LicenseKey)
	}

	state.mu.Lock()
	inserts := state.licenseInserts
	state.mu.Unlock()
	if inserts != 1 {
		t.Fatalf("concurrent redemption created %d licenses", inserts)
	}

	_, message, err := redeemLicenseCard(db, "AUTH-ABCD-EFGH-JKMN-PQRS", "user", 12)
	if err != nil || message != "卡密无效或不可用" {
		t.Fatalf("cross-account retry error=%v message=%q", err, message)
	}
}

func TestLicenseCardRedeemRateLimiter(t *testing.T) {
	limiter := newLicenseCardRateLimiter(10, time.Minute)
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	for attempt := 1; attempt <= 10; attempt++ {
		if !limiter.allow("user:11", now) {
			t.Fatalf("attempt %d was rejected", attempt)
		}
	}
	if limiter.allow("user:11", now) {
		t.Fatal("attempt beyond limit was accepted")
	}
	if !limiter.allow("user:12", now) {
		t.Fatal("one account rate limit affected another account")
	}
	if !limiter.allow("user:11", now.Add(time.Minute)) {
		t.Fatal("rate limit did not reset after the window")
	}
}

func TestAdminLicenseCardDetailsReturnPlaintextAndOwnerAccounts(t *testing.T) {
	createdAt := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	redeemedAt := createdAt.Add(time.Hour)
	state := newLicenseCardTestState("domain")
	state.adminCards = []licenseCardTestAdminCard{
		{id: 1, code: "AUTH-USER-ABCD-EFGH-JKMN", status: licenseCardStatusRedeemed, ownerType: "user", ownerAccount: "user@example.com", licenseID: 101, redeemedAt: &redeemedAt, createdAt: createdAt},
		{id: 2, code: "AUTH-AGENT-ABCD-EFGH-JKMN", status: licenseCardStatusRedeemed, ownerType: "agent", ownerAccount: "agent@example.com", licenseID: 102, redeemedAt: &redeemedAt, createdAt: createdAt},
		{id: 3, code: "AUTH-UNUSED-ABCD-EFGH-JKMN", status: licenseCardStatusUnused, createdAt: createdAt},
	}
	db := openLicenseCardTestDB(t, state)

	list, total, err := loadAdminLicenseCards(db, 2, "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(list) != 3 {
		t.Fatalf("total=%d len=%d, want 3", total, len(list))
	}
	if list[0]["cardCode"] != state.adminCards[0].code || strings.Contains(list[0]["cardCode"].(string), "****") {
		t.Fatalf("user card was not returned as plaintext: %#v", list[0])
	}
	if list[0]["redeemedByType"] != "user" || list[0]["redeemedByAccount"] != "user@example.com" {
		t.Fatalf("user account mapping invalid: %#v", list[0])
	}
	if list[1]["redeemedByType"] != "agent" || list[1]["redeemedByAccount"] != "agent@example.com" {
		t.Fatalf("agent account mapping invalid: %#v", list[1])
	}
	if _, exists := list[0]["redeemedById"]; exists {
		t.Fatalf("detail still exposes owner ID: %#v", list[0])
	}
	if _, exists := list[2]["redeemedByAccount"]; exists {
		t.Fatalf("unused card has redemption account: %#v", list[2])
	}
}

func TestPendingCardLicenseDoesNotMatchSDKRequest(t *testing.T) {
	requests := []struct {
		licenseType string
		request     licenseVerifyRequest
	}{
		{licenseType: "domain", request: licenseVerifyRequest{Domain: "example.com"}},
		{licenseType: "wildcard", request: licenseVerifyRequest{Domain: "sub.example.com"}},
		{licenseType: "ip", request: licenseVerifyRequest{ServerIP: "192.0.2.10"}},
	}
	for _, test := range requests {
		if licenseRowMatchesRequest(test.licenseType, "", test.licenseType == "wildcard", test.request) {
			t.Fatalf("unbound %s license matched SDK request", test.licenseType)
		}
	}
}

func TestRedeemLicenseCardRejectsUnavailableState(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*licenseCardTestState)
	}{
		{name: "disabled card", mutate: func(state *licenseCardTestState) { state.cardStatus = licenseCardStatusDisabled }},
		{name: "disabled batch", mutate: func(state *licenseCardTestState) { state.batchStatus = licenseCardBatchDisabled }},
		{name: "disabled app", mutate: func(state *licenseCardTestState) { state.appEnabled = false }},
		{name: "disabled owner", mutate: func(state *licenseCardTestState) { state.userEnabled[11] = false }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := newLicenseCardTestState("domain")
			test.mutate(state)
			db := openLicenseCardTestDB(t, state)
			_, message, err := redeemLicenseCard(db, "AUTH-ABCD-EFGH-JKMN-PQRS", "user", 11)
			if err != nil || message == "" {
				t.Fatalf("unavailable card error=%v message=%q", err, message)
			}
			state.mu.Lock()
			inserts := state.licenseInserts
			state.mu.Unlock()
			if inserts != 0 {
				t.Fatalf("unavailable card created %d licenses", inserts)
			}
		})
	}
}
