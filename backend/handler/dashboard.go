package handler

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

type dashboardCard struct {
	Title  string  `json:"title"`
	Value  float64 `json:"value"`
	Unit   string  `json:"unit"`
	Icon   string  `json:"icon"`
	Trend  float64 `json:"trend"`
	Prefix string  `json:"prefix"`
}

type dashboardTrendItem struct {
	Date     string  `json:"date"`
	Revenue  float64 `json:"revenue"`
	Orders   int64   `json:"orders"`
	Licenses int64   `json:"licenses"`
}

type dashboardStatusItem struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
	Type  string `json:"type"`
}

type dashboardRankItem struct {
	Name    string  `json:"name"`
	Value   int64   `json:"value"`
	Revenue float64 `json:"revenue"`
	Extra   string  `json:"extra"`
}

type dashboardTodoItem struct {
	Title string `json:"title"`
	Value int64  `json:"value"`
	Level string `json:"level"`
	Desc  string `json:"desc"`
}

type dashboardActivityItem struct {
	Title string `json:"title"`
	Desc  string `json:"desc"`
	Time  string `json:"time"`
	Type  string `json:"type"`
}

type dashboardMetricItem struct {
	Label  string  `json:"label"`
	Value  float64 `json:"value"`
	Unit   string  `json:"unit"`
	Prefix string  `json:"prefix"`
	Desc   string  `json:"desc"`
	Level  string  `json:"level"`
}

type dashboardQuickEntry struct {
	Title string `json:"title"`
	Desc  string `json:"desc"`
	Path  string `json:"path"`
	Icon  string `json:"icon"`
	Type  string `json:"type"`
}

func AdminDashboardOverview(c *gin.Context) {
	db, ok := openDashboardDB(c)
	if !ok {
		return
	}
	defer db.Close()

	now := time.Now()
	today := now.Format("2006-01-02")

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"cards":          dashboardCards(db, now),
			"trend":          dashboardTrendList(db, now),
			"licenseStatus":  dashboardLicenseStatus(db),
			"appRanking":     dashboardAppRanking(db),
			"agentRanking":   dashboardAgentRanking(db),
			"todos":          dashboardTodos(db),
			"activities":     dashboardActivities(db),
			"paymentMethods": dashboardPaymentMethods(db, today),
			"agentMetrics":   dashboardAgentMetrics(db, today),
			"userMetrics":    dashboardUserMetrics(db, today),
			"appMetrics":     dashboardAppMetrics(db, today),
			"riskAlerts":     dashboardRiskAlerts(db),
			"quickEntries":   dashboardQuickEntries(),
		},
	})
}

func AdminDashboardCards(c *gin.Context) {
	db, ok := openDashboardDB(c)
	if !ok {
		return
	}
	defer db.Close()
	dashboardOK(c, dashboardCards(db, time.Now()))
}

func AdminDashboardTrend(c *gin.Context) {
	db, ok := openDashboardDB(c)
	if !ok {
		return
	}
	defer db.Close()
	dashboardOK(c, dashboardTrendList(db, time.Now()))
}

func AdminDashboardLicenseStatus(c *gin.Context) {
	db, ok := openDashboardDB(c)
	if !ok {
		return
	}
	defer db.Close()
	dashboardOK(c, dashboardLicenseStatus(db))
}

func AdminDashboardPaymentMethods(c *gin.Context) {
	db, ok := openDashboardDB(c)
	if !ok {
		return
	}
	defer db.Close()
	dashboardOK(c, dashboardPaymentMethods(db, time.Now().Format("2006-01-02")))
}

func AdminDashboardAgentMetrics(c *gin.Context) {
	db, ok := openDashboardDB(c)
	if !ok {
		return
	}
	defer db.Close()
	dashboardOK(c, dashboardAgentMetrics(db, time.Now().Format("2006-01-02")))
}

func AdminDashboardUserMetrics(c *gin.Context) {
	db, ok := openDashboardDB(c)
	if !ok {
		return
	}
	defer db.Close()
	dashboardOK(c, dashboardUserMetrics(db, time.Now().Format("2006-01-02")))
}

func AdminDashboardAppMetrics(c *gin.Context) {
	db, ok := openDashboardDB(c)
	if !ok {
		return
	}
	defer db.Close()
	dashboardOK(c, dashboardAppMetrics(db, time.Now().Format("2006-01-02")))
}

