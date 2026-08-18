package handler

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

const promotionCampaignTestDriverName = "promotion-campaign-test"

var registerPromotionCampaignTestDriver sync.Once
var promotionCampaignTestStates sync.Map

type promotionCampaignTestState struct {
	mu          sync.Mutex
	conflict    bool
	appExists   bool
	campaignID  int64
	appID       int64
	startsAt    time.Time
	endsAt      time.Time
	execErrorOn string
	queries     []string
	queryArgs   [][]driver.NamedValue
	execs       []string
	execArgs    [][]driver.NamedValue
	commits     int
	rollbacks   int
}

type promotionCampaignTestDriver struct{}
type promotionCampaignTestConn struct{ state *promotionCampaignTestState }
type promotionCampaignTestTx struct{ state *promotionCampaignTestState }
type promotionCampaignTestResult struct{ id, affected int64 }
type promotionCampaignTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (promotionCampaignTestDriver) Open(name string) (driver.Conn, error) {
	value, ok := promotionCampaignTestStates.Load(name)
	if !ok {
		return nil, errors.New("promotion campaign test state not found")
	}
	return &promotionCampaignTestConn{state: value.(*promotionCampaignTestState)}, nil
}

func (*promotionCampaignTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}
func (*promotionCampaignTestConn) Close() error { return nil }
func (conn *promotionCampaignTestConn) Begin() (driver.Tx, error) {
	return &promotionCampaignTestTx{state: conn.state}, nil
}

func (conn *promotionCampaignTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	conn.state.mu.Lock()
	defer conn.state.mu.Unlock()
	conn.state.queries = append(conn.state.queries, query)
	conn.state.queryArgs = append(conn.state.queryArgs, append([]driver.NamedValue(nil), args...))

	switch {
	case strings.Contains(query, "SELECT id FROM apps"):
		if !conn.state.appExists {
			return &promotionCampaignTestRows{columns: []string{"id"}}, nil
		}
		return &promotionCampaignTestRows{columns: []string{"id"}, values: [][]driver.Value{{conn.state.appID}}}, nil
	case strings.Contains(query, "SELECT price") && strings.Contains(query, "FROM license_plans"):
		return &promotionCampaignTestRows{columns: []string{"price"}, values: [][]driver.Value{{float64(100)}}}, nil
	case strings.Contains(query, "SELECT pc.id, pc.app_id") && strings.Contains(query, "FROM promotion_campaigns pc"):
		if conn.state.campaignID == 0 {
			return &promotionCampaignTestRows{columns: []string{"id", "app_id", "app_name", "name", "audience", "starts_at", "ends_at", "enabled", "created_at", "updated_at"}}, nil
		}
		return &promotionCampaignTestRows{
			columns: []string{"id", "app_id", "app_name", "name", "audience", "starts_at", "ends_at", "enabled", "created_at", "updated_at"},
			values: [][]driver.Value{{
				conn.state.campaignID, conn.state.appID, "Test App", "春季活动", "all",
				conn.state.startsAt, conn.state.endsAt, true, conn.state.startsAt, conn.state.startsAt,
			}},
		}, nil
	case strings.Contains(query, "FROM promotion_campaign_plans pcp") && strings.Contains(query, "JOIN license_plans p"):
		return &promotionCampaignTestRows{
			columns: []string{"campaign_id", "id", "name", "price", "rule_type", "rule_value"},
			values:  [][]driver.Value{{conn.state.campaignID, int64(3), "年度套餐", float64(100), "discount", float64(8.5)}},
		}, nil
	case strings.Contains(query, "SELECT id, name") && strings.Contains(query, "FROM promotion_campaigns"):
		if !conn.state.conflict {
			return &promotionCampaignTestRows{columns: []string{"id", "name"}}, nil
		}
		return &promotionCampaignTestRows{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(99), "重叠活动"}}}, nil
	case strings.Contains(query, "SELECT app_id, starts_at, ends_at"):
		return &promotionCampaignTestRows{
			columns: []string{"app_id", "starts_at", "ends_at"},
			values:  [][]driver.Value{{conn.state.appID, conn.state.startsAt, conn.state.endsAt}},
		}, nil
	case strings.Contains(query, "SELECT app_id") && strings.Contains(query, "FROM promotion_campaigns"):
		if conn.state.campaignID == 0 {
			return &promotionCampaignTestRows{columns: []string{"app_id"}}, nil
		}
		return &promotionCampaignTestRows{columns: []string{"app_id"}, values: [][]driver.Value{{conn.state.appID}}}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

