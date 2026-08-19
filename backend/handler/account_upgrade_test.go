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
	"sync/atomic"
	"testing"
)

type upgradeDBStep struct {
	op            string
	queryContains []string
	columns       []string
	rows          [][]driver.Value
	result        driver.Result
	err           error
	checkArgs     func([]driver.NamedValue) error
}

type upgradeDBScript struct {
	mu    sync.Mutex
	steps []upgradeDBStep
	err   error
}

type upgradeTestDriver struct {
	script *upgradeDBScript
}

type upgradeTestConn struct {
	script *upgradeDBScript
}

type upgradeTestTx struct {
	script *upgradeDBScript
}

type upgradeTestRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

type upgradeTestResult struct {
	lastInsertID int64
	rowsAffected int64
}

var upgradeTestDriverID atomic.Uint64

func (d *upgradeTestDriver) Open(string) (driver.Conn, error) {
	return &upgradeTestConn{script: d.script}, nil
}

func (c *upgradeTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported by upgrade test driver")
}

func (c *upgradeTestConn) Close() error { return nil }

func (c *upgradeTestConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *upgradeTestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	if err := c.script.consume("begin", "", nil); err != nil {
		return nil, err
	}
	return &upgradeTestTx{script: c.script}, nil
}

func (c *upgradeTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	step, err := c.script.consumeStep("query", query, args)
	if err != nil {
		return nil, err
	}
	if step.err != nil {
		return nil, step.err
	}
	return &upgradeTestRows{columns: step.columns, rows: step.rows}, nil
}

func (c *upgradeTestConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	step, err := c.script.consumeStep("exec", query, args)
	if err != nil {
		return nil, err
	}
	if step.err != nil {
		return nil, step.err
	}
	if step.result == nil {
		return upgradeTestResult{rowsAffected: 1}, nil
	}
	return step.result, nil
}

func (tx *upgradeTestTx) Commit() error {
	return tx.script.consume("commit", "", nil)
}

func (tx *upgradeTestTx) Rollback() error {
	return tx.script.consume("rollback", "", nil)
}

func (r *upgradeTestRows) Columns() []string { return r.columns }
func (r *upgradeTestRows) Close() error      { return nil }

func (r *upgradeTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func (r upgradeTestResult) LastInsertId() (int64, error) { return r.lastInsertID, nil }
func (r upgradeTestResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }

func (s *upgradeDBScript) consume(op, query string, args []driver.NamedValue) error {
	_, err := s.consumeStep(op, query, args)
	return err
}

func (s *upgradeDBScript) consumeStep(op, query string, args []driver.NamedValue) (upgradeDBStep, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return upgradeDBStep{}, s.err
	}
	if len(s.steps) == 0 {
		s.err = fmt.Errorf("unexpected %s: %s", op, compactSQL(query))
		return upgradeDBStep{}, s.err
	}
	step := s.steps[0]
	s.steps = s.steps[1:]
	if step.op != op {
		s.err = fmt.Errorf("got %s, want %s; query=%s", op, step.op, compactSQL(query))
		return upgradeDBStep{}, s.err
	}
	compact := compactSQL(query)
	for _, fragment := range step.queryContains {
		if !strings.Contains(compact, compactSQL(fragment)) {
			s.err = fmt.Errorf("query %q does not contain %q", compact, compactSQL(fragment))
			return upgradeDBStep{}, s.err
		}
	}
	if step.checkArgs != nil {
		if err := step.checkArgs(args); err != nil {
			s.err = err
			return upgradeDBStep{}, err
		}
	}
	return step, nil
}

func compactSQL(query string) string {
	return strings.Join(strings.Fields(query), " ")
}

func openUpgradeTestDB(t *testing.T, steps ...upgradeDBStep) (*sql.DB, *upgradeDBScript) {
	t.Helper()
	script := &upgradeDBScript{steps: append([]upgradeDBStep(nil), steps...)}
	name := fmt.Sprintf("account-upgrade-test-%d", upgradeTestDriverID.Add(1))
	sql.Register(name, &upgradeTestDriver{script: script})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db, script
}

