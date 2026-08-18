package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"auto_pro/config"
	"auto_pro/middleware"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

type loginRequest struct {
	UserName string `json:"userName"`
	Password string `json:"password"`
}

// Login 管理员登录
func Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	if remaining := middleware.LoginLockRemaining(c.ClientIP(), req.UserName); remaining > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 429, "message": fmt.Sprintf("登录尝试次数过多，请 %d 秒后重试", int(remaining.Seconds())+1)})
		return
	}

	cfg, err := config.LoadDBConfig()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "系统未配置"})
		return
	}

	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "数据库连接失败"})
		return
	}
	defer db.Close()

	var id uint
	var passwordHash string
	err = db.QueryRow("SELECT id, password_hash FROM admins WHERE username = ? AND enabled = 1", req.UserName).Scan(&id, &passwordHash)
	if err != nil {
		middleware.RecordLoginFailure(c.ClientIP(), req.UserName)
		c.JSON(http.StatusOK, gin.H{"code": 401, "message": "账号或密码错误"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		middleware.RecordLoginFailure(c.ClientIP(), req.UserName)
		c.JSON(http.StatusOK, gin.H{"code": 401, "message": "账号或密码错误"})
		return
	}

	middleware.RecordLoginSuccess(c.ClientIP(), req.UserName)

	// 更新最后登录时间
	_, _ = db.Exec("UPDATE admins SET last_login_at = NOW(), last_login_ip = ? WHERE id = ?", c.ClientIP(), id)

	// 生成 token
	now := time.Now()
	claims := &middleware.Claims{
		UserID:   id,
		Username: req.UserName,
		Role:     "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(middleware.JWTSecret())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "生成token失败"})
		return
	}

	// refresh token (7天)
	refreshClaims := &middleware.Claims{
		UserID:   id,
		Username: req.UserName,
		Role:     "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	refreshToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(middleware.JWTSecret())

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "登录成功",
		"data": gin.H{
			"token":        token,
			"refreshToken": refreshToken,
		},
	})
}
