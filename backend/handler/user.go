package handler

import (
	"database/sql"
	"net/http"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

// GetUserInfo 获取当前登录用户信息
func GetUserInfo(c *gin.Context) {
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")

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

	var email, avatar, nickname string
	var roleID sql.NullInt64
	err = db.QueryRow("SELECT email, avatar, nickname, role_id FROM admins WHERE id = ?", userID).Scan(&email, &avatar, &nickname, &roleID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询用户失败"})
		return
	}

	// 查询角色编码
	roles := []string{}
	if roleID.Valid {
		var roleCode string
		err = db.QueryRow("SELECT role_code FROM roles WHERE id = ?", roleID.Int64).Scan(&roleCode)
		if err == nil {
			roles = append(roles, roleCode)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"userId":   userID,
			"userName": username,
			"email":    email,
			"avatar":   avatar,
			"roles":    roles,
			"buttons":  []string{},
		},
	})
}

type changePasswordRequest struct {
	OldPassword     string `json:"oldPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=6"`
	ConfirmPassword string `json:"confirmPassword" binding:"required"`
}

// ChangePassword 管理员修改密码
func ChangePassword(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误，密码至少6位"})
		return
	}

	if req.NewPassword != req.ConfirmPassword {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "两次输入的新密码不一致"})
		return
	}

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

	// 查询当前密码哈希
	var passwordHash string
	err = db.QueryRow("SELECT password_hash FROM admins WHERE id = ?", userID).Scan(&passwordHash)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询用户失败"})
		return
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "当前密码错误"})
		return
	}

	// 生成新密码哈希
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "密码加密失败"})
		return
	}

	// 更新密码
	_, err = db.Exec("UPDATE admins SET password_hash = ? WHERE id = ?", string(newHash), userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "修改密码失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "密码修改成功"})
}