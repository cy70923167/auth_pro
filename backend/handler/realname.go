package handler

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

// ========== 实名认证（支付宝 / 快瞳 / 靓仔聚合认证） ==========
//
// 支持三种服务商，管理员在系统设置「实名认证」分区切换：
//
// 支付宝（provider=alipay）：金融级实人认证，扫码刷脸。
//   - datadigital.fincloud.generalsaas.face.certify.initialize 初始化认证，生成 certify_id
//   - datadigital.fincloud.generalsaas.face.certify.query 查询认证结果
//
// 快瞳（provider=kuaitong）：支持身份证二要素核验与扫码人脸认证。
//   - GET  /s/api/getAccessToken 获取鉴权 token（服务端缓存，401/403 时主动刷新）
//   - POST /s/api/ocr/cloudCode/IdCard multipart 提交 token/idCard/realName，直接完成二要素核验
//   - POST /s/api/ocr/cloudCode/certificate multipart 提交 token/idCard/realName + 人像照片
//     （imgBase64，相似度分数 pic > 60 判定为同一人）
//   管理员选择二要素认证时，用户提交姓名+身份证号后直接核验，无需扫码；选择人脸认证时，
//   生成认证单与二维码，用户扫码打开本站拍照页 /realname-face?t=token 自动抓拍并完成比对。
//
// 靓仔聚合认证（provider=tencent）：生成本站拍照单，用户扫码打开本站拍照页
// /realname-face?t=token 自动抓拍，提交时调用靓仔 /verify 接口传入姓名、身份证和人脸图片完成核验。
//
// 管理员可按应用勾选哪些应用强制要求实名；开启后对应应用的授权校验接口
// 会对未实名用户返回 realname_required，拒绝安装。

const (
	realnameGroup              = "realname"
	realnameProviderAlipay     = "alipay"
	realnameProviderKuaitong   = "kuaitong"
	realnameProviderTencent    = "tencent"
	kuaitongAuthTypeFace       = "face"
	kuaitongAuthTypeTwoElement = "two_element"
	alipayDefaultGateway       = "https://openapi.alipay.com/gateway.do"
	alipaySandboxGateway       = "https://openapi-sandbox.dl.alipaydev.com/gateway.do"
	realnameInitMethod         = "datadigital.fincloud.generalsaas.face.certify.initialize"
	realnameQueryMethod        = "datadigital.fincloud.generalsaas.face.certify.query"
	realnameH5Prefix           = "https://certifyweb.alipay.com/certify/h5?certifyId="
	kuaitongTokenURL           = "https://ai.inspirvision.cn/s/api/getAccessToken"
	kuaitongVerifyURL          = "https://ai.inspirvision.cn/s/api/ocr/cloudCode/certificate"
	kuaitongIDCardURL          = "https://ai.inspirvision.cn/s/api/ocr/cloudCode/IdCard"
	tencentRealnameBaseURL     = "https://real.4775.cn/common/openapi"
	tencentRealnameProduct     = "cloud_tencent_renlian_zq"
	kuaitongTokenRefreshSkew   = time.Hour // token 剩余有效期不足该值时提前刷新
	kuaitongPassScore          = 60        // 快瞳人证合一判定阈值：相似度分数 > 60 为同一人
	maxRealNameLen             = 30
	realnameHTTPTimeout        = 10 * time.Second
	tencentRealnameHTTPTimeout = 30 * time.Second
	realnameCertifyIDMaxKeep   = 24 * time.Hour
)

type realnameConfig struct {
	Enabled            bool
	PluginEnabled      bool // 当前 provider 对应的插件是否在应用商店中启用
	Provider           string
	AppID              string
	PrivateKey         string
	AlipayPublicKey    string
	Gateway            string
	KuaitongAccessKey  string
	KuaitongSecret     string
	KuaitongAuthType   string
	TencentAPIKey      string
	TencentAPISecret   string
	TencentBaseURL     string
	TencentProductCode string
	TencentUsePackage  bool
	XiaomuAppKey       string
	XiaomuAppSecret    string
	XiaomuBaseURL      string
	XiaomuProductMode  string
	RequireAppIDs      map[int64]bool
}

type realnameConfigResponse struct {
	Enabled            bool              `json:"enabled"`
	PluginEnabled      bool              `json:"pluginEnabled"`
	Provider           string            `json:"provider"`
	AppID              string            `json:"appId"`
	PrivateKeySet      bool              `json:"privateKeySet"`
	AlipayPublicKey    string            `json:"alipayPublicKey"`
	Gateway            string            `json:"gateway"`
	KuaitongAccessKey  string            `json:"kuaitongAccessKey"`
	KuaitongSecretSet  bool              `json:"kuaitongSecretSet"`
	KuaitongAuthType   string            `json:"kuaitongAuthType"`
	TencentAPIKey      string            `json:"tencentApiKey"`
	TencentSecretSet   bool              `json:"tencentSecretSet"`
	TencentBaseURL     string            `json:"tencentBaseUrl"`
	TencentUsePackage  bool              `json:"tencentUsePackage"`
	TencentProductCode string            `json:"tencentProductCode"`
	XiaomuAppKey       string            `json:"xiaomuAppKey"`
	XiaomuSecretSet    bool              `json:"xiaomuSecretSet"`
	XiaomuBaseURL      string            `json:"xiaomuBaseUrl"`
	XiaomuProductMode  string            `json:"xiaomuProductMode"`
	RequireAppIDs      []int64           `json:"requireAppIds"`
	Apps               []realnameAppItem `json:"apps"`
}

type realnameAppItem struct {
	ID      int64  `json:"id"`
	AppName string `json:"appName"`
	AppKey  string `json:"appKey"`
}

type updateRealnameConfigRequest struct {
	Enabled           bool    `json:"enabled"`
	Provider          string  `json:"provider"`
	AppID             string  `json:"appId"`
	PrivateKey        string  `json:"privateKey"`
	AlipayPublicKey   string  `json:"alipayPublicKey"`
	Gateway           string  `json:"gateway"`
	KuaitongAccessKey string  `json:"kuaitongAccessKey"`
	KuaitongSecret    string  `json:"kuaitongSecret"`
	KuaitongAuthType  string  `json:"kuaitongAuthType"`
	TencentAPIKey     string  `json:"tencentApiKey"`
	TencentAPISecret  string  `json:"tencentApiSecret"`
	TencentBaseURL    string  `json:"tencentBaseUrl"`
	TencentUsePackage *bool   `json:"tencentUsePackage"`
	XiaomuAppKey      string  `json:"xiaomuAppKey"`
	XiaomuAppSecret   string  `json:"xiaomuAppSecret"`
	XiaomuBaseURL     string  `json:"xiaomuBaseUrl"`
	XiaomuProductMode string  `json:"xiaomuProductMode"`
	RequireAppIDs     []int64 `json:"requireAppIds"`
}

// ensureRealnameStorage 幂等建表/建列：
//   - users / agents 表加实名相关字段（姓名与身份证号明文存储，展示层脱敏）
//   - realname_records 认证记录表，每次认证成功/失败留档
func ensureRealnameStorage(db *sql.DB) error {
	columns := []struct {
		table string
		name  string
		ddl   string
	}{
		{"users", "real_name", "ALTER TABLE users ADD COLUMN real_name VARCHAR(60) DEFAULT '' COMMENT '实名姓名' AFTER nickname"},
		{"users", "real_id_card", "ALTER TABLE users ADD COLUMN real_id_card VARCHAR(30) DEFAULT '' COMMENT '身份证号' AFTER real_name"},
		{"users", "realname_at", "ALTER TABLE users ADD COLUMN realname_at DATETIME DEFAULT NULL COMMENT '实名认证时间' AFTER real_id_card"},
		{"agents", "real_name", "ALTER TABLE agents ADD COLUMN real_name VARCHAR(60) DEFAULT '' COMMENT '实名姓名'"},
		{"agents", "real_id_card", "ALTER TABLE agents ADD COLUMN real_id_card VARCHAR(30) DEFAULT '' COMMENT '身份证号'"},
		{"agents", "realname_at", "ALTER TABLE agents ADD COLUMN realname_at DATETIME DEFAULT NULL COMMENT '实名认证时间'"},
	}
	for _, col := range columns {
		var count int
		if err := db.QueryRow(`
			SELECT COUNT(*) FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?
		`, col.table, col.name).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if _, err := db.Exec(col.ddl); err != nil {
			return err
		}
	}

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS realname_records (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
		owner_type VARCHAR(20) NOT NULL DEFAULT 'user' COMMENT '主体类型(user/agent)',
		owner_id BIGINT UNSIGNED NOT NULL COMMENT '主体ID',
		provider VARCHAR(20) NOT NULL DEFAULT '' COMMENT '服务商(alipay/kuaitong/tencent)',
		real_name VARCHAR(60) NOT NULL DEFAULT '' COMMENT '实名姓名',
		id_card VARCHAR(30) NOT NULL DEFAULT '' COMMENT '身份证号',
		status VARCHAR(20) NOT NULL DEFAULT '' COMMENT '结果(passed/failed)',
		fail_reason TEXT NOT NULL COMMENT '详细失败原因',
		serial_no VARCHAR(64) NOT NULL DEFAULT '' COMMENT '服务商流水号',
		score VARCHAR(20) NOT NULL DEFAULT '' COMMENT '人脸相似度分数',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
		PRIMARY KEY (id),
		KEY idx_owner (owner_type, owner_id),
		KEY idx_created_at (created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='实名认证记录表'`)
	if err != nil {
		return err
	}

	var failReasonLength sql.NullInt64
	if err := db.QueryRow(`
		SELECT CHARACTER_MAXIMUM_LENGTH FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'realname_records' AND COLUMN_NAME = 'fail_reason'
	`).Scan(&failReasonLength); err != nil {
		return err
	}
	if !failReasonLength.Valid || failReasonLength.Int64 < 65535 {
		_, err = db.Exec(`ALTER TABLE realname_records
			MODIFY COLUMN fail_reason TEXT NOT NULL COMMENT '详细失败原因'`)
		return err
	}
	return nil
}

// writeRealnameRecord 写入一条实名认证记录（姓名/身份证明文留档）。
func writeRealnameRecord(db *sql.DB, ownerType string, ownerID int64, provider, realName, idCard, status, failReason, serialNo, score string) {
	_, _ = db.Exec(`INSERT INTO realname_records (owner_type, owner_id, provider, real_name, id_card, status, fail_reason, serial_no, score)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ownerType, ownerID, provider, realName, idCard, status, failReason, serialNo, score)
}

func validKuaitongAuthType(value string) bool {
	return value == kuaitongAuthTypeFace || value == kuaitongAuthTypeTwoElement
}

func loadRealnameConfig(db *sql.DB) (realnameConfig, error) {
	cfg := realnameConfig{
		Provider: realnameProviderAlipay, Gateway: alipayDefaultGateway,
		KuaitongAuthType: kuaitongAuthTypeFace,
		TencentBaseURL:   tencentRealnameBaseURL, TencentProductCode: tencentRealnameProduct, TencentUsePackage: true,
		XiaomuBaseURL: xiaomuRealnameBaseURL, XiaomuProductMode: xiaomuModeThreeElement,
		RequireAppIDs: map[int64]bool{},
	}
	rows, err := db.Query("SELECT `key`, value FROM system_configs WHERE `group` = ?", realnameGroup)
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
		case "enabled":
			cfg.Enabled = value == "1"
		case "provider":
			if v := strings.TrimSpace(value); v == realnameProviderAlipay || v == realnameProviderKuaitong || v == realnameProviderTencent || v == realnameProviderXiaomu {
				cfg.Provider = v
			}
		case "app_id":
			cfg.AppID = strings.TrimSpace(value)
		case "private_key":
			cfg.PrivateKey = strings.TrimSpace(value)
		case "alipay_public_key":
			cfg.AlipayPublicKey = strings.TrimSpace(value)
		case "gateway":
			if v := strings.TrimSpace(value); v != "" {
				cfg.Gateway = v
			}
		case "kuaitong_access_key":
			cfg.KuaitongAccessKey = strings.TrimSpace(value)
		case "kuaitong_secret":
			cfg.KuaitongSecret = strings.TrimSpace(value)
		case "kuaitong_auth_type":
			if v := strings.TrimSpace(value); validKuaitongAuthType(v) {
				cfg.KuaitongAuthType = v
			}
		case "tencent_api_key":
			cfg.TencentAPIKey = strings.TrimSpace(value)
		case "tencent_api_secret":
			cfg.TencentAPISecret = strings.TrimSpace(value)
		case "tencent_base_url":
			if v := strings.TrimSpace(value); v != "" {
				cfg.TencentBaseURL = strings.TrimRight(v, "/")
			}
		case "tencent_product_code":
			if v := strings.TrimSpace(value); v != "" {
				cfg.TencentProductCode = v
			}
		case "tencent_use_package":
			cfg.TencentUsePackage = value == "1"
		case "xiaomu_app_key":
			cfg.XiaomuAppKey = strings.TrimSpace(value)
		case "xiaomu_app_secret":
			cfg.XiaomuAppSecret = strings.TrimSpace(value)
		case "xiaomu_base_url":
			if v := strings.TrimSpace(value); v != "" {
				cfg.XiaomuBaseURL = strings.TrimRight(v, "/")
			}
		case "xiaomu_product_mode":
			if v := strings.TrimSpace(value); validXiaomuProductMode(v) {
				cfg.XiaomuProductMode = v
			}
		case "require_app_ids":
			for _, part := range strings.Split(value, ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				if id, err := strconv.ParseInt(part, 10, 64); err == nil && id > 0 {
					cfg.RequireAppIDs[id] = true
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return cfg, err
	}

	// 插件中心联动：当前 provider 的插件未启用时，回落到已启用的实名插件。
	cfg.PluginEnabled = isPluginEnabled(db, realnamePluginIDByProvider(cfg.Provider))
	if !cfg.PluginEnabled {
		fallback := ""
		if isPluginEnabled(db, "alipay-realname") {
			fallback = realnameProviderAlipay
		} else if isPluginEnabled(db, "kuaitong-realname") {
			fallback = realnameProviderKuaitong
		} else if isPluginEnabled(db, "tencent-realname") {
			fallback = realnameProviderTencent
		} else if isPluginEnabled(db, "xiaomu-realname") {
			fallback = realnameProviderXiaomu
		}
		if fallback != "" && fallback != cfg.Provider {
			cfg.Provider = fallback
			cfg.PluginEnabled = true
		}
	}
	return cfg, nil
}

func loadRealnameApps(db *sql.DB) ([]realnameAppItem, error) {
	rows, err := db.Query("SELECT id, app_name, app_key FROM apps ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	apps := make([]realnameAppItem, 0)
	for rows.Next() {
		var item realnameAppItem
		if err := rows.Scan(&item.ID, &item.AppName, &item.AppKey); err != nil {
			return nil, err
		}
		apps = append(apps, item)
	}
	return apps, rows.Err()
}

// AdminRealnameConfig 读取实名认证配置（私钥不回显）。
func AdminRealnameConfig(c *gin.Context) {
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

	cfg, err := loadRealnameConfig(db)
	if err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 500, "msg": "读取实名认证配置失败"})
		return
	}
	apps, err := loadRealnameApps(db)
	if err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 500, "msg": "读取应用列表失败"})
		return
	}

	ids := make([]int64, 0, len(cfg.RequireAppIDs))
	for id := range cfg.RequireAppIDs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	writeSystemConfig(c, http.StatusOK, gin.H{"code": 200, "msg": "", "data": realnameConfigResponse{
		Enabled:            cfg.Enabled,
		PluginEnabled:      cfg.PluginEnabled,
		Provider:           cfg.Provider,
		AppID:              cfg.AppID,
		PrivateKeySet:      cfg.PrivateKey != "",
		AlipayPublicKey:    cfg.AlipayPublicKey,
		Gateway:            cfg.Gateway,
		KuaitongAccessKey:  cfg.KuaitongAccessKey,
		KuaitongSecretSet:  cfg.KuaitongSecret != "",
		KuaitongAuthType:   cfg.KuaitongAuthType,
		TencentAPIKey:      cfg.TencentAPIKey,
		TencentSecretSet:   cfg.TencentAPISecret != "",
		TencentBaseURL:     cfg.TencentBaseURL,
		TencentUsePackage:  cfg.TencentUsePackage,
		TencentProductCode: cfg.TencentProductCode,
		XiaomuAppKey:       cfg.XiaomuAppKey,
		XiaomuSecretSet:    cfg.XiaomuAppSecret != "",
		XiaomuBaseURL:      cfg.XiaomuBaseURL,
		XiaomuProductMode:  cfg.XiaomuProductMode,
		RequireAppIDs:      ids,
		Apps:               apps,
	}})
}

