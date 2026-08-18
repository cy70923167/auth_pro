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

type agentPanelLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AgentPanelLogin 代理端登录
func AgentPanelLogin(c *gin.Context) {
	var req agentPanelLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	if remaining := middleware.LoginLockRemaining(c.ClientIP(), req.Username); remaining > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 429, "msg": fmt.Sprintf("登录尝试次数过多，请 %d 秒后重试", int(remaining.Seconds())+1)})
		return
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var id uint
	var passwordHash, email, name string
	var balance float64
	err = db.QueryRow(`
		SELECT id, email, password_hash, name, balance
		FROM agents
		WHERE (email = ? OR contact = ?) AND enabled = 1
	`, req.Username, req.Username).Scan(&id, &email, &passwordHash, &name, &balance)
	if err != nil {
		middleware.RecordLoginFailure(c.ClientIP(), req.Username)
		c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "账号或密码错误"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		middleware.RecordLoginFailure(c.ClientIP(), req.Username)
		c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "账号或密码错误"})
		return
	}

	middleware.RecordLoginSuccess(c.ClientIP(), req.Username)

	_, _ = db.Exec("UPDATE agents SET last_login_at = NOW(), last_login_ip = ? WHERE id = ?", c.ClientIP(), id)

	now := time.Now()
	claims := &middleware.Claims{
		UserID:   id,
		Username: email,
		Role:     "agent",
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
		"code": 200,
		"msg":  "登录成功",
		"data": gin.H{
			"accessToken": token,
			"agentId":     id,
			"email":       email,
			"name":        name,
			"balance":     balance,
		},
	})
}

// AgentPanelPurchaseApps 代理购买页应用列表（含启用套餐）
func AgentPanelPurchaseApps(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
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
	if err := ensureAgentPurchaseSchemas(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	if err := ensurePurchasePromotionSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化促销活动失败"})
		return
	}

	discount, err := currentAgentDiscount(db, agentID)
	if err != nil || discount <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "代理商不存在或已禁用"})
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
		Discount       float64                `json:"discount"`
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
				var licenseType sql.NullString
				planRows.Scan(&plan.ID, &plan.Name, &licenseType, &plan.DurationDays, &plan.OriginalPrice)
				plan.LicenseType = strings.ToLower(strings.TrimSpace(licenseType.String))
				plan.Discount = discount
				if quote, quoteErr := quoteAgentPurchase(db, purchasePlanPricing{
					AppID:      item.ID,
					PlanID:     plan.ID,
					PriceCents: floatAmountToCents(plan.OriginalPrice),
				}, discount); quoteErr == nil {
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
					plan.BasePrice = plan.OriginalPrice * discount / 10
					plan.Price = plan.BasePrice
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

// AgentPanelUserOptions 返回代理开通授权时可选择的启用用户。
func AgentPanelUserOptions(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}

	db, err := openAppPurchaseDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var enabled int
	if err := db.QueryRow("SELECT enabled FROM agents WHERE id = ?", agentID).Scan(&enabled); err != nil || enabled == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "代理商不存在或已禁用"})
		return
	}

	keyword := strings.TrimSpace(c.Query("keyword"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 50 {
		limit = 20
	}

	query := `
		SELECT id, COALESCE(NULLIF(nickname, ''), email), email
		FROM users
		WHERE enabled = 1
	`
	args := []interface{}{}
	if keyword != "" {
		pattern := "%" + strings.ToLower(keyword) + "%"
		query += " AND LOWER(email) LIKE ?"
		args = append(args, pattern)
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询用户失败"})
		return
	}
	defer rows.Close()

	type userOption struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	list := make([]userOption, 0)
	for rows.Next() {
		var item userOption
		if err := rows.Scan(&item.ID, &item.Name, &item.Email); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取用户失败"})
			return
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询用户失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": list})
}

// AgentPanelBalance 获取代理余额
func AgentPanelBalance(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var balance float64
	err = db.QueryRow("SELECT balance FROM agents WHERE id = ? AND enabled = 1", agentID).Scan(&balance)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "代理商不存在或已禁用"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{"balance": balance}})
}

