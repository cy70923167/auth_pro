package handler

import (
	"database/sql"
	"net/http"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

// LicenseDashboard 授权概览数据
func LicenseDashboard(c *gin.Context) {
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

	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")

	// 统计数据
	var totalLicenses, activeLicenses, expiredLicenses int64
	var todayVerify, yesterdayVerify int64
	var yesterdayTotal, yesterdayActive, yesterdayExpired int64

	db.QueryRow("SELECT COUNT(*) FROM licenses").Scan(&totalLicenses)
	db.QueryRow("SELECT COUNT(*) FROM licenses WHERE status = 'active'").Scan(&activeLicenses)
	db.QueryRow("SELECT COUNT(*) FROM licenses WHERE status = 'expired'").Scan(&expiredLicenses)
	db.QueryRow("SELECT COUNT(*) FROM verify_logs WHERE DATE(created_at) = ?", today).Scan(&todayVerify)
	db.QueryRow("SELECT COUNT(*) FROM verify_logs WHERE DATE(created_at) = ?", yesterday).Scan(&yesterdayVerify)

	// 昨日统计（用于计算趋势）
	db.QueryRow("SELECT COUNT(*) FROM licenses WHERE DATE(created_at) <= ?", yesterday).Scan(&yesterdayTotal)
	db.QueryRow("SELECT COUNT(*) FROM licenses WHERE status = 'active' AND DATE(created_at) <= ?", yesterday).Scan(&yesterdayActive)
	db.QueryRow("SELECT COUNT(*) FROM licenses WHERE status = 'expired' AND DATE(created_at) <= ?", yesterday).Scan(&yesterdayExpired)

	// 最近授权（最新5条）
	recentRows, err := db.Query(`
		SELECT l.id, COALESCE(ld.domain, l.license_key) as domain, COALESCE(a.app_name, '') as app_name,
		       l.type, l.created_at
		FROM licenses l
		LEFT JOIN apps a ON a.id = l.app_id
		LEFT JOIN license_domains ld ON ld.license_id = l.id
		ORDER BY l.created_at DESC
		LIMIT 5
	`)

	type recentItem struct {
		Domain    string `json:"domain"`
		AppName   string `json:"appName"`
		Type      string `json:"type"`
		TypeLabel string `json:"typeLabel"`
		CreatedAt string `json:"createdAt"`
	}

	typeLabels := map[string]string{
		"domain":   "单域名",
		"wildcard": "泛域名",
		"ip":       "IP",
		"key":      "密钥",
	}

	var recentLicenses []recentItem
	if err == nil {
		defer recentRows.Close()
		for recentRows.Next() {
			var id int64
			var domain, appName, lType string
			var createdAt time.Time
			if err := recentRows.Scan(&id, &domain, &appName, &lType, &createdAt); err == nil {
				recentLicenses = append(recentLicenses, recentItem{
					Domain:    domain,
					AppName:   appName,
					Type:      lType,
					TypeLabel: typeLabels[lType],
					CreatedAt: createdAt.Format("2006-01-02 15:04"),
				})
			}
		}
	}
	if recentLicenses == nil {
		recentLicenses = []recentItem{}
	}

	// 即将到期（30天内到期的活跃授权）
	expireRows, err := db.Query(`
		SELECT l.id, COALESCE(ld.domain, l.license_key) as domain, COALESCE(a.app_name, '') as app_name,
		       DATEDIFF(l.expired_at, NOW()) as days_left
		FROM licenses l
		LEFT JOIN apps a ON a.id = l.app_id
		LEFT JOIN license_domains ld ON ld.license_id = l.id
		WHERE l.status = 'active' AND l.expired_at IS NOT NULL
		  AND l.expired_at BETWEEN NOW() AND DATE_ADD(NOW(), INTERVAL 30 DAY)
		ORDER BY l.expired_at ASC
		LIMIT 10
	`)

	type expireItem struct {
		ID       int64  `json:"id"`
		Domain   string `json:"domain"`
		AppName  string `json:"appName"`
		DaysLeft int    `json:"daysLeft"`
	}

	var expiringSoon []expireItem
	if err == nil {
		defer expireRows.Close()
		for expireRows.Next() {
			var item expireItem
			var domain, appName string
			if err := expireRows.Scan(&item.ID, &domain, &appName, &item.DaysLeft); err == nil {
				item.Domain = domain
				item.AppName = appName
				expiringSoon = append(expiringSoon, item)
			}
		}
	}
	if expiringSoon == nil {
		expiringSoon = []expireItem{}
	}

	// 计算趋势百分比
	calcTrend := func(current, previous int64) float64 {
		if previous == 0 {
			if current > 0 {
				return 100
			}
			return 0
		}
		return float64(current-previous) / float64(previous) * 100
	}

	verifyTrend := calcTrend(todayVerify, yesterdayVerify)
	totalTrend := calcTrend(totalLicenses, yesterdayTotal)
	activeTrend := calcTrend(activeLicenses, yesterdayActive)
	expiredTrend := calcTrend(expiredLicenses, yesterdayExpired)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"stats": []gin.H{
				{"title": "总授权数", "value": totalLicenses, "trend": totalTrend},
				{"title": "活跃授权", "value": activeLicenses, "trend": activeTrend},
				{"title": "已过期", "value": expiredLicenses, "trend": expiredTrend},
				{"title": "今日验证", "value": todayVerify, "trend": verifyTrend},
			},
			"recentLicenses": recentLicenses,
			"expiringSoon":   expiringSoon,
		},
	})
}
