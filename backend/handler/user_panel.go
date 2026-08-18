package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"auto_pro/config"
	"auto_pro/middleware"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// ========== 用户端认证 ==========

type userLoginRequest struct {
	Account  string `json:"account"`
	Email    string `json:"email"` // 兼容旧版前端字段
	Password string `json:"password" binding:"required"`
}

type userRegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	EmailCode string `json:"emailCode" binding:"required,len=6,numeric"`
	Nickname  string `json:"nickname" binding:"required"`
	Password  string `json:"password" binding:"required,min=6"`
	Phone     string `json:"phone"`
}

// UserLogin 用户登录（支持邮箱 / 手机号 / 用户ID 三种账号标识）
func UserLogin(c *gin.Context) {
	var req userLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 详细错误信息用于调试
		fmt.Printf("[UserLogin] JSON绑定失败: %v, 请求体: %+v\n", err, req)
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": fmt.Sprintf("参数错误: %v", err)})
		return
	}

	account := strings.TrimSpace(req.Account)
	if account == "" {
		account = strings.TrimSpace(req.Email)
	}
	if account == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请输入账号"})
		return
	}

	if remaining := middleware.LoginLockRemaining(c.ClientIP(), account); remaining > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 429, "msg": fmt.Sprintf("登录尝试次数过多，请 %d 秒后重试", int(remaining.Seconds())+1)})
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

	if err := ensureUserAuthStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化失败"})
		return
	}

	// 账号类型识别：含 @ 按邮箱；纯数字同时匹配手机号与用户ID（手机号优先，
	// 自增ID与11位手机号冲突概率可忽略）；其余输入按手机号匹配
	var id uint
	var email, passwordHash, nickname string
	var row *sql.Row
	if strings.Contains(account, "@") {
		row = db.QueryRow("SELECT id, email, password_hash, nickname FROM users WHERE email = ? AND enabled = 1", strings.ToLower(account))
	} else if uid, parseErr := strconv.ParseUint(account, 10, 64); parseErr == nil {
		row = db.QueryRow(
			"SELECT id, email, password_hash, nickname FROM users WHERE (phone = ? OR id = ?) AND enabled = 1 ORDER BY (phone = ?) DESC LIMIT 1",
			account, uid, account,
		)
	} else {
		row = db.QueryRow("SELECT id, email, password_hash, nickname FROM users WHERE phone = ? AND enabled = 1", account)
	}
	err = row.Scan(&id, &email, &passwordHash, &nickname)
	if err != nil {
		middleware.RecordLoginFailure(c.ClientIP(), account)
		c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "账号或密码错误"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		middleware.RecordLoginFailure(c.ClientIP(), account)
		c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "账号或密码错误"})
		return
	}

	middleware.RecordLoginSuccess(c.ClientIP(), account)

	_, _ = db.Exec("UPDATE users SET last_login_at = NOW(), last_login_ip = ? WHERE id = ?", c.ClientIP(), id)

	now := time.Now()
	claims := &middleware.Claims{
		UserID:   id,
		Username: email,
		Role:     "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(middleware.JWTSecret())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "生成token失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200, "msg": "登录成功",
		"data": gin.H{
			"accessToken": token,
			"userId":      id,
			"email":       email,
			"nickname":    nickname,
		},
	})
}