func assertUpgradeScriptComplete(t *testing.T, script *upgradeDBScript) {
	t.Helper()
	script.mu.Lock()
	defer script.mu.Unlock()
	if script.err != nil {
		t.Fatalf("database script failed: %v", script.err)
	}
	if len(script.steps) != 0 {
		t.Fatalf("database script has %d unconsumed steps; next operation is %s", len(script.steps), script.steps[0].op)
	}
}

func upgradeOrderStep(status, amount string) upgradeDBStep {
	return upgradeOrderBonusStep(status, amount, "0.00")
}

func upgradeOrderBonusStep(status, amount, openingBonus string) upgradeDBStep {
	return upgradeOrderPaymentBonusStep(status, amount, openingBonus, "balance", "balance", "")
}

func upgradeOrderPaymentStep(status, amount, payChannel, payMethod, gatewayTradeNo string) upgradeDBStep {
	return upgradeOrderPaymentBonusStep(status, amount, "0.00", payChannel, payMethod, gatewayTradeNo)
}

func upgradeOrderPaymentBonusStep(status, amount, openingBonus, payChannel, payMethod, gatewayTradeNo string) upgradeDBStep {
	return upgradeDBStep{
		op:            "query",
		queryContains: []string{"FROM agent_upgrade_orders", "FOR UPDATE"},
		columns:       []string{"id", "order_no", "user_id", "level_id", "level_code_snapshot", "level_name_snapshot", "discount_snapshot", "opening_bonus_snapshot", "amount", "pay_channel", "pay_method", "gateway_trade_no", "status", "agent_id"},
		rows:          [][]driver.Value{{int64(11), "AU-TEST", int64(7), int64(3), "gold", "金牌代理", 8.0, openingBonus, amount, payChannel, payMethod, gatewayTradeNo, status, nil}},
	}
}

func onlinePaymentRecordOrderStep(status, amount, payChannel, payMethod, gatewayTradeNo string) upgradeDBStep {
	return upgradeDBStep{
		op:            "query",
		queryContains: []string{"SELECT id, amount, pay_channel, pay_method", "FROM agent_upgrade_orders", "FOR UPDATE"},
		columns:       []string{"id", "amount", "pay_channel", "pay_method", "gateway_trade_no", "status"},
		rows:          [][]driver.Value{{int64(11), amount, payChannel, payMethod, gatewayTradeNo, status}},
	}
}

func upgradeUserStep(balance string) upgradeDBStep {
	return upgradeDBStep{
		op:            "query",
		queryContains: []string{"FROM users", "FOR UPDATE"},
		columns:       []string{"id", "email", "phone", "password_hash", "nickname", "balance", "account_status", "enabled", "real_name", "real_id_card", "realname_at"},
		rows:          [][]driver.Value{{int64(7), "user@example.com", "13800138000", "hash", "测试用户", balance, "active", true, "测试用户", "ID123", nil}},
	}
}

func upgradeLevelStep(price string) upgradeDBStep {
	return upgradeLevelBonusStep(price, "0.00")
}

func upgradeLevelBonusStep(price, openingBonus string) upgradeDBStep {
	return upgradeDBStep{
		op:            "query",
		queryContains: []string{"FROM agent_levels", "opening_bonus", "FOR UPDATE"},
		columns:       []string{"code", "name", "discount", "upgrade_price", "opening_bonus", "enabled", "self_service_enabled"},
		rows:          [][]driver.Value{{"gold", "金牌代理", 8.0, price, openingBonus, true, true}},
	}
}

func upgradeSettlementLevelStep(price string) upgradeDBStep {
	return upgradeDBStep{
		op:            "query",
		queryContains: []string{"FROM agent_levels", "FOR UPDATE"},
		columns:       []string{"code", "name", "discount", "upgrade_price", "enabled", "self_service_enabled"},
		rows:          [][]driver.Value{{"gold", "金牌代理", 8.0, price, true, true}},
	}
}

func countStep(fragment string, count int64) upgradeDBStep {
	return upgradeDBStep{
		op:            "query",
		queryContains: []string{fragment},
		columns:       []string{"count"},
		rows:          [][]driver.Value{{count}},
	}
}

