package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// AgentPanelFinanceOverview 我的财务-顶部余额概览
func AgentPanelFinanceOverview(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}

	db := agentPanelDB(c)
	if db == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var balance float64
	err := db.QueryRow("SELECT balance FROM agents WHERE id = ? AND enabled = 1", agentID).Scan(&balance)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "代理商不存在或已禁用"})
		return
	}

	// 累计充值/消费（负数转正展示）
	var totalRecharge, totalConsume float64
	_ = db.QueryRow(`
		SELECT COALESCE(SUM(amount), 0) FROM transactions
		WHERE subject_type = 'agent' AND subject_id = ? AND type = 'recharge'
	`, agentID).Scan(&totalRecharge)
	_ = db.QueryRow(`
		SELECT COALESCE(ABS(SUM(amount)), 0) FROM transactions
		WHERE subject_type = 'agent' AND subject_id = ? AND type IN ('consume', 'purchase') AND amount < 0
	`, agentID).Scan(&totalConsume)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"balance":       balance,
			"totalRecharge": totalRecharge,
			"totalConsume":  totalConsume,
		},
	})
}

// AgentPanelFinanceQuotas 我的财务-各应用配额使用情况
func AgentPanelFinanceQuotas(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}

	db := agentPanelDB(c)
	if db == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureAgentQuotaSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "配额表初始化失败"})
		return
	}

	rows, err := db.Query(`
		SELECT q.app_id, COALESCE(a.app_name, '已删除应用'), q.total, q.used, q.price
		FROM agent_quotas q
		LEFT JOIN apps a ON a.id = q.app_id
		WHERE q.agent_id = ?
		ORDER BY q.app_id ASC
	`, agentID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询配额失败"})
		return
	}
	defer rows.Close()

	type quotaItem struct {
		AppID  int64   `json:"appId"`
		Name   string  `json:"appName"`
		Total  int     `json:"total"`
		Used   int     `json:"used"`
		Remain int     `json:"remain"`
		Price  float64 `json:"price"`
	}
	list := []quotaItem{}
	for rows.Next() {
		var item quotaItem
		if err := rows.Scan(&item.AppID, &item.Name, &item.Total, &item.Used, &item.Price); err != nil {
			continue
		}
		item.Remain = item.Total - item.Used
		if item.Remain < 0 {
			item.Remain = 0
		}
		list = append(list, item)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": list})
}

