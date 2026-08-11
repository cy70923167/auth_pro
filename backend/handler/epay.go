package handler

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	epayDefaultPayType  = "alipay"
	minRechargeCents    = int64(1)
	maxRechargeCents    = int64(1_000_000_00)
	maxPaymentTestCents = int64(100_00)
)

// epayAllPayTypes 用户侧可开关的支付方式，顺序即展示顺序。
var epayAllPayTypes = []string{"alipay", "wxpay", "qqpay"}

type epayConfig struct {
	Enabled        bool
	Gateway        string
	PID            string
	Key            string
	DefaultPayType string
	PayTypes       []string
	NotifyURL      string
	ReturnURL      string
}

type createUserRechargeRequest struct {
	Amount  float64 `json:"amount" binding:"required,gt=0"`
	PayType string  `json:"payType"`
}

// UserRechargeCreate 创建用户余额充值订单，仅生成易支付跳转地址，不直接购买授权。
func UserRechargeCreate(c *gin.Context) {
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

	payConfig, err := loadEpayConfig(db)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取易支付配置失败"})
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
	payURL, frontendReturnURL, err := buildEpaySubmitURL(c, payConfig, orderNo, amountCents, payType, "用户余额充值", "/user/dashboard")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	amount := formatCents(amountCents)
	_, err = db.Exec(`
		INSERT INTO recharge_orders (
			order_no, subject_type, subject_id, user_id, amount, pay_channel, pay_method, status, return_url, remark
		) VALUES (?, 'user', ?, ?, ?, 'easypay', ?, 'pending', ?, ?)
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

// UserRechargeStatus 查询当前用户的充值订单状态。
func UserRechargeStatus(c *gin.Context) {
	userID, ok := getUserPanelID(c)
	if !ok {
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

	var amountText, payMethod, status string
	var paidAt sql.NullTime
	err = db.QueryRow(`
		SELECT amount, pay_method, status, paid_at
		FROM recharge_orders
		WHERE order_no = ? AND subject_type = 'user' AND subject_id = ?
	`, orderNo, userID).Scan(&amountText, &payMethod, &status, &paidAt)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "充值订单不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询充值订单失败"})
		return
	}

	var balance float64
	_ = db.QueryRow("SELECT balance FROM users WHERE id = ?", userID).Scan(&balance)

	paidAtText := ""
	if paidAt.Valid {
		paidAtText = paidAt.Time.Format("2006-01-02 15:04:05")
	}
	amount, _ := strconv.ParseFloat(amountText, 64)
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"orderNo": orderNo,
			"amount":  amount,
			"payType": payMethod,
			"status":  status,
			"paidAt":  paidAtText,
			"balance": balance,
		},
	})
}

// settleEpayCallback 校验易支付回调参数并在支付成功时结算，异步通知与同步跳转共用。
// 结算本身幂等，因此两个入口都可安全调用。
func settleEpayCallback(db *sql.DB, params map[string]string) error {
	payConfig, err := loadEpayConfig(db)
	if err != nil {
		return err
	}
	if err := payConfig.validateForNotify(); err != nil {
		return err
	}
	if params["pid"] != "" && params["pid"] != payConfig.PID {
		return errors.New("商户号不匹配")
	}
	if !verifyEpaySign(params, payConfig.Key) {
		return errors.New("验签失败")
	}
	if strings.ToUpper(strings.TrimSpace(params["trade_status"])) != "TRADE_SUCCESS" {
		return errors.New("交易未成功")
	}

	orderNo := strings.TrimSpace(params["out_trade_no"])
	paidCents, err := parseAmountToCents(params["money"])
	if orderNo == "" || err != nil || paidCents <= 0 {
		return errors.New("回调参数不完整")
	}

	payload, _ := json.Marshal(params)
	if err := settleRechargeOrder(db, orderNo, paidCents, params["trade_no"], params["type"], string(payload)); err == nil {
		return nil
	}
	// 不是充值订单时，尝试按授权购买订单结算（代理端线上支付开通授权）。
	return settleLicensePurchaseOrder(db, orderNo, paidCents, params["trade_no"], params["type"], string(payload))
}

// EpayNotify 易支付异步通知入口。只有验签、金额、订单号全部通过后才入账。
func EpayNotify(c *gin.Context) {
	params := collectEpayParams(c)

	db, err := openSystemConfigDB()
	if err != nil {
		c.String(http.StatusOK, "fail")
		return
	}
	defer db.Close()

	if err := settleEpayCallback(db, params); err != nil {
		c.String(http.StatusOK, "fail")
		return
	}

	c.String(http.StatusOK, "success")
}

// EpayReturn 易支付同步跳转入口。因异步通知在本地或内网环境常常无法送达，
// 这里对同步跳转做一次验签兜底结算，随后把用户重定向回前端落地页。
func EpayReturn(c *gin.Context) {
	params := collectEpayParams(c)
	orderNo := strings.TrimSpace(params["out_trade_no"])
	if orderNo == "" {
		orderNo = strings.TrimSpace(params["rechargeOrder"])
	}

	if db, err := openSystemConfigDB(); err == nil {
		// 兜底结算失败时忽略错误（可能验签不通过或已由 notify 入账），仅继续跳转。
		_ = settleEpayCallback(db, params)
		db.Close()
	}

	redirectURL := ""
	if orderNo != "" {
		redirectURL = loadRechargeReturnURL(orderNo)
	}
	if redirectURL == "" {
		redirectURL = buildFrontendReturnURL(c, orderNo, "")
	}
	c.Redirect(http.StatusFound, redirectURL)
}

func getUserPanelID(c *gin.Context) (uint, bool) {
	role, _ := c.Get("role")
	if role != "user" {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "无权限"})
		return 0, false
	}
	value, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "认证信息缺失"})
		return 0, false
	}
	userID, ok := value.(uint)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "认证信息异常"})
		return 0, false
	}
	return userID, true
}

func loadEpayConfig(db *sql.DB) (epayConfig, error) {
	cfg := epayConfig{DefaultPayType: epayDefaultPayType}
	if err := ensureSystemConfigStorage(db); err != nil {
		return cfg, err
	}

	rows, err := db.Query("SELECT `key`, value FROM system_configs WHERE `group` = 'payment'")
	if err != nil {
		return cfg, err
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return cfg, err
		}
		switch key {
		case "easypay_enabled":
			cfg.Enabled = value == "1"
		case "easypay_gateway":
			cfg.Gateway = strings.TrimSpace(value)
		case "easypay_pid":
			cfg.PID = strings.TrimSpace(value)
		case "easypay_key":
			cfg.Key = strings.TrimSpace(value)
		case "easypay_default_type":
			cfg.DefaultPayType = strings.TrimSpace(value)
		case "easypay_pay_types":
			cfg.PayTypes = parseEpayPayTypes(value)
		case "easypay_notify_url":
			cfg.NotifyURL = strings.TrimSpace(value)
		case "easypay_return_url":
			cfg.ReturnURL = strings.TrimSpace(value)
		}
	}
	if cfg.DefaultPayType == "" {
		cfg.DefaultPayType = epayDefaultPayType
	}
	if cfg.PayTypes == nil {
		cfg.PayTypes = append([]string{}, epayAllPayTypes...)
	}
	return cfg, rows.Err()
}

// parseEpayPayTypes 解析逗号分隔的支付方式配置，仅保留合法值并按固定顺序去重。
// 返回 nil 表示配置缺失（历史数据），调用方应回退为全部开启。
func parseEpayPayTypes(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	enabled := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		payType, ok := normalizeEpayPayType(item, "")
		if ok {
			enabled[payType] = true
		}
	}
	result := []string{}
	for _, payType := range epayAllPayTypes {
		if enabled[payType] {
			result = append(result, payType)
		}
	}
	return result
}

func (cfg epayConfig) isPayTypeEnabled(payType string) bool {
	for _, item := range cfg.PayTypes {
		if item == payType {
			return true
		}
	}
	return false
}

// resolveDefaultPayType 返回默认支付方式；默认方式未开启时退回第一个已开启方式。
func (cfg epayConfig) resolveDefaultPayType() string {
	payType, ok := normalizeEpayPayType(cfg.DefaultPayType, epayDefaultPayType)
	if !ok {
		payType = epayDefaultPayType
	}
	if cfg.isPayTypeEnabled(payType) {
		return payType
	}
	if len(cfg.PayTypes) > 0 {
		return cfg.PayTypes[0]
	}
	return payType
}

// normalizeEpayPayTypes 校验管理端提交的支付方式列表，去重并按固定顺序返回。
func normalizeEpayPayTypes(values []string) ([]string, error) {
	togglable := map[string]bool{}
	for _, payType := range epayAllPayTypes {
		togglable[payType] = true
	}

	enabled := map[string]bool{}
	for _, item := range values {
		if strings.TrimSpace(item) == "" {
			continue
		}
		payType, ok := normalizeEpayPayType(item, "")
		if !ok || !togglable[payType] {
			return nil, errors.New("存在不支持的支付方式")
		}
		enabled[payType] = true
	}

	result := []string{}
	for _, payType := range epayAllPayTypes {
		if enabled[payType] {
			result = append(result, payType)
		}
	}
	return result, nil
}

// UserRechargeOptions 返回用户充值时可用的支付方式，仅包含管理员已开启的方式。
func UserRechargeOptions(c *gin.Context) {
	if _, ok := getUserPanelID(c); !ok {
		return
	}

	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	payConfig, err := loadEpayConfig(db)
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

func requireAdminRole(c *gin.Context) bool {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "无权限"})
		return false
	}
	return true
}

type createPaymentTestRequest struct {
	Amount  float64 `json:"amount" binding:"required,gt=0"`
	PayType string  `json:"payType"`
}

// AdminPaymentTestCreate 管理员发起易支付测试订单。测试订单支付成功后只记录结果，不入账。
func AdminPaymentTestCreate(c *gin.Context) {
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

	payConfig, err := loadEpayConfig(db)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取易支付配置失败"})
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
	payURL, frontendReturnURL, err := buildEpaySubmitURL(c, payConfig, orderNo, amountCents, payType, "易支付测试支付", "/system/epay-config")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	amount := formatCents(amountCents)
	_, err = db.Exec(`
		INSERT INTO recharge_orders (
			order_no, subject_type, subject_id, amount, pay_channel, pay_method, status, return_url, remark
		) VALUES (?, 'test', 0, ?, 'easypay', ?, 'pending', ?, ?)
	`, orderNo, amount, payType, frontendReturnURL, "管理员支付测试")
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

// AdminPaymentTestStatus 查询测试订单支付结果。
func AdminPaymentTestStatus(c *gin.Context) {
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
		SELECT amount, pay_method, status, COALESCE(gateway_trade_no, ''), paid_at
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
			"paidAt":         paidAtText,
			"gatewayTradeNo": gatewayTradeNo,
		},
	})
}

// AdminPaymentOrderList 支付订单列表，支持订单号搜索与主体类型/状态/支付方式筛选和分页。
func AdminPaymentOrderList(c *gin.Context) {
	if !requireAdminRole(c) {
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

	if err := ensureLicensePurchaseOrderSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化购买订单表失败"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	where := []string{"1=1"}
	args := []any{}

	if orderNo := strings.TrimSpace(c.Query("orderNo")); orderNo != "" {
		where = append(where, "o.order_no LIKE ?")
		args = append(args, "%"+orderNo+"%")
	}
	if subjectType := strings.TrimSpace(c.Query("subjectType")); subjectType != "" {
		where = append(where, "o.subject_type = ?")
		args = append(args, subjectType)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		where = append(where, "o.status = ?")
		args = append(args, status)
	}
	if payMethod := strings.TrimSpace(c.Query("payMethod")); payMethod != "" {
		where = append(where, "o.pay_method = ?")
		args = append(args, payMethod)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	countSQL := `
		SELECT COUNT(*) FROM (
			SELECT o.order_no FROM recharge_orders o WHERE ` + whereSQL + `
			UNION ALL
			SELECT o.order_no FROM license_purchase_orders o WHERE ` + whereSQL + `
		) t`
	if err := db.QueryRow(countSQL, append(args, args...)...).Scan(&total); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询订单总数失败"})
		return
	}

	offset := (page - 1) * pageSize
	listSQL := `
		SELECT * FROM (
			SELECT o.order_no, o.subject_type, o.subject_id,
			       COALESCE(u.nickname, u.email, a.name, '') AS subject_name,
			       o.amount, o.paid_amount, o.pay_channel, o.pay_method, o.status,
			       COALESCE(o.gateway_trade_no, ''), COALESCE(o.remark, ''),
			       o.created_at, o.paid_at
			FROM recharge_orders o
			LEFT JOIN users u ON u.id = o.subject_id AND o.subject_type = 'user'
			LEFT JOIN agents a ON a.id = o.subject_id AND o.subject_type = 'agent'
			WHERE ` + whereSQL + `
			UNION ALL
			SELECT o.order_no, o.owner_type AS subject_type, o.owner_id AS subject_id,
			       COALESCE(u.nickname, u.email, a.name, '') AS subject_name,
			       o.amount, o.paid_amount, 'easypay' AS pay_channel, o.pay_method, o.status,
			       COALESCE(o.gateway_trade_no, ''), COALESCE(o.remark, ''),
			       o.created_at, o.paid_at
			FROM license_purchase_orders o
			LEFT JOIN users u ON u.id = o.owner_id AND o.owner_type = 'user'
			LEFT JOIN agents a ON a.id = o.owner_id AND o.owner_type = 'agent'
			WHERE ` + whereSQL + `
		) t
		ORDER BY t.created_at DESC
		LIMIT ? OFFSET ?`
	queryArgs := append(args, args...)
	queryArgs = append(queryArgs, pageSize, offset)
	rows, err := db.Query(listSQL, queryArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询订单失败: " + err.Error()})
		return
	}
	defer rows.Close()

	type orderItem struct {
		OrderNo        string  `json:"orderNo"`
		SubjectType    string  `json:"subjectType"`
		SubjectID      uint64  `json:"subjectId"`
		SubjectName    string  `json:"subjectName"`
		Amount         float64 `json:"amount"`
		PaidAmount     float64 `json:"paidAmount"`
		PayChannel     string  `json:"payChannel"`
		PayMethod      string  `json:"payMethod"`
		Status         string  `json:"status"`
		GatewayTradeNo string  `json:"gatewayTradeNo"`
		Remark         string  `json:"remark"`
		CreatedAt      string  `json:"createdAt"`
		PaidAt         string  `json:"paidAt"`
	}

	list := []orderItem{}
	for rows.Next() {
		var item orderItem
		var amountText string
		var paidAmount sql.NullString
		var createdAt sql.NullTime
		var paidAt sql.NullTime
		if err := rows.Scan(
			&item.OrderNo, &item.SubjectType, &item.SubjectID, &item.SubjectName,
			&amountText, &paidAmount, &item.PayChannel, &item.PayMethod, &item.Status,
			&item.GatewayTradeNo, &item.Remark, &createdAt, &paidAt,
		); err != nil {
			continue
		}
		item.Amount, _ = strconv.ParseFloat(amountText, 64)
		if paidAmount.Valid {
			item.PaidAmount, _ = strconv.ParseFloat(paidAmount.String, 64)
		}
		if createdAt.Valid {
			item.CreatedAt = createdAt.Time.Format("2006-01-02 15:04:05")
		}
		if paidAt.Valid {
			item.PaidAt = paidAt.Time.Format("2006-01-02 15:04:05")
		}
		list = append(list, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{"list": list, "total": total, "page": page, "pageSize": pageSize},
	})
}

// validateForTest 测试支付只要求商户信息完整，不要求通道已启用。
func (cfg epayConfig) validateForTest() error {
	if cfg.Gateway == "" {
		return errors.New("请先配置易支付网关地址")
	}
	if cfg.PID == "" {
		return errors.New("请先配置易支付商户 PID")
	}
	if cfg.Key == "" {
		return errors.New("请先配置易支付商户 Key")
	}
	return nil
}

func (cfg epayConfig) validateForPay() error {
	if !cfg.Enabled {
		return errors.New("易支付未启用")
	}
	if err := cfg.validateForTest(); err != nil {
		return err
	}
	if len(cfg.PayTypes) == 0 {
		return errors.New("请先开启至少一种支付方式")
	}
	return nil
}

// validateForNotify 入账安全以验签为准；通道开关只控制新订单创建，
// 不拦截回调，避免已支付的订单（含测试单）因开关关闭而无法入账。
func (cfg epayConfig) validateForNotify() error {
	if cfg.PID == "" || cfg.Key == "" {
		return errors.New("易支付配置不可用")
	}
	return nil
}

func normalizeEpayPayType(value string, fallback string) (string, bool) {
	payType := strings.ToLower(strings.TrimSpace(value))
	if payType == "" {
		payType = strings.ToLower(strings.TrimSpace(fallback))
	}
	if payType == "" {
		payType = epayDefaultPayType
	}
	switch payType {
	case "alipay", "wxpay", "qqpay", "bank":
		return payType, true
	case "wechat", "weixin":
		return "wxpay", true
	default:
		return "", false
	}
}

func buildEpaySubmitURL(c *gin.Context, cfg epayConfig, orderNo string, amountCents int64, payType string, orderName string, returnPath string) (string, string, error) {
	endpoint, err := resolveEpayEndpoint(cfg.Gateway)
	if err != nil {
		return "", "", err
	}

	notifyURL := strings.TrimSpace(cfg.NotifyURL)
	if notifyURL == "" {
		notifyURL = buildRequestURL(c, "/api/payment/easypay/notify")
	}
	// frontendReturnURL 存入订单，供后端 EpayReturn 结算后重定向回落地页。
	frontendReturnURL := buildFrontendReturnURL(c, orderNo, returnPath)
	// 网关同步跳转指向后端 return 端点，且保持无额外查询参数：
	// 额外参数会被算进网关回调验签导致失败。本地/内网下异步 notify 常无法送达，
	// 由后端在同步跳转时凭网关回传的 out_trade_no 做验签兜底结算，避免订单一直 pending。
	gatewayReturnURL := strings.TrimSpace(cfg.ReturnURL)
	if gatewayReturnURL == "" {
		gatewayReturnURL = buildRequestURL(c, "/api/payment/easypay/return")
	}

	params := map[string]string{
		"pid":          cfg.PID,
		"type":         payType,
		"out_trade_no": orderNo,
		"notify_url":   notifyURL,
		"return_url":   gatewayReturnURL,
		"name":         orderName,
		"money":        formatCents(amountCents),
	}
	params["sign"] = signEpayParams(params, cfg.Key)
	params["sign_type"] = "MD5"

	query := endpoint.Query()
	for key, value := range params {
		query.Set(key, value)
	}
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), frontendReturnURL, nil
}

func resolveEpayEndpoint(rawGateway string) (*url.URL, error) {
	gateway := strings.TrimSpace(rawGateway)
	if gateway == "" {
		return nil, errors.New("易支付网关地址不能为空")
	}
	parsed, err := url.Parse(gateway)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("易支付网关地址格式不正确")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("易支付网关仅支持 http 或 https")
	}
	cleanPath := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(cleanPath, "/submit.php") && !strings.HasSuffix(cleanPath, "/mapi.php") {
		cleanPath += "/submit.php"
	}
	parsed.Path = cleanPath
	return parsed, nil
}

func signEpayParams(params map[string]string, key string) string {
	keys := make([]string, 0, len(params))
	for name, value := range params {
		if name == "sign" || name == "sign_type" || strings.TrimSpace(value) == "" {
			continue
		}
		keys = append(keys, name)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, name := range keys {
		parts = append(parts, name+"="+params[name])
	}
	sum := md5.Sum([]byte(strings.Join(parts, "&") + key))
	return strings.ToLower(hex.EncodeToString(sum[:]))
}

func verifyEpaySign(params map[string]string, key string) bool {
	sign := strings.ToLower(strings.TrimSpace(params["sign"]))
	if sign == "" {
		return false
	}
	return sign == signEpayParams(params, key)
}

func collectEpayParams(c *gin.Context) map[string]string {
	_ = c.Request.ParseForm()
	params := map[string]string{}
	for key, values := range c.Request.Form {
		if len(values) == 0 {
			continue
		}
		params[key] = values[0]
	}
	return params
}

func settleRechargeOrder(db *sql.DB, orderNo string, paidCents int64, gatewayTradeNo string, payMethod string, notifyPayload string) error {
	if err := ensureRechargeOrderSchema(db); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var id uint64
	var subjectType, amountText, status string
	var subjectID uint64
	err = tx.QueryRow(`
		SELECT id, subject_type, subject_id, amount, status
		FROM recharge_orders
		WHERE order_no = ?
		FOR UPDATE
	`, orderNo).Scan(&id, &subjectType, &subjectID, &amountText, &status)
	if err != nil {
		return err
	}

	expectedCents, err := parseAmountToCents(amountText)
	if err != nil || expectedCents != paidCents {
		return errors.New("充值订单金额不一致")
	}
	if status == "paid" {
		return tx.Commit()
	}
	if status != "pending" {
		return errors.New("充值订单状态不可入账")
	}

	if payMethod == "" {
		payMethod = "easypay"
	}

	// 测试订单只记录支付结果，不给任何账户入账，也不产生流水。
	if subjectType == "test" {
		_, err = tx.Exec(`
			UPDATE recharge_orders
			SET status = 'paid', paid_at = NOW(), paid_amount = ?, gateway_trade_no = ?, pay_method = ?, notify_payload = ?
			WHERE id = ?
		`, formatCents(paidCents), gatewayTradeNo, payMethod, notifyPayload, id)
		if err != nil {
			return err
		}
		return tx.Commit()
	}

	balanceAfter, err := increaseSubjectBalance(tx, subjectType, subjectID, paidCents)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE recharge_orders
		SET status = 'paid', paid_at = NOW(), paid_amount = ?, gateway_trade_no = ?, pay_method = ?, notify_payload = ?
		WHERE id = ?
	`, formatCents(paidCents), gatewayTradeNo, payMethod, notifyPayload, id)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO transactions (tx_no, subject_type, subject_id, type, amount, balance_after, ref_type, ref_id, remark)
		VALUES (?, ?, ?, 'recharge', ?, ?, 'recharge_order', ?, ?)
	`, generateTransactionNo(), subjectType, subjectID, formatCents(paidCents), balanceAfter, id, "易支付余额充值")
	if err != nil {
		return err
	}

	return tx.Commit()
}

func increaseSubjectBalance(tx *sql.Tx, subjectType string, subjectID uint64, amountCents int64) (string, error) {
	amount := formatCents(amountCents)
	table := ""
	switch subjectType {
	case "user":
		table = "users"
	case "agent":
		table = "agents"
	default:
		return "", errors.New("不支持的充值主体")
	}

	result, err := tx.Exec("UPDATE "+table+" SET balance = balance + ? WHERE id = ?", amount, subjectID)
	if err != nil {
		return "", err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return "", err
	}
	if affected == 0 {
		return "", errors.New("充值主体不存在")
	}

	var balanceAfter string
	if err := tx.QueryRow("SELECT balance FROM "+table+" WHERE id = ?", subjectID).Scan(&balanceAfter); err != nil {
		return "", err
	}
	return balanceAfter, nil
}

func ensureRechargeOrderSchema(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS recharge_orders (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
			order_no VARCHAR(64) NOT NULL COMMENT '订单号',
			subject_type ENUM('agent','user','test') NOT NULL DEFAULT 'agent' COMMENT '充值主体类型',
			subject_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '充值主体ID',
			agent_id BIGINT UNSIGNED DEFAULT NULL COMMENT '充值代理商ID',
			user_id BIGINT UNSIGNED DEFAULT NULL COMMENT '充值用户ID',
			amount DECIMAL(12,2) NOT NULL COMMENT '充值金额',
			paid_amount DECIMAL(12,2) DEFAULT NULL COMMENT '实际支付金额',
			pay_channel VARCHAR(30) DEFAULT '' COMMENT '支付渠道',
			pay_method VARCHAR(30) DEFAULT '' COMMENT '支付方式',
			gateway_trade_no VARCHAR(100) DEFAULT '' COMMENT '支付网关交易号',
			return_url VARCHAR(500) DEFAULT '' COMMENT '支付完成前端返回地址',
			status ENUM('pending','paid','failed','cancelled') DEFAULT 'pending' COMMENT '状态',
			paid_at DATETIME DEFAULT NULL COMMENT '支付完成时间',
			approved_by BIGINT UNSIGNED DEFAULT NULL COMMENT '审核人',
			remark VARCHAR(255) DEFAULT '' COMMENT '备注',
			notify_payload TEXT COMMENT '支付回调原始参数',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
			PRIMARY KEY (id),
			UNIQUE KEY uk_order_no (order_no),
			KEY idx_subject (subject_type, subject_id, status),
			KEY idx_agent (agent_id, status),
			KEY idx_user (user_id, status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='充值订单表'
	`); err != nil {
		return err
	}

	columns := []struct {
		name string
		ddl  string
	}{
		{name: "subject_type", ddl: "ALTER TABLE recharge_orders ADD COLUMN subject_type ENUM('agent','user','test') NOT NULL DEFAULT 'agent' COMMENT '充值主体类型' AFTER order_no"},
		{name: "subject_id", ddl: "ALTER TABLE recharge_orders ADD COLUMN subject_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '充值主体ID' AFTER subject_type"},
		{name: "user_id", ddl: "ALTER TABLE recharge_orders ADD COLUMN user_id BIGINT UNSIGNED DEFAULT NULL COMMENT '充值用户ID' AFTER agent_id"},
		{name: "paid_amount", ddl: "ALTER TABLE recharge_orders ADD COLUMN paid_amount DECIMAL(12,2) DEFAULT NULL COMMENT '实际支付金额' AFTER amount"},
		{name: "pay_channel", ddl: "ALTER TABLE recharge_orders ADD COLUMN pay_channel VARCHAR(30) DEFAULT '' COMMENT '支付渠道' AFTER paid_amount"},
		{name: "gateway_trade_no", ddl: "ALTER TABLE recharge_orders ADD COLUMN gateway_trade_no VARCHAR(100) DEFAULT '' COMMENT '支付网关交易号' AFTER pay_method"},
		{name: "return_url", ddl: "ALTER TABLE recharge_orders ADD COLUMN return_url VARCHAR(500) DEFAULT '' COMMENT '支付完成前端返回地址' AFTER gateway_trade_no"},
		{name: "notify_payload", ddl: "ALTER TABLE recharge_orders ADD COLUMN notify_payload TEXT COMMENT '支付回调原始参数' AFTER remark"},
	}
	for _, column := range columns {
		if err := ensureColumn(db, "recharge_orders", column.name, column.ddl); err != nil {
			return err
		}
	}

	_, _ = db.Exec("ALTER TABLE recharge_orders MODIFY COLUMN subject_type ENUM('agent','user','test') NOT NULL DEFAULT 'agent' COMMENT '充值主体类型'")
	_, _ = db.Exec("ALTER TABLE recharge_orders MODIFY COLUMN agent_id BIGINT UNSIGNED DEFAULT NULL COMMENT '充值代理商ID'")
	_, _ = db.Exec("UPDATE recharge_orders SET subject_type = 'agent' WHERE subject_type = '' OR subject_type IS NULL")
	_, _ = db.Exec("UPDATE recharge_orders SET subject_id = agent_id WHERE subject_type = 'agent' AND subject_id = 0 AND agent_id IS NOT NULL")
	_, _ = db.Exec("UPDATE recharge_orders SET agent_id = subject_id WHERE subject_type = 'agent' AND (agent_id IS NULL OR agent_id = 0)")
	_, _ = db.Exec("UPDATE recharge_orders SET user_id = subject_id WHERE subject_type = 'user' AND (user_id IS NULL OR user_id = 0)")

	indexes := []struct {
		name    string
		columns []string
		unique  bool
	}{
		{name: "uk_order_no", columns: []string{"order_no"}, unique: true},
		{name: "idx_subject", columns: []string{"subject_type", "subject_id", "status"}},
		{name: "idx_agent", columns: []string{"agent_id", "status"}},
		{name: "idx_user", columns: []string{"user_id", "status"}},
	}
	for _, index := range indexes {
		if err := ensureIndex(db, "recharge_orders", index.name, index.columns, index.unique); err != nil {
			return err
		}
	}
	return nil
}

