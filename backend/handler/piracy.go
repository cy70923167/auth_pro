package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
)

// ========== Piracy Tracking (盗版追踪) ==========

// PiracyTrackingStats 追踪统计
func PiracyTrackingStats(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var total, pending, blocked, todayNew int
	db.QueryRow("SELECT COUNT(*) FROM piracy_records").Scan(&total)
	db.QueryRow("SELECT COUNT(*) FROM piracy_records WHERE status='discovered'").Scan(&pending)
	db.QueryRow("SELECT COUNT(*) FROM piracy_records WHERE status='blocked'").Scan(&blocked)
	db.QueryRow("SELECT COUNT(*) FROM piracy_records WHERE DATE(first_seen)=CURDATE()").Scan(&todayNew)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"total":    total,
			"pending":  pending,
			"blocked":  blocked,
			"todayNew": todayNew,
		},
	})
}

// PiracyTrackingList 追踪列表
func PiracyTrackingList(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	keyword := c.Query("keyword")
	appId := c.Query("appId")
	status := c.Query("status")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	where := []string{"1=1"}
	args := []any{}

	if keyword != "" {
		where = append(where, "(pr.domain LIKE ? OR pr.server_ip LIKE ?)")
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if appId != "" {
		where = append(where, "pr.app_id = ?")
		args = append(args, appId)
	}
	if status != "" {
		where = append(where, "pr.status = ?")
		args = append(args, status)
	}
	if startDate != "" {
		where = append(where, "pr.first_seen >= ?")
		args = append(args, startDate)
	}
	if endDate != "" {
		where = append(where, "pr.first_seen <= ?")
		args = append(args, endDate+" 23:59:59")
	}

	whereSQL := strings.Join(where, " AND ")

	var total int
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM piracy_records pr WHERE %s", whereSQL)
	db.QueryRow(countSQL, args...).Scan(&total)

	offset := (page - 1) * pageSize
	querySQL := fmt.Sprintf(`SELECT pr.id, pr.domain, pr.server_ip, pr.status, pr.hit_count, pr.first_seen, pr.last_seen, pr.remark, pr.app_id, COALESCE(a.app_name,'') as app_name
		FROM piracy_records pr
		LEFT JOIN apps a ON a.id = pr.app_id
		WHERE %s ORDER BY pr.last_seen DESC LIMIT ? OFFSET ?`, whereSQL)
	queryArgs := append(args, pageSize, offset)

	rows, err := db.Query(querySQL, queryArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	list := []gin.H{}
	for rows.Next() {
		var id, hitCount int
		var appID int
		var domain, serverIP, sts, remark, appName string
		var firstSeen, lastSeen sql.NullTime
		rows.Scan(&id, &domain, &serverIP, &sts, &hitCount, &firstSeen, &lastSeen, &remark, &appID, &appName)

		statusLabel := "发现"
		if sts == "blocked" {
			statusLabel = "已拉黑"
		}

		list = append(list, gin.H{
			"id":          id,
			"domain":      domain,
			"serverIp":    serverIP,
			"status":      sts,
			"statusLabel": statusLabel,
			"hitCount":    hitCount,
			"firstSeenAt": formatNullTime(firstSeen),
			"lastSeenAt":  formatNullTime(lastSeen),
			"remark":      remark,
			"appId":       appID,
			"appName":     appName,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"list":  list,
			"total": total,
		},
	})
}

// PiracyTrackingCreate 手动入库
func PiracyTrackingCreate(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var req struct {
		Domain   string `json:"domain"`
		AppID    int    `json:"appId"`
		SourceIP string `json:"sourceIp"`
		Remark   string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if req.Domain == "" || req.AppID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "域名和应用不能为空"})
		return
	}

	now := time.Now()
	_, err = db.Exec(`INSERT INTO piracy_records (app_id, domain, server_ip, status, hit_count, first_seen, last_seen, remark)
		VALUES (?, ?, ?, 'discovered', 1, ?, ?, ?)`,
		req.AppID, req.Domain, req.SourceIP, now, now, req.Remark)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "入库失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "入库成功"})
}

