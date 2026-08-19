package middleware

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"sync"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
)

var (
	cachedSecret    []byte
	ephemeralSecret []byte
	secretStateMu   sync.Mutex
)

// JWTSecret 返回签名密钥。优先使用安装时持久化的密钥；
// 读取失败时退化为进程内随机密钥（重启后旧 token 失效，但不会泄露硬编码密钥）。
func JWTSecret() []byte {
	secretStateMu.Lock()
	defer secretStateMu.Unlock()

	if cachedSecret != nil {
		return cachedSecret
	}
	if secret, err := config.LoadOrCreateJWTSecret(); err == nil {
		cachedSecret = secret
		return cachedSecret
	} else {
		log.Printf("jwt secret unavailable (%v), falling back to ephemeral secret", err)
	}
	if ephemeralSecret == nil {
		raw := make([]byte, 48)
		_, _ = rand.Read(raw)
		ephemeralSecret = []byte(hex.EncodeToString(raw))
	}
	return ephemeralSecret
}

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未提供认证信息"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "认证格式错误"})
			c.Abort()
			return
		}

		token, err := jwt.ParseWithClaims(parts[1], &Claims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return JWTSecret(), nil
		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "认证已过期或无效"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "认证信息解析失败"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func activeUserRejection(enabled bool, accountStatus string, convertedAgentID sql.NullInt64) (string, bool) {
	if accountStatus == "converted" || convertedAgentID.Valid {
		return "该账号已升级为代理，请前往代理端登录", true
	}
	if !enabled {
		return "用户账户已禁用", false
	}
	return "", false
}

// RequireActiveUser rejects stale user tokens immediately after account conversion.
func RequireActiveUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("role") != "user" {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "无权限访问用户端"})
			c.Abort()
			return
		}

		cfg, err := config.LoadDBConfig()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "用户状态校验失败"})
			c.Abort()
			return
		}
		db, err := sql.Open("mysql", config.GetDSN(cfg))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "用户状态校验失败"})
			c.Abort()
			return
		}
		defer db.Close()

		var enabled bool
		var accountStatus string
		var convertedAgentID sql.NullInt64
		err = db.QueryRow(`
			SELECT enabled, account_status, converted_agent_id
			FROM users WHERE id = ?
		`, c.GetUint("user_id")).Scan(&enabled, &accountStatus, &convertedAgentID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "用户账户不存在或已失效"})
			c.Abort()
			return
		}
		message, converted := activeUserRejection(enabled, accountStatus, convertedAgentID)
		if message != "" {
			response := gin.H{"code": 401, "message": message}
			if converted {
				response["data"] = gin.H{"converted": true, "agentId": convertedAgentID.Int64}
			}
			c.JSON(http.StatusUnauthorized, response)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireAdmin 仅允许管理员角色访问，需置于 JWTAuth 之后。
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("role") != "admin" {
			c.JSON(http.StatusOK, gin.H{"code": 403, "message": "无权限访问"})
			c.Abort()
			return
		}
		c.Next()
	}
}
