package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
)

// RoleList 角色列表
func RoleList(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	keyword := c.Query("roleName")
	roleCode := c.Query("roleCode")
	enabled := c.Query("enabled")
	page, _ := strconv.Atoi(c.DefaultQuery("current", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	where := []string{"1=1"}
	args := []any{}

	if keyword != "" {
		where = append(where, "role_name LIKE ?")
		args = append(args, "%"+keyword+"%")
	}
	if roleCode != "" {
		where = append(where, "role_code LIKE ?")
		args = append(args, "%"+roleCode+"%")
	}
	if enabled != "" {
		if enabled == "true" || enabled == "1" {
			where = append(where, "enabled = 1")
		} else {
			where = append(where, "enabled = 0")
		}
	}
// PLACEHOLDER_ROLE_LIST

	whereSQL := strings.Join(where, " AND ")

	var total int
	db.QueryRow("SELECT COUNT(*) FROM roles WHERE "+whereSQL, args...).Scan(&total)

	offset := (page - 1) * pageSize
	querySQL := fmt.Sprintf("SELECT id, role_name, role_code, description, discount, enabled, created_at FROM roles WHERE %s ORDER BY id ASC LIMIT ? OFFSET ?", whereSQL)
	queryArgs := append(args, pageSize, offset)

	rows, err := db.Query(querySQL, queryArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	list := []gin.H{}
	for rows.Next() {
		var id int
		var roleName, roleCode, description string
		var discount float64
		var enabledVal bool
		var createdAt sql.NullTime
		rows.Scan(&id, &roleName, &roleCode, &description, &discount, &enabledVal, &createdAt)

		ct := ""
		if createdAt.Valid {
			ct = createdAt.Time.Format("2006-01-02 15:04:05")
		}

		list = append(list, gin.H{
			"roleId":      id,
			"roleName":    roleName,
			"roleCode":    roleCode,
			"description": description,
			"discount":    discount,
			"enabled":     enabledVal,
			"createTime":  ct,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"records": list,
			"current": page,
			"size":    pageSize,
			"total":   total,
		},
	})
}

// RoleCreate 创建角色
func RoleCreate(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var req struct {
		RoleName       string   `json:"roleName"`
		RoleCode       string   `json:"roleCode"`
		Description    string   `json:"description"`
		Discount       float64  `json:"discount"`
		Enabled        bool     `json:"enabled"`
		AppPermissions []string `json:"appPermissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if req.RoleName == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "角色名称不能为空"})
		return
	}
	if req.RoleCode == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "角色编码不能为空"})
		return
	}
	if req.Discount <= 0 || req.Discount > 10 {
		req.Discount = 10.0
	}

	enabledInt := 0
	if req.Enabled {
		enabledInt = 1
	}

	result, err := db.Exec("INSERT INTO roles (role_name, role_code, description, discount, enabled) VALUES (?, ?, ?, ?, ?)",
		req.RoleName, req.RoleCode, req.Description, req.Discount, enabledInt)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "角色编码已存在"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建失败: " + err.Error()})
		return
	}

	roleID, _ := result.LastInsertId()

	// 分配所有菜单权限给新角色（默认全部）
	rows, _ := db.Query("SELECT id FROM menus")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var menuID int
			rows.Scan(&menuID)
			db.Exec("INSERT IGNORE INTO role_menus (role_id, menu_id) VALUES (?, ?)", roleID, menuID)
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "创建成功", "data": gin.H{"id": roleID}})
}

// RoleUpdate 更新角色
func RoleUpdate(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	id := c.Param("id")
	var req struct {
		RoleName    string  `json:"roleName"`
		Description string  `json:"description"`
		Discount    float64 `json:"discount"`
		Enabled     bool    `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	if req.Discount <= 0 || req.Discount > 10 {
		req.Discount = 10.0
	}

	enabledInt := 0
	if req.Enabled {
		enabledInt = 1
	}

	_, err = db.Exec("UPDATE roles SET role_name=?, description=?, discount=?, enabled=? WHERE id=?",
		req.RoleName, req.Description, req.Discount, enabledInt, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "更新成功"})
}

// RoleDelete 删除角色
func RoleDelete(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	id := c.Param("id")

	// 不允许删除超级管理员角色
	var roleCode string
	db.QueryRow("SELECT role_code FROM roles WHERE id=?", id).Scan(&roleCode)
	if roleCode == "R_SUPER" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "不能删除超级管理员角色"})
		return
	}

	// 检查是否有用户关联
	var count int
	db.QueryRow("SELECT COUNT(*) FROM admins WHERE role_id=?", id).Scan(&count)
	if count > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该角色下还有用户，无法删除"})
		return
	}

	db.Exec("DELETE FROM role_menus WHERE role_id=?", id)
	db.Exec("DELETE FROM roles WHERE id=?", id)

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "删除成功"})
}

// RoleMenus 获取角色的菜单权限
func RoleMenus(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	id := c.Param("id")

	rows, err := db.Query("SELECT menu_id FROM role_menus WHERE role_id=?", id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	defer rows.Close()

	menuIDs := []int{}
	for rows.Next() {
		var mid int
		rows.Scan(&mid)
		menuIDs = append(menuIDs, mid)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": menuIDs})
}

// RoleUpdateMenus 更新角色菜单权限
func RoleUpdateMenus(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	id := c.Param("id")
	var req struct {
		MenuIDs []int `json:"menuIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	// 清除旧权限
	db.Exec("DELETE FROM role_menus WHERE role_id=?", id)

	// 插入新权限
	for _, menuID := range req.MenuIDs {
		db.Exec("INSERT INTO role_menus (role_id, menu_id) VALUES (?, ?)", id, menuID)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "权限保存成功"})
}