func (conn *promotionCampaignTestConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	conn.state.mu.Lock()
	defer conn.state.mu.Unlock()
	conn.state.execs = append(conn.state.execs, query)
	conn.state.execArgs = append(conn.state.execArgs, append([]driver.NamedValue(nil), args...))
	if conn.state.execErrorOn != "" && strings.Contains(query, conn.state.execErrorOn) {
		return nil, errors.New("forced exec failure")
	}
	return promotionCampaignTestResult{id: conn.state.campaignID, affected: 1}, nil
}

func (tx *promotionCampaignTestTx) Commit() error {
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()
	tx.state.commits++
	return nil
}
func (tx *promotionCampaignTestTx) Rollback() error {
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()
	tx.state.rollbacks++
	return nil
}
func (result promotionCampaignTestResult) LastInsertId() (int64, error) { return result.id, nil }
func (result promotionCampaignTestResult) RowsAffected() (int64, error) { return result.affected, nil }
func (rows *promotionCampaignTestRows) Columns() []string               { return rows.columns }
func (*promotionCampaignTestRows) Close() error                         { return nil }
func (rows *promotionCampaignTestRows) Next(dest []driver.Value) error {
	if rows.index >= len(rows.values) {
		return io.EOF
	}
	copy(dest, rows.values[rows.index])
	rows.index++
	return nil
}

func openPromotionCampaignTestDB(t *testing.T, state *promotionCampaignTestState) *sql.DB {
	t.Helper()
	registerPromotionCampaignTestDriver.Do(func() {
		sql.Register(promotionCampaignTestDriverName, promotionCampaignTestDriver{})
	})
	name := strings.ReplaceAll(t.Name(), "/", "-")
	promotionCampaignTestStates.Store(name, state)
	t.Cleanup(func() { promotionCampaignTestStates.Delete(name) })
	db, err := sql.Open(promotionCampaignTestDriverName, name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func promotionFloat(value float64) *float64 {
	return &value
}

func TestPromotionCampaignStatus(t *testing.T) {
	now := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.Local)
	tests := []struct {
		name      string
		enabled   bool
		startsAt  time.Time
		endsAt    time.Time
		wantState string
	}{
		{name: "disabled", enabled: false, startsAt: now.Add(-time.Hour), endsAt: now.Add(time.Hour), wantState: "disabled"},
		{name: "upcoming", enabled: true, startsAt: now.Add(time.Hour), endsAt: now.Add(2 * time.Hour), wantState: "upcoming"},
		{name: "active", enabled: true, startsAt: now.Add(-time.Hour), endsAt: now.Add(time.Hour), wantState: "active"},
		{name: "ended at exclusive boundary", enabled: true, startsAt: now.Add(-time.Hour), endsAt: now, wantState: "ended"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := promotionCampaignStatus(test.enabled, test.startsAt, test.endsAt, now); got != test.wantState {
				t.Fatalf("status = %q; want %q", got, test.wantState)
			}
		})
	}
}

