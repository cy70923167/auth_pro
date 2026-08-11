package handler

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

func generateRandomLicenseKey() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	key := make([]byte, 16)
	limit := big.NewInt(int64(len(alphabet)))
	for i := range key {
		index, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		key[i] = alphabet[index.Int64()]
	}
	return string(key), nil
}

// LicenseList 授权列表（分页+筛选）
func LicenseList(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	keyword := c.Query("keyword")
	lType := c.Query("type")
	status := c.Query("status")
	appId := c.Query("appId")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	where := []string{"1=1"}
	args := []any{}

	if keyword != "" {
		where = append(where, "(ld.domain LIKE ? OR l.license_key LIKE ?)")
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if lType != "" {
		where = append(where, "l.type = ?")
		args = append(args, lType)
	}
	if status != "" {
		if status == "disabled" {
			where = append(where, "l.status = 'revoked'")
		} else {
			where = append(where, "l.status = ?")
			args = append(args, status)
		}
	}
	if appId != "" {
		where = append(where, "l.app_id = ?")
		args = append(args, appId)
	}

	whereSQL := strings.Join(where, " AND ")

	// 总数
	var total int64
	countSQL := fmt.Sprintf(`
		SELECT COUNT(DISTINCT l.id) FROM licenses l
		LEFT JOIN license_domains ld ON ld.license_id = l.id
		WHERE %s
	`, whereSQL)
	db.QueryRow(countSQL, args...).Scan(&total)

	// 列表
	offset := (page - 1) * pageSize
	listSQL := fmt.Sprintf(`
		SELECT l.id, COALESCE(ld.domain, l.license_key) as domain,
		       COALESCE(a.app_name, '') as app_name, l.app_id, l.type, l.status,
		       l.owner_type, l.owner_id,
		       CASE
		         WHEN l.owner_type = 'user' THEN COALESCE(NULLIF(u.nickname, ''), u.email, '')
		         WHEN l.owner_type = 'agent' THEN COALESCE(NULLIF(ag.name, ''), ag.email, '')
		         ELSE ''
		       END as owner_name,
		       l.expired_at, l.remark, l.created_at,
		       (SELECT COUNT(*) FROM verify_logs v WHERE v.license_id = l.id) as verify_count
		FROM licenses l
		LEFT JOIN apps a ON a.id = l.app_id
		LEFT JOIN users u ON l.owner_type = 'user' AND u.id = l.owner_id
		LEFT JOIN agents ag ON l.owner_type = 'agent' AND ag.id = l.owner_id
		LEFT JOIN license_domains ld ON ld.license_id = l.id
		WHERE %s
		ORDER BY l.created_at DESC
		LIMIT ? OFFSET ?
	`, whereSQL)

	listArgs := append(args, pageSize, offset)
	rows, err := db.Query(listSQL, listArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	typeLabels := map[string]string{"domain": "单域名", "wildcard": "泛域名", "ip": "IP", "key": "密钥"}
	statusLabels := map[string]string{"active": "正常", "expired": "已过期", "revoked": "已禁用"}

	type listItem struct {
		ID          int64  `json:"id"`
		Domain      string `json:"domain"`
		AppName     string `json:"appName"`
		AppID       int64  `json:"appId"`
		Type        string `json:"type"`
		TypeLabel   string `json:"typeLabel"`
		Status      string `json:"status"`
		StatusLabel string `json:"statusLabel"`
		OwnerType   string `json:"ownerType"`
		OwnerID     int64  `json:"ownerId"`
		OwnerName   string `json:"ownerName"`
		ExpireAt    string `json:"expireAt"`
		VerifyCount int64  `json:"verifyCount"`
		CreatedAt   string `json:"createdAt"`
		Remark      string `json:"remark"`
	}

	var list []listItem
	for rows.Next() {
		var item listItem
		var expiredAt sql.NullTime
		var createdAt time.Time
		var remark sql.NullString
		err := rows.Scan(&item.ID, &item.Domain, &item.AppName, &item.AppID,
			&item.Type, &item.Status, &item.OwnerType, &item.OwnerID, &item.OwnerName,
			&expiredAt, &remark, &createdAt, &item.VerifyCount)
		if err != nil {
			continue
		}
		item.TypeLabel = typeLabels[item.Type]
		if item.Status == "revoked" {
			item.Status = "disabled"
		}
		item.StatusLabel = statusLabels[item.Status]
		if item.Status == "disabled" {
			item.StatusLabel = "已禁用"
		}
		if expiredAt.Valid {
			item.ExpireAt = expiredAt.Time.Format("2006-01-02 15:04")
		}
		item.CreatedAt = createdAt.Format("2006-01-02 15:04")
		if remark.Valid {
			item.Remark = remark.String
		}
		list = append(list, item)
	}
	if list == nil {
		list = []listItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"list":  list,
			"total": total,
		},
	})
}