// UserRegister 用户注册
func UserRegister(c *gin.Context) {
	if !isRegistrationEnabled() {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "普通用户注册已关闭，请联系管理员"})
		return
	}

	var req userRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误，请检查邮箱格式和密码长度"})
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.EmailCode = strings.TrimSpace(req.EmailCode)
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Password = strings.TrimSpace(req.Password)
	req.Phone = strings.TrimSpace(req.Phone)
	if req.Email == "" || len(req.EmailCode) != 6 || req.Nickname == "" || len(req.Password) < 6 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误，请检查邮箱、验证码和密码长度"})
		return
	}
	if req.Phone != "" && !phoneRegexp.MatchString(req.Phone) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "手机号格式不正确"})
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

	if err := ensureUserAuthStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化失败"})
		return
	}

	var exists int
	if err := db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", req.Email).Scan(&exists); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "检查邮箱失败"})
		return
	}
	if exists > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该邮箱已注册"})
		return
	}
	if req.Phone != "" {
		var phoneExists int
		if err := db.QueryRow("SELECT COUNT(*) FROM users WHERE phone = ?", req.Phone).Scan(&phoneExists); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "检查手机号失败"})
			return
		}
		if phoneExists > 0 {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该手机号已被使用"})
			return
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "密码处理失败"})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "注册失败"})
		return
	}
	if err := consumeRegisterEmailCode(tx, req.Email, req.EmailCode); err != nil {
		switch err {
		case errEmailCodeIncorrect:
			// 错误次数需要落库；事务中没有用户写入。
			if commitErr := tx.Commit(); commitErr != nil {
				c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "验证码校验失败"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		case errEmailCodeInvalid:
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "验证码校验失败"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		default:
			_ = tx.Rollback()
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "验证码校验失败"})
		}
		return
	}

	var phoneArg interface{}
	if req.Phone != "" {
		phoneArg = req.Phone
	}
	result, err := tx.Exec("INSERT INTO users (email, password_hash, nickname, phone) VALUES (?, ?, ?, ?)", req.Email, string(hash), req.Nickname, phoneArg)
	if err != nil {
		_ = tx.Rollback()
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "注册失败"})
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "注册失败"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "注册失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "注册成功", "data": gin.H{"userId": id}})
}

// ========== 用户端授权列表 ==========