func execStep(fragment string, rowsAffected int64) upgradeDBStep {
	return upgradeDBStep{
		op:            "exec",
		queryContains: []string{fragment},
		result:        upgradeTestResult{rowsAffected: rowsAffected},
	}
}

func TestSettleAgentUpgradeInsufficientFundsRollsBack(t *testing.T) {
	db, script := openUpgradeTestDB(t,
		upgradeDBStep{op: "begin"},
		upgradeOrderStep("pending", "30.00"),
		upgradeUserStep("20.00"),
		upgradeDBStep{op: "rollback"},
	)

	_, err := settleAgentUpgradeOrder(db, "AU-TEST", accountUpgradePayment{PaidCents: 3000, DeductOpeningFee: true})
	if !errors.Is(err, errAgentUpgradeInsufficientFunds) {
		t.Fatalf("error = %v, want insufficient funds", err)
	}
	assertUpgradeScriptComplete(t, script)
}

func TestSettleAgentUpgradeAccountConflictRollsBack(t *testing.T) {
	db, script := openUpgradeTestDB(t,
		upgradeDBStep{op: "begin"},
		upgradeOrderStep("pending", "30.00"),
		upgradeUserStep("100.00"),
		upgradeSettlementLevelStep("30.00"),
		countStep("FROM agents", 1),
		upgradeDBStep{op: "rollback"},
	)

	_, err := settleAgentUpgradeOrder(db, "AU-TEST", accountUpgradePayment{PaidCents: 3000, DeductOpeningFee: true})
	if !errors.Is(err, errAgentUpgradeAccountConflict) {
		t.Fatalf("error = %v, want account conflict", err)
	}
	assertUpgradeScriptComplete(t, script)
}

func TestSettleAgentUpgradeMigrationFailureRollsBack(t *testing.T) {
	migrationErr := errors.New("license update failed")
	db, script := openUpgradeTestDB(t,
		upgradeDBStep{op: "begin"},
		upgradeOrderStep("pending", "30.00"),
		upgradeUserStep("100.00"),
		upgradeSettlementLevelStep("30.00"),
		countStep("FROM agents", 0),
		countStep("FROM license_purchase_orders", 0),
		execStep("UPDATE agent_upgrade_orders SET status = 'processing'", 1),
		upgradeDBStep{
			op:            "exec",
			queryContains: []string{"INSERT INTO agents"},
			result:        upgradeTestResult{lastInsertID: 501, rowsAffected: 1},
		},
		upgradeDBStep{op: "exec", queryContains: []string{"UPDATE licenses SET owner_type = 'agent'"}, err: migrationErr},
		upgradeDBStep{op: "rollback"},
	)

	_, err := settleAgentUpgradeOrder(db, "AU-TEST", accountUpgradePayment{PaidCents: 3000, DeductOpeningFee: true})
	if !errors.Is(err, migrationErr) {
		t.Fatalf("error = %v, want migration failure", err)
	}
	assertUpgradeScriptComplete(t, script)
}

func TestSettleCompletedAgentUpgradeIsIdempotent(t *testing.T) {
	completedConversion := upgradeDBStep{
		op:            "query",
		queryContains: []string{"FROM account_conversions", "c.status = 'completed'"},
		columns:       []string{"user_id", "agent_id", "email", "level_id", "level_code_snapshot", "level_name_snapshot", "opening_fee", "transferred_balance", "opening_bonus", "migrated_license_count"},
		rows:          [][]driver.Value{{int64(7), int64(501), "user@example.com", int64(3), "gold", "金牌代理", "30.00", "70.00", "10.00", int64(2)}},
	}
	db, script := openUpgradeTestDB(t,
		upgradeDBStep{op: "begin"},
		upgradeOrderStep("completed", "30.00"),
		completedConversion,
		upgradeDBStep{op: "commit"},
	)

	result, err := settleAgentUpgradeOrder(db, "AU-TEST", accountUpgradePayment{PaidCents: 3000, DeductOpeningFee: true})
	if err != nil {
		t.Fatalf("settle completed order: %v", err)
	}
	if result.AgentID != 501 || result.TransferredCents != 7000 || result.OpeningBonusCents != 1000 || result.FinalBalanceCents != 8000 || result.MigratedLicenseCount != 2 {
		t.Fatalf("unexpected completed result: %#v", result)
	}
	assertUpgradeScriptComplete(t, script)
}

