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
	_ "github.com/go-sql-driver/mysql"
)

// VerifyLogList 验证日志列表
func VerifyLogList(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	keyword := c.Query("keyword")
	appId := c.Query("appId")
	result := c.Query("result")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	where := []string{"1=1"}
	args := []any{}

	if keyword != "" {
		where = append(where, "v.domain LIKE ?")
		args = append(args, "%"+keyword+"%")
	}
	if appId != "" {
		where = append(where, "v.app_id = ?")
		args = append(args, appId)
	}
	if result != "" {
		if result == "reject" {
			where = append(where, "v.result != 'pass'")
		} else {
			where = append(where, "v.result = ?")
			args = append(args, result)
		}
	}
	if startDate != "" {
		where = append(where, "v.created_at >= ?")
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where = append(where, "v.created_at <= ?")
		args = append(args, endDate+" 23:59:59")
	}

	whereSQL := strings.Join(where, " AND ")

	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM verify_logs v WHERE %s", whereSQL)
	db.QueryRow(countSQL, args...).Scan(&total)

	offset := (page - 1) * pageSize
	listSQL := fmt.Sprintf(`
		SELECT v.id, v.domain, COALESCE(a.app_name, '') as app_name, v.result,
		       v.fail_reason, v.client_ip, v.server_ip, v.created_at
		FROM verify_logs v
		LEFT JOIN apps a ON a.id = v.app_id
		WHERE %s
		ORDER BY v.created_at DESC
		LIMIT ? OFFSET ?
	`, whereSQL)

	listArgs := append(args, pageSize, offset)
	rows, err := db.Query(listSQL, listArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	type logItem struct {
		ID            int64  `json:"id"`
		RequestDomain string `json:"requestDomain"`
		AppName       string `json:"appName"`
		Result        string `json:"result"`
		Reason        string `json:"reason"`
		ClientIP      string `json:"clientIp"`
		ServerIP      string `json:"serverIp"`
		CreatedAt     string `json:"createdAt"`
	}

	var list []logItem
	for rows.Next() {
		var item logItem
		var createdAt time.Time
		var failReason sql.NullString
		err := rows.Scan(&item.ID, &item.RequestDomain, &item.AppName, &item.Result,
			&failReason, &item.ClientIP, &item.ServerIP, &createdAt)
		if err != nil {
			continue
		}
		if failReason.Valid {
			item.Reason = failReason.String
		}
		item.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		list = append(list, item)
	}
	if list == nil {
		list = []logItem{}
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

// VerifyLogClear 清空验证日志
func VerifyLogClear(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	_, err = db.Exec("TRUNCATE TABLE verify_logs")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "清空失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "日志已清空"})
}