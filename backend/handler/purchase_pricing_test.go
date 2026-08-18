package handler

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestParsePurchaseAudience(t *testing.T) {
	tests := []struct {
		input string
		want  purchaseAudience
	}{
		{input: "user", want: purchaseAudienceUser},
		{input: " AGENT ", want: purchaseAudienceAgent},
		{input: "all", want: purchaseAudienceAll},
	}
	for _, test := range tests {
		got, err := parsePurchaseAudience(test.input)
		if err != nil || got != test.want {
			t.Fatalf("parsePurchaseAudience(%q) = %q, %v; want %q", test.input, got, err, test.want)
		}
	}
	if _, err := parsePurchaseAudience("admin"); !errors.Is(err, errInvalidPurchaseAudience) {
		t.Fatalf("invalid audience error = %v", err)
	}
}

func TestPurchaseAudienceIncludes(t *testing.T) {
	tests := []struct {
		activity purchaseAudience
		buyer    purchaseAudience
		want     bool
	}{
		{activity: purchaseAudienceUser, buyer: purchaseAudienceUser, want: true},
		{activity: purchaseAudienceUser, buyer: purchaseAudienceAgent, want: false},
		{activity: purchaseAudienceAgent, buyer: purchaseAudienceAgent, want: true},
		{activity: purchaseAudienceAgent, buyer: purchaseAudienceUser, want: false},
		{activity: purchaseAudienceAll, buyer: purchaseAudienceUser, want: true},
		{activity: purchaseAudienceAll, buyer: purchaseAudienceAgent, want: true},
	}
	for _, test := range tests {
		if got := purchaseAudienceIncludes(test.activity, test.buyer); got != test.want {
			t.Fatalf("purchaseAudienceIncludes(%q, %q) = %v; want %v", test.activity, test.buyer, got, test.want)
		}
	}
}

func TestCalculatePurchasePriceByBuyer(t *testing.T) {
	userPromotion := &purchasePromotionCandidate{
		ID:          11,
		Name:        "用户活动",
		Audience:    purchaseAudienceUser,
		AmountCents: 6000,
	}
	userQuote, err := calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceUser,
		OriginalCents: 10000,
		Promotion:     userPromotion,
	})
	if err != nil {
		t.Fatal(err)
	}
	if userQuote.BaseCents != 10000 || userQuote.AmountCents != 6000 || userQuote.DiscountCents != 4000 || userQuote.PromotionID != 11 {
		t.Fatalf("unexpected user quote: %#v", userQuote)
	}

	agentQuote, err := calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceAgent,
		OriginalCents: 10000,
		AgentDiscount: 8,
		Promotion:     userPromotion,
	})
	if err != nil {
		t.Fatal(err)
	}
	if agentQuote.BaseCents != 8000 || agentQuote.AmountCents != 8000 || agentQuote.PromotionID != 0 {
		t.Fatalf("user-only promotion leaked to agent: %#v", agentQuote)
	}

	allPromotion := &purchasePromotionCandidate{
		ID:          12,
		Name:        "全员活动",
		Audience:    purchaseAudienceAll,
		AmountCents: 7000,
	}
	agentQuote, err = calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceAgent,
		OriginalCents: 10000,
		AgentDiscount: 8,
		Promotion:     allPromotion,
	})
	if err != nil {
		t.Fatal(err)
	}
	if agentQuote.BaseCents != 8000 || agentQuote.AmountCents != 7000 || agentQuote.DiscountCents != 1000 || agentQuote.PromotionID != 12 {
		t.Fatalf("unexpected all-audience agent quote: %#v", agentQuote)
	}
}