func ensureColumn(db *sql.DB, table string, column string, ddl string) error {
	var exists int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?
	`, table, column).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}
	_, err := db.Exec(ddl)
	return err
}

func ensureIndex(db *sql.DB, table string, preferredName string, columns []string, unique bool) error {
	exists, err := indexWithColumnsExists(db, table, columns, unique)
	if err != nil || exists {
		return err
	}

	for attempt := 0; attempt < 8; attempt++ {
		indexName := preferredName
		if attempt > 0 {
			indexName = fmt.Sprintf("%s_%d", preferredName, attempt+1)
		}

		nameExists, err := indexNameExists(db, table, indexName)
		if err != nil {
			return err
		}
		if nameExists {
			continue
		}

		_, err = db.Exec(buildAddIndexDDL(table, indexName, columns, unique))
		if err == nil {
			return nil
		}
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key name") {
			continue
		}
		return err
	}

	exists, err = indexWithColumnsExists(db, table, columns, unique)
	if err != nil || exists {
		return err
	}
	return fmt.Errorf("创建索引 %s 失败", preferredName)
}

func indexWithColumnsExists(db *sql.DB, table string, columns []string, unique bool) (bool, error) {
	columnList := strings.Join(columns, ",")
	requireUnique := 0
	if unique {
		requireUnique = 1
	}

	var exists int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT INDEX_NAME, NON_UNIQUE, GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') AS col_names
			FROM information_schema.STATISTICS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
			GROUP BY INDEX_NAME, NON_UNIQUE
			HAVING col_names = ? AND (? = 0 OR NON_UNIQUE = 0)
		) matched
	`, table, columnList, requireUnique).Scan(&exists); err != nil {
		return false, err
	}
	return exists > 0, nil
}

