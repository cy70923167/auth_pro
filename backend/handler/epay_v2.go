package handler

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ========== 易支付 V2 插件 ==========
//
// 与 V1（MD5 + submit.php 页面跳转）相比，V2 走 mapi.php 下单接口：
// 服务端 POST 表单签名（RSA-SHA256），返回 JSON 的支付地址/二维码，前端跳收银台。
// 回调验签用平台公钥，商户私钥不出服务端。

type epayV2Config struct {
	Enabled        bool
	Gateway        string
	PID            string
	MerchantKey    string // 商户 RSA 私钥（PEM），仅服务端保存
	PlatformKey    string // 平台 RSA 公钥（PEM），用于验签
	DefaultPayType string
	PayTypes       []string
	NotifyURL      string
	ReturnURL      string
	MerchantKeySet bool
	PlatformKeySet bool
}

// loadEpayV2Config 读取 V2 支付配置。
func loadEpayV2Config(db *sql.DB) (epayV2Config, error) {
	cfg := epayV2Config{DefaultPayType: epayDefaultPayType}
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
		case "easypay_v2_enabled":
			cfg.Enabled = value == "1"
		case "easypay_v2_gateway":
			cfg.Gateway = strings.TrimSpace(value)
		case "easypay_v2_pid":
			cfg.PID = strings.TrimSpace(value)
		case "easypay_v2_merchant_key":
			cfg.MerchantKey = strings.TrimSpace(value)
		case "easypay_v2_platform_key":
			cfg.PlatformKey = strings.TrimSpace(value)
		case "easypay_v2_default_type":
			cfg.DefaultPayType = strings.TrimSpace(value)
		case "easypay_v2_pay_types":
			cfg.PayTypes = parseEpayPayTypes(value)
		case "easypay_v2_notify_url":
			cfg.NotifyURL = strings.TrimSpace(value)
		case "easypay_v2_return_url":
			cfg.ReturnURL = strings.TrimSpace(value)
		}
	}
	if cfg.DefaultPayType == "" {
		cfg.DefaultPayType = epayDefaultPayType
	}
	if cfg.PayTypes == nil {
		cfg.PayTypes = append([]string{}, epayAllPayTypes...)
	}
	cfg.MerchantKeySet = cfg.MerchantKey != ""
	cfg.PlatformKeySet = cfg.PlatformKey != ""
	return cfg, rows.Err()
}

func (cfg epayV2Config) isPayTypeEnabled(payType string) bool {
	if len(cfg.PayTypes) == 0 {
		return true
	}
	for _, item := range cfg.PayTypes {
		if item == payType {
			return true
		}
	}
	return false
}

func (cfg epayV2Config) resolveDefaultPayType() string {
	if cfg.DefaultPayType != "" && cfg.isPayTypeEnabled(cfg.DefaultPayType) {
		return cfg.DefaultPayType
	}
	if len(cfg.PayTypes) > 0 {
		return cfg.PayTypes[0]
	}
	return epayDefaultPayType
}

func (cfg epayV2Config) validateForTest() error {
	if cfg.Gateway == "" {
		return errors.New("请先配置易支付 V2 网关地址")
	}
	if cfg.PID == "" {
		return errors.New("请先配置易支付 V2 商户 PID")
	}
	if cfg.MerchantKey == "" {
		return errors.New("请先配置易支付 V2 商户私钥")
	}
	if cfg.PlatformKey == "" {
		return errors.New("请先配置易支付 V2 平台公钥")
	}
	return nil
}

func (cfg epayV2Config) validateForPay() error {
	if !cfg.Enabled {
		return errors.New("易支付 V2 未启用")
	}
	if err := cfg.validateForTest(); err != nil {
		return err
	}
	if len(cfg.PayTypes) == 0 {
		return errors.New("请先开启至少一种支付方式")
	}
	return nil
}

func (cfg epayV2Config) validateForNotify() error {
	if cfg.PID == "" || cfg.PlatformKey == "" {
		return errors.New("易支付 V2 配置不可用")
	}
	return nil
}

// resolveEpayV2Endpoint 规范化网关地址，最终指向 mapi.php。
func resolveEpayV2Endpoint(rawGateway string) (*url.URL, error) {
	gateway := strings.TrimSpace(rawGateway)
	if gateway == "" {
		return nil, errors.New("易支付 V2 网关地址不能为空")
	}
	parsed, err := url.Parse(gateway)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("易支付 V2 网关地址格式不正确")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("易支付 V2 网关仅支持 http 或 https")
	}
	cleanPath := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(cleanPath, "/mapi.php") {
		cleanPath += "/mapi.php"
	}
	parsed.Path = cleanPath
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, nil
}

