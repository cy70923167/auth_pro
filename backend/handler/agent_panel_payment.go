package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type payOption struct {
	Code    string `json:"code"`
	Channel string `json:"channel,omitempty"`
	PayType string `json:"payType,omitempty"`
	Label   string `json:"label"`
	Icon    string `json:"icon"`
	Color   string `json:"color"`
}

const (
	payChannelEpayV1 = "easypay"
	payChannelEpayV2 = "easypay-v2"
)

type onlinePaySelection struct {
	Channel string
	PayType string
}

func parseOnlinePaySelection(value string) (onlinePaySelection, bool) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	channel := ""
	payTypeText := parts[0]
	if len(parts) == 2 {
		channel = strings.TrimSpace(parts[0])
		payTypeText = parts[1]
		if channel != payChannelEpayV1 && channel != payChannelEpayV2 {
			return onlinePaySelection{}, false
		}
	}
	payType, ok := normalizeEpayPayType(payTypeText, "")
	if !ok {
		return onlinePaySelection{}, false
	}
	return onlinePaySelection{Channel: channel, PayType: payType}, true
}

func discountedAgentPrice(price, discount float64) float64 {
	quote, err := calculatePurchasePrice(purchasePricingInput{
		BuyerType:     purchaseAudienceAgent,
		OriginalCents: floatAmountToCents(price),
		AgentDiscount: discount,
	})
	if err != nil {
		return 0
	}
	return purchaseAmount(quote.AmountCents)
}

// AgentPanelPurchasePayOptions 返回代理端开通授权时当前应用可用的支付方式。
// 余额支付固定可用；配额支付仅当代理商对当前应用还有剩余配额时出现；
// 支付宝/微信/QQ 等线上方式按管理端支付配置中已开启的方式返回。
func AgentPanelPurchasePayOptions(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}

	appID, _ := strconv.ParseInt(strings.TrimSpace(c.Query("appId")), 10, 64)

	db := agentPanelDB(c)
	if db == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	options := []payOption{{Code: "balance", Label: "余额支付", Icon: "ri:wallet-3-line", Color: "#2e7d32"}}

	quota := gin.H{"total": 0, "used": 0, "remain": 0}
	if appID > 0 {
		if err := ensureAgentQuotaSchema(db); err == nil {
			var total, used int
			err = db.QueryRow(`
				SELECT total, used FROM agent_quotas WHERE agent_id = ? AND app_id = ?
			`, agentID, appID).Scan(&total, &used)
			if err == nil {
				remain := total - used
				if remain < 0 {
					remain = 0
				}
				quota = gin.H{"total": total, "used": used, "remain": remain}
				if remain > 0 {
					options = append([]payOption{{Code: "quota", Label: "配额支付", Icon: "ri:stack-line", Color: "#7c3aed"}}, options...)
				}
			}
		}
	}

	options = append(options, configuredOnlinePayOptions(db)...)
	options = dedupePayOptions(options)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"options": options,
			"quota":   quota,
		},
	})
}

func epayPayTypeOptions(channel string, payTypes []string) []payOption {
	type meta struct {
		label string
		icon  string
		color string
	}
	metas := map[string]meta{
		"alipay": {"支付宝", "ri:alipay-fill", "#1677ff"},
		"wxpay":  {"微信", "ri:wechat-pay-fill", "#07c160"},
		"qqpay":  {"QQ支付", "ri:qq-fill", "#12b7f5"},
	}
	var out []payOption
	for _, payType := range payTypes {
		normalized, ok := normalizeEpayPayType(payType, "")
		if !ok {
			continue
		}
		m, ok := metas[normalized]
		if !ok {
			continue
		}
		out = append(out, payOption{
			Code:    channel + ":" + normalized,
			Channel: channel,
			PayType: normalized,
			Label:   m.label,
			Icon:    m.icon,
			Color:   m.color,
		})
	}
	return out
}

func dedupePayOptions(options []payOption) []payOption {
	seen := map[string]bool{}
	out := options[:0]
	for _, option := range options {
		key := option.Code
		if option.PayType != "" {
			key = "online:" + option.PayType
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, option)
	}
	return out
}

func configuredOnlinePayOptions(db *sql.DB) []payOption {
	options := []payOption{}
	if cfg, err := loadEpayConfig(db); err == nil && cfg.validateForPay() == nil {
		options = append(options, epayPayTypeOptions(payChannelEpayV1, cfg.PayTypes)...)
	}
	if cfg, err := loadEpayV2Config(db); err == nil && cfg.validateForPay() == nil {
		options = append(options, epayPayTypeOptions(payChannelEpayV2, cfg.PayTypes)...)
	}
	return dedupePayOptions(options)
}

// UserPurchasePayOptions 返回用户购买授权时可用的支付方式。
func UserPurchasePayOptions(c *gin.Context) {
	if _, ok := getUserPanelID(c); !ok {
		return
	}

	db, err := openAppPurchaseDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	options := []payOption{{Code: "balance", Label: "余额支付", Icon: "ri:wallet-3-line", Color: "#2e7d32"}}
	options = append(options, configuredOnlinePayOptions(db)...)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{"options": dedupePayOptions(options)},
	})
}

