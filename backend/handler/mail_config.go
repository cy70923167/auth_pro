package handler

import (
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	mailGroup = "mail"

	defaultPurchaseSubject = "{{appName}} 开通成功通知"
	defaultPurchaseContent = "您好，{{ownerName}}：\n\n您购买的应用 {{appName}} 已开通成功。\n\n开通时间：{{openedAt}}\n到期时间：{{expiresAt}}\n授权码：{{licenseKey}}\n\n请妥善保存相关信息。"
	defaultExpireSubject   = "{{appName}} 即将到期提醒"
	defaultExpireContent   = "您好，{{ownerName}}：\n\n您的应用 {{appName}} 将于 {{expiresAt}} 到期，剩余 {{daysLeft}} 天。\n\n请及时续费，以免影响正常使用。"
	defaultOpenedSubject   = "{{appName}} 授权开通通知"
	defaultOpenedContent   = "您好，{{ownerName}}：\n\n管理员已为您开通应用 {{appName}} 的授权。\n\n开通时间：{{openedAt}}\n到期时间：{{expiresAt}}\n授权码：{{licenseKey}}\n\n请妥善保存相关信息。"
)

var (
	mailStorageMu          sync.Mutex
	mailStorageReady       bool
	mailWorkerOnce         sync.Once
	errInvalidMailTemplate = errors.New("邮件模板类型不正确")
)

type mailConfig struct {
	Provider               string `json:"provider"`
	SMTPHost               string `json:"smtpHost"`
	SMTPPort               int    `json:"smtpPort"`
	SMTPSecure             string `json:"smtpSecure"`
	SMTPUsername           string `json:"smtpUsername"`
	SMTPPassword           string `json:"smtpPassword,omitempty"`
	PasswordSet            bool   `json:"passwordSet"`
	SMTPFromEmail          string `json:"smtpFromEmail"`
	SMTPFromName           string `json:"smtpFromName"`
	EnabledPurchaseSuccess bool   `json:"enabledPurchaseSuccess"`
	EnabledExpireReminder  bool   `json:"enabledExpireReminder"`
	EnabledLicenseOpened   bool   `json:"enabledLicenseOpened"`
	ExpireRemindDays       string `json:"expireRemindDays"`
	PurchaseSubject        string `json:"purchaseSubject"`
	PurchaseContent        string `json:"purchaseContent"`
	PurchaseContentType    string `json:"purchaseContentType"`
	ExpireSubject          string `json:"expireSubject"`
	ExpireContent          string `json:"expireContent"`
	ExpireContentType      string `json:"expireContentType"`
	OpenedSubject          string `json:"openedSubject"`
	OpenedContent          string `json:"openedContent"`
	OpenedContentType      string `json:"openedContentType"`
}

type updateMailConfigRequest = mailConfig

type updateMailContentTypeRequest struct {
	Template    string `json:"template" binding:"required"`
	ContentType string `json:"contentType" binding:"required"`
	Content     string `json:"content" binding:"required"`
}

type testMailRequest struct {
	Recipient string `json:"recipient" binding:"required,email"`
}

type mailMessage struct {
	To          string
	Subject     string
	Content     string
	ContentType string
}

type mailLogItem struct {
	ID         int64  `json:"id"`
	EventType  string `json:"eventType"`
	TargetType string `json:"targetType"`
	TargetID   *int64 `json:"targetId"`
	LicenseID  *int64 `json:"licenseId"`
	Recipient  string `json:"recipient"`
	Subject    string `json:"subject"`
	Content    string `json:"content"`
	Status     string `json:"status"`
	Error      string `json:"error"`
	RemindDays *int   `json:"remindDays"`
	CreatedAt  string `json:"createdAt"`
	SentAt     string `json:"sentAt"`
}

type licenseMailContext struct {
	LicenseID  int64
	OwnerType  string
	OwnerID    int64
	Recipient  string
	OwnerName  string
	AppName    string
	LicenseNo  string
	LicenseKey string
	OpenedAt   time.Time
	ExpiresAt  sql.NullTime
}

