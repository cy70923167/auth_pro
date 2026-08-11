package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
)

// ReportOverview 数据报表综合接口
func ReportOverview(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	days := c.DefaultQuery("days", "30")
	daysInt := 30
	switch days {
	case "7":
		daysInt = 7
	case "90":
		daysInt = 90
	}

	now := time.Now()
	startDate := now.AddDate(0, 0, -daysInt).Format("2006-01-02")

	// 1. 盗版趋势: 按天统计 hit_count 总和 + 新增案例数
	piracyTrend := buildPiracyTrend(db, startDate, daysInt, now)

	// 2. 验证通过率: 按天
	verifyTrend := buildVerifyTrend(db, startDate, daysInt, now)

	// 3. 收入报表: 按天充值/消费
	revenueTrend := buildRevenueTrend(db, startDate, daysInt, now)

	// 4. 代理商业绩排行
	agentRank := buildAgentRank(db, startDate)

	// 5. 应用数据明细
	appStats := buildAppStats(db, startDate)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"piracyTrend":  piracyTrend,
			"verifyTrend":  verifyTrend,
			"revenueTrend": revenueTrend,
			"agentRank":    agentRank,
			"appStats":     appStats,
		},
	})
}

// PLACEHOLDER_FUNCS

func buildPiracyTrend(db *sql.DB, startDate string, daysInt int, now time.Time) gin.H {
	dates := []string{}
	piracyRequests := []int{}
	newCases := []int{}

	for i := daysInt - 1; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		dateStr := d.Format("2006-01-02")
		label := fmt.Sprintf("%d/%d", int(d.Month()), d.Day())
		dates = append(dates, label)

		var hitSum int
		db.QueryRow("SELECT COALESCE(SUM(hit_count),0) FROM piracy_records WHERE DATE(last_seen)=?", dateStr).Scan(&hitSum)
		piracyRequests = append(piracyRequests, hitSum)

		var newCount int
		db.QueryRow("SELECT COUNT(*) FROM piracy_records WHERE DATE(first_seen)=?", dateStr).Scan(&newCount)
		newCases = append(newCases, newCount)
	}

	return gin.H{"dates": dates, "piracyRequests": piracyRequests, "newCases": newCases}
}

func buildVerifyTrend(db *sql.DB, startDate string, daysInt int, now time.Time) gin.H {
	dates := []string{}
	passRates := []float64{}

	for i := daysInt - 1; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		dateStr := d.Format("2006-01-02")
		label := fmt.Sprintf("%d/%d", int(d.Month()), d.Day())
		dates = append(dates, label)

		var total, passed int
		db.QueryRow("SELECT COUNT(*) FROM verify_logs WHERE DATE(created_at)=?", dateStr).Scan(&total)
		db.QueryRow("SELECT COUNT(*) FROM verify_logs WHERE DATE(created_at)=? AND status='pass'", dateStr).Scan(&passed)

		rate := 100.0
		if total > 0 {
			rate = float64(passed) / float64(total) * 100
		}
		passRates = append(passRates, float64(int(rate*10))/10)
	}

	return gin.H{"dates": dates, "passRates": passRates}
}

func buildRevenueTrend(db *sql.DB, startDate string, daysInt int, now time.Time) gin.H {
	dates := []string{}
	recharges := []float64{}
	consumes := []float64{}

	for i := daysInt - 1; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		dateStr := d.Format("2006-01-02")
		label := fmt.Sprintf("%d/%d", int(d.Month()), d.Day())
		dates = append(dates, label)

		var recharge, consume float64
		db.QueryRow("SELECT COALESCE(SUM(amount),0) FROM transactions WHERE DATE(created_at)=? AND type='recharge'", dateStr).Scan(&recharge)
		db.QueryRow("SELECT COALESCE(SUM(amount),0) FROM transactions WHERE DATE(created_at)=? AND type='consume'", dateStr).Scan(&consume)
		recharges = append(recharges, recharge)
		consumes = append(consumes, -consume)
	}

	return gin.H{"dates": dates, "recharges": recharges, "consumes": consumes}
}

func buildAgentRank(db *sql.DB, startDate string) []gin.H {
	rows, err := db.Query(`SELECT a.id, a.name,
		COUNT(DISTINCT l.id) as license_count,
		COALESCE(SUM(t.amount),0) as total_amount
		FROM agents a
		LEFT JOIN licenses l ON l.created_by = a.id AND l.created_at >= ?
		LEFT JOIN transactions t ON t.agent_id = a.id AND t.type='recharge' AND t.created_at >= ?
		WHERE a.enabled = 1
		GROUP BY a.id, a.name
		ORDER BY total_amount DESC
		LIMIT 10`, startDate, startDate)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()

	list := []gin.H{}
	for rows.Next() {
		var id int
		var name string
		var licenses int
		var amount float64
		rows.Scan(&id, &name, &licenses, &amount)
		list = append(list, gin.H{"id": id, "name": name, "licenses": licenses, "amount": amount})
	}
	return list
}

func buildAppStats(db *sql.DB, startDate string) []gin.H {
	rows, err := db.Query(`SELECT a.id, a.app_name FROM apps a WHERE a.enabled=1 ORDER BY a.id`)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()

	list := []gin.H{}
	for rows.Next() {
		var appID int
		var appName string
		rows.Scan(&appID, &appName)

		var totalLicenses, activeLicenses, verifyCount, piracyCount int
		var revenue float64
		db.QueryRow("SELECT COUNT(*) FROM licenses WHERE app_id=?", appID).Scan(&totalLicenses)
		db.QueryRow("SELECT COUNT(*) FROM licenses WHERE app_id=? AND enabled=1 AND (expired_at IS NULL OR expired_at > NOW())", appID).Scan(&activeLicenses)
		db.QueryRow("SELECT COUNT(*) FROM verify_logs WHERE app_id=? AND created_at >= ?", appID, startDate).Scan(&verifyCount)
		var passCount int
		db.QueryRow("SELECT COUNT(*) FROM verify_logs WHERE app_id=? AND created_at >= ? AND status='pass'", appID, startDate).Scan(&passCount)
		db.QueryRow("SELECT COALESCE(SUM(hit_count),0) FROM piracy_records WHERE app_id=? AND first_seen >= ?", appID, startDate).Scan(&piracyCount)
		db.QueryRow("SELECT COALESCE(SUM(amount),0) FROM transactions WHERE type='recharge' AND created_at >= ?", startDate).Scan(&revenue)

		passRate := 100.0
		if verifyCount > 0 {
			passRate = float64(passCount) / float64(verifyCount) * 100
		}

		list = append(list, gin.H{
			"appName":        appName,
			"totalLicenses":  totalLicenses,
			"activeLicenses": activeLicenses,
			"verifyCount":    verifyCount,
			"passRate":       float64(int(passRate*10)) / 10,
			"piracyCount":    piracyCount,
			"revenue":        revenue,
		})
	}
	return list
}
