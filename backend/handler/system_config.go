package handler

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

const (
	defaultSiteName     = "授权管理系统"
	defaultSiteSubtitle = "专业的软件授权与服务平台"
	maxSiteNameLen      = 60
	maxSiteSubtitleLen  = 120
	maxStationQQlen     = 30
	maxICPNumberLen     = 100
	maxSiteNoticeLen    = 2000
	maxLogoBytes        = 512 * 1024
)

var (
	systemConfigStorageMu    sync.Mutex
	systemConfigStorageReady bool
)

type systemConfigResponse struct {
	SiteName               string `json:"siteName"`
	SiteSubtitle           string `json:"siteSubtitle"`
	SiteLogo               string `json:"siteLogo"`
	InstalledAt            string `json:"installedAt"`
	StationQQ              string `json:"stationQQ"`
	ICPNumber              string `json:"icpNumber"`
	DomainLicenseNotice    string `json:"domainLicenseNotice"`
	RegistrationEnabled    bool   `json:"registrationEnabled"`
	SelfPurchaseEnabled    bool   `json:"selfPurchaseEnabled"`
	PiracyDetectionEnabled bool   `json:"piracyDetectionEnabled"`
}

type updateSystemConfigRequest struct {
	SiteName               string `json:"siteName"`
	SiteSubtitle           string `json:"siteSubtitle"`
	SiteLogo               string `json:"siteLogo"`
	StationQQ              string `json:"stationQQ"`
	ICPNumber              string `json:"icpNumber"`
	DomainLicenseNotice    string `json:"domainLicenseNotice"`
	RegistrationEnabled    bool   `json:"registrationEnabled"`
	SelfPurchaseEnabled    bool   `json:"selfPurchaseEnabled"`
	PiracyDetectionEnabled bool   `json:"piracyDetectionEnabled"`
}

type paymentConfigResponse struct {
	EasypayEnabled        bool     `json:"easypayEnabled"`
	EasypayGateway        string   `json:"easypayGateway"`
	EasypayPID            string   `json:"easypayPid"`
	EasypayMerchantKey    string   `json:"easypayMerchantKey,omitempty"`
	EasypayMerchantKeySet bool     `json:"easypayMerchantKeySet"`
	EasypayDefaultType    string   `json:"easypayDefaultType"`
	EasypayPayTypes       []string `json:"easypayPayTypes"`
	EasypayNotifyURL      string   `json:"easypayNotifyUrl"`
	EasypayReturnURL      string   `json:"easypayReturnUrl"`
}

type updatePaymentConfigRequest struct {
	EasypayEnabled     bool     `json:"easypayEnabled"`
	EasypayGateway     string   `json:"easypayGateway"`
	EasypayPID         string   `json:"easypayPid"`
	EasypayMerchantKey string   `json:"easypayMerchantKey"`
	EasypayDefaultType string   `json:"easypayDefaultType"`
	EasypayPayTypes    []string `json:"easypayPayTypes"`
	EasypayNotifyURL   string   `json:"easypayNotifyUrl"`
	EasypayReturnURL   string   `json:"easypayReturnUrl"`
}

type updateSystemFeatureSwitchRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

type systemFeatureSwitchDefinition struct {
	StorageKey  string
	Description string
}

