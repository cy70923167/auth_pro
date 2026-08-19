package handler

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

const purchaseLicenseTypeTestDriverName = "purchase-license-type-test"

var registerPurchaseLicenseTypeTestDriver sync.Once
var purchaseLicenseTypeTestStates sync.Map

type purchaseLicenseTypeTestState struct {
	mask                  uint8
	appExists             bool
	backfillPaidOrders    bool
	promotion             *purchasePromotionCandidate
	promotionPlanIDs      map[int64]bool
	agentDiscount         float64
	mu                    sync.Mutex
	execCount             int
	queries               []string
	queryArgs             [][]driver.NamedValue
	execQueries           []string
	execArgs              [][]driver.NamedValue
	failTransactionInsert bool
	purchasePayChannel    string
	purchasePayMethod     string
	purchaseTradeNo       string
	purchaseStatus        string
	commits               int
	rollbacks             int
}

type purchaseLicenseTypeTestDriver struct{}
type purchaseLicenseTypeTestConn struct{ state *purchaseLicenseTypeTestState }
type purchaseLicenseTypeTestTx struct{ state *purchaseLicenseTypeTestState }
type purchaseLicenseTypeTestResult struct{ id int64 }
type purchaseLicenseTypeTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (purchaseLicenseTypeTestDriver) Open(name string) (driver.Conn, error) {
	value, ok := purchaseLicenseTypeTestStates.Load(name)
	if !ok {
		return nil, errors.New("purchase license type test state not found")
	}
	return &purchaseLicenseTypeTestConn{state: value.(*purchaseLicenseTypeTestState)}, nil
}

func (conn *purchaseLicenseTypeTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}

func (conn *purchaseLicenseTypeTestConn) Close() error { return nil }
func (conn *purchaseLicenseTypeTestConn) Begin() (driver.Tx, error) {
	return &purchaseLicenseTypeTestTx{state: conn.state}, nil
}