// PiracyTrackingDetail 详情
func PiracyTrackingDetail(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	id := c.Param("id")

	var (
		recID, hitCount, appID                    int
		domain, serverIP, sts, remark, appName    string
		firstSeen, lastSeen, createdAt, updatedAt sql.NullTime
		evidence                                  sql.NullString
	)
	err = db.QueryRow(`SELECT pr.id, pr.domain, pr.server_ip, pr.status, pr.hit_count, pr.first_seen, pr.last_seen,
		pr.evidence, pr.remark, pr.app_id, pr.created_at, pr.updated_at, COALESCE(a.app_name,'')
		FROM piracy_records pr LEFT JOIN apps a ON a.id = pr.app_id WHERE pr.id=?`, id).
		Scan(&recID, &domain, &serverIP, &sts, &hitCount, &firstSeen, &lastSeen,
			&evidence, &remark, &appID, &createdAt, &updatedAt, &appName)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "记录不存在"})
		return
	}

	statusLabel := "发现"
	if sts == "blocked" {
		statusLabel = "已拉黑"
	}

	// 构建时间线
	timeline := []gin.H{
		{"id": 1, "time": formatNullTime(firstSeen), "type": "warning", "content": "系统自动发现：验证日志中检测到未授权请求"},
	}

	if hitCount > 10 {
		timeline = append(timeline, gin.H{
			"id": 2, "time": formatNullTime(lastSeen), "type": "primary",
			"content": fmt.Sprintf("累计拦截 %d 次请求", hitCount),
		})
	}

	if sts == "blocked" {
		timeline = append(timeline, gin.H{
			"id": 3, "time": formatNullTime(updatedAt), "type": "danger",
			"content": "已加入黑名单，所有请求将被自动阻断",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"id":          recID,
			"domain":      domain,
			"serverIp":    serverIP,
			"status":      sts,
			"statusLabel": statusLabel,
			"hitCount":    hitCount,
			"firstSeenAt": formatNullTime(firstSeen),
			"lastSeenAt":  formatNullTime(lastSeen),
			"evidence":    nullStr(evidence),
			"remark":      remark,
			"appId":       appID,
			"appName":     appName,
			"sourceIps":   serverIP,
			"timeline":    timeline,
		},
	})
}

// PiracyTrackingBlock 拉黑
func PiracyTrackingBlock(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	id := c.Param("id")
	_, err = db.Exec("UPDATE piracy_records SET status='blocked' WHERE id=?", id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "操作失败"})
		return
	}

	// 同时把域名加入黑名单
	var domain string
	var appID int
	db.QueryRow("SELECT domain, app_id FROM piracy_records WHERE id=?", id).Scan(&domain, &appID)
	if domain != "" {
		typ := "domain"
		if isIP(domain) {
			typ = "ip"
		}
		db.Exec(`INSERT IGNORE INTO piracy_blacklist (app_id, type, value, reason, piracy_id) VALUES (?, ?, ?, '盗版追踪自动拉黑', ?)`,
			appID, typ, domain, id)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "已拉黑"})
}

// PiracyTrackingUnblock 解黑
func PiracyTrackingUnblock(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	id := c.Param("id")
	_, err = db.Exec("UPDATE piracy_records SET status='discovered' WHERE id=?", id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "操作失败"})
		return
	}

	// 从黑名单移除
	var domain string
	var appID int
	db.QueryRow("SELECT domain, app_id FROM piracy_records WHERE id=?", id).Scan(&domain, &appID)
	if domain != "" {
		db.Exec("DELETE FROM piracy_blacklist WHERE app_id=? AND value=?", appID, domain)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "已解黑"})
}