// AdminRealnameConfigUpdate 保存实名认证配置。
func AdminRealnameConfigUpdate(c *gin.Context) {
	var req updateRealnameConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	req.AppID = strings.TrimSpace(req.AppID)
	req.PrivateKey = strings.TrimSpace(req.PrivateKey)
	req.AlipayPublicKey = strings.TrimSpace(req.AlipayPublicKey)
	req.Gateway = strings.TrimSpace(req.Gateway)
	req.KuaitongAccessKey = strings.TrimSpace(req.KuaitongAccessKey)
	req.KuaitongSecret = strings.TrimSpace(req.KuaitongSecret)
	req.KuaitongAuthType = strings.TrimSpace(req.KuaitongAuthType)
	req.TencentAPIKey = strings.TrimSpace(req.TencentAPIKey)
	req.TencentAPISecret = strings.TrimSpace(req.TencentAPISecret)
	req.TencentBaseURL = strings.TrimRight(strings.TrimSpace(req.TencentBaseURL), "/")
	req.XiaomuAppKey = strings.TrimSpace(req.XiaomuAppKey)
	req.XiaomuAppSecret = strings.TrimSpace(req.XiaomuAppSecret)
	req.XiaomuBaseURL = strings.TrimRight(strings.TrimSpace(req.XiaomuBaseURL), "/")
	req.XiaomuProductMode = strings.TrimSpace(req.XiaomuProductMode)
	if req.Provider != realnameProviderKuaitong && req.Provider != realnameProviderTencent && req.Provider != realnameProviderXiaomu {
		req.Provider = realnameProviderAlipay
	}
	if req.Provider == realnameProviderAlipay {
		if req.Gateway == "" {
			req.Gateway = alipayDefaultGateway
		}
		if _, err := url.ParseRequestURI(req.Gateway); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "网关地址格式不正确"})
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

	oldCfg, err := loadRealnameConfig(db)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取实名认证配置失败"})
		return
	}
	// 服务商只能在应用商店启用对应插件后变更
	if !oldCfg.PluginEnabled {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请先在应用商店启用一个实名认证服务商插件"})
		return
	}
	req.Provider = oldCfg.Provider
	if req.KuaitongAuthType == "" {
		req.KuaitongAuthType = oldCfg.KuaitongAuthType
	}
	if !validKuaitongAuthType(req.KuaitongAuthType) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "快瞳认证类型不正确"})
		return
	}
	if req.TencentBaseURL == "" {
		req.TencentBaseURL = oldCfg.TencentBaseURL
	}
	if req.XiaomuBaseURL == "" {
		req.XiaomuBaseURL = oldCfg.XiaomuBaseURL
	}
	if req.XiaomuProductMode == "" {
		req.XiaomuProductMode = oldCfg.XiaomuProductMode
	}
	if !validXiaomuProductMode(req.XiaomuProductMode) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "小沐聚合实名认证产品不正确"})
		return
	}

	privateKey := req.PrivateKey
	if privateKey == "" {
		privateKey = oldCfg.PrivateKey // 未提交新私钥时保留旧的
	}
	kuaitongSecret := req.KuaitongSecret
	if kuaitongSecret == "" {
		kuaitongSecret = oldCfg.KuaitongSecret // 未提交新 secret 时保留旧的
	}
	tencentSecret := req.TencentAPISecret
	if tencentSecret == "" {
		tencentSecret = oldCfg.TencentAPISecret
	}
	tencentUsePackage := oldCfg.TencentUsePackage
	if req.TencentUsePackage != nil {
		tencentUsePackage = *req.TencentUsePackage
	}
	xiaomuSecret := req.XiaomuAppSecret
	if xiaomuSecret == "" {
		xiaomuSecret = oldCfg.XiaomuAppSecret
	}

	if req.Enabled && req.Provider == realnameProviderAlipay {
		if req.AppID == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "开启实名认证前请填写支付宝应用 AppID"})
			return
		}
		if privateKey == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "开启实名认证前请填写应用私钥"})
			return
		}
		if req.AlipayPublicKey == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "开启实名认证前请填写支付宝公钥"})
			return
		}
		if _, err := parseAlipayPrivateKey(privateKey); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "应用私钥格式不正确：" + err.Error()})
			return
		}
	}
	if req.Enabled && req.Provider == realnameProviderKuaitong {
		if req.KuaitongAccessKey == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "开启实名认证前请填写快瞳 accessKey"})
			return
		}
		if kuaitongSecret == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "开启实名认证前请填写快瞳 accessSecret"})
			return
		}
	}
	if req.Enabled && req.Provider == realnameProviderTencent {
		if req.TencentAPIKey == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "开启靓仔聚合认证前请填写 API Key"})
			return
		}
		if tencentSecret == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "开启靓仔聚合认证前请填写 API Secret"})
			return
		}
		parsed, err := url.ParseRequestURI(req.TencentBaseURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "腾讯实名认证接口地址必须是有效的 HTTPS 地址"})
			return
		}
	}
	if req.Enabled && req.Provider == realnameProviderXiaomu {
		if req.XiaomuAppKey == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "开启小沐聚合实名前请填写 AppKey"})
			return
		}
		if xiaomuSecret == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "开启小沐聚合实名前请填写 AppSecret"})
			return
		}
		parsed, err := url.ParseRequestURI(req.XiaomuBaseURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "小沐聚合实名接口地址必须是有效的 HTTPS 地址"})
			return
		}
	}

	// 校验应用 id 真实存在
	validIDs := map[int64]bool{}
	if apps, err := loadRealnameApps(db); err == nil {
		for _, app := range apps {
			validIDs[app.ID] = true
		}
	}
	ids := make([]int64, 0, len(req.RequireAppIDs))
	seen := map[int64]bool{}
	for _, id := range req.RequireAppIDs {
		if id <= 0 || seen[id] || !validIDs[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	idStrs := make([]string, len(ids))
	for i, id := range ids {
		idStrs[i] = strconv.FormatInt(id, 10)
	}

	items := []struct{ key, value, desc string }{
		{"enabled", boolConfigValue(req.Enabled), "是否启用实名认证"},
		{"provider", req.Provider, "实名认证服务商(alipay/kuaitong/tencent/xiaomu)"},
		{"app_id", req.AppID, "支付宝开放平台应用 AppID"},
		{"private_key", privateKey, "支付宝应用私钥(RSA2)"},
		{"alipay_public_key", req.AlipayPublicKey, "支付宝公钥"},
		{"gateway", req.Gateway, "支付宝网关地址"},
		{"kuaitong_access_key", req.KuaitongAccessKey, "快瞳 accessKey"},
		{"kuaitong_secret", kuaitongSecret, "快瞳 accessSecret"},
		{"kuaitong_auth_type", req.KuaitongAuthType, "快瞳认证类型(face/two_element)"},
		{"tencent_api_key", req.TencentAPIKey, "腾讯增强人脸开放 API Key"},
		{"tencent_api_secret", tencentSecret, "腾讯增强人脸开放 API Secret"},
		{"tencent_base_url", req.TencentBaseURL, "靓仔聚合认证开放 API Base URL"},
		{"tencent_product_code", tencentRealnameProduct, "靓仔聚合认证产品编码"},
		{"tencent_use_package", boolConfigValue(tencentUsePackage), "靓仔聚合认证是否优先使用套餐"},
		{"xiaomu_app_key", req.XiaomuAppKey, "小沐聚合实名 AppKey"},
		{"xiaomu_app_secret", xiaomuSecret, "小沐聚合实名 AppSecret"},
		{"xiaomu_base_url", req.XiaomuBaseURL, "小沐聚合实名开放 API Base URL"},
		{"xiaomu_product_mode", req.XiaomuProductMode, "小沐聚合实名认证产品(three_element/face_h5/tencent_h5)"},
		{"require_app_ids", strings.Join(idStrs, ","), "强制实名认证的应用ID列表"},
	}
	for _, item := range items {
		if _, err := db.Exec(`
			INSERT INTO system_configs (`+"`group`"+`, `+"`key`"+`, value, description)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE value = VALUES(value), description = VALUES(description)
		`, realnameGroup, item.key, item.value, item.desc); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存实名认证配置失败"})
			return
		}
	}

	writeSystemConfig(c, http.StatusOK, gin.H{"code": 200, "msg": "实名认证配置已保存", "data": realnameConfigResponse{
		Enabled:            req.Enabled,
		PluginEnabled:      oldCfg.PluginEnabled,
		Provider:           req.Provider,
		AppID:              req.AppID,
		PrivateKeySet:      privateKey != "",
		AlipayPublicKey:    req.AlipayPublicKey,
		Gateway:            req.Gateway,
		KuaitongAccessKey:  req.KuaitongAccessKey,
		KuaitongSecretSet:  kuaitongSecret != "",
		KuaitongAuthType:   req.KuaitongAuthType,
		TencentAPIKey:      req.TencentAPIKey,
		TencentSecretSet:   tencentSecret != "",
		TencentBaseURL:     req.TencentBaseURL,
		TencentUsePackage:  tencentUsePackage,
		TencentProductCode: tencentRealnameProduct,
		XiaomuAppKey:       req.XiaomuAppKey,
		XiaomuSecretSet:    xiaomuSecret != "",
		XiaomuBaseURL:      req.XiaomuBaseURL,
		XiaomuProductMode:  req.XiaomuProductMode,
		RequireAppIDs:      ids,
	}})
}

// ========== 用户端 / 代理端实名 ==========

type userRealnameInitRequest struct {
	RealName string `json:"realName" binding:"required"`
	IDCard   string `json:"idCard" binding:"required"`
	// Mobile 仅小沐三要素模式使用：认证时临时填写，不写入 users / agents。
	Mobile string `json:"mobile"`
}

type realnamePendingOrder struct {
	OwnerType string `json:"ownerType"`
	OwnerID   uint   `json:"ownerId"`
	CertifyID string `json:"certifyId"`
	RealName  string `json:"realName"`
	IDCard    string `json:"idCard"`
}

func realnameOwnerTable(ownerType string) string {
	if ownerType == "agent" {
		return "agents"
	}
	return "users"
}

func requireRealnameOwner(c *gin.Context, expectedRole string) (uint, bool) {
	role, _ := c.Get("role")
	if role != expectedRole {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "无权限"})
		return 0, false
	}
	ownerID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "认证信息缺失"})
		return 0, false
	}
	id, ok := ownerID.(uint)
	if !ok || id == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "认证信息异常"})
		return 0, false
	}
	return id, true
}

func encodeRealnamePendingOrder(order realnamePendingOrder) string {
	raw, _ := json.Marshal(order)
	return string(raw)
}

func parseRealnamePendingOrder(raw string) (realnamePendingOrder, error) {
	var order realnamePendingOrder
	if json.Unmarshal([]byte(raw), &order) == nil && order.OwnerID > 0 && order.CertifyID != "" {
		if order.OwnerType != "agent" {
			order.OwnerType = "user"
		}
		return order, nil
	}

	// Backward compatibility for pending user orders created before ownerType was stored.
	parts := strings.Split(raw, "|")
	if len(parts) < 2 {
		return realnamePendingOrder{}, errors.New("认证单数据损坏")
	}
	uid, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || uid == 0 {
		return realnamePendingOrder{}, errors.New("认证单数据损坏")
	}
	order = realnamePendingOrder{OwnerType: "user", OwnerID: uint(uid), CertifyID: parts[1]}
	if len(parts) >= 4 {
		order.RealName, order.IDCard = parts[2], parts[3]
	}
	return order, nil
}

// UserRealnameInit 用户提交姓名+身份证号，生成实名认证单。
func UserRealnameInit(c *gin.Context) {
	userID, ok := requireRealnameOwner(c, "user")
	if !ok {
		return
	}
	realnameInit(c, "user", userID)
}

// AgentRealnameInit 代理商提交姓名+身份证号，生成实名认证单。
func AgentRealnameInit(c *gin.Context) {
	agentID, ok := requireRealnameOwner(c, "agent")
	if !ok {
		return
	}
	realnameInit(c, "agent", agentID)
}