// UserLicenseList 用户的授权列表
func UserLicenseList(c *gin.Context) {
	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")
	if role != "user" {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "无权限"})
		return
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureLicensePriceSnapshotSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化授权价格快照失败"})
		return
	}

	keyword := c.Query("keyword")
	appId := c.Query("appId")
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	where := "WHERE l.owner_type = 'user' AND l.owner_id = ?"
	args := []interface{}{userID}

	if keyword != "" {
		where += " AND (ld.domain LIKE ? OR l.license_key LIKE ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if appId != "" {
		where += " AND l.app_id = ?"
		args = append(args, appId)
	}
	if status == "active" {
		where += " AND l.status = 'active' AND (l.expired_at IS NULL OR l.expired_at > NOW() + INTERVAL 7 DAY)"
	} else if status == "expiring" {
		where += " AND l.status = 'active' AND l.expired_at IS NOT NULL AND l.expired_at <= NOW() + INTERVAL 7 DAY AND l.expired_at > NOW()"
	} else if status == "expired" {
		where += " AND (l.status = 'expired' OR (l.expired_at IS NOT NULL AND l.expired_at <= NOW()))"
	}

	// count
	countSQL := fmt.Sprintf(`SELECT COUNT(DISTINCT l.id) FROM licenses l LEFT JOIN license_domains ld ON ld.license_id = l.id %s`, where)
	var total int
	db.QueryRow(countSQL, args...).Scan(&total)

	// query
	querySQL := fmt.Sprintf(`
		SELECT l.id, l.license_no, l.app_id, a.app_name, l.type, l.status,
		       l.source, l.original_price, l.expired_at, l.created_at, l.license_key,
		       GROUP_CONCAT(ld.domain SEPARATOR ', ') as domains
		FROM licenses l
		LEFT JOIN apps a ON a.id = l.app_id
		LEFT JOIN license_domains ld ON ld.license_id = l.id
		%s
		GROUP BY l.id
		ORDER BY l.created_at DESC
		LIMIT ? OFFSET ?
	`, where)
	args = append(args, pageSize, offset)

	rows, err := db.Query(querySQL, args...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	typeLabels := map[string]string{"domain": "单域名", "wildcard": "泛域名", "ip": "IP", "key": "密钥"}
	sourceLabels := map[string]string{"admin": "管理员开通", "agent": "代理开通", "user_purchase": "自助购买", "card": "卡密兑换"}

	type licenseItem struct {
		ID             int64    `json:"id"`
		LicenseNo      string   `json:"licenseNo"`
		AppID          int64    `json:"appId"`
		AppName        string   `json:"appName"`
		Type           string   `json:"type"`
		TypeLabel      string   `json:"typeLabel"`
		Status         string   `json:"status"`
		StatusLabel    string   `json:"statusLabel"`
		Source         string   `json:"source"`
		Amount         *float64 `json:"amount"`
		Domain         string   `json:"domain"`
		BindingPending bool     `json:"bindingPending"`
		ExpireAt       string   `json:"expireAt"`
		CreatedAt      string   `json:"createdAt"`
	}

	var list []licenseItem
	for rows.Next() {
		var item licenseItem
		var expiredAt sql.NullTime
		var createdAt sql.NullTime
		var licenseKey, source string
		var originalPrice sql.NullFloat64
		var domains sql.NullString

		rows.Scan(&item.ID, &item.LicenseNo, &item.AppID, &item.AppName, &item.Type,
			&item.Status, &source, &originalPrice, &expiredAt, &createdAt, &licenseKey, &domains)

		item.TypeLabel = typeLabels[item.Type]
		item.Source = sourceLabels[source]
		if item.Source == "" {
			item.Source = source
		}
		if originalPrice.Valid {
			amount := originalPrice.Float64
			item.Amount = &amount
		}

		if domains.Valid && domains.String != "" {
			item.Domain = domains.String
		} else if licenseKey != "" {
			item.Domain = licenseKey
		}
		item.BindingPending = item.Type != "key" && item.Domain == ""

		if expiredAt.Valid {
			item.ExpireAt = expiredAt.Time.Format("2006-01-02")
		} else {
			item.ExpireAt = "永久"
		}
		if createdAt.Valid {
			item.CreatedAt = createdAt.Time.Format("2006-01-02")
		}

		// status label
		if item.Status == "active" {
			if expiredAt.Valid && expiredAt.Time.Before(time.Now().AddDate(0, 0, 7)) {
				item.StatusLabel = "即将到期"
				item.Status = "expiring"
			} else {
				item.StatusLabel = "正常"
			}
		} else if item.Status == "expired" {
			item.StatusLabel = "已过期"
		} else if item.Status == "revoked" {
			item.StatusLabel = "已吊销"
		}

		list = append(list, item)
	}

	if list == nil {
		list = []licenseItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200, "msg": "",
		"data": gin.H{
			"list":     list,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

// UserLicenseUpdateTarget 用户修改授权目标，仅允许修改域名/IP/密钥
func UserLicenseUpdateTarget(c *gin.Context) {
	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")
	if role != "user" {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "无权限"})
		return
	}

	licenseID := c.Param("id")
	type updateReq struct {
		Target string `json:"target" binding:"required"`
	}
	var req updateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请填写授权目标"})
		return
	}
	req.Target = strings.TrimSpace(req.Target)
	if req.Target == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请填写授权目标"})
		return
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var licenseType string
	err = db.QueryRow(`
		SELECT type
		FROM licenses
		WHERE id = ? AND owner_type = 'user' AND owner_id = ?
	`, licenseID, userID).Scan(&licenseType)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "授权不存在"})
		return
	}
	if licenseType != "domain" && licenseType != "wildcard" && licenseType != "ip" && licenseType != "key" {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "该授权类型不允许用户修改"})
		return
	}

	validatedTarget, errMsg := validateLicenseTargetForSave(licenseType, req.Target)
	if errMsg != "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": errMsg})
		return
	}
	req.Target = validatedTarget

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "系统错误"})
		return
	}
	defer tx.Rollback()

	if licenseType == "key" {
		_, err = tx.Exec("UPDATE licenses SET license_key = ? WHERE id = ? AND owner_type = 'user' AND owner_id = ?", req.Target, licenseID, userID)
	} else {
		isWildcard := 0
		if licenseType == "wildcard" {
			isWildcard = 1
		}
		_, err = tx.Exec("DELETE FROM license_domains WHERE license_id = ?", licenseID)
		if err == nil {
			_, err = tx.Exec("INSERT INTO license_domains (license_id, domain, is_wildcard) VALUES (?, ?, ?)", licenseID, req.Target, isWildcard)
		}
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "更新失败"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "提交失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "更新成功"})
}