func ensureMailStorage(db *sql.DB) error {
	mailStorageMu.Lock()
	defer mailStorageMu.Unlock()
	if mailStorageReady {
		return nil
	}

	if err := ensureSystemConfigStorage(db); err != nil {
		return err
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS mail_send_logs (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			event_type VARCHAR(50) NOT NULL,
			target_type VARCHAR(30) DEFAULT '',
			target_id BIGINT UNSIGNED DEFAULT NULL,
			license_id BIGINT UNSIGNED DEFAULT NULL,
			recipient VARCHAR(255) DEFAULT '',
			subject VARCHAR(255) DEFAULT '',
			content LONGTEXT,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			error TEXT,
			remind_days INT DEFAULT NULL,
			event_key VARCHAR(120) DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			sent_at DATETIME DEFAULT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uk_event_key (event_key),
			KEY idx_event_status (event_type, status, created_at),
			KEY idx_license_event (license_id, event_type)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`); err != nil {
		return err
	}
	mailStorageReady = true
	return nil
}

func defaultMailConfig() mailConfig {
	return mailConfig{
		Provider:               "qq",
		SMTPHost:               "smtp.qq.com",
		SMTPPort:               465,
		SMTPSecure:             "ssl",
		SMTPFromName:           defaultSiteName,
		EnabledPurchaseSuccess: true,
		EnabledExpireReminder:  true,
		EnabledLicenseOpened:   true,
		ExpireRemindDays:       "7,3,1",
		PurchaseSubject:        defaultPurchaseSubject,
		PurchaseContent:        defaultPurchaseContent,
		PurchaseContentType:    "text",
		ExpireSubject:          defaultExpireSubject,
		ExpireContent:          defaultExpireContent,
		ExpireContentType:      "text",
		OpenedSubject:          defaultOpenedSubject,
		OpenedContent:          defaultOpenedContent,
		OpenedContentType:      "text",
	}
}

func loadMailConfig(db *sql.DB, includePassword bool) (mailConfig, error) {
	cfg := defaultMailConfig()
	rows, err := db.Query("SELECT `key`, value FROM system_configs WHERE `group` = ?", mailGroup)
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
		case "provider":
			cfg.Provider = value
		case "smtp_host":
			cfg.SMTPHost = value
		case "smtp_port":
			if port, err := strconv.Atoi(value); err == nil {
				cfg.SMTPPort = port
			}
		case "smtp_secure":
			cfg.SMTPSecure = value
		case "smtp_username":
			cfg.SMTPUsername = value
		case "smtp_password":
			cfg.PasswordSet = strings.TrimSpace(value) != ""
			if includePassword {
				cfg.SMTPPassword = value
			}
		case "smtp_from_email":
			cfg.SMTPFromEmail = value
		case "smtp_from_name":
			cfg.SMTPFromName = value
		case "enabled_purchase_success":
			cfg.EnabledPurchaseSuccess = value == "true" || value == "1"
		case "enabled_expire_reminder":
			cfg.EnabledExpireReminder = value == "true" || value == "1"
		case "enabled_license_opened":
			cfg.EnabledLicenseOpened = value == "true" || value == "1"
		case "expire_remind_days":
			cfg.ExpireRemindDays = value
		case "purchase_subject":
			cfg.PurchaseSubject = value
		case "purchase_content":
			cfg.PurchaseContent = value
		case "purchase_content_type":
			cfg.PurchaseContentType = value
		case "expire_subject":
			cfg.ExpireSubject = value
		case "expire_content":
			cfg.ExpireContent = value
		case "expire_content_type":
			cfg.ExpireContentType = value
		case "opened_subject":
			cfg.OpenedSubject = value
		case "opened_content":
			cfg.OpenedContent = value
		case "opened_content_type":
			cfg.OpenedContentType = value
		}
	}
	return normalizeMailConfig(cfg), rows.Err()
}

func normalizeMailConfig(cfg mailConfig) mailConfig {
	cfg.Provider = strings.TrimSpace(cfg.Provider)
	if cfg.Provider == "" {
		cfg.Provider = "qq"
	}
	cfg.SMTPHost = strings.TrimSpace(cfg.SMTPHost)
	cfg.SMTPSecure = strings.ToLower(strings.TrimSpace(cfg.SMTPSecure))
	if cfg.SMTPSecure == "" {
		cfg.SMTPSecure = "ssl"
	}
	if cfg.SMTPPort == 0 {
		if cfg.SMTPSecure == "ssl" {
			cfg.SMTPPort = 465
		} else {
			cfg.SMTPPort = 587
		}
	}
	cfg.SMTPUsername = strings.TrimSpace(cfg.SMTPUsername)
	cfg.SMTPFromEmail = strings.TrimSpace(cfg.SMTPFromEmail)
	cfg.SMTPFromName = strings.TrimSpace(cfg.SMTPFromName)
	if cfg.SMTPFromName == "" {
		cfg.SMTPFromName = defaultSiteName
	}
	cfg.ExpireRemindDays = strings.TrimSpace(cfg.ExpireRemindDays)
	if cfg.ExpireRemindDays == "" {
		cfg.ExpireRemindDays = "7,3,1"
	}
	if strings.TrimSpace(cfg.PurchaseSubject) == "" {
		cfg.PurchaseSubject = defaultPurchaseSubject
	}
	if strings.TrimSpace(cfg.PurchaseContent) == "" {
		cfg.PurchaseContent = defaultPurchaseContent
	}
	cfg.PurchaseContentType = normalizeMailContentType(cfg.PurchaseContentType)
	if strings.TrimSpace(cfg.ExpireSubject) == "" {
		cfg.ExpireSubject = defaultExpireSubject
	}
	if strings.TrimSpace(cfg.ExpireContent) == "" {
		cfg.ExpireContent = defaultExpireContent
	}
	cfg.ExpireContentType = normalizeMailContentType(cfg.ExpireContentType)
	if strings.TrimSpace(cfg.OpenedSubject) == "" {
		cfg.OpenedSubject = defaultOpenedSubject
	}
	if strings.TrimSpace(cfg.OpenedContent) == "" {
		cfg.OpenedContent = defaultOpenedContent
	}
	cfg.OpenedContentType = normalizeMailContentType(cfg.OpenedContentType)
	return cfg
}

func normalizeMailContentType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "html" {
		return "html"
	}
	return "text"
}

func validateMailConfig(cfg mailConfig, requirePassword bool) error {
	if cfg.SMTPHost == "" {
		return errors.New("SMTP地址不能为空")
	}
	if cfg.SMTPPort < 1 || cfg.SMTPPort > 65535 {
		return errors.New("SMTP端口不正确")
	}
	if cfg.SMTPSecure != "ssl" && cfg.SMTPSecure != "starttls" && cfg.SMTPSecure != "none" {
		return errors.New("加密方式不正确")
	}
	if cfg.SMTPUsername == "" {
		return errors.New("发信账号不能为空")
	}
	if requirePassword && strings.TrimSpace(cfg.SMTPPassword) == "" {
		return errors.New("发信密码或授权码不能为空")
	}
	if _, err := mail.ParseAddress(cfg.SMTPFromEmail); err != nil {
		return errors.New("发件人邮箱不正确")
	}
	if _, err := parseRemindDays(cfg.ExpireRemindDays); err != nil {
		return err
	}
	return nil
}

func saveMailConfig(db *sql.DB, cfg mailConfig, keepPassword bool) error {
	cfg = normalizeMailConfig(cfg)
	items := []struct {
		key         string
		value       string
		description string
	}{
		{"provider", cfg.Provider, "邮箱类型"},
		{"smtp_host", cfg.SMTPHost, "SMTP地址"},
		{"smtp_port", strconv.Itoa(cfg.SMTPPort), "SMTP端口"},
		{"smtp_secure", cfg.SMTPSecure, "SMTP加密方式"},
		{"smtp_username", cfg.SMTPUsername, "SMTP账号"},
		{"smtp_from_email", cfg.SMTPFromEmail, "发件人邮箱"},
		{"smtp_from_name", cfg.SMTPFromName, "发件人名称"},
		{"enabled_purchase_success", strconv.FormatBool(cfg.EnabledPurchaseSuccess), "购买成功邮件开关"},
		{"enabled_expire_reminder", strconv.FormatBool(cfg.EnabledExpireReminder), "到期提醒邮件开关"},
		{"enabled_license_opened", strconv.FormatBool(cfg.EnabledLicenseOpened), "后台开通邮件开关"},
		{"expire_remind_days", cfg.ExpireRemindDays, "到期提醒天数"},
		{"purchase_subject", cfg.PurchaseSubject, "购买成功邮件标题"},
		{"purchase_content", cfg.PurchaseContent, "购买成功邮件内容"},
		{"purchase_content_type", cfg.PurchaseContentType, "购买成功邮件内容类型"},
		{"expire_subject", cfg.ExpireSubject, "到期提醒邮件标题"},
		{"expire_content", cfg.ExpireContent, "到期提醒邮件内容"},
		{"expire_content_type", cfg.ExpireContentType, "到期提醒邮件内容类型"},
		{"opened_subject", cfg.OpenedSubject, "后台开通邮件标题"},
		{"opened_content", cfg.OpenedContent, "后台开通邮件内容"},
		{"opened_content_type", cfg.OpenedContentType, "后台开通邮件内容类型"},
	}
	if !keepPassword {
		items = append(items, struct {
			key         string
			value       string
			description string
		}{"smtp_password", cfg.SMTPPassword, "SMTP密码或授权码"})
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range items {
		if _, err := tx.Exec(`
			INSERT INTO system_configs (`+"`group`"+`, `+"`key`"+`, value, description)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE value = VALUES(value), description = VALUES(description)
		`, mailGroup, item.key, item.value, item.description); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func saveMailContentType(db *sql.DB, template, contentType, content string) error {
	fields, ok := map[string]struct {
		contentKey         string
		contentTypeKey     string
		contentDescription string
		typeDescription    string
	}{
		"purchase": {"purchase_content", "purchase_content_type", "购买成功邮件内容", "购买成功邮件内容类型"},
		"expire":   {"expire_content", "expire_content_type", "到期提醒邮件内容", "到期提醒邮件内容类型"},
		"opened":   {"opened_content", "opened_content_type", "后台开通邮件内容", "后台开通邮件内容类型"},
	}[template]
	if !ok {
		return errInvalidMailTemplate
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	items := []struct {
		key         string
		value       string
		description string
	}{
		{fields.contentKey, content, fields.contentDescription},
		{fields.contentTypeKey, contentType, fields.typeDescription},
	}
	for _, item := range items {
		if _, err := tx.Exec(`
			INSERT INTO system_configs (`+"`group`"+`, `+"`key`"+`, value, description)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE value = VALUES(value), description = VALUES(description)
		`, mailGroup, item.key, item.value, item.description); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func AdminMailConfig(c *gin.Context) {
	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureMailStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化邮件配置失败"})
		return
	}
	cfg, err := loadMailConfig(db, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取邮件配置失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": cfg})
}

func AdminMailConfigUpdate(c *gin.Context) {
	var req updateMailConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureMailStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化邮件配置失败"})
		return
	}
	oldCfg, _ := loadMailConfig(db, true)
	req = normalizeMailConfig(req)
	keepPassword := strings.TrimSpace(req.SMTPPassword) == ""
	if keepPassword {
		req.SMTPPassword = oldCfg.SMTPPassword
	}
	if err := validateMailConfig(req, !keepPassword || !oldCfg.PasswordSet); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if err := saveMailConfig(db, req, keepPassword); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存邮件配置失败"})
		return
	}
	result := req
	result.SMTPPassword = ""
	result.PasswordSet = strings.TrimSpace(req.SMTPPassword) != ""
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "邮件配置保存成功", "data": result})
}

func AdminMailContentTypeUpdate(c *gin.Context) {
	var req updateMailContentTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	contentType := strings.ToLower(strings.TrimSpace(req.ContentType))
	if contentType != "text" && contentType != "html" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "邮件内容类型不正确"})
		return
	}

	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureMailStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化邮件配置失败"})
		return
	}
	if err := saveMailContentType(db, req.Template, contentType, req.Content); err != nil {
		if errors.Is(err, errInvalidMailTemplate) {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "自动保存邮件内容失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "邮件内容类型已自动保存"})
}

func AdminMailConfigTest(c *gin.Context) {
	var req testMailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请输入正确的测试邮箱"})
		return
	}
	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureMailStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化邮件配置失败"})
		return
	}
	cfg, err := loadMailConfig(db, true)
	if err != nil || validateMailConfig(cfg, true) != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请先保存完整的邮件配置"})
		return
	}
	siteName := loadSiteNameForMail(db)
	subject := "邮件发送测试 - " + siteName
	content := "这是一封测试邮件。\n\n如果您收到此邮件，说明当前发信配置可以正常使用。"
	logID, _ := createMailLog(db, "test", "admin", 0, 0, req.Recipient, subject, content, 0, nil)
	if err := sendSMTPMail(cfg, mailMessage{To: req.Recipient, Subject: subject, Content: content}); err != nil {
		markMailLogFailed(db, logID, err.Error())
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "测试邮件发送失败: " + err.Error()})
		return
	}
	markMailLogSent(db, logID)
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "测试邮件已发送"})
}

func QueuePurchaseSuccessMail(ownerType string, ownerID, licenseID int64) {
	go func() {
		db, err := openSystemConfigDB()
		if err != nil {
			return
		}
		defer db.Close()
		if err := ensureMailStorage(db); err != nil {
			return
		}
		cfg, err := loadMailConfig(db, true)
		if err != nil || !cfg.EnabledPurchaseSuccess {
			return
		}
		ctx, err := loadLicenseMailContext(db, licenseID)
		if err != nil {
			return
		}
		if ctx.OwnerType != ownerType || ctx.OwnerID != ownerID {
			ctx.OwnerType = ownerType
			ctx.OwnerID = ownerID
		}
		sendBusinessMail(db, cfg, "purchase_success", ctx, 0)
	}()
}

func QueueLicenseOpenedMail(licenseID int64) {
	go func() {
		db, err := openSystemConfigDB()
		if err != nil {
			return
		}
		defer db.Close()
		if err := ensureMailStorage(db); err != nil {
			return
		}
		cfg, err := loadMailConfig(db, true)
		if err != nil || !cfg.EnabledLicenseOpened {
			return
		}
		ctx, err := loadLicenseMailContext(db, licenseID)
		if err != nil {
			return
		}
		sendBusinessMail(db, cfg, "license_opened", ctx, 0)
	}()
}

func AdminMailLogList(c *gin.Context) {
	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureMailStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化邮件日志失败"})
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
	offset := (page - 1) * pageSize
	where := "WHERE 1=1"
	args := []interface{}{}
	if eventType := strings.TrimSpace(c.Query("eventType")); eventType != "" {
		where += " AND event_type = ?"
		args = append(args, eventType)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		where += " AND (recipient LIKE ? OR subject LIKE ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}

	var total int
	_ = db.QueryRow("SELECT COUNT(*) FROM mail_send_logs "+where, args...).Scan(&total)
	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, offset)
	rows, err := db.Query(`
		SELECT id, event_type, target_type, target_id, license_id, recipient, subject, content, status, error,
		       remind_days, created_at, sent_at
		FROM mail_send_logs `+where+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询邮件日志失败"})
		return
	}
	defer rows.Close()

	list := []mailLogItem{}
	for rows.Next() {
		var item mailLogItem
		var targetID, licenseID sql.NullInt64
		var remindDays sql.NullInt64
		var createdAt sql.NullTime
		var sentAt sql.NullTime
		var errText sql.NullString
		if err := rows.Scan(&item.ID, &item.EventType, &item.TargetType, &targetID, &licenseID, &item.Recipient, &item.Subject, &item.Content, &item.Status, &errText, &remindDays, &createdAt, &sentAt); err != nil {
			continue
		}
		if targetID.Valid {
			v := targetID.Int64
			item.TargetID = &v
		}
		if licenseID.Valid {
			v := licenseID.Int64
			item.LicenseID = &v
		}
		if remindDays.Valid {
			v := int(remindDays.Int64)
			item.RemindDays = &v
		}
		if errText.Valid {
			item.Error = errText.String
		}
		if createdAt.Valid {
			item.CreatedAt = createdAt.Time.Format("2006-01-02 15:04:05")
		}
		if sentAt.Valid {
			item.SentAt = sentAt.Time.Format("2006-01-02 15:04:05")
		}
		list = append(list, item)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{"list": list, "total": total, "page": page, "pageSize": pageSize}})
}

func AdminMailLogDetail(c *gin.Context) {
	id := c.Param("id")
	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureMailStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化邮件日志失败"})
		return
	}
	var item mailLogItem
	var targetID, licenseID sql.NullInt64
	var remindDays sql.NullInt64
	var createdAt sql.NullTime
	var sentAt sql.NullTime
	var errText sql.NullString
	err = db.QueryRow(`
		SELECT id, event_type, target_type, target_id, license_id, recipient, subject, content, status, error,
		       remind_days, created_at, sent_at
		FROM mail_send_logs WHERE id = ?
	`, id).Scan(&item.ID, &item.EventType, &item.TargetType, &targetID, &licenseID, &item.Recipient, &item.Subject, &item.Content, &item.Status, &errText, &remindDays, &createdAt, &sentAt)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "邮件日志不存在"})
		return
	}
	if targetID.Valid {
		v := targetID.Int64
		item.TargetID = &v
	}
	if licenseID.Valid {
		v := licenseID.Int64
		item.LicenseID = &v
	}
	if remindDays.Valid {
		v := int(remindDays.Int64)
		item.RemindDays = &v
	}
	if errText.Valid {
		item.Error = errText.String
	}
	if createdAt.Valid {
		item.CreatedAt = createdAt.Time.Format("2006-01-02 15:04:05")
	}
	if sentAt.Valid {
		item.SentAt = sentAt.Time.Format("2006-01-02 15:04:05")
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": item})
}

func StartMailReminderWorker() {
	mailWorkerOnce.Do(func() {
		go func() {
			time.Sleep(30 * time.Second)
			runExpireReminderScan()
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				runExpireReminderScan()
			}
		}()
	})
}

func runExpireReminderScan() {
	db, err := openSystemConfigDB()
	if err != nil {
		return
	}
	defer db.Close()
	if err := ensureMailStorage(db); err != nil {
		return
	}
	cfg, err := loadMailConfig(db, true)
	if err != nil || !cfg.EnabledExpireReminder {
		return
	}
	days, err := parseRemindDays(cfg.ExpireRemindDays)
	if err != nil || len(days) == 0 {
		return
	}
	for _, day := range days {
		scanExpireReminderByDay(db, cfg, day)
	}
}

func scanExpireReminderByDay(db *sql.DB, cfg mailConfig, day int) {
	rows, err := db.Query(`
		SELECT id
		FROM licenses
		WHERE status = 'active'
		  AND expired_at IS NOT NULL
		  AND expired_at > NOW()
		  AND DATE(expired_at) = DATE(DATE_ADD(NOW(), INTERVAL ? DAY))
		LIMIT 200
	`, day)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var licenseID int64
		if rows.Scan(&licenseID) != nil {
			continue
		}
		ctx, err := loadLicenseMailContext(db, licenseID)
		if err == nil {
			sendBusinessMail(db, cfg, "expire_reminder", ctx, day)
		}
	}
}

func sendBusinessMail(db *sql.DB, cfg mailConfig, eventType string, ctx licenseMailContext, remindDays int) {
	if strings.TrimSpace(ctx.Recipient) == "" {
		logID, _ := createMailLog(db, eventType, ctx.OwnerType, ctx.OwnerID, ctx.LicenseID, "", "", "", remindDays, eventKey(eventType, ctx.LicenseID, remindDays))
		markMailLogSkipped(db, logID, "收件邮箱为空")
		return
	}
	vars := buildMailVars(db, ctx, remindDays)
	subjectTemplate := cfg.PurchaseSubject
	contentTemplate := cfg.PurchaseContent
	contentType := cfg.PurchaseContentType
	if eventType == "expire_reminder" {
		subjectTemplate = cfg.ExpireSubject
		contentTemplate = cfg.ExpireContent
		contentType = cfg.ExpireContentType
	} else if eventType == "license_opened" {
		subjectTemplate = cfg.OpenedSubject
		contentTemplate = cfg.OpenedContent
		contentType = cfg.OpenedContentType
	}
	subject := renderMailTemplate(subjectTemplate, vars)
	content := renderMailTemplate(contentTemplate, vars)
	logID, inserted := createMailLog(db, eventType, ctx.OwnerType, ctx.OwnerID, ctx.LicenseID, ctx.Recipient, subject, content, remindDays, eventKey(eventType, ctx.LicenseID, remindDays))
	if !inserted {
		return
	}
	if err := sendSMTPMail(cfg, mailMessage{To: ctx.Recipient, Subject: subject, Content: content, ContentType: contentType}); err != nil {
		markMailLogFailed(db, logID, err.Error())
		return
	}
	markMailLogSent(db, logID)
}

func loadLicenseMailContext(db *sql.DB, licenseID int64) (licenseMailContext, error) {
	var ctx licenseMailContext
	var userEmail, userName, agentEmail, agentName sql.NullString
	err := db.QueryRow(`
		SELECT l.id, l.owner_type, l.owner_id, a.app_name, l.license_no, l.license_key, l.started_at, l.expired_at,
		       u.email, u.nickname, ag.email, ag.name
		FROM licenses l
		JOIN apps a ON a.id = l.app_id
		LEFT JOIN users u ON l.owner_type = 'user' AND u.id = l.owner_id
		LEFT JOIN agents ag ON l.owner_type = 'agent' AND ag.id = l.owner_id
		WHERE l.id = ?
	`, licenseID).Scan(&ctx.LicenseID, &ctx.OwnerType, &ctx.OwnerID, &ctx.AppName, &ctx.LicenseNo, &ctx.LicenseKey, &ctx.OpenedAt, &ctx.ExpiresAt, &userEmail, &userName, &agentEmail, &agentName)
	if err != nil {
		return ctx, err
	}
	if ctx.OwnerType == "agent" {
		ctx.Recipient = agentEmail.String
		ctx.OwnerName = agentName.String
	} else {
		ctx.Recipient = userEmail.String
		ctx.OwnerName = userName.String
	}
	if strings.TrimSpace(ctx.OwnerName) == "" {
		ctx.OwnerName = ctx.Recipient
	}
	return ctx, nil
}

func buildMailVars(db *sql.DB, ctx licenseMailContext, daysLeft int) map[string]string {
	expiresAt := "永久"
	if ctx.ExpiresAt.Valid {
		expiresAt = ctx.ExpiresAt.Time.Format("2006-01-02 15:04:05")
		if daysLeft == 0 {
			daysLeft = int(time.Until(ctx.ExpiresAt.Time).Hours() / 24)
			if daysLeft < 0 {
				daysLeft = 0
			}
		}
	}
	return map[string]string{
		"{{appName}}":    ctx.AppName,
		"{{licenseNo}}":  ctx.LicenseNo,
		"{{licenseKey}}": firstNonEmpty(ctx.LicenseKey, ctx.LicenseNo),
		"{{openedAt}}":   ctx.OpenedAt.Format("2006-01-02 15:04:05"),
		"{{expiresAt}}":  expiresAt,
		"{{daysLeft}}":   strconv.Itoa(daysLeft),
		"{{ownerName}}":  firstNonEmpty(ctx.OwnerName, ctx.Recipient),
		"{{siteName}}":   loadSiteNameForMail(db),
	}
}

func renderMailTemplate(template string, vars map[string]string) string {
	result := template
	for key, value := range vars {
		result = strings.ReplaceAll(result, key, value)
	}
	return result
}

func sendSMTPMail(cfg mailConfig, msg mailMessage) error {
	if err := validateMailConfig(cfg, true); err != nil {
		return err
	}
	if _, err := mail.ParseAddress(msg.To); err != nil {
		return errors.New("收件邮箱不正确")
	}
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	from := (&mail.Address{Name: cfg.SMTPFromName, Address: cfg.SMTPFromEmail}).String()
	contentType := "text/plain"
	if normalizeMailContentType(msg.ContentType) == "html" {
		contentType = "text/html"
	}
	headers := []string{
		"From: " + from,
		"To: " + msg.To,
		"Subject: " + mime.QEncoding.Encode("UTF-8", msg.Subject),
		"MIME-Version: 1.0",
		"Content-Type: " + contentType + "; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	}
	body := strings.Join(headers, "\r\n") + "\r\n\r\n" + msg.Content

	var client *smtp.Client
	var err error
	if cfg.SMTPSecure == "ssl" {
		tlsConfig := &tls.Config{ServerName: cfg.SMTPHost, MinVersion: tls.VersionTLS12}
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", addr, tlsConfig)
		if err != nil {
			return err
		}
		client, err = smtp.NewClient(conn, cfg.SMTPHost)
		if err != nil {
			conn.Close()
			return err
		}
	} else {
		client, err = smtp.Dial(addr)
		if err != nil {
			return err
		}
	}
	defer client.Close()

	if cfg.SMTPSecure == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.SMTPHost, MinVersion: tls.VersionTLS12}); err != nil {
				return err
			}
		}
	}
	if cfg.SMTPUsername != "" {
		if err := client.Auth(smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPHost)); err != nil {
			return err
		}
	}
	if err := client.Mail(cfg.SMTPFromEmail); err != nil {
		return err
	}
	if err := client.Rcpt(msg.To); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write([]byte(body)); err != nil {
		writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func createMailLog(db *sql.DB, eventType, targetType string, targetID, licenseID int64, recipient, subject, content string, remindDays int, key *string) (int64, bool) {
	var remind interface{}
	if remindDays > 0 {
		remind = remindDays
	}
	result, err := db.Exec(`
		INSERT INTO mail_send_logs (event_type, target_type, target_id, license_id, recipient, subject, content, status, remind_days, event_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?)
	`, eventType, targetType, nullableID(targetID), nullableID(licenseID), recipient, subject, content, remind, key)
	if err != nil {
		return 0, false
	}
	id, _ := result.LastInsertId()
	return id, true
}

func markMailLogSent(db *sql.DB, id int64) {
	if id > 0 {
		_, _ = db.Exec("UPDATE mail_send_logs SET status = 'sent', sent_at = NOW(), error = NULL WHERE id = ?", id)
	}
}

func markMailLogFailed(db *sql.DB, id int64, msg string) {
	if id > 0 {
		_, _ = db.Exec("UPDATE mail_send_logs SET status = 'failed', error = ? WHERE id = ?", msg, id)
	}
}

func markMailLogSkipped(db *sql.DB, id int64, msg string) {
	if id > 0 {
		_, _ = db.Exec("UPDATE mail_send_logs SET status = 'skipped', error = ? WHERE id = ?", msg, id)
	}
}

func parseRemindDays(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	seen := map[int]bool{}
	var days []int
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		day, err := strconv.Atoi(part)
		if err != nil || day < 1 || day > 365 {
			return nil, errors.New("到期提醒天数应为 1-365 之间的数字，多个数字用英文逗号分隔")
		}
		if !seen[day] {
			seen[day] = true
			days = append(days, day)
		}
	}
	if len(days) == 0 {
		return nil, errors.New("请至少配置一个到期提醒天数")
	}
	return days, nil
}

func eventKey(eventType string, licenseID int64, remindDays int) *string {
	if eventType != "expire_reminder" || licenseID == 0 || remindDays == 0 {
		return nil
	}
	key := fmt.Sprintf("%s:%d:%d", eventType, licenseID, remindDays)
	return &key
}

func nullableID(id int64) interface{} {
	if id == 0 {
		return nil
	}
	return id
}

func loadSiteNameForMail(db *sql.DB) string {
	var siteName string
	if err := db.QueryRow("SELECT value FROM system_configs WHERE `group` = 'site' AND `key` = 'site_name'").Scan(&siteName); err != nil || strings.TrimSpace(siteName) == "" {
		return defaultSiteName
	}
	return siteName
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