// UserLicenseQuery 管理端按用户账号或邮箱查询授权。
func UserLicenseQuery(c *gin.Context) {
	account := strings.TrimSpace(c.Query("account"))
	if account == "" {
		account = strings.TrimSpace(c.Query("email"))
	}
	if account == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请输入用户账号或邮箱"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
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

	const userCondition = "l.owner_type = 'user' AND (u.nickname = ? OR u.email = ?)"
	queryArgs := []any{account, strings.ToLower(account)}

	var total int64
	countSQL := `
		SELECT COUNT(*)
		FROM licenses l
		JOIN users u ON u.id = l.owner_id
		WHERE ` + userCondition
	if err := db.QueryRowContext(c.Request.Context(), countSQL, queryArgs...).Scan(&total); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询授权数量失败"})
		return
	}

	listSQL := `
		SELECT l.id, l.license_no, u.id, COALESCE(NULLIF(u.nickname, ''), u.email), u.email,
		       COALESCE(a.app_name, ''), COALESCE(NULLIF(p.name, ''), '未关联套餐'),
		       l.type,
		       CASE
		         WHEN l.status = 'active' AND l.expired_at IS NOT NULL AND l.expired_at <= NOW() THEN 'expired'
		         ELSE l.status
		       END,
		       l.started_at, l.expired_at
		FROM licenses l
		JOIN users u ON u.id = l.owner_id
		LEFT JOIN apps a ON a.id = l.app_id
		LEFT JOIN license_plans p ON p.id = l.plan_id
		WHERE ` + userCondition + `
		ORDER BY l.started_at DESC, l.id DESC
		LIMIT ? OFFSET ?`
	listArgs := append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := db.QueryContext(c.Request.Context(), listSQL, listArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询授权失败"})
		return
	}
	defer rows.Close()

	type licenseQueryItem struct {
		ID              int64   `json:"id"`
		LicenseNo       string  `json:"licenseNo"`
		UserID          int64   `json:"userId"`
		Account         string  `json:"account"`
		Email           string  `json:"email"`
		AppName         string  `json:"appName"`
		PlanName        string  `json:"planName"`
		LicenseType     string  `json:"licenseType"`
		LicenseTypeName string  `json:"licenseTypeName"`
		Status          string  `json:"status"`
		StatusName      string  `json:"statusName"`
		OpenedAt        string  `json:"openedAt"`
		ExpiredAt       *string `json:"expiredAt"`
		Permanent       bool    `json:"permanent"`
	}

	typeLabels := map[string]string{
		"domain":   "单域名",
		"wildcard": "泛域名",
		"ip":       "IP",
		"key":      "密钥",
	}
	statusLabels := map[string]string{
		"active":  "正常",
		"expired": "已过期",
		"revoked": "已吊销",
	}

	list := make([]licenseQueryItem, 0)
	for rows.Next() {
		var item licenseQueryItem
		var startedAt time.Time
		var expiredAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.LicenseNo, &item.UserID, &item.Account, &item.Email,
			&item.AppName, &item.PlanName, &item.LicenseType, &item.Status,
			&startedAt, &expiredAt,
		); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取授权信息失败"})
			return
		}
		item.LicenseTypeName = typeLabels[item.LicenseType]
		item.StatusName = statusLabels[item.Status]
		item.OpenedAt = startedAt.Format("2006-01-02 15:04:05")
		item.Permanent = !expiredAt.Valid
		if expiredAt.Valid {
			value := expiredAt.Time.Format("2006-01-02 15:04:05")
			item.ExpiredAt = &value
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取授权信息失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"list":     list,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

// PublicUserLicenseQuery 首页按用户账号或邮箱查询公开授权概况。
func PublicUserLicenseQuery(c *gin.Context) {
	account := strings.TrimSpace(c.Query("account"))
	if account == "" {
		account = strings.TrimSpace(c.Query("email"))
	}
	if account == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请输入用户账号或邮箱"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
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

	const userCondition = "l.owner_type = 'user' AND (u.nickname = ? OR u.email = ?)"
	queryArgs := []any{account, strings.ToLower(account)}

	var total int64
	countSQL := `
		SELECT COUNT(*)
		FROM licenses l
		JOIN users u ON u.id = l.owner_id
		WHERE ` + userCondition
	if err := db.QueryRowContext(c.Request.Context(), countSQL, queryArgs...).Scan(&total); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询授权数量失败"})
		return
	}

	querySQL := `
		SELECT COALESCE(a.app_name, ''), COALESCE(NULLIF(p.name, ''), '未关联套餐'),
		       l.type,
		       CASE
		         WHEN l.status = 'active' AND l.expired_at IS NOT NULL AND l.expired_at <= NOW() THEN 'expired'
		         ELSE l.status
		       END,
		       l.started_at, l.expired_at
		FROM licenses l
		JOIN users u ON u.id = l.owner_id
		LEFT JOIN apps a ON a.id = l.app_id
		LEFT JOIN license_plans p ON p.id = l.plan_id
		WHERE ` + userCondition + `
		ORDER BY l.started_at DESC, l.id DESC
		LIMIT ? OFFSET ?`
	listArgs := append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := db.QueryContext(c.Request.Context(), querySQL, listArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询授权失败"})
		return
	}
	defer rows.Close()

	type publicLicenseItem struct {
		AppName         string  `json:"appName"`
		PlanName        string  `json:"planName"`
		LicenseType     string  `json:"licenseType"`
		LicenseTypeName string  `json:"licenseTypeName"`
		Status          string  `json:"status"`
		StatusName      string  `json:"statusName"`
		OpenedAt        string  `json:"openedAt"`
		ExpiredAt       *string `json:"expiredAt"`
		Permanent       bool    `json:"permanent"`
	}

	typeLabels := map[string]string{
		"domain":   "单域名",
		"wildcard": "泛域名",
		"ip":       "IP",
		"key":      "密钥",
	}
	statusLabels := map[string]string{
		"active":  "正常",
		"expired": "已过期",
		"revoked": "已吊销",
	}

	list := make([]publicLicenseItem, 0)
	for rows.Next() {
		var item publicLicenseItem
		var openedAt time.Time
		var expiredAt sql.NullTime
		if err := rows.Scan(
			&item.AppName, &item.PlanName, &item.LicenseType, &item.Status,
			&openedAt, &expiredAt,
		); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取授权信息失败"})
			return
		}
		item.LicenseTypeName = typeLabels[item.LicenseType]
		item.StatusName = statusLabels[item.Status]
		item.OpenedAt = openedAt.Format("2006-01-02 15:04:05")
		item.Permanent = !expiredAt.Valid
		if expiredAt.Valid {
			value := expiredAt.Time.Format("2006-01-02 15:04:05")
			item.ExpiredAt = &value
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取授权信息失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"list":     list,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

// PublicAgentQuery 首页按代理商账号查询该账号是否为代理商及其等级。
func PublicAgentQuery(c *gin.Context) {
	account := strings.TrimSpace(c.Query("account"))
	if account == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请输入代理商账号"})
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

	var email, name, levelName string
	err = db.QueryRowContext(c.Request.Context(), `
		SELECT a.email, COALESCE(NULLIF(a.name, ''), a.email),
		       COALESCE(NULLIF(l.name, ''), a.level)
		FROM agents a
		LEFT JOIN agent_levels l ON l.code = a.level
		WHERE (a.email = ? OR a.contact = ?) AND a.enabled = 1
		LIMIT 1
	`, account, account).Scan(&email, &name, &levelName)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "未查询到当前账号",
			"data": gin.H{"found": false},
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询代理商失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"found":     true,
			"account":   email,
			"agentName": name,
			"levelName": levelName,
		},
	})
}

