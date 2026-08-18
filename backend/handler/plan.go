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

// PlanList 套餐列表
func PlanList(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensurePlanLicenseType(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化套餐授权方式失败: " + err.Error()})
		return
	}

	appID := c.Query("appId")
	keyword := c.Query("keyword")
	status := c.Query("status")

	where := []string{"1=1"}
	args := []any{}
	if appID != "" {
		where = append(where, "p.app_id = ?")
		args = append(args, appID)
	}
	if keyword != "" {
		where = append(where, "(p.name LIKE ? OR a.app_name LIKE ?)")
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if status == "enabled" {
		where = append(where, "p.enabled = 1")
	} else if status == "disabled" {
		where = append(where, "p.enabled = 0")
	}

	query := fmt.Sprintf(`
		SELECT p.id, p.app_id, a.app_name, p.name, p.license_type, p.duration_days, p.price, p.sort,
		       p.enabled, p.remark, p.created_at
		FROM license_plans p
		LEFT JOIN apps a ON a.id = p.app_id
		WHERE %s
		ORDER BY a.id ASC, p.sort ASC, p.id ASC
	`, strings.Join(where, " AND "))

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	type planItem struct {
		ID           int64   `json:"id"`
		AppID        int64   `json:"appId"`
		AppName      string  `json:"appName"`
		Name         string  `json:"name"`
		LicenseType  string  `json:"licenseType"`
		DurationDays int     `json:"durationDays"`
		DurationText string  `json:"durationText"`
		Price        float64 `json:"price"`
		Sort         int     `json:"sort"`
		Enabled      bool    `json:"enabled"`
		Remark       string  `json:"remark"`
		CreatedAt    string  `json:"createdAt"`
	}

	var list []planItem
	for rows.Next() {
		var item planItem
		var enabled int
		var remark sql.NullString
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.AppID, &item.AppName, &item.Name, &item.LicenseType, &item.DurationDays,
			&item.Price, &item.Sort, &enabled, &remark, &createdAt); err == nil {
			item.Enabled = enabled == 1
			if item.DurationDays == 0 {
				item.DurationText = "永久"
			} else {
				item.DurationText = fmt.Sprintf("%d天", item.DurationDays)
			}
			if remark.Valid {
				item.Remark = remark.String
			}
			item.CreatedAt = createdAt.Format("2006-01-02 15:04")
			list = append(list, item)
		}
	}
	if list == nil {
		list = []planItem{}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": list})
}

// PlanCreate 新增套餐
func PlanCreate(c *gin.Context) {
	var req struct {
		AppID        int64   `json:"appId" binding:"required"`
		Name         string  `json:"name" binding:"required"`
		LicenseType  string  `json:"licenseType"`
		DurationDays int     `json:"durationDays"`
		Price        float64 `json:"price"`
		Sort         int     `json:"sort"`
		Enabled      bool    `json:"enabled"`
		Remark       string  `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if req.Price < 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "价格不能小于0"})
		return
	}
	req.LicenseType = strings.ToLower(strings.TrimSpace(req.LicenseType))
	if !validPlanLicenseType(req.LicenseType) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "无效的适用授权方式"})
		return
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensurePlanLicenseType(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化套餐授权方式失败: " + err.Error()})
		return
	}

	var appExists int
	db.QueryRow("SELECT COUNT(*) FROM apps WHERE id = ?", req.AppID).Scan(&appExists)
	if appExists == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "应用不存在"})
		return
	}

	result, err := db.Exec(`
		INSERT INTO license_plans (app_id, name, license_type, duration_days, price, sort, enabled, remark)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, req.AppID, req.Name, req.LicenseType, req.DurationDays, req.Price, req.Sort, req.Enabled, req.Remark)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建失败: " + err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "创建成功", "data": gin.H{"id": id}})
}

// PlanUpdate 编辑套餐
func PlanUpdate(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		AppID        int64   `json:"appId" binding:"required"`
		Name         string  `json:"name" binding:"required"`
		LicenseType  string  `json:"licenseType"`
		DurationDays int     `json:"durationDays"`
		Price        float64 `json:"price"`
		Sort         int     `json:"sort"`
		Enabled      bool    `json:"enabled"`
		Remark       string  `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if req.Price < 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "价格不能小于0"})
		return
	}
	req.LicenseType = strings.ToLower(strings.TrimSpace(req.LicenseType))
	if !validPlanLicenseType(req.LicenseType) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "无效的适用授权方式"})
		return
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensurePlanLicenseType(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化套餐授权方式失败: " + err.Error()})
		return
	}

	_, err = db.Exec(`
		UPDATE license_plans
		SET app_id = ?, name = ?, license_type = ?, duration_days = ?, price = ?, sort = ?, enabled = ?, remark = ?
		WHERE id = ?
	`, req.AppID, req.Name, req.LicenseType, req.DurationDays, req.Price, req.Sort, req.Enabled, req.Remark, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "更新成功"})
}

// PlanToggle 启用/禁用套餐
func PlanToggle(c *gin.Context) {
	id := c.Param("id")

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	_, err = db.Exec("UPDATE license_plans SET enabled = 1 - enabled WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "操作失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "操作成功"})
}

// PlanDelete 删除套餐
func PlanDelete(c *gin.Context) {
	id := c.Param("id")

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	planID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || planID <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "套餐ID错误"})
		return
	}

	_, err = db.Exec("DELETE FROM license_plans WHERE id = ?", planID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "删除成功"})
}