// AgentPanelAppList 代理端授权筛选应用列表
func AgentPanelAppList(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT DISTINCT a.id, a.app_name
		FROM apps a
		JOIN licenses l ON l.app_id = a.id
		WHERE a.enabled = 1
		  AND l.owner_type = 'agent'
		  AND l.owner_id = ?
		ORDER BY a.id ASC
	`, agentID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	defer rows.Close()

	type appItem struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	list := []appItem{}
	for rows.Next() {
		var item appItem
		if err := rows.Scan(&item.ID, &item.Name); err == nil {
			list = append(list, item)
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": list})
}

// AgentPanelLicenseList 代理端我的授权列表，只返回当前代理商名下授权
func AgentPanelLicenseList(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	keyword := strings.TrimSpace(c.Query("keyword"))
	appID := strings.TrimSpace(c.Query("appId"))
	status := strings.TrimSpace(c.Query("status"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	where := "WHERE l.owner_type = 'agent' AND l.owner_id = ?"
	args := []interface{}{agentID}
	if keyword != "" {
		where += " AND (ld.domain LIKE ? OR l.license_key LIKE ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if appID != "" {
		where += " AND l.app_id = ?"
		args = append(args, appID)
	}
	if status == "active" {
		where += " AND l.status = 'active' AND (l.expired_at IS NULL OR l.expired_at > NOW() + INTERVAL 7 DAY)"
	} else if status == "expiring" {
		where += " AND l.status = 'active' AND l.expired_at IS NOT NULL AND l.expired_at <= NOW() + INTERVAL 7 DAY AND l.expired_at > NOW()"
	} else if status == "expired" {
		where += " AND (l.status = 'expired' OR (l.expired_at IS NOT NULL AND l.expired_at <= NOW()))"
	}

	countSQL := fmt.Sprintf("SELECT COUNT(DISTINCT l.id) FROM licenses l LEFT JOIN license_domains ld ON ld.license_id = l.id %s", where)
	var total int
	db.QueryRow(countSQL, args...).Scan(&total)

	querySQL := fmt.Sprintf(`
		SELECT l.id, l.license_no, l.app_id, a.app_name, l.type, l.status,
		       l.source, l.expired_at, l.created_at, l.license_key,
		       GROUP_CONCAT(ld.domain SEPARATOR ', ') as domains
		FROM licenses l
		LEFT JOIN apps a ON a.id = l.app_id
		LEFT JOIN license_domains ld ON ld.license_id = l.id
		%s
		GROUP BY l.id
		ORDER BY l.created_at DESC
		LIMIT ? OFFSET ?
	`, where)
	queryArgs := append(args, pageSize, offset)
	rows, err := db.Query(querySQL, queryArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	typeLabels := map[string]string{"domain": "单域名", "wildcard": "泛域名", "ip": "IP", "key": "密钥"}
	sourceLabels := map[string]string{"admin": "管理员开通", "agent": "代理商开通", "user_purchase": "自助购买", "card": "卡密兑换"}
	type licenseItem struct {
		ID             int64  `json:"id"`
		LicenseNo      string `json:"licenseNo"`
		AppID          int64  `json:"appId"`
		AppName        string `json:"appName"`
		Type           string `json:"type"`
		TypeLabel      string `json:"typeLabel"`
		Status         string `json:"status"`
		StatusLabel    string `json:"statusLabel"`
		Source         string `json:"source"`
		Domain         string `json:"domain"`
		BindingPending bool   `json:"bindingPending"`
		ExpireAt       string `json:"expireAt"`
		CreatedAt      string `json:"createdAt"`
	}

	list := []licenseItem{}
	for rows.Next() {
		var item licenseItem
		var expiredAt sql.NullTime
		var createdAt sql.NullTime
		var licenseKey, source string
		var domains sql.NullString
		if err := rows.Scan(&item.ID, &item.LicenseNo, &item.AppID, &item.AppName, &item.Type, &item.Status, &source, &expiredAt, &createdAt, &licenseKey, &domains); err != nil {
			continue
		}
		item.TypeLabel = typeLabels[item.Type]
		item.Source = sourceLabels[source]
		if item.Source == "" {
			item.Source = source
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
		if item.Status == "active" {
			if expiredAt.Valid && expiredAt.Time.Before(time.Now().AddDate(0, 0, 7)) {
				item.Status = "expiring"
				item.StatusLabel = "即将到期"
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

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{"list": list, "total": total, "page": page, "pageSize": pageSize}})
}

// AgentPanelLicenseUpdate lets an agent change the target without changing the
// license type. Key licenses can only be rotated through the refresh endpoint.
func AgentPanelLicenseUpdate(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}

	licenseID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || licenseID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "授权ID不正确"})
		return
	}

	var req struct {
		Type   string `json:"type"`
		Target string `json:"target" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请填写授权目标"})
		return
	}
	req.Type = strings.TrimSpace(req.Type)
	req.Target = strings.TrimSpace(req.Target)
	if req.Target == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "授权目标不能为空"})
		return
	}
	if len(req.Target) > 255 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "授权目标不能超过255个字符"})
		return
	}

	db := agentPanelDB(c)
	if db == nil {
		return
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "系统错误"})
		return
	}
	defer tx.Rollback()

	var existingID uint64
	var licenseType string
	err = tx.QueryRow(`
		SELECT id, type FROM licenses
		WHERE id = ? AND owner_type = 'agent' AND owner_id = ?
		FOR UPDATE
	`, licenseID, agentID).Scan(&existingID, &licenseType)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "授权不存在或不属于当前代理商"})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询授权失败"})
		return
	}

	if req.Type != "" && req.Type != licenseType {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "授权类型不允许修改"})
		return
	}
	if licenseType == "key" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "密钥授权请使用刷新密钥功能"})
		return
	}

	validatedTarget, errMsg := validateLicenseTargetForSave(licenseType, req.Target)
	if errMsg != "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": errMsg})
		return
	}

	if _, err = tx.Exec("DELETE FROM license_domains WHERE license_id = ?", licenseID); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "清理旧授权目标失败"})
		return
	}
	isWildcard := 0
	if licenseType == "wildcard" {
		isWildcard = 1
	}
	if _, err = tx.Exec(`
		INSERT INTO license_domains (license_id, domain, is_wildcard)
		VALUES (?, ?, ?)
	`, licenseID, validatedTarget, isWildcard); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存授权目标失败"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "提交更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "授权已更新",
		"data": gin.H{"id": existingID, "type": licenseType, "target": validatedTarget},
	})
}