// LicenseOwnerOptions 返回新增授权可选择的用户或代理账号。
func LicenseOwnerOptions(c *gin.Context) {
	ownerType := strings.TrimSpace(c.Query("ownerType"))
	keyword := strings.TrimSpace(c.Query("keyword"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 50 {
		limit = 20
	}

	var query string
	switch ownerType {
	case "user":
		query = `
			SELECT id, COALESCE(NULLIF(nickname, ''), email), email
			FROM users
			WHERE enabled = 1`
		if keyword != "" {
			query += " AND (nickname LIKE ? OR email LIKE ?)"
		}
	case "agent":
		query = `
			SELECT id, COALESCE(NULLIF(name, ''), email), email
			FROM agents
			WHERE enabled = 1`
		if keyword != "" {
			query += " AND (name LIKE ? OR email LIKE ? OR contact LIKE ?)"
		}
	default:
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "授权归属类型不正确"})
		return
	}
	query += " ORDER BY id DESC LIMIT ?"

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

	args := make([]any, 0, 4)
	if keyword != "" {
		likeKeyword := "%" + keyword + "%"
		args = append(args, likeKeyword, likeKeyword)
		if ownerType == "agent" {
			args = append(args, likeKeyword)
		}
	}
	args = append(args, limit)

	rows, err := db.QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询授权归属账号失败"})
		return
	}
	defer rows.Close()

	type ownerOption struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Account string `json:"account"`
		Type    string `json:"type"`
	}

	list := make([]ownerOption, 0)
	for rows.Next() {
		var item ownerOption
		if err := rows.Scan(&item.ID, &item.Name, &item.Account); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取授权归属账号失败"})
			return
		}
		item.Type = ownerType
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取授权归属账号失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": list})
}