func TestBalanceAgentUpgradeAppliesOrderBonusSnapshot(t *testing.T) {
	consumeCheck := func(args []driver.NamedValue) error {
		if len(args) != 6 || args[1].Value != int64(7) || args[2].Value != "-30.00" || args[3].Value != "70.00" || args[4].Value != int64(11) {
			return fmt.Errorf("unexpected opening fee arguments: %#v", args)
		}
		return nil
	}
	userTransferCheck := func(args []driver.NamedValue) error {
		if len(args) != 4 || args[1].Value != int64(7) || args[2].Value != "-70.00" || args[3].Value != int64(11) {
			return fmt.Errorf("unexpected user transfer arguments: %#v", args)
		}
		return nil
	}
	agentTransferCheck := func(args []driver.NamedValue) error {
		if len(args) != 5 || args[1].Value != int64(501) || args[2].Value != "70.00" || args[3].Value != "70.00" || args[4].Value != int64(11) {
			return fmt.Errorf("unexpected agent transfer arguments: %#v", args)
		}
		return nil
	}
	bonusCheck := func(args []driver.NamedValue) error {
		if len(args) != 6 || args[0].Value != "AB11" || args[1].Value != int64(501) || args[2].Value != "25.00" || args[3].Value != "95.00" || args[4].Value != int64(11) {
			return fmt.Errorf("unexpected opening bonus arguments: %#v", args)
		}
		return nil
	}

	db, script := openUpgradeTestDB(t,
		upgradeDBStep{op: "begin"},
		upgradeOrderBonusStep("pending", "30.00", "25.00"),
		upgradeUserStep("100.00"),
		upgradeSettlementLevelStep("30.00"),
		countStep("FROM agents", 0),
		countStep("FROM license_purchase_orders", 0),
		execStep("UPDATE agent_upgrade_orders SET status = 'processing'", 1),
		upgradeDBStep{
			op:            "exec",
			queryContains: []string{"INSERT INTO agents"},
			result:        upgradeTestResult{lastInsertID: 501, rowsAffected: 1},
			checkArgs: func(args []driver.NamedValue) error {
				if len(args) < 7 || args[6].Value != "95.00" {
					return fmt.Errorf("opening bonus not included in agent balance: %#v", args)
				}
				return nil
			},
		},
		execStep("UPDATE licenses SET owner_type = 'agent'", 2),
		execStep("UPDATE license_purchase_orders SET owner_type = 'agent'", 1),
		upgradeDBStep{op: "exec", queryContains: []string{"INSERT INTO transactions", "'consume'"}, result: upgradeTestResult{rowsAffected: 1}, checkArgs: consumeCheck},
		upgradeDBStep{op: "exec", queryContains: []string{"INSERT INTO transactions", "'user'", "'transfer'"}, result: upgradeTestResult{rowsAffected: 1}, checkArgs: userTransferCheck},
		upgradeDBStep{op: "exec", queryContains: []string{"INSERT INTO transactions", "'agent'", "'transfer'"}, result: upgradeTestResult{rowsAffected: 1}, checkArgs: agentTransferCheck},
		upgradeDBStep{op: "exec", queryContains: []string{"INSERT INTO transactions", "'bonus'", "'agent_upgrade_bonus'"}, result: upgradeTestResult{rowsAffected: 1}, checkArgs: bonusCheck},
		execStep("UPDATE user_password_resets", 1),
		execStep("UPDATE user_email_codes", 1),
		upgradeDBStep{
			op:            "exec",
			queryContains: []string{"UPDATE users", "balance = 0", "account_status = 'converted'", "enabled = 0"},
			result:        upgradeTestResult{rowsAffected: 1},
			checkArgs: func(args []driver.NamedValue) error {
				if len(args) != 3 || args[0].Value != int64(501) || args[2].Value != int64(7) {
					return fmt.Errorf("unexpected converted user arguments: %#v", args)
				}
				return nil
			},
		},
		upgradeDBStep{
			op:            "exec",
			queryContains: []string{"INSERT INTO account_conversions", "opening_bonus"},
			result:        upgradeTestResult{rowsAffected: 1},
			checkArgs: func(args []driver.NamedValue) error {
				if len(args) != 12 || args[7].Value != "25.00" {
					return fmt.Errorf("unexpected conversion bonus audit arguments: %#v", args)
				}
				return nil
			},
		},
		upgradeDBStep{
			op:            "exec",
			queryContains: []string{"UPDATE agent_upgrade_orders", "status = 'completed'"},
			result:        upgradeTestResult{rowsAffected: 1},
			checkArgs: func(args []driver.NamedValue) error {
				if len(args) != 9 || args[0].Value != "30.00" || args[1].Value != "balance" || args[2].Value != "balance" || args[5].Value != int64(501) || args[8].Value != int64(11) {
					return fmt.Errorf("unexpected completed order arguments: %#v", args)
				}
				return nil
			},
		},
		upgradeDBStep{op: "commit"},
	)

	result, err := settleAgentUpgradeOrder(db, "AU-TEST", accountUpgradePayment{
		PaidCents:        3000,
		PayChannel:       "balance",
		PayMethod:        "balance",
		DeductOpeningFee: true,
	})
	if err != nil {
		t.Fatalf("settle balance upgrade: %v", err)
	}
	if result.OpeningFeeCents+result.TransferredCents != 10000 {
		t.Fatalf("user funds not conserved: fee=%d transferred=%d", result.OpeningFeeCents, result.TransferredCents)
	}
	if result.AgentID != 501 || result.OpeningBonusCents != 2500 || result.FinalBalanceCents != 9500 || result.MigratedLicenseCount != 2 {
		t.Fatalf("unexpected bonus conversion result = %#v", result)
	}
	assertUpgradeScriptComplete(t, script)
}