// UserPurchaseOrderStatus 查询当前用户的线上授权购买订单。
func UserPurchaseOrderStatus(c *gin.Context) {
	userID, ok := getUserPanelID(c)
	if !ok {
		return
	}

	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "订单号不能为空"})
		return
	}

	db, err := openAppPurchaseDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureLicensePurchaseOrderSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化购买订单表失败"})
		return
	}

	var status, licenseNo, payMethod, amount, appName, planName string
	var licenseID, appID, planID int64
	var durationDays int
	err = db.QueryRow(`
		SELECT status, COALESCE(license_id, 0), COALESCE(license_no, ''),
		       COALESCE(pay_method, ''), amount, app_id, plan_id,
		       COALESCE(app_name_snapshot, ''), COALESCE(plan_name_snapshot, ''),
		       COALESCE(duration_days_snapshot, 0)
		FROM license_purchase_orders
		WHERE order_no = ? AND agent_id = 0 AND user_id = ?
	`, orderNo, userID).Scan(&status, &licenseID, &licenseNo, &payMethod, &amount, &appID, &planID,
		&appName, &planName, &durationDays)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "订单不存在"})
		return
	}

	if appName == "" || planName == "" {
		_ = db.QueryRow(`SELECT COALESCE(app_name, '') FROM apps WHERE id = ?`, appID).Scan(&appName)
		_ = db.QueryRow(`SELECT COALESCE(name, ''), COALESCE(duration_days, 0) FROM license_plans WHERE id = ?`, planID).Scan(&planName, &durationDays)
	}

	var balance float64
	_ = db.QueryRow(`SELECT balance FROM users WHERE id = ?`, userID).Scan(&balance)
	cost, _ := strconv.ParseFloat(amount, 64)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"orderNo":      orderNo,
			"status":       status,
			"licenseId":    licenseID,
			"licenseNo":    licenseNo,
			"payMethod":    payMethod,
			"appName":      appName,
			"planName":     planName,
			"durationDays": durationDays,
			"cost":         cost,
			"newBalance":   balance,
		},
	})
}

// AgentPanelPurchaseOrderStatus 代理端查询购买订单状态（轮询支付结果）。
func AgentPanelPurchaseOrderStatus(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}

	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "订单号不能为空"})
		return
	}

	db := agentPanelDB(c)
	if db == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureLicensePurchaseOrderSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化购买订单表失败"})
		return
	}

	var status, licenseNo, payMethod, amount, appName, planName string
	var licenseID, appID, planID int64
	var durationDays int
	err := db.QueryRow(`
		SELECT status, COALESCE(license_id, 0), COALESCE(license_no, ''),
		       COALESCE(pay_method, ''), amount, app_id, plan_id,
		       COALESCE(app_name_snapshot, ''), COALESCE(plan_name_snapshot, ''),
		       COALESCE(duration_days_snapshot, 0)
		FROM license_purchase_orders
		WHERE order_no = ? AND agent_id = ?
	`, orderNo, agentID).Scan(&status, &licenseID, &licenseNo, &payMethod, &amount, &appID, &planID,
		&appName, &planName, &durationDays)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "订单不存在"})
		return
	}

	if appName == "" || planName == "" {
		_ = db.QueryRow(`SELECT COALESCE(app_name, '') FROM apps WHERE id = ?`, appID).Scan(&appName)
		_ = db.QueryRow(`SELECT COALESCE(name, ''), COALESCE(duration_days, 0) FROM license_plans WHERE id = ?`, planID).Scan(&planName, &durationDays)
	}

	var balance float64
	_ = db.QueryRow(`SELECT balance FROM agents WHERE id = ?`, agentID).Scan(&balance)

	cost, _ := strconv.ParseFloat(amount, 64)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"orderNo":      orderNo,
			"status":       status,
			"licenseId":    licenseID,
			"licenseNo":    licenseNo,
			"payMethod":    payMethod,
			"appName":      appName,
			"planName":     planName,
			"durationDays": durationDays,
			"cost":         cost,
			"newBalance":   balance,
		},
	})
}

