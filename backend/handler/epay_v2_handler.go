package handler

import (
	"database/sql"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// UserRechargeV2Create 创建 V2 用户充值订单。
func UserRechargeV2Create(c *gin.Context) {
	userID, ok := getUserPanelID(c)
	if !ok {
		return
	}

	var req createUserRechargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "充值金额必须大于 0"})
		return
	}

	amountCents := floatAmountToCents(req.Amount)
	if amountCents < minRechargeCents {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "充值金额不能低于 ¥0.01"})
		return
	}
	if amountCents > maxRechargeCents {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "单次充值金额不能超过 ¥1000000.00"})
		return
	}

	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	if err := ensureRechargeOrderSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化充值订单表失败"})
		return
	}

	payConfig, err := loadEpayV2Config(db)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取支付配置失败"})
		return
	}
	if err := payConfig.validateForPay(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	payType, ok := normalizeEpayPayType(req.PayType, payConfig.resolveDefaultPayType())
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "不支持的支付方式"})
		return
	}
	if !payConfig.isPayTypeEnabled(payType) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该支付方式未开启"})
		return
	}

	orderNo := generateRechargeOrderNo()
	payURL, frontendReturnURL, err := buildEpayV2Payment(c, payConfig, orderNo, amountCents, payType, "用户余额充值", "/user/purchase")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	amount := formatCents(amountCents)
	_, err = db.Exec(`
		INSERT INTO recharge_orders (
			order_no, subject_type, subject_id, user_id, amount, pay_channel, pay_method, status, return_url, remark
		) VALUES (?, 'user', ?, ?, ?, 'easypay-v2', ?, 'pending', ?, ?)
	`, orderNo, userID, userID, amount, payType, frontendReturnURL, "用户余额充值")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建充值订单失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "充值订单已创建",
		"data": gin.H{
			"orderNo": orderNo,
			"amount":  amount,
			"payType": payType,
			"payUrl":  payURL,
		},
	})
}

// UserRechargeV2Options 用户端 V2 支付选项。
func UserRechargeV2Options(c *gin.Context) {
	if _, ok := getUserPanelID(c); !ok {
		return
	}

	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	payConfig, err := loadEpayV2Config(db)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取支付配置失败"})
		return
	}

	available := payConfig.validateForPay() == nil
	payTypes := payConfig.PayTypes
	if !available {
		payTypes = []string{}
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"easypayEnabled": available,
			"payTypes":       payTypes,
			"defaultType":    payConfig.resolveDefaultPayType(),
		},
	})
}

// AdminPaymentV2TestCreate 管理员发起 V2 测试订单，只记录结果不入账。
func AdminPaymentV2TestCreate(c *gin.Context) {
	if !requireAdminRole(c) {
		return
	}

	var req createPaymentTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "测试金额必须大于 0"})
		return
	}

	amountCents := floatAmountToCents(req.Amount)
	if amountCents < minRechargeCents {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "测试金额不能低于 ¥0.01"})
		return
	}
	if amountCents > maxPaymentTestCents {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "测试金额不能超过 ¥100.00"})
		return
	}

	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	if err := ensureRechargeOrderSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化充值订单表失败"})
		return
	}

	payConfig, err := loadEpayV2Config(db)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取支付配置失败"})
		return
	}
	if err := payConfig.validateForTest(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	payType, ok := normalizeEpayPayType(req.PayType, payConfig.resolveDefaultPayType())
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "不支持的支付方式"})
		return
	}
	if !payConfig.isPayTypeEnabled(payType) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该支付方式未开启"})
		return
	}

	orderNo := generatePaymentTestOrderNo()
	payURL, frontendReturnURL, err := buildEpayV2Payment(c, payConfig, orderNo, amountCents, payType, "易支付 V2 测试支付", "/system/epay-config")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	amount := formatCents(amountCents)
	_, err = db.Exec(`
		INSERT INTO recharge_orders (
			order_no, subject_type, subject_id, amount, pay_channel, pay_method, status, return_url, remark
		) VALUES (?, 'test', 0, ?, 'easypay-v2', ?, 'pending', ?, ?)
	`, orderNo, amount, payType, frontendReturnURL, "管理员支付测试(V2)")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建测试订单失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "测试订单已创建",
		"data": gin.H{
			"orderNo": orderNo,
			"amount":  amount,
			"payType": payType,
			"payUrl":  payURL,
		},
	})
}

