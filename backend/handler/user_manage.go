package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// AdminUserList 管理端-用户列表
func AdminUserList(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	keyword := c.Query("keyword")
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("current", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	where := "WHERE 1=1"
	args := []interface{}{}

	if keyword != "" {
		where += " AND (email LIKE ? OR nickname LIKE ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if status == "enabled" {
		where += " AND enabled = 1"
	} else if status == "disabled" {
		where += " AND enabled = 0"
	}

	var total int
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM users %s", where)
	db.QueryRow(countSQL, args...).Scan(&total)

	querySQL := fmt.Sprintf(`
		SELECT id, email, nickname, balance, enabled, last_login_at, last_login_ip, created_at
		FROM users %s ORDER BY id DESC LIMIT ? OFFSET ?
	`, where)
	args = append(args, pageSize, offset)

	rows, err := db.Query(querySQL, args...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	type userItem struct {
		ID          int64   `json:"userId"`
		Email       string  `json:"userEmail"`
		Nickname    string  `json:"userName"`
		Balance     float64 `json:"balance"`
		Enabled     bool    `json:"enabled"`
		Status      string  `json:"status"`
		LastLoginAt string  `json:"lastLoginAt"`
		LastLoginIP string  `json:"lastLoginIp"`
		CreatedAt   string  `json:"createTime"`
	}

	var list []userItem
	for rows.Next() {
		var item userItem
		var lastLoginAt, createdAt sql.NullTime
		var lastLoginIP sql.NullString
		var enabled int

		rows.Scan(&item.ID, &item.Email, &item.Nickname, &item.Balance, &enabled,
			&lastLoginAt, &lastLoginIP, &createdAt)

		item.Enabled = enabled == 1
		if item.Enabled {
			item.Status = "1"
		} else {
			item.Status = "4"
		}

		if lastLoginAt.Valid {
			item.LastLoginAt = lastLoginAt.Time.Format("2006-01-02 15:04:05")
		}
		if lastLoginIP.Valid {
			item.LastLoginIP = lastLoginIP.String
		}
		if createdAt.Valid {
			item.CreatedAt = createdAt.Time.Format("2006-01-02 15:04:05")
		}

		list = append(list, item)
	}
	if list == nil {
		list = []userItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200, "msg": "",
		"data": gin.H{
			"records": list,
			"current": page,
			"size":    pageSize,
			"total":   total,
		},
	})
}

// AdminUserCreate 管理端-创建用户
func AdminUserCreate(c *gin.Context) {
	type createReq struct {
		Email    string   `json:"email" binding:"required,email"`
		Nickname string   `json:"nickname" binding:"required"`
		Password string   `json:"password" binding:"required,min=6"`
		Balance  *float64 `json:"balance"`
	}
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误，请检查邮箱格式和密码长度"})
		return
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var exists int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", req.Email).Scan(&exists)
	if exists > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该邮箱已注册"})
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	
	balance := 0.0
	if req.Balance != nil {
		balance = *req.Balance
	}
	
	_, err = db.Exec("INSERT INTO users (email, password_hash, nickname, balance) VALUES (?, ?, ?, ?)",
		req.Email, string(hash), req.Nickname, balance)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "创建成功"})
}

// AdminUserUpdate 管理端-更新用户
func AdminUserUpdate(c *gin.Context) {
	id := c.Param("id")

	type updateReq struct {
		Nickname string   `json:"nickname"`
		Email    string   `json:"email"`
		Password string   `json:"password"`
		Balance  *float64 `json:"balance"`
	}
	var req updateReq
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

	sets := ""
	args := []interface{}{}

	if req.Nickname != "" {
		sets += "nickname = ?"
		args = append(args, req.Nickname)
	}
	if req.Email != "" {
		var exists int
		db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ? AND id != ?", req.Email, id).Scan(&exists)
		if exists > 0 {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "邮箱已被使用"})
			return
		}
		if sets != "" {
			sets += ", "
		}
		sets += "email = ?"
		args = append(args, req.Email)
	}
	if req.Balance != nil {
		if sets != "" {
			sets += ", "
		}
		sets += "balance = ?"
		args = append(args, *req.Balance)
	}
	if req.Password != "" {
		if len(req.Password) < 6 {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "密码至少6位"})
			return
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if sets != "" {
			sets += ", "
		}
		sets += "password_hash = ?"
		args = append(args, string(hash))
	}

	if sets == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "无可更新字段"})
		return
	}

	args = append(args, id)
	_, err = db.Exec("UPDATE users SET "+sets+" WHERE id = ?", args...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "更新成功"})
}

// AdminUserToggle 管理端-启用/禁用用户
func AdminUserToggle(c *gin.Context) {
	id := c.Param("id")

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	_, err = db.Exec("UPDATE users SET enabled = 1 - enabled WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "操作失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "操作成功"})
}

// AdminUserDelete 管理端-删除用户
func AdminUserDelete(c *gin.Context) {
	id := c.Param("id")

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	_, err = db.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "删除成功"})
}