// LicenseCreate 新增授权
func LicenseCreate(c *gin.Context) {
	var req struct {
		AppID     int64  `json:"appId" binding:"required,gt=0"`
		PlanID    int64  `json:"planId" binding:"required,gt=0"`
		Type      string `json:"type" binding:"required,oneof=domain wildcard ip key"`
		OwnerType string `json:"ownerType" binding:"required,oneof=user agent"`
		OwnerID   int64  `json:"ownerId" binding:"required,gt=0"`
		Domain    string `json:"domain"`
		Remark    string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	validatedTarget, errMsg := validateLicenseTargetForSave(req.Type, req.Domain)
	if errMsg != "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": errMsg})
		return
	}
	req.Domain = validatedTarget

	db, err := openAdminLicenseDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "系统未配置"})
		return
	}
	defer db.Close()

	// 生成授权编号
	licenseNo := fmt.Sprintf("LIC-%d", time.Now().UnixNano()/1e6)

	// 密钥类型自动生成
	licenseKey := ""
	if req.Type == "key" {
		licenseKey, err = generateRandomLicenseKey()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "生成密钥失败"})
			return
		}
	}

	now := time.Now()
	userID, _ := c.Get("user_id")

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建授权失败"})
		return
	}
	defer tx.Rollback()

	ownerErrMsg, err := validateLicenseOwner(func(query string, args ...any) licenseOwnerRowScanner {
		return tx.QueryRowContext(c.Request.Context(), query, args...)
	}, req.OwnerType, req.OwnerID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "校验授权归属账号失败"})
		return
	}
	if ownerErrMsg != "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": ownerErrMsg})
		return
	}

	plan, planErrMsg, err := loadAdminLicensePlan(func(query string, args ...any) licensePlanRowScanner {
		return tx.QueryRowContext(c.Request.Context(), query, args...)
	}, req.AppID, req.PlanID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "校验套餐失败"})
		return
	}
	if planErrMsg != "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": planErrMsg})
		return
	}

	var expiredAt sql.NullTime
	if plan.DurationDays > 0 {
		expiredAt = sql.NullTime{Time: now.AddDate(0, 0, plan.DurationDays), Valid: true}
	}

	result, err := tx.ExecContext(c.Request.Context(), `
		INSERT INTO licenses (license_no, app_id, plan_id, original_price, type, status, source, owner_type, owner_id,
		                      issued_by, duration_days, started_at, expired_at, license_key, remark)
		VALUES (?, ?, ?, ?, ?, 'active', 'admin', ?, ?, ?, ?, ?, ?, ?, ?)
	`, licenseNo, req.AppID, req.PlanID, plan.OriginalPrice, req.Type, req.OwnerType, req.OwnerID, userID,
		plan.DurationDays, now, expiredAt, licenseKey, req.Remark)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建授权失败"})
		return
	}

	licenseID, err := result.LastInsertId()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建授权失败"})
		return
	}

	// 如果是域名/泛域名/IP类型，写入 license_domains
	if req.Type != "key" && req.Domain != "" {
		isWildcard := 0
		if req.Type == "wildcard" {
			isWildcard = 1
		}
		if _, err := tx.ExecContext(c.Request.Context(),
			"INSERT INTO license_domains (license_id, domain, is_wildcard) VALUES (?, ?, ?)",
			licenseID, req.Domain, isWildcard); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存授权目标失败"})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建授权失败"})
		return
	}

	queueAdminLicenseOpenedMail(licenseID)

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "创建成功", "data": gin.H{"id": licenseID}})
}