func openSystemConfigDB() (*sql.DB, error) {
	cfg, err := config.LoadDBConfig()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func ensureSystemConfigStorage(db *sql.DB) error {
	systemConfigStorageMu.Lock()
	defer systemConfigStorageMu.Unlock()
	if systemConfigStorageReady {
		return nil
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS system_configs (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			` + "`group`" + ` VARCHAR(50) NOT NULL,
			` + "`key`" + ` VARCHAR(100) NOT NULL,
			value LONGTEXT NOT NULL,
			description VARCHAR(255) DEFAULT '',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uk_group_key (` + "`group`" + `, ` + "`key`" + `)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`); err != nil {
		return err
	}

	if _, err := db.Exec("ALTER TABLE system_configs MODIFY COLUMN value LONGTEXT NOT NULL"); err != nil {
		return err
	}

	installedAt := time.Now()
	_ = db.QueryRow("SELECT COALESCE(MIN(created_at), NOW()) FROM admins").Scan(&installedAt)
	_, err := db.Exec(`
		INSERT INTO system_configs (`+"`group`"+`, `+"`key`"+`, value, description) VALUES
			('site', 'site_name', ?, '网站名称'),
			('site', 'site_subtitle', ?, '网站副标题'),
			('site', 'site_logo', '', '网站 Logo'),
			('site', 'installed_at', ?, '系统安装时间'),
			('site', 'station_qq', '', '站长 QQ'),
			('site', 'icp_number', '', '网站备案号'),
			('site', 'domain_license_notice', '', '域名授权网站公告'),
			('site', 'registration_enabled', '1', '是否允许普通用户注册'),
			('site', 'self_purchase_enabled', '1', '是否允许用户自助购买'),
			('site', 'piracy_detection_enabled', '0', '是否启用盗版检测入库'),
			('payment', 'easypay_enabled', '0', '是否启用易支付'),
			('payment', 'easypay_gateway', '', '易支付网关地址'),
			('payment', 'easypay_pid', '', '易支付商户 PID'),
			('payment', 'easypay_key', '', '易支付商户 Key'),
			('payment', 'easypay_default_type', 'alipay', '易支付默认支付方式'),
			('payment', 'easypay_pay_types', 'alipay,wxpay,qqpay', '易支付已开启支付方式'),
			('payment', 'easypay_notify_url', '', '易支付异步通知地址'),
			('payment', 'easypay_return_url', '', '易支付同步跳转地址')
		ON DUPLICATE KEY UPDATE `+"`key`"+` = VALUES(`+"`key`"+`)
	`, defaultSiteName, defaultSiteSubtitle, installedAt.Format("2006-01-02 15:04:05"))
	if err == nil {
		systemConfigStorageReady = true
	}
	return err
}

func loadSystemConfig(db *sql.DB) (systemConfigResponse, error) {
	result := systemConfigResponse{
		SiteName:               defaultSiteName,
		SiteSubtitle:           defaultSiteSubtitle,
		RegistrationEnabled:    true,
		SelfPurchaseEnabled:    true,
		PiracyDetectionEnabled: false,
	}
	rows, err := db.Query("SELECT `key`, value FROM system_configs WHERE `group` = 'site'")
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return result, err
		}
		switch key {
		case "site_name":
			if strings.TrimSpace(value) != "" {
				result.SiteName = value
			}
		case "site_subtitle":
			result.SiteSubtitle = value
		case "site_logo":
			result.SiteLogo = value
		case "installed_at":
			result.InstalledAt = value
		case "station_qq":
			result.StationQQ = value
		case "icp_number":
			result.ICPNumber = value
		case "domain_license_notice":
			result.DomainLicenseNotice = value
		case "registration_enabled":
			result.RegistrationEnabled = value == "1"
		case "self_purchase_enabled":
			result.SelfPurchaseEnabled = value == "1"
		case "piracy_detection_enabled":
			result.PiracyDetectionEnabled = value == "1"
		}
	}
	return result, rows.Err()
}

func validateSiteLogo(dataURL string) error {
	if dataURL == "" {
		return nil
	}

	prefix, encoded, found := strings.Cut(dataURL, ",")
	if !found {
		return errors.New("Logo 数据格式不正确")
	}

	allowed := map[string]bool{
		"data:image/png;base64":  true,
		"data:image/jpeg;base64": true,
		"data:image/webp;base64": true,
	}
	if !allowed[strings.ToLower(prefix)] {
		return errors.New("Logo 仅支持 PNG、JPG 或 WebP 格式")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return errors.New("Logo 数据格式不正确")
	}
	if len(decoded) == 0 || len(decoded) > maxLogoBytes {
		return errors.New("Logo 大小必须在 512KB 以内")
	}
	return nil
}

func writeSystemConfig(c *gin.Context, status int, body gin.H) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.JSON(status, body)
}

func PublicSystemConfig(c *gin.Context) {
	fallback := systemConfigResponse{
		SiteName:               defaultSiteName,
		SiteSubtitle:           defaultSiteSubtitle,
		RegistrationEnabled:    true,
		SelfPurchaseEnabled:    true,
		PiracyDetectionEnabled: false,
	}
	db, err := openSystemConfigDB()
	if err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 200, "msg": "", "data": fallback})
		return
	}
	defer db.Close()

	if err := ensureSystemConfigStorage(db); err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 200, "msg": "", "data": fallback})
		return
	}

	result, err := loadSystemConfig(db)
	if err != nil {
		result = fallback
	}
	writeSystemConfig(c, http.StatusOK, gin.H{"code": 200, "msg": "", "data": result})
}

func AdminSystemConfig(c *gin.Context) {
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
	result, err := loadSystemConfig(db)
	if err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 500, "msg": "读取系统配置失败"})
		return
	}
	writeSystemConfig(c, http.StatusOK, gin.H{"code": 200, "msg": "", "data": result})
}

func AdminSystemConfigUpdate(c *gin.Context) {
	var req updateSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	req.SiteName = strings.TrimSpace(req.SiteName)
	req.SiteSubtitle = strings.TrimSpace(req.SiteSubtitle)
	req.StationQQ = strings.TrimSpace(req.StationQQ)
	req.ICPNumber = strings.TrimSpace(req.ICPNumber)
	req.DomainLicenseNotice = strings.TrimSpace(req.DomainLicenseNotice)
	if req.SiteName == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "网站名称不能为空"})
		return
	}
	if utf8.RuneCountInString(req.SiteName) > maxSiteNameLen {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "网站名称不能超过 60 个字符"})
		return
	}
	if utf8.RuneCountInString(req.SiteSubtitle) > maxSiteSubtitleLen {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "网站副标题不能超过 120 个字符"})
		return
	}
	if utf8.RuneCountInString(req.StationQQ) > maxStationQQlen {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "站长 QQ 不能超过 30 个字符"})
		return
	}
	if req.StationQQ != "" {
		if _, err := strconv.ParseUint(req.StationQQ, 10, 64); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "站长 QQ 只能填写数字"})
			return
		}
	}
	if utf8.RuneCountInString(req.ICPNumber) > maxICPNumberLen {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "网站备案号不能超过 100 个字符"})
		return
	}
	if utf8.RuneCountInString(req.DomainLicenseNotice) > maxSiteNoticeLen {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "域名授权网站公告不能超过 2000 个字符"})
		return
	}
	if err := validateSiteLogo(req.SiteLogo); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
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

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存系统配置失败"})
		return
	}
	defer tx.Rollback()

	items := []struct {
		group       string
		key         string
		value       string
		description string
	}{
		{group: "site", key: "site_name", value: req.SiteName, description: "网站名称"},
		{group: "site", key: "site_subtitle", value: req.SiteSubtitle, description: "网站副标题"},
		{group: "site", key: "site_logo", value: req.SiteLogo, description: "网站 Logo"},
		{group: "site", key: "station_qq", value: req.StationQQ, description: "站长 QQ"},
		{group: "site", key: "icp_number", value: req.ICPNumber, description: "网站备案号"},
		{group: "site", key: "domain_license_notice", value: req.DomainLicenseNotice, description: "域名授权网站公告"},
		{group: "site", key: "registration_enabled", value: boolConfigValue(req.RegistrationEnabled), description: "是否允许普通用户注册"},
		{group: "site", key: "self_purchase_enabled", value: boolConfigValue(req.SelfPurchaseEnabled), description: "是否允许用户自助购买"},
		{group: "site", key: "piracy_detection_enabled", value: boolConfigValue(req.PiracyDetectionEnabled), description: "是否启用盗版检测入库"},
	}
	for _, item := range items {
		if _, err := tx.Exec(`
			INSERT INTO system_configs (`+"`group`"+`, `+"`key`"+`, value, description)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE value = VALUES(value), description = VALUES(description)
		`, item.group, item.key, item.value, item.description); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存系统配置失败"})
			return
		}
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存系统配置失败"})
		return
	}

	result, err := loadSystemConfig(db)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取系统配置失败"})
		return
	}
	writeSystemConfig(c, http.StatusOK, gin.H{
		"code": 200,
		"msg":  "系统配置保存成功",
		"data": result,
	})
}

func paymentConfigFromEpayConfig(cfg epayConfig) paymentConfigResponse {
	payType, ok := normalizeEpayPayType(cfg.DefaultPayType, epayDefaultPayType)
	if !ok {
		payType = epayDefaultPayType
	}
	return paymentConfigResponse{
		EasypayEnabled:        cfg.Enabled,
		EasypayGateway:        cfg.Gateway,
		EasypayPID:            cfg.PID,
		EasypayMerchantKeySet: strings.TrimSpace(cfg.Key) != "",
		EasypayDefaultType:    payType,
		EasypayPayTypes:       cfg.PayTypes,
		EasypayNotifyURL:      cfg.NotifyURL,
		EasypayReturnURL:      cfg.ReturnURL,
	}
}

func AdminPaymentConfig(c *gin.Context) {
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
	cfg, err := loadEpayConfig(db)
	if err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 500, "msg": "读取易支付配置失败"})
		return
	}
	writeSystemConfig(c, http.StatusOK, gin.H{"code": 200, "msg": "", "data": paymentConfigFromEpayConfig(cfg)})
}

func AdminPaymentConfigUpdate(c *gin.Context) {
	var req updatePaymentConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	req.EasypayGateway = strings.TrimSpace(req.EasypayGateway)
	req.EasypayPID = strings.TrimSpace(req.EasypayPID)
	req.EasypayMerchantKey = strings.TrimSpace(req.EasypayMerchantKey)
	req.EasypayDefaultType = strings.TrimSpace(req.EasypayDefaultType)
	req.EasypayNotifyURL = strings.TrimSpace(req.EasypayNotifyURL)
	req.EasypayReturnURL = strings.TrimSpace(req.EasypayReturnURL)

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
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "启用易支付前请至少开启一种支付方式"})
		return
	}
	if len(payTypes) > 0 && !slices.Contains(payTypes, req.EasypayDefaultType) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "默认支付方式必须是已开启的支付方式"})
		return
	}
	if req.EasypayGateway != "" {
		if _, err := resolveEpayEndpoint(req.EasypayGateway); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
			return
		}
	}
	if err := validateOptionalHTTPURL(req.EasypayNotifyURL, "易支付异步通知地址"); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if err := validateOptionalHTTPURL(req.EasypayReturnURL, "易支付同步跳转地址"); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
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
	existingPayment, err := loadEpayConfig(db)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取易支付配置失败"})
		return
	}
	if req.EasypayEnabled {
		if req.EasypayGateway == "" || req.EasypayPID == "" || (req.EasypayMerchantKey == "" && existingPayment.Key == "") {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "启用易支付前请完整配置网关、PID 和商户 Key"})
			return
		}
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存易支付配置失败"})
		return
	}
	defer tx.Rollback()

	items := []struct {
		key         string
		value       string
		description string
	}{
		{key: "easypay_enabled", value: boolConfigValue(req.EasypayEnabled), description: "是否启用易支付"},
		{key: "easypay_gateway", value: req.EasypayGateway, description: "易支付网关地址"},
		{key: "easypay_pid", value: req.EasypayPID, description: "易支付商户 PID"},
		{key: "easypay_default_type", value: req.EasypayDefaultType, description: "易支付默认支付方式"},
		{key: "easypay_pay_types", value: strings.Join(payTypes, ","), description: "易支付已开启支付方式"},
		{key: "easypay_notify_url", value: req.EasypayNotifyURL, description: "易支付异步通知地址"},
		{key: "easypay_return_url", value: req.EasypayReturnURL, description: "易支付同步跳转地址"},
	}
	if req.EasypayMerchantKey != "" {
		items = append(items, struct {
			key         string
			value       string
			description string
		}{key: "easypay_key", value: req.EasypayMerchantKey, description: "易支付商户 Key"})
	}
	for _, item := range items {
		if _, err := tx.Exec(`
			INSERT INTO system_configs (`+"`group`"+`, `+"`key`"+`, value, description)
			VALUES ('payment', ?, ?, ?)
			ON DUPLICATE KEY UPDATE value = VALUES(value), description = VALUES(description)
		`, item.key, item.value, item.description); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存易支付配置失败"})
			return
		}
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存易支付配置失败"})
		return
	}

	result, err := loadEpayConfig(db)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取易支付配置失败"})
		return
	}
	writeSystemConfig(c, http.StatusOK, gin.H{
		"code": 200,
		"msg":  "易支付配置保存成功",
		"data": paymentConfigFromEpayConfig(result),
	})
}

// AdminSystemFeatureSwitchUpdate 独立保存用户端功能开关，避免覆盖其他未提交配置。
func AdminSystemFeatureSwitchUpdate(c *gin.Context) {
	definitions := map[string]systemFeatureSwitchDefinition{
		"registrationEnabled": {
			StorageKey:  "registration_enabled",
			Description: "是否允许普通用户注册",
		},
		"selfPurchaseEnabled": {
			StorageKey:  "self_purchase_enabled",
			Description: "是否允许用户自助购买",
		},
		"piracyDetectionEnabled": {
			StorageKey:  "piracy_detection_enabled",
			Description: "是否启用盗版检测入库",
		},
	}
	key := c.Param("key")
	definition, ok := definitions[key]
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "不支持的功能开关"})
		return
	}

	var req updateSystemFeatureSwitchRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
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

	if _, err := db.Exec(`
		INSERT INTO system_configs (`+"`group`"+`, `+"`key`"+`, value, description)
		VALUES ('site', ?, ?, ?)
		ON DUPLICATE KEY UPDATE value = VALUES(value), description = VALUES(description)
	`, definition.StorageKey, boolConfigValue(*req.Enabled), definition.Description); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存功能开关失败"})
		return
	}

	writeSystemConfig(c, http.StatusOK, gin.H{
		"code": 200,
		"msg":  "功能开关已更新",
		"data": gin.H{
			"key":     key,
			"enabled": *req.Enabled,
		},
	})
}

func validateOptionalHTTPURL(value string, label string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New(label + "格式不正确")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New(label + "仅支持 http 或 https")
	}
	return nil
}

func boolConfigValue(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func loadSystemFeatureSwitch(key string, defaultValue bool) bool {
	db, err := openSystemConfigDB()
	if err != nil {
		return defaultValue
	}
	defer db.Close()

	if err := ensureSystemConfigStorage(db); err != nil {
		return defaultValue
	}

	var value string
	if err := db.QueryRow("SELECT value FROM system_configs WHERE `group` = 'site' AND `key` = ?", key).Scan(&value); err != nil {
		return defaultValue
	}
	return value == "1"
}

func isRegistrationEnabled() bool {
	return loadSystemFeatureSwitch("registration_enabled", true)
}

func isSelfPurchaseEnabled() bool {
	return loadSystemFeatureSwitch("self_purchase_enabled", true)
}

func isPiracyDetectionEnabled() bool {
	return loadSystemFeatureSwitch("piracy_detection_enabled", false)
}