func AdminDashboardAppRanking(c *gin.Context) {
	db, ok := openDashboardDB(c)
	if !ok {
		return
	}
	defer db.Close()
	dashboardOK(c, dashboardAppRanking(db))
}

func AdminDashboardAgentRanking(c *gin.Context) {
	db, ok := openDashboardDB(c)
	if !ok {
		return
	}
	defer db.Close()
	dashboardOK(c, dashboardAgentRanking(db))
}

func AdminDashboardActivities(c *gin.Context) {
	db, ok := openDashboardDB(c)
	if !ok {
		return
	}
	defer db.Close()
	dashboardOK(c, dashboardActivities(db))
}

func AdminDashboardQuickEntries(c *gin.Context) {
	dashboardOK(c, dashboardQuickEntries())
}

func openDashboardDB(c *gin.Context) (*sql.DB, bool) {
	cfg, err := config.LoadDBConfig()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "系统未配置"})
		return nil, false
	}

	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return nil, false
	}
	return db, true
}

func dashboardOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": data})
}

func dashboardCards(db *sql.DB, now time.Time) []dashboardCard {
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")

	todayRevenue := dashboardFloat(db, "SELECT COALESCE(SUM(ABS(amount)), 0) FROM transactions WHERE type IN ('purchase', 'consume') AND amount < 0 AND DATE(created_at) = ?", today)
	yesterdayRevenue := dashboardFloat(db, "SELECT COALESCE(SUM(ABS(amount)), 0) FROM transactions WHERE type IN ('purchase', 'consume') AND amount < 0 AND DATE(created_at) = ?", yesterday)
	todayOrders := dashboardInt(db, "SELECT COUNT(*) FROM transactions WHERE type IN ('purchase', 'consume') AND amount < 0 AND DATE(created_at) = ?", today)
	yesterdayOrders := dashboardInt(db, "SELECT COUNT(*) FROM transactions WHERE type IN ('purchase', 'consume') AND amount < 0 AND DATE(created_at) = ?", yesterday)
	activeLicenses := dashboardInt(db, "SELECT COUNT(*) FROM licenses WHERE status = 'active' AND (expired_at IS NULL OR expired_at > NOW())")
	yesterdayActiveLicenses := dashboardInt(db, "SELECT COUNT(*) FROM licenses WHERE status = 'active' AND (expired_at IS NULL OR expired_at > ?) AND created_at < ?", yesterday+" 23:59:59", today+" 00:00:00")
	expiringLicenses := dashboardInt(db, "SELECT COUNT(*) FROM licenses WHERE status = 'active' AND expired_at IS NOT NULL AND expired_at > NOW() AND expired_at <= DATE_ADD(NOW(), INTERVAL 7 DAY)")
	yesterdayExpiringLicenses := dashboardInt(db, "SELECT COUNT(*) FROM licenses WHERE status = 'active' AND expired_at IS NOT NULL AND expired_at > ? AND expired_at <= DATE_ADD(?, INTERVAL 7 DAY) AND created_at < ?", yesterday+" 23:59:59", yesterday+" 23:59:59", today+" 00:00:00")
	totalUsers := dashboardInt(db, "SELECT COUNT(*) FROM users")
	yesterdayUsers := dashboardInt(db, "SELECT COUNT(*) FROM users WHERE created_at < ?", today+" 00:00:00")
	totalAgents := dashboardInt(db, "SELECT COUNT(*) FROM agents")
	yesterdayAgents := dashboardInt(db, "SELECT COUNT(*) FROM agents WHERE created_at < ?", today+" 00:00:00")

	return []dashboardCard{
		{Title: "今日收入", Value: todayRevenue, Unit: "元", Icon: "ri:money-cny-circle-line", Trend: dashboardTrend(todayRevenue, yesterdayRevenue), Prefix: "¥"},
		{Title: "今日订单", Value: float64(todayOrders), Unit: "单", Icon: "ri:file-list-3-line", Trend: dashboardTrend(float64(todayOrders), float64(yesterdayOrders))},
		{Title: "有效授权", Value: float64(activeLicenses), Unit: "个", Icon: "ri:shield-check-line", Trend: dashboardTrend(float64(activeLicenses), float64(yesterdayActiveLicenses))},
		{Title: "即将到期", Value: float64(expiringLicenses), Unit: "个", Icon: "ri:timer-flash-line", Trend: dashboardTrend(float64(expiringLicenses), float64(yesterdayExpiringLicenses))},
		{Title: "用户总数", Value: float64(totalUsers), Unit: "人", Icon: "ri:user-3-line", Trend: dashboardTrend(float64(totalUsers), float64(yesterdayUsers))},
		{Title: "代理商", Value: float64(totalAgents), Unit: "个", Icon: "ri:team-line", Trend: dashboardTrend(float64(totalAgents), float64(yesterdayAgents))},
	}
}