func TestInsertAgentUpgradeOrderStoresOpeningBonusSnapshot(t *testing.T) {
	db, script := openUpgradeTestDB(t,
		upgradeDBStep{op: "begin"},
		upgradeDBStep{
			op:            "exec",
			queryContains: []string{"INSERT INTO agent_upgrade_orders", "opening_bonus_snapshot"},
			result:        upgradeTestResult{rowsAffected: 1},
			checkArgs: func(args []driver.NamedValue) error {
				if len(args) != 11 || args[6].Value != "25.00" || args[7].Value != "30.00" {
					return fmt.Errorf("unexpected upgrade order snapshot arguments: %#v", args)
				}
				return nil
			},
		},
		upgradeDBStep{op: "commit"},
	)
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin order snapshot transaction: %v", err)
	}
	if err := insertAgentUpgradeOrder(tx, accountUpgradeOrder{
		OrderNo: "AU-TEST", UserID: 7, LevelID: 3, LevelCode: "gold", LevelName: "金牌代理",
		Discount: 8, OpeningBonusText: "25.00", AmountText: "30.00", PayChannel: "balance", PayMethod: "balance",
	}); err != nil {
		t.Fatalf("insert upgrade order: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit order snapshot transaction: %v", err)
	}
	assertUpgradeScriptComplete(t, script)
}

func TestValidateAgentLevelOpeningBonus(t *testing.T) {
	if price, bonus, msg := validateAgentLevelSelfService(true, 30, 25.25); msg != "" || price != 3000 || bonus != 2525 {
		t.Fatalf("valid amounts rejected: price=%d bonus=%d msg=%q", price, bonus, msg)
	}
	for _, tt := range []struct {
		name  string
		bonus float64
	}{
		{name: "negative", bonus: -0.01},
		{name: "more than two decimals", bonus: 0.001},
		{name: "above decimal limit", bonus: 10000000000},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, msg := validateAgentLevelSelfService(true, 30, tt.bonus); msg == "" {
				t.Fatalf("invalid opening bonus %v was accepted", tt.bonus)
			}
		})
	}
}