// normalizeRSAKey 把用户粘贴的密钥规范化为可解析的 DER 字节。
// 兼容两种形态：标准 PEM（带 BEGIN/END 头尾）和易支付后台导出的裸 base64。
func normalizeRSAKey(raw string, isPrivate bool) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("密钥不能为空")
	}
	// 兼容后台配置里用 \n 转义写入的情况
	raw = strings.ReplaceAll(raw, "\\n", "\n")
	if block, _ := pem.Decode([]byte(raw)); block != nil {
		return block.Bytes, nil
	}
	// 裸 base64：去掉所有空白后解码
	compact := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || r == ' ' {
			return -1
		}
		return r
	}, raw)
	der, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		if isPrivate {
			return nil, errors.New("商户私钥不是有效的 PEM 或 base64 格式")
		}
		return nil, errors.New("平台公钥不是有效的 PEM 或 base64 格式")
	}
	return der, nil
}

// parsePrivateKeyPEM 解析 PKCS#1 / PKCS#8 RSA 私钥（支持 PEM 或裸 base64）。
func parsePrivateKeyPEM(raw string) (*rsa.PrivateKey, error) {
	der, err := normalizeRSAKey(raw, true)
	if err != nil {
		return nil, err
	}
	// 优先按 PKCS#8 解析，失败再回退 PKCS#1
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("商户私钥必须是 RSA 私钥")
		}
		return rsaKey, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	return nil, errors.New("商户私钥解析失败，请确认是 RSA 私钥")
}

// parsePublicKeyPEM 解析 PKIX / PKCS#1 RSA 公钥（支持 PEM 或裸 base64）。
func parsePublicKeyPEM(raw string) (*rsa.PublicKey, error) {
	der, err := normalizeRSAKey(raw, false)
	if err != nil {
		return nil, err
	}
	if key, err := x509.ParsePKIXPublicKey(der); err == nil {
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("平台公钥必须是 RSA 公钥")
		}
		return rsaKey, nil
	}
	if key, err := x509.ParsePKCS1PublicKey(der); err == nil {
		return key, nil
	}
	return nil, errors.New("平台公钥解析失败，请确认是 RSA 公钥")
}

// signEpayV2Params 按 V2 规则做 RSA-SHA256 签名：
// 参数按键名升序拼接 k=v&k=v（跳过 sign/sign_type 和空值），SHA256 后用商户私钥签名，Base64 输出。
func signEpayV2Params(params map[string]string, privateKey *rsa.PrivateKey) (string, error) {
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
	sum := sha256.Sum256([]byte(strings.Join(parts, "&")))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

// verifyEpayV2Sign 用平台公钥验签 V2 回调。
func verifyEpayV2Sign(params map[string]string, publicKey *rsa.PublicKey) bool {
	signText := strings.TrimSpace(params["sign"])
	if signText == "" {
		return false
	}
	signature, err := base64.StdEncoding.DecodeString(signText)
	if err != nil {
		return false
	}
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
	sum := sha256.Sum256([]byte(strings.Join(parts, "&")))
	return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, sum[:], signature) == nil
}