func TestListPromotionCampaignsIncludesSelectedPlanRules(t *testing.T) {
	now := time.Now()
	state := &promotionCampaignTestState{
		campaignID: 42,
		appID:      9,
		startsAt:   now.Add(-time.Hour),
		endsAt:     now.Add(time.Hour),
	}
	db := openPromotionCampaignTestDB(t, state)
	items, err := listPromotionCampaigns(context.Background(), db, promotionCampaignListFilter{
		AppID:    9,
		Keyword:  "春季",
		Audience: purchaseAudienceAll,
		Status:   "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("campaign count = %d", len(items))
	}
	item := items[0]
	if item.ID != 42 || item.AppID != 9 || item.AppName != "Test App" || item.Status != "active" || item.Audience != purchaseAudienceAll {
		t.Fatalf("unexpected campaign: %#v", item)
	}
	if len(item.Plans) != 1 || item.Plans[0].PlanID != 3 || item.Plans[0].RuleType != promotionRuleDiscount || item.Plans[0].Value != 8.5 || item.Plans[0].OriginalPrice != 100 {
		t.Fatalf("unexpected campaign plans: %#v", item.Plans)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.queries) != 2 || !strings.Contains(state.queries[0], "pc.enabled = 1 AND pc.starts_at <= NOW() AND pc.ends_at > NOW()") || !strings.Contains(state.queries[1], "pcp.campaign_id IN (?)") {
		t.Fatalf("unexpected list queries: %#v", state.queries)
	}
	args := state.queryArgs[0]
	if len(args) != 4 || args[0].Value != int64(9) || args[1].Value != "%春季%" || args[2].Value != "%春季%" || args[3].Value != "all" {
		t.Fatalf("unexpected list args: %#v", args)
	}
}

func TestPromotionTimeRangesOverlap(t *testing.T) {
	base := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.Local)
	tests := []struct {
		name                   string
		firstStart, firstEnd   time.Time
		secondStart, secondEnd time.Time
		want                   bool
	}{
		{
			name:        "部分重叠",
			firstStart:  base,
			firstEnd:    base.Add(2 * time.Hour),
			secondStart: base.Add(time.Hour),
			secondEnd:   base.Add(3 * time.Hour),
			want:        true,
		},
		{
			name:        "完全包含",
			firstStart:  base,
			firstEnd:    base.Add(4 * time.Hour),
			secondStart: base.Add(time.Hour),
			secondEnd:   base.Add(2 * time.Hour),
			want:        true,
		},
		{
			name:        "首尾相接",
			firstStart:  base,
			firstEnd:    base.Add(time.Hour),
			secondStart: base.Add(time.Hour),
			secondEnd:   base.Add(2 * time.Hour),
			want:        false,
		},
		{
			name:        "完全分离",
			firstStart:  base,
			firstEnd:    base.Add(time.Hour),
			secondStart: base.Add(2 * time.Hour),
			secondEnd:   base.Add(3 * time.Hour),
			want:        false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := promotionTimeRangesOverlap(test.firstStart, test.firstEnd, test.secondStart, test.secondEnd); got != test.want {
				t.Fatalf("overlap = %v; want %v", got, test.want)
			}
		})
	}
}

func TestParsePromotionCampaignRequest(t *testing.T) {
	request := promotionCampaignRequest{
		AppID:    9,
		Name:     " 春季活动 ",
		Audience: "all",
		StartsAt: "2026-03-01T10:00:00+08:00",
		EndsAt:   "2026-03-01T12:00:00+08:00",
		Enabled:  true,
		Plans: []promotionCampaignPlanRequest{
			{PlanID: 3, Price: promotionFloat(60)},
		},
	}
	campaign, err := parsePromotionCampaignRequest(request, 7)
	if err != nil {
		t.Fatal(err)
	}
	if campaign.Name != "春季活动" || campaign.Audience != purchaseAudienceAll || campaign.CreatedBy != 7 || len(campaign.Plans) != 1 || campaign.Plans[0].RuleType != promotionRuleFixedPrice || campaign.Plans[0].ValueUnits != 6000 {
		t.Fatalf("unexpected campaign: %#v", campaign)
	}

	request.Audience = "admin"
	if _, err := parsePromotionCampaignRequest(request, 7); !errors.Is(err, errInvalidPurchaseAudience) {
		t.Fatalf("invalid audience error = %v", err)
	}
	request.Audience = "user"
	request.EndsAt = request.StartsAt
	if _, err := parsePromotionCampaignRequest(request, 7); err == nil {
		t.Fatal("equal start and end should be rejected")
	}
}