// UserLicenseRefreshKey 重置密钥类型授权的密钥
func UserLicenseRefreshKey(c *gin.Context) {
	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")
	if role != "user" {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "无权限"})
		return
	}

	licenseID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || licenseID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "授权ID不正确"})
		return
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "系统错误"})
		return
	}
	defer tx.Rollback()

	var licenseType string
	err = tx.QueryRow(`
		SELECT type FROM licenses
		WHERE id = ? AND owner_type = 'user' AND owner_id = ?
		FOR UPDATE
	`, licenseID, userID).Scan(&licenseType)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "授权不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询授权失败"})
		return
	}
	if licenseType != "key" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "只有密钥类型授权可以重置密钥"})
		return
	}

	licenseKey, err := generateRandomLicenseKey()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "生成密钥失败"})
		return
	}
	result, err := tx.Exec(`
		UPDATE licenses SET license_key = ?
		WHERE id = ? AND owner_type = 'user' AND owner_id = ? AND type = 'key'
	`, licenseKey, licenseID, userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "重置密钥失败"})
		return
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		c.JSON(http.StatusOK, gin.H{"code": 409, "msg": "授权状态已变化，请刷新后重试"})
		return
	}
	if _, err = tx.Exec("DELETE FROM license_domains WHERE license_id = ?", licenseID); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "清理授权目标失败"})
		return
	}
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "提交更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "密钥已重置", "data": gin.H{"id": licenseID, "licenseKey": licenseKey}})
}

// UserAppList 用户可见的应用列表（用于筛选下拉）
func UserAppList(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, app_name FROM apps WHERE enabled = 1 ORDER BY id ASC")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	defer rows.Close()

	type appItem struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	var list []appItem
	for rows.Next() {
		var item appItem
		rows.Scan(&item.ID, &item.Name)
		list = append(list, item)
	}
	if list == nil {
		list = []appItem{}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": list})
}

// UserAppListForPurchase 购买页应用列表（含启用套餐）
func UserAppListForPurchase(c *gin.Context) {
	if !selfPurchaseEnabledForPurchase() {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "用户自助购买已关闭，请联系管理员"})
		return
	}

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
	if err := ensurePurchasePromotionSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化促销活动失败"})
		return
	}

	rows, err := db.Query(`
		SELECT id, app_name, description, icon, purchase_license_type_mask
		FROM apps
		WHERE enabled = 1
		  AND purchase_license_type_mask <> 0
		  AND EXISTS (SELECT 1 FROM license_plans p WHERE p.app_id = apps.id AND p.enabled = 1)
		ORDER BY id ASC
	`)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	defer rows.Close()

	type purchasePlanPromotion struct {
		ID       int64   `json:"id"`
		Name     string  `json:"name"`
		RuleType string  `json:"ruleType"`
		Discount float64 `json:"discount,omitempty"`
		StartsAt string  `json:"startsAt"`
		EndsAt   string  `json:"endsAt"`
	}
	type purchasePlan struct {
		ID             int64                  `json:"id"`
		Name           string                 `json:"name"`
		LicenseType    string                 `json:"licenseType"`
		DurationDays   int                    `json:"durationDays"`
		DurationText   string                 `json:"durationText"`
		OriginalPrice  float64                `json:"originalPrice"`
		BasePrice      float64                `json:"basePrice"`
		Price          float64                `json:"price"`
		DiscountAmount float64                `json:"discountAmount"`
		Promotion      *purchasePlanPromotion `json:"promotion,omitempty"`
	}
	type purchaseApp struct {
		ID                   int64          `json:"id"`
		Name                 string         `json:"name"`
		Description          string         `json:"desc"`
		Icon                 string         `json:"icon"`
		PurchaseLicenseTypes []string       `json:"purchaseLicenseTypes"`
		Plans                []purchasePlan `json:"plans"`
	}

	var list []purchaseApp
	for rows.Next() {
		var item purchaseApp
		var purchaseLicenseTypeMask uint8
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Icon, &purchaseLicenseTypeMask); err != nil {
			continue
		}
		item.PurchaseLicenseTypes = purchaseLicenseTypesFromMask(purchaseLicenseTypeMask)

		planRows, err := db.Query(`
			SELECT id, name, license_type, duration_days, price
			FROM license_plans
			WHERE app_id = ? AND enabled = 1
			ORDER BY sort ASC, id ASC
		`, item.ID)
		if err == nil {
			for planRows.Next() {
				var plan purchasePlan
				var licenseType, priceText sql.NullString
				if err := planRows.Scan(&plan.ID, &plan.Name, &licenseType, &plan.DurationDays, &priceText); err != nil {
					continue
				}
				if licenseType.Valid {
					plan.LicenseType = licenseType.String
				}
				if priceText.Valid && priceText.String != "" {
					plan.OriginalPrice, _ = strconv.ParseFloat(priceText.String, 64)
				}
				if quote, quoteErr := quoteUserPurchase(db, purchasePlanPricing{
					AppID:      item.ID,
					PlanID:     plan.ID,
					PriceCents: floatAmountToCents(plan.OriginalPrice),
				}); quoteErr == nil {
					plan.BasePrice = purchaseAmount(quote.BaseCents)
					plan.Price = purchaseAmount(quote.AmountCents)
					plan.DiscountAmount = purchaseAmount(quote.DiscountCents)
					if quote.PromotionID > 0 {
						plan.Promotion = &purchasePlanPromotion{
							ID:       quote.PromotionID,
							Name:     quote.PromotionName,
							RuleType: string(quote.PromotionRuleType),
							Discount: quote.PromotionDiscount,
							StartsAt: quote.PromotionStartsAt,
							EndsAt:   quote.PromotionEndsAt,
						}
					}
				} else {
					plan.BasePrice = plan.OriginalPrice
					plan.Price = plan.OriginalPrice
				}
				if plan.DurationDays == 0 {
					plan.DurationText = "永久"
				} else {
					plan.DurationText = fmt.Sprintf("%d天", plan.DurationDays)
				}
				item.Plans = append(item.Plans, plan)
			}
			planRows.Close()
		}
		if item.Plans == nil {
			item.Plans = []purchasePlan{}
		}
		list = append(list, item)
	}
	if list == nil {
		list = []purchaseApp{}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": list})
}