// epayV2CreateOrder 调 mapi.php 创建支付订单，返回收银台跳转地址。
func epayV2CreateOrder(cfg epayV2Config, orderNo string, amountCents int64, payType string, orderName string, notifyURL string, returnURL string, clientIP string) (string, error) {
	endpoint, err := resolveEpayV2Endpoint(cfg.Gateway)
	if err != nil {
		return "", err
	}
	privateKey, err := parsePrivateKeyPEM(cfg.MerchantKey)
	if err != nil {
		return "", err
	}

	params := map[string]string{
		"pid":          cfg.PID,
		"type":         payType,
		"out_trade_no": orderNo,
		"notify_url":   notifyURL,
		"return_url":   returnURL,
		"name":         orderName,
		"money":        formatCents(amountCents),
		"clientip":     clientIP,
		"timestamp":    strconv.FormatInt(time.Now().Unix(), 10),
		"sign_type":    "RSA",
	}
	sign, err := signEpayV2Params(params, privateKey)
	if err != nil {
		return "", errors.New("生成签名失败")
	}
	params["sign"] = sign

	form := url.Values{}
	for key, value := range params {
		form.Set(key, value)
	}
	req, err := http.NewRequest(http.MethodPost, endpoint.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", errors.New("创建支付请求失败")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "auto_pro/1.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("连接支付网关失败：%w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", errors.New("读取支付网关响应失败")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("支付网关返回状态码 %d", resp.StatusCode)
	}

	var result struct {
		Code      int    `json:"code"`
		Msg       string `json:"msg"`
		TradeNo   string `json:"trade_no"`
		PayURL    string `json:"payurl"`
		QRCode    string `json:"qrcode"`
		URLScheme string `json:"urlscheme"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", errors.New("支付网关响应不是有效的 JSON")
	}
	if result.Code != 1 && result.Code != 200 {
		msg := strings.TrimSpace(result.Msg)
		if msg == "" {
			msg = "支付网关下单失败"
		}
		return "", errors.New(msg)
	}
	payURL := strings.TrimSpace(result.PayURL)
	if payURL == "" {
		payURL = strings.TrimSpace(result.QRCode)
	}
	if payURL == "" {
		payURL = strings.TrimSpace(result.URLScheme)
	}
	if payURL == "" {
		return "", errors.New("支付网关未返回支付地址")
	}
	return payURL, nil
}

// buildEpayV2Payment 组装 V2 下单所需的回调地址并发起下单。
func buildEpayV2Payment(c *gin.Context, cfg epayV2Config, orderNo string, amountCents int64, payType string, orderName string, returnPath string) (string, string, error) {
	notifyURL := strings.TrimSpace(cfg.NotifyURL)
	if notifyURL == "" {
		notifyURL = buildRequestURL(c, "/api/payment/easypay-v2/notify")
	}
	frontendReturnURL := buildFrontendReturnURL(c, orderNo, returnPath)
	gatewayReturnURL := strings.TrimSpace(cfg.ReturnURL)
	if gatewayReturnURL == "" {
		gatewayReturnURL = buildRequestURL(c, "/api/payment/easypay-v2/return")
	}
	payURL, err := epayV2CreateOrder(cfg, orderNo, amountCents, payType, orderName, notifyURL, gatewayReturnURL, c.ClientIP())
	if err != nil {
		return "", "", err
	}
	return payURL, frontendReturnURL, nil
}

// settleEpayV2Callback 校验 V2 回调并结算。
func settleEpayV2Callback(db *sql.DB, params map[string]string) error {
	cfg, err := loadEpayV2Config(db)
	if err != nil {
		return err
	}
	if err := cfg.validateForNotify(); err != nil {
		return err
	}
	if params["pid"] != "" && params["pid"] != cfg.PID {
		return errors.New("商户号不匹配")
	}
	publicKey, err := parsePublicKeyPEM(cfg.PlatformKey)
	if err != nil {
		return err
	}
	if !verifyEpayV2Sign(params, publicKey) {
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
	payMethod, ok := normalizeEpayPayType(params["type"], "")
	if !ok {
		return errors.New("回调支付方式不受支持")
	}
	if strings.HasPrefix(orderNo, "AU") {
		return settleAgentUpgradeOnlinePayment(db, orderNo, paidCents, payChannelEpayV2, payMethod, params["trade_no"], string(payload))
	}
	if err := settleRechargeOrder(db, orderNo, paidCents, params["trade_no"], payMethod, string(payload)); err == nil {
		return nil
	}
	// 不是充值订单时，尝试按授权购买订单结算（代理端线上支付开通授权）。
	return settleLicensePurchaseOrder(db, orderNo, paidCents, payChannelEpayV2, params["trade_no"], payMethod, string(payload))
}

// EpayV2Notify V2 异步通知入口。
func EpayV2Notify(c *gin.Context) {
	params := collectEpayParams(c)

	db, err := openSystemConfigDB()
	if err != nil {
		c.String(http.StatusOK, "fail")
		return
	}
	defer db.Close()

	if err := settleEpayV2Callback(db, params); err != nil {
		c.String(http.StatusOK, "fail")
		return
	}
	c.String(http.StatusOK, "success")
}

// EpayV2Return V2 同步跳转入口，验签兜底结算后重定向回前端。
func EpayV2Return(c *gin.Context) {
	params := collectEpayParams(c)
	orderNo := strings.TrimSpace(params["out_trade_no"])
	if orderNo == "" {
		orderNo = strings.TrimSpace(params["rechargeOrder"])
	}

	if db, err := openSystemConfigDB(); err == nil {
		_ = settleEpayV2Callback(db, params)
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