func TestParsePromotionCampaignRuleTypes(t *testing.T) {
	request := promotionCampaignRequest{
		AppID:    9,
		Name:     "多规则活动",
		Audience: "all",
		StartsAt: "2026-03-01T10:00:00+08:00",
		EndsAt:   "2026-03-01T12:00:00+08:00",
		Plans: []promotionCampaignPlanRequest{
			{PlanID: 3, RuleType: "discount", Value: promotionFloat(8.5)},
			{PlanID: 4, RuleType: "reduction", Value: promotionFloat(20)},
			{PlanID: 5, RuleType: "fixed_price", Value: promotionFloat(60)},
		},
	}
	campaign, err := parsePromotionCampaignRequest(request, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(campaign.Plans) != 3 ||
		campaign.Plans[0].RuleType != promotionRuleDiscount || campaign.Plans[0].ValueUnits != 8500 ||
		campaign.Plans[1].RuleType != promotionRuleReduction || campaign.Plans[1].ValueUnits != 2000 ||
		campaign.Plans[2].RuleType != promotionRuleFixedPrice || campaign.Plans[2].ValueUnits != 6000 {
		t.Fatalf("unexpected parsed rules: %#v", campaign.Plans)
	}

	invalid := []promotionCampaignPlanRequest{
		{PlanID: 3, RuleType: "discount", Value: promotionFloat(0)},
		{PlanID: 3, RuleType: "discount", Value: promotionFloat(10.1)},
		{PlanID: 3, RuleType: "reduction", Value: promotionFloat(0)},
		{PlanID: 3, RuleType: "fixed_price", Value: promotionFloat(-1)},
		{PlanID: 3, RuleType: "unknown", Value: promotionFloat(1)},
	}
	for _, plan := range invalid {
		request.Plans = []promotionCampaignPlanRequest{plan}
		if _, err := parsePromotionCampaignRequest(request, 7); err == nil {
			t.Fatalf("invalid promotion rule was accepted: %#v", plan)
		}
	}
}

func TestPromotionCampaignPersistsOnlySelectedPlanRules(t *testing.T) {
	state := &promotionCampaignTestState{}
	db := openPromotionCampaignTestDB(t, state)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	plans := []promotionCampaignPlanWrite{
		{PlanID: 3, RuleType: promotionRuleDiscount, ValueUnits: 8500},
		{PlanID: 5, RuleType: promotionRuleReduction, ValueUnits: 2000},
		{PlanID: 8, RuleType: promotionRuleFixedPrice, ValueUnits: 6000},
	}
	if err := replacePromotionCampaignPlans(context.Background(), tx, 42, plans); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.execs) != 4 || !strings.Contains(state.execs[0], "DELETE FROM promotion_campaign_plans") {
		t.Fatalf("unexpected selected-plan writes: %#v", state.execs)
	}
	wantArgs := [][]any{
		{int64(42), int64(3), "discount", "8.5", "0.00"},
		{int64(42), int64(5), "reduction", "20.00", "0.00"},
		{int64(42), int64(8), "fixed_price", "60.00", "60.00"},
	}
	for index, want := range wantArgs {
		args := state.execArgs[index+1]
		if len(args) != len(want) {
			t.Fatalf("rule %d args = %#v", index, args)
		}
		for argIndex, value := range want {
			if args[argIndex].Value != value {
				t.Fatalf("rule %d arg %d = %#v; want %#v", index, argIndex, args[argIndex].Value, value)
			}
		}
	}
	if state.commits != 1 || state.rollbacks != 0 {
		t.Fatalf("unexpected transaction result: commits=%d rollbacks=%d", state.commits, state.rollbacks)
	}
}

func TestValidatePromotionCampaignPlanRuleBounds(t *testing.T) {
	tests := []struct {
		name      string
		ruleType  promotionRuleType
		value     int64
		wantError string
	}{
		{name: "valid discount", ruleType: promotionRuleDiscount, value: 8500},
		{name: "invalid discount", ruleType: promotionRuleDiscount, value: 10001, wantError: "活动折扣"},
		{name: "valid full reduction", ruleType: promotionRuleReduction, value: 10000},
		{name: "reduction above original price", ruleType: promotionRuleReduction, value: 10001, wantError: "立减金额不能超过套餐原价"},
		{name: "valid zero fixed price", ruleType: promotionRuleFixedPrice, value: 0},
		{name: "fixed price above original price", ruleType: promotionRuleFixedPrice, value: 10001, wantError: "固定活动价不能高于套餐原价"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := &promotionCampaignTestState{}
			db := openPromotionCampaignTestDB(t, state)
			tx, err := db.BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			campaign := promotionCampaignWrite{
				AppID: 9,
				Plans: []promotionCampaignPlanWrite{
					{PlanID: 3, RuleType: test.ruleType, ValueUnits: test.value},
				},
			}
			err = validatePromotionCampaignPlans(context.Background(), tx, campaign)
			if test.wantError == "" && err != nil {
				t.Fatal(err)
			}
			if test.wantError != "" && (err == nil || !strings.Contains(err.Error(), test.wantError)) {
				t.Fatalf("validation error = %v; want %q", err, test.wantError)
			}
		})
	}
}

