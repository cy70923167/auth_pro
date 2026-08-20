package handler

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
)

func ensureAgentLevelSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS agent_levels (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
			code VARCHAR(50) NOT NULL COMMENT '等级编码',
			name VARCHAR(50) NOT NULL COMMENT '等级名称',
			discount DECIMAL(3,1) NOT NULL DEFAULT 9.0 COMMENT '折扣(1-10)',
			self_service_enabled TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否允许用户自助开通',
			upgrade_price DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '用户自助开通价格',
			opening_bonus DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '自助开通赠送余额',
			benefits TEXT COMMENT '等级权益说明',
			sort INT DEFAULT 0 COMMENT '排序',
			enabled TINYINT(1) DEFAULT 1 COMMENT '是否启用',
			remark VARCHAR(255) DEFAULT '' COMMENT '备注',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
			PRIMARY KEY (id),
			UNIQUE KEY uk_agent_level_code (code),
			KEY idx_agent_level_self_service (self_service_enabled, enabled, sort)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理商等级表'
	`)
	if err != nil {
		return err
	}

	_, _ = db.Exec("ALTER TABLE agents ADD COLUMN level VARCHAR(50) DEFAULT 'bronze' COMMENT '等级编码' AFTER contact")
	_, _ = db.Exec("ALTER TABLE agents MODIFY COLUMN level VARCHAR(50) DEFAULT 'bronze' COMMENT '等级编码'")
	_, _ = db.Exec("ALTER TABLE agents ADD COLUMN discount DECIMAL(3,1) DEFAULT 9.0 COMMENT '折扣(1-10)' AFTER level")
	_, _ = db.Exec("ALTER TABLE agents ADD COLUMN remark VARCHAR(255) DEFAULT '' COMMENT '备注' AFTER balance")
	if err := ensureColumn(db, "agent_levels", "self_service_enabled",
		"ALTER TABLE agent_levels ADD COLUMN self_service_enabled TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否允许用户自助开通' AFTER discount"); err != nil {
		return err
	}
	if err := ensureColumn(db, "agent_levels", "upgrade_price",
		"ALTER TABLE agent_levels ADD COLUMN upgrade_price DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '用户自助开通价格' AFTER self_service_enabled"); err != nil {
		return err
	}
	if err := ensureColumn(db, "agent_levels", "opening_bonus",
		"ALTER TABLE agent_levels ADD COLUMN opening_bonus DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '自助开通赠送余额' AFTER upgrade_price"); err != nil {
		return err
	}
	if err := ensureColumn(db, "agent_levels", "benefits",
		"ALTER TABLE agent_levels ADD COLUMN benefits TEXT COMMENT '等级权益说明' AFTER opening_bonus"); err != nil {
		return err
	}
	if err := ensureIndex(db, "agent_levels", "idx_agent_level_self_service", []string{"self_service_enabled", "enabled", "sort"}, false); err != nil {
		return err
	}

	_, err = db.Exec(`
		INSERT IGNORE INTO agent_levels (code, name, discount, self_service_enabled, upgrade_price, sort, enabled, remark) VALUES
		('gold', '金牌代理', 7.0, 0, 0.00, 1, 1, '默认金牌等级'),
		('silver', '银牌代理', 8.0, 0, 0.00, 2, 1, '默认银牌等级'),
		('bronze', '铜牌代理', 9.0, 0, 0.00, 3, 1, '默认铜牌等级')
	`)
	if err != nil {
		return err
	}

	_, _ = db.Exec(`
		UPDATE agents a
		JOIN agent_levels al ON a.level = al.name
		SET a.level = al.code, a.discount = al.discount
		WHERE a.level <> al.code
	`)
	return nil
}

func openAgentLevelDB(c *gin.Context) (*sql.DB, bool) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return nil, false
	}
	if err := ensureAgentLevelSchema(db); err != nil {
		db.Close()
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "代理商等级表初始化失败: " + err.Error()})
		return nil, false
	}
	return db, true
}

func agentLevelDiscountByCode(db *sql.DB, code string) (float64, string, string) {
	code = strings.TrimSpace(code)
	if code == "" {
		code = "bronze"
	}

	var name string
	var discount float64
	err := db.QueryRow("SELECT name, discount FROM agent_levels WHERE code = ? AND enabled = 1", code).Scan(&name, &discount)
	if err != nil {
		return 0, "", "代理商等级不存在或已禁用"
	}
	return discount, name, ""
}

// currentAgentDiscount returns the agent's effective discount. The per-agent
// value takes precedence; the level discount only covers legacy/invalid data.
func currentAgentDiscount(db *sql.DB, agentID uint) (float64, error) {
	var discount float64
	err := db.QueryRow(`
		SELECT CASE
			WHEN a.discount BETWEEN 1 AND 10 THEN a.discount
			WHEN al.discount BETWEEN 1 AND 10 THEN al.discount
			ELSE 10
		END
		FROM agents a
		LEFT JOIN agent_levels al ON al.code = a.level AND al.enabled = 1
		WHERE a.id = ? AND a.enabled = 1
	`, agentID).Scan(&discount)
	return discount, err
}

func resolveAgentDiscount(db *sql.DB, level string, requested float64) (float64, string) {
	levelDiscount, _, errMsg := agentLevelDiscountByCode(db, level)
	if errMsg != "" {
		return 0, errMsg
	}
	if requested == 0 {
		return levelDiscount, ""
	}
	if requested < 1 || requested > 10 {
		return 0, "代理商折扣需在 1-10 之间"
	}
	return requested, ""
}

func validateAgentLevelPayload(name string, discount float64) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "请填写等级名称"
	}
	if discount < 1 || discount > 10 {
		return "折扣需在 1-10 之间"
	}
	return ""
}

func generateAgentLevelCode() (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "level_" + hex.EncodeToString(value), nil
}

const maxAgentLevelMoneyCents int64 = 999999999999

func validateAgentLevelMoney(value float64, field string) (int64, string) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 9999999999.99 {
		return 0, fmt.Sprintf("%s需在 0-9999999999.99 元之间", field)
	}
	scaled := value * 100
	cents := int64(math.Round(scaled))
	if cents < 0 || cents > maxAgentLevelMoneyCents || math.Abs(scaled-float64(cents)) > 0.000001 {
		return 0, fmt.Sprintf("%s最多保留两位小数", field)
	}
	return cents, ""
}

func validateAgentLevelSelfService(enabled bool, price, openingBonus float64) (int64, int64, string) {
	priceCents, msg := validateAgentLevelMoney(price, "自助开通价格")
	if msg != "" {
		return 0, 0, msg
	}
	bonusCents, msg := validateAgentLevelMoney(openingBonus, "开通赠送余额")
	if msg != "" {
		return 0, 0, msg
	}
	if enabled && priceCents < 1 {
		return 0, 0, "允许自助开通时，开通价格不能低于 0.01 元"
	}
	return priceCents, bonusCents, ""
}

// AgentLevelList 代理商等级列表
func AgentLevelList(c *gin.Context) {
	db, ok := openAgentLevelDB(c)
	if !ok {
		return
	}
	defer db.Close()

	keyword := strings.TrimSpace(c.Query("keyword"))
	status := c.Query("status")
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
		where = append(where, "(code LIKE ? OR name LIKE ?)")
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if status == "enabled" {
		where = append(where, "enabled = 1")
	} else if status == "disabled" {
		where = append(where, "enabled = 0")
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM agent_levels WHERE %s", whereSQL), args...).Scan(&total)

	offset := (page - 1) * pageSize
	rows, err := db.Query(fmt.Sprintf(`
		SELECT id, code, name, discount, self_service_enabled, upgrade_price, opening_bonus,
		       COALESCE(benefits, ''), sort, enabled, remark, created_at, updated_at,
		       (SELECT COUNT(*) FROM agents a WHERE a.level = agent_levels.code OR a.level = agent_levels.name) AS agent_count
		FROM agent_levels
		WHERE %s
		ORDER BY sort ASC, id ASC
		LIMIT ? OFFSET ?
	`, whereSQL), append(args, pageSize, offset)...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	type levelItem struct {
		ID                 int64   `json:"id"`
		Code               string  `json:"code"`
		Name               string  `json:"name"`
		Discount           float64 `json:"discount"`
		SelfServiceEnabled bool    `json:"selfServiceEnabled"`
		UpgradePrice       float64 `json:"upgradePrice"`
		OpeningBonus       float64 `json:"openingBonus"`
		Benefits           string  `json:"benefits"`
		Sort               int     `json:"sort"`
		Enabled            bool    `json:"enabled"`
		Remark             string  `json:"remark"`
		AgentCount         int64   `json:"agentCount"`
		CreatedAt          string  `json:"createdAt"`
		UpdatedAt          string  `json:"updatedAt"`
	}

	list := []levelItem{}
	for rows.Next() {
		var item levelItem
		var remark sql.NullString
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Discount, &item.SelfServiceEnabled,
			&item.UpgradePrice, &item.OpeningBonus, &item.Benefits, &item.Sort, &item.Enabled, &remark, &createdAt, &updatedAt,
			&item.AgentCount); err != nil {
			continue
		}
		if remark.Valid {
			item.Remark = remark.String
		}
		item.CreatedAt = createdAt.Format("2006-01-02 15:04")
		item.UpdatedAt = updatedAt.Format("2006-01-02 15:04")
		list = append(list, item)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{"list": list, "total": total}})
}

// AgentLevelSelectList 代理商等级下拉列表
func AgentLevelSelectList(c *gin.Context) {
	db, ok := openAgentLevelDB(c)
	if !ok {
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT code, name, discount FROM agent_levels WHERE enabled = 1 ORDER BY sort ASC, id ASC")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	defer rows.Close()

	type option struct {
		Code     string  `json:"code"`
		Name     string  `json:"name"`
		Discount float64 `json:"discount"`
	}
	list := []option{}
	for rows.Next() {
		var item option
		if err := rows.Scan(&item.Code, &item.Name, &item.Discount); err == nil {
			list = append(list, item)
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": list})
}

// AgentLevelCreate 新增代理商等级
func AgentLevelCreate(c *gin.Context) {
	var req struct {
		Name               string  `json:"name"`
		Discount           float64 `json:"discount"`
		SelfServiceEnabled bool    `json:"selfServiceEnabled"`
		UpgradePrice       float64 `json:"upgradePrice"`
		OpeningBonus       float64 `json:"openingBonus"`
		Benefits           string  `json:"benefits"`
		Sort               int     `json:"sort"`
		Enabled            bool    `json:"enabled"`
		Remark             string  `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if msg := validateAgentLevelPayload(req.Name, req.Discount); msg != "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": msg})
		return
	}
	priceCents, bonusCents, msg := validateAgentLevelSelfService(req.SelfServiceEnabled, req.UpgradePrice, req.OpeningBonus)
	if msg != "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": msg})
		return
	}

	db, ok := openAgentLevelDB(c)
	if !ok {
		return
	}
	defer db.Close()

	for attempt := 0; attempt < 3; attempt++ {
		code, err := generateAgentLevelCode()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "生成等级编码失败"})
			return
		}
		_, err = db.Exec(`
			INSERT INTO agent_levels (
				code, name, discount, self_service_enabled, upgrade_price, opening_bonus, benefits, sort, enabled, remark
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, code, strings.TrimSpace(req.Name), req.Discount, req.SelfServiceEnabled,
			formatCents(priceCents), formatCents(bonusCents), strings.TrimSpace(req.Benefits), req.Sort, req.Enabled, strings.TrimSpace(req.Remark))
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "创建成功"})
			return
		}
		var mysqlErr *mysql.MySQLError
		if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1062 {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建失败: " + err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "生成唯一等级编码失败，请重试"})
}

// AgentLevelUpdate 编辑代理商等级
func AgentLevelUpdate(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name               string  `json:"name"`
		Discount           float64 `json:"discount"`
		SelfServiceEnabled bool    `json:"selfServiceEnabled"`
		UpgradePrice       float64 `json:"upgradePrice"`
		OpeningBonus       float64 `json:"openingBonus"`
		Benefits           string  `json:"benefits"`
		Sort               int     `json:"sort"`
		Enabled            bool    `json:"enabled"`
		Remark             string  `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请填写等级名称"})
		return
	}
	if req.Discount < 1 || req.Discount > 10 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "折扣需在 1-10 之间"})
		return
	}
	priceCents, bonusCents, msg := validateAgentLevelSelfService(req.SelfServiceEnabled, req.UpgradePrice, req.OpeningBonus)
	if msg != "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": msg})
		return
	}

	db, ok := openAgentLevelDB(c)
	if !ok {
		return
	}
	defer db.Close()

	var code string
	var name string
	var oldDiscount float64
	var agentCount int64
	if err := db.QueryRow("SELECT code, name, discount, (SELECT COUNT(*) FROM agents a WHERE a.level = agent_levels.code OR a.level = agent_levels.name) FROM agent_levels WHERE id = ?", id).Scan(&code, &name, &oldDiscount, &agentCount); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "等级不存在"})
		return
	}
	if !req.Enabled && agentCount > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该等级已有代理商使用，不能禁用"})
		return
	}

	_, err := db.Exec(`
		UPDATE agent_levels
		SET name = ?, discount = ?, self_service_enabled = ?, upgrade_price = ?, opening_bonus = ?, benefits = ?,
		    sort = ?, enabled = ?, remark = ?
		WHERE id = ?
	`, strings.TrimSpace(req.Name), req.Discount, req.SelfServiceEnabled, formatCents(priceCents),
		formatCents(bonusCents), strings.TrimSpace(req.Benefits), req.Sort, req.Enabled, strings.TrimSpace(req.Remark), id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "更新失败"})
		return
	}
	// Keep custom per-agent discounts. Only agents still using the old level
	// default inherit the new default discount.
	_, _ = db.Exec(`
		UPDATE agents
		SET level = ?, discount = CASE WHEN discount = ? THEN ? ELSE discount END
		WHERE level = ? OR level = ?
	`, code, oldDiscount, req.Discount, code, name)

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "更新成功"})
}

// AgentLevelDelete 删除代理商等级
func AgentLevelDelete(c *gin.Context) {
	id := c.Param("id")
	db, ok := openAgentLevelDB(c)
	if !ok {
		return
	}
	defer db.Close()

	var code string
	var name string
	var agentCount int64
	if err := db.QueryRow("SELECT code, name, (SELECT COUNT(*) FROM agents a WHERE a.level = agent_levels.code OR a.level = agent_levels.name) FROM agent_levels WHERE id = ?", id).Scan(&code, &name, &agentCount); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "等级不存在"})
		return
	}
	if agentCount > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该等级已有代理商使用，不能删除"})
		return
	}

	_, err := db.Exec("DELETE FROM agent_levels WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "删除失败"})
		return
	}
	_ = code
	_ = name
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "删除成功"})
}