func TestCreateBalanceAgentUpgradeRejectsConcurrentActiveOrder(t *testing.T) {
	userLock := upgradeDBStep{
		op:            "query",
		queryContains: []string{"FROM users", "FOR UPDATE"},
		columns:       []string{"account_status", "enabled", "converted_agent_id"},
		rows:          [][]driver.Value{{"active", true, nil}},
	}
	db, script := openUpgradeTestDB(t,
		upgradeDBStep{op: "begin"},
		userLock,
		upgradeLevelStep("30.00"),
		countStep("FROM agent_upgrade_orders", 1),
		upgradeDBStep{op: "rollback"},
	)

	_, err := createBalanceAgentUpgrade(db, 7, 3)
	if !errors.Is(err, errAgentUpgradePendingOrder) {
		t.Fatalf("error = %v, want pending order conflict", err)
	}
	assertUpgradeScriptComplete(t, script)
}

func TestAgentDeletionProtected(t *testing.T) {
	tests := []struct {
		name           string
		source         string
		originalUserID sql.NullInt64
		want           bool
	}{
		{name: "self-service source", source: "user_upgrade", want: true},
		{name: "linked original user", source: "admin", originalUserID: sql.NullInt64{Int64: 7, Valid: true}, want: true},
		{name: "ordinary admin agent", source: "admin", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := agentDeletionProtected(tt.source, tt.originalUserID); got != tt.want {
				t.Fatalf("agentDeletionProtected() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCancelPendingAgentUpgradeOrder(t *testing.T) {
	for _, tt := range []struct {
		name          string
		rowsAffected  int64
		wantCancelled bool
	}{
		{name: "pending online order", rowsAffected: 1, wantCancelled: true},
		{name: "non-pending order", rowsAffected: 0, wantCancelled: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, script := openUpgradeTestDB(t, upgradeDBStep{
				op:            "exec",
				queryContains: []string{"SET status = 'cancelled'", "status = 'pending'", "pay_channel <> 'balance'"},
				result:        upgradeTestResult{rowsAffected: tt.rowsAffected},
				checkArgs: func(args []driver.NamedValue) error {
					if len(args) != 2 || args[0].Value != "AU-TEST" || args[1].Value != int64(7) {
						return fmt.Errorf("unexpected cancel arguments: %#v", args)
					}
					return nil
				},
			})

			cancelled, err := cancelPendingAgentUpgradeOrder(db, 7, "AU-TEST")
			if err != nil {
				t.Fatalf("cancel order: %v", err)
			}
			if cancelled != tt.wantCancelled {
				t.Fatalf("cancelled = %v, want %v", cancelled, tt.wantCancelled)
			}
			assertUpgradeScriptComplete(t, script)
		})
	}
}

func TestParseOnlinePaySelectionSupportsExplicitAndLegacyCodes(t *testing.T) {
	tests := []struct {
		value       string
		wantChannel string
		wantType    string
		wantOK      bool
	}{
		{value: "easypay:alipay", wantChannel: payChannelEpayV1, wantType: "alipay", wantOK: true},
		{value: "easypay-v2:wxpay", wantChannel: payChannelEpayV2, wantType: "wxpay", wantOK: true},
		{value: "qqpay", wantType: "qqpay", wantOK: true},
		{value: "unknown:alipay", wantOK: false},
		{value: "easypay:unknown", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, ok := parseOnlinePaySelection(tt.value)
			if ok != tt.wantOK || got.Channel != tt.wantChannel || got.PayType != tt.wantType {
				t.Fatalf("parseOnlinePaySelection(%q) = %#v, %v", tt.value, got, ok)
			}
		})
	}
}

func TestRecordAgentUpgradeOnlinePaymentIsIdempotent(t *testing.T) {
	db, script := openUpgradeTestDB(t,
		upgradeDBStep{op: "begin"},
		onlinePaymentRecordOrderStep("pending", "30.00", payChannelEpayV1, "alipay", ""),
		countStep("WHERE gateway_trade_no = ?", 0),
		upgradeDBStep{
			op:            "exec",
			queryContains: []string{"SET status = 'paid'", "gateway_trade_no = ?"},
			result:        upgradeTestResult{rowsAffected: 1},
			checkArgs: func(args []driver.NamedValue) error {
				if len(args) != 4 || args[0].Value != "30.00" || args[1].Value != "TRADE-1" || args[2].Value != "payload" || args[3].Value != int64(11) {
					return fmt.Errorf("unexpected paid order arguments: %#v", args)
				}
				return nil
			},
		},
		upgradeDBStep{op: "commit"},
		upgradeDBStep{op: "begin"},
		onlinePaymentRecordOrderStep("paid", "30.00", payChannelEpayV1, "alipay", "TRADE-1"),
		upgradeDBStep{op: "commit"},
	)

	for i := 0; i < 2; i++ {
		if err := recordAgentUpgradeOnlinePayment(db, "AU-TEST", 3000, payChannelEpayV1, "alipay", "TRADE-1", "payload"); err != nil {
			t.Fatalf("record callback %d: %v", i+1, err)
		}
	}
	assertUpgradeScriptComplete(t, script)
}

func TestRecordAgentUpgradeOnlinePaymentRejectsMismatches(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		paidCents int64
		channel   string
		payMethod string
		tradeNo   string
	}{
		{name: "amount", status: "pending", paidCents: 2999, channel: payChannelEpayV1, payMethod: "alipay", tradeNo: "TRADE-1"},
		{name: "channel", status: "pending", paidCents: 3000, channel: payChannelEpayV2, payMethod: "alipay", tradeNo: "TRADE-1"},
		{name: "method", status: "pending", paidCents: 3000, channel: payChannelEpayV1, payMethod: "wxpay", tradeNo: "TRADE-1"},
		{name: "trade number", status: "paid", paidCents: 3000, channel: payChannelEpayV1, payMethod: "alipay", tradeNo: "TRADE-2"},
		{name: "cancelled order", status: "cancelled", paidCents: 3000, channel: payChannelEpayV1, payMethod: "alipay", tradeNo: "TRADE-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storedTradeNo := ""
			if tt.name == "trade number" {
				storedTradeNo = "TRADE-1"
			}
			db, script := openUpgradeTestDB(t,
				upgradeDBStep{op: "begin"},
				onlinePaymentRecordOrderStep(tt.status, "30.00", payChannelEpayV1, "alipay", storedTradeNo),
				upgradeDBStep{op: "rollback"},
			)
			if err := recordAgentUpgradeOnlinePayment(db, "AU-TEST", tt.paidCents, tt.channel, tt.payMethod, tt.tradeNo, "payload"); err == nil {
				t.Fatal("mismatched callback was accepted")
			}
			assertUpgradeScriptComplete(t, script)
		})
	}
}

