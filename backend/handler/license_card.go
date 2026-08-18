package handler

import (
	"crypto/rand"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
)

const (
	licenseCardStatusUnused   = "unused"
	licenseCardStatusRedeemed = "redeemed"
	licenseCardStatusDisabled = "disabled"
	licenseCardBatchActive    = "active"
	licenseCardBatchDisabled  = "disabled"
	licenseCardMaxBatchSize   = 5000
	licenseCardRedeemAttempts = 10
	licenseCardRedeemWindow   = time.Minute
)

type licenseCardRateWindow struct {
	startedAt time.Time
	attempts  int
}

type licenseCardRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]licenseCardRateWindow
}

func newLicenseCardRateLimiter(limit int, window time.Duration) *licenseCardRateLimiter {
	return &licenseCardRateLimiter{limit: limit, window: window, entries: make(map[string]licenseCardRateWindow)}
}

func (limiter *licenseCardRateLimiter) allow(key string, now time.Time) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	entry, exists := limiter.entries[key]
	if !exists || now.Sub(entry.startedAt) >= limiter.window {
		limiter.entries[key] = licenseCardRateWindow{startedAt: now, attempts: 1}
		return true
	}
	if entry.attempts >= limiter.limit {
		return false
	}
	entry.attempts++
	limiter.entries[key] = entry
	return true
}

var (
	licenseCardSchemaMu      sync.Mutex
	licenseCardSchemaOK      bool
	licenseCardRedeemLimiter = newLicenseCardRateLimiter(licenseCardRedeemAttempts, licenseCardRedeemWindow)
)