// UserGetBalance 获取用户余额
func UserGetBalance(c *gin.Context) {
	userID, _ := c.Get("user_id")

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var balance float64
	db.QueryRow("SELECT balance FROM users WHERE id = ?", userID).Scan(&balance)

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{"balance": balance}})
}

// UserPurchase 用户购买授权
func UserPurchase(c *gin.Context) {
	userID, ok := getUserPanelID(c)
	if !ok {
		return
	}
	if !selfPurchaseEnabledForPurchase() {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "用户自助购买已关闭，请联系管理员"})
		return
	}

	type purchaseReq struct {
		AppID     int64  `json:"appId" binding:"required"`
		PlanID    int64  `json:"planId" binding:"required"`
		Type      string `json:"type" binding:"required"`
		Domain    string `json:"domain"`
		PayMethod string `json:"payMethod"`
	}

	var req purchaseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	validTypes := map[string]bool{"domain": true, "wildcard": true, "ip": true, "key": true}
	if !validTypes[req.Type] {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "无效的授权类型"})
		return
	}
	validatedTarget, errMsg := validateLicenseTargetForSave(req.Type, req.Domain)
	if errMsg != "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": errMsg})
		return
	}
	req.Domain = validatedTarget
	req.PayMethod = strings.TrimSpace(req.PayMethod)
	if req.PayMethod == "" {
		req.PayMethod = "balance"
	}
	if req.PayMethod != "balance" {
		userPurchaseOnline(c, req.AppID, req.PlanID, req.Type, req.Domain, req.PayMethod, userID)
		return
	}

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
	if err := ensurePurchaseOrderPricingSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化购买订单价格快照失败"})
		return
	}
	if err := ensurePurchasePromotionSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化促销活动失败"})
		return
	}

	plan, err := loadPurchasePlanPricing(db, req.AppID, req.PlanID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "套餐不存在、已禁用或应用已下架"})
		return
	}
	if !planLicenseTypeMatches(plan.LicenseType, req.Type) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该套餐不适用于所选授权方式"})
		return
	}
	quote, err := quoteUserPurchase(db, plan)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "计算购买价格失败"})
		return
	}
	orderSnapshot, err := newPurchaseOrderSnapshot(plan, quote)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "生成订单价格快照失败"})
		return
	}
	appName, planName := plan.AppName, plan.PlanName
	durationDays := plan.DurationDays
	cost := purchaseAmount(quote.AmountCents)

	// 查用户余额
	var balance float64
	db.QueryRow("SELECT balance FROM users WHERE id = ?", userID).Scan(&balance)
	if balance < cost {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": fmt.Sprintf("余额不足，需要 ¥%.2f，当前余额 ¥%.2f", cost, balance)})
		return
	}

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "系统错误"})
		return
	}
	defer tx.Rollback()
	if err := requireAppPurchaseLicenseType(tx, req.AppID, req.Type); err != nil {
		if err == errPurchaseTypeNotAllowed {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": purchaseLicenseTypeNotAllowedMessage(req.Type)})
		} else {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "应用不存在或已下架"})
		}
		return
	}

	// 扣减余额
	newBalance := balance - cost
	_, err = tx.Exec("UPDATE users SET balance = ? WHERE id = ? AND balance >= ?", newBalance, userID, cost)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "扣款失败"})
		return
	}

	// 生成授权编号
	licenseNo := fmt.Sprintf("LIC%d%06d", time.Now().Unix(), time.Now().Nanosecond()%1000000)

	// 计算时间：duration_days = 0 表示永久，expired_at 为空
	now := time.Now()
	var expiredAt *time.Time
	if durationDays > 0 {
		t := now.AddDate(0, 0, durationDays)
		expiredAt = &t
	}

	// 生成密钥（key类型时使用）
	var licenseKey string
	if req.Type == "key" {
		licenseKey, err = generateRandomLicenseKey()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "生成密钥失败"})
			return
		}
	}

	// 插入授权
	result, err := tx.Exec(`
		INSERT INTO licenses (license_no, app_id, plan_id, original_price, type, status, source, owner_type, owner_id,
		                      duration_days, started_at, expired_at, license_key, max_domains, remark)
		VALUES (?, ?, ?, ?, ?, 'active', 'user_purchase', 'user', ?, ?, ?, ?, ?, 1, ?)`,
		licenseNo, req.AppID, req.PlanID, purchaseAmount(quote.OriginalCents), req.Type, userID,
		durationDays, now, expiredAt, licenseKey, fmt.Sprintf("用户自助购买 %s - %s", appName, planName))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建授权失败: " + err.Error()})
		return
	}

	licenseID, _ := result.LastInsertId()

	// 绑定域名/IP
	if req.Type != "key" && req.Domain != "" {
		isWildcard := 0
		if req.Type == "wildcard" {
			isWildcard = 1
		}
		_, err = tx.Exec("INSERT INTO license_domains (license_id, domain, is_wildcard) VALUES (?, ?, ?)",
			licenseID, req.Domain, isWildcard)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "绑定域名失败"})
			return
		}
	}

	// 记录流水
	txNo := fmt.Sprintf("TX%d%06d", time.Now().Unix(), time.Now().Nanosecond()%1000000)
	_, err = tx.Exec(`
		INSERT INTO transactions (tx_no, subject_type, subject_id, type, amount, balance_after, ref_type, ref_id, remark)
		VALUES (?, 'user', ?, 'consume', ?, ?, 'license', ?, ?)`,
		txNo, userID, -cost, newBalance, licenseID, fmt.Sprintf("购买 %s - %s 授权", appName, planName))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "记录流水失败"})
		return
	}

	// 写入订单表（余额支付直接标记为已支付）
	orderNo := generateUserPurchaseOrderNo()
	_, err = tx.Exec(`
		INSERT INTO license_purchase_orders (
			order_no, agent_id, user_id, app_id, plan_id, owner_type, owner_id,
			type, target, amount, original_amount, base_amount, discount_amount,
			promotion_id, promotion_name, promotion_rule_snapshot, pricing_snapshot,
			app_name_snapshot, plan_name_snapshot, duration_days_snapshot,
			pay_channel, pay_method, status, return_url, remark,
			license_id, license_no, paid_at
		) VALUES (?, 0, ?, ?, ?, 'user', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'balance', 'balance', 'paid', '', ?, ?, ?, NOW())`,
		orderNo, userID, req.AppID, req.PlanID, userID, req.Type, req.Domain,
		orderSnapshot.Amount, orderSnapshot.OriginalAmount, orderSnapshot.BaseAmount, orderSnapshot.DiscountAmount,
		orderSnapshot.PromotionID, orderSnapshot.PromotionName, orderSnapshot.PromotionRule, orderSnapshot.PricingSnapshot,
		orderSnapshot.AppName, orderSnapshot.PlanName, orderSnapshot.DurationDays,
		"用户余额支付购买授权", licenseID, licenseNo)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "写入订单记录失败"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "提交事务失败"})
		return
	}

	queuePurchaseSuccessMail("user", int64(userID), licenseID)

	c.JSON(http.StatusOK, gin.H{
		"code": 200, "msg": "购买成功",
		"data": gin.H{
			"licenseNo":    licenseNo,
			"licenseId":    licenseID,
			"appName":      appName,
			"planName":     planName,
			"durationDays": durationDays,
			"cost":         cost,
			"newBalance":   newBalance,
		},
	})
}

