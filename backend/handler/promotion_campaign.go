package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

var (
	errPromotionCampaignConflict = errors.New("该应用在所选时间段内已有活动")
	errPromotionCampaignNotFound = errors.New("活动不存在")
	errPromotionCampaignApp      = errors.New("应用不存在")
	openPromotionCampaignDB      = func() (*sql.DB, error) {
		cfg, err := config.LoadDBConfig()
		if err != nil {
			return nil, err
		}
		return sql.Open("mysql", config.GetDSN(cfg))
	}
)

type promotionRuleType string

const (
	promotionRuleDiscount   promotionRuleType = "discount"
	promotionRuleReduction  promotionRuleType = "reduction"
	promotionRuleFixedPrice promotionRuleType = "fixed_price"
)

type promotionCampaignPlanWrite struct {
	PlanID     int64
	RuleType   promotionRuleType
	ValueUnits int64
}

type promotionCampaignWrite struct {
	AppID     int64
	Name      string
	Audience  purchaseAudience
	StartsAt  time.Time
	EndsAt    time.Time
	Enabled   bool
	CreatedBy int64
	Plans     []promotionCampaignPlanWrite
}

type promotionCampaignPlanRequest struct {
	PlanID   int64    `json:"planId"`
	RuleType string   `json:"ruleType"`
	Value    *float64 `json:"value"`
	Price    *float64 `json:"price"`
}

type promotionCampaignRequest struct {
	AppID    int64                          `json:"appId"`
	Name     string                         `json:"name"`
	Audience string                         `json:"audience"`
	StartsAt string                         `json:"startsAt"`
	EndsAt   string                         `json:"endsAt"`
	Enabled  bool                           `json:"enabled"`
	Plans    []promotionCampaignPlanRequest `json:"plans"`
}

type promotionCampaignPlanItem struct {
	PlanID        int64             `json:"planId"`
	PlanName      string            `json:"planName"`
	OriginalPrice float64           `json:"originalPrice"`
	RuleType      promotionRuleType `json:"ruleType"`
	Value         float64           `json:"value"`
}

type promotionCampaignListItem struct {
	ID        int64                       `json:"id"`
	AppID     int64                       `json:"appId"`
	AppName   string                      `json:"appName"`
	Name      string                      `json:"name"`
	Audience  purchaseAudience            `json:"audience"`
	StartsAt  string                      `json:"startsAt"`
	EndsAt    string                      `json:"endsAt"`
	Enabled   bool                        `json:"enabled"`
	Status    string                      `json:"status"`
	CreatedAt string                      `json:"createdAt"`
	UpdatedAt string                      `json:"updatedAt"`
	Plans     []promotionCampaignPlanItem `json:"plans"`
}

type promotionCampaignListFilter struct {
	AppID    int64
	Keyword  string
	Audience purchaseAudience
	Status   string
}

func promotionCampaignStatus(enabled bool, startsAt, endsAt, now time.Time) string {
	if !enabled {
		return "disabled"
	}
	if now.Before(startsAt) {
		return "upcoming"
	}
	if !now.Before(endsAt) {
		return "ended"
	}
	return "active"
}

func parsePromotionRuleType(value string, legacyPrice *float64) (promotionRuleType, error) {
	ruleType := promotionRuleType(strings.ToLower(strings.TrimSpace(value)))
	if ruleType == "" && legacyPrice != nil {
		return promotionRuleFixedPrice, nil
	}
	switch ruleType {
	case promotionRuleDiscount, promotionRuleReduction, promotionRuleFixedPrice:
		return ruleType, nil
	default:
		return "", fmt.Errorf("无效的活动优惠方式")
	}
}