func ensureLicenseCardSchema(db *sql.DB) error {
	licenseCardSchemaMu.Lock()
	defer licenseCardSchemaMu.Unlock()
	if licenseCardSchemaOK {
		return nil
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS license_card_batches (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			batch_no VARCHAR(64) NOT NULL,
			app_id BIGINT UNSIGNED NOT NULL,
			plan_id BIGINT UNSIGNED NOT NULL,
			app_name_snapshot VARCHAR(100) NOT NULL,
			plan_name_snapshot VARCHAR(100) NOT NULL,
			duration_days INT UNSIGNED NOT NULL DEFAULT 0,
			price_snapshot DECIMAL(12,2) NOT NULL DEFAULT 0.00,
			license_type ENUM('domain','wildcard','ip','key') NOT NULL,
			total_count INT UNSIGNED NOT NULL,
			status ENUM('active','disabled') NOT NULL DEFAULT 'active',
			remark VARCHAR(255) DEFAULT '',
			created_by BIGINT UNSIGNED DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uk_card_batch_no (batch_no),
			KEY idx_card_batch_app (app_id, plan_id),
			KEY idx_card_batch_status (status, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='授权卡密批次表'`,
		`CREATE TABLE IF NOT EXISTS license_cards (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			batch_id BIGINT UNSIGNED NOT NULL,
			card_code VARCHAR(64) NOT NULL,
			card_suffix VARCHAR(8) NOT NULL,
			status ENUM('unused','redeemed','disabled') NOT NULL DEFAULT 'unused',
			redeemed_by_type ENUM('user','agent') DEFAULT NULL,
			redeemed_by_id BIGINT UNSIGNED DEFAULT NULL,
			license_id BIGINT UNSIGNED DEFAULT NULL,
			redeemed_at DATETIME DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uk_license_card_code (card_code),
			UNIQUE KEY uk_license_card_license (license_id),
			KEY idx_license_card_batch (batch_id, status),
			KEY idx_license_card_redeemer (redeemed_by_type, redeemed_by_id, redeemed_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='授权卡密库存表'`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}

	var sourceType string
	if err := db.QueryRow(`
		SELECT COLUMN_TYPE FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'licenses' AND COLUMN_NAME = 'source'
	`).Scan(&sourceType); err != nil {
		return err
	}
	if !strings.Contains(sourceType, "'card'") {
		if _, err := db.Exec(`
			ALTER TABLE licenses
			MODIFY COLUMN source ENUM('admin','agent','user_purchase','card') NOT NULL COMMENT '来源'
		`); err != nil {
			return err
		}
	}

	licenseCardSchemaOK = true
	return nil
}

func openLicenseCardDB() (*sql.DB, error) {
	cfg, err := config.LoadDBConfig()
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		return nil, err
	}
	if err := ensureLicenseCardSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func generateLicenseCardCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	segments := make([]string, 4)
	limit := big.NewInt(int64(len(alphabet)))
	for segment := range segments {
		part := make([]byte, 4)
		for index := range part {
			value, err := rand.Int(rand.Reader, limit)
			if err != nil {
				return "", err
			}
			part[index] = alphabet[value.Int64()]
		}
		segments[segment] = string(part)
	}
	return "AUTH-" + strings.Join(segments, "-"), nil
}

func generateLicenseCardBatchNo() (string, error) {
	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return fmt.Sprintf("CB%s-%X", time.Now().Format("20060102150405"), raw), nil
}

func normalizeLicenseCardCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func licenseCardTypeLabel(value string) string {
	return map[string]string{
		"domain":   "单域名",
		"wildcard": "泛域名",
		"ip":       "IP",
		"key":      "密钥",
	}[value]
}

type licenseCardBatchSnapshot struct {
	AppName      string
	PlanName     string
	DurationDays int
	Price        float64
	TypeMask     uint8
}

func loadLicenseCardBatchSnapshot(tx *sql.Tx, appID, planID int64) (licenseCardBatchSnapshot, error) {
	var snapshot licenseCardBatchSnapshot
	var appEnabled, planEnabled bool
	err := tx.QueryRow(`
		SELECT a.app_name, p.name, p.duration_days, p.price,
		       a.purchase_license_type_mask, a.enabled, p.enabled
		FROM apps a
		JOIN license_plans p ON p.app_id = a.id
		WHERE a.id = ? AND p.id = ?
		FOR UPDATE
	`, appID, planID).Scan(
		&snapshot.AppName, &snapshot.PlanName, &snapshot.DurationDays, &snapshot.Price,
		&snapshot.TypeMask, &appEnabled, &planEnabled,
	)
	if err != nil {
		return licenseCardBatchSnapshot{}, err
	}
	if !appEnabled || !planEnabled {
		return licenseCardBatchSnapshot{}, errors.New("应用或套餐已禁用")
	}
	return snapshot, nil
}

func AdminLicenseCardBatchList(c *gin.Context) {
	db, err := openLicenseCardDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化卡密模块失败: " + err.Error()})
		return
	}
	defer db.Close()

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	where := []string{"1=1"}
	args := make([]any, 0, 5)
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		where = append(where, "(b.batch_no LIKE ? OR b.app_name_snapshot LIKE ? OR b.plan_name_snapshot LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like, like)
	}
	if status := strings.TrimSpace(c.Query("status")); status == licenseCardBatchActive || status == licenseCardBatchDisabled {
		where = append(where, "b.status = ?")
		args = append(args, status)
	}
	if appID := strings.TrimSpace(c.Query("appId")); appID != "" {
		where = append(where, "b.app_id = ?")
		args = append(args, appID)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := db.QueryRow("SELECT COUNT(*) FROM license_card_batches b WHERE "+whereSQL, args...).Scan(&total); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询卡密批次失败"})
		return
	}

	query := fmt.Sprintf(`
		SELECT b.id, b.batch_no, b.app_id, b.plan_id, b.app_name_snapshot, b.plan_name_snapshot,
		       b.duration_days, b.price_snapshot, b.license_type, b.total_count, b.status,
		       COALESCE(b.remark, ''), b.created_at,
		       COALESCE(SUM(c.status = 'unused'), 0),
		       COALESCE(SUM(c.status = 'redeemed'), 0),
		       COALESCE(SUM(c.status = 'disabled'), 0)
		FROM license_card_batches b
		LEFT JOIN license_cards c ON c.batch_id = b.id
		WHERE %s
		GROUP BY b.id
		ORDER BY b.id DESC LIMIT ? OFFSET ?
	`, whereSQL)
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询卡密批次失败: " + err.Error()})
		return
	}
	defer rows.Close()

	list := make([]gin.H, 0)
	for rows.Next() {
		var id, appID, planID, totalCount, unusedCount, redeemedCount, disabledCount int64
		var batchNo, appName, planName, licenseType, status, remark string
		var durationDays int
		var price float64
		var createdAt time.Time
		if err := rows.Scan(&id, &batchNo, &appID, &planID, &appName, &planName,
			&durationDays, &price, &licenseType, &totalCount, &status, &remark, &createdAt,
			&unusedCount, &redeemedCount, &disabledCount); err != nil {
			continue
		}
		list = append(list, gin.H{
			"id": id, "batchNo": batchNo, "appId": appID, "planId": planID,
			"appName": appName, "planName": planName, "durationDays": durationDays,
			"price": price, "type": licenseType, "typeLabel": licenseCardTypeLabel(licenseType),
			"totalCount": totalCount, "unusedCount": unusedCount, "redeemedCount": redeemedCount,
			"disabledCount": disabledCount, "status": status, "remark": remark,
			"createdAt": createdAt.Format("2006-01-02 15:04:05"),
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{"list": list, "total": total}})
}

func AdminLicenseCardBatchCreate(c *gin.Context) {
	var req struct {
		AppID    int64  `json:"appId" binding:"required,gt=0"`
		PlanID   int64  `json:"planId" binding:"required,gt=0"`
		Type     string `json:"type" binding:"required,oneof=domain wildcard ip key"`
		Quantity int    `json:"quantity" binding:"required,gt=0"`
		Remark   string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Quantity > licenseCardMaxBatchSize {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": fmt.Sprintf("参数错误，单批最多生成%d张", licenseCardMaxBatchSize)})
		return
	}
	if len([]rune(req.Remark)) > 255 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "备注不能超过255个字符"})
		return
	}

	db, err := openLicenseCardDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化卡密模块失败: " + err.Error()})
		return
	}
	defer db.Close()
	if err := EnsureAppPurchaseLicenseTypesColumn(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化应用授权类型失败"})
		return
	}

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建卡密批次失败"})
		return
	}
	defer tx.Rollback()

	snapshot, err := loadLicenseCardBatchSnapshot(tx, req.AppID, req.PlanID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "应用或套餐不存在，或套餐不属于所选应用"})
		} else {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		}
		return
	}
	if !purchaseLicenseTypeAllowed(snapshot.TypeMask, req.Type) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该应用不允许生成" + licenseCardTypeLabel(req.Type) + "授权卡"})
		return
	}

	batchNo, err := generateLicenseCardBatchNo()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "生成批次号失败"})
		return
	}
	createdBy, _ := c.Get("user_id")
	result, err := tx.ExecContext(c.Request.Context(), `
		INSERT INTO license_card_batches
		(batch_no, app_id, plan_id, app_name_snapshot, plan_name_snapshot, duration_days,
		 price_snapshot, license_type, total_count, status, remark, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)
	`, batchNo, req.AppID, req.PlanID, snapshot.AppName, snapshot.PlanName, snapshot.DurationDays,
		snapshot.Price, req.Type, req.Quantity, strings.TrimSpace(req.Remark), createdBy)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建卡密批次失败: " + err.Error()})
		return
	}
	batchID, _ := result.LastInsertId()

	cards := make([]string, 0, req.Quantity)
	for len(cards) < req.Quantity {
		code, err := generateLicenseCardCode()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "生成卡密失败"})
			return
		}
		_, err = tx.ExecContext(c.Request.Context(), `
			INSERT INTO license_cards (batch_id, card_code, card_suffix, status)
			VALUES (?, ?, ?, 'unused')
		`, batchID, code, code[len(code)-4:])
		if err != nil {
			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
				continue
			}
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存卡密失败: " + err.Error()})
			return
		}
		cards = append(cards, code)
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "提交卡密批次失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "卡密生成成功", "data": gin.H{
		"id": batchID, "batchNo": batchNo, "cards": cards,
	}})
}

func AdminLicenseCardBatchToggle(c *gin.Context) {
	batchID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || batchID <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "批次ID不正确"})
		return
	}
	var req struct {
		Status string `json:"status" binding:"required,oneof=active disabled"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "状态不正确"})
		return
	}
	db, err := openLicenseCardDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化卡密模块失败"})
		return
	}
	defer db.Close()
	result, err := db.Exec("UPDATE license_card_batches SET status = ? WHERE id = ?", req.Status, batchID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "更新批次失败"})
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "卡密批次不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "批次状态已更新"})
}