// userPurchaseOnline 用户线上支付购买授权：创建待支付订单，支付成功后生成授权。
func userPurchaseOnline(c *gin.Context, appID int64, planID int64, licenseType string, domain string, payType string, userID uint) {
	db, err := openAppPurchaseDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureAppPurchaseLicenseTypes(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化应用授权方式失败"})
		return
	}
	if err := ensurePlanLicenseType(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化套餐授权方式失败"})
		return
	}
	if err := ensureLicensePurchaseOrderSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化购买订单表失败: " + err.Error()})
		return
	}
	if err := ensurePurchasePromotionSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化促销活动失败"})
		return
	}

	plan, err := loadPurchasePlanPricing(db, appID, planID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "套餐不存在、已禁用或应用已下架"})
		return
	}
	quote, err := quoteUserPurchase(db, plan)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "计算购买价格失败"})
		return
	}
	appName, planName := plan.AppName, plan.PlanName
	amountCents := quote.AmountCents
	if amountCents < minRechargeCents {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "订单金额不能低于 ¥0.01，请使用余额支付"})
		return
	}

	orderName := fmt.Sprintf("购买授权 %s - %s", appName, planName)
	orderNo := generateUserPurchaseOrderNo()
	ownerID := int64(userID)
	selection, ok := parseOnlinePaySelection(payType)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "不支持的支付方式"})
		return
	}
	payType = selection.PayType

	payConfig, err := loadEpayConfig(db)
	if (selection.Channel == "" || selection.Channel == payChannelEpayV1) && err == nil && payConfig.validateForPay() == nil && payConfig.isPayTypeEnabled(payType) {
		payURL, frontendReturnURL, err := buildEpaySubmitURL(c, payConfig, orderNo, amountCents, payType, orderName, "/user/purchase")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		if err := insertAllowedLicensePurchaseOrder(db, orderNo, 0, "user", ownerID, licenseType, domain, plan, quote, payChannelEpayV1, payType, frontendReturnURL); err != nil {
			if err == errPurchaseTypeNotAllowed {
				c.JSON(http.StatusOK, gin.H{"code": 400, "msg": purchaseLicenseTypeNotAllowedMessage(licenseType)})
			} else if err == sql.ErrNoRows {
				c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "应用不存在或已下架"})
			} else {
				c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建购买订单失败"})
			}
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "支付订单已创建，正在跳转收银台",
			"data": gin.H{"orderNo": orderNo, "amount": formatCents(amountCents), "payType": payType, "payUrl": payURL},
		})
		return
	}

	payConfigV2, err := loadEpayV2Config(db)
	if (selection.Channel == "" || selection.Channel == payChannelEpayV2) && err == nil && payConfigV2.validateForPay() == nil && payConfigV2.isPayTypeEnabled(payType) {
		frontendReturnURL := buildFrontendReturnURL(c, orderNo, "/user/purchase")
		if err := insertAllowedLicensePurchaseOrder(db, orderNo, 0, "user", ownerID, licenseType, domain, plan, quote, payChannelEpayV2, payType, frontendReturnURL); err != nil {
			if err == errPurchaseTypeNotAllowed {
				c.JSON(http.StatusOK, gin.H{"code": 400, "msg": purchaseLicenseTypeNotAllowedMessage(licenseType)})
			} else if err == sql.ErrNoRows {
				c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "应用不存在或已下架"})
			} else {
				c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建购买订单失败"})
			}
			return
		}
		payURL, _, err := buildEpayV2Payment(c, payConfigV2, orderNo, amountCents, payType, orderName, "/user/purchase")
		if err != nil {
			_, _ = db.Exec(`UPDATE license_purchase_orders SET status = 'failed', remark = ? WHERE order_no = ? AND status = 'pending'`, "支付网关下单失败: "+err.Error(), orderNo)
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "支付订单已创建，正在跳转收银台",
			"data": gin.H{"orderNo": orderNo, "amount": formatCents(amountCents), "payType": payType, "payUrl": payURL},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该支付方式未开启"})
}

