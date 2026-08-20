package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type purchaseAudience string

const (
	purchaseAudienceUser  purchaseAudience = "user"
	purchaseAudienceAgent purchaseAudience = "agent"
	purchaseAudienceAll   purchaseAudience = "all"
)

var (
	errInvalidPurchaseAudience  = errors.New("无效的活动适用对象")
	loadActivePurchasePromotion = queryActivePurchasePromotion
)

type purchasePlanPricing struct {
	AppID        int64
	PlanID       int64
	AppName      string
	PlanName     string
	DurationDays int
	MaxSites     int
	PriceCents   int64
	LicenseType  string
}

type purchasePromotionCandidate struct {
	ID             int64
	Name           string
	Audience       purchaseAudience
	RuleType       promotionRuleType
	RuleValueUnits int64
	AmountCents    int64
	RuleSnapshot   string
	StartsAt       string
	EndsAt         string
}

type purchasePricingInput struct {
	BuyerType     purchaseAudience
	OriginalCents int64
	AgentDiscount float64
	Promotion     *purchasePromotionCandidate
}

type purchasePriceQuote struct {
	BuyerType         purchaseAudience
	OriginalCents     int64
	BaseCents         int64
	AmountCents       int64
	DiscountCents     int64
	AgentDiscount     float64
	PromotionID       int64
	PromotionName     string
	PromotionAudience purchaseAudience
	PromotionRule     string
	PromotionRuleType promotionRuleType
	PromotionDiscount float64
	PromotionStartsAt string
	PromotionEndsAt   string
}

type purchasePricingSnapshot struct {
	Version           int              `json:"version"`
	BuyerType         purchaseAudience `json:"buyerType"`
	OriginalAmount    string           `json:"originalAmount"`
	BaseAmount        string           `json:"baseAmount"`
	Amount            string           `json:"amount"`
	DiscountAmount    string           `json:"discountAmount"`
	AgentDiscount     float64          `json:"agentDiscount,omitempty"`
	PromotionID       int64            `json:"promotionId,omitempty"`
	PromotionName     string           `json:"promotionName,omitempty"`
	PromotionAudience purchaseAudience `json:"promotionAudience,omitempty"`
	PromotionRule     json.RawMessage  `json:"promotionRule,omitempty"`
}

type purchaseOrderSnapshot struct {
	OriginalAmount  string
	BaseAmount      string
	Amount          string
	DiscountAmount  string
	PromotionID     interface{}
	PromotionName   string
	PromotionRule   interface{}
	PricingSnapshot string
	AppName         string
	PlanName        string
	DurationDays    int
	MaxSites        int
}

func parsePurchaseAudience(value string) (purchaseAudience, error) {
	audience := purchaseAudience(strings.ToLower(strings.TrimSpace(value)))
	switch audience {
	case purchaseAudienceUser, purchaseAudienceAgent, purchaseAudienceAll:
		return audience, nil
	default:
		return "", errInvalidPurchaseAudience
	}
}

func purchaseAudienceIncludes(activityAudience, buyerType purchaseAudience) bool {
	if buyerType != purchaseAudienceUser && buyerType != purchaseAudienceAgent {
		return false
	}
	return activityAudience == purchaseAudienceAll || activityAudience == buyerType
}

func calculatePurchasePrice(input purchasePricingInput) (purchasePriceQuote, error) {
	if input.BuyerType != purchaseAudienceUser && input.BuyerType != purchaseAudienceAgent {
		return purchasePriceQuote{}, errInvalidPurchaseAudience
	}

	originalCents := input.OriginalCents
	if originalCents < 0 {
		originalCents = 0
	}
	quote := purchasePriceQuote{
		BuyerType:     input.BuyerType,
		OriginalCents: originalCents,
		BaseCents:     originalCents,
		AmountCents:   originalCents,
	}
	if input.BuyerType == purchaseAudienceAgent {
		discount := input.AgentDiscount
		if discount < 1 || discount > 10 {
			discount = 10
		}
		quote.AgentDiscount = discount
		quote.BaseCents = floatAmountToCents(float64(originalCents) / 100 * discount / 10)
		quote.AmountCents = quote.BaseCents
	}

	promotion := input.Promotion
	if promotion == nil || !purchaseAudienceIncludes(promotion.Audience, input.BuyerType) {
		return quote, nil
	}
	if promotion.AmountCents < 0 {
		return purchasePriceQuote{}, fmt.Errorf("活动价格不能小于 0")
	}
	if promotion.AmountCents >= quote.BaseCents {
		return quote, nil
	}

	quote.AmountCents = promotion.AmountCents
	quote.DiscountCents = quote.BaseCents - quote.AmountCents
	quote.PromotionID = promotion.ID
	quote.PromotionName = strings.TrimSpace(promotion.Name)
	quote.PromotionAudience = promotion.Audience
	quote.PromotionRuleType = promotion.RuleType
	if promotion.RuleType == promotionRuleDiscount {
		quote.PromotionDiscount = float64(promotion.RuleValueUnits) / 1000
	}
	quote.PromotionStartsAt = promotion.StartsAt
	quote.PromotionEndsAt = promotion.EndsAt
	quote.PromotionRule = strings.TrimSpace(promotion.RuleSnapshot)
	return quote, nil
}