func dashboardInt(db *sql.DB, query string, args ...any) int64 {
	var value int64
	_ = db.QueryRow(query, args...).Scan(&value)
	return value
}

func dashboardFloat(db *sql.DB, query string, args ...any) float64 {
	var value float64
	_ = db.QueryRow(query, args...).Scan(&value)
	return value
}

func dashboardTrend(current float64, previous float64) float64 {
	if previous == 0 {
		if current > 0 {
			return 100
		}
		return 0
	}
	return math.Round((current-previous)/previous*1000) / 10
}

func dashboardTrendList(db *sql.DB, now time.Time) []dashboardTrendItem {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start := today.AddDate(0, 0, -6)
	end := today.AddDate(0, 0, 1)

	list := make([]dashboardTrendItem, 7)
	dayIndexes := make(map[string]int, 7)
	for i := 0; i < 7; i++ {
		day := start.AddDate(0, 0, i)
		list[i].Date = day.Format("01-02")
		dayIndexes[day.Format("2006-01-02")] = i
	}

	// 原先这里按天循环执行 21 条 SQL，远程 MySQL 下会超过前端 15 秒超时。
	// 改成两张表各聚合一次，再由 Go 补齐 7 天空缺日期。
	transactionRows, err := db.Query(`
		SELECT DATE(created_at) AS day, COALESCE(SUM(ABS(amount)), 0), COUNT(*)
		FROM transactions
		WHERE type IN ('purchase', 'consume')
		  AND amount < 0
		  AND created_at >= ?
		  AND created_at < ?
		GROUP BY DATE(created_at)
	`, start, end)
	if err == nil {
		defer transactionRows.Close()
		for transactionRows.Next() {
			var day time.Time
			var revenue float64
			var orders int64
			if err := transactionRows.Scan(&day, &revenue, &orders); err == nil {
				if index, ok := dayIndexes[day.Format("2006-01-02")]; ok {
					list[index].Revenue = revenue
					list[index].Orders = orders
				}
			}
		}
	}

	licenseRows, err := db.Query(`
		SELECT DATE(created_at) AS day, COUNT(*)
		FROM licenses
		WHERE created_at >= ?
		  AND created_at < ?
		GROUP BY DATE(created_at)
	`, start, end)
	if err == nil {
		defer licenseRows.Close()
		for licenseRows.Next() {
			var day time.Time
			var count int64
			if err := licenseRows.Scan(&day, &count); err == nil {
				if index, ok := dayIndexes[day.Format("2006-01-02")]; ok {
					list[index].Licenses = count
				}
			}
		}
	}

	return list
}

func dashboardLicenseStatus(db *sql.DB) []dashboardStatusItem {
	return []dashboardStatusItem{
		{Name: "有效授权", Type: "active", Value: dashboardInt(db, "SELECT COUNT(*) FROM licenses WHERE status = 'active' AND (expired_at IS NULL OR expired_at > NOW())")},
		{Name: "即将到期", Type: "expiring", Value: dashboardInt(db, "SELECT COUNT(*) FROM licenses WHERE status = 'active' AND expired_at IS NOT NULL AND expired_at > NOW() AND expired_at <= DATE_ADD(NOW(), INTERVAL 7 DAY)")},
		{Name: "已过期", Type: "expired", Value: dashboardInt(db, "SELECT COUNT(*) FROM licenses WHERE status = 'expired' OR (expired_at IS NOT NULL AND expired_at <= NOW())")},
		{Name: "已吊销", Type: "revoked", Value: dashboardInt(db, "SELECT COUNT(*) FROM licenses WHERE status = 'revoked'")},
	}
}