// agentPanelPurchaseOnline 代理端线上支付购买授权：创建待支付订单并返回收银台地址。
// 支付成功后由 settleLicensePurchaseOrder 统一生成授权。
func agentPanelPurchaseOnline(c *gin.Context, appID int64, planID int64, userID int64, licenseType string, domain string, payType string, agentID uint) {
	db, err := openAppPurchaseDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureAppPurchaseLicenseTypes(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化应用授权方式失败"})
		return
	}
	if err := ensurePlanLicenseType(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化套餐授权方式失败"})
		return
	}
	if err := ensureAgentPurchaseSchemas(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	if err := ensureLicensePurchaseOrderSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化购买订单表失败: " + err.Error()})
		return
	}

	plan, err := loadPurchasePlanPricing(db, appID, planID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "套餐不存在、已禁用或应用已下架"})
		return
	}

	discount, err := currentAgentDiscount(db, agentID)
	if err != nil || discount <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "代理商不存在或已禁用"})
		return
	}
	quote, err := quoteAgentPurchase(db, plan, discount)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "计算购买价格失败"})
		return
	}
	appName, planName := plan.AppName, plan.PlanName

	ownerType := "agent"
	ownerID := int64(agentID)
	if userID > 0 {
		var exists int
		err = db.QueryRow("SELECT COUNT(*) FROM users WHERE id = ? AND enabled = 1", userID).Scan(&exists)
		if err != nil || exists == 0 {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "目标用户不存在或已禁用"})
			return
		}
		ownerType = "user"
		ownerID = userID
	}

	amountCents := quote.AmountCents
	if amountCents < minRechargeCents {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "订单金额不能低于 ¥0.01，请使用余额或配额支付"})
		return
	}

	orderName := fmt.Sprintf("开通授权 %s - %s", appName, planName)
	orderNo := generateLicensePurchaseOrderNo()
	selection, ok := parseOnlinePaySelection(payType)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "不支持的支付方式"})
		return
	}
	payType = selection.PayType

	payConfig, err := loadEpayConfig(db)
	if (selection.Channel == "" || selection.Channel == payChannelEpayV1) && err == nil && payConfig.validateForPay() == nil && payConfig.isPayTypeEnabled(payType) {
		payURL, frontendReturnURL, err := buildEpaySubmitURL(c, payConfig, orderNo, amountCents, payType, orderName, "/agent/purchase")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		if err := insertAllowedLicensePurchaseOrder(db, orderNo, agentID, ownerType, ownerID, licenseType, domain, plan, quote, payChannelEpayV1, payType, frontendReturnURL); err != nil {
			if err == errPurchaseTypeNotAllowed {
				c.JSON(http.StatusOK, gin.H{"code": 400, "msg": purchaseLicenseTypeNotAllowedMessage(licenseType)})
			} else if err == sql.ErrNoRows {
				c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "应用不存在或已下架"})
			} else {
				c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建购买订单失败"})
			}
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "支付订单已创建，正在跳转收银台",
			"data": gin.H{"orderNo": orderNo, "amount": formatCents(amountCents), "payType": payType, "payUrl": payURL},
		})
		return
	}

	payConfigV2, err := loadEpayV2Config(db)
	if (selection.Channel == "" || selection.Channel == payChannelEpayV2) && err == nil && payConfigV2.validateForPay() == nil && payConfigV2.isPayTypeEnabled(payType) {
		frontendReturnURL := buildFrontendReturnURL(c, orderNo, "/agent/purchase")
		if err := insertAllowedLicensePurchaseOrder(db, orderNo, agentID, ownerType, ownerID, licenseType, domain, plan, quote, payChannelEpayV2, payType, frontendReturnURL); err != nil {
			if err == errPurchaseTypeNotAllowed {
				c.JSON(http.StatusOK, gin.H{"code": 400, "msg": purchaseLicenseTypeNotAllowedMessage(licenseType)})
			} else if err == sql.ErrNoRows {
				c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "应用不存在或已下架"})
			} else {
				c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建购买订单失败"})
			}
			return
		}
		payURL, _, err := buildEpayV2Payment(c, payConfigV2, orderNo, amountCents, payType, orderName, "/agent/purchase")
		if err != nil {
			_, _ = db.Exec(`UPDATE license_purchase_orders SET status = 'failed', remark = ? WHERE order_no = ? AND status = 'pending'`, "支付网关下单失败: "+err.Error(), orderNo)
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "支付订单已创建，正在跳转收银台",
			"data": gin.H{"orderNo": orderNo, "amount": formatCents(amountCents), "payType": payType, "payUrl": payURL},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该支付方式未开启"})
}

func insertAllowedLicensePurchaseOrder(db *sql.DB, orderNo string, agentID uint, ownerType string, ownerID int64, licenseType string, domain string, plan purchasePlanPricing, quote purchasePriceQuote, payChannel string, payType string, returnURL string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := requireAppPurchaseLicenseType(tx, plan.AppID, licenseType); err != nil {
		return err
	}

	snapshot, err := newPurchaseOrderSnapshot(plan, quote)
	if err != nil {
		return err
	}
	var uid interface{}
	if ownerType == "user" {
		uid = ownerID
	}
	orderRemark := "用户线上支付购买授权"
	if agentID > 0 {
		orderRemark = "代理商线上支付开通授权"
	}
	if _, err := tx.Exec(`
		INSERT INTO license_purchase_orders (
			order_no, agent_id, user_id, app_id, plan_id, owner_type, owner_id,
			type, target, amount, original_amount, base_amount, discount_amount,
			promotion_id, promotion_name, promotion_rule_snapshot, pricing_snapshot,
			app_name_snapshot, plan_name_snapshot, duration_days_snapshot,
			pay_channel, pay_method, status, return_url, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?)
	`, orderNo, agentID, uid, plan.AppID, plan.PlanID, ownerType, ownerID, licenseType, domain,
		snapshot.Amount, snapshot.OriginalAmount, snapshot.BaseAmount, snapshot.DiscountAmount,
		snapshot.PromotionID, snapshot.PromotionName, snapshot.PromotionRule, snapshot.PricingSnapshot,
		snapshot.AppName, snapshot.PlanName, snapshot.DurationDays,
		payChannel, payType, returnURL, orderRemark); err != nil {
		return err
	}
	return tx.Commit()
}

