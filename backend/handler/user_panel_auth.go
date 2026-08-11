package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// 用户端认证增强：
//   - users.phone 手机号登录（注册选填、资料页可改）
//   - user_password_resets 忘记密码重置令牌（仅存哈希，30 分钟有效，一次性）
//   - user_email_codes 注册邮箱验证码（6 位数字，仅存哈希，10 分钟有效，错 5 次作废）

const (
	passwordResetTokenTTL      = 30 * time.Minute
	passwordResetSendInterval  = 60 * time.Second
	passwordResetTokenByteSize = 32

	emailCodeTTL           = 10 * time.Minute
	emailCodeMaxAttempts   = 5
	emailCodeSceneRegister = "register"
)

var phoneRegexp = regexp.MustCompile(`^1\d{10}$`)

// ensureUserAuthStorage 幂等建列/建表：
//   - users.phone 手机号列（唯一索引，NULL 不冲突，未绑定手机号的用户不受影响）
//   - user_password_resets 密码重置令牌表
func ensureUserAuthStorage(db *sql.DB) error {
	var phoneCount int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'phone'
	`).Scan(&phoneCount); err != nil {
		return err
	}
	if phoneCount == 0 {
		if _, err := db.Exec("ALTER TABLE users ADD COLUMN phone VARCHAR(20) DEFAULT NULL COMMENT '手机号' AFTER email"); err != nil {
			return err
		}
	}

	var idxCount int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND INDEX_NAME = 'uk_phone'
	`).Scan(&idxCount); err != nil {
		return err
	}
	if idxCount == 0 {
		// 历史脏数据兜底：清理空串为 NULL，避免唯一索引冲突
		_, _ = db.Exec("UPDATE users SET phone = NULL WHERE phone = ''")
		if _, err := db.Exec("ALTER TABLE users ADD UNIQUE KEY uk_phone (phone)"); err != nil {
			return err
		}
	}

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS user_password_resets (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
		user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
		email VARCHAR(100) NOT NULL COMMENT '申请邮箱',
		token_hash CHAR(64) NOT NULL COMMENT '令牌SHA256',
		expires_at DATETIME NOT NULL COMMENT '过期时间',
		used_at DATETIME DEFAULT NULL COMMENT '使用时间',
		created_ip VARCHAR(45) NOT NULL DEFAULT '' COMMENT '申请IP',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
		PRIMARY KEY (id),
		KEY idx_token_hash (token_hash),
		KEY idx_email_created (email, created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户密码重置令牌表'`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS user_email_codes (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
		email VARCHAR(100) NOT NULL COMMENT '目标邮箱',
		scene VARCHAR(20) NOT NULL DEFAULT 'register' COMMENT '场景(register)',
		code_hash CHAR(64) NOT NULL COMMENT '验证码SHA256',
		attempts INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '失败尝试次数',
		expires_at DATETIME NOT NULL COMMENT '过期时间',
		used_at DATETIME DEFAULT NULL COMMENT '使用时间',
		created_ip VARCHAR(45) NOT NULL DEFAULT '' COMMENT '申请IP',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
		PRIMARY KEY (id),
		KEY idx_email_scene (email, scene, created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户邮箱验证码表'`)
	return err
}

// ========== 忘记密码 ==========

// 邮件类发送频率限制（内存态，与 login_guard 同生命周期，重启清零）。
// key 带场景前缀，忘记密码与注册验证码独立计频；
// 对存在与不存在的邮箱一致限频，避免通过"是否被限频"探测邮箱注册状态。
var emailSendMu sync.Mutex
var emailSendLast = map[string]time.Time{}

func emailSendLimited(scene, email string) bool {
	emailSendMu.Lock()
	defer emailSendMu.Unlock()
	key := scene + "|" + email
	last, ok := emailSendLast[key]
	if ok && time.Since(last) < passwordResetSendInterval {
		return true
	}
	emailSendLast[key] = time.Now()
	return false
}

func clearEmailSendLimit(scene, email string) {
	emailSendMu.Lock()
	delete(emailSendLast, scene+"|"+email)
	emailSendMu.Unlock()
}

// ========== 注册邮箱验证码 ==========

type userRegisterEmailCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
}

var (
	errEmailCodeInvalid   = errors.New("邮箱验证码无效或已过期，请重新获取")
	errEmailCodeIncorrect = errors.New("邮箱验证码错误")
)

func generateEmailCode() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", binary.BigEndian.Uint32(buf)%1000000), nil
}

func hashEmailCode(email, scene, code string) string {
	sum := sha256.Sum256([]byte(email + "|" + scene + "|" + code))
	return hex.EncodeToString(sum[:])
}