func realnameInit(c *gin.Context, ownerType string, ownerID uint) {

	var req userRealnameInitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	req.RealName = strings.TrimSpace(req.RealName)
	req.IDCard = strings.ToUpper(strings.TrimSpace(req.IDCard))

	if utf8.RuneCountInString(req.RealName) < 2 || utf8.RuneCountInString(req.RealName) > maxRealNameLen {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请输入 2-30 个字符的真实姓名"})
		return
	}
	if !isValidIDCard(req.IDCard) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "身份证号格式不正确"})
		return
	}

	cfg, err := config.LoadDBConfig()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "系统未配置"})
		return
	}
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	if err := ensureSystemConfigStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化配置失败"})
		return
	}
	if err := ensureRealnameStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化实名存储失败"})
		return
	}

	rnCfg, err := loadRealnameConfig(db)
	if err != nil || !rnCfg.Enabled {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "管理员尚未开启实名认证"})
		return
	}
	if !rnCfg.PluginEnabled {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "实名认证服务商未启用，请联系管理员"})
		return
	}

	// 已实名直接返回
	var existingName string
	var existingAt sql.NullTime
	_ = db.QueryRow("SELECT real_name, realname_at FROM "+realnameOwnerTable(ownerType)+" WHERE id = ?", ownerID).Scan(&existingName, &existingAt)
	if existingName != "" && existingAt.Valid {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "您已完成实名认证，无需重复认证"})
		return
	}

	// 小沐聚合实名：三要素同步核验，人脸 / 微信模式跳转上游 H5 后轮询。
	if rnCfg.Provider == realnameProviderXiaomu {
		startXiaomuRealname(c, db, rnCfg, ownerType, ownerID, req)
		return
	}

	// 靓仔聚合认证生成本站拍照单，用户扫码后在本站拍照，提交时调用靓仔 API。
	if rnCfg.Provider == realnameProviderTencent {
		if rnCfg.TencentAPIKey == "" || rnCfg.TencentAPISecret == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "管理员尚未完成靓仔聚合认证配置"})
			return
		}
		token, terr := newRealnameFaceToken()
		if terr != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建认证单失败"})
			return
		}
		expiresAt := time.Now().Add(realnameFaceSessionTTL).Unix()
		value := realnameFaceSession{
			Provider:  rnCfg.Provider,
			OwnerType: ownerType,
			UserID:    int64(ownerID),
			RealName:  req.RealName,
			IDCard:    req.IDCard,
			ReturnURL: buildFrontendReturnURL(c, "", "/realname-face?t="+url.QueryEscape(token)),
			Status:    realnameFaceStatusPending,
			ExpireAt:  time.Unix(expiresAt, 0),
		}.encode()
		if _, err := db.Exec(`
			INSERT INTO system_configs (`+"`group`"+`, `+"`key`"+`, value, description)
			VALUES (?, ?, ?, '实名认证拍照单')
			ON DUPLICATE KEY UPDATE value = VALUES(value)
		`, realnameGroup, "kt_face_"+token, value); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建认证单失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "请使用微信或支付宝扫码完成人脸核验",
			"data": gin.H{"status": "pending", "provider": rnCfg.Provider, "faceToken": token},
		})
		return
	}

	// 快瞳二要素直接核验；人脸模式继续生成本站扫码拍照单。
	if rnCfg.Provider == realnameProviderKuaitong {
		if rnCfg.KuaitongAccessKey == "" || rnCfg.KuaitongSecret == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "管理员尚未完成快瞳实名认证配置"})
			return
		}
		if rnCfg.KuaitongAuthType == kuaitongAuthTypeTwoElement {
			completeKuaitongTwoElement(c, db, rnCfg, ownerType, ownerID, req)
			return
		}
		token, terr := newRealnameFaceToken()
		if terr != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建认证单失败"})
			return
		}
		expiresAt := time.Now().Add(realnameFaceSessionTTL).Unix()
		value := realnameFaceSession{
			Provider:  rnCfg.Provider,
			OwnerType: ownerType,
			UserID:    int64(ownerID),
			RealName:  req.RealName,
			IDCard:    req.IDCard,
			ReturnURL: buildFrontendReturnURL(c, "", "/realname-face?t="+url.QueryEscape(token)),
			Status:    realnameFaceStatusPending,
			ExpireAt:  time.Unix(expiresAt, 0),
		}.encode()
		if _, err := db.Exec(`
			INSERT INTO system_configs (`+"`group`"+`, `+"`key`"+`, value, description)
			VALUES (?, ?, ?, '实名认证拍照单')
			ON DUPLICATE KEY UPDATE value = VALUES(value)
		`, realnameGroup, "kt_face_"+token, value); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建认证单失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "请使用微信或支付宝扫码完成人脸核验",
			"data": gin.H{"status": "pending", "provider": rnCfg.Provider, "faceToken": token},
		})
		return
	}

	if rnCfg.AppID == "" || rnCfg.PrivateKey == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "管理员尚未完成支付宝实名认证配置"})
		return
	}

	ownerPrefix := "U"
	if ownerType == "agent" {
		ownerPrefix = "A"
	}
	outerOrderNo := fmt.Sprintf("RN%s%d%d", ownerPrefix, ownerID, time.Now().Unix())

	initResp, err := alipayRealnameInitialize(rnCfg, outerOrderNo, req.RealName, req.IDCard)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化实名认证失败：" + err.Error()})
		return
	}

	// 认证通过后姓名、身份证明文写入主体表与认证记录表；接口展示时脱敏。
	_, _ = db.Exec(`
		INSERT INTO system_configs (`+"`group`"+`, `+"`key`"+`, value, description)
		VALUES (?, ?, ?, '实名认证临时单')
		ON DUPLICATE KEY UPDATE value = VALUES(value)
	`, realnameGroup, "pending_"+outerOrderNo, encodeRealnamePendingOrder(realnamePendingOrder{
		OwnerType: ownerType,
		OwnerID:   ownerID,
		CertifyID: initResp.CertifyID,
		RealName:  req.RealName,
		IDCard:    req.IDCard,
	}))

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "请使用支付宝扫码或跳转完成认证",
		"data": gin.H{
			"certifyId":    initResp.CertifyID,
			"outerOrderNo": outerOrderNo,
			"certifyUrl":   realnameH5Prefix + url.QueryEscape(initResp.CertifyID),
		},
	})
}

// UserRealnameQuery 用户轮询认证结果；认证成功后写入实名信息。
func UserRealnameQuery(c *gin.Context) {
	userID, ok := requireRealnameOwner(c, "user")
	if !ok {
		return
	}
	realnameQuery(c, "user", userID)
}

// AgentRealnameQuery 代理商轮询认证结果；认证成功后写入代理商实名信息。
func AgentRealnameQuery(c *gin.Context) {
	agentID, ok := requireRealnameOwner(c, "agent")
	if !ok {
		return
	}
	realnameQuery(c, "agent", agentID)
}

func realnameQuery(c *gin.Context, ownerType string, ownerID uint) {
	outerOrderNo := strings.TrimSpace(c.Query("outerOrderNo"))
	certifyID := strings.TrimSpace(c.Query("certifyId"))
	faceToken := strings.TrimSpace(c.Query("faceToken"))
	if certifyID == "" && faceToken == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "缺少认证参数"})
		return
	}

	cfg, err := config.LoadDBConfig()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "系统未配置"})
		return
	}
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	if err := ensureSystemConfigStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化配置失败"})
		return
	}
	if err := ensureRealnameStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化实名存储失败"})
		return
	}

	rnCfg, err := loadRealnameConfig(db)
	if err != nil || !rnCfg.Enabled {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "管理员尚未开启实名认证"})
		return
	}
	if rnCfg.Provider == realnameProviderXiaomu {
		xiaomuRealnameQuery(c, db, rnCfg, ownerType, ownerID)
		return
	}
	if rnCfg.Provider == realnameProviderKuaitong || rnCfg.Provider == realnameProviderTencent {
		realnameFaceQuery(c, db, rnCfg, ownerType, ownerID)
		return
	}

	// 防串号：支付宝认证单必须同时匹配当前角色、当前主体和 certifyId。
	if outerOrderNo == "" || certifyID == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "缺少认证参数"})
		return
	}
	var stored string
	err = db.QueryRow("SELECT value FROM system_configs WHERE `group` = ? AND `key` = ?", realnameGroup, "pending_"+outerOrderNo).Scan(&stored)
	pending, parseErr := parseRealnamePendingOrder(stored)
	if err != nil || parseErr != nil || pending.OwnerType != ownerType || pending.OwnerID != ownerID || pending.CertifyID != certifyID {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "认证单号不匹配"})
		return
	}

	queryResp, err := alipayRealnameQuery(rnCfg, certifyID, outerOrderNo)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "查询失败", "data": gin.H{"status": "pending"}})
		return
	}

	if queryResp.Passed != "T" {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "认证未完成", "data": gin.H{"status": "pending"}})
		return
	}

	// 认证通过：优先用支付宝回传的明文姓名/证件号，未回传时取 init 时用户提交的值。
	// 数据库明文存储（audit 留档），接口响应统一脱敏。
	realName := queryResp.IdentityInfo.CertName
	idCard := strings.ToUpper(queryResp.IdentityInfo.CertNo)
	if realName == "" || idCard == "" {
		realName, idCard = pending.RealName, pending.IDCard
	}
	if realName == "" || idCard == "" {
		realName = "已认证"
		idCard = "已认证"
	}
	_, _ = db.Exec("UPDATE "+realnameOwnerTable(ownerType)+" SET real_name = ?, real_id_card = ?, realname_at = NOW() WHERE id = ?", realName, idCard, ownerID)
	writeRealnameRecord(db, ownerType, int64(ownerID), realnameProviderAlipay, realName, idCard, realnameFaceStatusPassed, "", certifyID, "")

	if outerOrderNo != "" {
		_, _ = db.Exec("DELETE FROM system_configs WHERE `group` = ? AND `key` = ?", realnameGroup, "pending_"+outerOrderNo)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "实名认证成功",
		"data": gin.H{"status": "passed", "realName": maskRealName(realName), "idCard": maskIDCard(idCard)},
	})
}

func completeKuaitongTwoElement(c *gin.Context, db *sql.DB, cfg realnameConfig, ownerType string, ownerID uint, req userRealnameInitRequest) {
	message, serialNo, err := kuaitongVerifyIDCard(cfg, req.RealName, req.IDCard)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "核验请求失败：" + err.Error()})
		return
	}
	if message != "" {
		writeRealnameRecord(db, ownerType, int64(ownerID), realnameProviderKuaitong, req.RealName, req.IDCard, realnameFaceStatusFailed, message, serialNo, "")
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "实名核验未通过：" + message})
		return
	}

	if _, err := db.Exec(
		"UPDATE "+realnameOwnerTable(ownerType)+" SET real_name = ?, real_id_card = ?, realname_at = NOW() WHERE id = ?",
		req.RealName, req.IDCard, ownerID,
	); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存实名认证结果失败"})
		return
	}
	writeRealnameRecord(db, ownerType, int64(ownerID), realnameProviderKuaitong, req.RealName, req.IDCard, realnameFaceStatusPassed, "", serialNo, "")
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "实名认证成功",
		"data": gin.H{
			"status":   realnameFaceStatusPassed,
			"provider": realnameProviderKuaitong,
			"authType": kuaitongAuthTypeTwoElement,
			"realName": maskRealName(req.RealName),
			"idCard":   maskIDCard(req.IDCard),
		},
	})
}

// ========== 快瞳实名认证 ==========

// 快瞳 token 进程级缓存：官方要求自行缓存、不要频繁调鉴权接口（有限流）。
type kuaitongTokenCacheEntry struct {
	token     string
	expiresAt time.Time
}

var (
	kuaitongTokenMu    sync.Mutex
	kuaitongTokenCache = map[string]kuaitongTokenCacheEntry{}
)

// kuaitongGetToken 获取快瞳 access_token，优先命中缓存；forceRefresh 用于 401/403 后主动刷新。
func kuaitongGetToken(cfg realnameConfig, forceRefresh bool) (string, error) {
	cacheKey := cfg.KuaitongAccessKey

	kuaitongTokenMu.Lock()
	if !forceRefresh {
		if entry, ok := kuaitongTokenCache[cacheKey]; ok && entry.token != "" && time.Now().Add(kuaitongTokenRefreshSkew).Before(entry.expiresAt) {
			token := entry.token
			kuaitongTokenMu.Unlock()
			return token, nil
		}
	}
	kuaitongTokenMu.Unlock()

	token, expiresIn, err := kuaitongFetchToken(cfg)
	if err != nil {
		return "", err
	}

	kuaitongTokenMu.Lock()
	kuaitongTokenCache[cacheKey] = kuaitongTokenCacheEntry{
		token:     token,
		expiresAt: time.Now().Add(time.Duration(expiresIn) * time.Second),
	}
	kuaitongTokenMu.Unlock()
	return token, nil
}