// settleLicensePurchaseOrder 支付回调验签通过后生成授权并标记订单已支付。
// 在事务内行锁订单，幂等：重复回调直接返回成功。
func settleLicensePurchaseOrder(db *sql.DB, orderNo string, paidCents int64, payChannel string, gatewayTradeNo string, payMethod string, notifyPayload string) error {
	payChannel = strings.TrimSpace(payChannel)
	gatewayTradeNo = strings.TrimSpace(gatewayTradeNo)
	payMethod = strings.TrimSpace(payMethod)
	if orderNo == "" || paidCents <= 0 || payChannel == "" || gatewayTradeNo == "" || payMethod == "" {
		return errors.New("购买订单支付回调参数不完整")
	}
	if err := ensureLicensePurchaseOrderSchema(db); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var (
		id               uint64
		agentID          int64
		ownerType        string
		ownerID          int64
		appID            int64
		planID           int64
		licenseType      string
		target           string
		amountText       string
		originalPrice    string
		status           string
		appNameSnapshot  sql.NullString
		planNameSnapshot sql.NullString
		durationSnapshot sql.NullInt64
		expectedChannel  string
		expectedMethod   string
		currentTradeNo   string
	)
	err = tx.QueryRow(`
		SELECT id, agent_id, owner_type, owner_id, app_id, plan_id, type, COALESCE(target, ''),
		       amount, COALESCE(original_amount, amount), status,
		       app_name_snapshot, plan_name_snapshot, duration_days_snapshot,
		       COALESCE(pay_channel, ''), COALESCE(pay_method, ''), COALESCE(gateway_trade_no, '')
		FROM license_purchase_orders
		WHERE order_no = ?
		FOR UPDATE
	`, orderNo).Scan(&id, &agentID, &ownerType, &ownerID, &appID, &planID, &licenseType, &target,
		&amountText, &originalPrice, &status, &appNameSnapshot, &planNameSnapshot, &durationSnapshot,
		&expectedChannel, &expectedMethod, &currentTradeNo)
	if err != nil {
		return err
	}

	expectedCents, err := parseAmountToCents(amountText)
	if err != nil || expectedCents != paidCents {
		return fmt.Errorf("购买订单金额不一致")
	}
	if expectedChannel != "" && expectedChannel != payChannel {
		return fmt.Errorf("购买订单支付通道不一致")
	}
	if expectedMethod != "" && expectedMethod != payMethod {
		return fmt.Errorf("购买订单支付方式不一致")
	}
	if currentTradeNo != "" && currentTradeNo != gatewayTradeNo {
		return fmt.Errorf("购买订单支付流水不一致")
	}
	if status == "paid" {
		return tx.Commit()
	}
	if status != "pending" {
		return fmt.Errorf("购买订单状态不可处理")
	}

	appName, planName := appNameSnapshot.String, planNameSnapshot.String
	durationDays := int(durationSnapshot.Int64)
	if !appNameSnapshot.Valid || appNameSnapshot.String == "" || !planNameSnapshot.Valid || planNameSnapshot.String == "" || !durationSnapshot.Valid {
		err = tx.QueryRow(`
			SELECT a.app_name, p.name, p.duration_days
			FROM license_plans p
			JOIN apps a ON a.id = p.app_id
			WHERE p.id = ? AND p.app_id = ?
		`, planID, appID).Scan(&appName, &planName, &durationDays)
		if err != nil {
			return fmt.Errorf("订单缺少套餐快照且套餐已不存在，无法生成授权")
		}
	}

	licenseNo := fmt.Sprintf("LIC%d%06d", time.Now().Unix(), time.Now().Nanosecond()%1000000)
	now := time.Now()
	var expiredAt *time.Time
	if durationDays > 0 {
		t := now.AddDate(0, 0, durationDays)
		expiredAt = &t
	}

	var licenseKey string
	if licenseType == "key" {
		licenseKey, err = generateRandomLicenseKey()
		if err != nil {
			return fmt.Errorf("生成密钥失败: %w", err)
		}
	}

	licenseSource := "agent"
	var issuedBy interface{} = agentID
	licenseRemark := fmt.Sprintf("代理商线上支付开通 %s - %s", appName, planName)
	if agentID <= 0 {
		licenseSource = "user_purchase"
		issuedBy = nil
		licenseRemark = fmt.Sprintf("用户线上支付购买 %s - %s", appName, planName)
	}
	licenseResult, err := tx.Exec(`
		INSERT INTO licenses (license_no, app_id, plan_id, original_price, type, status, source, owner_type, owner_id, issued_by,
		                      duration_days, started_at, expired_at, license_key, max_domains, remark)
		VALUES (?, ?, ?, ?, ?, 'active', ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
		licenseNo, appID, planID, originalPrice, licenseType, licenseSource, ownerType, ownerID, issuedBy,
		durationDays, now, expiredAt, licenseKey, licenseRemark)
	if err != nil {
		return err
	}
	licenseID, _ := licenseResult.LastInsertId()

	if licenseType != "key" && target != "" {
		isWildcard := 0
		if licenseType == "wildcard" {
			isWildcard = 1
		}
		if _, err := tx.Exec("INSERT INTO license_domains (license_id, domain, is_wildcard) VALUES (?, ?, ?)", licenseID, target, isWildcard); err != nil {
			return err
		}
	}

	_, err = tx.Exec(`
		UPDATE license_purchase_orders
		SET status = 'paid', paid_at = NOW(), paid_amount = ?, gateway_trade_no = ?, pay_method = ?, license_id = ?, license_no = ?, notify_payload = ?
		WHERE id = ?
	`, formatCents(paidCents), gatewayTradeNo, payMethod, licenseID, licenseNo, notifyPayload, id)
	if err != nil {
		return err
	}

	// 线上支付购买授权计入付款主体的收支明细，不改变账户余额。
	subjectType, subjectID := "agent", agentID
	if agentID <= 0 {
		subjectType, subjectID = ownerType, ownerID
	}
	txNo := fmt.Sprintf("TX%d%06d", time.Now().Unix(), time.Now().Nanosecond()%1000000)
	_, err = tx.Exec(`
		INSERT INTO transactions (tx_no, subject_type, subject_id, type, amount, balance_after, ref_type, ref_id, remark)
		VALUES (?, ?, ?, 'consume', ?, NULL, 'license_purchase_order', ?, ?)`,
		txNo, subjectType, subjectID, -float64(paidCents)/100, id,
		fmt.Sprintf("%s支付购买 %s - %s 授权", payMethodLabel(payMethod), appName, planName))
	if err != nil {
		return fmt.Errorf("记录流水失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	queuePurchaseSuccessMail(ownerType, ownerID, licenseID)
	return nil
}

// ensureLicensePurchaseOrderSchema 代理商线上支付购买授权订单表。
func ensureLicensePurchaseOrderSchema(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS license_purchase_orders (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
			order_no VARCHAR(64) NOT NULL COMMENT '订单号',
			agent_id BIGINT UNSIGNED NOT NULL COMMENT '代理商ID',
			user_id BIGINT UNSIGNED DEFAULT NULL COMMENT '归属用户ID',
			app_id BIGINT UNSIGNED NOT NULL COMMENT '应用ID',
			plan_id BIGINT UNSIGNED NOT NULL COMMENT '套餐ID',
			owner_type ENUM('agent','user') NOT NULL DEFAULT 'agent' COMMENT '授权归属类型',
			owner_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '授权归属ID',
			type VARCHAR(20) NOT NULL DEFAULT 'domain' COMMENT '授权类型',
			target VARCHAR(255) DEFAULT '' COMMENT '授权目标',
			amount DECIMAL(12,2) NOT NULL COMMENT '实际应付金额',
			original_amount DECIMAL(12,2) DEFAULT NULL COMMENT '套餐原价快照',
			base_amount DECIMAL(12,2) DEFAULT NULL COMMENT '促销前基础成交价快照',
			discount_amount DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '活动优惠金额快照',
			promotion_id BIGINT UNSIGNED DEFAULT NULL COMMENT '活动ID快照',
			promotion_name VARCHAR(100) NOT NULL DEFAULT '' COMMENT '活动名称快照',
			promotion_rule_snapshot JSON DEFAULT NULL COMMENT '活动规则快照',
			pricing_snapshot JSON DEFAULT NULL COMMENT '完整定价快照',
			app_name_snapshot VARCHAR(100) NOT NULL DEFAULT '' COMMENT '应用名称快照',
			plan_name_snapshot VARCHAR(100) NOT NULL DEFAULT '' COMMENT '套餐名称快照',
			duration_days_snapshot INT UNSIGNED DEFAULT NULL COMMENT '套餐时长快照',
			paid_amount DECIMAL(12,2) DEFAULT NULL COMMENT '实际支付金额',
			pay_channel VARCHAR(30) DEFAULT '' COMMENT '支付渠道',
			pay_method VARCHAR(30) DEFAULT '' COMMENT '支付方式',
			gateway_trade_no VARCHAR(100) DEFAULT '' COMMENT '支付网关交易号',
			return_url VARCHAR(500) DEFAULT '' COMMENT '支付完成前端返回地址',
			license_id BIGINT UNSIGNED DEFAULT NULL COMMENT '生成的授权ID',
			license_no VARCHAR(64) DEFAULT '' COMMENT '生成的授权编号',
			status ENUM('pending','paid','failed','cancelled') DEFAULT 'pending' COMMENT '状态',
			paid_at DATETIME DEFAULT NULL COMMENT '支付完成时间',
			remark VARCHAR(255) DEFAULT '' COMMENT '备注',
			notify_payload TEXT COMMENT '支付回调原始参数',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
			PRIMARY KEY (id),
			UNIQUE KEY uk_order_no (order_no),
			KEY idx_agent (agent_id, status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='授权购买订单表'
	`); err != nil {
		return err
	}
	if err := ensureColumn(db, "license_purchase_orders", "user_id",
		"ALTER TABLE license_purchase_orders ADD COLUMN user_id BIGINT UNSIGNED DEFAULT NULL COMMENT '归属用户ID' AFTER agent_id"); err != nil {
		return err
	}
	if err := ensureColumn(db, "license_purchase_orders", "original_amount",
		"ALTER TABLE license_purchase_orders ADD COLUMN original_amount DECIMAL(12,2) DEFAULT NULL COMMENT '套餐原价快照' AFTER amount"); err != nil {
		return err
	}
	columns := []struct {
		name string
		sql  string
	}{
		{"base_amount", "ALTER TABLE license_purchase_orders ADD COLUMN base_amount DECIMAL(12,2) DEFAULT NULL COMMENT '促销前基础成交价快照' AFTER original_amount"},
		{"discount_amount", "ALTER TABLE license_purchase_orders ADD COLUMN discount_amount DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '活动优惠金额快照' AFTER base_amount"},
		{"promotion_id", "ALTER TABLE license_purchase_orders ADD COLUMN promotion_id BIGINT UNSIGNED DEFAULT NULL COMMENT '活动ID快照' AFTER discount_amount"},
		{"promotion_name", "ALTER TABLE license_purchase_orders ADD COLUMN promotion_name VARCHAR(100) NOT NULL DEFAULT '' COMMENT '活动名称快照' AFTER promotion_id"},
		{"promotion_rule_snapshot", "ALTER TABLE license_purchase_orders ADD COLUMN promotion_rule_snapshot JSON DEFAULT NULL COMMENT '活动规则快照' AFTER promotion_name"},
		{"pricing_snapshot", "ALTER TABLE license_purchase_orders ADD COLUMN pricing_snapshot JSON DEFAULT NULL COMMENT '完整定价快照' AFTER promotion_rule_snapshot"},
		{"app_name_snapshot", "ALTER TABLE license_purchase_orders ADD COLUMN app_name_snapshot VARCHAR(100) NOT NULL DEFAULT '' COMMENT '应用名称快照' AFTER pricing_snapshot"},
		{"plan_name_snapshot", "ALTER TABLE license_purchase_orders ADD COLUMN plan_name_snapshot VARCHAR(100) NOT NULL DEFAULT '' COMMENT '套餐名称快照' AFTER app_name_snapshot"},
		{"duration_days_snapshot", "ALTER TABLE license_purchase_orders ADD COLUMN duration_days_snapshot INT UNSIGNED DEFAULT NULL COMMENT '套餐时长快照' AFTER plan_name_snapshot"},
	}
	for _, column := range columns {
		if err := ensureColumn(db, "license_purchase_orders", column.name, column.sql); err != nil {
			return err
		}
	}
	return ensureLicensePriceSnapshotSchema(db)
}