// AgentPanelLicenseRefreshKey rotates a key license to a new random 16-character key.
func AgentPanelLicenseRefreshKey(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}
	licenseID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || licenseID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "授权ID不正确"})
		return
	}

	db := agentPanelDB(c)
	if db == nil {
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
		WHERE id = ? AND owner_type = 'agent' AND owner_id = ?
		FOR UPDATE
	`, licenseID, agentID).Scan(&licenseType)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "授权不存在或不属于当前代理商"})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询授权失败"})
		return
	}
	if licenseType != "key" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "只有密钥类型授权可以刷新密钥"})
		return
	}

	licenseKey, err := generateRandomLicenseKey()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "生成密钥失败"})
		return
	}
	result, err := tx.Exec(`
		UPDATE licenses SET license_key = ?
		WHERE id = ? AND owner_type = 'agent' AND owner_id = ? AND type = 'key'
	`, licenseKey, licenseID, agentID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "刷新密钥失败"})
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

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "密钥已刷新", "data": gin.H{"id": licenseID, "licenseKey": licenseKey}})
}

// agentPanelDB 打开数据库连接，失败时直接响应 500。
func agentPanelDB(c *gin.Context) *sql.DB {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return nil
	}
	return db
}

// AgentPanelStats 概览-统计卡片（余额/有效授权/剩余配额/即将到期/较上周）
func AgentPanelStats(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}
	db := agentPanelDB(c)
	if db == nil {
		return
	}
	defer db.Close()

	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)

	var balance float64
	_ = db.QueryRow("SELECT balance FROM agents WHERE id = ?", agentID).Scan(&balance)

	var activeLicenses int
	_ = db.QueryRow(`
		SELECT COUNT(*) FROM licenses
		WHERE owner_type = 'agent' AND owner_id = ?
		  AND status = 'active' AND (expired_at IS NULL OR expired_at > NOW())
	`, agentID).Scan(&activeLicenses)

	var quotaRemain int
	_ = db.QueryRow(`
		SELECT COALESCE(SUM(GREATEST(total - used, 0)), 0) FROM agent_quotas WHERE agent_id = ?
	`, agentID).Scan(&quotaRemain)

	var expiringSoon int
	_ = db.QueryRow(`
		SELECT COUNT(*) FROM licenses
		WHERE owner_type = 'agent' AND owner_id = ?
		  AND status = 'active' AND expired_at IS NOT NULL
		  AND expired_at > NOW() AND expired_at <= NOW() + INTERVAL 7 DAY
	`, agentID).Scan(&expiringSoon)

	var thisWeek, lastWeek int
	_ = db.QueryRow(`
		SELECT COUNT(*) FROM licenses WHERE owner_type = 'agent' AND owner_id = ? AND created_at >= ?
	`, agentID, weekAgo).Scan(&thisWeek)
	_ = db.QueryRow(`
		SELECT COUNT(*) FROM licenses WHERE owner_type = 'agent' AND owner_id = ? AND created_at >= ? AND created_at < ?
	`, agentID, weekAgo.AddDate(0, 0, -7), weekAgo).Scan(&lastWeek)
	change := "+0%"
	if lastWeek > 0 {
		change = fmt.Sprintf("%+.0f%%", float64(thisWeek-lastWeek)/float64(lastWeek)*100)
	} else if thisWeek > 0 {
		change = "+100%"
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200, "msg": "",
		"data": gin.H{
			"balance":        balance,
			"activeLicenses": activeLicenses,
			"quotaRemain":    quotaRemain,
			"expiringSoon":   expiringSoon,
			"weekChange":     change,
		},
	})
}

// AgentPanelInfo 概览-我的信息（名称/等级/折扣/联系方式/注册时间）
func AgentPanelInfo(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}
	db := agentPanelDB(c)
	if db == nil {
		return
	}
	defer db.Close()

	var name, level, levelName, contact string
	var discount float64
	var createdAt sql.NullTime
	err := db.QueryRow(`
		SELECT a.name, a.level, COALESCE(NULLIF(l.name, ''), a.level), a.discount, a.contact, a.created_at
		FROM agents a
		LEFT JOIN agent_levels l ON l.code = a.level
		WHERE a.id = ?
	`, agentID).Scan(&name, &level, &levelName, &discount, &contact, &createdAt)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "代理商不存在"})
		return
	}

	created := ""
	if createdAt.Valid {
		created = createdAt.Time.Format("2006-01-02")
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200, "msg": "",
		"data": gin.H{
			"name":      name,
			"level":     level,
			"levelName": levelName,
			"discount":  fmt.Sprintf("%g折", discount),
			"contact":   contact,
			"createdAt": created,
		},
	})
}

// AgentPanelTrend 概览-授权趋势（近12个月每月新增，含0月份补齐）
func AgentPanelTrend(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}
	db := agentPanelDB(c)
	if db == nil {
		return
	}
	defer db.Close()

	type monthCount struct {
		Month string `json:"month"`
		Count int    `json:"count"`
	}
	trend := []monthCount{}
	rows, err := db.Query(`
		SELECT DATE_FORMAT(created_at, '%Y-%m') AS ym, COUNT(*)
		FROM licenses
		WHERE owner_type = 'agent' AND owner_id = ?
		  AND created_at >= DATE_FORMAT(DATE_SUB(CURDATE(), INTERVAL 11 MONTH), '%Y-%m-01')
		GROUP BY ym
		ORDER BY ym
	`, agentID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	defer rows.Close()

	m := map[string]int{}
	for rows.Next() {
		var ym string
		var cnt int
		if rows.Scan(&ym, &cnt) == nil {
			m[ym] = cnt
		}
	}
	now := time.Now()
	cursor := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, -11, 0)
	for i := 0; i < 12; i++ {
		ym := cursor.Format("2006-01")
		trend = append(trend, monthCount{Month: ym, Count: m[ym]})
		cursor = cursor.AddDate(0, 1, 0)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": trend})
}

// AgentPanelAppDist 概览-应用分布（按授权数量，前8个应用）
func AgentPanelAppDist(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}
	db := agentPanelDB(c)
	if db == nil {
		return
	}
	defer db.Close()

	type appDist struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	dist := []appDist{}
	rows, err := db.Query(`
		SELECT a.app_name, COUNT(*) AS cnt
		FROM licenses l
		JOIN apps a ON a.id = l.app_id
		WHERE l.owner_type = 'agent' AND l.owner_id = ?
		GROUP BY l.app_id, a.app_name
		ORDER BY cnt DESC
		LIMIT 8
	`, agentID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	defer rows.Close()
	for rows.Next() {
		var d appDist
		if rows.Scan(&d.Name, &d.Count) == nil {
			dist = append(dist, d)
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": dist})
}

// AgentPanelRecentLicenses 概览-最近开码（最近5条）
func AgentPanelRecentLicenses(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}
	db := agentPanelDB(c)
	if db == nil {
		return
	}
	defer db.Close()

	type recentItem struct {
		Domain    string `json:"domain"`
		AppName   string `json:"appName"`
		TypeLabel string `json:"typeLabel"`
		ExpireAt  string `json:"expireAt"`
		CreatedAt string `json:"createdAt"`
	}
	recent := []recentItem{}
	typeLabels := map[string]string{"domain": "单域名", "wildcard": "泛域名", "ip": "IP", "key": "密钥"}
	rows, err := db.Query(`
		SELECT l.type, a.app_name, l.expired_at, l.created_at, l.license_key,
		       (SELECT GROUP_CONCAT(ld.domain SEPARATOR ', ') FROM license_domains ld WHERE ld.license_id = l.id) AS domains
		FROM licenses l
		LEFT JOIN apps a ON a.id = l.app_id
		WHERE l.owner_type = 'agent' AND l.owner_id = ?
		ORDER BY l.created_at DESC
		LIMIT 5
	`, agentID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	defer rows.Close()
	for rows.Next() {
		var item recentItem
		var licType, appName, licenseKey string
		var expiredAt, createdAt sql.NullTime
		var domains sql.NullString
		if rows.Scan(&licType, &appName, &expiredAt, &createdAt, &licenseKey, &domains) != nil {
			continue
		}
		item.AppName = appName
		item.TypeLabel = typeLabels[licType]
		if item.TypeLabel == "" {
			item.TypeLabel = licType
		}
		if domains.Valid && domains.String != "" {
			item.Domain = domains.String
		} else {
			item.Domain = licenseKey
		}
		if expiredAt.Valid {
			item.ExpireAt = expiredAt.Time.Format("2006-01-02")
		} else {
			item.ExpireAt = "永久"
		}
		if createdAt.Valid {
			item.CreatedAt = createdAt.Time.Format("2006-01-02")
		}
		recent = append(recent, item)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": recent})
}

// AgentPanelPurchase 代理端购买授权
func AgentPanelPurchase(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}

	type purchaseReq struct {
		AppID     int64  `json:"appId" binding:"required"`
		PlanID    int64  `json:"planId" binding:"required"`
		UserID    int64  `json:"userId"`
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

	// 线上支付（支付宝/微信/QQ 等）单独走支付下单流程，不在这里结算。
	if req.PayMethod != "" && req.PayMethod != "balance" && req.PayMethod != "quota" {
		if _, enabled := normalizeEpayPayType(req.PayMethod, ""); !enabled {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "不支持的支付方式"})
			return
		}
		agentPanelPurchaseOnline(c, req.AppID, req.PlanID, req.UserID, req.Type, req.Domain, req.PayMethod, agentID)
		return
	}

	payMethod := req.PayMethod
	if payMethod == "" {
		payMethod = "balance"
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
	if err := ensureAgentPurchaseSchemas(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": err.Error()})
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

	discount, err := currentAgentDiscount(db, agentID)
	if err != nil || discount <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "代理商不存在或已禁用"})
		return
	}
	quote, err := quoteAgentPurchase(db, plan, discount)
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
	originalPrice := purchaseAmount(quote.OriginalCents)
	durationDays := plan.DurationDays
	cost := purchaseAmount(quote.AmountCents)

	ownerType := "agent"
	ownerID := int64(agentID)
	if req.UserID > 0 {
		var exists int
		err = db.QueryRow("SELECT COUNT(*) FROM users WHERE id = ? AND enabled = 1", req.UserID).Scan(&exists)
		if err != nil || exists == 0 {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "目标用户不存在或已禁用"})
			return
		}
		ownerType = "user"
		ownerID = req.UserID
	}

	var balance float64
	err = db.QueryRow("SELECT balance FROM agents WHERE id = ? AND enabled = 1", agentID).Scan(&balance)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "代理商不存在或已禁用"})
		return
	}

	// 0 元套餐不走配额/余额扣减，直接免费开通。
	freeOrder := cost <= 0

	if payMethod == "quota" && !freeOrder {
		var quotaRemain int
		err = db.QueryRow(`
			SELECT GREATEST(total - used, 0) FROM agent_quotas WHERE agent_id = ? AND app_id = ?
		`, agentID, req.AppID).Scan(&quotaRemain)
		if err == sql.ErrNoRows || (err == nil && quotaRemain <= 0) {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "当前应用没有可用配额，请选择其他支付方式"})
			return
		}
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询配额失败"})
			return
		}
	} else if !freeOrder && payMethod == "balance" && balance < cost {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": fmt.Sprintf("余额不足，需要 ¥%.2f，当前余额 ¥%.2f", cost, balance)})
		return
	}

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

	newBalance := balance
	var chargeAmount float64
	var txRemark string
	switch {
	case freeOrder:
		txRemark = fmt.Sprintf("免费开通 %s - %s 授权", appName, planName)
	case payMethod == "quota":
		result, err := tx.Exec(`
			UPDATE agent_quotas SET used = used + 1
			WHERE agent_id = ? AND app_id = ? AND used < total
		`, agentID, req.AppID)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "扣减配额失败"})
			return
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "配额不足"})
			return
		}
		txRemark = fmt.Sprintf("配额支付开通 %s - %s 授权", appName, planName)
	default:
		newBalance = balance - cost
		result, err := tx.Exec("UPDATE agents SET balance = ? WHERE id = ? AND balance >= ?", newBalance, agentID, cost)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "扣款失败"})
			return
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "余额不足"})
			return
		}
		chargeAmount = -cost
		txRemark = fmt.Sprintf("开通 %s - %s 授权", appName, planName)
	}

	licenseNo := fmt.Sprintf("LIC%d%06d", time.Now().Unix(), time.Now().Nanosecond()%1000000)
	now := time.Now()
	var expiredAt *time.Time
	if durationDays > 0 {
		t := now.AddDate(0, 0, durationDays)
		expiredAt = &t
	}

	var licenseKey string
	if req.Type == "key" {
		licenseKey, err = generateRandomLicenseKey()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "生成密钥失败"})
			return
		}
	}

	licenseResult, err := tx.Exec(`
		INSERT INTO licenses (license_no, app_id, plan_id, original_price, type, status, source, owner_type, owner_id, issued_by,
		                      duration_days, started_at, expired_at, license_key, max_domains, remark)
		VALUES (?, ?, ?, ?, ?, 'active', 'agent', ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
		licenseNo, req.AppID, req.PlanID, originalPrice, req.Type, ownerType, ownerID, agentID,
		durationDays, now, expiredAt, licenseKey, fmt.Sprintf("代理商开通 %s - %s", appName, planName))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建授权失败: " + err.Error()})
		return
	}

	licenseID, _ := licenseResult.LastInsertId()
	if req.Type != "key" && req.Domain != "" {
		isWildcard := 0
		if req.Type == "wildcard" {
			isWildcard = 1
		}
		_, err = tx.Exec("INSERT INTO license_domains (license_id, domain, is_wildcard) VALUES (?, ?, ?)", licenseID, req.Domain, isWildcard)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "绑定授权目标失败"})
			return
		}
	}

	txNo := fmt.Sprintf("TX%d%06d", time.Now().Unix(), time.Now().Nanosecond()%1000000)
	_, err = tx.Exec(`
		INSERT INTO transactions (tx_no, subject_type, subject_id, type, amount, balance_after, ref_type, ref_id, remark)
		VALUES (?, 'agent', ?, 'consume', ?, ?, 'license', ?, ?)`,
		txNo, agentID, chargeAmount, newBalance, licenseID, txRemark)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "记录流水失败"})
		return
	}

	// 写入订单表（余额/配额支付直接标记为已支付）
	orderNo := generateAgentPurchaseOrderNo()
	orderRemark := "代理商余额支付开通授权"
	if payMethod == "quota" {
		orderRemark = "代理商配额支付开通授权"
	} else if freeOrder {
		orderRemark = "代理商免费开通授权"
	}
	_, err = tx.Exec(`
		INSERT INTO license_purchase_orders (
			order_no, agent_id, user_id, app_id, plan_id, owner_type, owner_id,
			type, target, amount, original_amount, base_amount, discount_amount,
			promotion_id, promotion_name, promotion_rule_snapshot, pricing_snapshot,
			app_name_snapshot, plan_name_snapshot, duration_days_snapshot,
			pay_channel, pay_method, status, return_url, remark,
			license_id, license_no, paid_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'paid', '', ?, ?, ?, NOW())`,
		orderNo, agentID, req.UserID, req.AppID, req.PlanID, ownerType, ownerID, req.Type, req.Domain,
		orderSnapshot.Amount, orderSnapshot.OriginalAmount, orderSnapshot.BaseAmount, orderSnapshot.DiscountAmount,
		orderSnapshot.PromotionID, orderSnapshot.PromotionName, orderSnapshot.PromotionRule, orderSnapshot.PricingSnapshot,
		orderSnapshot.AppName, orderSnapshot.PlanName, orderSnapshot.DurationDays,
		payMethod, payMethod, orderRemark, licenseID, licenseNo)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "写入订单记录失败"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "提交事务失败"})
		return
	}

	queuePurchaseSuccessMail(ownerType, ownerID, licenseID)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "开通成功",
		"data": gin.H{
			"licenseNo":    licenseNo,
			"licenseId":    licenseID,
			"appName":      appName,
			"planName":     planName,
			"durationDays": durationDays,
			"cost":         cost,
			"payMethod":    payMethod,
			"newBalance":   newBalance,
			"ownerType":    ownerType,
			"ownerId":      ownerID,
		},
	})
}

func requireAgentRole(c *gin.Context) bool {
	role, _ := c.Get("role")
	if role != "agent" {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "无权限"})
		return false
	}
	return true
}

func getAgentID(c *gin.Context) (uint, bool) {
	if !requireAgentRole(c) {
		return 0, false
	}
	agentID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "认证信息缺失"})
		return 0, false
	}
	id, ok := agentID.(uint)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "认证信息异常"})
		return 0, false
	}
	return id, true
}