func dashboardAppRanking(db *sql.DB) []dashboardRankItem {
	rows, err := db.Query(`
		SELECT a.app_name,
		       COUNT(l.id) AS license_count,
		       COALESCE(SUM(CASE WHEN t.type IN ('purchase', 'consume') AND t.amount < 0 THEN ABS(t.amount) ELSE 0 END), 0) AS revenue
		FROM apps a
		LEFT JOIN licenses l ON l.app_id = a.id
		LEFT JOIN transactions t ON t.ref_type = 'license' AND t.ref_id = l.id
		GROUP BY a.id, a.app_name
		ORDER BY license_count DESC, revenue DESC
		LIMIT 6
	`)
	if err != nil {
		return []dashboardRankItem{}
	}
	defer rows.Close()

	list := []dashboardRankItem{}
	for rows.Next() {
		var item dashboardRankItem
		if err := rows.Scan(&item.Name, &item.Value, &item.Revenue); err == nil {
			item.Extra = fmt.Sprintf("收入 ¥%.2f", item.Revenue)
			list = append(list, item)
		}
	}
	return list
}

func dashboardAgentRanking(db *sql.DB) []dashboardRankItem {
	rows, err := db.Query(`
		SELECT COALESCE(a.name, a.email, ''),
		       COUNT(l.id) AS license_count,
		       COALESCE(SUM(CASE WHEN DATE(l.created_at) = CURDATE() THEN 1 ELSE 0 END), 0) AS today_count,
		       COALESCE(a.balance, 0)
		FROM agents a
		LEFT JOIN licenses l ON l.owner_type = 'agent' AND l.owner_id = a.id
		GROUP BY a.id, a.name, a.email, a.balance
		ORDER BY license_count DESC, today_count DESC
		LIMIT 6
	`)
	if err != nil {
		return []dashboardRankItem{}
	}
	defer rows.Close()

	list := []dashboardRankItem{}
	for rows.Next() {
		var item dashboardRankItem
		var todayCount int64
		var balance float64
		if err := rows.Scan(&item.Name, &item.Value, &todayCount, &balance); err == nil {
			item.Revenue = balance
			item.Extra = fmt.Sprintf("今日开通 %d 个｜余额 ¥%.2f", todayCount, balance)
			list = append(list, item)
		}
	}
	return list
}

func dashboardTodos(db *sql.DB) []dashboardTodoItem {
	return []dashboardTodoItem{
		{Title: "7 天内到期授权", Value: dashboardInt(db, "SELECT COUNT(*) FROM licenses WHERE status = 'active' AND expired_at IS NOT NULL AND expired_at > NOW() AND expired_at <= DATE_ADD(NOW(), INTERVAL 7 DAY)"), Level: "warning", Desc: "建议提前联系续费或处理"},
		{Title: "今日验证失败", Value: dashboardInt(db, "SELECT COUNT(*) FROM verify_logs WHERE result IN ('fail', 'expired', 'blacklisted') AND DATE(created_at) = CURDATE()"), Level: "danger", Desc: "关注异常域名、过期授权和黑名单命中"},
		{Title: "余额不足代理商", Value: dashboardInt(db, "SELECT COUNT(*) FROM agents WHERE enabled = 1 AND balance < 100"), Level: "warning", Desc: "余额低于 100 元可能影响开通授权"},
		{Title: "禁用账号", Value: dashboardInt(db, "SELECT (SELECT COUNT(*) FROM agents WHERE enabled = 0) + (SELECT COUNT(*) FROM users WHERE enabled = 0)"), Level: "info", Desc: "包含禁用代理商和禁用用户"},
		{Title: "待处理盗版告警", Value: dashboardInt(db, "SELECT COUNT(*) FROM piracy_alerts WHERE status = 'pending'"), Level: "danger", Desc: "需要在反盗版模块确认"},
	}
}

func dashboardActivities(db *sql.DB) []dashboardActivityItem {
	rows, err := db.Query(`
		SELECT COALESCE(a.app_name, ''), l.source, l.owner_type, l.created_at
		FROM licenses l
		LEFT JOIN apps a ON a.id = l.app_id
		ORDER BY l.created_at DESC
		LIMIT 8
	`)
	if err != nil {
		return []dashboardActivityItem{}
	}
	defer rows.Close()

	sourceText := map[string]string{
		"admin":         "管理员开通",
		"agent":         "代理商开通",
		"user_purchase": "用户自助购买",
		"card":          "卡密开通",
		"card_key":      "卡密开通",
		"gift":          "赠送开通",
		"api":           "API开通",
	}
	list := []dashboardActivityItem{}
	for rows.Next() {
		var appName, source, ownerType string
		var createdAt time.Time
		if err := rows.Scan(&appName, &source, &ownerType, &createdAt); err == nil {
			text := sourceText[source]
			if text == "" {
				text = source
			}
			list = append(list, dashboardActivityItem{
				Title: text,
				Desc:  fmt.Sprintf("%s 新增 1 个授权", appName),
				Time:  createdAt.Format("2006-01-02 15:04"),
				Type:  ownerType,
			})
		}
	}
	return list
}