func loadPurchasePlanPricing(db *sql.DB, appID, planID int64) (purchasePlanPricing, error) {
	var plan purchasePlanPricing
	var price float64
	var licenseType sql.NullString
	err := db.QueryRow(`
		SELECT a.app_name, p.name, p.license_type, p.price, p.duration_days, COALESCE(p.max_sites, 0)
		FROM license_plans p
		JOIN apps a ON a.id = p.app_id
		WHERE p.id = ? AND p.app_id = ? AND p.enabled = 1 AND a.enabled = 1
	`, planID, appID).Scan(&plan.AppName, &plan.PlanName, &licenseType, &price, &plan.DurationDays, &plan.MaxSites)
	if err != nil {
		return purchasePlanPricing{}, err
	}
	plan.AppID = appID
	plan.PlanID = planID
	plan.LicenseType = strings.ToLower(strings.TrimSpace(licenseType.String))
	plan.PriceCents = floatAmountToCents(price)
	return plan, nil
}

// planLicenseTypeMatches 判断套餐是否适用于当前授权方式：空值=通用
func planLicenseTypeMatches(planLicenseType, licenseType string) bool {
	planLicenseType = strings.ToLower(strings.TrimSpace(planLicenseType))
	licenseType = strings.ToLower(strings.TrimSpace(licenseType))
	if planLicenseType == "" {
		return true
	}
	return planLicenseType == licenseType
}

func calculatePromotionAmountCents(originalCents int64, ruleType promotionRuleType, valueUnits int64) (int64, error) {
	if originalCents < 0 {
		originalCents = 0
	}
	switch ruleType {
	case promotionRuleDiscount:
		if valueUnits <= 0 || valueUnits > 10000 {
			return 0, fmt.Errorf("活动折扣超出范围")
		}
		return int64(math.Round(float64(originalCents) * float64(valueUnits) / 10000)), nil
	case promotionRuleReduction:
		if valueUnits <= 0 {
			return 0, fmt.Errorf("立减金额必须大于0")
		}
		if valueUnits >= originalCents {
			return 0, nil
		}
		return originalCents - valueUnits, nil
	case promotionRuleFixedPrice:
		if valueUnits < 0 {
			return 0, fmt.Errorf("固定活动价不能小于0")
		}
		return valueUnits, nil
	default:
		return 0, fmt.Errorf("无效的活动优惠方式")
	}
}

func buildPromotionRuleSnapshot(ruleType promotionRuleType, valueUnits, amountCents int64) (string, error) {
	rule := struct {
		Type            promotionRuleType `json:"type"`
		Discount        float64           `json:"discount,omitempty"`
		ReductionAmount string            `json:"reductionAmount,omitempty"`
		FixedAmount     string            `json:"fixedAmount,omitempty"`
		Amount          string            `json:"amount"`
		Stacking        string            `json:"stacking"`
	}{
		Type:     ruleType,
		Amount:   formatCents(amountCents),
		Stacking: "lowest_only",
	}
	switch ruleType {
	case promotionRuleDiscount:
		rule.Discount = float64(valueUnits) / 1000
	case promotionRuleReduction:
		rule.ReductionAmount = formatCents(valueUnits)
	case promotionRuleFixedPrice:
		rule.FixedAmount = formatCents(valueUnits)
	default:
		return "", fmt.Errorf("无效的活动优惠方式")
	}
	encoded, err := json.Marshal(rule)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func queryActivePurchasePromotion(db *sql.DB, appID, planID, originalCents int64, buyerType purchaseAudience) (*purchasePromotionCandidate, error) {
	if buyerType != purchaseAudienceUser && buyerType != purchaseAudienceAgent {
		return nil, errInvalidPurchaseAudience
	}
	var candidate purchasePromotionCandidate
	var audienceText, ruleTypeText, ruleValueText string
	var startsAt, endsAt time.Time
	err := db.QueryRow(`
		SELECT pc.id, pc.name, pc.audience, pcp.rule_type,
		       CASE
		         WHEN pcp.rule_type = 'fixed_price' AND pcp.rule_value = 0 AND pcp.promotion_price <> 0
		         THEN pcp.promotion_price
		         ELSE pcp.rule_value
		       END AS rule_value,
		       pc.starts_at, pc.ends_at
		FROM promotion_campaigns pc
		JOIN promotion_campaign_plans pcp ON pcp.campaign_id = pc.id
		WHERE pc.app_id = ? AND pcp.plan_id = ?
		  AND pc.enabled = 1
		  AND pc.starts_at <= NOW() AND pc.ends_at > NOW()
		  AND pc.audience IN (?, 'all')
		ORDER BY pc.starts_at DESC, pc.id DESC
		LIMIT 1
	`, appID, planID, buyerType).Scan(
		&candidate.ID, &candidate.Name, &audienceText, &ruleTypeText, &ruleValueText,
		&startsAt, &endsAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	audience, err := parsePurchaseAudience(audienceText)
	if err != nil {
		return nil, err
	}
	ruleType, err := parsePromotionRuleType(ruleTypeText, nil)
	if err != nil {
		return nil, err
	}
	var valueUnits int64
	if ruleType == promotionRuleDiscount {
		discount, err := strconv.ParseFloat(strings.TrimSpace(ruleValueText), 64)
		if err != nil {
			return nil, fmt.Errorf("活动折扣格式错误: %w", err)
		}
		valueUnits = int64(math.Round(discount * 1000))
	} else {
		valueUnits, err = parseAmountToCents(ruleValueText)
		if err != nil {
			return nil, fmt.Errorf("活动金额格式错误: %w", err)
		}
	}
	amountCents, err := calculatePromotionAmountCents(originalCents, ruleType, valueUnits)
	if err != nil {
		return nil, err
	}
	ruleSnapshot, err := buildPromotionRuleSnapshot(ruleType, valueUnits, amountCents)
	if err != nil {
		return nil, err
	}
	candidate.Audience = audience
	candidate.RuleType = ruleType
	candidate.RuleValueUnits = valueUnits
	candidate.AmountCents = amountCents
	candidate.RuleSnapshot = ruleSnapshot
	candidate.StartsAt = startsAt.Format("2006-01-02 15:04")
	candidate.EndsAt = endsAt.Format("2006-01-02 15:04")
	return &candidate, nil
}

func quoteUserPurchase(db *sql.DB, plan purchasePlanPricing) (purchasePriceQuote, error) {
	promotion, err := loadActivePurchasePromotion(db, plan.AppID, plan.PlanID, plan.PriceCents, purchaseAudienceUser)
	if err != nil {
		return purchasePriceQuote{}, err
	}
	return calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceUser,
		OriginalCents: plan.PriceCents,
		Promotion:     promotion,
	})
}