func (conn *purchaseLicenseTypeTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	conn.state.mu.Lock()
	conn.state.queries = append(conn.state.queries, query)
	conn.state.queryArgs = append(conn.state.queryArgs, append([]driver.NamedValue(nil), args...))
	conn.state.mu.Unlock()
	switch {
	case strings.Contains(query, "information_schema.COLUMNS"):
		return &purchaseLicenseTypeTestRows{
			columns: []string{"count"},
			values:  [][]driver.Value{{int64(1)}},
		}, nil
	case strings.Contains(query, "SELECT purchase_license_type_mask"):
		if !conn.state.appExists {
			return &purchaseLicenseTypeTestRows{columns: []string{"purchase_license_type_mask"}}, nil
		}
		return &purchaseLicenseTypeTestRows{
			columns: []string{"purchase_license_type_mask"},
			values:  [][]driver.Value{{int64(conn.state.mask)}},
		}, nil
	case strings.Contains(query, "FROM license_purchase_orders") && strings.Contains(query, "FOR UPDATE"):
		payChannel := conn.state.purchasePayChannel
		if payChannel == "" {
			payChannel = payChannelEpayV1
		}
		payMethod := conn.state.purchasePayMethod
		if payMethod == "" {
			payMethod = "alipay"
		}
		status := conn.state.purchaseStatus
		if status == "" {
			status = "pending"
		}
		return &purchaseLicenseTypeTestRows{
			columns: []string{"id", "agent_id", "owner_type", "owner_id", "app_id", "plan_id", "type", "target", "amount", "original_amount", "status", "app_name_snapshot", "plan_name_snapshot", "duration_days_snapshot", "pay_channel", "pay_method", "gateway_trade_no"},
			values:  [][]driver.Value{{int64(1), int64(7), "user", int64(42), int64(9), int64(3), "key", "", "10.00", "20.00", status, "Test App", "Test Plan", int64(30), payChannel, payMethod, conn.state.purchaseTradeNo}},
		}, nil
	case strings.Contains(query, "SELECT a.app_name, p.name, p.duration_days"):
		return &purchaseLicenseTypeTestRows{
			columns: []string{"app_name", "name", "duration_days"},
			values:  [][]driver.Value{{"Test App", "Test Plan", int64(30)}},
		}, nil
	case strings.Contains(query, "SELECT p.duration_days, p.price"):
		return &purchaseLicenseTypeTestRows{
			columns: []string{"duration_days", "price"},
			values:  [][]driver.Value{{int64(30), float64(10)}},
		}, nil
	case strings.Contains(query, "FROM license_plans p") && strings.Contains(query, "JOIN apps a"):
		return &purchaseLicenseTypeTestRows{
			columns: []string{"app_name", "name", "price", "duration_days"},
			values:  [][]driver.Value{{"Test App", "Test Plan", float64(10), int64(30)}},
		}, nil
	case strings.Contains(query, "FROM promotion_campaigns pc") && strings.Contains(query, "JOIN promotion_campaign_plans pcp"):
		promotion := conn.state.promotion
		if promotion == nil || len(args) < 3 {
			return &purchaseLicenseTypeTestRows{columns: []string{"id", "name", "audience", "rule_type", "rule_value"}}, nil
		}
		planID, _ := args[1].Value.(int64)
		if conn.state.promotionPlanIDs != nil && !conn.state.promotionPlanIDs[planID] {
			return &purchaseLicenseTypeTestRows{columns: []string{"id", "name", "audience", "rule_type", "rule_value"}}, nil
		}
		buyer, ok := args[2].Value.(string)
		if !ok {
			if value, valid := args[2].Value.(purchaseAudience); valid {
				buyer = string(value)
			}
		}
		if promotion.Audience != purchaseAudienceAll && string(promotion.Audience) != buyer {
			return &purchaseLicenseTypeTestRows{columns: []string{"id", "name", "audience", "rule_type", "rule_value"}}, nil
		}
		ruleType := promotion.RuleType
		valueUnits := promotion.RuleValueUnits
		if ruleType == "" {
			ruleType = promotionRuleFixedPrice
			valueUnits = promotion.AmountCents
		}
		return &purchaseLicenseTypeTestRows{
			columns: []string{"id", "name", "audience", "rule_type", "rule_value"},
			values: [][]driver.Value{{
				promotion.ID, promotion.Name, string(promotion.Audience), string(ruleType), promotionRuleValueText(ruleType, valueUnits),
			}},
		}, nil
	case strings.Contains(query, "SELECT CASE") && strings.Contains(query, "FROM agents a"):
		discount := conn.state.agentDiscount
		if discount == 0 {
			discount = 10
		}
		return &purchaseLicenseTypeTestRows{
			columns: []string{"discount"},
			values:  [][]driver.Value{{discount}},
		}, nil
	case strings.Contains(query, "SELECT enabled FROM users WHERE id"):
		return &purchaseLicenseTypeTestRows{
			columns: []string{"enabled"},
			values:  [][]driver.Value{{true}},
		}, nil
	case strings.Contains(query, "SELECT enabled FROM apps WHERE id"):
		return &purchaseLicenseTypeTestRows{
			columns: []string{"enabled"},
			values:  [][]driver.Value{{true}},
		}, nil
	case strings.Contains(query, "FROM apps") && strings.Contains(query, "purchase_license_type_mask <> 0"):
		return &purchaseLicenseTypeTestRows{
			columns: []string{"id", "app_name", "description", "icon", "purchase_license_type_mask"},
		}, nil
	case strings.Contains(query, "SELECT balance FROM users"):
		return &purchaseLicenseTypeTestRows{
			columns: []string{"balance"},
			values:  [][]driver.Value{{float64(100)}},
		}, nil
	case strings.Contains(query, "SELECT balance FROM agents"):
		return &purchaseLicenseTypeTestRows{
			columns: []string{"balance"},
			values:  [][]driver.Value{{float64(100)}},
		}, nil
	case strings.Contains(query, "SELECT enabled FROM agents WHERE id = ?"):
		return &purchaseLicenseTypeTestRows{
			columns: []string{"enabled"},
			values:  [][]driver.Value{{int64(1)}},
		}, nil
	case strings.Contains(query, "COALESCE(NULLIF(nickname, ''), email)") && strings.Contains(query, "FROM users"):
		return &purchaseLicenseTypeTestRows{
			columns: []string{"id", "name", "email"},
			values:  [][]driver.Value{{int64(42), "Alice", "alice@example.com"}, {int64(43), "bob@example.com", "bob@example.com"}},
		}, nil
	case conn.state.backfillPaidOrders && strings.Contains(query, "WHERE o.status = 'paid'"):
		return &purchaseLicenseTypeTestRows{
			columns: []string{"id", "order_no", "agent_id", "owner_type", "owner_id", "pay_method", "paid_amount", "app_name", "plan_name", "paid_at"},
			values:  [][]driver.Value{{int64(5), "LP-history", int64(7), "user", int64(42), "alipay", "10.00", "Test App", "Test Plan", nil}},
		}, nil
	case strings.Contains(query, "SELECT GREATEST(total - used, 0) FROM agent_quotas"):
		return &purchaseLicenseTypeTestRows{
			columns: []string{"remain"},
			values:  [][]driver.Value{{int64(3)}},
		}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

func (conn *purchaseLicenseTypeTestConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	conn.state.mu.Lock()
	defer conn.state.mu.Unlock()
	conn.state.execCount++
	conn.state.execQueries = append(conn.state.execQueries, query)
	copiedArgs := append([]driver.NamedValue(nil), args...)
	conn.state.execArgs = append(conn.state.execArgs, copiedArgs)
	if conn.state.failTransactionInsert && strings.Contains(query, "INSERT INTO transactions") {
		return nil, errors.New("forced transaction insert failure")
	}
	return purchaseLicenseTypeTestResult{id: 1}, nil
}

func (tx *purchaseLicenseTypeTestTx) Commit() error {
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()
	tx.state.commits++
	return nil
}
func (tx *purchaseLicenseTypeTestTx) Rollback() error {
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()
	tx.state.rollbacks++
	return nil
}
func (result purchaseLicenseTypeTestResult) LastInsertId() (int64, error) {
	return result.id, nil
}
func (purchaseLicenseTypeTestResult) RowsAffected() (int64, error) { return 1, nil }
func (rows purchaseLicenseTypeTestRows) Columns() []string         { return rows.columns }
func (purchaseLicenseTypeTestRows) Close() error                   { return nil }
func (rows *purchaseLicenseTypeTestRows) Next(dest []driver.Value) error {
	if rows.index >= len(rows.values) {
		return io.EOF
	}
	copy(dest, rows.values[rows.index])
	rows.index++
	return nil
}

func openPurchaseLicenseTypeTestDB(t *testing.T, state *purchaseLicenseTypeTestState) *sql.DB {
	t.Helper()
	registerPurchaseLicenseTypeTestDriver.Do(func() {
		sql.Register(purchaseLicenseTypeTestDriverName, purchaseLicenseTypeTestDriver{})
	})
	name := strings.ReplaceAll(t.Name(), "/", "-")
	purchaseLicenseTypeTestStates.Store(name, state)
	t.Cleanup(func() { purchaseLicenseTypeTestStates.Delete(name) })
	db, err := sql.Open(purchaseLicenseTypeTestDriverName, name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestManualAgentRechargeIsAtomic(t *testing.T) {
	t.Run("success commits balance and transaction", func(t *testing.T) {
		state := &purchaseLicenseTypeTestState{appExists: true}
		db := openPurchaseLicenseTypeTestDB(t, state)

		if err := rechargeAgentManually(db, 12, 2550, "测试充值", int64(3)); err != nil {
			t.Fatalf("manual recharge failed: %v", err)
		}

		state.mu.Lock()
		defer state.mu.Unlock()
		if state.commits != 1 || state.rollbacks != 0 {
			t.Fatalf("transaction state commits=%d rollbacks=%d", state.commits, state.rollbacks)
		}
		foundBalanceUpdate := false
		foundTransaction := false
		for index, query := range state.execQueries {
			if strings.Contains(query, "UPDATE agents SET balance = balance +") {
				foundBalanceUpdate = true
			}
			if strings.Contains(query, "INSERT INTO transactions") {
				args := state.execArgs[index]
				foundTransaction = len(args) > 5 &&
					args[1].Value == int64(12) &&
					args[2].Value == "25.50" &&
					args[4].Value == int64(3) &&
					strings.Contains(args[5].Value.(string), "后台管理员 3 手工充值")
			}
		}
		if !foundBalanceUpdate || !foundTransaction {
			t.Fatalf("missing atomic writes: balance=%v transaction=%v", foundBalanceUpdate, foundTransaction)
		}
	})

	t.Run("transaction insert failure rolls back balance", func(t *testing.T) {
		state := &purchaseLicenseTypeTestState{appExists: true, failTransactionInsert: true}
		db := openPurchaseLicenseTypeTestDB(t, state)

		if err := rechargeAgentManually(db, 12, 2550, "", int64(3)); err == nil {
			t.Fatal("manual recharge unexpectedly succeeded")
		}

		state.mu.Lock()
		defer state.mu.Unlock()
		if state.commits != 0 || state.rollbacks != 1 {
			t.Fatalf("transaction state commits=%d rollbacks=%d", state.commits, state.rollbacks)
		}
	})
}

func TestPurchaseLicenseTypeMasks(t *testing.T) {
	mask, err := purchaseLicenseTypeMaskForCreate(nil)
	if err != nil || mask != purchaseLicenseTypeAll {
		t.Fatalf("legacy create mask = %d, error = %v; want %d", mask, err, purchaseLicenseTypeAll)
	}
	if got := purchaseLicenseTypesFromMask(mask); strings.Join(got, ",") != "domain,wildcard,ip,key" {
		t.Fatalf("legacy type list = %v", got)
	}

	mask, err = parsePurchaseLicenseTypes([]string{"key", "domain", "key", " IP "})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(purchaseLicenseTypesFromMask(mask), ","); got != "domain,ip,key" {
		t.Fatalf("deduplicated type list = %q", got)
	}

	emptyMask, err := purchaseLicenseTypeMaskForCreate([]string{})
	if err != nil || emptyMask != 0 || len(purchaseLicenseTypesFromMask(emptyMask)) != 0 {
		t.Fatalf("empty selection mask = %d, error = %v", emptyMask, err)
	}

	if _, err := parsePurchaseLicenseTypes([]string{"domain", "device"}); err == nil {
		t.Fatal("invalid purchase license type was accepted")
	}
}

func TestPurchaseLicenseTypeConfigurationIsPerApp(t *testing.T) {
	appAMask, err := parsePurchaseLicenseTypes([]string{"domain", "key"})
	if err != nil {
		t.Fatal(err)
	}
	appBMask, err := parsePurchaseLicenseTypes([]string{"wildcard", "ip"})
	if err != nil {
		t.Fatal(err)
	}

	if !purchaseLicenseTypeAllowed(appAMask, "domain") || purchaseLicenseTypeAllowed(appAMask, "ip") {
		t.Fatalf("app A mask leaked types: %v", purchaseLicenseTypesFromMask(appAMask))
	}
	if !purchaseLicenseTypeAllowed(appBMask, "ip") || purchaseLicenseTypeAllowed(appBMask, "domain") {
		t.Fatalf("app B mask leaked types: %v", purchaseLicenseTypesFromMask(appBMask))
	}
	if appAMask == appBMask {
		t.Fatal("independent app configurations unexpectedly share the same mask")
	}
}

func TestValidatePlanLicenseTypeForApp(t *testing.T) {
	db := openPurchaseLicenseTypeTestDB(t, &purchaseLicenseTypeTestState{
		mask:      purchaseLicenseTypeDomain | purchaseLicenseTypeIP,
		appExists: true,
	})

	for _, licenseType := range []string{"", "domain", "ip"} {
		if err := validatePlanLicenseTypeForApp(db, 1, licenseType); err != nil {
			t.Fatalf("configured plan license type %q rejected: %v", licenseType, err)
		}
	}
	if err := validatePlanLicenseTypeForApp(db, 1, "key"); !errors.Is(err, errPurchaseTypeNotAllowed) {
		t.Fatalf("unconfigured plan license type error = %v", err)
	}
}

func TestRequireAppPurchaseLicenseType(t *testing.T) {
	t.Run("allowed", func(t *testing.T) {
		db := openPurchaseLicenseTypeTestDB(t, &purchaseLicenseTypeTestState{
			mask:      purchaseLicenseTypeDomain,
			appExists: true,
		})
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if err := requireAppPurchaseLicenseType(tx, 1, "domain"); err != nil {
			t.Fatalf("allowed type rejected: %v", err)
		}
	})

	t.Run("forged disabled type", func(t *testing.T) {
		db := openPurchaseLicenseTypeTestDB(t, &purchaseLicenseTypeTestState{
			mask:      purchaseLicenseTypeDomain,
			appExists: true,
		})
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if err := requireAppPurchaseLicenseType(tx, 1, "key"); !errors.Is(err, errPurchaseTypeNotAllowed) {
			t.Fatalf("forged disabled type error = %v", err)
		}
	})

	t.Run("disabled app", func(t *testing.T) {
		db := openPurchaseLicenseTypeTestDB(t, &purchaseLicenseTypeTestState{appExists: false})
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if err := requireAppPurchaseLicenseType(tx, 1, "domain"); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("disabled app error = %v", err)
		}
	})
}

func TestDisabledTypeDoesNotCreateOnlinePurchaseOrder(t *testing.T) {
	state := &purchaseLicenseTypeTestState{
		mask:      purchaseLicenseTypeDomain,
		appExists: true,
	}
	db := openPurchaseLicenseTypeTestDB(t, state)

	plan := purchasePlanPricing{AppID: 9, PlanID: 3, AppName: "Test App", PlanName: "Test Plan", DurationDays: 30, PriceCents: 2000}
	quote, err := calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceAgent,
		OriginalCents: plan.PriceCents,
		AgentDiscount: 5,
		Promotion: &purchasePromotionCandidate{
			ID:           41,
			Name:         "代理活动",
			Audience:     purchaseAudienceAgent,
			AmountCents:  800,
			RuleSnapshot: `{"type":"fixed_price","amount":"8.00","stacking":"lowest_only"}`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = insertAllowedLicensePurchaseOrder(
		db, "LP-test", 2, "agent", 2, "key", "", plan, quote, payChannelEpayV1, "alipay", "https://example.test/return",
	)
	if !errors.Is(err, errPurchaseTypeNotAllowed) {
		t.Fatalf("disabled online order error = %v", err)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.execCount != 0 {
		t.Fatalf("disabled online order executed %d write statements", state.execCount)
	}
}

func TestAllowedTypeCreatesOnlinePurchaseOrder(t *testing.T) {
	state := &purchaseLicenseTypeTestState{
		mask:      purchaseLicenseTypeKey,
		appExists: true,
	}
	db := openPurchaseLicenseTypeTestDB(t, state)

	plan := purchasePlanPricing{AppID: 9, PlanID: 3, AppName: "Test App", PlanName: "Test Plan", DurationDays: 30, PriceCents: 2000}
	quote, err := calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceAgent,
		OriginalCents: plan.PriceCents,
		AgentDiscount: 5,
		Promotion: &purchasePromotionCandidate{
			ID:           41,
			Name:         "代理活动",
			Audience:     purchaseAudienceAgent,
			AmountCents:  800,
			RuleSnapshot: `{"type":"fixed_price","amount":"8.00","stacking":"lowest_only"}`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = insertAllowedLicensePurchaseOrder(
		db, "LP-test", 2, "agent", 2, "key", "", plan, quote, payChannelEpayV1, "alipay", "https://example.test/return",
	)
	if err != nil {
		t.Fatalf("allowed online order rejected: %v", err)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.execCount != 1 {
		t.Fatalf("allowed online order executed %d write statements, want 1", state.execCount)
	}
	query := state.execQueries[0]
	args := state.execArgs[0]
	if !strings.Contains(query, "base_amount") || !strings.Contains(query, "promotion_rule_snapshot") || !strings.Contains(query, "duration_days_snapshot") {
		t.Fatalf("online order did not persist complete pricing snapshot: %s", query)
	}
	if strings.Count(query, "?") != len(args) {
		t.Fatalf("online order placeholders=%d args=%d", strings.Count(query, "?"), len(args))
	}
	if len(args) != 24 || args[9].Value != "8.00" || args[10].Value != "20.00" || args[11].Value != "10.00" || args[12].Value != "2.00" || args[13].Value != int64(41) || args[14].Value != "代理活动" || args[17].Value != "Test App" || args[18].Value != "Test Plan" || args[19].Value != int64(30) || args[20].Value != payChannelEpayV1 || args[21].Value != "alipay" {
		t.Fatalf("unexpected online order snapshot args: %#v", args)
	}
	var pricing purchasePricingSnapshot
	if err := json.Unmarshal([]byte(args[16].Value.(string)), &pricing); err != nil {
		t.Fatalf("decode online order pricing snapshot: %v", err)
	}
	if pricing.BaseAmount != "10.00" || pricing.Amount != "8.00" || pricing.PromotionID != 41 || pricing.PromotionAudience != purchaseAudienceAgent {
		t.Fatalf("unexpected online order pricing snapshot: %#v", pricing)
	}
}

func TestExistingOnlineOrderSettlesFromSnapshotAfterTypeDisabled(t *testing.T) {
	state := &purchaseLicenseTypeTestState{
		mask:      0,
		appExists: true,
		promotion: &purchasePromotionCandidate{
			ID:          99,
			Name:        "下单后变更的活动",
			Audience:    purchaseAudienceAll,
			AmountCents: 100,
		},
	}
	db := openPurchaseLicenseTypeTestDB(t, state)
	previousQueue := queuePurchaseSuccessMail
	queuePurchaseSuccessMail = func(string, int64, int64) {}
	t.Cleanup(func() { queuePurchaseSuccessMail = previousQueue })

	if err := settleLicensePurchaseOrder(db, "LP-existing", 1000, payChannelEpayV1, "trade-1", "alipay", "{}"); err != nil {
		t.Fatalf("settle existing order after type disabled: %v", err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if state.execCount < 4 {
		t.Fatalf("settlement executed %d statements, want schema plus license/order/transaction writes", state.execCount)
	}
	for _, query := range state.queries {
		if strings.Contains(query, "purchase_license_type_mask") {
			t.Fatalf("settlement unexpectedly rechecked current app mask: %s", query)
		}
		if strings.Contains(query, "FROM promotion_campaigns") {
			t.Fatalf("settlement unexpectedly recalculated the frozen promotion: %s", query)
		}
	}

	foundLicenseSnapshot := false
	foundAgentTransaction := false
	for index, query := range state.execQueries {
		args := state.execArgs[index]
		if strings.Contains(query, "INSERT INTO licenses") {
			foundLicenseSnapshot = len(args) > 3 && args[3].Value == "20.00"
		}
		if strings.Contains(query, "INSERT INTO transactions") {
			foundAgentTransaction = len(args) > 2 &&
				args[1].Value == "agent" &&
				args[2].Value == int64(7)
		}
	}
	if !foundLicenseSnapshot {
		t.Fatal("settlement did not persist the original plan price snapshot")
	}
	if !foundAgentTransaction {
		t.Fatal("settlement did not attribute consumption to the paying agent")
	}
}

func TestOnlinePurchaseSettlementRejectsPaymentMismatches(t *testing.T) {
	tests := []struct {
		name       string
		state      purchaseLicenseTypeTestState
		paidCents  int64
		payChannel string
		tradeNo    string
		payMethod  string
	}{
		{name: "amount", paidCents: 999, payChannel: payChannelEpayV1, tradeNo: "trade-1", payMethod: "alipay"},
		{name: "channel", state: purchaseLicenseTypeTestState{purchasePayChannel: payChannelEpayV2}, paidCents: 1000, payChannel: payChannelEpayV1, tradeNo: "trade-1", payMethod: "alipay"},
		{name: "method", state: purchaseLicenseTypeTestState{purchasePayMethod: "wxpay"}, paidCents: 1000, payChannel: payChannelEpayV1, tradeNo: "trade-1", payMethod: "alipay"},
		{name: "gateway trade", state: purchaseLicenseTypeTestState{purchaseTradeNo: "trade-original"}, paidCents: 1000, payChannel: payChannelEpayV1, tradeNo: "trade-forged", payMethod: "alipay"},
		{name: "cancelled order", state: purchaseLicenseTypeTestState{purchaseStatus: "cancelled"}, paidCents: 1000, payChannel: payChannelEpayV1, tradeNo: "trade-1", payMethod: "alipay"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.state
			state.appExists = true
			db := openPurchaseLicenseTypeTestDB(t, &state)
			if err := settleLicensePurchaseOrder(db, "LP-existing", tt.paidCents, tt.payChannel, tt.tradeNo, tt.payMethod, "{}"); err == nil {
				t.Fatal("mismatched purchase callback was accepted")
			}

			state.mu.Lock()
			defer state.mu.Unlock()
			for _, query := range state.execQueries {
				if strings.Contains(query, "INSERT INTO licenses") || strings.Contains(query, "SET status = 'paid'") {
					t.Fatalf("rejected callback mutated purchase state: %s", query)
				}
			}
		})
	}
}

func TestOnlinePurchaseSettlementIsIdempotent(t *testing.T) {
	state := &purchaseLicenseTypeTestState{
		appExists:          true,
		purchaseStatus:     "paid",
		purchasePayChannel: payChannelEpayV2,
		purchasePayMethod:  "wxpay",
		purchaseTradeNo:    "trade-2",
	}
	db := openPurchaseLicenseTypeTestDB(t, state)
	if err := settleLicensePurchaseOrder(db, "LP-existing", 1000, payChannelEpayV2, "trade-2", "wxpay", "{}"); err != nil {
		t.Fatalf("repeat purchase callback failed: %v", err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if state.commits != 1 {
		t.Fatalf("repeat callback commits=%d, want 1", state.commits)
	}
	for _, query := range state.execQueries {
		if strings.Contains(query, "INSERT INTO licenses") || strings.Contains(query, "SET status = 'paid'") {
			t.Fatalf("repeat callback duplicated purchase settlement: %s", query)
		}
	}
}

func TestBackfillPurchaseTransactionsUsesPayingAgent(t *testing.T) {
	state := &purchaseLicenseTypeTestState{appExists: true, backfillPaidOrders: true}
	db := openPurchaseLicenseTypeTestDB(t, state)

	BackfillLicensePurchaseTransactions(db)

	state.mu.Lock()
	defer state.mu.Unlock()
	foundRepair := false
	foundMissingTransaction := false
	for index, query := range state.execQueries {
		args := state.execArgs[index]
		if strings.Contains(query, "UPDATE transactions t") && strings.Contains(query, "SET t.subject_type = 'agent'") {
			foundRepair = true
		}
		if strings.Contains(query, "INSERT INTO transactions") {
			foundMissingTransaction = len(args) > 3 &&
				args[1].Value == "agent" &&
				args[2].Value == int64(7) &&
				args[3].Value == float64(-10)
		}
	}
	if !foundRepair {
		t.Fatal("backfill did not repair existing purchase transaction subjects")
	}
	if !foundMissingTransaction {
		t.Fatal("backfill did not attribute missing historical transaction to the paying agent")
	}
}

func TestPurchaseListsFilterAppsWithNoAllowedTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name    string
		role    string
		handler gin.HandlerFunc
	}{
		{name: "user", role: "user", handler: UserAppListForPurchase},
		{name: "agent", role: "agent", handler: AgentPanelPurchaseApps},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &purchaseLicenseTypeTestState{appExists: true}
			db := openPurchaseLicenseTypeTestDB(t, state)
			usePurchaseLicenseTypeTestHooks(t, db)

			router := gin.New()
			router.GET("/apps", func(c *gin.Context) {
				c.Set("role", tt.role)
				c.Set("user_id", uint(7))
				tt.handler(c)
			})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/apps", nil))

			var envelope struct {
				Code int   `json:"code"`
				Data []any `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
			}
			if envelope.Code != 200 || len(envelope.Data) != 0 {
				t.Fatalf("unexpected purchase app list: code=%d data=%v", envelope.Code, envelope.Data)
			}

			state.mu.Lock()
			defer state.mu.Unlock()
			foundFilter := false
			for _, query := range state.queries {
				if strings.Contains(query, "purchase_license_type_mask <> 0") {
					foundFilter = true
					break
				}
			}
			if !foundFilter {
				t.Fatal("purchase app list did not filter zero masks")
			}
		})
	}
}

func TestAdminLicenseCreationIgnoresPurchaseTypeMask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	state := &purchaseLicenseTypeTestState{
		mask:      0,
		appExists: true,
	}
	db := openPurchaseLicenseTypeTestDB(t, state)
	previousOpen := openAdminLicenseDB
	previousQueue := queueAdminLicenseOpenedMail
	openAdminLicenseDB = func() (*sql.DB, error) { return db, nil }
	queueAdminLicenseOpenedMail = func(int64) {}
	t.Cleanup(func() {
		openAdminLicenseDB = previousOpen
		queueAdminLicenseOpenedMail = previousQueue
	})

	router := gin.New()
	router.POST("/licenses", func(c *gin.Context) {
		c.Set("user_id", uint(1))
		LicenseCreate(c)
	})
	body := []byte(`{"appId":9,"planId":3,"type":"key","ownerType":"user","ownerId":7,"remark":"manual"}`)
	request := httptest.NewRequest(http.MethodPost, "/licenses", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	var envelope struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
	}
	if envelope.Code != 200 {
		t.Fatalf("admin manual license rejected: code=%d msg=%q", envelope.Code, envelope.Msg)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.execCount != 1 {
		t.Fatalf("admin manual license executed %d writes, want 1", state.execCount)
	}
	for _, query := range state.queries {
		if strings.Contains(query, "purchase_license_type_mask") {
			t.Fatalf("admin manual license unexpectedly checked purchase mask: %s", query)
		}
	}
}

func TestForgedDisabledTypeDoesNotMutateUserOrAgentPurchaseState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name      string
		role      string
		payMethod string
		handler   gin.HandlerFunc
	}{
		{name: "user balance", role: "user", payMethod: "balance", handler: UserPurchase},
		{name: "agent balance", role: "agent", payMethod: "balance", handler: AgentPanelPurchase},
		{name: "agent quota", role: "agent", payMethod: "quota", handler: AgentPanelPurchase},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &purchaseLicenseTypeTestState{
				mask:      purchaseLicenseTypeDomain,
				appExists: true,
			}
			db := openPurchaseLicenseTypeTestDB(t, state)
			usePurchaseLicenseTypeTestHooks(t, db)

			router := gin.New()
			router.POST("/purchase", func(c *gin.Context) {
				c.Set("role", tt.role)
				c.Set("user_id", uint(7))
				tt.handler(c)
			})
			body, err := json.Marshal(map[string]any{
				"appId": 9, "planId": 3, "type": "key", "payMethod": tt.payMethod,
			})
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/purchase", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			var envelope struct {
				Code int    `json:"code"`
				Msg  string `json:"msg"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
			}
			if envelope.Code != 400 || !strings.Contains(envelope.Msg, "不支持购买密钥授权") {
				t.Fatalf("unexpected response: code=%d msg=%q", envelope.Code, envelope.Msg)
			}
			state.mu.Lock()
			defer state.mu.Unlock()
			if state.execCount != 0 {
				t.Fatalf("forged purchase executed %d write statements", state.execCount)
			}
		})
	}
}

func TestBalancePurchasesPersistPricingSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name               string
		role               string
		handler            gin.HandlerFunc
		amountIndex        int
		pricingIndex       int
		appNameIndex       int
		planNameIndex      int
		durationIndex      int
		expectedBuyer      purchaseAudience
		expectedBase       string
		expectedDiscount   string
		expectedDebitQuery string
	}{
		{
			name:               "user",
			role:               "user",
			handler:            UserPurchase,
			amountIndex:        7,
			pricingIndex:       14,
			appNameIndex:       15,
			planNameIndex:      16,
			durationIndex:      17,
			expectedBuyer:      purchaseAudienceUser,
			expectedBase:       "10.00",
			expectedDiscount:   "3.00",
			expectedDebitQuery: "UPDATE users SET balance",
		},
		{
			name:               "agent",
			role:               "agent",
			handler:            AgentPanelPurchase,
			amountIndex:        9,
			pricingIndex:       16,
			appNameIndex:       17,
			planNameIndex:      18,
			durationIndex:      19,
			expectedBuyer:      purchaseAudienceAgent,
			expectedBase:       "8.00",
			expectedDiscount:   "1.00",
			expectedDebitQuery: "UPDATE agents SET balance",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := &purchaseLicenseTypeTestState{
				mask:          purchaseLicenseTypeKey,
				appExists:     true,
				agentDiscount: 8,
				promotion: &purchasePromotionCandidate{
					ID:          31,
					Name:        "限时活动",
					Audience:    test.expectedBuyer,
					AmountCents: 700,
				},
			}
			db := openPurchaseLicenseTypeTestDB(t, state)
			usePurchaseLicenseTypeTestHooks(t, db)

			router := gin.New()
			router.POST("/purchase", func(c *gin.Context) {
				c.Set("role", test.role)
				c.Set("user_id", uint(7))
				test.handler(c)
			})
			request := httptest.NewRequest(http.MethodPost, "/purchase", bytes.NewBufferString(`{"appId":9,"planId":3,"type":"key","payMethod":"balance"}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			var envelope struct {
				Code int    `json:"code"`
				Msg  string `json:"msg"`
				Data struct {
					Cost       float64 `json:"cost"`
					NewBalance float64 `json:"newBalance"`
				} `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
			}
			if envelope.Code != 200 {
				t.Fatalf("balance purchase failed: code=%d msg=%q", envelope.Code, envelope.Msg)
			}
			if envelope.Data.Cost != 7 || envelope.Data.NewBalance != 93 {
				t.Fatalf("unexpected balance result: cost=%v newBalance=%v", envelope.Data.Cost, envelope.Data.NewBalance)
			}

			state.mu.Lock()
			defer state.mu.Unlock()
			foundDebit := false
			foundOrder := false
			for index, query := range state.execQueries {
				if strings.Contains(query, test.expectedDebitQuery) {
					foundDebit = true
				}
				if !strings.Contains(query, "INSERT INTO license_purchase_orders") {
					continue
				}
				foundOrder = true
				args := state.execArgs[index]
				if strings.Count(query, "?") != len(args) {
					t.Fatalf("balance order placeholders=%d args=%d", strings.Count(query, "?"), len(args))
				}
				if !strings.Contains(query, "base_amount") || !strings.Contains(query, "pricing_snapshot") || !strings.Contains(query, "duration_days_snapshot") {
					t.Fatalf("balance order missing pricing snapshots: %s", query)
				}
				if args[test.amountIndex].Value != "7.00" || args[test.amountIndex+1].Value != "10.00" || args[test.amountIndex+2].Value != test.expectedBase || args[test.amountIndex+3].Value != test.expectedDiscount {
					t.Fatalf("unexpected monetary snapshots: %#v", args[test.amountIndex:test.amountIndex+4])
				}
				if args[test.amountIndex+4].Value != int64(31) || args[test.amountIndex+5].Value != "限时活动" {
					t.Fatalf("unexpected promotion snapshot: %#v", args[test.amountIndex+4:test.amountIndex+6])
				}
				if args[test.appNameIndex].Value != "Test App" || args[test.planNameIndex].Value != "Test Plan" || args[test.durationIndex].Value != int64(30) {
					t.Fatalf("unexpected plan snapshots: app=%#v plan=%#v duration=%#v", args[test.appNameIndex].Value, args[test.planNameIndex].Value, args[test.durationIndex].Value)
				}
				pricingJSON, ok := args[test.pricingIndex].Value.(string)
				if !ok {
					t.Fatalf("pricing snapshot type = %T", args[test.pricingIndex].Value)
				}
				var pricing purchasePricingSnapshot
				if err := json.Unmarshal([]byte(pricingJSON), &pricing); err != nil {
					t.Fatalf("decode pricing snapshot: %v", err)
				}
				if pricing.Version != 1 || pricing.BuyerType != test.expectedBuyer || pricing.OriginalAmount != "10.00" || pricing.BaseAmount != test.expectedBase || pricing.Amount != "7.00" || pricing.DiscountAmount != test.expectedDiscount || pricing.PromotionID != 31 || pricing.PromotionName != "限时活动" || pricing.PromotionAudience != test.expectedBuyer {
					t.Fatalf("unexpected pricing snapshot: %#v", pricing)
				}
				var rule struct {
					Type     string `json:"type"`
					Amount   string `json:"amount"`
					Stacking string `json:"stacking"`
				}
				if err := json.Unmarshal(pricing.PromotionRule, &rule); err != nil || rule.Type != "fixed_price" || rule.Amount != "7.00" || rule.Stacking != "lowest_only" {
					t.Fatalf("unexpected promotion rule: %#v, error=%v", rule, err)
				}
			}
			if !foundDebit {
				t.Fatal("balance purchase did not deduct buyer balance")
			}
			if !foundOrder {
				t.Fatal("balance purchase did not write an order")
			}
			if state.commits != 1 || state.rollbacks != 0 {
				t.Fatalf("unexpected transaction result: commits=%d rollbacks=%d", state.commits, state.rollbacks)
			}
		})
	}
}

func TestAgentPanelUserOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	state := &purchaseLicenseTypeTestState{appExists: true}
	db := openPurchaseLicenseTypeTestDB(t, state)
	usePurchaseLicenseTypeTestHooks(t, db)

	router := gin.New()
	router.GET("/users/options", func(c *gin.Context) {
		c.Set("role", "agent")
		c.Set("user_id", uint(7))
		AgentPanelUserOptions(c)
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/users/options?keyword=alice", nil))

	var envelope struct {
		Code int `json:"code"`
		Data []struct {
			ID    int64  `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
	}
	if envelope.Code != 200 {
		t.Fatalf("unexpected response code: %d", envelope.Code)
	}
	if len(envelope.Data) != 2 || envelope.Data[0].ID != 42 || envelope.Data[0].Email != "alice@example.com" {
		t.Fatalf("unexpected users: %#v", envelope.Data)
	}
}

func usePurchaseLicenseTypeTestHooks(t *testing.T, db *sql.DB) {
	t.Helper()
	previousOpen := openAppPurchaseDB
	previousEnsureTypes := ensureAppPurchaseLicenseTypes
	previousEnsureAgent := ensureAgentPurchaseSchemas
	previousEnsurePricing := ensurePurchaseOrderPricingSchema
	previousEnsurePromotion := ensurePurchasePromotionSchema
	previousSelfPurchase := selfPurchaseEnabledForPurchase
	previousQueue := queuePurchaseSuccessMail
	openAppPurchaseDB = func() (*sql.DB, error) { return db, nil }
	ensureAppPurchaseLicenseTypes = func(*sql.DB) error { return nil }
	ensureAgentPurchaseSchemas = func(*sql.DB) error { return nil }
	ensurePurchaseOrderPricingSchema = func(*sql.DB) error { return nil }
	ensurePurchasePromotionSchema = func(*sql.DB) error { return nil }
	selfPurchaseEnabledForPurchase = func() bool { return true }
	queuePurchaseSuccessMail = func(string, int64, int64) {}
	t.Cleanup(func() {
		openAppPurchaseDB = previousOpen
		ensureAppPurchaseLicenseTypes = previousEnsureTypes
		ensureAgentPurchaseSchemas = previousEnsureAgent
		ensurePurchaseOrderPricingSchema = previousEnsurePricing
		ensurePurchasePromotionSchema = previousEnsurePromotion
		selfPurchaseEnabledForPurchase = previousSelfPurchase
		queuePurchaseSuccessMail = previousQueue
	})
}