type adminLicensePlan struct {
	DurationDays  int
	OriginalPrice float64
}

type licensePlanRowScanner interface {
	Scan(dest ...any) error
}

type licensePlanQueryFunc func(query string, args ...any) licensePlanRowScanner

func loadAdminLicensePlan(queryRow licensePlanQueryFunc, appID, planID int64) (adminLicensePlan, string, error) {
	var plan adminLicensePlan
	if err := queryRow(`
		SELECT p.duration_days, p.price
		FROM license_plans p
		JOIN apps a ON a.id = p.app_id
		WHERE p.id = ? AND p.app_id = ? AND p.enabled = 1 AND a.enabled = 1
		FOR UPDATE
	`, planID, appID).Scan(&plan.DurationDays, &plan.OriginalPrice); err != nil {
		if err == sql.ErrNoRows {
			return adminLicensePlan{}, "套餐不存在、已禁用或不属于所选应用", nil
		}
		return adminLicensePlan{}, "", err
	}
	return plan, "", nil
}

type licenseOwnerRowScanner interface {
	Scan(dest ...any) error
}

type licenseOwnerQueryFunc func(query string, args ...any) licenseOwnerRowScanner

func validateLicenseOwner(queryRow licenseOwnerQueryFunc, ownerType string, ownerID int64) (string, error) {
	if ownerID <= 0 {
		return "请选择授权归属账号", nil
	}

	var query, ownerLabel string
	switch ownerType {
	case "user":
		query = "SELECT enabled FROM users WHERE id = ? FOR UPDATE"
		ownerLabel = "用户"
	case "agent":
		query = "SELECT enabled FROM agents WHERE id = ? FOR UPDATE"
		ownerLabel = "代理"
	default:
		return "授权归属类型不正确", nil
	}

	var enabled bool
	if err := queryRow(query, ownerID).Scan(&enabled); err != nil {
		if err == sql.ErrNoRows {
			return "指定" + ownerLabel + "不存在或已禁用", nil
		}
		return "", err
	}
	if !enabled {
		return "指定" + ownerLabel + "不存在或已禁用", nil
	}
	return "", nil
}