// UserDashboard 用户概览数据
func UserDashboard(c *gin.Context) {
	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")
	if role != "user" {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "无权限"})
		return
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	// 统计数据
	var total, active, expiring, expired int
	db.QueryRow("SELECT COUNT(*) FROM licenses WHERE owner_type='user' AND owner_id=?", userID).Scan(&total)
	db.QueryRow("SELECT COUNT(*) FROM licenses WHERE owner_type='user' AND owner_id=? AND status='active' AND (expired_at IS NULL OR expired_at > NOW() + INTERVAL 7 DAY)", userID).Scan(&active)
	db.QueryRow("SELECT COUNT(*) FROM licenses WHERE owner_type='user' AND owner_id=? AND status='active' AND expired_at IS NOT NULL AND expired_at <= NOW() + INTERVAL 7 DAY AND expired_at > NOW()", userID).Scan(&expiring)
	db.QueryRow("SELECT COUNT(*) FROM licenses WHERE owner_type='user' AND owner_id=? AND (status='expired' OR (expired_at IS NOT NULL AND expired_at <= NOW()))", userID).Scan(&expired)

	// 最近5条授权
	rows, err := db.Query(`
		SELECT l.id, l.app_id, a.app_name, l.status, l.expired_at, l.license_key,
		       GROUP_CONCAT(ld.domain SEPARATOR ', ') as domains
		FROM licenses l
		LEFT JOIN apps a ON a.id = l.app_id
		LEFT JOIN license_domains ld ON ld.license_id = l.id
		WHERE l.owner_type = 'user' AND l.owner_id = ?
		GROUP BY l.id
		ORDER BY l.created_at DESC
		LIMIT 5
	`, userID)

	type recentItem struct {
		ID          int64  `json:"id"`
		AppName     string `json:"appName"`
		Domain      string `json:"domain"`
		Status      string `json:"status"`
		StatusLabel string `json:"statusLabel"`
		ExpireAt    string `json:"expireAt"`
	}

	var recent []recentItem
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var item recentItem
			var appID int64
			var expiredAt sql.NullTime
			var licenseKey string
			var domains sql.NullString

			rows.Scan(&item.ID, &appID, &item.AppName, &item.Status, &expiredAt, &licenseKey, &domains)

			if domains.Valid && domains.String != "" {
				item.Domain = domains.String
			} else if licenseKey != "" {
				item.Domain = licenseKey
			}

			if expiredAt.Valid {
				item.ExpireAt = expiredAt.Time.Format("2006-01-02")
				if item.Status == "active" && expiredAt.Time.Before(time.Now().AddDate(0, 0, 7)) {
					item.Status = "expiring"
					item.StatusLabel = "即将到期"
				} else if item.Status == "active" {
					item.StatusLabel = "正常"
				}
			} else {
				item.ExpireAt = "永久"
				if item.Status == "active" {
					item.StatusLabel = "正常"
				}
			}
			if item.Status == "expired" {
				item.StatusLabel = "已过期"
			} else if item.Status == "revoked" {
				item.StatusLabel = "已吊销"
			}

			recent = append(recent, item)
		}
	}
	if recent == nil {
		recent = []recentItem{}
	}

	// 获取用户昵称
	var nickname string
	db.QueryRow("SELECT nickname FROM users WHERE id = ?", userID).Scan(&nickname)

	c.JSON(http.StatusOK, gin.H{
		"code": 200, "msg": "",
		"data": gin.H{
			"nickname": nickname,
			"stats": gin.H{
				"total":    total,
				"active":   active,
				"expiring": expiring,
				"expired":  expired,
			},
			"recentLicenses": recent,
		},
	})
}