func parsePromotionRuleValue(plan promotionCampaignPlanRequest, ruleType promotionRuleType) (int64, error) {
	value := plan.Value
	if value == nil && ruleType == promotionRuleFixedPrice {
		value = plan.Price
	}
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) {
		return 0, fmt.Errorf("请填写活动优惠值")
	}
	switch ruleType {
	case promotionRuleDiscount:
		units := int64(math.Round(*value * 1000))
		if *value <= 0 || *value > 10 || units <= 0 || units > 10000 {
			return 0, fmt.Errorf("活动折扣必须大于0且不超过10折")
		}
		return units, nil
	case promotionRuleReduction:
		amountCents := floatAmountToCents(*value)
		if *value <= 0 || amountCents <= 0 {
			return 0, fmt.Errorf("立减金额必须大于0")
		}
		return amountCents, nil
	case promotionRuleFixedPrice:
		if *value < 0 {
			return 0, fmt.Errorf("固定活动价不能小于0")
		}
		return floatAmountToCents(*value), nil
	default:
		return 0, fmt.Errorf("无效的活动优惠方式")
	}
}

func promotionRuleValueText(ruleType promotionRuleType, valueUnits int64) string {
	if ruleType == promotionRuleDiscount {
		return strconv.FormatFloat(float64(valueUnits)/1000, 'f', -1, 64)
	}
	return formatCents(valueUnits)
}

func ensurePromotionCampaignSchema(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS promotion_campaigns (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
			app_id BIGINT UNSIGNED NOT NULL COMMENT '应用ID',
			name VARCHAR(100) NOT NULL COMMENT '活动名称',
			audience ENUM('user','agent','all') NOT NULL COMMENT '适用对象',
			starts_at DATETIME NOT NULL COMMENT '开始时间（含）',
			ends_at DATETIME NOT NULL COMMENT '结束时间（不含）',
			enabled TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否启用',
			created_by BIGINT UNSIGNED DEFAULT NULL COMMENT '创建管理员ID',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
			PRIMARY KEY (id),
			KEY idx_promotion_app_time (app_id, enabled, starts_at, ends_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='促销活动表'
	`); err != nil {
		return err
	}
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS promotion_campaign_plans (
			campaign_id BIGINT UNSIGNED NOT NULL COMMENT '活动ID',
			plan_id BIGINT UNSIGNED NOT NULL COMMENT '套餐ID',
			rule_type ENUM('discount','reduction','fixed_price') NOT NULL DEFAULT 'fixed_price' COMMENT '优惠方式',
			rule_value DECIMAL(12,4) NOT NULL DEFAULT 0 COMMENT '折扣值或金额',
			promotion_price DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '兼容旧版固定活动价',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
			PRIMARY KEY (campaign_id, plan_id),
			KEY idx_promotion_plan (plan_id, campaign_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='活动套餐规则表'
	`)
	if err != nil {
		return err
	}
	if err := ensureColumn(db, "promotion_campaign_plans", "rule_type",
		"ALTER TABLE promotion_campaign_plans ADD COLUMN rule_type ENUM('discount','reduction','fixed_price') NOT NULL DEFAULT 'fixed_price' COMMENT '优惠方式' AFTER plan_id"); err != nil {
		return err
	}
	if err := ensureColumn(db, "promotion_campaign_plans", "rule_value",
		"ALTER TABLE promotion_campaign_plans ADD COLUMN rule_value DECIMAL(12,4) NOT NULL DEFAULT 0 COMMENT '折扣值或金额' AFTER rule_type"); err != nil {
		return err
	}
	_, err = db.Exec(`
		UPDATE promotion_campaign_plans
		SET rule_value = promotion_price
		WHERE rule_type = 'fixed_price' AND rule_value = 0 AND promotion_price <> 0
	`)
	return err
}

func parsePromotionCampaignRequest(req promotionCampaignRequest, createdBy int64) (promotionCampaignWrite, error) {
	name := strings.TrimSpace(req.Name)
	if req.AppID <= 0 {
		return promotionCampaignWrite{}, fmt.Errorf("请选择应用")
	}
	if name == "" || len([]rune(name)) > 100 {
		return promotionCampaignWrite{}, fmt.Errorf("活动名称长度需为 1-100 个字符")
	}
	audience, err := parsePurchaseAudience(req.Audience)
	if err != nil {
		return promotionCampaignWrite{}, err
	}
	startsAt, err := time.Parse(time.RFC3339, strings.TrimSpace(req.StartsAt))
	if err != nil {
		return promotionCampaignWrite{}, fmt.Errorf("开始时间格式错误")
	}
	endsAt, err := time.Parse(time.RFC3339, strings.TrimSpace(req.EndsAt))
	if err != nil {
		return promotionCampaignWrite{}, fmt.Errorf("结束时间格式错误")
	}
	startsAt = startsAt.Local().Truncate(time.Second)
	endsAt = endsAt.Local().Truncate(time.Second)
	if !endsAt.After(startsAt) {
		return promotionCampaignWrite{}, fmt.Errorf("结束时间必须晚于开始时间")
	}
	if len(req.Plans) == 0 {
		return promotionCampaignWrite{}, fmt.Errorf("请至少设置一个套餐活动价")
	}
	plans := make([]promotionCampaignPlanWrite, 0, len(req.Plans))
	seenPlans := make(map[int64]struct{}, len(req.Plans))
	for _, plan := range req.Plans {
		if plan.PlanID <= 0 {
			return promotionCampaignWrite{}, fmt.Errorf("套餐ID错误")
		}
		if _, exists := seenPlans[plan.PlanID]; exists {
			return promotionCampaignWrite{}, fmt.Errorf("套餐不能重复设置")
		}
		ruleType, err := parsePromotionRuleType(plan.RuleType, plan.Price)
		if err != nil {
			return promotionCampaignWrite{}, err
		}
		valueUnits, err := parsePromotionRuleValue(plan, ruleType)
		if err != nil {
			return promotionCampaignWrite{}, err
		}
		seenPlans[plan.PlanID] = struct{}{}
		plans = append(plans, promotionCampaignPlanWrite{
			PlanID:     plan.PlanID,
			RuleType:   ruleType,
			ValueUnits: valueUnits,
		})
	}
	return promotionCampaignWrite{
		AppID:     req.AppID,
		Name:      name,
		Audience:  audience,
		StartsAt:  startsAt,
		EndsAt:    endsAt,
		Enabled:   req.Enabled,
		CreatedBy: createdBy,
		Plans:     plans,
	}, nil
}

func promotionTimeRangesOverlap(firstStart, firstEnd, secondStart, secondEnd time.Time) bool {
	return firstStart.Before(secondEnd) && firstEnd.After(secondStart)
}

func lockPromotionCampaignApp(ctx context.Context, tx *sql.Tx, appID int64) error {
	var lockedAppID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM apps WHERE id = ? FOR UPDATE", appID).Scan(&lockedAppID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errPromotionCampaignApp
		}
		return err
	}
	return nil
}