func kuaitongFetchToken(cfg realnameConfig) (string, int64, error) {
	reqURL := kuaitongTokenURL + "?accessKey=" + url.QueryEscape(cfg.KuaitongAccessKey) +
		"&accessSecret=" + url.QueryEscape(cfg.KuaitongSecret)

	client := &http.Client{Timeout: realnameHTTPTimeout}
	resp, err := client.Get(reqURL)
	if err != nil {
		return "", 0, fmt.Errorf("请求快瞳鉴权接口失败：%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", 0, fmt.Errorf("读取快瞳响应失败：%w", err)
	}
	var result struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Data    struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int64  `json:"expires_in"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, errors.New("快瞳鉴权响应解析失败")
	}
	if result.Status != 200 || result.Data.AccessToken == "" {
		msg := result.Message
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", result.Status)
		}
		return "", 0, fmt.Errorf("快瞳鉴权失败：%s", msg)
	}
	if result.Data.ExpiresIn <= 0 {
		result.Data.ExpiresIn = 7 * 24 * 3600 // 文档说明 token 有效期七天
	}
	return result.Data.AccessToken, result.Data.ExpiresIn, nil
}

// kuaitongVerifyIDCard 调用快瞳身份证二要素核验（姓名+身份证号）。
func kuaitongVerifyIDCard(cfg realnameConfig, realName, idCard string) (string, string, error) {
	token, err := kuaitongGetToken(cfg, false)
	if err != nil {
		return "", "", err
	}

	message, serialNo, shouldRetry, err := kuaitongPostIDCard(token, realName, idCard)
	if err != nil {
		return "", "", err
	}
	if shouldRetry {
		token, err = kuaitongGetToken(cfg, true)
		if err != nil {
			return "", "", err
		}
		message, serialNo, _, err = kuaitongPostIDCard(token, realName, idCard)
		if err != nil {
			return "", "", err
		}
	}
	return message, serialNo, nil
}

func kuaitongPostIDCard(token, realName, idCard string) (string, string, bool, error) {
	client := &http.Client{Timeout: realnameHTTPTimeout}
	return kuaitongPostIDCardWithClient(client, kuaitongIDCardURL, token, realName, idCard)
}

func kuaitongPostIDCardWithClient(client *http.Client, endpoint, token, realName, idCard string) (string, string, bool, error) {
	var buf strings.Builder
	writer := multipart.NewWriter(&buf)
	fields := []struct {
		name  string
		value string
	}{
		{name: "token", value: token},
		{name: "idCard", value: idCard},
		{name: "realName", value: realName},
	}
	for _, field := range fields {
		if err := writer.WriteField(field.name, field.value); err != nil {
			return "", "", false, fmt.Errorf("构造请求失败：%w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return "", "", false, fmt.Errorf("构造请求失败：%w", err)
	}

	resp, err := client.Post(endpoint, writer.FormDataContentType(), strings.NewReader(buf.String()))
	if err != nil {
		return "", "", false, fmt.Errorf("请求快瞳二要素核验接口失败：%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", false, fmt.Errorf("读取快瞳响应失败：%w", err)
	}
	var result struct {
		Status   int    `json:"status"`
		Code     int    `json:"code"`
		Message  string `json:"message"`
		SerialNo string `json:"serialNo"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", false, errors.New("快瞳二要素核验响应解析失败")
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden ||
		result.Status == http.StatusUnauthorized || result.Status == http.StatusForbidden {
		return "", "", true, nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && result.Status == http.StatusOK {
		return "", result.SerialNo, false, nil
	}

	message := strings.TrimSpace(result.Message)
	if message == "" {
		if result.Code != 0 {
			message = fmt.Sprintf("业务码 %d", result.Code)
		} else {
			message = fmt.Sprintf("状态码 %d", result.Status)
		}
	}
	return message, result.SerialNo, false, nil
}

// kuaitongVerifyFace 调用快瞳人证合一核验（姓名+身份证号+人像照片比对）。
// 返回 (message, serialNo, score, error)：error 表示请求级失败；message 非空表示核验未通过。
func kuaitongVerifyFace(cfg realnameConfig, realName, idCard, imgBase64 string) (string, string, string, error) {
	token, err := kuaitongGetToken(cfg, false)
	if err != nil {
		return "", "", "", err
	}

	message, serialNo, score, shouldRetry, err := kuaitongPostVerify(token, realName, idCard, imgBase64)
	if err != nil {
		return "", "", "", err
	}
	if shouldRetry {
		// token 过期/无效，主动刷新后重试一次
		token, err = kuaitongGetToken(cfg, true)
		if err != nil {
			return "", "", "", err
		}
		message, serialNo, score, _, err = kuaitongPostVerify(token, realName, idCard, imgBase64)
		if err != nil {
			return "", "", "", err
		}
	}
	return message, serialNo, score, nil
}

// kuaitongPostVerify 执行一次人证合一核验请求，返回 (未通过原因, 流水号, 相似度分数, 是否需要刷新 token 重试, 请求级错误)。
func kuaitongPostVerify(token, realName, idCard, imgBase64 string) (string, string, string, bool, error) {
	var buf strings.Builder
	writer := multipart.NewWriter(&buf)
	fields := map[string]string{
		"token":     token,
		"idCard":    idCard,
		"realName":  realName,
		"imgBase64": imgBase64,
	}
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			return "", "", "", false, fmt.Errorf("构造请求失败：%w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return "", "", "", false, fmt.Errorf("构造请求失败：%w", err)
	}

	client := &http.Client{Timeout: realnameHTTPTimeout}
	resp, err := client.Post(kuaitongVerifyURL, writer.FormDataContentType(), strings.NewReader(buf.String()))
	if err != nil {
		return "", "", "", false, fmt.Errorf("请求快瞳核验接口失败：%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", "", false, fmt.Errorf("读取快瞳响应失败：%w", err)
	}
	var result struct {
		Status   int    `json:"status"`
		Code     int    `json:"code"`
		Message  string `json:"message"`
		SerialNo string `json:"serialNo"`
		Data     struct {
			Pic string `json:"pic"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", "", false, errors.New("快瞳核验响应解析失败")
	}

	// token 过期或非法账号：需要刷新 token 重试
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden ||
		result.Status == http.StatusUnauthorized || result.Status == http.StatusForbidden {
		return "", "", "", true, nil
	}
	if result.Status == 200 && result.Code == 10000 {
		// data.pic 为相似度分数，分数大于阈值判定为同一人
		score, serr := strconv.ParseFloat(strings.TrimSpace(result.Data.Pic), 64)
		if serr == nil && score <= kuaitongPassScore {
			return fmt.Sprintf("人脸与证件照相似度不足（%.0f 分）", score), result.SerialNo, result.Data.Pic, false, nil
		}
		return "", result.SerialNo, result.Data.Pic, false, nil
	}
	msg := result.Message
	if msg == "" {
		msg = fmt.Sprintf("状态码 %d", result.Status)
	}
	return msg, result.SerialNo, "", false, nil
}

// tencentRealnameFlexText 兼容上游把业务码返回为字符串、数字或布尔值的情况。
type tencentRealnameFlexText string

func (v *tencentRealnameFlexText) UnmarshalJSON(raw []byte) error {
	trimmed := strings.TrimSpace(string(raw))
	switch {
	case trimmed == "" || trimmed == "null":
		*v = ""
	case strings.HasPrefix(trimmed, `"`):
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return err
		}
		*v = tencentRealnameFlexText(strings.TrimSpace(text))
	default:
		*v = tencentRealnameFlexText(trimmed)
	}
	return nil
}

// tencentRealnameVerifyData 只声明形态稳定的字段。上游不同产品对“是否通过”的键名
// 并不统一（success / result / pass / status ...），结论判定交给 resolveTencentRealnameOutcome
// 从原始 data 中扫描，避免固定字段缺失时把成功响应误判为未通过。
type tencentRealnameVerifyData struct {
	Message   string                  `json:"message"`
	Code      tencentRealnameFlexText `json:"code"`
	RawData   json.RawMessage         `json:"raw_data"`
	CertifyID string                  `json:"certify_id"`
}

// tencentRealnameVerifyResponse 的 data 用原始字节承载：上游在参数校验失败时会把
// data 返回为 false 等非对象形态，固定结构会让真实错误信息被解析错误覆盖。
type tencentRealnameVerifyResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// decodeTencentRealnameVerifyData 返回解析结果，以及 data 非对象时的原始形态摘要。
func decodeTencentRealnameVerifyData(raw json.RawMessage) (tencentRealnameVerifyData, string) {
	var data tencentRealnameVerifyData
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return data, ""
	}
	if !strings.HasPrefix(trimmed, "{") {
		return data, summarizeTencentRealnameResponseBody([]byte(trimmed))
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return tencentRealnameVerifyData{}, summarizeTencentRealnameResponseBody([]byte(trimmed))
	}
	return data, ""
}

// ---- 结论判定：上游结论字段命名不统一，用候选键表统一扫描 ----

type tencentRealnameOutcome int

const (
	tencentRealnameOutcomeUnknown tencentRealnameOutcome = iota
	tencentRealnameOutcomePassed
	tencentRealnameOutcomeRejected
)

var (
	// 真值语义键：值本身表达通过与否。
	tencentRealnameOutcomeKeys = []string{
		"success", "is_success", "issuccess", "pass", "passed", "is_pass",
		"result", "verify_result", "check_result", "auth_result", "face_result",
		"match", "is_match", "status", "state", "result_code", "certify_status", "auth_status",
	}
	// 错误码语义键：0 表示通过。
	tencentRealnameErrCodeKeys = []string{"errcode", "err_code", "error_code", "errorcode"}

	tencentRealnameOutcomePassedValues = map[string]bool{
		"true": true, "1": true, "y": true, "yes": true, "ok": true,
		"success": true, "succeed": true, "succeeded": true, "pass": true, "passed": true,
		"通过": true, "成功": true, "一致": true, "认证成功": true, "核验通过": true,
	}
	tencentRealnameOutcomeRejectedValues = map[string]bool{
		"false": true, "0": true, "n": true, "no": true,
		"fail": true, "failed": true, "failure": true, "reject": true, "rejected": true,
		"未通过": true, "失败": true, "不一致": true, "认证失败": true, "核验失败": true,
	}
	tencentRealnameSuccessCodes = map[string]bool{
		"": true, "0": true, "00": true, "000": true, "0000": true,
		"200": true, "success": true, "ok": true,
	}
)

type tencentRealnameFieldScope struct {
	path   string
	fields map[string]interface{}
}

// collectTencentRealnameScopes 逐层展开 data，使 raw_data 及其嵌套对象里的结论字段同样可见。
func collectTencentRealnameScopes(path string, raw json.RawMessage, depth int) []tencentRealnameFieldScope {
	if depth < 0 || len(raw) == 0 {
		return nil
	}
	var fields map[string]interface{}
	if json.Unmarshal(raw, &fields) != nil || len(fields) == 0 {
		return nil
	}
	scopes := []tencentRealnameFieldScope{{path: path, fields: fields}}
	if depth == 0 {
		return scopes
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		nested, ok := fields[key].(map[string]interface{})
		if !ok {
			continue
		}
		encoded, err := json.Marshal(nested)
		if err != nil {
			continue
		}
		scopes = append(scopes, collectTencentRealnameScopes(path+"."+key, encoded, depth-1)...)
	}
	return scopes
}

func normalizeTencentRealnameFlagValue(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case string:
		return strings.ToLower(strings.TrimSpace(typed))
	default:
		return strings.ToLower(strings.TrimSpace(fmt.Sprint(typed)))
	}
}

func matchTencentRealnameOutcome(scope tencentRealnameFieldScope) (tencentRealnameOutcome, string) {
	normalizedFields := make(map[string]interface{}, len(scope.fields))
	for key, value := range scope.fields {
		normalizedFields[strings.ToLower(strings.TrimSpace(key))] = value
	}
	for _, key := range tencentRealnameOutcomeKeys {
		value, ok := normalizedFields[key]
		if !ok {
			continue
		}
		normalized := normalizeTencentRealnameFlagValue(value)
		switch {
		case tencentRealnameOutcomePassedValues[normalized]:
			return tencentRealnameOutcomePassed, scope.path + "." + key + "=" + normalized
		case tencentRealnameOutcomeRejectedValues[normalized]:
			return tencentRealnameOutcomeRejected, scope.path + "." + key + "=" + normalized
		}
	}
	for _, key := range tencentRealnameErrCodeKeys {
		value, ok := normalizedFields[key]
		if !ok {
			continue
		}
		normalized := normalizeTencentRealnameFlagValue(value)
		if tencentRealnameSuccessCodes[normalized] {
			return tencentRealnameOutcomePassed, scope.path + "." + key + "=" + normalized
		}
		return tencentRealnameOutcomeRejected, scope.path + "." + key + "=" + normalized
	}
	return tencentRealnameOutcomeUnknown, ""
}

// resolveTencentRealnameOutcome 返回结论及命中依据，依据会随认证记录一起留档。
func resolveTencentRealnameOutcome(raw json.RawMessage) (tencentRealnameOutcome, string) {
	for _, scope := range collectTencentRealnameScopes("data", raw, 2) {
		if outcome, basis := matchTencentRealnameOutcome(scope); outcome != tencentRealnameOutcomeUnknown {
			return outcome, basis
		}
	}
	return tencentRealnameOutcomeUnknown, ""
}

// tencentRealnameFaceResult 把“给用户看的原因”和“进认证记录的明细”分开：
// Reason 简短可读，Detail 含判定依据、响应码与完整上游正文。
type tencentRealnameFaceResult struct {
	Passed    bool
	Reason    string
	Detail    string
	SerialNo  string
	Score     string
	CertifyID string // 认证会话ID，用于查询最终结果
}

// tencentRealnameErrorCodeHints 来自聚合网关公开错误码表，用于把上游码翻译成处置建议。
var tencentRealnameErrorCodeHints = map[string]string{
	"AUTH_FAILED":          "签名验证失败，请核对 API Key/Secret 与服务器时间",
	"INSUFFICIENT_BALANCE": "上游余额不足，请充值后重试",
	"INSUFFICIENT_PACKAGE": "无可用套餐或套餐不含该产品，请购买套餐或关闭套餐扣费改用余额",
	"CHARGE_FAILED":        "上游扣费失败或并发冲突，请稍后重试",
	"PRODUCT_MISMATCH":     "product_code 与认证记录不一致",
	"RECORD_NOT_FOUND":     "未找到开放 API 认证记录，请核对 certify_id",
	"INVALID_PARAM":        "请求参数无效，请检查参数并确认未传 package_id",
	"PRODUCT_NOT_FOUND":    "产品不存在或已被禁用，请核对 product_code",
}

func tencentRealnameErrorCodeHint(businessCode string) string {
	return tencentRealnameErrorCodeHints[strings.ToUpper(strings.TrimSpace(businessCode))]
}

// tencentRealnameChargeFields 是网关通用响应里的扣费与套餐元数据，进认证记录用于核对是否计费。
var tencentRealnameChargeFields = []struct {
	key   string
	label string
}{
	{"is_charged", "已扣费"},
	{"charge_amount", "扣费金额"},
	{"package_used", "使用套餐"},
	{"package_id_used", "套餐ID"},
	{"use_package", "请求套餐模式"},
	{"cost_time", "上游耗时(ms)"},
}

// describeTencentRealnameCharge 汇总扣费元数据，缺失字段直接跳过。
func describeTencentRealnameCharge(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var fields map[string]interface{}
	if json.Unmarshal(raw, &fields) != nil || len(fields) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tencentRealnameChargeFields))
	for _, field := range tencentRealnameChargeFields {
		value, ok := fields[field.key]
		if !ok || value == nil {
			continue
		}
		parts = append(parts, field.label+"="+normalizeTencentRealnameFlagValue(value))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "；")
}

type tencentRealnameVerifyError struct {
	ResultUnknown bool
	Message       string
	HTTPStatus    int
	ResponseCode  int
	BusinessCode  string
	ResponseBody  string
}

func (e *tencentRealnameVerifyError) Error() string {
	return e.Message
}

func newTencentRealnameVerifyError(message string, resultUnknown bool) error {
	return &tencentRealnameVerifyError{ResultUnknown: resultUnknown, Message: message}
}

func newTencentRealnameResponseError(message string, resultUnknown bool, httpStatus, responseCode int, businessCode string) error {
	return &tencentRealnameVerifyError{
		ResultUnknown: resultUnknown,
		Message:       message,
		HTTPStatus:    httpStatus,
		ResponseCode:  responseCode,
		BusinessCode:  strings.TrimSpace(businessCode),
	}
}

func tencentRealnameErrorMetadata(err error) (httpStatus, responseCode int, businessCode string) {
	var target *tencentRealnameVerifyError
	if !errors.As(err, &target) {
		return 0, 0, ""
	}
	return target.HTTPStatus, target.ResponseCode, target.BusinessCode
}

// withTencentRealnameResponseBody 把完整上游正文挂到错误上，只用于认证记录留档，不进用户提示。
func withTencentRealnameResponseBody(err error, respBody []byte) error {
	var target *tencentRealnameVerifyError
	if err == nil || !errors.As(err, &target) {
		return err
	}
	target.ResponseBody = flattenTencentRealnameResponseBody(respBody)
	return target
}

func tencentRealnameErrorResponseBody(err error) string {
	var target *tencentRealnameVerifyError
	if !errors.As(err, &target) {
		return ""
	}
	return target.ResponseBody
}

func isTencentRealnameResultUnknown(err error) bool {
	var target *tencentRealnameVerifyError
	return errors.As(err, &target) && target.ResultUnknown
}

func tencentRealnameSignature(apiKey, timestamp, nonce, apiSecret string) string {
	mac := hmac.New(sha256.New, []byte(apiSecret))
	_, _ = mac.Write([]byte(apiKey + timestamp + nonce))
	return hex.EncodeToString(mac.Sum(nil))
}

func newTencentRealnameNonce() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

type tencentRealnameInitResult struct {
	CertifyID string
	AuthURL   string
}

type tencentRealnameInitData struct {
	Code       tencentRealnameFlexText `json:"code"`
	Message    string                  `json:"message"`
	CertifyID  string                  `json:"certify_id"`
	CertifyURL string                  `json:"certify_url"`
}

func tencentRealnameProductCode(cfg realnameConfig) string {
	if productCode := strings.TrimSpace(cfg.TencentProductCode); productCode != "" {
		return productCode
	}
	return tencentRealnameProduct
}

var tencentRealnameHTTPClient = &http.Client{Timeout: tencentRealnameHTTPTimeout}