func TestCreatePromotionCampaignRejectsOverlapAfterLockingApp(t *testing.T) {
	state := &promotionCampaignTestState{
		conflict:   true,
		appExists:  true,
		campaignID: 42,
		appID:      9,
	}
	db := openPromotionCampaignTestDB(t, state)
	campaign := promotionCampaignWrite{
		AppID:    9,
		Name:     "活动 B",
		Audience: purchaseAudienceUser,
		StartsAt: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.Local),
		EndsAt:   time.Date(2026, time.March, 1, 12, 0, 0, 0, time.Local),
		Enabled:  true,
		Plans: []promotionCampaignPlanWrite{
			{PlanID: 3, RuleType: promotionRuleFixedPrice, ValueUnits: 6000},
		},
	}

	_, err := createPromotionCampaign(context.Background(), db, campaign)
	if !errors.Is(err, errPromotionCampaignConflict) {
		t.Fatalf("create error = %v", err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.queries) != 3 {
		t.Fatalf("queries = %d; want app lock, plan validation and conflict check", len(state.queries))
	}
	if !strings.Contains(state.queries[0], "FROM apps") || !strings.Contains(state.queries[0], "FOR UPDATE") {
		t.Fatalf("first query did not lock app: %s", state.queries[0])
	}
	if !strings.Contains(state.queries[1], "FROM license_plans") || !strings.Contains(state.queries[1], "FOR UPDATE") {
		t.Fatalf("second query did not validate plan: %s", state.queries[1])
	}
	if !strings.Contains(state.queries[2], "starts_at < ? AND ends_at > ?") || !strings.Contains(state.queries[2], "FOR UPDATE") {
		t.Fatalf("invalid overlap query: %s", state.queries[2])
	}
	if strings.Contains(state.queries[2], "audience") {
		t.Fatalf("audience must not bypass app-level exclusivity: %s", state.queries[2])
	}
	if len(state.execs) != 0 || state.commits != 0 || state.rollbacks != 1 {
		t.Fatalf("unexpected transaction result: execs=%d commits=%d rollbacks=%d", len(state.execs), state.commits, state.rollbacks)
	}
}

func TestCreatePromotionCampaignAllowsAdjacentRange(t *testing.T) {
	state := &promotionCampaignTestState{
		appExists:  true,
		campaignID: 42,
		appID:      9,
	}
	db := openPromotionCampaignTestDB(t, state)
	campaign := promotionCampaignWrite{
		AppID:    9,
		Name:     "相邻活动",
		Audience: purchaseAudienceAgent,
		StartsAt: time.Date(2026, time.March, 1, 12, 0, 0, 0, time.Local),
		EndsAt:   time.Date(2026, time.March, 1, 14, 0, 0, 0, time.Local),
		Enabled:  true,
		Plans: []promotionCampaignPlanWrite{
			{PlanID: 3, RuleType: promotionRuleFixedPrice, ValueUnits: 6000},
		},
	}

	id, err := createPromotionCampaign(context.Background(), db, campaign)
	if err != nil {
		t.Fatal(err)
	}
	if id != 42 {
		t.Fatalf("campaign id = %d", id)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.execs) != 3 || !strings.Contains(state.execs[0], "INSERT INTO promotion_campaigns") || !strings.Contains(state.execs[1], "DELETE FROM promotion_campaign_plans") || !strings.Contains(state.execs[2], "INSERT INTO promotion_campaign_plans") {
		t.Fatalf("unexpected writes: %#v", state.execs)
	}
	if len(state.execArgs[2]) != 5 || state.execArgs[2][0].Value != int64(42) || state.execArgs[2][1].Value != int64(3) || state.execArgs[2][2].Value != "fixed_price" || state.execArgs[2][3].Value != "60.00" || state.execArgs[2][4].Value != "60.00" {
		t.Fatalf("unexpected campaign plan rule: %#v", state.execArgs[2])
	}
	if state.commits != 1 || state.rollbacks != 0 {
		t.Fatalf("unexpected transaction result: commits=%d rollbacks=%d", state.commits, state.rollbacks)
	}
}