func TestRecordAgentUpgradeOnlinePaymentRejectsDuplicateGatewayTrade(t *testing.T) {
	db, script := openUpgradeTestDB(t,
		upgradeDBStep{op: "begin"},
		onlinePaymentRecordOrderStep("pending", "30.00", payChannelEpayV1, "alipay", ""),
		countStep("WHERE gateway_trade_no = ?", 1),
		upgradeDBStep{op: "rollback"},
	)
	if err := recordAgentUpgradeOnlinePayment(db, "AU-TEST", 3000, payChannelEpayV1, "alipay", "TRADE-1", "payload"); !errors.Is(err, errAgentUpgradePaymentUnavailable) {
		t.Fatalf("error = %v, want duplicate gateway trade rejection", err)
	}
	assertUpgradeScriptComplete(t, script)
}

func TestOnlineAgentUpgradeFailureRemainsPaidAndAuditable(t *testing.T) {
	db, script := openUpgradeTestDB(t,
		upgradeDBStep{op: "begin"},
		onlinePaymentRecordOrderStep("pending", "30.00", payChannelEpayV2, "wxpay", ""),
		countStep("WHERE gateway_trade_no = ?", 0),
		execStep("SET status = 'paid'", 1),
		upgradeDBStep{op: "commit"},
		upgradeDBStep{op: "begin"},
		upgradeOrderPaymentStep("paid", "30.00", payChannelEpayV2, "wxpay", "TRADE-2"),
		upgradeUserStep("100.00"),
		countStep("FROM agents", 1),
		upgradeDBStep{op: "rollback"},
		upgradeDBStep{
			op:            "exec",
			queryContains: []string{"SET error_message = ?", "status = 'paid'"},
			result:        upgradeTestResult{rowsAffected: 1},
			checkArgs: func(args []driver.NamedValue) error {
				if len(args) != 2 || !strings.Contains(fmt.Sprint(args[0].Value), "账户转换失败") || args[1].Value != "AU-TEST" {
					return fmt.Errorf("unexpected audit failure arguments: %#v", args)
				}
				return nil
			},
		},
	)

	err := settleAgentUpgradeOnlinePayment(db, "AU-TEST", 3000, payChannelEpayV2, "wxpay", "TRADE-2", "payload")
	if !errors.Is(err, errAgentUpgradeAccountConflict) {
		t.Fatalf("error = %v, want account conflict", err)
	}
	assertUpgradeScriptComplete(t, script)
}