// ========== 用户个人设置 ==========

// UserProfile 获取用户个人信息
func UserProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	if err := ensureUserAuthStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化失败"})
		return
	}

	var email, nickname string
	var phone sql.NullString
	var balance float64
	var createdAt sql.NullTime
	err = db.QueryRow("SELECT email, nickname, balance, created_at, phone FROM users WHERE id = ?", userID).Scan(&email, &nickname, &balance, &createdAt, &phone)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "用户不存在"})
		return
	}

	// 实名信息（列可能不存在，容错处理）
	realName, realIDCard := "", ""
	var realnameAt sql.NullTime
	if err := ensureRealnameStorage(db); err == nil {
		_ = db.QueryRow("SELECT real_name, real_id_card, realname_at FROM users WHERE id = ?", userID).Scan(&realName, &realIDCard, &realnameAt)
	}
	realnameVerified := realnameAt.Valid
	realnameAtStr := ""
	if realnameAt.Valid {
		realnameAtStr = realnameAt.Time.Format("2006-01-02 15:04:05")
	}
	realnameEnabled := false
	realnameProvider := ""
	realnameAuthMode := ""
	if rnCfg, err := loadRealnameConfig(db); err == nil {
		realnameEnabled = rnCfg.Enabled
		realnameProvider = rnCfg.Provider
		if rnCfg.Provider == realnameProviderXiaomu {
			realnameAuthMode = rnCfg.XiaomuProductMode
		}
	}

	var createdAtStr string
	if createdAt.Valid {
		createdAtStr = createdAt.Time.Format("2006-01-02 15:04:05")
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200, "msg": "",
		"data": gin.H{
			"userId":           userID,
			"email":            email,
			"nickname":         nickname,
			"phone":            phone.String,
			"balance":          balance,
			"createdAt":        createdAtStr,
			"realnameEnabled":  realnameEnabled,
			"realnameProvider": realnameProvider,
			"realnameAuthMode": realnameAuthMode,
			"realnameVerified": realnameVerified,
			"realName":         maskRealName(realName),
			"realIdCard":       maskIDCard(realIDCard),
			"realnameAt":       realnameAtStr,
		},
	})
}