func indexNameExists(db *sql.DB, table string, index string) (bool, error) {
	var exists int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?
	`, table, index).Scan(&exists); err != nil {
		return false, err
	}
	return exists > 0, nil
}

func buildAddIndexDDL(table string, index string, columns []string, unique bool) string {
	keyword := "KEY"
	if unique {
		keyword = "UNIQUE KEY"
	}
	quotedColumns := make([]string, 0, len(columns))
	for _, column := range columns {
		quotedColumns = append(quotedColumns, quoteIdentifier(column))
	}
	return fmt.Sprintf("ALTER TABLE %s ADD %s %s (%s)", quoteIdentifier(table), keyword, quoteIdentifier(index), strings.Join(quotedColumns, ", "))
}

func quoteIdentifier(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func floatAmountToCents(amount float64) int64 {
	return int64(math.Round(amount * 100))
}

func parseAmountToCents(value string) (int64, error) {
	amount := strings.TrimSpace(value)
	if amount == "" {
		return 0, errors.New("金额为空")
	}
	parts := strings.Split(amount, ".")
	if len(parts) > 2 {
		return 0, errors.New("金额格式错误")
	}
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || yuan < 0 {
		return 0, errors.New("金额格式错误")
	}
	centText := "00"
	if len(parts) == 2 {
		centText = parts[1]
		if len(centText) == 1 {
			centText += "0"
		}
		if len(centText) > 2 {
			return 0, errors.New("金额最多保留两位小数")
		}
	}
	cent, err := strconv.ParseInt(centText, 10, 64)
	if err != nil || cent < 0 || cent > 99 {
		return 0, errors.New("金额格式错误")
	}
	return yuan*100 + cent, nil
}

func formatCents(cents int64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

func generateRechargeOrderNo() string {
	return fmt.Sprintf("UR%d%06d", time.Now().Unix(), time.Now().Nanosecond()%1_000_000)
}

func generatePaymentTestOrderNo() string {
	return fmt.Sprintf("PT%d%06d", time.Now().Unix(), time.Now().Nanosecond()%1_000_000)
}

func generateTransactionNo() string {
	return fmt.Sprintf("TX%d%06d", time.Now().Unix(), time.Now().Nanosecond()%1_000_000)
}

func loadRechargeReturnURL(orderNo string) string {
	db, err := openSystemConfigDB()
	if err != nil {
		return ""
	}
	defer db.Close()

	if err := ensureRechargeOrderSchema(db); err != nil {
		return ""
	}

	var returnURL string
	if err := db.QueryRow("SELECT return_url FROM recharge_orders WHERE order_no = ?", orderNo).Scan(&returnURL); err != nil {
		// 充值订单查不到时，再查授权购买订单（代理/用户线上购买），两者共用回跳地址表
		if err2 := db.QueryRow("SELECT return_url FROM license_purchase_orders WHERE order_no = ?", orderNo).Scan(&returnURL); err2 != nil {
			return ""
		}
	}
	returnURL = strings.TrimSpace(returnURL)
	parsed, err := url.Parse(returnURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return returnURL
}

func buildRequestURL(c *gin.Context, routePath string) string {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	return scheme + "://" + host + routePath
}

func buildFrontendReturnURL(c *gin.Context, orderNo string, returnPath string) string {
	if returnPath == "" {
		// 兜底按订单来源决定回跳端：UP=用户购买，LP=代理购买，其余=充值（用户端）
		if strings.HasPrefix(orderNo, "UP") {
			returnPath = "/user/purchase"
		} else if strings.HasPrefix(orderNo, "LP") {
			returnPath = "/agent/purchase"
		} else {
			returnPath = "/user/dashboard"
		}
	}
	// 前端地址优先级：环境变量 > Referer（浏览器真实来源，支付下单请求由浏览器发出，
	// Origin 在某些代理链路下会丢或变成后端地址，Referer 更可靠）> Origin > 后端自身地址。
	origin := frontendOriginFromEnv()
	if origin == "" {
		origin = originFromURL(c.GetHeader("Referer"))
	}
	if origin == "" {
		origin = originFromURL(c.GetHeader("Origin"))
	}
	if origin == "" {
		origin = frontendOriginFromPort(c)
	}
	if origin == "" {
		origin = strings.TrimRight(buildRequestURL(c, ""), "/")
	}
	returnURL := origin + returnPath
	if orderNo == "" {
		return returnURL
	}
	returnURL, err := appendURLQuery(returnURL, map[string]string{
		"rechargeOrder":  orderNo,
		"rechargeReturn": "1",
	})
	if err != nil {
		return origin + returnPath + "?rechargeOrder=" + url.QueryEscape(orderNo) + "&rechargeReturn=1"
	}
	return returnURL
}

// frontendOriginFromEnv 读取显式配置的前端地址，生产部署（前后端分离/反代）时建议设置。
func frontendOriginFromEnv() string {
	value := strings.TrimRight(strings.TrimSpace(os.Getenv("AUTO_PRO_FRONTEND_URL")), "/")
	if value == "" {
		return ""
	}
	return originFromURL(value)
}

// frontendOriginFromPort 当请求直接打到后端（如网关同步回跳，无 Referer/Origin）时，
// 按本机前端 dev server 常用端口推断前端地址，优先于后端自身地址。
func frontendOriginFromPort(c *gin.Context) string {
	host := c.Request.Host
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	if host == "" {
		return ""
	}
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	for _, port := range []string{"3006", "5173", "3000"} {
		// 简单探测该端口是否有前端服务在监听（本地开发场景）
		if probeLocalPort(port) {
			return scheme + "://" + host + ":" + port
		}
	}
	return ""
}

// probeLocalPort 快速探测本机端口是否有 HTTP 服务响应。
func probeLocalPort(port string) bool {
	client := &http.Client{Timeout: 300 * time.Millisecond}
	resp, err := client.Get("http://127.0.0.1:" + port + "/")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}

// originFromURL 从完整 URL 中提取 scheme://host，非法或不支持的协议返回空串。
func originFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func appendURLQuery(rawURL string, values map[string]string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("URL 格式不正确")
	}
	query := parsed.Query()
	for key, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
