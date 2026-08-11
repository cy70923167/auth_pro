package handler

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
)

type licenseVerifyRequest struct {
	AppKey     string `json:"appKey" binding:"required"`
	Domain     string `json:"domain"`
	ServerIP   string `json:"serverIp"`
	LicenseKey string `json:"licenseKey"`
	Timestamp  int64  `json:"timestamp" binding:"required"`
	Sign       string `json:"sign" binding:"required"`
}

type matchedLicense struct {
	ID        int64
	PlanID    int64
	PlanName  string
	Type      string
	Status    string
	ExpiredAt sql.NullTime
}

var (
	appLicenseRequiredMu sync.Mutex
	appLicenseRequiredOK bool
)

func ensureAppLicenseRequiredColumn(db *sql.DB) error {
	appLicenseRequiredMu.Lock()
	defer appLicenseRequiredMu.Unlock()
	if appLicenseRequiredOK {
		return nil
	}

	var exists int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'apps'
		  AND COLUMN_NAME = 'license_required'
	`).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		_, err := db.Exec(`
			ALTER TABLE apps
			ADD COLUMN license_required TINYINT(1) NOT NULL DEFAULT 1
			COMMENT '是否要求授权验证: 1要求 0免授权' AFTER enabled
		`)
		if err != nil {
			var mysqlErr *mysql.MySQLError
			if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1060 {
				return err
			}
		}
	}
	appLicenseRequiredOK = true
	return nil
}

// LicenseVerify 公开授权校验接口，供业务系统 SDK 调用。
func LicenseVerify(c *gin.Context) {
	var req licenseVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误", "data": gin.H{"result": "fail", "reason": "bad_request"}})
		return
	}

	rawDomain := strings.TrimSpace(req.Domain)
	rawServerIP := strings.TrimSpace(req.ServerIP)
	rawLicenseKey := strings.TrimSpace(req.LicenseKey)

	req.AppKey = strings.TrimSpace(req.AppKey)
	req.Domain = normalizeLicenseTarget(rawDomain)
	req.ServerIP = rawServerIP
	req.LicenseKey = rawLicenseKey
	req.Sign = strings.ToLower(strings.TrimSpace(req.Sign))

	cfg, err := config.LoadDBConfig()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "系统未配置", "data": gin.H{"result": "fail", "reason": "system_not_configured"}})
		return
	}

	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败", "data": gin.H{"result": "fail", "reason": "db_connect_failed"}})
		return
	}
	defer db.Close()
	if err := ensureAppLicenseRequiredColumn(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化应用授权开关失败", "data": gin.H{"result": "fail", "reason": "schema_init_failed"}})
		return
	}

	var appID int64
	var appName, appSecret string
	var licenseRequired bool
	err = db.QueryRow("SELECT id, app_name, app_secret, license_required FROM apps WHERE app_key = ? AND enabled = 1", req.AppKey).Scan(&appID, &appName, &appSecret, &licenseRequired)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "应用不存在或已禁用", "data": gin.H{"result": "fail", "reason": "app_not_found"}})
		return
	}

	if req.Timestamp <= 0 || absInt64(time.Now().Unix()-req.Timestamp) > 600 {
		writeVerifyLog(db, sql.NullInt64{}, appID, req.Domain, req.ServerIP, c.ClientIP(), "fail", "invalid_timestamp", c.GetHeader("User-Agent"))
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "请求已过期", "data": gin.H{"result": "fail", "reason": "invalid_timestamp"}})
		return
	}

	signTarget := licenseVerifySignTarget(req)
	if signTarget == "" {
		writeVerifyLog(db, sql.NullInt64{}, appID, req.Domain, req.ServerIP, c.ClientIP(), "fail", "empty_target", c.GetHeader("User-Agent"))
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "授权目标不能为空", "data": gin.H{"result": "fail", "reason": "empty_target"}})
		return
	}

	if !licenseVerifySignValid(req, appSecret, rawDomain, rawServerIP, rawLicenseKey) {
		writeVerifyLog(db, sql.NullInt64{}, appID, req.Domain, req.ServerIP, c.ClientIP(), "fail", "invalid_sign", c.GetHeader("User-Agent"))
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "签名错误", "data": gin.H{"result": "fail", "reason": "invalid_sign"}})
		return
	}

	if isLicenseTargetBlacklisted(db, appID, req.Domain, req.ServerIP) {
		writeVerifyLog(db, sql.NullInt64{}, appID, req.Domain, req.ServerIP, c.ClientIP(), "blacklisted", "target_blacklisted", c.GetHeader("User-Agent"))
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "授权目标已被拉黑", "data": gin.H{"result": "blacklisted", "reason": "target_blacklisted"}})
		return
	}

	if !licenseRequired {
		writeVerifyLog(db, sql.NullInt64{}, appID, req.Domain, req.ServerIP, c.ClientIP(), "pass", "license_not_required", c.GetHeader("User-Agent"))
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "应用无需授权验证",
			"data": gin.H{
				"result":          "pass",
				"appName":         appName,
				"licenseRequired": false,
			},
		})
		return
	}

	license, ok, reason := findMatchedLicense(db, appID, req)
	if !ok {
		writeVerifyLog(db, sql.NullInt64{}, appID, req.Domain, req.ServerIP, c.ClientIP(), "fail", reason, c.GetHeader("User-Agent"))
		if reason == "license_not_found" && isPiracyDetectionEnabled() {
			recordPiracyHit(db, appID, req.Domain, req.ServerIP)
		}
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "授权无效", "data": gin.H{"result": "fail", "reason": reason}})
		return
	}

	// 实名认证门槛：应用要求实名且授权归属未实名用户时拒绝
	if required, verified, err := userRealnameRequired(db, appID, license.ID); err == nil && required && !verified {
		writeVerifyLog(db, sql.NullInt64{Int64: license.ID, Valid: true}, appID, req.Domain, req.ServerIP, c.ClientIP(), "fail", "realname_required", c.GetHeader("User-Agent"))
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "该应用要求实名认证，请先在用户中心完成实名后再安装", "data": gin.H{"result": "fail", "reason": "realname_required"}})
		return
	}

	if license.Status == "revoked" {
		writeVerifyLog(db, sql.NullInt64{Int64: license.ID, Valid: true}, appID, req.Domain, req.ServerIP, c.ClientIP(), "fail", "license_revoked", c.GetHeader("User-Agent"))
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "授权已禁用", "data": gin.H{"result": "fail", "reason": "license_revoked"}})
		return
	}
	if license.Status == "expired" || (license.ExpiredAt.Valid && !license.ExpiredAt.Time.After(time.Now())) {
		writeVerifyLog(db, sql.NullInt64{Int64: license.ID, Valid: true}, appID, req.Domain, req.ServerIP, c.ClientIP(), "expired", "license_expired", c.GetHeader("User-Agent"))
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "授权已过期", "data": gin.H{"result": "expired", "reason": "license_expired"}})
		return
	}

	writeVerifyLog(db, sql.NullInt64{Int64: license.ID, Valid: true}, appID, req.Domain, req.ServerIP, c.ClientIP(), "pass", "", c.GetHeader("User-Agent"))
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "授权有效",
		"data": licenseVerifySuccessData(appName, license),
	})
}

func findMatchedLicense(db *sql.DB, appID int64, req licenseVerifyRequest) (matchedLicense, bool, string) {
	rows, err := db.Query(`
		SELECT l.id, COALESCE(l.plan_id, 0), COALESCE(p.name, ''), l.type, l.status, l.expired_at,
		       COALESCE(ld.domain, ''), COALESCE(ld.is_wildcard, 0)
		FROM licenses l
		LEFT JOIN license_plans p ON p.id = l.plan_id AND p.app_id = l.app_id
		LEFT JOIN license_domains ld ON ld.license_id = l.id
		WHERE l.app_id = ?
		  AND (
		    (l.type = 'key' AND l.license_key = ?)
		    OR (l.type IN ('domain', 'wildcard', 'ip'))
		  )
		ORDER BY l.created_at DESC
	`, appID, req.LicenseKey)
	if err != nil {
		return matchedLicense{}, false, "query_failed"
	}
	defer rows.Close()

	for rows.Next() {
		var item matchedLicense
		var target string
		var isWildcard int
		if err := rows.Scan(
			&item.ID, &item.PlanID, &item.PlanName, &item.Type, &item.Status, &item.ExpiredAt, &target, &isWildcard,
		); err != nil {
			continue
		}
		target = normalizeLicenseTarget(target)
		if licenseRowMatchesRequest(item.Type, target, isWildcard == 1, req) {
			return item, true, ""
		}
	}

	return matchedLicense{}, false, "license_not_found"
}

func licenseRowMatchesRequest(licenseType, storedTarget string, isWildcard bool, req licenseVerifyRequest) bool {
	switch licenseType {
	case "key":
		return req.LicenseKey != ""
	case "domain":
		return req.Domain != "" && storedTarget == req.Domain
	case "wildcard":
		return req.Domain != "" && isWildcard && wildcardDomainMatch(storedTarget, req.Domain)
	case "ip":
		return storedTarget != "" && (storedTarget == req.ServerIP || storedTarget == req.Domain)
	default:
		return false
	}
}

func isLicenseTargetBlacklisted(db *sql.DB, appID int64, domain, serverIP string) bool {
	candidates := []struct {
		typeName string
		value    string
	}{
		{typeName: "domain", value: domain},
		{typeName: "ip", value: serverIP},
	}
	if net.ParseIP(domain) != nil {
		candidates = append(candidates, struct {
			typeName string
			value    string
		}{typeName: "ip", value: domain})
	}

	for _, item := range candidates {
		if item.value == "" {
			continue
		}
		var count int
		_ = db.QueryRow("SELECT COUNT(*) FROM piracy_blacklist WHERE app_id = ? AND type = ? AND value = ?", appID, item.typeName, item.value).Scan(&count)
		if count > 0 {
			return true
		}
	}
	return false
}

func writeVerifyLog(db *sql.DB, licenseID sql.NullInt64, appID int64, domain, serverIP, clientIP, result, reason, userAgent string) {
	_, _ = db.Exec(`
		INSERT INTO verify_logs (license_id, app_id, domain, server_ip, client_ip, result, fail_reason, user_agent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, licenseID, appID, domain, serverIP, clientIP, result, reason, userAgent)
}