func tencentRealnameRequest(cfg realnameConfig, endpointPath string, payload interface{}) (int, []byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, newTencentRealnameVerifyError(fmt.Sprintf("构造靓仔聚合认证请求失败：%v", err), false)
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce, err := newTencentRealnameNonce()
	if err != nil {
		return 0, nil, newTencentRealnameVerifyError("生成靓仔聚合认证签名随机数失败", false)
	}
	endpoint := strings.TrimRight(cfg.TencentBaseURL, "/") + "/" + strings.TrimLeft(endpointPath, "/")

	logPayload := make(map[string]interface{})
	if err := json.Unmarshal(body, &logPayload); err == nil {
		for _, key := range []string{"name", "idcard", "id_card", "image", "image_base64", "img_base64", "certify_id"} {
			if _, exists := logPayload[key]; exists {
				logPayload[key] = "<redacted>"
			}
		}
		if sanitized, marshalErr := json.Marshal(logPayload); marshalErr == nil {
			fmt.Printf("[靓仔认证] 请求 %s %s, 参数: %s\n", http.MethodPost, endpointPath, string(sanitized))
		}
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, newTencentRealnameVerifyError(fmt.Sprintf("创建靓仔聚合认证请求失败：%v", err), false)
	}
	req.Header.Set("X-Api-Key", cfg.TencentAPIKey)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", tencentRealnameSignature(cfg.TencentAPIKey, timestamp, nonce, cfg.TencentAPISecret))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AuthPro-Tencent-Realname/1.0")

	resp, err := tencentRealnameHTTPClient.Do(req)
	if err != nil {
		fmt.Printf("[靓仔认证] 请求失败: %v\n", err)
		return 0, nil, newTencentRealnameVerifyError(fmt.Sprintf("请求靓仔聚合认证接口失败：%v", err), true)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		fmt.Printf("[靓仔认证] 读取响应失败: %v\n", err)
		return resp.StatusCode, nil, newTencentRealnameVerifyError(fmt.Sprintf("读取靓仔聚合认证响应失败：%v", err), true)
	}
	fmt.Printf("[靓仔认证] 响应状态: %d, 内容长度: %d\n", resp.StatusCode, len(respBody))

	return resp.StatusCode, respBody, nil
}

func buildTencentRealnameInitPayload(cfg realnameConfig, realName, idCard, returnURL string) map[string]interface{} {
	return map[string]interface{}{
		"product_code": tencentRealnameProductCode(cfg),
		"use_package":  cfg.TencentUsePackage,
		"name":         realName,
		"idcard":       idCard,
		"return_url":   returnURL,
	}
}

func tencentRealnameInitialize(cfg realnameConfig, realName, idCard, returnURL string) (tencentRealnameInitResult, error) {
	statusCode, respBody, err := tencentRealnameRequest(
		cfg,
		"/verify",
		buildTencentRealnameInitPayload(cfg, realName, idCard, returnURL),
	)
	if err != nil {
		return tencentRealnameInitResult{}, err
	}
	return parseTencentRealnameInitResponse(statusCode, respBody)
}

func parseTencentRealnameInitResponse(statusCode int, respBody []byte) (tencentRealnameInitResult, error) {
	var response tencentRealnameVerifyResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return tencentRealnameInitResult{}, withTencentRealnameResponseBody(
			newTencentRealnameResponseError(
				fmt.Sprintf("靓仔人脸认证初始化响应解析失败：HTTP %d，JSON错误：%v", statusCode, err),
				statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices,
				statusCode,
				0,
				"",
			),
			respBody,
		)
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices || response.Code != 0 {
		_, err := parseTencentRealnameVerifyResponse(statusCode, respBody)
		if err != nil {
			return tencentRealnameInitResult{}, err
		}
		return tencentRealnameInitResult{}, newTencentRealnameResponseError("靓仔人脸认证初始化失败", false, statusCode, response.Code, "")
	}

	var data tencentRealnameInitData
	if err := json.Unmarshal(response.Data, &data); err != nil {
		return tencentRealnameInitResult{}, withTencentRealnameResponseBody(
			newTencentRealnameResponseError("靓仔人脸认证初始化响应缺少有效 data", true, statusCode, response.Code, ""),
			respBody,
		)
	}
	certifyID := strings.TrimSpace(data.CertifyID)
	certifyURL := strings.TrimSpace(data.CertifyURL)
	if certifyID == "" || certifyURL == "" {
		return tencentRealnameInitResult{}, withTencentRealnameResponseBody(
			newTencentRealnameResponseError("靓仔人脸认证初始化未返回 certify_id 或 certify_url", true, statusCode, response.Code, string(data.Code)),
			respBody,
		)
	}
	parsedURL, err := url.ParseRequestURI(certifyURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return tencentRealnameInitResult{}, withTencentRealnameResponseBody(
			newTencentRealnameResponseError("靓仔人脸认证初始化返回的 certify_url 无效", true, statusCode, response.Code, string(data.Code)),
			respBody,
		)
	}
	return tencentRealnameInitResult{CertifyID: certifyID, AuthURL: certifyURL}, nil
}

func buildTencentRealnameQueryPayload(cfg realnameConfig, certifyID string) map[string]interface{} {
	return map[string]interface{}{
		"certify_id":   certifyID,
		"product_code": tencentRealnameProductCode(cfg),
	}
}

func tencentRealnameQuery(cfg realnameConfig, certifyID string) (tencentRealnameFaceResult, bool, error) {
	statusCode, respBody, err := tencentRealnameRequest(cfg, "/faceQuery", buildTencentRealnameQueryPayload(cfg, certifyID))
	if err != nil {
		return tencentRealnameFaceResult{}, false, err
	}
	return parseTencentRealnameQueryResponse(statusCode, respBody)
}

// tencentRealnameVerifyWithImage 调用靓仔人脸核验（姓名+身份证号+人像照片比对）。
// 返回 (faceResult, error)：error 表示请求级失败；faceResult.Passed 表示是否通过。
func tencentRealnameVerifyWithImage(cfg realnameConfig, realName, idCard, imgBase64 string) (tencentRealnameFaceResult, error) {
	// 从 data:image/...;base64,... 中提取纯 base64 部分
	imgData := imgBase64
	if strings.HasPrefix(imgBase64, "data:image/") {
		commaIdx := strings.Index(imgBase64, ";base64,")
		if commaIdx >= 0 {
			imgData = imgBase64[commaIdx+8:]
		}
	}

	// 靓仔接口要求 Base64 图片 ≤500KB
	decoded, err := base64.StdEncoding.DecodeString(imgData)
	if err != nil {
		return tencentRealnameFaceResult{}, newTencentRealnameVerifyError("图片数据无效", false)
	}
	if len(decoded) > 375*1024 { // 375KB 原始数据 ≈ 500KB Base64
		return tencentRealnameFaceResult{}, newTencentRealnameVerifyError("图片超过 500KB 限制，请压缩后重试", false)
	}

	payload := map[string]interface{}{
		"product_code": tencentRealnameProductCode(cfg),
		"use_package":  cfg.TencentUsePackage,
		"name":         realName,
		"idcard":       idCard,
		"image_base64": imgData,
	}

	statusCode, respBody, err := tencentRealnameRequest(cfg, "/verify", payload)
	if err != nil {
		return tencentRealnameFaceResult{}, err
	}

	return parseTencentRealnameVerifyResponse(statusCode, respBody)
}

func tencentRealnameQueryPending(raw json.RawMessage, message string) bool {
	pendingValues := map[string]bool{
		"pending": true, "processing": true, "waiting": true, "initialized": true,
		"initializing": true, "in_progress": true,
	}
	var fields map[string]interface{}
	if json.Unmarshal(raw, &fields) == nil {
		for _, key := range []string{"status", "state", "certify_status", "auth_status"} {
			if value, ok := fields[key]; ok && pendingValues[normalizeTencentRealnameFlagValue(value)] {
				return true
			}
		}
		if value, ok := fields["message"]; ok {
			message += " " + fmt.Sprint(value)
		}
	}
	message = strings.ToLower(strings.TrimSpace(message))
	for _, keyword := range []string{"pending", "processing", "waiting", "未完成", "认证中", "处理中", "等待认证", "尚未认证"} {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}

func parseTencentRealnameQueryResponse(statusCode int, respBody []byte) (tencentRealnameFaceResult, bool, error) {
	var response tencentRealnameVerifyResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		_, parseErr := parseTencentRealnameVerifyResponse(statusCode, respBody)
		return tencentRealnameFaceResult{}, false, parseErr
	}
	if tencentRealnameQueryPending(response.Data, response.Message) {
		return tencentRealnameFaceResult{}, true, nil
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices || response.Code != 0 {
		_, err := parseTencentRealnameVerifyResponse(statusCode, respBody)
		return tencentRealnameFaceResult{}, false, err
	}
	data, dataAnomaly := decodeTencentRealnameVerifyData(response.Data)
	if dataAnomaly != "" {
		return tencentRealnameFaceResult{}, true, nil
	}
	outcome, _ := resolveTencentRealnameOutcome(response.Data)
	businessCode := strings.ToLower(strings.TrimSpace(string(data.Code)))
	if outcome == tencentRealnameOutcomeUnknown && tencentRealnameSuccessCodes[businessCode] {
		return tencentRealnameFaceResult{}, true, nil
	}
	result, err := parseTencentRealnameVerifyResponse(statusCode, respBody)
	return result, false, err
}

type realnameProductItem struct {
	ProductCode string `json:"productCode"`
	Name        string `json:"name"`
	Price       string `json:"price"`
}

type realnameProductRequest struct {
	TencentAPIKey    string `json:"tencentApiKey"`
	TencentAPISecret string `json:"tencentApiSecret"`
	TencentBaseURL   string `json:"tencentBaseUrl"`
}

type tencentRealnameProductsResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func AdminRealnameProducts(c *gin.Context) {
	var input realnameProductRequest
	if err := c.ShouldBindJSON(&input); err != nil {
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
	cfg, err := loadRealnameConfig(db)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取实名认证配置失败"})
		return
	}

	apiKey := strings.TrimSpace(input.TencentAPIKey)
	if apiKey == "" {
		apiKey = cfg.TencentAPIKey
	}
	apiSecret := strings.TrimSpace(input.TencentAPISecret)
	if apiSecret == "" && apiKey == cfg.TencentAPIKey {
		apiSecret = cfg.TencentAPISecret
	}
	baseURL := strings.TrimRight(strings.TrimSpace(input.TencentBaseURL), "/")
	if baseURL == "" {
		baseURL = cfg.TencentBaseURL
	}
	if apiKey == "" || apiSecret == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请先填写 API Key 和 API Secret"})
		return
	}
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "接口地址必须是有效的 HTTPS 地址"})
		return
	}

	products, err := fetchTencentRealnameProducts(apiKey, apiSecret, baseURL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{"list": products}})
}

func fetchTencentRealnameProducts(apiKey, apiSecret, baseURL string) ([]realnameProductItem, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce, err := newTencentRealnameNonce()
	if err != nil {
		return nil, errors.New("生成产品列表签名随机数失败")
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/getProducts", nil)
	if err != nil {
		return nil, fmt.Errorf("创建产品列表请求失败：%w", err)
	}
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", tencentRealnameSignature(apiKey, timestamp, nonce, apiSecret))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AuthPro-Tencent-Realname/1.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取产品列表失败：%w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, errors.New("读取产品列表失败")
	}
	return parseTencentRealnameProducts(resp.StatusCode, body)
}

func parseTencentRealnameProducts(statusCode int, body []byte) ([]realnameProductItem, error) {
	var response tencentRealnameProductsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, errors.New("产品列表响应解析失败")
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices || response.Code != 0 {
		message := strings.TrimSpace(response.Message)
		var detail struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(response.Data, &detail) == nil {
			if message == "" {
				message = strings.TrimSpace(detail.Message)
			}
			if detail.Code != "" {
				if message == "" {
					message = detail.Code
				} else {
					message += "（" + detail.Code + "）"
				}
			}
		}
		if message == "" {
			message = fmt.Sprintf("HTTP %d", statusCode)
		}
		return nil, errors.New("获取产品列表失败：" + message)
	}

	var data interface{}
	decoder := json.NewDecoder(strings.NewReader(string(response.Data)))
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		return nil, errors.New("产品列表数据格式不正确")
	}
	items := findTencentRealnameProductItems(data)
	if len(items) == 0 {
		return []realnameProductItem{}, nil
	}
	return items, nil
}

func findTencentRealnameProductItems(data interface{}) []realnameProductItem {
	var rawItems []interface{}
	switch value := data.(type) {
	case []interface{}:
		rawItems = value
	case map[string]interface{}:
		for _, key := range []string{"products", "productList", "product_list", "list", "items", "records", "data"} {
			if nested, ok := value[key]; ok {
				if items := findTencentRealnameProductItems(nested); len(items) > 0 {
					return items
				}
			}
		}
		rawItems = []interface{}{value}
	}

	items := make([]realnameProductItem, 0, len(rawItems))
	seen := map[string]bool{}
	for _, raw := range rawItems {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		productCode := firstTencentRealnameProductValue(item, "product_code", "productCode", "code")
		if productCode == "" || seen[productCode] {
			continue
		}
		seen[productCode] = true
		name := firstTencentRealnameProductValue(item, "product_name", "productName", "name", "title")
		if name == "" {
			name = productCode
		}
		items = append(items, realnameProductItem{
			ProductCode: productCode,
			Name:        name,
			Price:       firstTencentRealnameProductValue(item, "price", "unit_price", "unitPrice"),
		})
	}
	return items
}

func firstTencentRealnameProductValue(item map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := item[key]
		if !ok || value == nil {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" {
			return text
		}
	}
	return ""
}

func summarizeTencentRealnameResponseBody(body []byte) string {
	const maxRunes = 96

	text := flattenTencentRealnameResponseBody(body)
	runes := []rune(text)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return text
}

// flattenTencentRealnameResponseBody 压成单行但不截断，认证记录需要完整上游返回。
func flattenTencentRealnameResponseBody(body []byte) string {
	text := strings.Join(strings.Fields(string(body)), " ")
	if text == "" {
		return "<empty>"
	}
	return text
}

// tencentRealnameVerifyDetail 组装进认证记录的明细：判定依据、扣费元数据、响应码、完整正文。
func tencentRealnameVerifyDetail(statusCode, responseCode int, basis, charge string, respBody []byte) string {
	lines := make([]string, 0, 4)
	if basis != "" {
		lines = append(lines, "判定依据："+basis)
	}
	if charge != "" {
		lines = append(lines, "扣费信息："+charge)
	}
	lines = append(lines, fmt.Sprintf("接口响应码：%d；HTTP状态：%d", responseCode, statusCode))
	lines = append(lines, "上游响应正文："+flattenTencentRealnameResponseBody(respBody))
	return strings.Join(lines, "\n")
}