func TestUpdatePromotionCampaignPersistsSelectedPlanRules(t *testing.T) {
	state := &promotionCampaignTestState{
		appExists:  true,
		campaignID: 42,
		appID:      9,
	}
	db := openPromotionCampaignTestDB(t, state)
	startsAt := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.Local)
	campaign := promotionCampaignWrite{
		AppID:    9,
		Name:     "修改后的多规则活动",
		Audience: purchaseAudienceAll,
		StartsAt: startsAt,
		EndsAt:   startsAt.Add(2 * time.Hour),
		Enabled:  false,
		Plans: []promotionCampaignPlanWrite{
			{PlanID: 3, RuleType: promotionRuleDiscount, ValueUnits: 8500},
			{PlanID: 5, RuleType: promotionRuleReduction, ValueUnits: 2000},
			{PlanID: 8, RuleType: promotionRuleFixedPrice, ValueUnits: 6000},
		},
	}

	if err := updatePromotionCampaign(context.Background(), db, 42, campaign); err != nil {
		t.Fatal(err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.execs) != 5 || !strings.Contains(state.execs[0], "UPDATE promotion_campaigns") || !strings.Contains(state.execs[1], "DELETE FROM promotion_campaign_plans") {
		t.Fatalf("unexpected update writes: %#v", state.execs)
	}
	wantRules := []string{"discount", "reduction", "fixed_price"}
	wantPlanIDs := []int64{3, 5, 8}
	for index, wantRule := range wantRules {
		args := state.execArgs[index+2]
		if len(args) != 5 || args[0].Value != int64(42) || args[1].Value != wantPlanIDs[index] || args[2].Value != wantRule {
			t.Fatalf("unexpected updated plan rule %d: %#v", index, args)
		}
	}
	if state.commits != 1 || state.rollbacks != 0 {
		t.Fatalf("unexpected transaction result: commits=%d rollbacks=%d", state.commits, state.rollbacks)
	}
}

func TestUpdatePromotionCampaignRejectsOverlap(t *testing.T) {
	state := &promotionCampaignTestState{
		conflict:   true,
		appExists:  true,
		campaignID: 42,
		appID:      9,
	}
	db := openPromotionCampaignTestDB(t, state)
	startsAt := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.Local)
	campaign := promotionCampaignWrite{
		AppID:    9,
		Name:     "修改后的活动",
		Audience: purchaseAudienceAll,
		StartsAt: startsAt,
		EndsAt:   startsAt.Add(2 * time.Hour),
		Enabled:  true,
		Plans: []promotionCampaignPlanWrite{
			{PlanID: 3, RuleType: promotionRuleFixedPrice, ValueUnits: 6000},
		},
	}

	err := updatePromotionCampaign(context.Background(), db, 42, campaign)
	if !errors.Is(err, errPromotionCampaignConflict) {
		t.Fatalf("update error = %v", err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.execs) != 0 || state.commits != 0 || state.rollbacks != 1 {
		t.Fatalf("conflicting campaign was updated: execs=%d commits=%d rollbacks=%d", len(state.execs), state.commits, state.rollbacks)
	}
	if len(state.queries) != 4 || !strings.Contains(state.queries[0], "FROM apps") || !strings.Contains(state.queries[0], "FOR UPDATE") {
		t.Fatalf("update did not lock app first: %#v", state.queries)
	}
	conflictArgs := state.queryArgs[3]
	if len(conflictArgs) != 4 || conflictArgs[1].Value != int64(42) {
		t.Fatalf("update conflict query did not exclude itself: %#v", conflictArgs)
	}
}