func recordPiracyHit(db *sql.DB, appID int64, domain, serverIP string) {
	target := domain
	if target == "" {
		target = serverIP
	}
	if target == "" {
		return
	}

	result, err := db.Exec(`
		UPDATE piracy_records
		SET hit_count = hit_count + 1, last_seen = NOW(), server_ip = ?
		WHERE app_id = ? AND domain = ?
	`, serverIP, appID, target)
	if err == nil {
		if affected, affectedErr := result.RowsAffected(); affectedErr == nil && affected > 0 {
			return
		}
	}

	_, _ = db.Exec(`
		INSERT INTO piracy_records (app_id, domain, server_ip, status, hit_count, first_seen, last_seen)
		VALUES (?, ?, ?, 'discovered', 1, NOW(), NOW())
	`, appID, target, serverIP)
}

func licenseVerifySignTarget(req licenseVerifyRequest) string {
	if req.LicenseKey != "" {
		return req.LicenseKey
	}
	if req.Domain != "" {
		return req.Domain
	}
	return req.ServerIP
}

func licenseVerifySignValid(req licenseVerifyRequest, appSecret, rawDomain, rawServerIP, rawLicenseKey string) bool {
	targets := []string{licenseVerifySignTarget(req), licenseVerifyRawSignTarget(rawDomain, rawServerIP, rawLicenseKey)}
	for _, target := range targets {
		if target == "" {
			continue
		}
		if req.Sign == licenseVerifyMD5(req.AppKey+target+int64ToString(req.Timestamp)+appSecret) {
			return true
		}
	}
	return false
}

