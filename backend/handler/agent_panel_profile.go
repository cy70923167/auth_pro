package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// AgentPanelProfile 个人中心-账户信息
func AgentPanelProfile(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}
	db := agentPanelDB(c)
	if db == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var (
		email, name, level, levelName string
		contact                       sql.NullString
		discount, balance             float64
		createdAt, realnameAt         sql.NullTime
		realName, realIDCard          string
	)
	if err := ensureRealnameStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化实名存储失败"})
		return
	}
	err := db.QueryRow(`
		SELECT a.email, a.name, a.level, COALESCE(NULLIF(l.name, ''), a.level),
		       a.contact, a.discount, a.balance, a.created_at,
		       COALESCE(a.real_name, ''), COALESCE(a.real_id_card, ''), a.realname_at
		FROM agents a
		LEFT JOIN agent_levels l ON l.code = a.level
		WHERE a.id = ? AND a.enabled = 1
	`, agentID).Scan(&email, &name, &level, &levelName, &contact, &discount, &balance, &createdAt, &realName, &realIDCard, &realnameAt)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "代理商不存在或已禁用"})
		return
	}

	var quotaRemain int
	_ = db.QueryRow(`
		SELECT COALESCE(SUM(GREATEST(total - used, 0)), 0) FROM agent_quotas WHERE agent_id = ?
	`, agentID).Scan(&quotaRemain)

	var licenseCount int
	_ = db.QueryRow(`
		SELECT COUNT(*) FROM licenses WHERE owner_type = 'agent' AND owner_id = ?
	`, agentID).Scan(&licenseCount)

	created := ""
	if createdAt.Valid {
		created = createdAt.Time.Format("2006-01-02")
	}
	realnameAtStr := ""
	if realnameAt.Valid {
		realnameAtStr = realnameAt.Time.Format("2006-01-02 15:04:05")
	}
	realnameEnabled := false
	realnameProvider := ""
	realnameAuthMode := ""
	if rnCfg, err := loadRealnameConfig(db); err == nil {
		realnameEnabled = rnCfg.Enabled
		realnameProvider = rnCfg.Provider
		if rnCfg.Provider == realnameProviderXiaomu {
			realnameAuthMode = rnCfg.XiaomuProductMode
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200, "msg": "",
		"data": gin.H{
			"id":               agentID,
			"email":            email,
			"name":             name,
			"contact":          contact.String,
			"level":            level,
			"levelName":        levelName,
			"discount":         fmt.Sprintf("%g折", discount),
			"balance":          balance,
			"quotaRemain":      quotaRemain,
			"licenseCount":     licenseCount,
			"createdAt":        created,
			"realnameEnabled":  realnameEnabled,
			"realnameProvider": realnameProvider,
			"realnameAuthMode": realnameAuthMode,
			"realnameVerified": realnameAt.Valid,
			"realName":         maskRealName(realName),
			"realIdCard":       maskIDCard(realIDCard),
			"realnameAt":       realnameAtStr,
		},
	})
}

// AgentPanelUpdateProfile 个人中心-保存基本信息
func AgentPanelUpdateProfile(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}
	var req struct {
		Name    string `json:"name"`
		Contact string `json:"contact"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "名称不能为空"})
		return
	}
	if len([]rune(req.Name)) > 50 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "名称过长"})
		return
	}

	db := agentPanelDB(c)
	if db == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	if _, err := db.Exec(`UPDATE agents SET name = ?, contact = ? WHERE id = ?`, req.Name, strings.TrimSpace(req.Contact), agentID); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "保存成功"})
}

// AgentPanelChangePassword 个人中心-修改密码
func AgentPanelChangePassword(c *gin.Context) {
	agentID, ok := getAgentID(c)
	if !ok {
		return
	}
	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if len(req.NewPassword) < 6 || len(req.NewPassword) > 64 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "新密码长度需 6-64 位"})
		return
	}

	db := agentPanelDB(c)
	if db == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var hash string
	if err := db.QueryRow(`SELECT password_hash FROM agents WHERE id = ?`, agentID).Scan(&hash); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "代理商不存在"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.OldPassword)) != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "当前密码不正确"})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "密码加密失败"})
		return
	}
	if _, err := db.Exec(`UPDATE agents SET password_hash = ? WHERE id = ?`, string(newHash), agentID); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "修改失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "密码修改成功"})
}