func TestOnlineAgentUpgradePreservesUserBalance(t *testing.T) {
	db, script := openUpgradeTestDB(t,
		upgradeDBStep{op: "begin"},
		onlinePaymentRecordOrderStep("pending", "30.00", payChannelEpayV1, "alipay", ""),
		countStep("WHERE gateway_trade_no = ?", 0),
		execStep("SET status = 'paid'", 1),
		upgradeDBStep{op: "commit"},
		upgradeDBStep{op: "begin"},
		upgradeOrderPaymentBonusStep("paid", "30.00", "25.00", payChannelEpayV1, "alipay", "TRADE-1"),
		upgradeUserStep("100.00"),
		countStep("FROM agents", 0),
		countStep("FROM license_purchase_orders", 0),
		execStep("UPDATE agent_upgrade_orders SET status = 'processing'", 1),
		upgradeDBStep{
			op:            "exec",
			queryContains: []string{"INSERT INTO agents"},
			result:        upgradeTestResult{lastInsertID: 501, rowsAffected: 1},
			checkArgs: func(args []driver.NamedValue) error {
				if len(args) < 7 || args[6].Value != "125.00" {
					return fmt.Errorf("online conversion did not include opening bonus: %#v", args)
				}
				return nil
			},
		},
		execStep("UPDATE licenses SET owner_type = 'agent'", 2),
		execStep("UPDATE license_purchase_orders SET owner_type = 'agent'", 1),
		upgradeDBStep{
			op:            "exec",
			queryContains: []string{"INSERT INTO transactions", "'consume'"},
			result:        upgradeTestResult{rowsAffected: 1},
			checkArgs: func(args []driver.NamedValue) error {
				if len(args) != 6 || args[2].Value != "-30.00" || args[3].Value != nil {
					return fmt.Errorf("unexpected external payment transaction: %#v", args)
				}
				return nil
			},
		},
		execStep("'user', ?, 'transfer'", 1),
		execStep("'agent', ?, 'transfer'", 1),
		upgradeDBStep{
			op:            "exec",
			queryContains: []string{"'agent', ?, 'bonus'", "'agent_upgrade_bonus'"},
			result:        upgradeTestResult{rowsAffected: 1},
			checkArgs: func(args []driver.NamedValue) error {
				if len(args) != 6 || args[2].Value != "25.00" || args[3].Value != "125.00" {
					return fmt.Errorf("unexpected online bonus transaction: %#v", args)
				}
				return nil
			},
		},
		execStep("UPDATE user_password_resets", 1),
		execStep("UPDATE user_email_codes", 1),
		execStep("UPDATE users", 1),
		execStep("INSERT INTO account_conversions", 1),
		upgradeDBStep{
			op:            "exec",
			queryContains: []string{"UPDATE agent_upgrade_orders", "status = 'completed'"},
			result:        upgradeTestResult{rowsAffected: 1},
			checkArgs: func(args []driver.NamedValue) error {
				if len(args) != 9 || args[0].Value != "30.00" || args[1].Value != payChannelEpayV1 || args[2].Value != "alipay" || args[3].Value != "TRADE-1" || args[5].Value != int64(501) {
					return fmt.Errorf("unexpected completed online order arguments: %#v", args)
				}
				return nil
			},
		},
		upgradeDBStep{op: "commit"},
	)

	if err := settleAgentUpgradeOnlinePayment(db, "AU-TEST", 3000, payChannelEpayV1, "alipay", "TRADE-1", "payload"); err != nil {
		t.Fatalf("settle online upgrade: %v", err)
	}
	assertUpgradeScriptComplete(t, script)
}