func parseTencentRealnameVerifyResponse(statusCode int, respBody []byte) (tencentRealnameFaceResult, error) {
	var result tencentRealnameVerifyResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		resultUnknown := statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
		detail := fmt.Sprintf(
			"腾讯增强人脸响应解析失败：HTTP %d，JSON错误：%v，响应正文：%s",
			statusCode,
			err,
			summarizeTencentRealnameResponseBody(respBody),
		)
		return tencentRealnameFaceResult{}, withTencentRealnameResponseBody(
			newTencentRealnameResponseError(detail, resultUnknown, statusCode, 0, ""),
			respBody,
		)
	}
	data, dataAnomaly := decodeTencentRealnameVerifyData(result.Data)
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices || result.Code != 0 {
		message := strings.TrimSpace(result.Message)
		detailMessage := strings.TrimSpace(data.Message)
		if detailMessage != "" && !strings.Contains(message, detailMessage) {
			if message == "" {
				message = detailMessage
			} else {
				message += "：" + detailMessage
			}
		}
		businessCode := strings.TrimSpace(string(data.Code))
		if businessCode != "" && !strings.Contains(message, businessCode) {
			if message == "" {
				message = businessCode
			} else {
				message += "（" + businessCode + "）"
			}
		}
		if message == "" && dataAnomaly != "" {
			message = "data=" + dataAnomaly
		}
		if message == "" {
			message = fmt.Sprintf("HTTP %d", statusCode)
		}
		if hint := tencentRealnameErrorCodeHint(businessCode); hint != "" && !strings.Contains(message, hint) {
			message += "；处置建议：" + hint
		}
		return tencentRealnameFaceResult{}, withTencentRealnameResponseBody(
			newTencentRealnameResponseError(
				"腾讯增强人脸请求失败："+message,
				false,
				statusCode,
				result.Code,
				businessCode,
			),
			respBody,
		)
	}

	if dataAnomaly != "" {
		detail := fmt.Sprintf(
			"腾讯增强人脸响应未包含认证结果：HTTP %d，data=%s，响应正文：%s",
			statusCode,
			dataAnomaly,
			summarizeTencentRealnameResponseBody(respBody),
		)
		return tencentRealnameFaceResult{}, withTencentRealnameResponseBody(
			newTencentRealnameResponseError(detail, true, statusCode, result.Code, ""),
			respBody,
		)
	}

	serialNo := data.CertifyID
	score := extractTencentRealnameScore(data.RawData)
	businessCode := strings.TrimSpace(string(data.Code))
	outcome, basis := resolveTencentRealnameOutcome(result.Data)
	if outcome == tencentRealnameOutcomeUnknown {
		// 接口层已判成功，响应里又找不到任何否定结论时按通过处理，
		// 完整正文仍会留档，便于事后核对上游到底返回了什么。
		if tencentRealnameSuccessCodes[strings.ToLower(businessCode)] {
			outcome = tencentRealnameOutcomePassed
			basis = "接口返回成功码，响应未包含结论字段"
		} else {
			outcome = tencentRealnameOutcomeRejected
			basis = "data.code=" + businessCode
		}
	}
	detail := tencentRealnameVerifyDetail(statusCode, result.Code, basis, describeTencentRealnameCharge(result.Data), respBody)
	if outcome == tencentRealnameOutcomePassed {
		return tencentRealnameFaceResult{
			Passed:    true,
			Detail:    detail,
			SerialNo:  serialNo,
			Score:     score,
			CertifyID: data.CertifyID,
		}, nil
	}
	reason := strings.TrimSpace(data.Message)
	if reason == "" {
		reason = businessCode
	} else if businessCode != "" && !strings.Contains(reason, businessCode) {
		reason += "（" + businessCode + "）"
	}
	if reason == "" {
		reason = "人脸核验未通过"
	}
	if hint := tencentRealnameErrorCodeHint(businessCode); hint != "" && !strings.Contains(reason, hint) {
		reason += "；处置建议：" + hint
	}
	return tencentRealnameFaceResult{
		Reason:    reason,
		Detail:    reason + "\n" + detail,
		SerialNo:  serialNo,
		Score:     score,
		CertifyID: data.CertifyID,
	}, nil
}

func extractTencentRealnameScore(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var data map[string]interface{}
	if json.Unmarshal(raw, &data) != nil {
		return ""
	}
	for _, key := range []string{"score", "similarity", "confidence"} {
		if value, ok := data[key]; ok {
			return fmt.Sprint(value)
		}
	}
	return ""
}

// ========== 管理端：实名认证记录 ==========

type realnameRecordItem struct {
	ID         int64  `json:"id"`
	OwnerType  string `json:"ownerType"`
	OwnerID    int64  `json:"ownerId"`
	OwnerName  string `json:"ownerName"` // 用户昵称/代理名称
	OwnerEmail string `json:"ownerEmail"`
	Provider   string `json:"provider"`
	RealName   string `json:"realName"` // 展示层脱敏
	IDCard     string `json:"idCard"`   // 展示层脱敏
	Status     string `json:"status"`
	FailReason string `json:"failReason"`
	SerialNo   string `json:"serialNo"`
	Score      string `json:"score"`
	CreatedAt  string `json:"createdAt"`
}