// PiracyTrackingBatchBlock 批量拉黑
func PiracyTrackingBatchBlock(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var req struct {
		IDs []int `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	for _, id := range req.IDs {
		db.Exec("UPDATE piracy_records SET status='blocked' WHERE id=? AND status='discovered'", id)
		var domain string
		var appID int
		db.QueryRow("SELECT domain, app_id FROM piracy_records WHERE id=?", id).Scan(&domain, &appID)
		if domain != "" {
			typ := "domain"
			if isIP(domain) {
				typ = "ip"
			}
			db.Exec(`INSERT IGNORE INTO piracy_blacklist (app_id, type, value, reason, piracy_id) VALUES (?, ?, ?, '批量拉黑', ?)`,
				appID, typ, domain, id)
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": fmt.Sprintf("已拉黑 %d 条", len(req.IDs))})
}

// ========== Piracy Alerts (告警中心) ==========

// PiracyAlertStats 告警统计
func PiracyAlertStats(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var unhandled, today, week, handled int
	db.QueryRow("SELECT COUNT(*) FROM piracy_alerts WHERE status='pending'").Scan(&unhandled)
	db.QueryRow("SELECT COUNT(*) FROM piracy_alerts WHERE DATE(created_at)=CURDATE()").Scan(&today)
	db.QueryRow("SELECT COUNT(*) FROM piracy_alerts WHERE created_at >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)").Scan(&week)
	db.QueryRow("SELECT COUNT(*) FROM piracy_alerts WHERE status='processed'").Scan(&handled)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"unhandled": unhandled,
			"today":     today,
			"week":      week,
			"handled":   handled,
		},
	})
}

// PiracyAlertList 告警列表
func PiracyAlertList(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	alertType := c.Query("type")
	level := c.Query("level")
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	where := []string{"1=1"}
	args := []any{}

	if alertType != "" {
		where = append(where, "pa.alert_type = ?")
		args = append(args, alertType)
	}
	if level != "" {
		where = append(where, "pa.severity = ?")
		args = append(args, level)
	}
	if status != "" {
		// 前端: pending/handled/ignored → DB: pending/processed/ignored
		dbStatus := status
		if status == "handled" {
			dbStatus = "processed"
		}
		where = append(where, "pa.status = ?")
		args = append(args, dbStatus)
	}

	whereSQL := strings.Join(where, " AND ")

	var total int
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM piracy_alerts pa WHERE %s", whereSQL)
	db.QueryRow(countSQL, args...).Scan(&total)

	offset := (page - 1) * pageSize
	querySQL := fmt.Sprintf(`SELECT pa.id, pa.alert_type, pa.severity, pa.status, pa.domain, pa.detail, pa.created_at
		FROM piracy_alerts pa
		WHERE %s ORDER BY pa.created_at DESC LIMIT ? OFFSET ?`, whereSQL)
	queryArgs := append(args, pageSize, offset)

	rows, err := db.Query(querySQL, queryArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	levelLabelMap := map[string]string{"low": "提示", "medium": "警告", "high": "紧急"}
	typeLabelMap := map[string]string{"piracy": "盗版异常", "expire": "授权到期", "balance": "余额不足", "quota": "配额耗尽", "verify_anomaly": "验证异常"}
	statusLabelMap := map[string]string{"pending": "未处理", "processed": "已处理", "ignored": "已忽略"}
	// 前端 level 映射: DB severity(low/medium/high) → 前端(info/warning/critical)
	levelKeyMap := map[string]string{"low": "info", "medium": "warning", "high": "critical"}

	list := []gin.H{}
	for rows.Next() {
		var id int
		var alertType, severity, sts, domain string
		var detail sql.NullString
		var createdAt sql.NullTime
		rows.Scan(&id, &alertType, &severity, &sts, &domain, &detail, &createdAt)

		frontStatus := sts
		if sts == "processed" {
			frontStatus = "handled"
		}

		list = append(list, gin.H{
			"id":          id,
			"type":        alertType,
			"typeLabel":   typeLabelMap[alertType],
			"level":       levelKeyMap[severity],
			"levelLabel":  levelLabelMap[severity],
			"title":       nullStr(detail),
			"target":      domain,
			"status":      frontStatus,
			"statusLabel": statusLabelMap[sts],
			"createdAt":   formatNullTime(createdAt),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"list":  list,
			"total": total,
		},
	})
}

// PiracyAlertMark 标记告警状态
func PiracyAlertMark(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	id := c.Param("id")
	var req struct {
		Status string `json:"status"` // handled / ignored
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	dbStatus := req.Status
	if req.Status == "handled" {
		dbStatus = "processed"
	}

	now := time.Now()
	_, err = db.Exec("UPDATE piracy_alerts SET status=?, processed_at=? WHERE id=?", dbStatus, now, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "操作失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "操作成功"})
}

// PiracyAlertBatchMark 批量标记
func PiracyAlertBatchMark(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var req struct {
		IDs    []int  `json:"ids"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	dbStatus := req.Status
	if req.Status == "handled" {
		dbStatus = "processed"
	}

	now := time.Now()
	placeholders := make([]string, len(req.IDs))
	batchArgs := []any{dbStatus, now}
	for i, id := range req.IDs {
		placeholders[i] = "?"
		batchArgs = append(batchArgs, id)
	}
	sql2 := fmt.Sprintf("UPDATE piracy_alerts SET status=?, processed_at=? WHERE id IN (%s) AND status='pending'",
		strings.Join(placeholders, ","))
	db.Exec(sql2, batchArgs...)

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": fmt.Sprintf("已处理 %d 条", len(req.IDs))})
}

// ========== Piracy Blacklist (黑名单) ==========

// PiracyBlacklistList 黑名单列表
func PiracyBlacklistList(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	keyword := c.Query("keyword")
	typ := c.Query("type")
	source := c.Query("source")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	where := []string{"1=1"}
	args := []any{}

	if keyword != "" {
		where = append(where, "pb.value LIKE ?")
		args = append(args, "%"+keyword+"%")
	}
	if typ != "" {
		where = append(where, "pb.type = ?")
		args = append(args, typ)
	}
	if source != "" {
		switch source {
		case "piracy":
			where = append(where, "pb.piracy_id IS NOT NULL")
		case "manual":
			where = append(where, "pb.piracy_id IS NULL AND pb.reason NOT LIKE '%自动%'")
		case "auto":
			where = append(where, "pb.piracy_id IS NULL AND pb.reason LIKE '%自动%'")
		}
	}

	whereSQL := strings.Join(where, " AND ")

	var total int
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM piracy_blacklist pb WHERE %s", whereSQL)
	db.QueryRow(countSQL, args...).Scan(&total)

	offset := (page - 1) * pageSize
	querySQL := fmt.Sprintf(`SELECT pb.id, pb.value, pb.type, pb.reason, pb.piracy_id, pb.created_at
		FROM piracy_blacklist pb
		WHERE %s ORDER BY pb.created_at DESC LIMIT ? OFFSET ?`, whereSQL)
	queryArgs := append(args, pageSize, offset)

	rows, err := db.Query(querySQL, queryArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	typeLabelMap := map[string]string{"domain": "域名", "ip": "IP"}

	list := []gin.H{}
	for rows.Next() {
		var id int
		var value, bType, reason string
		var piracyID sql.NullInt64
		var createdAt sql.NullTime
		rows.Scan(&id, &value, &bType, &reason, &piracyID, &createdAt)

		source := "manual"
		sourceLabel := "手动添加"
		if piracyID.Valid {
			source = "piracy"
			sourceLabel = "盗版入库"
		} else if strings.Contains(reason, "自动") {
			source = "auto"
			sourceLabel = "自动规则"
		}

		typeLabel := typeLabelMap[bType]
		if typeLabel == "" {
			typeLabel = bType
		}

		list = append(list, gin.H{
			"id":          id,
			"value":       value,
			"type":        bType,
			"typeLabel":   typeLabel,
			"source":      source,
			"sourceLabel": sourceLabel,
			"remark":      reason,
			"createdAt":   formatNullTime(createdAt),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"list":  list,
			"total": total,
		},
	})
}

// PiracyBlacklistCreate 添加黑名单
func PiracyBlacklistCreate(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var req struct {
		Type   string `json:"type"`
		Value  string `json:"value"`
		Remark string `json:"remark"`
		AppID  int    `json:"appId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if req.Value == "" || req.Type == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "类型和值不能为空"})
		return
	}

	appID := req.AppID
	if appID == 0 {
		appID = 1 // 默认关联第一个应用
	}

	_, err = db.Exec(`INSERT INTO piracy_blacklist (app_id, type, value, reason) VALUES (?, ?, ?, ?)`,
		appID, req.Type, req.Value, req.Remark)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该记录已存在"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "添加失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "添加成功"})
}

// PiracyBlacklistUpdate 编辑黑名单
func PiracyBlacklistUpdate(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	id := c.Param("id")
	var req struct {
		Type   string `json:"type"`
		Value  string `json:"value"`
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	_, err = db.Exec("UPDATE piracy_blacklist SET type=?, value=?, reason=? WHERE id=?",
		req.Type, req.Value, req.Remark, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "修改失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "修改成功"})
}

// PiracyBlacklistDelete 删除黑名单
func PiracyBlacklistDelete(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	id := c.Param("id")
	_, err = db.Exec("DELETE FROM piracy_blacklist WHERE id=?", id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "已移除"})
}

// PiracyBlacklistBatchDelete 批量移除
func PiracyBlacklistBatchDelete(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var req struct {
		IDs []int `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	placeholders := make([]string, len(req.IDs))
	batchArgs := []any{}
	for i, id := range req.IDs {
		placeholders[i] = "?"
		batchArgs = append(batchArgs, id)
	}
	sql2 := fmt.Sprintf("DELETE FROM piracy_blacklist WHERE id IN (%s)", strings.Join(placeholders, ","))
	db.Exec(sql2, batchArgs...)

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": fmt.Sprintf("已移除 %d 条", len(req.IDs))})
}

// ========== Helpers ==========

func formatNullTime(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format("2006-01-02 15:04")
}

func nullStr(s sql.NullString) string {
	if !s.Valid {
		return ""
	}
	return s.String
}

func isIP(s string) bool {
	for _, c := range s {
		if c != '.' && c != ':' && !(c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}