func licenseVerifyRawSignTarget(rawDomain, rawServerIP, rawLicenseKey string) string {
	if rawLicenseKey != "" {
		return rawLicenseKey
	}
	if rawDomain != "" {
		return rawDomain
	}
	return rawServerIP
}

func licenseVerifyMD5(text string) string {
	sum := md5.Sum([]byte(text))
	return hex.EncodeToString(sum[:])
}

func wildcardDomainMatch(pattern, domain string) bool {
	pattern = normalizeLicenseTarget(pattern)
	domain = normalizeLicenseTarget(domain)
	if !strings.HasPrefix(pattern, "*.") {
		return pattern == domain
	}
	suffix := strings.TrimPrefix(pattern, "*")
	return strings.HasSuffix(domain, suffix) && domain != strings.TrimPrefix(pattern, "*.")
}

func normalizeLicenseTarget(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "https://")
	if idx := strings.Index(value, "/"); idx >= 0 {
		value = value[:idx]
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	return value
}

func int64ToString(value int64) string {
	return strconv.FormatInt(value, 10)
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func licenseVerifySuccessData(appName string, license matchedLicense) gin.H {
	return gin.H{
		"result":   "pass",
		"appName":  appName,
		"planId":   license.PlanID,
		"planName": license.PlanName,
		"type":     license.Type,
		"expireAt": formatVerifyExpireAt(license.ExpiredAt),
	}
}

func formatVerifyExpireAt(expiredAt sql.NullTime) string {
	if !expiredAt.Valid {
		return "永久"
	}
	return expiredAt.Time.Format("2006-01-02 15:04:05")
}