func dashboardPaymentMethods(db *sql.DB, today string) []dashboardRankItem {
	balanceAmount := dashboardFloat(db, "SELECT COALESCE(SUM(ABS(amount)), 0) FROM transactions WHERE type IN ('purchase', 'consume') AND amount < 0 AND DATE(created_at) = ?", today)
	return []dashboardRankItem{
		{Name: "余额支付", Value: dashboardInt(db, "SELECT COUNT(*) FROM transactions WHERE type IN ('purchase', 'consume') AND amount < 0 AND DATE(created_at) = ?", today), Revenue: balanceAmount, Extra: "当前实际支付通道"},
		{Name: "支付宝", Value: 0, Revenue: 0, Extra: "通道预留"},
		{Name: "微信支付", Value: 0, Revenue: 0, Extra: "通道预留"},
	}
}

func dashboardAgentMetrics(db *sql.DB, today string) []dashboardMetricItem {
	return []dashboardMetricItem{
		{Label: "代理商总数", Value: float64(dashboardInt(db, "SELECT COUNT(*) FROM agents")), Unit: "个", Desc: fmt.Sprintf("启用 %d 个 / 禁用 %d 个", dashboardInt(db, "SELECT COUNT(*) FROM agents WHERE enabled = 1"), dashboardInt(db, "SELECT COUNT(*) FROM agents WHERE enabled = 0")), Level: "primary"},
		{Label: "今日新增代理商", Value: float64(dashboardInt(db, "SELECT COUNT(*) FROM agents WHERE DATE(created_at) = ?", today)), Unit: "个", Desc: "当天创建的代理账号", Level: "success"},
		{Label: "代理商余额", Value: dashboardFloat(db, "SELECT COALESCE(SUM(balance), 0) FROM agents"), Unit: "元", Prefix: "¥", Desc: "所有代理商账户余额合计", Level: "warning"},
		{Label: "今日代理消费", Value: dashboardFloat(db, "SELECT COALESCE(SUM(ABS(amount)), 0) FROM transactions WHERE subject_type = 'agent' AND type = 'consume' AND amount < 0 AND DATE(created_at) = ?", today), Unit: "元", Prefix: "¥", Desc: "代理商今日开通授权扣费", Level: "danger"},
	}
}

func dashboardUserMetrics(db *sql.DB, today string) []dashboardMetricItem {
	return []dashboardMetricItem{
		{Label: "用户总数", Value: float64(dashboardInt(db, "SELECT COUNT(*) FROM users")), Unit: "人", Desc: fmt.Sprintf("启用 %d 人 / 禁用 %d 人", dashboardInt(db, "SELECT COUNT(*) FROM users WHERE enabled = 1"), dashboardInt(db, "SELECT COUNT(*) FROM users WHERE enabled = 0")), Level: "primary"},
		{Label: "今日新增用户", Value: float64(dashboardInt(db, "SELECT COUNT(*) FROM users WHERE DATE(created_at) = ?", today)), Unit: "人", Desc: "当天注册或后台创建用户", Level: "success"},
		{Label: "用户余额", Value: dashboardFloat(db, "SELECT COALESCE(SUM(balance), 0) FROM users"), Unit: "元", Prefix: "¥", Desc: "所有用户账户余额合计", Level: "warning"},
		{Label: "今日用户消费", Value: dashboardFloat(db, "SELECT COALESCE(SUM(ABS(amount)), 0) FROM transactions WHERE subject_type = 'user' AND type = 'purchase' AND amount < 0 AND DATE(created_at) = ?", today), Unit: "元", Prefix: "¥", Desc: "用户今日购买授权支出", Level: "danger"},
	}
}