func TestAgentKeepsLowerLevelPriceWhenPromotionIsWorse(t *testing.T) {
	quote, err := calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceAgent,
		OriginalCents: 10000,
		AgentDiscount: 7,
		Promotion: &purchasePromotionCandidate{
			ID:          13,
			Name:        "普通活动",
			Audience:    purchaseAudienceAgent,
			AmountCents: 8000,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if quote.BaseCents != 7000 || quote.AmountCents != 7000 || quote.PromotionID != 0 {
		t.Fatalf("worse promotion replaced agent level price: %#v", quote)
	}
}

func TestUserKeepsLowerOriginalPriceAfterPlanPriceDrops(t *testing.T) {
	quote, err := calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceUser,
		OriginalCents: 5000,
		Promotion: &purchasePromotionCandidate{
			ID:          14,
			Name:        "旧活动价",
			Audience:    purchaseAudienceUser,
			AmountCents: 6000,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if quote.BaseCents != 5000 || quote.AmountCents != 5000 || quote.PromotionID != 0 {
		t.Fatalf("higher promotion replaced current plan price: %#v", quote)
	}
}

func TestPurchaseOrderSnapshot(t *testing.T) {
	plan := purchasePlanPricing{
		AppID:        9,
		PlanID:       3,
		AppName:      "Test App",
		PlanName:     "Annual",
		DurationDays: 365,
		PriceCents:   10000,
	}
	quote, err := calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceAgent,
		OriginalCents: plan.PriceCents,
		AgentDiscount: 8,
		Promotion: &purchasePromotionCandidate{
			ID:             15,
			Name:           "周年活动",
			Audience:       purchaseAudienceAll,
			RuleType:       promotionRuleReduction,
			RuleValueUnits: 3500,
			AmountCents:    6500,
			RuleSnapshot:   `{"type":"reduction","reductionAmount":"35.00","amount":"65.00","stacking":"lowest_only"}`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := newPurchaseOrderSnapshot(plan, quote)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.OriginalAmount != "100.00" || snapshot.BaseAmount != "80.00" || snapshot.Amount != "65.00" || snapshot.DiscountAmount != "15.00" {
		t.Fatalf("unexpected monetary snapshot: %#v", snapshot)
	}
	if snapshot.PromotionID != int64(15) || snapshot.PromotionName != "周年活动" || snapshot.AppName != "Test App" || snapshot.PlanName != "Annual" || snapshot.DurationDays != 365 {
		t.Fatalf("unexpected order snapshot: %#v", snapshot)
	}
	var pricing purchasePricingSnapshot
	if err := json.Unmarshal([]byte(snapshot.PricingSnapshot), &pricing); err != nil {
		t.Fatal(err)
	}
	if pricing.Version != 1 || pricing.BuyerType != purchaseAudienceAgent || pricing.PromotionID != 15 || pricing.PromotionName != "周年活动" || pricing.PromotionAudience != purchaseAudienceAll || pricing.Amount != "65.00" {
		t.Fatalf("unexpected pricing JSON: %#v", pricing)
	}
	if string(pricing.PromotionRule) != `{"type":"reduction","reductionAmount":"35.00","amount":"65.00","stacking":"lowest_only"}` {
		t.Fatalf("unexpected promotion rule snapshot: %s", pricing.PromotionRule)
	}
}

func TestAgentPromotionIsFixedPriceWithoutDoubleDiscount(t *testing.T) {
	quote, err := calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceAgent,
		OriginalCents: 10000,
		AgentDiscount: 5,
		Promotion: &purchasePromotionCandidate{
			ID:          21,
			Name:        "代理固定价",
			Audience:    purchaseAudienceAgent,
			AmountCents: 4000,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if quote.BaseCents != 5000 || quote.AmountCents != 4000 || quote.DiscountCents != 1000 || quote.PromotionID != 21 {
		t.Fatalf("promotion was stacked with agent discount: %#v", quote)
	}
}

func TestCalculatePromotionAmountByRule(t *testing.T) {
	tests := []struct {
		name       string
		ruleType   promotionRuleType
		valueUnits int64
		wantCents  int64
	}{
		{name: "八折", ruleType: promotionRuleDiscount, valueUnits: 8000, wantCents: 8000},
		{name: "立减二十五元", ruleType: promotionRuleReduction, valueUnits: 2500, wantCents: 7500},
		{name: "立减不低于零", ruleType: promotionRuleReduction, valueUnits: 12000, wantCents: 0},
		{name: "固定活动价", ruleType: promotionRuleFixedPrice, valueUnits: 6500, wantCents: 6500},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := calculatePromotionAmountCents(10000, test.ruleType, test.valueUnits)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.wantCents {
				t.Fatalf("promotion amount = %d; want %d", got, test.wantCents)
			}
		})
	}
}

func TestPromotionRulesUseCurrentPlanPriceAfterPriceChanges(t *testing.T) {
	tests := []struct {
		name            string
		ruleType        promotionRuleType
		valueUnits      int64
		wantRuleCents   int64
		wantAmountCents int64
		wantPromotionID int64
	}{
		{name: "discount recalculates from lower price", ruleType: promotionRuleDiscount, valueUnits: 8000, wantRuleCents: 4000, wantAmountCents: 4000, wantPromotionID: 50},
		{name: "reduction is floored at zero", ruleType: promotionRuleReduction, valueUnits: 6000, wantRuleCents: 0, wantAmountCents: 0, wantPromotionID: 50},
		{name: "higher legacy fixed price loses", ruleType: promotionRuleFixedPrice, valueUnits: 6000, wantRuleCents: 6000, wantAmountCents: 5000},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ruleAmount, err := calculatePromotionAmountCents(5000, test.ruleType, test.valueUnits)
			if err != nil {
				t.Fatal(err)
			}
			if ruleAmount != test.wantRuleCents {
				t.Fatalf("rule amount = %d; want %d", ruleAmount, test.wantRuleCents)
			}
			quote, err := calculatePurchasePrice(purchasePricingInput{
				BuyerType:     purchaseAudienceUser,
				OriginalCents: 5000,
				Promotion: &purchasePromotionCandidate{
					ID:          50,
					Name:        "原价变更活动",
					Audience:    purchaseAudienceUser,
					AmountCents: ruleAmount,
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if quote.AmountCents != test.wantAmountCents || quote.PromotionID != test.wantPromotionID {
				t.Fatalf("unexpected quote after price change: %#v", quote)
			}
		})
	}
}

func TestAgentComparesPromotionRulePriceWithoutStacking(t *testing.T) {
	discountAmount, err := calculatePromotionAmountCents(10000, promotionRuleDiscount, 5000)
	if err != nil {
		t.Fatal(err)
	}
	quote, err := calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceAgent,
		OriginalCents: 10000,
		AgentDiscount: 8,
		Promotion: &purchasePromotionCandidate{
			ID:          23,
			Name:        "五折活动",
			Audience:    purchaseAudienceAgent,
			AmountCents: discountAmount,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if quote.BaseCents != 8000 || quote.AmountCents != 5000 || quote.DiscountCents != 3000 {
		t.Fatalf("discount promotion was stacked with agent price: %#v", quote)
	}

	reductionAmount, err := calculatePromotionAmountCents(10000, promotionRuleReduction, 2000)
	if err != nil {
		t.Fatal(err)
	}
	quote, err = calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceAgent,
		OriginalCents: 10000,
		AgentDiscount: 6,
		Promotion: &purchasePromotionCandidate{
			ID:          24,
			Name:        "立减活动",
			Audience:    purchaseAudienceAgent,
			AmountCents: reductionAmount,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if quote.BaseCents != 6000 || quote.AmountCents != 6000 || quote.PromotionID != 0 {
		t.Fatalf("higher reduction price replaced agent price: %#v", quote)
	}
}

func TestLosingPromotionIsExcludedFromOrderSnapshot(t *testing.T) {
	plan := purchasePlanPricing{
		AppID:        9,
		PlanID:       3,
		AppName:      "Test App",
		PlanName:     "Annual",
		DurationDays: 365,
		PriceCents:   10000,
	}
	quote, err := calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceAgent,
		OriginalCents: plan.PriceCents,
		AgentDiscount: 6,
		Promotion: &purchasePromotionCandidate{
			ID:           22,
			Name:         "未胜出活动",
			Audience:     purchaseAudienceAll,
			AmountCents:  7000,
			RuleSnapshot: `{"type":"fixed_price","amount":"70.00","stacking":"lowest_only"}`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := newPurchaseOrderSnapshot(plan, quote)
	if err != nil {
		t.Fatal(err)
	}
	if quote.AmountCents != 6000 || snapshot.PromotionID != nil || snapshot.PromotionName != "" || snapshot.PromotionRule != nil {
		t.Fatalf("losing promotion leaked into order snapshot: quote=%#v snapshot=%#v", quote, snapshot)
	}
	var pricing purchasePricingSnapshot
	if err := json.Unmarshal([]byte(snapshot.PricingSnapshot), &pricing); err != nil {
		t.Fatal(err)
	}
	if pricing.PromotionID != 0 || pricing.PromotionName != "" || pricing.PromotionAudience != "" || len(pricing.PromotionRule) != 0 {
		t.Fatalf("losing promotion leaked into pricing JSON: %#v", pricing)
	}
}

func TestQuotePurchaseLoadsOnlyApplicableActivePromotion(t *testing.T) {
	state := &purchaseLicenseTypeTestState{
		promotion: &purchasePromotionCandidate{
			ID:          31,
			Name:        "用户活动",
			Audience:    purchaseAudienceUser,
			AmountCents: 6000,
		},
	}
	db := openPurchaseLicenseTypeTestDB(t, state)
	plan := purchasePlanPricing{AppID: 9, PlanID: 3, PriceCents: 10000}

	userQuote, err := quoteUserPurchase(db, plan)
	if err != nil {
		t.Fatal(err)
	}
	if userQuote.AmountCents != 6000 || userQuote.PromotionID != 31 || userQuote.PromotionAudience != purchaseAudienceUser {
		t.Fatalf("user promotion was not applied: %#v", userQuote)
	}

	agentQuote, err := quoteAgentPurchase(db, plan, 8)
	if err != nil {
		t.Fatal(err)
	}
	if agentQuote.AmountCents != 8000 || agentQuote.PromotionID != 0 {
		t.Fatalf("user-only promotion leaked to agent quote: %#v", agentQuote)
	}

	state.promotion = &purchasePromotionCandidate{
		ID:          32,
		Name:        "全员活动",
		Audience:    purchaseAudienceAll,
		AmountCents: 7000,
	}
	agentQuote, err = quoteAgentPurchase(db, plan, 8)
	if err != nil {
		t.Fatal(err)
	}
	if agentQuote.BaseCents != 8000 || agentQuote.AmountCents != 7000 || agentQuote.PromotionID != 32 || agentQuote.PromotionAudience != purchaseAudienceAll {
		t.Fatalf("all-audience promotion was not applied to agent: %#v", agentQuote)
	}
	var rule struct {
		Type     string `json:"type"`
		Amount   string `json:"amount"`
		Stacking string `json:"stacking"`
	}
	if err := json.Unmarshal([]byte(agentQuote.PromotionRule), &rule); err != nil || rule.Type != "fixed_price" || rule.Amount != "70.00" || rule.Stacking != "lowest_only" {
		t.Fatalf("unexpected promotion rule: %#v, error=%v", rule, err)
	}

	state.promotion = nil
	userQuote, err = quoteUserPurchase(db, plan)
	if err != nil {
		t.Fatal(err)
	}
	if userQuote.AmountCents != 10000 || userQuote.PromotionID != 0 {
		t.Fatalf("missing promotion changed user price: %#v", userQuote)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.queries) != 4 || len(state.queryArgs) != 4 {
		t.Fatalf("promotion query count=%d args=%d", len(state.queries), len(state.queryArgs))
	}
	wantAudiences := []string{"user", "agent", "agent", "user"}
	for index, query := range state.queries {
		for _, predicate := range []string{
			"CASE",
			"pcp.promotion_price <> 0",
			"pc.app_id = ? AND pcp.plan_id = ?",
			"pc.enabled = 1",
			"pc.starts_at <= NOW() AND pc.ends_at > NOW()",
			"pc.audience IN (?, 'all')",
		} {
			if !strings.Contains(query, predicate) {
				t.Fatalf("active promotion query missing %q: %s", predicate, query)
			}
		}
		args := state.queryArgs[index]
		if len(args) != 3 || args[0].Value != int64(9) || args[1].Value != int64(3) || args[2].Value != wantAudiences[index] {
			t.Fatalf("unexpected promotion query args: %#v", args)
		}
	}
}

func TestQuotePurchaseSupportsAllRulesAndSelectedPlans(t *testing.T) {
	state := &purchaseLicenseTypeTestState{
		promotionPlanIDs: map[int64]bool{3: true},
		promotion: &purchasePromotionCandidate{
			ID:             40,
			Name:           "套餐活动",
			Audience:       purchaseAudienceAll,
			RuleType:       promotionRuleDiscount,
			RuleValueUnits: 7500,
		},
	}
	db := openPurchaseLicenseTypeTestDB(t, state)
	selectedPlan := purchasePlanPricing{AppID: 9, PlanID: 3, PriceCents: 10000}
	unselectedPlan := purchasePlanPricing{AppID: 9, PlanID: 4, PriceCents: 10000}

	tests := []struct {
		name       string
		ruleType   promotionRuleType
		valueUnits int64
		wantCents  int64
	}{
		{name: "折扣", ruleType: promotionRuleDiscount, valueUnits: 7500, wantCents: 7500},
		{name: "立减", ruleType: promotionRuleReduction, valueUnits: 3000, wantCents: 7000},
		{name: "固定活动价", ruleType: promotionRuleFixedPrice, valueUnits: 6500, wantCents: 6500},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state.promotion.RuleType = test.ruleType
			state.promotion.RuleValueUnits = test.valueUnits
			quote, err := quoteUserPurchase(db, selectedPlan)
			if err != nil {
				t.Fatal(err)
			}
			if quote.AmountCents != test.wantCents || quote.PromotionID != 40 {
				t.Fatalf("unexpected selected-plan quote: %#v", quote)
			}
			var rule struct {
				Type            promotionRuleType `json:"type"`
				Discount        float64           `json:"discount"`
				ReductionAmount string            `json:"reductionAmount"`
				FixedAmount     string            `json:"fixedAmount"`
				Amount          string            `json:"amount"`
				Stacking        string            `json:"stacking"`
			}
			if err := json.Unmarshal([]byte(quote.PromotionRule), &rule); err != nil || rule.Type != test.ruleType || rule.Amount != formatCents(test.wantCents) || rule.Stacking != "lowest_only" {
				t.Fatalf("unexpected rule snapshot: %#v, error=%v", rule, err)
			}
			switch test.ruleType {
			case promotionRuleDiscount:
				if rule.Discount != 7.5 {
					t.Fatalf("discount snapshot = %#v", rule)
				}
			case promotionRuleReduction:
				if rule.ReductionAmount != "30.00" {
					t.Fatalf("reduction snapshot = %#v", rule)
				}
			case promotionRuleFixedPrice:
				if rule.FixedAmount != "65.00" {
					t.Fatalf("fixed-price snapshot = %#v", rule)
				}
			}
		})
	}

	quote, err := quoteUserPurchase(db, unselectedPlan)
	if err != nil {
		t.Fatal(err)
	}
	if quote.AmountCents != 10000 || quote.PromotionID != 0 {
		t.Fatalf("unselected plan received promotion: %#v", quote)
	}
}