// AgentPanelFinanceTransactions 我的财务-收支明细（分页+筛选）
func AgentPanelFinanceTransactions(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}

	db := agentPanelDB(c)
	if db == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	txType := strings.TrimSpace(c.Query("type"))
	startDate := strings.TrimSpace(c.Query("startDate"))
	endDate := strings.TrimSpace(c.Query("endDate"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	where := []string{"t.subject_type = 'agent'", "t.subject_id = ?"}
	args := []any{agentID}

	// 前端 type=consume 同时匹配 consume 和 purchase（购买授权）
	switch txType {
	case "consume", "purchase":
		where = append(where, "t.type IN ('consume', 'purchase')")
	case "recharge", "refund", "transfer", "bonus":
		where = append(where, "t.type = ?")
		args = append(args, txType)
	}
	if startDate != "" {
		// created_at is stored in UTC; date filters are entered as Beijing dates.
		where = append(where, "t.created_at >= DATE_SUB(?, INTERVAL 8 HOUR)")
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where = append(where, "t.created_at <= DATE_SUB(?, INTERVAL 8 HOUR)")
		args = append(args, endDate+" 23:59:59")
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	if err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM transactions t WHERE %s", whereSQL), args...).Scan(&total); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败"})
		return
	}

	offset := (page - 1) * pageSize
	listSQL := fmt.Sprintf(`
		SELECT t.id, t.tx_no, t.type, t.amount, t.balance_after, t.remark,
		       DATE_FORMAT(DATE_ADD(t.created_at, INTERVAL 8 HOUR), '%%Y-%%m-%%d %%H:%%i:%%s') AS created_at_beijing
		FROM transactions t
		WHERE %s
		ORDER BY t.created_at DESC, t.id DESC
		LIMIT ? OFFSET ?
	`, whereSQL)
	rows, err := db.Query(listSQL, append(args, pageSize, offset)...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	typeLabels := map[string]string{
		"recharge": "充值", "consume": "消费", "purchase": "消费", "refund": "退款",
		"transfer": "账户迁移", "bonus": "开通赠送",
	}
	type txItem struct {
		ID           int64   `json:"id"`
		OrderNo      string  `json:"orderNo"`
		Type         string  `json:"type"`
		TypeLabel    string  `json:"typeLabel"`
		Amount       float64 `json:"amount"`
		BalanceAfter float64 `json:"balanceAfter"`
		Remark       string  `json:"remark"`
		CreatedAt    string  `json:"createdAt"`
	}

	list := []txItem{}
	for rows.Next() {
		var item txItem
		var balanceAfter sql.NullFloat64
		var remark sql.NullString
		if err := rows.Scan(&item.ID, &item.OrderNo, &item.Type, &item.Amount, &balanceAfter, &remark, &item.CreatedAt); err != nil {
			continue
		}
		item.TypeLabel = typeLabels[item.Type]
		if item.TypeLabel == "" {
			item.TypeLabel = item.Type
		}
		if balanceAfter.Valid {
			item.BalanceAfter = balanceAfter.Float64
		}
		if remark.Valid {
			item.Remark = remark.String
		}
		list = append(list, item)
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

// AgentPanelRechargeOptions 我的财务-充值可用支付方式
func AgentPanelRechargeOptions(c *gin.Context) {
	if _, ok := getAgentID(c); !ok {
		return
	}

	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	payTypes := []string{}
	defaultType := ""
	if cfg, err := loadEpayConfig(db); err == nil && cfg.validateForPay() == nil {
		payTypes = append(payTypes, cfg.PayTypes...)
		if defaultType == "" {
			defaultType = cfg.resolveDefaultPayType()
		}
	}
	if cfg, err := loadEpayV2Config(db); err == nil && cfg.validateForPay() == nil {
		payTypes = append(payTypes, cfg.PayTypes...)
		if defaultType == "" {
			defaultType = cfg.resolveDefaultPayType()
		}
	}

	// 去重
	seen := map[string]bool{}
	unique := []string{}
	for _, t := range payTypes {
		if normalized, ok := normalizeEpayPayType(t, ""); ok && !seen[normalized] {
			seen[normalized] = true
			unique = append(unique, normalized)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"enabled":     len(unique) > 0,
			"payTypes":    unique,
			"defaultType": defaultType,
		},
	})
}

// AgentPanelRechargeCreate 我的财务-创建充值订单（代理商给自己余额充值）
func AgentPanelRechargeCreate(c *gin.Context) {
	agentID, ok := getAgentID(c)
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

	orderNo := generateRechargeOrderNo()
	amount := formatCents(amountCents)

	// V1 优先，未开启则 V2
	if cfg, err := loadEpayConfig(db); err == nil && cfg.validateForPay() == nil {
		payType, ok := normalizeEpayPayType(req.PayType, cfg.resolveDefaultPayType())
		if !ok || !cfg.isPayTypeEnabled(payType) {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该支付方式未开启"})
			return
		}
		payURL, frontendReturnURL, err := buildEpaySubmitURL(c, cfg, orderNo, amountCents, payType, "代理商余额充值", "/agent/finance")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		if err := insertAgentRechargeOrder(db, orderNo, agentID, amount, payType, frontendReturnURL); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建充值订单失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "充值订单已创建", "data": gin.H{"orderNo": orderNo, "amount": amount, "payType": payType, "payUrl": payURL}})
		return
	}

	if cfg, err := loadEpayV2Config(db); err == nil && cfg.validateForPay() == nil {
		payType, ok := normalizeEpayPayType(req.PayType, cfg.resolveDefaultPayType())
		if !ok || !cfg.isPayTypeEnabled(payType) {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该支付方式未开启"})
			return
		}
		payURL, frontendReturnURL, err := buildEpayV2Payment(c, cfg, orderNo, amountCents, payType, "代理商余额充值", "/agent/finance")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		if err := insertAgentRechargeOrder(db, orderNo, agentID, amount, payType, frontendReturnURL); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建充值订单失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "充值订单已创建", "data": gin.H{"orderNo": orderNo, "amount": amount, "payType": payType, "payUrl": payURL}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "线上支付未开启，请联系管理员"})
}

func insertAgentRechargeOrder(db *sql.DB, orderNo string, agentID uint, amount string, payType string, returnURL string) error {
	aid := int64(agentID)
	_, err := db.Exec(`
		INSERT INTO recharge_orders (
			order_no, subject_type, subject_id, agent_id, amount, pay_channel, pay_method, status, return_url, remark
		) VALUES (?, 'agent', ?, ?, ?, 'easypay', ?, 'pending', ?, ?)
	`, orderNo, aid, aid, amount, payType, returnURL, "代理商余额充值")
	return err
}

// AgentPanelRechargeStatus 我的财务-查询充值订单状态
func AgentPanelRechargeStatus(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}

	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "订单号不能为空"})
		return
	}

	db := agentPanelDB(c)
	if db == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureRechargeOrderSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化充值订单表失败"})
		return
	}

	var status, amount string
	err := db.QueryRow(`
		SELECT status, amount FROM recharge_orders
		WHERE order_no = ? AND subject_type = 'agent' AND subject_id = ?
	`, orderNo, agentID).Scan(&status, &amount)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "订单不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{"orderNo": orderNo, "status": status, "amount": amount},
	})
}
