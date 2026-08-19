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
	"golang.org/x/crypto/bcrypt"
)

// AgentList 代理商列表
func AgentList(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := EnsureAccountUpgradeSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "代理商账户结构初始化失败: " + err.Error()})
		return
	}

	keyword := c.Query("keyword")
	level := c.Query("level")
	status := c.Query("status")
	source := c.Query("source")
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

	if keyword != "" {
		where = append(where, "(a.name LIKE ? OR a.contact LIKE ? OR a.email LIKE ?)")
		args = append(args, "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if level != "" {
		where = append(where, "a.level = ?")
		args = append(args, level)
	}
	if status != "" {
		if status == "active" {
			where = append(where, "a.enabled = 1")
		} else {
			where = append(where, "a.enabled = 0")
		}
	}
	if source == "admin" || source == "user_upgrade" {
		where = append(where, "a.source = ?")
		args = append(args, source)
	}

	whereSQL := strings.Join(where, " AND ")

	var total int64
	db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM agents a WHERE %s", whereSQL), args...).Scan(&total)

	offset := (page - 1) * pageSize
	listSQL := fmt.Sprintf(`
		SELECT a.id, a.name, a.contact, a.level,
		       COALESCE(al.name, a.level) AS level_label,
		       CASE
		         WHEN a.discount BETWEEN 1 AND 10 THEN a.discount
		         WHEN al.discount BETWEEN 1 AND 10 THEN al.discount
		         ELSE 10
		       END AS discount,
		       a.balance, a.remark, a.enabled, a.created_at,
		       a.source, a.original_user_id, a.converted_at,
		       COALESCE(c.conversion_no, ''), CAST(c.transferred_balance AS CHAR),
		       COALESCE(c.migrated_license_count, 0),
		       (SELECT COUNT(*) FROM licenses l WHERE l.owner_type = 'agent' AND l.owner_id = a.id) as total_licenses
		FROM agents a
		LEFT JOIN agent_levels al ON al.code = a.level
		LEFT JOIN account_conversions c ON c.agent_id = a.id
		WHERE %s
		ORDER BY a.created_at DESC
		LIMIT ? OFFSET ?
	`, whereSQL)

	listArgs := append(args, pageSize, offset)
	rows, err := db.Query(listSQL, listArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	type agentItem struct {
		ID                   int64   `json:"id"`
		Name                 string  `json:"name"`
		Contact              string  `json:"contact"`
		Level                string  `json:"level"`
		LevelLabel           string  `json:"levelLabel"`
		Discount             float64 `json:"discount"`
		Balance              float64 `json:"balance"`
		Remark               string  `json:"remark"`
		Status               string  `json:"status"`
		StatusLabel          string  `json:"statusLabel"`
		Source               string  `json:"source"`
		SourceLabel          string  `json:"sourceLabel"`
		OriginalUserID       *int64  `json:"originalUserId"`
		ConvertedAt          string  `json:"convertedAt"`
		ConversionNo         string  `json:"conversionNo"`
		TransferredBalance   float64 `json:"transferredBalance"`
		MigratedLicenseCount int64   `json:"migratedLicenseCount"`
		TotalLicenses        int64   `json:"totalLicenses"`
		CreatedAt            string  `json:"createdAt"`
	}

	var list []agentItem
	for rows.Next() {
		var item agentItem
		var enabled bool
		var createdAt time.Time
		var convertedAt sql.NullTime
		var originalUserID sql.NullInt64
		var transferredBalance sql.NullString
		var remark sql.NullString
		err := rows.Scan(&item.ID, &item.Name, &item.Contact, &item.Level, &item.LevelLabel, &item.Discount,
			&item.Balance, &remark, &enabled, &createdAt, &item.Source, &originalUserID, &convertedAt,
			&item.ConversionNo, &transferredBalance, &item.MigratedLicenseCount, &item.TotalLicenses)
		if err != nil {
			continue
		}
		if remark.Valid {
			item.Remark = remark.String
		}
		if originalUserID.Valid {
			value := originalUserID.Int64
			item.OriginalUserID = &value
		}
		if convertedAt.Valid {
			item.ConvertedAt = convertedAt.Time.Format("2006-01-02 15:04")
		}
		if transferredBalance.Valid {
			item.TransferredBalance = adminAmountValue(transferredBalance.String)
		}
		if item.Source == "user_upgrade" {
			item.SourceLabel = "用户自助升级"
		} else {
			item.SourceLabel = "后台创建"
		}
		if enabled {
			item.Status = "active"
			item.StatusLabel = "正常"
		} else {
			item.Status = "frozen"
			item.StatusLabel = "冻结"
		}
		item.CreatedAt = createdAt.Format("2006-01-02 15:04")
		list = append(list, item)
	}
	if list == nil {
		list = []agentItem{}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{"list": list, "total": total}})
}