func TestEnablePromotionCampaignRechecksOverlap(t *testing.T) {
	startsAt := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.Local)
	state := &promotionCampaignTestState{
		conflict:   true,
		appExists:  true,
		campaignID: 42,
		appID:      9,
		startsAt:   startsAt,
		endsAt:     startsAt.Add(2 * time.Hour),
	}
	db := openPromotionCampaignTestDB(t, state)

	err := setPromotionCampaignEnabled(context.Background(), db, 42, true)
	if !errors.Is(err, errPromotionCampaignConflict) {
		t.Fatalf("toggle error = %v", err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.execs) != 0 || state.commits != 0 || state.rollbacks != 1 {
		t.Fatalf("conflicting campaign was enabled: execs=%d commits=%d rollbacks=%d", len(state.execs), state.commits, state.rollbacks)
	}
	if len(state.queries) != 4 || !strings.Contains(state.queries[1], "FROM apps") || !strings.Contains(state.queries[1], "FOR UPDATE") {
		t.Fatalf("toggle did not lock app before checking conflict: %#v", state.queries)
	}
	conflictArgs := state.queryArgs[3]
	if len(conflictArgs) != 4 || conflictArgs[1].Value != int64(42) {
		t.Fatalf("toggle conflict query did not exclude itself: %#v", conflictArgs)
	}
}

func TestDeletePromotionCampaignLocksAppAndPreservesOrderSnapshots(t *testing.T) {
	state := &promotionCampaignTestState{
		appExists:  true,
		campaignID: 42,
		appID:      9,
	}
	db := openPromotionCampaignTestDB(t, state)

	if err := deletePromotionCampaign(context.Background(), db, 42); err != nil {
		t.Fatal(err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.queries) != 3 {
		t.Fatalf("queries = %d; want campaign lookup, app lock and campaign lock", len(state.queries))
	}
	if strings.Contains(state.queries[0], "FOR UPDATE") || !strings.Contains(state.queries[0], "SELECT app_id FROM promotion_campaigns") {
		t.Fatalf("unexpected campaign lookup: %s", state.queries[0])
	}
	if !strings.Contains(state.queries[1], "FROM apps") || !strings.Contains(state.queries[1], "FOR UPDATE") {
		t.Fatalf("delete did not lock app: %s", state.queries[1])
	}
	if !strings.Contains(state.queries[2], "FROM promotion_campaigns") || !strings.Contains(state.queries[2], "FOR UPDATE") {
		t.Fatalf("delete did not lock campaign: %s", state.queries[2])
	}
	if len(state.execs) != 2 || !strings.Contains(state.execs[0], "DELETE FROM promotion_campaign_plans") || !strings.Contains(state.execs[1], "DELETE FROM promotion_campaigns") {
		t.Fatalf("unexpected delete writes: %#v", state.execs)
	}
	for index, args := range state.execArgs {
		if len(args) != 1 || args[0].Value != int64(42) {
			t.Fatalf("delete args %d = %#v", index, args)
		}
	}
	writes := strings.ToLower(strings.Join(state.execs, " "))
	if strings.Contains(writes, "order") || strings.Contains(writes, "pricing_snapshot") {
		t.Fatalf("delete must not modify historical order snapshots: %#v", state.execs)
	}
	if state.commits != 1 || state.rollbacks != 0 {
		t.Fatalf("unexpected transaction result: commits=%d rollbacks=%d", state.commits, state.rollbacks)
	}
}

func TestDeletePromotionCampaignReturnsNotFound(t *testing.T) {
	state := &promotionCampaignTestState{}
	db := openPromotionCampaignTestDB(t, state)

	err := deletePromotionCampaign(context.Background(), db, 42)
	if !errors.Is(err, errPromotionCampaignNotFound) {
		t.Fatalf("delete error = %v", err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.queries) != 1 || len(state.execs) != 0 || state.commits != 0 || state.rollbacks != 0 {
		t.Fatalf("unexpected missing-campaign operations: queries=%d execs=%d commits=%d rollbacks=%d", len(state.queries), len(state.execs), state.commits, state.rollbacks)
	}
}

func TestDeletePromotionCampaignRollsBackWhenCampaignDeleteFails(t *testing.T) {
	state := &promotionCampaignTestState{
		appExists:   true,
		campaignID:  42,
		appID:       9,
		execErrorOn: "DELETE FROM promotion_campaigns WHERE id",
	}
	db := openPromotionCampaignTestDB(t, state)

	err := deletePromotionCampaign(context.Background(), db, 42)
	if err == nil || err.Error() != "forced exec failure" {
		t.Fatalf("delete error = %v", err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.execs) != 2 || state.commits != 0 || state.rollbacks != 1 {
		t.Fatalf("delete failure was not rolled back: execs=%d commits=%d rollbacks=%d", len(state.execs), state.commits, state.rollbacks)
	}
}