func ensureLicensePriceSnapshotSchema(db *sql.DB) error {
	return ensureColumn(db, "licenses", "original_price",
		"ALTER TABLE licenses ADD COLUMN original_price DECIMAL(12,2) DEFAULT NULL COMMENT '套餐原价快照' AFTER plan_id")
}

func EnsureLicensePurchasePriceSnapshotSchema(db *sql.DB) error {
	return ensureLicensePurchaseOrderSchema(db)
}

// ensureAgentQuotaSchema 配额表兜底建表（管理端配额功能正常使用时会已存在）。
func ensureAgentQuotaSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS agent_quotas (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
			agent_id BIGINT UNSIGNED NOT NULL COMMENT '代理商ID',
			app_id BIGINT UNSIGNED NOT NULL COMMENT '应用ID',
			total INT NOT NULL DEFAULT 0 COMMENT '配额总数',
			used INT NOT NULL DEFAULT 0 COMMENT '已使用数量',
			price DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '配额单价',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
			PRIMARY KEY (id),
			UNIQUE KEY uk_agent_app (agent_id, app_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理商配额表'
	`)
	return err
}

// generateLicensePurchaseOrderNo 授权购买订单号，代理端 LP 前缀、用户端 UP 前缀，与充值订单 UR 区分。
func generateLicensePurchaseOrderNo() string {
	return generateAgentPurchaseOrderNo()
}

func generateAgentPurchaseOrderNo() string {
	return fmt.Sprintf("LP%d%06d", time.Now().Unix(), time.Now().Nanosecond()%1_000_000)
}

func generateUserPurchaseOrderNo() string {
	return fmt.Sprintf("UP%d%06d", time.Now().Unix(), time.Now().Nanosecond()%1_000_000)
}

// payMethodLabel 支付方式码转中文名，用于流水备注。
func payMethodLabel(code string) string {
	switch code {
	case "alipay":
		return "支付宝"
	case "wxpay":
		return "微信"
	case "qqpay":
		return "QQ支付"
	case "balance":
		return "余额"
	default:
		return code
	}
}

// ensurePurchaseOrderUserIDColumn 老库的授权购买订单表可能缺 user_id 列，兜底补上。
func EnsurePurchaseOrderUserIDColumn(db *sql.DB) error {
	var cnt int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'license_purchase_orders' AND COLUMN_NAME = 'user_id'
	`).Scan(&cnt); err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}
	_, err := db.Exec("ALTER TABLE license_purchase_orders ADD COLUMN user_id BIGINT UNSIGNED DEFAULT NULL COMMENT '归属用户ID' AFTER agent_id")
	return err
}

