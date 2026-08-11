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

// TransactionList 财务流水列表
func TransactionList(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	agentId := c.Query("agentId")
	txType := c.Query("type")
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

	if agentId != "" {
		where = append(where, "t.subject_id = ? AND t.subject_type = 'agent'")
		args = append(args, agentId)
	}
	if txType != "" {
		where = append(where, "t.type = ?")
		args = append(args, txType)
	}
	if startDate != "" {
		where = append(where, "t.created_at >= ?")
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where = append(where, "t.created_at <= ?")
		args = append(args, endDate+" 23:59:59")
	}

	whereSQL := strings.Join(where, " AND ")

	var total int64
	db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM transactions t WHERE %s", whereSQL), args...).Scan(&total)

	offset := (page - 1) * pageSize
	listSQL := fmt.Sprintf(`
		SELECT t.id, t.tx_no, COALESCE(a.name, '') as agent_name, t.type, t.amount,
		       t.balance_after, t.remark, t.created_at
		FROM transactions t
		LEFT JOIN agents a ON a.id = t.subject_id AND t.subject_type = 'agent'
		WHERE %s
		ORDER BY t.created_at DESC
		LIMIT ? OFFSET ?
	`, whereSQL)

	listArgs := append(args, pageSize, offset)
	rows, err := db.Query(listSQL, listArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	typeLabels := map[string]string{"recharge": "充值", "consume": "消费", "refund": "退款", "purchase": "购买"}

	type txItem struct {
		ID           int64   `json:"id"`
		OrderNo      string  `json:"orderNo"`
		AgentName    string  `json:"agentName"`
		Type         string  `json:"type"`
		TypeLabel    string  `json:"typeLabel"`
		Amount       float64 `json:"amount"`
		BalanceAfter float64 `json:"balanceAfter"`
		Remark       string  `json:"remark"`
		CreatedAt    string  `json:"createdAt"`
	}

	var list []txItem
	for rows.Next() {
		var item txItem
		var balanceAfter sql.NullFloat64
		var remark sql.NullString
		var createdAt time.Time
		err := rows.Scan(&item.ID, &item.OrderNo, &item.AgentName, &item.Type,
			&item.Amount, &balanceAfter, &remark, &createdAt)
		if err != nil {
			continue
		}
		item.TypeLabel = typeLabels[item.Type]
		if balanceAfter.Valid {
			item.BalanceAfter = balanceAfter.Float64
		}
		if remark.Valid {
			item.Remark = remark.String
		}
		item.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		list = append(list, item)
	}
	if list == nil {
		list = []txItem{}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{"list": list, "total": total}})
}

// TransactionStats 财务统计
func TransactionStats(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02 00:00:00")

	var totalRecharge, totalConsume, monthRecharge, monthConsume float64

	db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE type = 'recharge'").Scan(&totalRecharge)
	db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE type = 'consume'").Scan(&totalConsume)
	db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE type = 'recharge' AND created_at >= ?", monthStart).Scan(&monthRecharge)
	db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE type = 'consume' AND created_at >= ?", monthStart).Scan(&monthConsume)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"totalRecharge": totalRecharge,
			"totalConsume":  totalConsume,
			"monthRecharge": monthRecharge,
			"monthConsume":  monthConsume,
		},
	})
}

// AgentSelectList 代理商下拉列表（用于筛选）
func AgentSelectList(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, name FROM agents ORDER BY id ASC")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	defer rows.Close()

	type item struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	var list []item
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.ID, &i.Name); err == nil {
			list = append(list, i)
		}
	}
	if list == nil {
		list = []item{}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": list})
}