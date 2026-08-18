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

// QuotaList 开码配额列表
func QuotaList(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	agentId := c.Query("agentId")
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

	if agentId != "" {
		where = append(where, "q.agent_id = ?")
		args = append(args, agentId)
	}
	if appId != "" {
		where = append(where, "q.app_id = ?")
		args = append(args, appId)
	}

	whereSQL := strings.Join(where, " AND ")

	var total int64
	db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM agent_quotas q WHERE %s", whereSQL), args...).Scan(&total)

	offset := (page - 1) * pageSize
	listSQL := fmt.Sprintf(`
		SELECT q.id, q.agent_id, COALESCE(ag.name, '') as agent_name,
		       q.app_id, COALESCE(a.app_name, '') as app_name,
		       q.total, q.used, q.price, q.updated_at
		FROM agent_quotas q
		LEFT JOIN agents ag ON ag.id = q.agent_id
		LEFT JOIN apps a ON a.id = q.app_id
		WHERE %s
		ORDER BY q.updated_at DESC
		LIMIT ? OFFSET ?
	`, whereSQL)

	listArgs := append(args, pageSize, offset)
	rows, err := db.Query(listSQL, listArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	type quotaItem struct {
		ID         int64   `json:"id"`
		AgentID    int64   `json:"agentId"`
		AgentName  string  `json:"agentName"`
		AppID      int64   `json:"appId"`
		AppName    string  `json:"appName"`
		TotalQuota int     `json:"totalQuota"`
		UsedQuota  int     `json:"usedQuota"`
		Price      float64 `json:"price"`
		UpdatedAt  string  `json:"updatedAt"`
	}

	var list []quotaItem
	for rows.Next() {
		var item quotaItem
		var updatedAt time.Time
		err := rows.Scan(&item.ID, &item.AgentID, &item.AgentName, &item.AppID, &item.AppName,
			&item.TotalQuota, &item.UsedQuota, &item.Price, &updatedAt)
		if err != nil {
			continue
		}
		item.UpdatedAt = updatedAt.Format("2006-01-02 15:04")
		list = append(list, item)
	}
	if list == nil {
		list = []quotaItem{}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{"list": list, "total": total}})
}

const (
	quotaTotalMin = 0
	quotaTotalMax = 999999
	quotaPriceMin = 0
	quotaPriceMax = 999999
)

// clampQuotaTotal 总配额落在 [0, 999999]，负数按 0 处理。
func clampQuotaTotal(total int) int {
	if total < quotaTotalMin {
		return quotaTotalMin
	}
	if total > quotaTotalMax {
		return quotaTotalMax
	}
	return total
}

// clampQuotaPrice 配额单价仅作记录字段，允许 0，落在 [0, 999999]。
func clampQuotaPrice(price float64) float64 {
	if price < quotaPriceMin {
		return quotaPriceMin
	}
	if price > quotaPriceMax {
		return quotaPriceMax
	}
	return price
}

// QuotaCreate 分配配额
func QuotaCreate(c *gin.Context) {
	var req struct {
		AgentID    int64   `json:"agentId" binding:"required"`
		AppID      int64   `json:"appId" binding:"required"`
		TotalQuota int     `json:"totalQuota"`
		Price      float64 `json:"price"`
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

	total := clampQuotaTotal(req.TotalQuota)
	price := clampQuotaPrice(req.Price)

	result, err := db.Exec(`
		INSERT INTO agent_quotas (agent_id, app_id, total, used, price)
		VALUES (?, ?, ?, 0, ?)
		ON DUPLICATE KEY UPDATE total = VALUES(total), price = VALUES(price)
	`, req.AgentID, req.AppID, total, price)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "分配失败: " + err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "分配成功", "data": gin.H{"id": id}})
}

// QuotaUpdate 调整配额
func QuotaUpdate(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		TotalQuota int     `json:"totalQuota"`
		Price      float64 `json:"price"`
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

	_, err = db.Exec("UPDATE agent_quotas SET total = ?, price = ? WHERE id = ?", clampQuotaTotal(req.TotalQuota), clampQuotaPrice(req.Price), id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "调整成功"})
}

// QuotaDelete 移除配额
func QuotaDelete(c *gin.Context) {
	id := c.Param("id")

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	_, err = db.Exec("DELETE FROM agent_quotas WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "已移除"})
}