// AgentCreate 新增代理商
func AgentCreate(c *gin.Context) {
	var req struct {
		Name     string  `json:"name" binding:"required"`
		Contact  string  `json:"contact" binding:"required"`
		Password string  `json:"password" binding:"required"`
		Level    string  `json:"level"`
		Discount float64 `json:"discount"`
		Remark   string  `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	password := strings.TrimSpace(req.Password)
	if password == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "登录密码不能为空"})
		return
	}

	if req.Level == "" {
		req.Level = "bronze"
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureAgentLevelSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "代理商等级表初始化失败: " + err.Error()})
		return
	}
	var errMsg string
	req.Discount, errMsg = resolveAgentDiscount(db, req.Level, req.Discount)
	if errMsg != "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": errMsg})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "密码加密失败"})
		return
	}

	// 用 contact 作为 email（唯一键）
	email := req.Contact
	result, err := db.Exec(`
		INSERT INTO agents (email, password_hash, name, contact, level, discount, remark)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, email, string(hash), req.Name, req.Contact, req.Level, req.Discount, req.Remark)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建失败: " + err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "创建成功", "data": gin.H{"id": id}})
}

// AgentUpdate 编辑代理商
func AgentUpdate(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name     string  `json:"name"`
		Contact  string  `json:"contact"`
		Password string  `json:"password"`
		Level    string  `json:"level"`
		Discount float64 `json:"discount"`
		Remark   string  `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	if req.Level == "" {
		req.Level = "bronze"
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensureAgentLevelSchema(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "代理商等级表初始化失败: " + err.Error()})
		return
	}
	var errMsg string
	req.Discount, errMsg = resolveAgentDiscount(db, req.Level, req.Discount)
	if errMsg != "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": errMsg})
		return
	}

	if strings.TrimSpace(req.Password) != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "密码加密失败"})
			return
		}
		_, err = db.Exec(`UPDATE agents SET name = ?, contact = ?, password_hash = ?, level = ?, discount = ?, remark = ? WHERE id = ?`,
			req.Name, req.Contact, string(hash), req.Level, req.Discount, req.Remark, id)
	} else {
		_, err = db.Exec(`UPDATE agents SET name = ?, contact = ?, level = ?, discount = ?, remark = ? WHERE id = ?`,
			req.Name, req.Contact, req.Level, req.Discount, req.Remark, id)
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "更新成功"})
}

// AgentToggle 冻结/解冻代理商
func AgentToggle(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status string `json:"status" binding:"required"`
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

	enabled := 1
	if req.Status == "frozen" {
		enabled = 0
	}

	_, err = db.Exec("UPDATE agents SET enabled = ? WHERE id = ?", enabled, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "操作失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "操作成功"})
}

// AgentRecharge 代理商充值
func AgentRecharge(c *gin.Context) {
	agentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || agentID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "代理商ID无效"})
		return
	}
	var req struct {
		Amount float64 `json:"amount" binding:"required,gt=0"`
		Remark string  `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "充值金额必须大于0"})
		return
	}
	amountCents := floatAmountToCents(req.Amount)
	if amountCents < minRechargeCents {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "充值金额不能低于 ¥0.01"})
		return
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	operatorID, _ := c.Get("user_id")
	if err := rechargeAgentManually(db, agentID, amountCents, strings.TrimSpace(req.Remark), operatorID); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "充值失败，余额未变更"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": fmt.Sprintf("成功充值 ¥%.2f", float64(amountCents)/100)})
}

func rechargeAgentManually(db *sql.DB, agentID uint64, amountCents int64, remark string, operatorID any) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	balanceAfter, err := increaseSubjectBalance(tx, "agent", agentID, amountCents)
	if err != nil {
		return err
	}

	operatorRemark := "后台管理员手工充值"
	if operatorID != nil {
		operatorRemark = fmt.Sprintf("后台管理员 %v 手工充值", operatorID)
	}
	if remark != "" {
		operatorRemark += "：" + remark
	}
	if _, err := tx.Exec(`
		INSERT INTO transactions (tx_no, subject_type, subject_id, type, amount, balance_after, ref_type, ref_id, remark)
		VALUES (?, 'agent', ?, 'recharge', ?, ?, 'admin_manual_recharge', ?, ?)
	`, generateTransactionNo(), agentID, formatCents(amountCents), balanceAfter, operatorID, operatorRemark); err != nil {
		return fmt.Errorf("记录充值流水失败: %w", err)
	}

	return tx.Commit()
}

func agentDeletionProtected(source string, originalUserID sql.NullInt64) bool {
	return source == "user_upgrade" || originalUserID.Valid
}

// AgentDelete 删除代理商
func AgentDelete(c *gin.Context) {
	id := c.Param("id")

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var source string
	var originalUserID sql.NullInt64
	if err := db.QueryRow("SELECT source, original_user_id FROM agents WHERE id = ?", id).Scan(&source, &originalUserID); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "代理商不存在"})
		return
	}
	if agentDeletionProtected(source, originalUserID) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "用户升级产生的代理账户需要保留审计关联，不能删除；如需停用请执行冻结"})
		return
	}

	db.Exec("DELETE FROM agent_quotas WHERE agent_id = ?", id)
	_, err = db.Exec("DELETE FROM agents WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "删除成功"})
}