// AdminPaymentV2TestStatus 查询 V2 测试订单状态。
func AdminPaymentV2TestStatus(c *gin.Context) {
	if !requireAdminRole(c) {
		return
	}
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "订单号不能为空"})
		return
	}

	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	if err := ensureRechargeOrderSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化充值订单表失败"})
		return
	}

	var amountText, payMethod, status, gatewayTradeNo string
	var paidAt sql.NullTime
	err = db.QueryRow(`
		SELECT amount, pay_method, status, gateway_trade_no, paid_at
		FROM recharge_orders
		WHERE order_no = ? AND subject_type = 'test'
	`, orderNo).Scan(&amountText, &payMethod, &status, &gatewayTradeNo, &paidAt)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "测试订单不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询测试订单失败"})
		return
	}

	paidAtText := ""
	if paidAt.Valid {
		paidAtText = paidAt.Time.Format("2006-01-02 15:04:05")
	}
	amount, _ := strconv.ParseFloat(amountText, 64)
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"orderNo":        orderNo,
			"amount":         amount,
			"payType":        payMethod,
			"status":         status,
			"gatewayTradeNo": gatewayTradeNo,
			"paidAt":         paidAtText,
		},
	})
}

// AdminPaymentV2Config 读取 V2 支付配置（密钥不回显明文）。
func AdminPaymentV2Config(c *gin.Context) {
	db, err := openSystemConfigDB()
	if err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	if err := ensureSystemConfigStorage(db); err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 500, "msg": "初始化系统配置失败"})
		return
	}
	cfg, err := loadEpayV2Config(db)
	if err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 500, "msg": "读取支付配置失败"})
		return
	}
	writeSystemConfig(c, http.StatusOK, gin.H{"code": 200, "msg": "", "data": paymentV2ConfigResponseFrom(cfg)})
}

func paymentV2ConfigResponseFrom(cfg epayV2Config) paymentV2ConfigResponse {
	payType, ok := normalizeEpayPayType(cfg.DefaultPayType, epayDefaultPayType)
	if !ok {
		payType = epayDefaultPayType
	}
	return paymentV2ConfigResponse{
		EasypayEnabled:        cfg.Enabled,
		EasypayGateway:        cfg.Gateway,
		EasypayPid:            cfg.PID,
		EasypayMerchantKeySet: cfg.MerchantKeySet,
		EasypayPlatformKeySet: cfg.PlatformKeySet,
		EasypayDefaultType:    payType,
		EasypayPayTypes:       cfg.PayTypes,
		EasypayNotifyUrl:      cfg.NotifyURL,
		EasypayReturnUrl:      cfg.ReturnURL,
	}
}

type paymentV2ConfigResponse struct {
	EasypayEnabled        bool     `json:"easypayEnabled"`
	EasypayGateway        string   `json:"easypayGateway"`
	EasypayPid            string   `json:"easypayPid"`
	EasypayMerchantKey    string   `json:"easypayMerchantKey,omitempty"`
	EasypayMerchantKeySet bool     `json:"easypayMerchantKeySet"`
	EasypayPlatformKey    string   `json:"easypayPlatformKey,omitempty"`
	EasypayPlatformKeySet bool     `json:"easypayPlatformKeySet"`
	EasypayDefaultType    string   `json:"easypayDefaultType"`
	EasypayPayTypes       []string `json:"easypayPayTypes"`
	EasypayNotifyUrl      string   `json:"easypayNotifyUrl"`
	EasypayReturnUrl      string   `json:"easypayReturnUrl"`
}

type updatePaymentV2ConfigRequest struct {
	EasypayEnabled     bool     `json:"easypayEnabled"`
	EasypayGateway     string   `json:"easypayGateway"`
	EasypayPid         string   `json:"easypayPid"`
	EasypayMerchantKey string   `json:"easypayMerchantKey"`
	EasypayPlatformKey string   `json:"easypayPlatformKey"`
	EasypayDefaultType string   `json:"easypayDefaultType"`
	EasypayPayTypes    []string `json:"easypayPayTypes"`
	EasypayNotifyUrl   string   `json:"easypayNotifyUrl"`
	EasypayReturnUrl   string   `json:"easypayReturnUrl"`
}