func validatePromotionCampaignPlans(ctx context.Context, tx *sql.Tx, campaign promotionCampaignWrite) error {
	if len(campaign.Plans) == 0 {
		return fmt.Errorf("请至少设置一个套餐活动价")
	}
	for _, campaignPlan := range campaign.Plans {
		var originalPrice float64
		if err := tx.QueryRowContext(ctx, `
			SELECT price
			FROM license_plans
			WHERE id = ? AND app_id = ?
			FOR UPDATE
		`, campaignPlan.PlanID, campaign.AppID).Scan(&originalPrice); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("套餐不存在或不属于该应用")
			}
			return err
		}
		originalCents := floatAmountToCents(originalPrice)
		switch campaignPlan.RuleType {
		case promotionRuleDiscount:
			if campaignPlan.ValueUnits <= 0 || campaignPlan.ValueUnits > 10000 {
				return fmt.Errorf("活动折扣必须大于0且不超过10折")
			}
		case promotionRuleReduction:
			if campaignPlan.ValueUnits <= 0 || campaignPlan.ValueUnits > originalCents {
				return fmt.Errorf("立减金额不能超过套餐原价")
			}
		case promotionRuleFixedPrice:
			if campaignPlan.ValueUnits < 0 || campaignPlan.ValueUnits > originalCents {
				return fmt.Errorf("固定活动价不能高于套餐原价")
			}
		default:
			return fmt.Errorf("无效的活动优惠方式")
		}
	}
	return nil
}