// UserSendRegisterEmailCode 发送注册邮箱验证码。
// 前端滑块只负责阻止无意点击；真正的限频、邮箱占用检查和验证码生成均在服务端完成。
func UserSendRegisterEmailCode(c *gin.Context) {
	if !isRegistrationEnabled() {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "普通用户注册已关闭，请联系管理员"})
		return
	}

	var req userRegisterEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请输入有效的邮箱地址"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))

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

	if err := ensureUserAuthStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化失败"})
		return
	}

	var exists int
	if err := db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", email).Scan(&exists); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "检查邮箱失败"})
		return
	}
	if exists > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该邮箱已注册"})
		return
	}

	mailCfg, err := loadMailConfig(db, true)
	if err != nil || validateMailConfig(mailCfg, true) != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "邮件服务未配置，请联系管理员"})
		return
	}
	if emailSendLimited(emailCodeSceneRegister, email) {
		c.JSON(http.StatusOK, gin.H{"code": 429, "msg": "发送过于频繁，请 1 分钟后再试"})
		return
	}

	code, err := generateEmailCode()
	if err != nil {
		clearEmailSendLimit(emailCodeSceneRegister, email)
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "生成验证码失败"})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		clearEmailSendLimit(emailCodeSceneRegister, email)
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建验证码失败"})
		return
	}
	if _, err := tx.Exec(
		"UPDATE user_email_codes SET used_at = NOW() WHERE email = ? AND scene = ? AND used_at IS NULL",
		email, emailCodeSceneRegister,
	); err != nil {
		_ = tx.Rollback()
		clearEmailSendLimit(emailCodeSceneRegister, email)
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建验证码失败"})
		return
	}
	result, err := tx.Exec(
		"INSERT INTO user_email_codes (email, scene, code_hash, expires_at, created_ip) VALUES (?, ?, ?, DATE_ADD(NOW(), INTERVAL ? SECOND), ?)",
		email, emailCodeSceneRegister, hashEmailCode(email, emailCodeSceneRegister, code), int(emailCodeTTL.Seconds()), c.ClientIP(),
	)
	if err != nil {
		_ = tx.Rollback()
		clearEmailSendLimit(emailCodeSceneRegister, email)
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建验证码失败"})
		return
	}
	codeID, err := result.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		clearEmailSendLimit(emailCodeSceneRegister, email)
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建验证码失败"})
		return
	}
	if err := tx.Commit(); err != nil {
		clearEmailSendLimit(emailCodeSceneRegister, email)
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建验证码失败"})
		return
	}

	siteName := loadSiteNameForMail(db)
	subject := fmt.Sprintf("%s 注册邮箱验证", siteName)
	content := fmt.Sprintf(
		"您好：\n\n您正在注册 %s，邮箱验证码为：%s\n\n验证码 %d 分钟内有效，请勿泄露给他人。\n\n如果这不是您的操作，请忽略本邮件。",
		siteName, code, int(emailCodeTTL.Minutes()),
	)
	logContent := fmt.Sprintf("注册邮箱验证码已发送，有效期 %d 分钟（验证码内容已隐藏）", int(emailCodeTTL.Minutes()))
	logID, logged := createMailLog(db, "register_email_code", "user", 0, 0, email, subject, logContent, 0, nil)
	if err := sendSMTPMail(mailCfg, mailMessage{To: email, Subject: subject, Content: content, ContentType: "text"}); err != nil {
		_, _ = db.Exec("UPDATE user_email_codes SET used_at = NOW() WHERE id = ?", codeID)
		clearEmailSendLimit(emailCodeSceneRegister, email)
		if logged {
			markMailLogFailed(db, logID, err.Error())
		}
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "验证码发送失败，请稍后重试"})
		return
	}
	if logged {
		markMailLogSent(db, logID)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "验证码已发送，请查收邮件", "data": gin.H{"expiresIn": int(emailCodeTTL.Seconds())}})
}

// consumeRegisterEmailCode 在注册事务中校验并占用验证码。
// 校验失败会累计次数，达到上限后当前验证码立即作废。
func consumeRegisterEmailCode(tx *sql.Tx, email, code string) error {
	var id uint64
	var storedHash string
	var attempts int
	err := tx.QueryRow(`
		SELECT id, code_hash, attempts FROM user_email_codes
		WHERE email = ? AND scene = ? AND used_at IS NULL AND expires_at > NOW()
		ORDER BY id DESC LIMIT 1 FOR UPDATE
	`, email, emailCodeSceneRegister).Scan(&id, &storedHash, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return errEmailCodeInvalid
	}
	if err != nil {
		return fmt.Errorf("查询邮箱验证码: %w", err)
	}

	expectedHash := hashEmailCode(email, emailCodeSceneRegister, code)
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(expectedHash)) != 1 {
		attempts++
		var updateErr error
		if attempts >= emailCodeMaxAttempts {
			_, updateErr = tx.Exec("UPDATE user_email_codes SET attempts = ?, used_at = NOW() WHERE id = ?", attempts, id)
		} else {
			_, updateErr = tx.Exec("UPDATE user_email_codes SET attempts = ? WHERE id = ?", attempts, id)
		}
		if updateErr != nil {
			return fmt.Errorf("更新邮箱验证码尝试次数: %w", updateErr)
		}
		return errEmailCodeIncorrect
	}

	result, err := tx.Exec("UPDATE user_email_codes SET used_at = NOW() WHERE id = ? AND used_at IS NULL", id)
	if err != nil {
		return fmt.Errorf("消费邮箱验证码: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("确认邮箱验证码消费结果: %w", err)
	}
	if affected != 1 {
		return errEmailCodeInvalid
	}
	return nil
}

type userForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// UserForgotPassword 申请密码重置：生成一次性令牌并邮件发送重置链接。
// 无论邮箱是否注册都返回相同文案，防止账号枚举。
func UserForgotPassword(c *gin.Context) {
	var req userForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请输入有效的邮箱地址"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))

	if emailSendLimited("forgot", email) {
		c.JSON(http.StatusOK, gin.H{"code": 429, "msg": "发送过于频繁，请 1 分钟后再试"})
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

	if err := ensureUserAuthStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化失败"})
		return
	}

	// 邮件服务未配置时明确报错，避免用户干等一封永远发不出的邮件
	mailCfg, err := loadMailConfig(db, true)
	if err != nil || validateMailConfig(mailCfg, true) != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "邮件服务未配置，请联系管理员重置密码"})
		return
	}

	const okMsg = "如果该邮箱已注册，重置链接将发送到您的邮箱，请注意查收"

	var userID uint
	err = db.QueryRow("SELECT id FROM users WHERE email = ? AND enabled = 1", email).Scan(&userID)
	if err != nil {
		// 邮箱未注册：同样返回成功文案，不发送邮件
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": okMsg})
		return
	}

	tokenBytes := make([]byte, passwordResetTokenByteSize)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "生成令牌失败"})
		return
	}
	token := hex.EncodeToString(tokenBytes)
	tokenHash := sha256.Sum256([]byte(token))

	// 旧链接作废，保证任意时刻只有最新一封邮件可用
	_, _ = db.Exec("UPDATE user_password_resets SET used_at = NOW() WHERE email = ? AND used_at IS NULL", email)
	if _, err := db.Exec(
		"INSERT INTO user_password_resets (user_id, email, token_hash, expires_at, created_ip) VALUES (?, ?, ?, DATE_ADD(NOW(), INTERVAL ? SECOND), ?)",
		userID, email, hex.EncodeToString(tokenHash[:]), int(passwordResetTokenTTL.Seconds()), c.ClientIP(),
	); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建重置令牌失败"})
		return
	}

	resetURL := buildPasswordResetURL(c, token)
	siteName := loadSiteNameForMail(db)
	subject := fmt.Sprintf("%s 密码重置", siteName)
	content := fmt.Sprintf(
		"您好：\n\n我们收到了您在 %s 的密码重置申请。\n\n请点击以下链接重置密码（%d 分钟内有效，仅可使用一次）：\n%s\n\n如果这不是您的操作，请忽略本邮件，您的密码不会改变。",
		siteName, int(passwordResetTokenTTL.Minutes()), resetURL,
	)

	logID, logged := createMailLog(db, "password_reset", "user", int64(userID), 0, email, subject, content, 0, nil)
	if err := sendSMTPMail(mailCfg, mailMessage{To: email, Subject: subject, Content: content, ContentType: "text"}); err != nil {
		if logged {
			markMailLogFailed(db, logID, err.Error())
		}
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "邮件发送失败，请稍后重试或联系管理员"})
		return
	}
	if logged {
		markMailLogSent(db, logID)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": okMsg})
}

// buildPasswordResetURL 依据请求来源拼装前端重置页链接
func buildPasswordResetURL(c *gin.Context, token string) string {
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	return fmt.Sprintf("%s://%s/user/reset-password?token=%s", scheme, host, token)
}

// ========== 重置密码 ==========

type userResetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

// UserResetPassword 凭邮件令牌重置密码
func UserResetPassword(c *gin.Context) {
	var req userResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误，密码至少 6 位"})
		return
	}
	token := strings.TrimSpace(req.Token)
	if len(token) != passwordResetTokenByteSize*2 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "重置链接无效或已过期，请重新申请"})
		return
	}
	tokenHash := sha256.Sum256([]byte(token))

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

	if err := ensureUserAuthStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化失败"})
		return
	}

	var resetID, userID uint
	var email string
	err = db.QueryRow(`
		SELECT id, user_id, email FROM user_password_resets
		WHERE token_hash = ? AND used_at IS NULL AND expires_at > NOW()
	`, hex.EncodeToString(tokenHash[:])).Scan(&resetID, &userID, &email)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "重置链接无效或已过期，请重新申请"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(req.Password)), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "密码处理失败"})
		return
	}
	if _, err := db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(hash), userID); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "重置失败"})
		return
	}
	_, _ = db.Exec("UPDATE user_password_resets SET used_at = NOW() WHERE id = ?", resetID)
	_, _ = db.Exec("UPDATE user_password_resets SET used_at = NOW() WHERE email = ? AND used_at IS NULL", email)

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "密码重置成功，请使用新密码登录"})
}
