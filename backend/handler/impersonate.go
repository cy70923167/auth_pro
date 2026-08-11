package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"auto_pro/config"
	"auto_pro/middleware"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// signImpersonateToken 为指定用户/代理签发对应端 token，与普通登录 token 结构一致。
func signImpersonateToken(id uint, username, role string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := &middleware.Claims{
		UserID:   id,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(middleware.JWTSecret())
}

// AdminImpersonateUser 管理员代登录用户账号（无需密码）。
// 仅签发用户端 token，不影响当前管理员会话。
func AdminImpersonateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
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

	var email, nickname string
	var enabled bool
	err = db.QueryRow("SELECT email, nickname, enabled FROM users WHERE id = ?", id).
		Scan(&email, &nickname, &enabled)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "用户不存在"})
		return
	}
	if !enabled {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "该账号已被禁用，无法登录"})
		return
	}

	token, err := signImpersonateToken(uint(id), email, "user", 7*24*time.Hour)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "生成token失败"})
		return
	}

	_, _ = db.Exec("UPDATE users SET last_login_at = NOW(), last_login_ip = ? WHERE id = ?", c.ClientIP(), id)

	c.JSON(http.StatusOK, gin.H{
		"code": 200, "msg": "ok",
		"data": gin.H{
			"accessToken": token,
			"userId":      id,
			"email":       email,
			"nickname":    nickname,
		},
	})
}

// AdminImpersonateAgent 管理员代登录代理商账号（无需密码）。
func AdminImpersonateAgent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
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

	var email, name string
	var balance float64
	var enabled bool
	err = db.QueryRow("SELECT email, name, balance, enabled FROM agents WHERE id = ?", id).
		Scan(&email, &name, &balance, &enabled)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "代理商不存在"})
		return
	}
	if !enabled {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "该代理商已被冻结，无法登录"})
		return
	}

	token, err := signImpersonateToken(uint(id), email, "agent", 7*24*time.Hour)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "生成token失败"})
		return
	}

	_, _ = db.Exec("UPDATE agents SET last_login_at = NOW(), last_login_ip = ? WHERE id = ?", c.ClientIP(), id)

	c.JSON(http.StatusOK, gin.H{
		"code": 200, "msg": "ok",
		"data": gin.H{
			"accessToken": token,
			"agentId":     id,
			"email":       email,
			"name":        name,
			"balance":     balance,
		},
	})
}