func dashboardAppMetrics(db *sql.DB, today string) []dashboardMetricItem {
	return []dashboardMetricItem{
		{Label: "应用总数", Value: float64(dashboardInt(db, "SELECT COUNT(*) FROM apps")), Unit: "个", Desc: fmt.Sprintf("上架 %d 个 / 下架 %d 个", dashboardInt(db, "SELECT COUNT(*) FROM apps WHERE enabled = 1"), dashboardInt(db, "SELECT COUNT(*) FROM apps WHERE enabled = 0")), Level: "primary"},
		{Label: "套餐数量", Value: float64(dashboardInt(db, "SELECT COUNT(*) FROM license_plans")), Unit: "个", Desc: fmt.Sprintf("启用套餐 %d 个", dashboardInt(db, "SELECT COUNT(*) FROM license_plans WHERE enabled = 1")), Level: "success"},
		{Label: "今日新增授权", Value: float64(dashboardInt(db, "SELECT COUNT(*) FROM licenses WHERE DATE(created_at) = ?", today)), Unit: "个", Desc: "今天新开通的授权", Level: "warning"},
		{Label: "永久授权", Value: float64(dashboardInt(db, "SELECT COUNT(*) FROM licenses WHERE expired_at IS NULL OR duration_days = 0")), Unit: "个", Desc: "无固定到期时间的授权", Level: "danger"},
	}
}

func dashboardRiskAlerts(db *sql.DB) []dashboardTodoItem {
	items := []dashboardTodoItem{
		{Title: "数据库连接", Value: 0, Level: "success", Desc: "工作台数据读取正常"},
	}

	expiring := dashboardInt(db, "SELECT COUNT(*) FROM licenses WHERE status = 'active' AND expired_at IS NOT NULL AND expired_at > NOW() AND expired_at <= DATE_ADD(NOW(), INTERVAL 7 DAY)")
	if expiring > 0 {
		items = append(items, dashboardTodoItem{Title: "授权即将到期", Value: expiring, Level: "warning", Desc: "7 天内到期，建议提前续费处理"})
	}

	failedVerify := dashboardInt(db, "SELECT COUNT(*) FROM verify_logs WHERE result IN ('fail', 'expired', 'blacklisted') AND DATE(created_at) = CURDATE()")
	if failedVerify > 0 {
		items = append(items, dashboardTodoItem{Title: "今日验证异常", Value: failedVerify, Level: "danger", Desc: "包含失败、过期和黑名单命中"})
	}

	lowBalanceAgents := dashboardInt(db, "SELECT COUNT(*) FROM agents WHERE enabled = 1 AND balance < 100")
	if lowBalanceAgents > 0 {
		items = append(items, dashboardTodoItem{Title: "代理商余额不足", Value: lowBalanceAgents, Level: "warning", Desc: "余额低于 100 元，可能影响开通授权"})
	}

	pendingAlerts := dashboardInt(db, "SELECT COUNT(*) FROM piracy_alerts WHERE status = 'pending'")
	if pendingAlerts > 0 {
		items = append(items, dashboardTodoItem{Title: "待处理盗版告警", Value: pendingAlerts, Level: "danger", Desc: "请到反盗版模块确认处理"})
	}

	disabledAccounts := dashboardInt(db, "SELECT (SELECT COUNT(*) FROM agents WHERE enabled = 0) + (SELECT COUNT(*) FROM users WHERE enabled = 0)")
	if disabledAccounts > 0 {
		items = append(items, dashboardTodoItem{Title: "禁用账号", Value: disabledAccounts, Level: "info", Desc: "包含禁用代理商和禁用用户"})
	}

	return items
}

func dashboardQuickEntries() []dashboardQuickEntry {
	return []dashboardQuickEntry{
		{Title: "新增授权", Desc: "后台快速开通授权", Path: "/license/list", Icon: "ri:shield-keyhole-line", Type: "primary"},
		{Title: "新增代理商", Desc: "创建代理商账号", Path: "/agent/list", Icon: "ri:team-line", Type: "success"},
		{Title: "应用管理", Desc: "维护应用和密钥", Path: "/license/apps", Icon: "ri:apps-2-line", Type: "warning"},
		{Title: "交易流水", Desc: "查看充值和消费记录", Path: "/agent/recharge", Icon: "ri:bill-line", Type: "danger"},
		{Title: "验证日志", Desc: "排查授权验证异常", Path: "/license/logs", Icon: "ri:file-search-line", Type: "info"},
		{Title: "盗版告警", Desc: "处理风险告警", Path: "/piracy/alerts", Icon: "ri:alarm-warning-line", Type: "danger"},
	}
}