func replacePromotionCampaignPlans(ctx context.Context, tx *sql.Tx, campaignID int64, plans []promotionCampaignPlanWrite) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM promotion_campaign_plans WHERE campaign_id = ?", campaignID); err != nil {
		return err
	}
	for _, plan := range plans {
		legacyPrice := "0.00"
		if plan.RuleType == promotionRuleFixedPrice {
			legacyPrice = formatCents(plan.ValueUnits)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO promotion_campaign_plans (campaign_id, plan_id, rule_type, rule_value, promotion_price)
			VALUES (?, ?, ?, ?, ?)
		`, campaignID, plan.PlanID, plan.RuleType, promotionRuleValueText(plan.RuleType, plan.ValueUnits), legacyPrice); err != nil {
			return err
		}
	}
	return nil
}

func findPromotionCampaignConflict(ctx context.Context, tx *sql.Tx, campaignID, appID int64, startsAt, endsAt time.Time) (int64, string, error) {
	var conflictingID int64
	var conflictingName string
	err := tx.QueryRowContext(ctx, `
		SELECT id, name
		FROM promotion_campaigns
		WHERE app_id = ? AND enabled = 1 AND id <> ?
		  AND starts_at < ? AND ends_at > ?
		ORDER BY starts_at ASC, id ASC
		LIMIT 1
		FOR UPDATE
	`, appID, campaignID, endsAt, startsAt).Scan(&conflictingID, &conflictingName)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", nil
	}
	return conflictingID, conflictingName, err
}

func createPromotionCampaign(ctx context.Context, db *sql.DB, campaign promotionCampaignWrite) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if err := lockPromotionCampaignApp(ctx, tx, campaign.AppID); err != nil {
		return 0, err
	}
	if err := validatePromotionCampaignPlans(ctx, tx, campaign); err != nil {
		return 0, err
	}
	if campaign.Enabled {
		conflictingID, _, err := findPromotionCampaignConflict(ctx, tx, 0, campaign.AppID, campaign.StartsAt, campaign.EndsAt)
		if err != nil {
			return 0, err
		}
		if conflictingID > 0 {
			return 0, errPromotionCampaignConflict
		}
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO promotion_campaigns (app_id, name, audience, starts_at, ends_at, enabled, created_by)
		VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, 0))
	`, campaign.AppID, campaign.Name, campaign.Audience, campaign.StartsAt, campaign.EndsAt, campaign.Enabled, campaign.CreatedBy)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := replacePromotionCampaignPlans(ctx, tx, id, campaign.Plans); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func updatePromotionCampaign(ctx context.Context, db *sql.DB, campaignID int64, campaign promotionCampaignWrite) error {
	if campaignID <= 0 {
		return errPromotionCampaignNotFound
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := lockPromotionCampaignApp(ctx, tx, campaign.AppID); err != nil {
		return err
	}
	var storedAppID int64
	if err := tx.QueryRowContext(ctx, "SELECT app_id FROM promotion_campaigns WHERE id = ? FOR UPDATE", campaignID).Scan(&storedAppID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errPromotionCampaignNotFound
		}
		return err
	}
	if storedAppID != campaign.AppID {
		return fmt.Errorf("活动所属应用不可修改")
	}
	if err := validatePromotionCampaignPlans(ctx, tx, campaign); err != nil {
		return err
	}
	if campaign.Enabled {
		conflictingID, _, err := findPromotionCampaignConflict(ctx, tx, campaignID, campaign.AppID, campaign.StartsAt, campaign.EndsAt)
		if err != nil {
			return err
		}
		if conflictingID > 0 {
			return errPromotionCampaignConflict
		}
	}
	// 前面已 SELECT ... FOR UPDATE 确认行存在；MySQL 在值未变化时 RowsAffected=0，不能据此判断不存在
	if _, err := tx.ExecContext(ctx, `
		UPDATE promotion_campaigns
		SET name = ?, audience = ?, starts_at = ?, ends_at = ?, enabled = ?
		WHERE id = ?
	`, campaign.Name, campaign.Audience, campaign.StartsAt, campaign.EndsAt, campaign.Enabled, campaignID); err != nil {
		return err
	}
	if err := replacePromotionCampaignPlans(ctx, tx, campaignID, campaign.Plans); err != nil {
		return err
	}
	return tx.Commit()
}

func setPromotionCampaignEnabled(ctx context.Context, db *sql.DB, campaignID int64, enabled bool) error {
	if campaignID <= 0 {
		return errPromotionCampaignNotFound
	}
	var appID int64
	if err := db.QueryRowContext(ctx, "SELECT app_id FROM promotion_campaigns WHERE id = ?", campaignID).Scan(&appID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errPromotionCampaignNotFound
		}
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockPromotionCampaignApp(ctx, tx, appID); err != nil {
		return err
	}

	var storedAppID int64
	var startsAt, endsAt time.Time
	if err := tx.QueryRowContext(ctx, `
		SELECT app_id, starts_at, ends_at
		FROM promotion_campaigns
		WHERE id = ?
		FOR UPDATE
	`, campaignID).Scan(&storedAppID, &startsAt, &endsAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errPromotionCampaignNotFound
		}
		return err
	}
	if storedAppID != appID {
		return fmt.Errorf("活动所属应用已变更，请重试")
	}
	if enabled {
		conflictingID, _, err := findPromotionCampaignConflict(ctx, tx, campaignID, appID, startsAt, endsAt)
		if err != nil {
			return err
		}
		if conflictingID > 0 {
			return errPromotionCampaignConflict
		}
	}
	// 同上：行已锁定存在，值未变化时 RowsAffected=0 属于正常情况
	if _, err := tx.ExecContext(ctx, "UPDATE promotion_campaigns SET enabled = ? WHERE id = ?", enabled, campaignID); err != nil {
		return err
	}
	return tx.Commit()
}