func AdminLicenseCardBatchDelete(c *gin.Context) {
	batchID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || batchID <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "批次ID不正确"})
		return
	}
	db, err := openLicenseCardDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化卡密模块失败"})
		return
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "开启事务失败"})
		return
	}
	defer tx.Rollback()

	// 先删除该批次下的所有卡密
	if _, err := tx.Exec("DELETE FROM license_cards WHERE batch_id = ?", batchID); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "删除卡密失败"})
		return
	}

	// 再删除批次本身
	result, err := tx.Exec("DELETE FROM license_card_batches WHERE id = ?", batchID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "删除批次失败"})
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "卡密批次不存在"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "提交事务失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "批次已删除"})
}

func AdminLicenseCardList(c *gin.Context) {
	batchID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || batchID <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "批次ID不正确"})
		return
	}
	db, err := openLicenseCardDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化卡密模块失败"})
		return
	}
	defer db.Close()
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	status := strings.TrimSpace(c.Query("status"))
	if status != licenseCardStatusUnused && status != licenseCardStatusRedeemed && status != licenseCardStatusDisabled {
		status = ""
	}
	list, total, err := loadAdminLicenseCards(db, batchID, status, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询卡密失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{"list": list, "total": total}})
}

func loadAdminLicenseCards(db *sql.DB, batchID int64, status string, page, pageSize int) ([]gin.H, int64, error) {
	where := "c.batch_id = ?"
	args := []any{batchID}
	if status != "" {
		where += " AND c.status = ?"
		args = append(args, status)
	}
	var total int64
	if err := db.QueryRow("SELECT COUNT(*) FROM license_cards c WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := db.Query(`
		SELECT c.id, c.card_code, c.status, c.redeemed_by_type,
		       CASE
		         WHEN c.redeemed_by_type = 'user' THEN COALESCE(u.email, '')
		         WHEN c.redeemed_by_type = 'agent' THEN COALESCE(a.email, '')
		         ELSE ''
		       END AS redeemed_by_account,
		       c.license_id, c.redeemed_at, c.created_at
		FROM license_cards c
		LEFT JOIN users u ON c.redeemed_by_type = 'user' AND u.id = c.redeemed_by_id
		LEFT JOIN agents a ON c.redeemed_by_type = 'agent' AND a.id = c.redeemed_by_id
		WHERE `+where+`
		ORDER BY c.id ASC LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var code, cardStatus, ownerAccount string
		var ownerType sql.NullString
		var licenseID sql.NullInt64
		var redeemedAt sql.NullTime
		var createdAt time.Time
		if err := rows.Scan(&id, &code, &cardStatus, &ownerType, &ownerAccount, &licenseID, &redeemedAt, &createdAt); err != nil {
			return nil, 0, err
		}
		item := gin.H{
			"id": id, "cardCode": code, "status": cardStatus,
			"createdAt": createdAt.Format("2006-01-02 15:04:05"),
		}
		if ownerType.Valid && ownerAccount != "" {
			item["redeemedByType"] = ownerType.String
			item["redeemedByAccount"] = ownerAccount
		}
		if licenseID.Valid {
			item["licenseId"] = licenseID.Int64
		}
		if redeemedAt.Valid {
			item["redeemedAt"] = redeemedAt.Time.Format("2006-01-02 15:04:05")
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func AdminLicenseCardToggle(c *gin.Context) {
	cardID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || cardID <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "卡密ID不正确"})
		return
	}
	var req struct {
		Status string `json:"status" binding:"required,oneof=unused disabled"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "状态不正确"})
		return
	}
	db, err := openLicenseCardDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化卡密模块失败"})
		return
	}
	defer db.Close()
	result, err := db.Exec("UPDATE license_cards SET status = ? WHERE id = ? AND status <> 'redeemed'", req.Status, cardID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "更新卡密失败"})
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "卡密不存在或已兑换"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "卡密状态已更新"})
}

func AdminLicenseCardExport(c *gin.Context) {
	batchID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || batchID <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "批次ID不正确"})
		return
	}
	db, err := openLicenseCardDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化卡密模块失败"})
		return
	}
	defer db.Close()
	status := strings.TrimSpace(c.DefaultQuery("status", "unused"))
	where := "c.batch_id = ?"
	args := []any{batchID}
	if status != "all" {
		where += " AND c.status = ?"
		args = append(args, status)
	}
	var batchNo string
	if err := db.QueryRow("SELECT batch_no FROM license_card_batches WHERE id = ?", batchID).Scan(&batchNo); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "批次不存在"})
		return
	}
	rows, err := db.Query(`SELECT c.card_code, c.status, COALESCE(c.redeemed_by_type, ''), COALESCE(c.redeemed_by_id, 0), COALESCE(c.license_id, 0), c.redeemed_at FROM license_cards c WHERE `+where+` ORDER BY c.id ASC`, args...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "导出卡密失败"})
		return
	}
	defer rows.Close()
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, batchNo))
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()
	_ = writer.Write([]string{"卡密", "状态", "兑换主体", "兑换主体ID", "授权ID", "兑换时间"})
	for rows.Next() {
		var code, cardStatus, ownerType string
		var ownerID, licenseID int64
		var redeemedAt sql.NullTime
		if rows.Scan(&code, &cardStatus, &ownerType, &ownerID, &licenseID, &redeemedAt) != nil {
			continue
		}
		at := ""
		if redeemedAt.Valid {
			at = redeemedAt.Time.Format("2006-01-02 15:04:05")
		}
		_ = writer.Write([]string{code, cardStatus, ownerType, strconv.FormatInt(ownerID, 10), strconv.FormatInt(licenseID, 10), at})
	}
}

type licenseCardRedemption struct {
	LicenseID      int64  `json:"licenseId"`
	LicenseNo      string `json:"licenseNo"`
	AppName        string `json:"appName"`
	PlanName       string `json:"planName"`
	Type           string `json:"type"`
	TypeLabel      string `json:"typeLabel"`
	LicenseKey     string `json:"licenseKey,omitempty"`
	BindingPending bool   `json:"bindingPending"`
	ExpireAt       string `json:"expireAt"`
	Idempotent     bool   `json:"idempotent"`
}

func redeemLicenseCard(db *sql.DB, cardCode, ownerType string, ownerID int64) (licenseCardRedemption, string, error) {
	if ownerType != "user" && ownerType != "agent" {
		return licenseCardRedemption{}, "兑换主体不正确", nil
	}
	cardCode = normalizeLicenseCardCode(cardCode)
	if cardCode == "" {
		return licenseCardRedemption{}, "请输入卡密", nil
	}

	tx, err := db.Begin()
	if err != nil {
		return licenseCardRedemption{}, "", err
	}
	defer tx.Rollback()

	var cardID, batchID, appID, planID int64
	var cardStatus, redeemedType, batchStatus, appName, planName, licenseType string
	var redeemedID, existingLicenseID sql.NullInt64
	var durationDays int
	var price float64
	var appEnabled bool
	err = tx.QueryRow(`
		SELECT c.id, c.batch_id, c.status, COALESCE(c.redeemed_by_type, ''), c.redeemed_by_id, c.license_id,
		       b.status, b.app_id, b.plan_id, b.app_name_snapshot, b.plan_name_snapshot,
		       b.duration_days, b.price_snapshot, b.license_type, a.enabled
		FROM license_cards c
		JOIN license_card_batches b ON b.id = c.batch_id
		JOIN apps a ON a.id = b.app_id
		WHERE c.card_code = ?
		FOR UPDATE
	`, cardCode).Scan(&cardID, &batchID, &cardStatus, &redeemedType, &redeemedID, &existingLicenseID,
		&batchStatus, &appID, &planID, &appName, &planName, &durationDays, &price, &licenseType, &appEnabled)
	if err == sql.ErrNoRows {
		return licenseCardRedemption{}, "卡密无效或不可用", nil
	}
	if err != nil {
		return licenseCardRedemption{}, "", err
	}

	if cardStatus == licenseCardStatusRedeemed {
		if redeemedType != ownerType || !redeemedID.Valid || redeemedID.Int64 != ownerID || !existingLicenseID.Valid {
			return licenseCardRedemption{}, "卡密无效或不可用", nil
		}
		redemption, err := loadExistingCardLicense(tx, existingLicenseID.Int64, appName, planName)
		if err != nil {
			return licenseCardRedemption{}, "", err
		}
		redemption.Idempotent = true
		return redemption, "", nil
	}
	if cardStatus != licenseCardStatusUnused || batchStatus != licenseCardBatchActive || !appEnabled {
		return licenseCardRedemption{}, "卡密无效或不可用", nil
	}

	ownerTable := "users"
	if ownerType == "agent" {
		ownerTable = "agents"
	}
	var ownerEnabled bool
	if err := tx.QueryRow("SELECT enabled FROM "+ownerTable+" WHERE id = ? FOR UPDATE", ownerID).Scan(&ownerEnabled); err != nil || !ownerEnabled {
		if err == nil || err == sql.ErrNoRows {
			return licenseCardRedemption{}, "当前账号不可兑换卡密", nil
		}
		return licenseCardRedemption{}, "", err
	}

	licenseNo, err := generateCardLicenseNo()
	if err != nil {
		return licenseCardRedemption{}, "", err
	}
	licenseKey := ""
	if licenseType == "key" {
		licenseKey, err = generateRandomLicenseKey()
		if err != nil {
			return licenseCardRedemption{}, "", err
		}
	}
	now := time.Now()
	var expiredAt sql.NullTime
	if durationDays > 0 {
		expiredAt = sql.NullTime{Time: now.AddDate(0, 0, durationDays), Valid: true}
	}
	result, err := tx.Exec(`
		INSERT INTO licenses
		(license_no, app_id, plan_id, original_price, type, status, source, owner_type, owner_id,
		 duration_days, started_at, expired_at, license_key, remark)
		VALUES (?, ?, ?, ?, ?, 'active', 'card', ?, ?, ?, ?, ?, ?, ?)
	`, licenseNo, appID, planID, price, licenseType, ownerType, ownerID, durationDays, now, expiredAt, licenseKey,
		fmt.Sprintf("卡密兑换，批次ID：%d", batchID))
	if err != nil {
		return licenseCardRedemption{}, "", err
	}
	licenseID, err := result.LastInsertId()
	if err != nil {
		return licenseCardRedemption{}, "", err
	}
	result, err = tx.Exec(`
		UPDATE license_cards SET status = 'redeemed', redeemed_by_type = ?, redeemed_by_id = ?,
		license_id = ?, redeemed_at = ? WHERE id = ? AND status = 'unused'
	`, ownerType, ownerID, licenseID, now, cardID)
	if err != nil {
		return licenseCardRedemption{}, "", err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return licenseCardRedemption{}, "卡密无效或不可用", nil
	}
	_, _ = tx.Exec(`
		INSERT INTO operation_logs (operator_type, operator_id, action, target_type, target_id, detail)
		VALUES (?, ?, 'license_card_redeem', 'license', ?, JSON_OBJECT('cardId', ?, 'batchId', ?, 'cardSuffix', ?))
	`, ownerType, ownerID, licenseID, cardID, batchID, cardCode[len(cardCode)-4:])
	if err := tx.Commit(); err != nil {
		return licenseCardRedemption{}, "", err
	}

	return newLicenseCardRedemption(licenseID, licenseNo, appName, planName, licenseType, licenseKey, expiredAt, false), "", nil
}

func generateCardLicenseNo() (string, error) {
	raw := make([]byte, 5)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return fmt.Sprintf("LIC-CARD-%d-%X", time.Now().UnixMilli(), raw), nil
}

func newLicenseCardRedemption(licenseID int64, licenseNo, appName, planName, licenseType, licenseKey string, expiredAt sql.NullTime, idempotent bool) licenseCardRedemption {
	expireAt := "永久"
	if expiredAt.Valid {
		expireAt = expiredAt.Time.Format("2006-01-02 15:04:05")
	}
	return licenseCardRedemption{
		LicenseID: licenseID, LicenseNo: licenseNo, AppName: appName, PlanName: planName,
		Type: licenseType, TypeLabel: licenseCardTypeLabel(licenseType), LicenseKey: licenseKey,
		BindingPending: licenseType != "key", ExpireAt: expireAt, Idempotent: idempotent,
	}
}

func loadExistingCardLicense(tx *sql.Tx, licenseID int64, appName, planName string) (licenseCardRedemption, error) {
	var licenseNo, licenseType, licenseKey string
	var expiredAt sql.NullTime
	if err := tx.QueryRow("SELECT license_no, type, license_key, expired_at FROM licenses WHERE id = ?", licenseID).Scan(&licenseNo, &licenseType, &licenseKey, &expiredAt); err != nil {
		return licenseCardRedemption{}, err
	}
	return newLicenseCardRedemption(licenseID, licenseNo, appName, planName, licenseType, licenseKey, expiredAt, true), nil
}

func redeemLicenseCardHandler(c *gin.Context, ownerType string) {
	role := c.GetString("role")
	if role != ownerType {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "无权限"})
		return
	}
	ownerValue, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "登录状态无效"})
		return
	}
	ownerID := int64(ownerValue.(uint))
	var req struct {
		CardCode string `json:"cardCode" binding:"required"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请输入卡密"})
		return
	}
	limitKey := ownerType + ":" + strconv.FormatInt(ownerID, 10)
	if !licenseCardRedeemLimiter.allow(limitKey, time.Now()) {
		c.JSON(http.StatusOK, gin.H{"code": 429, "msg": "操作过于频繁，请稍后再试"})
		return
	}
	db, err := openLicenseCardDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化卡密模块失败"})
		return
	}
	defer db.Close()
	result, message, err := redeemLicenseCard(db, req.CardCode, ownerType, ownerID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "兑换失败，请稍后重试"})
		return
	}
	if message != "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": message})
		return
	}
	msg := "卡密兑换成功"
	if result.Idempotent {
		msg = "该卡密已由当前账号兑换"
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": msg, "data": result})
}

func UserLicenseCardRedeem(c *gin.Context)  { redeemLicenseCardHandler(c, "user") }
func AgentLicenseCardRedeem(c *gin.Context) { redeemLicenseCardHandler(c, "agent") }