// LicenseUpdate 编辑授权
func LicenseUpdate(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		AppID    int64  `json:"appId"`
		Type     string `json:"type"`
		Domain   string `json:"domain"`
		ExpireAt string `json:"expireAt"`
		Remark   string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	validatedTarget, errMsg := validateLicenseTargetForSave(req.Type, req.Domain)
	if errMsg != "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": errMsg})
		return
	}
	req.Domain = validatedTarget

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var expiredAt sql.NullTime
	if req.ExpireAt != "" {
		t, err := time.Parse("2006-01-02T15:04:05.000Z", req.ExpireAt)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", req.ExpireAt)
			if err != nil {
				t, _ = time.Parse("2006-01-02 15:04", req.ExpireAt)
			}
		}
		if !t.IsZero() {
			expiredAt = sql.NullTime{Time: t, Valid: true}
		}
	}

	_, err = db.Exec(`
		UPDATE licenses SET type = ?, expired_at = ?, remark = ? WHERE id = ?
	`, req.Type, expiredAt, req.Remark, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "更新失败: " + err.Error()})
		return
	}

	// 更新域名
	if req.Type != "key" && req.Domain != "" {
		isWildcard := 0
		if req.Type == "wildcard" {
			isWildcard = 1
		}
		db.Exec("DELETE FROM license_domains WHERE license_id = ?", id)
		db.Exec("INSERT INTO license_domains (license_id, domain, is_wildcard) VALUES (?, ?, ?)", id, req.Domain, isWildcard)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "更新成功"})
}

// LicenseToggle 启用/禁用授权
func LicenseToggle(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	dbStatus := req.Status
	if dbStatus == "disabled" {
		dbStatus = "revoked"
	}

	_, err = db.Exec("UPDATE licenses SET status = ? WHERE id = ?", dbStatus, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "操作失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "操作成功"})
}

// LicenseDelete 删除授权
func LicenseDelete(c *gin.Context) {
	id := c.Param("id")

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	db.Exec("DELETE FROM license_domains WHERE license_id = ?", id)
	db.Exec("DELETE FROM verify_logs WHERE license_id = ?", id)
	_, err = db.Exec("DELETE FROM licenses WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "删除成功"})
}

func validateLicenseTargetForSave(licenseType, target string) (string, string) {
	target = strings.TrimSpace(target)
	if licenseType == "key" {
		return target, ""
	}
	target = strings.ToLower(target)

	switch licenseType {
	case "domain":
		if !isValidSingleDomainTarget(target) {
			return "", "单域名格式不正确"
		}
	case "ip":
		if net.ParseIP(target) == nil {
			return "", "IP 格式不正确"
		}
	case "wildcard":
		if !strings.HasPrefix(target, "*.") || !isValidSingleDomainTarget(strings.TrimPrefix(target, "*.")) {
			return "", "泛域名格式不正确"
		}
	case "key":
		return target, ""
	default:
		return "", "授权类型不正确"
	}

	return target, ""
}

func isValidSingleDomainTarget(domain string) bool {
	if domain == "" || strings.HasPrefix(domain, "*.") || strings.ContainsAny(domain, "/:@ \\\t\r\n") || strings.HasSuffix(domain, ".") {
		return false
	}
	if net.ParseIP(domain) != nil {
		return false
	}

	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, ch := range label {
			if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
				continue
			}
			return false
		}
	}

	tld := labels[len(labels)-1]
	if len(tld) < 2 {
		return false
	}
	for _, ch := range tld {
		if ch < 'a' || ch > 'z' {
			return false
		}
	}
	return true
}