func deletePromotionCampaign(ctx context.Context, db *sql.DB, campaignID int64) error {
	if campaignID <= 0 {
		return errPromotionCampaignNotFound
	}

	var appID int64
	if err := db.QueryRowContext(ctx, "SELECT app_id FROM promotion_campaigns WHERE id = ?", campaignID).Scan(&appID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errPromotionCampaignNotFound
		}
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := lockPromotionCampaignApp(ctx, tx, appID); err != nil {
		return err
	}
	var storedAppID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT app_id
		FROM promotion_campaigns
		WHERE id = ?
		FOR UPDATE
	`, campaignID).Scan(&storedAppID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errPromotionCampaignNotFound
		}
		return err
	}
	if storedAppID != appID {
		return fmt.Errorf("活动所属应用已变更，请重试")
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM promotion_campaign_plans WHERE campaign_id = ?", campaignID); err != nil {
		return err
	}
	// 行已锁定存在，直接删除即可
	if _, err := tx.ExecContext(ctx, "DELETE FROM promotion_campaigns WHERE id = ?", campaignID); err != nil {
		return err
	}
	return tx.Commit()
}

func listPromotionCampaigns(ctx context.Context, db *sql.DB, filter promotionCampaignListFilter) ([]promotionCampaignListItem, error) {
	where := []string{"1 = 1"}
	args := make([]any, 0, 6)
	if filter.AppID > 0 {
		where = append(where, "pc.app_id = ?")
		args = append(args, filter.AppID)
	}
	if filter.Keyword != "" {
		where = append(where, "(pc.name LIKE ? OR a.app_name LIKE ?)")
		keyword := "%" + filter.Keyword + "%"
		args = append(args, keyword, keyword)
	}
	if filter.Audience != "" {
		where = append(where, "pc.audience = ?")
		args = append(args, filter.Audience)
	}
	switch filter.Status {
	case "active":
		where = append(where, "pc.enabled = 1 AND pc.starts_at <= NOW() AND pc.ends_at > NOW()")
	case "upcoming":
		where = append(where, "pc.enabled = 1 AND pc.starts_at > NOW()")
	case "ended":
		where = append(where, "pc.enabled = 1 AND pc.ends_at <= NOW()")
	case "disabled":
		where = append(where, "pc.enabled = 0")
	}

	rows, err := db.QueryContext(ctx, fmt.Sprintf(`
		SELECT pc.id, pc.app_id, a.app_name, pc.name, pc.audience,
		       pc.starts_at, pc.ends_at, pc.enabled, pc.created_at, pc.updated_at
		FROM promotion_campaigns pc
		JOIN apps a ON a.id = pc.app_id
		WHERE %s
		ORDER BY pc.starts_at DESC, pc.id DESC
	`, strings.Join(where, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now()
	items := make([]promotionCampaignListItem, 0)
	itemIndexes := make(map[int64]int)
	campaignIDs := make([]any, 0)
	for rows.Next() {
		var item promotionCampaignListItem
		var audienceText string
		var startsAt, endsAt, createdAt, updatedAt time.Time
		if err := rows.Scan(
			&item.ID, &item.AppID, &item.AppName, &item.Name, &audienceText,
			&startsAt, &endsAt, &item.Enabled, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		audience, err := parsePurchaseAudience(audienceText)
		if err != nil {
			return nil, err
		}
		item.Audience = audience
		item.StartsAt = startsAt.Format(time.RFC3339)
		item.EndsAt = endsAt.Format(time.RFC3339)
		item.Status = promotionCampaignStatus(item.Enabled, startsAt, endsAt, now)
		item.CreatedAt = createdAt.Format("2006-01-02 15:04")
		item.UpdatedAt = updatedAt.Format("2006-01-02 15:04")
		item.Plans = []promotionCampaignPlanItem{}
		itemIndexes[item.ID] = len(items)
		campaignIDs = append(campaignIDs, item.ID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(campaignIDs) == 0 {
		return items, nil
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(campaignIDs)), ",")
	planRows, err := db.QueryContext(ctx, fmt.Sprintf(`
		SELECT pcp.campaign_id, p.id, p.name, p.price, pcp.rule_type,
		       CASE
		         WHEN pcp.rule_type = 'fixed_price' AND pcp.rule_value = 0 AND pcp.promotion_price <> 0
		         THEN pcp.promotion_price
		         ELSE pcp.rule_value
		       END AS rule_value
		FROM promotion_campaign_plans pcp
		JOIN license_plans p ON p.id = pcp.plan_id
		WHERE pcp.campaign_id IN (%s)
		ORDER BY pcp.campaign_id, p.sort ASC, p.id ASC
	`, placeholders), campaignIDs...)
	if err != nil {
		return nil, err
	}
	defer planRows.Close()
	for planRows.Next() {
		var campaignID int64
		var plan promotionCampaignPlanItem
		var ruleTypeText string
		if err := planRows.Scan(
			&campaignID, &plan.PlanID, &plan.PlanName, &plan.OriginalPrice, &ruleTypeText, &plan.Value,
		); err != nil {
			return nil, err
		}
		ruleType, err := parsePromotionRuleType(ruleTypeText, nil)
		if err != nil {
			return nil, err
		}
		plan.RuleType = ruleType
		if index, exists := itemIndexes[campaignID]; exists {
			items[index].Plans = append(items[index].Plans, plan)
		}
	}
	if err := planRows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func AdminPromotionCampaignList(c *gin.Context) {
	filter := promotionCampaignListFilter{
		Keyword: strings.TrimSpace(c.Query("keyword")),
		Status:  strings.ToLower(strings.TrimSpace(c.Query("status"))),
	}
	if appIDText := strings.TrimSpace(c.Query("appId")); appIDText != "" {
		appID, err := parsePositiveInt64(appIDText)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "应用ID错误"})
			return
		}
		filter.AppID = appID
	}
	if audienceText := strings.TrimSpace(c.Query("audience")); audienceText != "" {
		audience, err := parsePurchaseAudience(audienceText)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		filter.Audience = audience
	}
	switch filter.Status {
	case "", "active", "upcoming", "ended", "disabled":
	default:
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "活动状态错误"})
		return
	}

	db, err := openPromotionCampaignDB()
	if err != nil {
		promotionCampaignError(c, err)
		return
	}
	defer db.Close()
	if err := ensurePromotionCampaignSchema(db); err != nil {
		promotionCampaignError(c, err)
		return
	}
	items, err := listPromotionCampaigns(c.Request.Context(), db, filter)
	if err != nil {
		promotionCampaignError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": items})
}

func promotionCampaignError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errPromotionCampaignConflict):
		c.JSON(http.StatusOK, gin.H{"code": 409, "msg": err.Error()})
	case errors.Is(err, errPromotionCampaignNotFound), errors.Is(err, errPromotionCampaignApp):
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": err.Error()})
	default:
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "活动操作失败: " + err.Error()})
	}
}

func AdminPromotionCampaignCreate(c *gin.Context) {
	var req promotionCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	createdBy := int64(0)
	if userID, exists := c.Get("user_id"); exists {
		switch value := userID.(type) {
		case uint:
			createdBy = int64(value)
		case int64:
			createdBy = value
		}
	}
	campaign, err := parsePromotionCampaignRequest(req, createdBy)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	db, err := openPromotionCampaignDB()
	if err != nil {
		promotionCampaignError(c, err)
		return
	}
	defer db.Close()
	if err := ensurePromotionCampaignSchema(db); err != nil {
		promotionCampaignError(c, err)
		return
	}
	id, err := createPromotionCampaign(c.Request.Context(), db, campaign)
	if err != nil {
		promotionCampaignError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "创建成功", "data": gin.H{"id": id}})
}

func AdminPromotionCampaignUpdate(c *gin.Context) {
	campaignID, err := parsePositiveInt64(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "活动ID错误"})
		return
	}
	var req promotionCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	campaign, err := parsePromotionCampaignRequest(req, 0)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	db, err := openPromotionCampaignDB()
	if err != nil {
		promotionCampaignError(c, err)
		return
	}
	defer db.Close()
	if err := ensurePromotionCampaignSchema(db); err != nil {
		promotionCampaignError(c, err)
		return
	}
	if err := updatePromotionCampaign(c.Request.Context(), db, campaignID, campaign); err != nil {
		promotionCampaignError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "更新成功"})
}

func AdminPromotionCampaignToggle(c *gin.Context) {
	campaignID, err := parsePositiveInt64(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "活动ID错误"})
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	db, err := openPromotionCampaignDB()
	if err != nil {
		promotionCampaignError(c, err)
		return
	}
	defer db.Close()
	if err := ensurePromotionCampaignSchema(db); err != nil {
		promotionCampaignError(c, err)
		return
	}
	if err := setPromotionCampaignEnabled(c.Request.Context(), db, campaignID, *req.Enabled); err != nil {
		promotionCampaignError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "操作成功"})
}

func AdminPromotionCampaignDelete(c *gin.Context) {
	campaignID, err := parsePositiveInt64(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "活动ID错误"})
		return
	}
	db, err := openPromotionCampaignDB()
	if err != nil {
		promotionCampaignError(c, err)
		return
	}
	defer db.Close()
	if err := ensurePromotionCampaignSchema(db); err != nil {
		promotionCampaignError(c, err)
		return
	}
	if err := deletePromotionCampaign(c.Request.Context(), db, campaignID); err != nil {
		promotionCampaignError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "删除成功"})
}

func parsePositiveInt64(value string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("必须为正整数")
	}
	return parsed, nil
}