// UserUpdateProfile 更新用户个人信息
func UserUpdateProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")

	type updateReq struct {
		Nickname string  `json:"nickname"`
		Email    string  `json:"email"`
		Phone    *string `json:"phone"` // 指针区分未传（不修改）与空串（清除绑定）
	}
	var req updateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	if req.Nickname == "" && req.Email == "" && req.Phone == nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请至少修改一项"})
		return
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	if err := ensureUserAuthStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化失败"})
		return
	}

	if req.Email != "" {
		var exists int
		db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ? AND id != ?", req.Email, userID).Scan(&exists)
		if exists > 0 {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该邮箱已被其他用户使用"})
			return
		}
	}

	newPhone := ""
	if req.Phone != nil {
		newPhone = strings.TrimSpace(*req.Phone)
		if newPhone != "" && !phoneRegexp.MatchString(newPhone) {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "手机号格式不正确"})
			return
		}
		if newPhone != "" {
			var exists int
			db.QueryRow("SELECT COUNT(*) FROM users WHERE phone = ? AND id != ?", newPhone, userID).Scan(&exists)
			if exists > 0 {
				c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该手机号已被其他用户使用"})
				return
			}
		}
	}

	sets := ""
	args := []interface{}{}
	if req.Nickname != "" {
		sets += "nickname = ?"
		args = append(args, req.Nickname)
	}
	if req.Email != "" {
		if sets != "" {
			sets += ", "
		}
		sets += "email = ?"
		args = append(args, req.Email)
	}
	if req.Phone != nil {
		if sets != "" {
			sets += ", "
		}
		sets += "phone = ?"
		if newPhone == "" {
			args = append(args, nil)
		} else {
			args = append(args, newPhone)
		}
	}
	args = append(args, userID)

	_, err = db.Exec("UPDATE users SET "+sets+" WHERE id = ?", args...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "更新成功"})
}

// UserChangePassword 用户修改密码
func UserChangePassword(c *gin.Context) {
	userID, _ := c.Get("user_id")

	type pwdReq struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required,min=6"`
	}
	var req pwdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误，新密码至少6位"})
		return
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var passwordHash string
	err = db.QueryRow("SELECT password_hash FROM users WHERE id = ?", userID).Scan(&passwordHash)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "用户不存在"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "旧密码错误"})
		return
	}

	newHash, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	_, err = db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(newHash), userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "修改失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "密码修改成功"})
}