func quoteAgentPurchase(db *sql.DB, plan purchasePlanPricing, discount float64) (purchasePriceQuote, error) {
	promotion, err := loadActivePurchasePromotion(db, plan.AppID, plan.PlanID, plan.PriceCents, purchaseAudienceAgent)
	if err != nil {
		return purchasePriceQuote{}, err
	}
	return calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceAgent,
		OriginalCents: plan.PriceCents,
		AgentDiscount: discount,
		Promotion:     promotion,
	})
}

func userPurchasePrice(plan purchasePlanPricing) (purchasePriceQuote, error) {
	return calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceUser,
		OriginalCents: plan.PriceCents,
	})
}

func agentPurchasePrice(plan purchasePlanPricing, discount float64) (purchasePriceQuote, error) {
	return calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceAgent,
		OriginalCents: plan.PriceCents,
		AgentDiscount: discount,
	})
}

func purchaseAmount(cents int64) float64 {
	return float64(cents) / 100
}

func newPurchaseOrderSnapshot(plan purchasePlanPricing, quote purchasePriceQuote) (purchaseOrderSnapshot, error) {
	var promotionRule json.RawMessage
	if quote.PromotionRule != "" {
		if !json.Valid([]byte(quote.PromotionRule)) {
			return purchaseOrderSnapshot{}, fmt.Errorf("活动规则快照不是有效 JSON")
		}
		promotionRule = json.RawMessage(quote.PromotionRule)
	}
	pricingJSON, err := json.Marshal(purchasePricingSnapshot{
		Version:           1,
		BuyerType:         quote.BuyerType,
		OriginalAmount:    formatCents(quote.OriginalCents),
		BaseAmount:        formatCents(quote.BaseCents),
		Amount:            formatCents(quote.AmountCents),
		DiscountAmount:    formatCents(quote.DiscountCents),
		AgentDiscount:     quote.AgentDiscount,
		PromotionID:       quote.PromotionID,
		PromotionName:     quote.PromotionName,
		PromotionAudience: quote.PromotionAudience,
		PromotionRule:     promotionRule,
	})
	if err != nil {
		return purchaseOrderSnapshot{}, err
	}

	snapshot := purchaseOrderSnapshot{
		OriginalAmount:  formatCents(quote.OriginalCents),
		BaseAmount:      formatCents(quote.BaseCents),
		Amount:          formatCents(quote.AmountCents),
		DiscountAmount:  formatCents(quote.DiscountCents),
		PromotionName:   quote.PromotionName,
		PricingSnapshot: string(pricingJSON),
		AppName:         plan.AppName,
		PlanName:        plan.PlanName,
		DurationDays:    plan.DurationDays,
		MaxSites:        plan.MaxSites,
	}
	if quote.PromotionID > 0 {
		snapshot.PromotionID = quote.PromotionID
	}
	if quote.PromotionRule != "" {
		snapshot.PromotionRule = quote.PromotionRule
	}
	return snapshot, nil
}