// BackfillLicensePurchaseTransactions 修正并补齐历史线上购买订单流水与原价快照。
// 订单存在付款代理时，消费流水固定归付款代理；旧数据没有代理信息时保留原归属。
func BackfillLicensePurchaseTransactions(db *sql.DB) {
	if err := ensureLicensePurchaseOrderSchema(db); err != nil {
		fmt.Printf("ensure purchase transaction schema failed: %v\n", err)
		return
	}

	if _, err := db.Exec(`
		UPDATE license_purchase_orders o
		LEFT JOIN license_plans p ON p.id = o.plan_id
		SET o.original_amount = COALESCE(p.price, o.amount)
		WHERE o.original_amount IS NULL
	`); err != nil {
		fmt.Printf("backfill purchase original amount failed: %v\n", err)
	}
	if _, err := db.Exec(`
		UPDATE license_purchase_orders o
		LEFT JOIN apps a ON a.id = o.app_id
		LEFT JOIN license_plans p ON p.id = o.plan_id
		SET o.base_amount = COALESCE(o.base_amount, o.amount),
		    o.discount_amount = COALESCE(o.discount_amount, 0),
		    o.app_name_snapshot = CASE WHEN o.app_name_snapshot = '' THEN COALESCE(a.app_name, '') ELSE o.app_name_snapshot END,
		    o.plan_name_snapshot = CASE WHEN o.plan_name_snapshot = '' THEN COALESCE(p.name, '') ELSE o.plan_name_snapshot END,
		    o.duration_days_snapshot = COALESCE(o.duration_days_snapshot, p.duration_days)
		WHERE o.base_amount IS NULL
		   OR o.app_name_snapshot = ''
		   OR o.plan_name_snapshot = ''
		   OR o.duration_days_snapshot IS NULL
	`); err != nil {
		fmt.Printf("backfill purchase pricing snapshots failed: %v\n", err)
	}
	if _, err := db.Exec(`
		UPDATE licenses l
		JOIN license_purchase_orders o ON o.license_id = l.id
		SET l.original_price = COALESCE(o.original_amount, o.amount)
		WHERE l.original_price IS NULL
	`); err != nil {
		fmt.Printf("backfill license original price failed: %v\n", err)
	}
	if _, err := db.Exec(`
		UPDATE licenses l
		JOIN license_plans p ON p.id = l.plan_id
		SET l.original_price = p.price
		WHERE l.original_price IS NULL
		  AND l.source IN ('agent', 'user_purchase')
	`); err != nil {
		fmt.Printf("backfill legacy license original price failed: %v\n", err)
	}
	if _, err := db.Exec(`
		UPDATE transactions t
		JOIN license_purchase_orders o ON t.ref_type = 'license_purchase_order' AND t.ref_id = o.id
		SET t.subject_type = 'agent', t.subject_id = o.agent_id
		WHERE o.agent_id > 0
		  AND (t.subject_type <> 'agent' OR t.subject_id <> o.agent_id)
	`); err != nil {
		fmt.Printf("repair purchase transaction subjects failed: %v\n", err)
	}

	type orderRow struct {
		id, agentID, ownerID                   int64
		orderNo, ownerType, payMethod, appName string
		planName                               string
		paidAmount                             string
		paidAt                                 interface{}
	}
	rows, err := db.Query(`
		SELECT o.id, o.order_no, o.agent_id, o.owner_type, o.owner_id,
		       COALESCE(o.pay_method, ''), COALESCE(o.paid_amount, o.amount),
		       COALESCE(a.app_name, ''), COALESCE(p.name, ''), o.paid_at
		FROM license_purchase_orders o
		LEFT JOIN apps a ON a.id = o.app_id
		LEFT JOIN license_plans p ON p.id = o.plan_id
		WHERE o.status = 'paid'
		  AND NOT EXISTS (
			SELECT 1 FROM transactions t
			WHERE t.ref_type = 'license_purchase_order' AND t.ref_id = o.id
		)
	`)
	if err != nil {
		fmt.Printf("backfill purchase transactions query failed: %v\n", err)
		return
	}
	defer rows.Close()

	var list []orderRow
	for rows.Next() {
		var o orderRow
		if err := rows.Scan(&o.id, &o.orderNo, &o.agentID, &o.ownerType, &o.ownerID,
			&o.payMethod, &o.paidAmount, &o.appName, &o.planName, &o.paidAt); err != nil {
			continue
		}
		list = append(list, o)
	}
	for _, o := range list {
		cents, err := parseAmountToCents(o.paidAmount)
		if err != nil {
			continue
		}
		subjectType, subjectID := "agent", o.agentID
		if subjectID <= 0 {
			subjectType, subjectID = o.ownerType, o.ownerID
		}
		txNo := fmt.Sprintf("TX%d%06d", time.Now().UnixNano()/1e3, time.Now().Nanosecond()%1000000)
		remark := fmt.Sprintf("%s支付开通 %s - %s 授权", payMethodLabel(o.payMethod), o.appName, o.planName)
		if o.paidAt != nil {
			_, err = db.Exec(`
				INSERT INTO transactions (tx_no, subject_type, subject_id, type, amount, balance_after, ref_type, ref_id, remark, created_at)
				VALUES (?, ?, ?, 'consume', ?, NULL, 'license_purchase_order', ?, ?, ?)`,
				txNo, subjectType, subjectID, -float64(cents)/100, o.id, remark, o.paidAt)
		} else {
			_, err = db.Exec(`
				INSERT INTO transactions (tx_no, subject_type, subject_id, type, amount, balance_after, ref_type, ref_id, remark)
				VALUES (?, ?, ?, 'consume', ?, NULL, 'license_purchase_order', ?, ?)`,
				txNo, subjectType, subjectID, -float64(cents)/100, o.id, remark)
		}
		if err != nil {
			fmt.Printf("backfill purchase transaction order %s failed: %v\n", o.orderNo, err)
		}
	}
	if len(list) > 0 {
		fmt.Printf("backfilled %d license purchase transaction(s)\n", len(list))
	}
}