// AdminRealnameRecordList 实名认证记录列表（姓名/身份证展示时脱敏，库中明文存储）。
func AdminRealnameRecordList(c *gin.Context) {
	db, err := openSystemConfigDB()
	if err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureRealnameStorage(db); err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 500, "msg": "初始化实名存储失败"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	ownerSource := ` FROM realname_records r
		LEFT JOIN (
			SELECT 'user' AS owner_type, id AS owner_id,
				COALESCE(nickname, '') AS owner_name, COALESCE(email, '') AS owner_email,
				COALESCE(real_name, '') AS real_name, COALESCE(real_id_card, '') AS real_id_card,
				realname_at
			FROM users
			UNION ALL
			SELECT 'agent' AS owner_type, id AS owner_id,
				COALESCE(name, '') AS owner_name, COALESCE(email, '') AS owner_email,
				COALESCE(real_name, '') AS real_name, COALESCE(real_id_card, '') AS real_id_card,
				realname_at
			FROM agents
		) o ON o.owner_type = r.owner_type AND o.owner_id = r.owner_id`
	where := ` WHERE r.status IN ('passed', 'failed')
		AND (
			r.status = 'failed'
			OR (
				o.realname_at IS NOT NULL
				AND TRIM(o.real_name) != '' AND TRIM(o.real_id_card) != ''
				AND o.real_name != '已认证' AND o.real_id_card != '已认证'
			)
		)`
	args := []interface{}{}
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		where += ` AND (
			(CASE WHEN r.status = 'passed' THEN o.real_name ELSE r.real_name END) LIKE ?
			OR (CASE WHEN r.status = 'passed' THEN o.real_id_card ELSE r.id_card END) LIKE ?
			OR o.owner_name LIKE ? OR o.owner_email LIKE ? OR r.fail_reason LIKE ? OR r.serial_no LIKE ?
		)`
		like := "%" + kw + "%"
		args = append(args, like, like, like, like, like, like)
	}
	if provider := strings.TrimSpace(c.Query("provider")); provider == realnameProviderAlipay || provider == realnameProviderKuaitong || provider == realnameProviderTencent {
		where += " AND r.provider = ?"
		args = append(args, provider)
	}
	if status := strings.TrimSpace(c.Query("status")); status == realnameFaceStatusPassed || status == realnameFaceStatusFailed {
		where += " AND r.status = ?"
		args = append(args, status)
	}

	var total int
	if err := db.QueryRow("SELECT COUNT(*)"+ownerSource+where, args...).Scan(&total); err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 500, "msg": "查询失败"})
		return
	}

	query := `SELECT r.id, r.owner_type, r.owner_id, r.provider,
		CASE WHEN r.status = 'passed' THEN o.real_name ELSE r.real_name END,
		CASE WHEN r.status = 'passed' THEN o.real_id_card ELSE r.id_card END,
		r.status, r.fail_reason, r.serial_no, r.score, r.created_at,
		COALESCE(o.owner_name, ''), COALESCE(o.owner_email, '')` +
		ownerSource + where + ` ORDER BY r.id DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := db.Query(query, args...)
	if err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	defer rows.Close()

	list := make([]realnameRecordItem, 0)
	for rows.Next() {
		var item realnameRecordItem
		var createdAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.OwnerType, &item.OwnerID, &item.Provider, &item.RealName, &item.IDCard,
			&item.Status, &item.FailReason, &item.SerialNo, &item.Score, &createdAt, &item.OwnerName, &item.OwnerEmail); err != nil {
			continue
		}
		if createdAt.Valid {
			item.CreatedAt = createdAt.Time.Format("2006-01-02 15:04:05")
		}
		item.RealName = maskRealName(item.RealName)
		item.IDCard = maskIDCard(item.IDCard)
		list = append(list, item)
	}

	writeSystemConfig(c, http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{
		"list":     list,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	}})
}

// ========== 授权校验用：判断用户是否需要实名 ==========

// userRealnameRequired 返回（该应用是否要求实名，授权归属主体是否已实名）。
// 用户与代理商都按各自主体表校验；未知归属类型才按管理员直接创建处理。
func userRealnameRequired(db *sql.DB, appID int64, licenseID int64) (required bool, verified bool, err error) {
	rnCfg, err := loadRealnameConfig(db)
	if err != nil {
		return false, true, nil // 配置读取失败时放行，避免误杀
	}
	if !rnCfg.Enabled || !rnCfg.RequireAppIDs[appID] {
		return false, true, nil
	}

	var ownerType string
	var ownerID int64
	if err := db.QueryRow("SELECT owner_type, owner_id FROM licenses WHERE id = ?", licenseID).Scan(&ownerType, &ownerID); err != nil {
		return true, false, err
	}
	table := ""
	switch ownerType {
	case "user":
		table = "users"
	case "agent":
		table = "agents"
	default:
		// 管理员直接创建的授权不受实名限制
		return true, true, nil
	}

	var realnameAt sql.NullTime
	if err := db.QueryRow("SELECT realname_at FROM "+table+" WHERE id = ?", ownerID).Scan(&realnameAt); err != nil {
		return true, false, err
	}
	return true, realnameAt.Valid, nil
}

// ========== 支付宝网关调用 ==========

type alipayRealnameInitResponse struct {
	CertifyID string `json:"certify_id"`
}

type alipayRealnameQueryResponse struct {
	Passed       string `json:"passed"` // T/F
	IdentityInfo struct {
		CertName string `json:"cert_name"`
		CertNo   string `json:"cert_no"`
	} `json:"identity_info"`
}

func alipayRealnameInitialize(cfg realnameConfig, outerOrderNo, realName, idCard string) (*alipayRealnameInitResponse, error) {
	bizContent, _ := json.Marshal(map[string]interface{}{
		"outer_order_no": outerOrderNo,
		"biz_code":       "FACE",
		"identity_param": map[string]string{
			"identity_type": "CERT_INFO",
			"cert_type":     "IDENTITY_CARD",
			"cert_name":     realName,
			"cert_no":       idCard,
		},
		"merchant_config": map[string]string{},
	})

	resp, err := alipayPost(cfg, realnameInitMethod, string(bizContent))
	if err != nil {
		return nil, err
	}
	node, ok := resp["datadigital_fincloud_generalsaas_face_certify_initialize_response"].(map[string]interface{})
	if !ok {
		return nil, errors.New("响应格式异常")
	}
	if code, _ := node["code"].(string); code != "10000" {
		subMsg, _ := node["sub_msg"].(string)
		if subMsg == "" {
			subMsg, _ = node["msg"].(string)
		}
		return nil, fmt.Errorf("支付宝返回错误：%s", subMsg)
	}
	certifyID, _ := node["certify_id"].(string)
	if certifyID == "" {
		return nil, errors.New("未返回 certify_id")
	}
	return &alipayRealnameInitResponse{CertifyID: certifyID}, nil
}

func alipayRealnameQuery(cfg realnameConfig, certifyID, outerOrderNo string) (*alipayRealnameQueryResponse, error) {
	biz := map[string]string{"certify_id": certifyID}
	if outerOrderNo != "" {
		biz["outer_order_no"] = outerOrderNo
	}
	bizContent, _ := json.Marshal(biz)

	resp, err := alipayPost(cfg, realnameQueryMethod, string(bizContent))
	if err != nil {
		return nil, err
	}
	node, ok := resp["datadigital_fincloud_generalsaas_face_certify_query_response"].(map[string]interface{})
	if !ok {
		return nil, errors.New("响应格式异常")
	}
	if code, _ := node["code"].(string); code != "10000" {
		return nil, fmt.Errorf("查询未成功")
	}
	raw, _ := json.Marshal(node)
	var out alipayRealnameQueryResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// alipayPost 以 RSA2 签名调用支付宝网关，返回整个响应 JSON。
func alipayPost(cfg realnameConfig, method, bizContent string) (map[string]interface{}, error) {
	privateKey, err := parseAlipayPrivateKey(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("应用私钥解析失败：%w", err)
	}

	params := map[string]string{
		"app_id":      cfg.AppID,
		"method":      method,
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"biz_content": bizContent,
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+params[k])
	}
	signSource := strings.Join(pairs, "&")

	digest := sha256.Sum256([]byte(signSource))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return nil, fmt.Errorf("签名失败：%w", err)
	}
	params["sign"] = base64.StdEncoding.EncodeToString(signature)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	client := &http.Client{Timeout: realnameHTTPTimeout}
	gateway := cfg.Gateway
	if gateway == "" {
		gateway = alipayDefaultGateway
	}
	httpResp, err := client.PostForm(gateway, form)
	if err != nil {
		return nil, fmt.Errorf("请求支付宝网关失败：%w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取支付宝响应失败：%w", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("支付宝响应解析失败")
	}
	return result, nil
}

func parseAlipayPrivateKey(raw string) (*rsa.PrivateKey, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, `\n`, "\n")
	if !strings.Contains(raw, "BEGIN") {
		raw = "-----BEGIN RSA PRIVATE KEY-----\n" + raw + "\n-----END RSA PRIVATE KEY-----"
	}
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, errors.New("PEM 解码失败")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("无法解析 RSA 私钥")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("不是 RSA 私钥")
	}
	return key, nil
}

// ========== 工具函数 ==========

func isValidIDCard(id string) bool {
	if len(id) == 15 {
		for _, ch := range id {
			if ch < '0' || ch > '9' {
				return false
			}
		}
		return true
	}
	if len(id) != 18 {
		return false
	}
	for i, ch := range id {
		if i < 17 {
			if ch < '0' || ch > '9' {
				return false
			}
		} else if !(ch >= '0' && ch <= '9' || ch == 'X') {
			return false
		}
	}
	// 校验码
	weights := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	codes := []byte{'1', '0', 'X', '9', '8', '7', '6', '5', '4', '3', '2'}
	sum := 0
	for i := 0; i < 17; i++ {
		sum += int(id[i]-'0') * weights[i]
	}
	return codes[sum%11] == id[17]
}

func maskRealName(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(name)
	if len(runes) <= 1 {
		return "*"
	}
	return string(runes[0]) + strings.Repeat("*", len(runes)-1)
}

func maskIDCard(id string) string {
	if len(id) < 8 {
		if id == "" {
			return ""
		}
		return strings.Repeat("*", len(id))
	}
	return id[:4] + strings.Repeat("*", len(id)-8) + id[len(id)-4:]
}

// ========== 快瞳 / 靓仔聚合认证扫码拍照认证单 ==========

// 认证单存储在 system_configs（group=realname, key=kt_face_<token>），value 格式：
// uid|realName|idCard|status|failReason|expiresAtUnix
// status: pending(待拍照) / processing(核验中) / passed / failed

const (
	realnameFaceStatusPending    = "pending"
	realnameFaceStatusProcessing = "processing"
	realnameFaceStatusPassed     = "passed"
	realnameFaceStatusFailed     = "failed"
	realnameFaceStatusUnknown    = "unknown"
	realnameFaceSessionTTL       = 10 * time.Minute
	realnameFaceTokenLen         = 32
	realnameFaceImageMaxBytes    = 2 * 1024 * 1024 // 拍照图片大小上限 2MB
	realnameFaceMaxAttempts      = 3               // 单个认证单允许的拍照核验次数
)

var realnameFaceAttempts sync.Map // token -> 已尝试次数（进程级，防止单认证单被无限重试）

// newRealnameFaceToken 生成不可猜测的认证单 token。
func newRealnameFaceToken() (string, error) {
	buf := make([]byte, realnameFaceTokenLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

type realnameFaceSession struct {
	Provider     string    `json:"provider"`
	OwnerType    string    `json:"ownerType"` // user / agent
	UserID       int64     `json:"ownerId"`
	RealName     string    `json:"realName"`
	IDCard       string    `json:"idCard"`
	ReturnURL    string    `json:"returnUrl,omitempty"`
	Status       string    `json:"status"`
	FailMsg      string    `json:"failMsg"`
	FailCode     string    `json:"failCode,omitempty"`
	HTTPStatus   int       `json:"httpStatus,omitempty"`
	ResponseCode int       `json:"responseCode,omitempty"`
	AuthMode     string    `json:"authMode,omitempty"`  // 小沐认证产品模式
	Mobile       string    `json:"mobile,omitempty"`    // 小沐三要素临时手机号
	OrderNo      string    `json:"orderNo,omitempty"`   // 上游订单号，用于对账与留档
	RecordID     string    `json:"recordId,omitempty"`  // 上游认证记录 ID，查询结果的 record 参数
	CertifyID    string    `json:"certifyId,omitempty"` // 靓仔人脸认证会话 ID
	AuthURL      string    `json:"authUrl,omitempty"`   // 上游 H5 认证地址
	ExpireAt     time.Time `json:"expireAt"`
}

func parseRealnameFaceSession(raw string) (realnameFaceSession, error) {
	var session realnameFaceSession
	if json.Unmarshal([]byte(raw), &session) == nil && session.UserID > 0 && session.RealName != "" {
		if session.OwnerType != "agent" {
			session.OwnerType = "user"
		}
		if session.Provider == "" {
			session.Provider = realnameProviderKuaitong
		}
		return session, nil
	}

	parts := strings.Split(raw, "|")
	if len(parts) != 6 && len(parts) != 7 {
		return realnameFaceSession{}, errors.New("认证单数据损坏")
	}
	session = realnameFaceSession{Provider: realnameProviderKuaitong, OwnerType: "user"}
	offset := 0
	if len(parts) == 7 {
		if parts[0] == "agent" || parts[0] == "user" {
			session.OwnerType = parts[0]
		}
		offset = 1
	}
	uid, err := strconv.ParseInt(parts[offset], 10, 64)
	if err != nil {
		return realnameFaceSession{}, errors.New("认证单数据损坏")
	}
	exp, err := strconv.ParseInt(parts[offset+5], 10, 64)
	if err != nil {
		return realnameFaceSession{}, errors.New("认证单数据损坏")
	}
	session.UserID = uid
	session.RealName = parts[offset+1]
	session.IDCard = parts[offset+2]
	session.Status = parts[offset+3]
	session.FailMsg = parts[offset+4]
	session.ExpireAt = time.Unix(exp, 0)
	return session, nil
}

func (s realnameFaceSession) encode() string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

func loadRealnameFaceSession(db *sql.DB, token string) (realnameFaceSession, error) {
	var raw string
	if err := db.QueryRow("SELECT value FROM system_configs WHERE `group` = ? AND `key` = ?", realnameGroup, "kt_face_"+token).Scan(&raw); err != nil {
		return realnameFaceSession{}, errors.New("认证单不存在或已过期")
	}
	session, err := parseRealnameFaceSession(raw)
	if err != nil {
		return realnameFaceSession{}, err
	}
	if time.Now().After(session.ExpireAt) {
		return realnameFaceSession{}, errors.New("认证单已过期，请重新发起")
	}
	return session, nil
}

func saveRealnameFaceSession(db *sql.DB, token string, session realnameFaceSession) {
	_, _ = db.Exec("UPDATE system_configs SET value = ? WHERE `group` = ? AND `key` = ?", session.encode(), realnameGroup, "kt_face_"+token)
}

// claimRealnameFaceSession 在事务内把待处理认证单切换为 processing，防止并发请求重复调用计费接口。
func claimRealnameFaceSession(db *sql.DB, token string) (realnameFaceSession, bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return realnameFaceSession{}, false, err
	}
	defer tx.Rollback()

	var raw string
	if err := tx.QueryRow(
		"SELECT value FROM system_configs WHERE `group` = ? AND `key` = ? FOR UPDATE",
		realnameGroup,
		"kt_face_"+token,
	).Scan(&raw); err != nil {
		return realnameFaceSession{}, false, errors.New("认证单不存在或已过期")
	}
	session, err := parseRealnameFaceSession(raw)
	if err != nil {
		return realnameFaceSession{}, false, err
	}
	if time.Now().After(session.ExpireAt) {
		return realnameFaceSession{}, false, errors.New("认证单已过期，请重新发起")
	}
	if session.Status != realnameFaceStatusPending && session.Status != realnameFaceStatusFailed {
		return session, false, nil
	}

	session.Status = realnameFaceStatusProcessing
	session.FailMsg = ""
	session.FailCode = ""
	session.HTTPStatus = 0
	session.ResponseCode = 0
	if _, err := tx.Exec(
		"UPDATE system_configs SET value = ? WHERE `group` = ? AND `key` = ?",
		session.encode(),
		realnameGroup,
		"kt_face_"+token,
	); err != nil {
		return realnameFaceSession{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return realnameFaceSession{}, false, err
	}
	return session, true, nil
}

func claimTencentRealnameQuerySession(db *sql.DB, token string) (realnameFaceSession, bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return realnameFaceSession{}, false, err
	}
	defer tx.Rollback()

	var raw string
	if err := tx.QueryRow(
		"SELECT value FROM system_configs WHERE `group` = ? AND `key` = ? FOR UPDATE",
		realnameGroup,
		"kt_face_"+token,
	).Scan(&raw); err != nil {
		return realnameFaceSession{}, false, errors.New("认证单不存在或已过期")
	}
	session, err := parseRealnameFaceSession(raw)
	if err != nil {
		return realnameFaceSession{}, false, err
	}
	if time.Now().After(session.ExpireAt) {
		return realnameFaceSession{}, false, errors.New("认证单已过期，请重新发起")
	}
	if session.Provider != realnameProviderTencent || session.Status != realnameFaceStatusPending {
		return session, false, nil
	}
	session.Status = realnameFaceStatusProcessing
	if _, err := tx.Exec(
		"UPDATE system_configs SET value = ? WHERE `group` = ? AND `key` = ?",
		session.encode(),
		realnameGroup,
		"kt_face_"+token,
	); err != nil {
		return realnameFaceSession{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return realnameFaceSession{}, false, err
	}
	return session, true, nil
}

// realnameFaceAttemptAllowed 限制单个认证单的核验次数，避免快瞳接口被刷。
func realnameFaceAttemptAllowed(token string) bool {
	actual, _ := realnameFaceAttempts.LoadOrStore(token, 0)
	count := actual.(int)
	if count >= realnameFaceMaxAttempts {
		return false
	}
	realnameFaceAttempts.Store(token, count+1)
	return true
}

func applyTencentRealnameError(session realnameFaceSession, err error) realnameFaceSession {
	httpStatus, responseCode, businessCode := tencentRealnameErrorMetadata(err)
	session.FailCode = businessCode
	session.HTTPStatus = httpStatus
	session.ResponseCode = responseCode
	if isTencentRealnameResultUnknown(err) {
		session.Status = realnameFaceStatusUnknown
		session.FailMsg = "腾讯增强人脸服务响应中断，认证结果无法确认，请返回电脑端重新发起认证；详细错误：" + err.Error()
		return session
	}
	session.Status = realnameFaceStatusFailed
	session.FailMsg = err.Error()
	return session
}

func tencentRealnameRecordFailureReason(session realnameFaceSession, err error) string {
	details := make([]string, 0, 3)
	if session.FailCode != "" {
		details = append(details, "错误代码："+session.FailCode)
	}
	if session.ResponseCode != 0 {
		details = append(details, "接口响应码："+strconv.Itoa(session.ResponseCode))
	}
	if session.HTTPStatus != 0 {
		details = append(details, "HTTP状态："+strconv.Itoa(session.HTTPStatus))
	}
	lines := make([]string, 0, 3)
	if session.FailMsg != "" {
		lines = append(lines, session.FailMsg)
	}
	if len(details) > 0 {
		lines = append(lines, strings.Join(details, "；"))
	}
	if body := tencentRealnameErrorResponseBody(err); body != "" {
		lines = append(lines, "上游响应正文："+body)
	}
	return strings.Join(lines, "\n")
}

func realnameFacePassedData(session realnameFaceSession) gin.H {
	return gin.H{
		"status":   "passed",
		"realName": maskRealName(session.RealName),
		"idCard":   maskIDCard(session.IDCard),
	}
}

func realnameFaceFailureData(session realnameFaceSession) gin.H {
	data := gin.H{
		"status": "failed",
		"reason": session.FailMsg,
	}
	if session.FailCode != "" {
		data["errorCode"] = session.FailCode
	}
	if session.HTTPStatus != 0 {
		data["upstreamHttpStatus"] = session.HTTPStatus
	}
	if session.ResponseCode != 0 {
		data["upstreamResponseCode"] = session.ResponseCode
	}
	return data
}

// realnameFaceQuery 按主体轮询认证单状态；靓仔认证单在这里调用上游 faceQuery。
func realnameFaceQuery(c *gin.Context, db *sql.DB, cfg realnameConfig, ownerType string, ownerID uint) {
	token := strings.TrimSpace(c.Query("faceToken"))
	if token == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "缺少 faceToken"})
		return
	}
	session, err := loadRealnameFaceSession(db, token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if session.OwnerType != ownerType || session.UserID != int64(ownerID) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "认证单号不匹配"})
		return
	}
	if session.Provider == realnameProviderTencent && session.Status == realnameFaceStatusPending {
		if cfg.Provider != realnameProviderTencent || cfg.TencentAPIKey == "" || cfg.TencentAPISecret == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "靓仔聚合认证服务不可用"})
			return
		}
		// 靓仔使用本站拍照流程时，不生成 certifyId，认证结果在提交时已写入 session.Status，
		// 无需调用上游查询接口，直接返回本地状态即可。
		if session.CertifyID == "" {
			// 本站拍照流程，等待用户扫码提交人脸图片
		} else {
			claimedSession, claimed, claimErr := claimTencentRealnameQuerySession(db, token)
			if claimErr != nil {
				c.JSON(http.StatusOK, gin.H{"code": 400, "msg": claimErr.Error()})
				return
			}
			if !claimed {
				session = claimedSession
			} else {
				session = claimedSession
				fmt.Printf("[靓仔认证] 开始查询认证结果\n")
				result, pending, queryErr := tencentRealnameQuery(cfg, session.CertifyID)
				fmt.Printf("[靓仔认证] 查询结果: pending=%v, passed=%v, reason=%s, err=%v\n", pending, result.Passed, result.Reason, queryErr)
				switch {
				case queryErr != nil:
					if isTencentRealnameResultUnknown(queryErr) {
						session.Status = realnameFaceStatusPending
						saveRealnameFaceSession(db, token, session)
						c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "认证结果查询中", "data": gin.H{"status": realnameFaceStatusPending}})
						return
					}
					session = applyTencentRealnameError(session, queryErr)
					session.Status = realnameFaceStatusFailed
					saveRealnameFaceSession(db, token, session)
					writeRealnameRecord(db, session.OwnerType, session.UserID, session.Provider, session.RealName, session.IDCard, realnameFaceStatusFailed, tencentRealnameRecordFailureReason(session, queryErr), session.CertifyID, "")
				case pending:
					session.Status = realnameFaceStatusPending
					saveRealnameFaceSession(db, token, session)
					c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "认证未完成", "data": gin.H{"status": realnameFaceStatusPending}})
					return
				case !result.Passed:
					session.Status = realnameFaceStatusFailed
					session.FailMsg = result.Reason
					saveRealnameFaceSession(db, token, session)
					recordReason := result.Detail
					if recordReason == "" {
						recordReason = result.Reason
					}
					writeRealnameRecord(db, session.OwnerType, session.UserID, session.Provider, session.RealName, session.IDCard, realnameFaceStatusFailed, recordReason, session.CertifyID, result.Score)
				default:
					session.Status = realnameFaceStatusPassed
					saveRealnameFaceSession(db, token, session)
					table := realnameOwnerTable(session.OwnerType)
					_, _ = db.Exec("UPDATE "+table+" SET real_name = ?, real_id_card = ?, realname_at = NOW() WHERE id = ?", session.RealName, session.IDCard, session.UserID)
					writeRealnameRecord(db, session.OwnerType, session.UserID, session.Provider, session.RealName, session.IDCard, realnameFaceStatusPassed, "", session.CertifyID, result.Score)
				}
			}
		}
	}
	switch session.Status {
	case realnameFaceStatusPassed:
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "实名认证成功",
			"data": realnameFacePassedData(session),
		})
	case realnameFaceStatusFailed, realnameFaceStatusUnknown:
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "认证未通过", "data": realnameFaceFailureData(session)})
	default:
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "认证未完成", "data": gin.H{"status": "pending"}})
	}
}

// RealnameFacePage 扫码落地页：返回无感拍照 HTML（无需登录，token 即凭证）。
func RealnameFacePage(c *gin.Context) {
	token := strings.TrimSpace(c.Query("t"))
	if token == "" {
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(realnameFaceErrorHTML("链接无效，请重新扫码")))
		return
	}

	cfg, err := config.LoadDBConfig()
	if err != nil {
		c.Data(http.StatusInternalServerError, "text/html; charset=utf-8", []byte(realnameFaceErrorHTML("系统未配置")))
		return
	}
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.Data(http.StatusInternalServerError, "text/html; charset=utf-8", []byte(realnameFaceErrorHTML("数据库连接失败")))
		return
	}
	defer db.Close()
	if err := ensureSystemConfigStorage(db); err != nil {
		c.Data(http.StatusInternalServerError, "text/html; charset=utf-8", []byte(realnameFaceErrorHTML("初始化配置失败")))
		return
	}

	session, err := loadRealnameFaceSession(db, token)
	if err != nil {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(realnameFaceErrorHTML(err.Error())))
		return
	}
	if session.Status == realnameFaceStatusPassed {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(realnameFaceResultHTML("✓", "实名认证成功", "请返回电脑端继续操作", true)))
		return
	}
	if session.Status == realnameFaceStatusFailed {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(realnameFaceResultHTML("✕", "认证未通过", htmlEscapeString(session.FailMsg)+"，可点击下方按钮重试", false)))
		return
	}
	if session.Status == realnameFaceStatusUnknown {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(realnameFaceErrorHTML(htmlEscapeString(session.FailMsg))))
		return
	}

	if session.Provider == realnameProviderXiaomu {
		// 小沐认证单不走本站拍照采集，避免 token 误入拍照链路。
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(realnameFaceErrorHTML("该认证单无需拍照，请返回原页面继续认证")))
		return
	}

	page := strings.ReplaceAll(realnameFacePageHTML, "__TOKEN__", htmlEscapeString(token))
	page = strings.ReplaceAll(page, "__NAME__", htmlEscapeString(session.RealName))
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
}

type realnameFaceSubmitRequest struct {
	Token     string `json:"token"`
	ImageData string `json:"imageData"`
}

// RealnameFaceSubmit 拍照页提交抓拍照片，后端调用认证单对应服务商完成核验。
func RealnameFaceSubmit(c *gin.Context) {
	var req realnameFaceSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	req.ImageData = strings.TrimSpace(req.ImageData)
	if req.Token == "" || req.ImageData == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	// 校验图片：必须是 data:image/...;base64, 前缀且解码后不超过 2MB
	if !strings.HasPrefix(req.ImageData, "data:image/") {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "图片格式不正确"})
		return
	}
	commaIdx := strings.Index(req.ImageData, ";base64,")
	if commaIdx < 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "图片格式不正确"})
		return
	}
	raw, err := base64.StdEncoding.DecodeString(req.ImageData[commaIdx+8:])
	if err != nil || len(raw) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "图片数据无效"})
		return
	}
	if len(raw) > realnameFaceImageMaxBytes {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "图片超过 2MB 限制，请重试"})
		return
	}

	cfg, err := config.LoadDBConfig()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "系统未配置"})
		return
	}
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureSystemConfigStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化配置失败"})
		return
	}
	if err := ensureRealnameStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化实名存储失败"})
		return
	}

	session, err := loadRealnameFaceSession(db, req.Token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if session.Status == realnameFaceStatusPassed {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "实名认证成功", "data": gin.H{"status": "passed"}})
		return
	}
	if session.Status == realnameFaceStatusProcessing {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "核验中，请稍候", "data": gin.H{"status": "pending"}})
		return
	}
	if session.Status == realnameFaceStatusUnknown {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": session.FailMsg, "data": realnameFaceFailureData(session)})
		return
	}
	rnCfg, err := loadRealnameConfig(db)
	if err != nil || !rnCfg.Enabled || (session.Provider != realnameProviderKuaitong && session.Provider != realnameProviderTencent) || rnCfg.Provider != session.Provider {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "实名认证服务不可用，请联系管理员"})
		return
	}

	claimedSession, claimed, err := claimRealnameFaceSession(db, req.Token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if !claimed {
		if claimedSession.Status == realnameFaceStatusPassed {
			c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "实名认证成功", "data": gin.H{"status": "passed"}})
		} else {
			c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "核验中，请稍候", "data": gin.H{"status": "pending"}})
		}
		return
	}
	session = claimedSession
	if !realnameFaceAttemptAllowed(req.Token) {
		session.Status = realnameFaceStatusFailed
		session.FailMsg = "重试次数过多，请返回电脑端重新发起认证"
		saveRealnameFaceSession(db, req.Token, session)
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": session.FailMsg})
		return
	}

	var message, serialNo, score string
	var verr error
	var faceResult tencentRealnameFaceResult

	if session.Provider == realnameProviderKuaitong {
		message, serialNo, score, verr = kuaitongVerifyFace(rnCfg, session.RealName, session.IDCard, req.ImageData)
		faceResult = tencentRealnameFaceResult{
			Passed:   message == "",
			Reason:   message,
			Detail:   message,
			SerialNo: serialNo,
			Score:    score,
		}
	} else if session.Provider == realnameProviderTencent {
		faceResult, verr = tencentRealnameVerifyWithImage(rnCfg, session.RealName, session.IDCard, req.ImageData)
		message = faceResult.Reason
		serialNo = faceResult.SerialNo
		score = faceResult.Score
	}

	switch {
	case verr != nil:
		// 请求级失败（网络/网关异常）未产生确定结果，保留原有重试行为。
		session.Status = realnameFaceStatusPending
		saveRealnameFaceSession(db, req.Token, session)
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "核验请求失败：" + verr.Error() + "，请重试"})
		return
	case !faceResult.Passed:
		session.Status = realnameFaceStatusFailed
		session.FailMsg = faceResult.Reason
		saveRealnameFaceSession(db, req.Token, session)
		recordReason := faceResult.Detail
		if recordReason == "" {
			recordReason = faceResult.Reason
		}
		writeRealnameRecord(db, session.OwnerType, session.UserID, session.Provider, session.RealName, session.IDCard, realnameFaceStatusFailed, recordReason, serialNo, score)
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "实名核验未通过：" + faceResult.Reason})
		return
	}

	// 靓仔认证通过后，如果返回了 certify_id，先查询最终结果再返回
	if session.Provider == realnameProviderTencent && faceResult.CertifyID != "" {
		fmt.Printf("[靓仔认证] 核验接口返回成功，开始查询最终结果\n")
		queryResult, pending, queryErr := tencentRealnameQuery(rnCfg, faceResult.CertifyID)
		fmt.Printf("[靓仔认证] 认证结果查询返回: pending=%v, passed=%v, reason=%s, err=%v\n", pending, queryResult.Passed, queryResult.Reason, queryErr)

		// 处理查询结果
		switch {
		case queryErr != nil:
			// 查询失败，按原结果处理但记录查询错误
			fmt.Printf("[靓仔认证] 查询失败: %v，使用核验接口结果\n", queryErr)
		case pending:
			// 仍在处理中，返回 pending 状态
			session.Status = realnameFaceStatusPending
			session.CertifyID = faceResult.CertifyID
			saveRealnameFaceSession(db, req.Token, session)
			c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "认证结果查询中，请稍候", "data": gin.H{"status": "pending"}})
			return
		case !queryResult.Passed:
			// 查询结果为不通过
			session.Status = realnameFaceStatusFailed
			session.FailMsg = queryResult.Reason
			saveRealnameFaceSession(db, req.Token, session)
			queryRecordReason := queryResult.Detail
			if queryRecordReason == "" {
				queryRecordReason = queryResult.Reason
			}
			writeRealnameRecord(db, session.OwnerType, session.UserID, session.Provider, session.RealName, session.IDCard, realnameFaceStatusFailed, queryRecordReason, queryResult.SerialNo, queryResult.Score)
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "实名核验未通过：" + queryResult.Reason})
			return
		default:
			// 查询结果为通过，更新 serialNo 和 score
			if queryResult.SerialNo != "" {
				serialNo = queryResult.SerialNo
			}
			if queryResult.Score != "" {
				score = queryResult.Score
			}
		}
	}

	session.Status = realnameFaceStatusPassed
	saveRealnameFaceSession(db, req.Token, session)
	table := "users"
	if session.OwnerType == "agent" {
		table = "agents"
	}
	_, _ = db.Exec("UPDATE "+table+" SET real_name = ?, real_id_card = ?, realname_at = NOW() WHERE id = ?",
		session.RealName, session.IDCard, session.UserID)
	writeRealnameRecord(db, session.OwnerType, session.UserID, session.Provider, session.RealName, session.IDCard, realnameFaceStatusPassed, "", serialNo, score)
	realnameFaceAttempts.Delete(req.Token)
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "实名认证成功", "data": gin.H{"status": "passed"}})
}

func htmlEscapeString(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(s)
}

const realnameFacePageStyle = `
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, "PingFang SC", "Helvetica Neue", sans-serif; background: #0f172a; color: #e2e8f0; min-height: 100vh; display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 24px; }
.wrap { width: 100%; max-width: 420px; text-align: center; }
h1 { font-size: 20px; font-weight: 600; margin-bottom: 8px; }
.tip { font-size: 13px; color: #94a3b8; margin-bottom: 20px; line-height: 1.6; }
.camera-box { position: relative; width: 100%; border-radius: 16px; overflow: hidden; background: #1e293b; aspect-ratio: 3/4; }
video { width: 100%; height: 100%; object-fit: cover; transform: scaleX(-1); }
.frame { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; pointer-events: none; }
.frame::before { content: ""; width: 62%; aspect-ratio: 3/4; border: 3px solid rgba(34, 211, 238, .85); border-radius: 50%; box-shadow: 0 0 0 999px rgba(2, 6, 23, .35); }
.status { margin-top: 18px; font-size: 14px; color: #67e8f9; min-height: 22px; }
.btn { display: inline-block; margin-top: 16px; padding: 12px 40px; border: none; border-radius: 999px; background: linear-gradient(135deg, #06b6d4, #3b82f6); color: #fff; font-size: 15px; cursor: pointer; }
.btn:disabled { opacity: .5; }
.result-icon { font-size: 56px; margin-bottom: 16px; }
.result-icon.ok { color: #34d399; }
.result-icon.bad { color: #f87171; }
.result-title { font-size: 20px; font-weight: 600; margin-bottom: 10px; }
.result-desc { font-size: 14px; color: #94a3b8; line-height: 1.7; }
`

func realnameFaceErrorHTML(msg string) string {
	return "<!DOCTYPE html><html lang=" + `"zh-CN"` + "><head><meta charset=" + `"UTF-8"` + "><meta name=" + `"viewport"` + " content=" + `"width=device-width,initial-scale=1"` + "><title>实名认证</title><style>" + realnameFacePageStyle + "</style></head><body><div class=" + `"wrap"` + "><div class=" + `"result-icon bad"` + ">✕</div><div class=" + `"result-title"` + ">无法认证</div><div class=" + `"result-desc"` + ">" + msg + "</div></div></body></html>"
}

func realnameFaceResultHTML(icon, title, desc string, ok bool) string {
	cls := "ok"
	if !ok {
		cls = "bad"
	}
	retry := ""
	if !ok {
		retry = "<button class=" + `"btn"` + " onclick=" + `"location.reload()"` + ">重新拍照</button>"
	}
	return "<!DOCTYPE html><html lang=" + `"zh-CN"` + "><head><meta charset=" + `"UTF-8"` + "><meta name=" + `"viewport"` + " content=" + `"width=device-width,initial-scale=1"` + "><title>实名认证</title><style>" + realnameFacePageStyle + "</style></head><body><div class=" + `"wrap"` + "><div class=" + `"result-icon ` + cls + `"` + ">" + icon + "</div><div class=" + `"result-title"` + ">" + title + "</div><div class=" + `"result-desc"` + ">" + desc + "</div>" + retry + "</div></body></html>"
}

// 无感拍照页：自动申请摄像头，画面稳定后自动抓拍上传，全程无需用户点击。
const realnameFacePageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1,user-scalable=no">
<title>人脸核验</title>
<style>` + realnameFacePageStyle + `</style>
</head>
<body>
<div class="wrap">
  <h1>人脸核验</h1>
  <div class="tip">正在核验 <b>__NAME__</b> 的实名信息<br>请正对屏幕，保持光线充足，将面部置于框内</div>
  <div class="camera-box">
    <video id="video" autoplay playsinline muted></video>
    <div class="frame"></div>
  </div>
  <div class="status" id="status">正在启动摄像头...</div>
</div>
<canvas id="canvas" style="display:none"></canvas>
<script>
(function () {
  var TOKEN = "__TOKEN__";
  var video = document.getElementById("video");
  var canvas = document.getElementById("canvas");
  var statusEl = document.getElementById("status");
  var submitted = false;

  function setStatus(text) { statusEl.textContent = text; }

  function escapeHTML(value) {
    var chars = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };
    return String(value).replace(/[&<>"']/g, function (ch) { return chars[ch]; });
  }

  function showResult(ok, title, desc, retry) {
    var html = '<div class="wrap"><div class="result-icon ' + (ok ? "ok" : "bad") + '">' + (ok ? "✓" : "✕") + '</div>' +
      '<div class="result-title">' + escapeHTML(title) + '</div><div class="result-desc">' + escapeHTML(desc) + '</div>';
    if (retry) html += '<button class="btn" onclick="location.reload()">重新拍照</button>';
    html += '</div>';
    document.body.innerHTML = html;
  }

  function imageByteLength(imageData) {
    var encoded = imageData.slice(imageData.indexOf(",") + 1);
    var padding = encoded.slice(-2) === "==" ? 2 : (encoded.slice(-1) === "=" ? 1 : 0);
    return Math.floor(encoded.length * 3 / 4) - padding;
  }

  function capture() {
    var w = video.videoWidth, h = video.videoHeight;
    if (!w || !h) return null;
    var presets = [[640, 0.76], [560, 0.7], [480, 0.64]];
    for (var i = 0; i < presets.length; i++) {
      var maxW = presets[i][0];
      var scale = w > maxW ? maxW / w : 1;
      canvas.width = Math.round(w * scale);
      canvas.height = Math.round(h * scale);
      var ctx = canvas.getContext("2d");
      ctx.translate(canvas.width, 0);
      ctx.scale(-1, 1);
      ctx.drawImage(video, 0, 0, canvas.width, canvas.height);
      var image = canvas.toDataURL("image/jpeg", presets[i][1]);
      if (imageByteLength(image) <= 500 * 1024) return image;
    }
    return null;
  }

  function submit(imageData) {
    if (submitted) return;
    submitted = true;
    setStatus("正在核验，请稍候...");
    fetch("/api/realname/face/submit", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token: TOKEN, imageData: imageData })
    }).then(function (r) { return r.json(); }).then(function (res) {
      if (res.code === 200) {
        showResult(true, "实名认证成功", "请返回电脑端继续操作", false);
      } else if (res.code === 500) {
        submitted = false;
        setStatus(res.msg || "核验失败，正在重试...");
        setTimeout(loop, 2000);
      } else {
        showResult(false, "认证未通过", res.msg || "请返回电脑端重新发起认证", true);
      }
    }).catch(function () {
      submitted = false;
      setStatus("网络异常，正在重试...");
      setTimeout(loop, 2000);
    });
  }

  function loop() {
    if (submitted) return;
    var image = capture();
    if (image && imageByteLength(image) <= 500 * 1024) {
      submit(image);
    } else {
      setTimeout(loop, 600);
    }
  }

  if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
    setStatus("当前浏览器不支持摄像头，请使用微信/支付宝内置浏览器或系统相机扫码");
    return;
  }
  navigator.mediaDevices.getUserMedia({ video: { facingMode: "user", width: { ideal: 720 } }, audio: false })
    .then(function (stream) {
      video.srcObject = stream;
      setStatus("请正对屏幕，正在准备抓拍...");
      setTimeout(function () { setStatus("正在抓拍..."); loop(); }, 1600);
    })
    .catch(function () {
      setStatus("摄像头权限被拒绝，请在浏览器设置中允许后刷新重试");
    });
})();
</script>
</body>
</html>`

// realnameCertifyIDExpired 保留给将来清理 pending 单使用。
var _ = realnameCertifyIDMaxKeep