// AdminPaymentV2ConfigUpdate 保存 V2 支付配置。
func AdminPaymentV2ConfigUpdate(c *gin.Context) {
	var req updatePaymentV2ConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	req.EasypayGateway = strings.TrimSpace(req.EasypayGateway)
	req.EasypayPid = strings.TrimSpace(req.EasypayPid)
	req.EasypayMerchantKey = strings.TrimSpace(req.EasypayMerchantKey)
	req.EasypayPlatformKey = strings.TrimSpace(req.EasypayPlatformKey)
	req.EasypayDefaultType = strings.TrimSpace(req.EasypayDefaultType)
	req.EasypayNotifyUrl = strings.TrimSpace(req.EasypayNotifyUrl)
	req.EasypayReturnUrl = strings.TrimSpace(req.EasypayReturnUrl)

	payType, ok := normalizeEpayPayType(req.EasypayDefaultType, epayDefaultPayType)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "默认支付方式不支持"})
		return
	}
	req.EasypayDefaultType = payType
	payTypes, err := normalizeEpayPayTypes(req.EasypayPayTypes)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if req.EasypayEnabled && len(payTypes) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "启用易支付 V2 前请至少开启一种支付方式"})
		return
	}
	if len(payTypes) > 0 && !slices.Contains(payTypes, req.EasypayDefaultType) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "默认支付方式必须是已开启的支付方式"})
		return
	}
	if req.EasypayGateway != "" {
		if _, err := resolveEpayV2Endpoint(req.EasypayGateway); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
			return
		}
	}
	if err := validateOptionalHTTPURL(req.EasypayNotifyUrl, "易支付 V2 异步通知地址"); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if err := validateOptionalHTTPURL(req.EasypayReturnUrl, "易支付 V2 同步跳转地址"); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	// 密钥只在填写时校验格式，避免误存坏值。
	if req.EasypayMerchantKey != "" {
		if _, err := parsePrivateKeyPEM(req.EasypayMerchantKey); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "商户私钥格式不正确：" + err.Error()})
			return
		}
	}
	if req.EasypayPlatformKey != "" {
		if _, err := parsePublicKeyPEM(req.EasypayPlatformKey); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "平台公钥格式不正确：" + err.Error()})
			return
		}
	}

	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	if err := ensureSystemConfigStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化系统配置失败"})
		return
	}
	existing, err := loadEpayV2Config(db)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取支付配置失败"})
		return
	}
	if req.EasypayEnabled {
		if req.EasypayGateway == "" || req.EasypayPid == "" ||
			(req.EasypayMerchantKey == "" && existing.MerchantKey == "") ||
			(req.EasypayPlatformKey == "" && existing.PlatformKey == "") {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "启用易支付 V2 前请完整配置网关、PID、商户私钥和平台公钥"})
			return
		}
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存支付配置失败"})
		return
	}
	defer tx.Rollback()

	items := []struct {
		key         string
		value       string
		description string
	}{
		{key: "easypay_v2_enabled", value: boolConfigValue(req.EasypayEnabled), description: "是否启用易支付 V2"},
		{key: "easypay_v2_gateway", value: req.EasypayGateway, description: "易支付 V2 网关地址"},
		{key: "easypay_v2_pid", value: req.EasypayPid, description: "易支付 V2 商户 PID"},
		{key: "easypay_v2_default_type", value: req.EasypayDefaultType, description: "易支付 V2 默认支付方式"},
		{key: "easypay_v2_pay_types", value: strings.Join(payTypes, ","), description: "易支付 V2 已开启支付方式"},
		{key: "easypay_v2_notify_url", value: req.EasypayNotifyUrl, description: "易支付 V2 异步通知地址"},
		{key: "easypay_v2_return_url", value: req.EasypayReturnUrl, description: "易支付 V2 同步跳转地址"},
	}
	if req.EasypayMerchantKey != "" {
		items = append(items, struct {
			key         string
			value       string
			description string
		}{key: "easypay_v2_merchant_key", value: req.EasypayMerchantKey, description: "易支付 V2 商户私钥"})
	}
	if req.EasypayPlatformKey != "" {
		items = append(items, struct {
			key         string
			value       string
			description string
		}{key: "easypay_v2_platform_key", value: req.EasypayPlatformKey, description: "易支付 V2 平台公钥"})
	}
	for _, item := range items {
		if _, err := tx.Exec(`
			INSERT INTO system_configs (`+"`group`"+`, `+"`key`"+`, value, description)
			VALUES ('payment', ?, ?, ?)
			ON DUPLICATE KEY UPDATE value = VALUES(value), description = VALUES(description)
		`, item.key, item.value, item.description); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存支付配置失败"})
			return
		}
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存支付配置失败"})
		return
	}

	result, err := loadEpayV2Config(db)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取支付配置失败"})
		return
	}
	writeSystemConfig(c, http.StatusOK, gin.H{
		"code": 200,
		"msg":  "易支付 V2 配置保存成功",
		"data": paymentV2ConfigResponseFrom(result),
	})
}
