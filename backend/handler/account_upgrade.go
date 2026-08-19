package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
)

// EnsureAccountUpgradeSchema upgrades existing installations without changing
// legacy account ownership or enabling self-service sales by default.
func EnsureAccountUpgradeSchema(db *sql.DB) error {
	if err := ensureAgentLevelSchema(db); err != nil {
		return fmt.Errorf("初始化代理等级失败: %w", err)
	}
	if err := ensureUserAuthStorage(db); err != nil {
		return fmt.Errorf("初始化用户认证资料失败: %w", err)
	}
	if err := ensureRealnameStorage(db); err != nil {
		return fmt.Errorf("初始化实名资料失败: %w", err)
	}

	columns := []struct {
		table string
		name  string
		ddl   string
	}{
		{"users", "account_status", "ALTER TABLE users ADD COLUMN account_status ENUM('active','converted') NOT NULL DEFAULT 'active' COMMENT '账户主体状态' AFTER balance"},
		{"users", "converted_agent_id", "ALTER TABLE users ADD COLUMN converted_agent_id BIGINT UNSIGNED DEFAULT NULL COMMENT '转换后的代理ID' AFTER account_status"},
		{"users", "converted_at", "ALTER TABLE users ADD COLUMN converted_at DATETIME DEFAULT NULL COMMENT '账户转换时间' AFTER converted_agent_id"},
		{"agents", "source", "ALTER TABLE agents ADD COLUMN source ENUM('admin','user_upgrade') NOT NULL DEFAULT 'admin' COMMENT '账户来源' AFTER balance"},
		{"agents", "original_user_id", "ALTER TABLE agents ADD COLUMN original_user_id BIGINT UNSIGNED DEFAULT NULL COMMENT '升级前用户ID' AFTER source"},
		{"agents", "converted_at", "ALTER TABLE agents ADD COLUMN converted_at DATETIME DEFAULT NULL COMMENT '账户转换完成时间' AFTER original_user_id"},
		{"agent_levels", "self_service_enabled", "ALTER TABLE agent_levels ADD COLUMN self_service_enabled TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否允许用户自助开通' AFTER discount"},
		{"agent_levels", "upgrade_price", "ALTER TABLE agent_levels ADD COLUMN upgrade_price DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '用户自助开通价格' AFTER self_service_enabled"},
		{"agent_levels", "opening_bonus", "ALTER TABLE agent_levels ADD COLUMN opening_bonus DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '自助开通赠送余额' AFTER upgrade_price"},
		{"agent_levels", "benefits", "ALTER TABLE agent_levels ADD COLUMN benefits TEXT COMMENT '等级权益说明' AFTER opening_bonus"},
	}
	for _, column := range columns {
		if err := ensureColumn(db, column.table, column.name, column.ddl); err != nil {
			return fmt.Errorf("补充 %s.%s 失败: %w", column.table, column.name, err)
		}
	}

	indexes := []struct {
		table   string
		name    string
		columns []string
		unique  bool
	}{
		{"users", "uk_user_converted_agent", []string{"converted_agent_id"}, true},
		{"users", "idx_user_account_status", []string{"account_status", "enabled"}, false},
		{"agents", "uk_agent_original_user", []string{"original_user_id"}, true},
		{"agents", "idx_agent_source", []string{"source", "created_at"}, false},
		{"agent_levels", "idx_agent_level_self_service", []string{"self_service_enabled", "enabled", "sort"}, false},
	}
	for _, index := range indexes {
		if err := ensureIndex(db, index.table, index.name, index.columns, index.unique); err != nil {
			return fmt.Errorf("补充 %s 索引失败: %w", index.table, err)
		}
	}

	if err := ensureAgentUpgradeOrderTable(db); err != nil {
		return err
	}
	if err := ensureAccountConversionTable(db); err != nil {
		return err
	}
	return ensureTransferTransactionType(db)
}

func ensureAgentUpgradeOrderTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS agent_upgrade_orders (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
		order_no VARCHAR(64) NOT NULL COMMENT '升级订单号',
		user_id BIGINT UNSIGNED NOT NULL COMMENT '发起升级的用户ID',
		level_id BIGINT UNSIGNED NOT NULL COMMENT '目标代理等级ID',
		level_code_snapshot VARCHAR(50) NOT NULL COMMENT '等级编码快照',
		level_name_snapshot VARCHAR(50) NOT NULL COMMENT '等级名称快照',
		discount_snapshot DECIMAL(3,1) NOT NULL COMMENT '代理折扣快照',
		opening_bonus_snapshot DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '开通赠送余额快照',
		amount DECIMAL(12,2) NOT NULL COMMENT '应付开通费',
		paid_amount DECIMAL(12,2) DEFAULT NULL COMMENT '实际支付金额',
		pay_channel VARCHAR(30) NOT NULL DEFAULT '' COMMENT '支付渠道(balance/easypay/easypay-v2)',
		pay_method VARCHAR(30) NOT NULL DEFAULT '' COMMENT '支付方式',
		gateway_trade_no VARCHAR(100) DEFAULT NULL COMMENT '支付网关交易号',
		return_url VARCHAR(500) NOT NULL DEFAULT '' COMMENT '支付完成前端返回地址',
		status ENUM('pending','paid','processing','completed','failed','cancelled') NOT NULL DEFAULT 'pending' COMMENT '订单状态',
		agent_id BIGINT UNSIGNED DEFAULT NULL COMMENT '转换后的代理ID',
		paid_at DATETIME DEFAULT NULL COMMENT '支付完成时间',
		completed_at DATETIME DEFAULT NULL COMMENT '转换完成时间',
		error_message TEXT COMMENT '最近一次失败原因',
		notify_payload TEXT COMMENT '支付回调原始参数',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
		PRIMARY KEY (id),
		UNIQUE KEY uk_agent_upgrade_order_no (order_no),
		KEY idx_agent_upgrade_user_status (user_id, status, created_at),
		UNIQUE KEY uk_agent_upgrade_gateway_trade (gateway_trade_no),
		KEY idx_agent_upgrade_status (status, updated_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户自助开通代理订单表'`)
	if err != nil {
		return fmt.Errorf("初始化代理升级订单表失败: %w", err)
	}
	if err := ensureColumn(db, "agent_upgrade_orders", "opening_bonus_snapshot",
		"ALTER TABLE agent_upgrade_orders ADD COLUMN opening_bonus_snapshot DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '开通赠送余额快照' AFTER discount_snapshot"); err != nil {
		return fmt.Errorf("补充代理升级订单赠送余额快照失败: %w", err)
	}
	if err := ensureColumn(db, "agent_upgrade_orders", "pay_channel",
		"ALTER TABLE agent_upgrade_orders ADD COLUMN pay_channel VARCHAR(30) NOT NULL DEFAULT '' COMMENT '支付渠道(balance/easypay/easypay-v2)' AFTER paid_amount"); err != nil {
		return fmt.Errorf("补充代理升级订单支付渠道失败: %w", err)
	}
	if err := ensureColumn(db, "agent_upgrade_orders", "pay_method",
		"ALTER TABLE agent_upgrade_orders ADD COLUMN pay_method VARCHAR(30) NOT NULL DEFAULT '' COMMENT '支付方式' AFTER pay_channel"); err != nil {
		return fmt.Errorf("补充代理升级订单支付方式失败: %w", err)
	}
	if err := ensureColumn(db, "agent_upgrade_orders", "gateway_trade_no",
		"ALTER TABLE agent_upgrade_orders ADD COLUMN gateway_trade_no VARCHAR(100) DEFAULT NULL COMMENT '支付网关交易号' AFTER pay_method"); err != nil {
		return fmt.Errorf("补充代理升级订单网关流水号失败: %w", err)
	}
	if err := ensureColumn(db, "agent_upgrade_orders", "return_url",
		"ALTER TABLE agent_upgrade_orders ADD COLUMN return_url VARCHAR(500) NOT NULL DEFAULT '' COMMENT '支付完成前端返回地址' AFTER gateway_trade_no"); err != nil {
		return fmt.Errorf("补充代理升级订单返回地址失败: %w", err)
	}
	if err := ensureColumn(db, "agent_upgrade_orders", "notify_payload",
		"ALTER TABLE agent_upgrade_orders ADD COLUMN notify_payload TEXT COMMENT '支付回调原始参数' AFTER error_message"); err != nil {
		return fmt.Errorf("补充代理升级订单回调数据失败: %w", err)
	}
	if _, err := db.Exec("ALTER TABLE agent_upgrade_orders MODIFY COLUMN gateway_trade_no VARCHAR(100) DEFAULT NULL COMMENT '支付网关交易号'"); err != nil {
		return fmt.Errorf("升级代理订单网关流水字段失败: %w", err)
	}
	if _, err := db.Exec("UPDATE agent_upgrade_orders SET gateway_trade_no = NULL WHERE gateway_trade_no = ''"); err != nil {
		return fmt.Errorf("清理代理升级订单空网关流水号失败: %w", err)
	}
	if err := ensureIndex(db, "agent_upgrade_orders", "uk_agent_upgrade_gateway_trade", []string{"gateway_trade_no"}, true); err != nil {
		return fmt.Errorf("补充代理升级订单网关流水唯一索引失败: %w", err)
	}
	return nil
}

func ensureAccountConversionTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS account_conversions (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
		conversion_no VARCHAR(64) NOT NULL COMMENT '转换流水号',
		upgrade_order_id BIGINT UNSIGNED NOT NULL COMMENT '升级订单ID',
		user_id BIGINT UNSIGNED NOT NULL COMMENT '原用户ID',
		agent_id BIGINT UNSIGNED DEFAULT NULL COMMENT '新代理ID',
		level_id BIGINT UNSIGNED NOT NULL COMMENT '代理等级ID',
		status ENUM('processing','completed','failed') NOT NULL DEFAULT 'processing' COMMENT '转换状态',
		opening_fee DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '开通费',
		transferred_balance DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '迁移余额',
		opening_bonus DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '开通赠送余额',
		migrated_license_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '迁移授权数',
		source_snapshot JSON DEFAULT NULL COMMENT '原用户关键资料快照',
		result_snapshot JSON DEFAULT NULL COMMENT '转换结果快照',
		error_message TEXT COMMENT '失败原因',
		started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '开始时间',
		completed_at DATETIME DEFAULT NULL COMMENT '完成时间',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
		PRIMARY KEY (id),
		UNIQUE KEY uk_account_conversion_no (conversion_no),
		UNIQUE KEY uk_account_conversion_order (upgrade_order_id),
		UNIQUE KEY uk_account_conversion_user (user_id),
		UNIQUE KEY uk_account_conversion_agent (agent_id),
		KEY idx_account_conversion_status (status, updated_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户转代理审计表'`)
	if err != nil {
		return fmt.Errorf("初始化账户转换审计表失败: %w", err)
	}
	if err := ensureColumn(db, "account_conversions", "opening_bonus",
		"ALTER TABLE account_conversions ADD COLUMN opening_bonus DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '开通赠送余额' AFTER transferred_balance"); err != nil {
		return fmt.Errorf("补充账户转换赠送余额审计失败: %w", err)
	}
	return nil
}

func ensureTransferTransactionType(db *sql.DB) error {
	var columnType string
	if err := db.QueryRow(`
		SELECT COLUMN_TYPE FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'transactions' AND COLUMN_NAME = 'type'
	`).Scan(&columnType); err != nil {
		return fmt.Errorf("读取财务流水类型失败: %w", err)
	}
	if strings.Contains(columnType, "'transfer'") && strings.Contains(columnType, "'bonus'") {
		return nil
	}
	if _, err := db.Exec(`ALTER TABLE transactions
		MODIFY COLUMN type ENUM('recharge','consume','refund','purchase','transfer','bonus') NOT NULL COMMENT '类型'`); err != nil {
		return fmt.Errorf("扩展账户升级财务流水类型失败: %w", err)
	}
	return nil
}

var (
	errAgentUpgradeOrderNotFound      = errors.New("代理升级订单不存在")
	errAgentUpgradeOrderState         = errors.New("代理升级订单状态不可处理")
	errAgentUpgradeAmountMismatch     = errors.New("代理升级订单金额不一致")
	errAgentUpgradeUserConverted      = errors.New("用户已转换为代理账户")
	errAgentUpgradeUserDisabled       = errors.New("用户账户已禁用")
	errAgentUpgradeLevelUnavailable   = errors.New("代理等级已关闭自助开通")
	errAgentUpgradeAccountConflict    = errors.New("邮箱或手机号已被代理账户使用")
	errAgentUpgradePendingOrder       = errors.New("存在待支付订单，请先完成或取消后再开通代理")
	errAgentUpgradeInsufficientFunds  = errors.New("用户余额不足")
	errAgentUpgradePaymentUnavailable = errors.New("所选在线支付通道不可用")
	errAgentUpgradePaymentCreate      = errors.New("创建在线支付订单失败")
)

type accountUpgradePayment struct {
	PaidCents        int64
	PayChannel       string
	PayMethod        string
	GatewayTradeNo   string
	NotifyPayload    string
	DeductOpeningFee bool
}

type accountConversionResult struct {
	OrderID              uint64
	OrderNo              string
	UserID               uint64
	AgentID              uint64
	AgentEmail           string
	LevelID              uint64
	LevelCode            string
	LevelName            string
	OpeningFeeCents      int64
	TransferredCents     int64
	OpeningBonusCents    int64
	FinalBalanceCents    int64
	MigratedLicenseCount int64
}

type accountUpgradeOrder struct {
	ID               uint64
	OrderNo          string
	UserID           uint64
	LevelID          uint64
	LevelCode        string
	LevelName        string
	Discount         float64
	OpeningBonusText string
	AmountText       string
	PayChannel       string
	PayMethod        string
	GatewayTradeNo   string
	ReturnURL        string
	Status           string
	AgentID          sql.NullInt64
}

type accountUpgradeUser struct {
	ID            uint64
	Email         string
	Phone         sql.NullString
	PasswordHash  string
	Nickname      string
	BalanceText   string
	AccountStatus string
	Enabled       bool
	RealName      string
	RealIDCard    string
	RealnameAt    sql.NullTime
}

func insertAgentUpgradeOrder(tx *sql.Tx, order accountUpgradeOrder) error {
	_, err := tx.Exec(`
		INSERT INTO agent_upgrade_orders (
			order_no, user_id, level_id, level_code_snapshot, level_name_snapshot,
			discount_snapshot, opening_bonus_snapshot, amount, pay_channel, pay_method, status, return_url
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)
	`, order.OrderNo, order.UserID, order.LevelID, order.LevelCode, order.LevelName, order.Discount,
		order.OpeningBonusText, order.AmountText, order.PayChannel, order.PayMethod, order.ReturnURL)
	return err
}

func settleAgentUpgradeOrder(db *sql.DB, orderNo string, payment accountUpgradePayment) (accountConversionResult, error) {
	tx, err := db.Begin()
	if err != nil {
		return accountConversionResult{}, err
	}
	defer tx.Rollback()

	result, err := settleAgentUpgradeOrderTx(tx, orderNo, payment)
	if err != nil {
		return accountConversionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return accountConversionResult{}, err
	}
	return result, nil
}

func settleAgentUpgradeOrderTx(tx *sql.Tx, orderNo string, payment accountUpgradePayment) (accountConversionResult, error) {
	order, err := lockAccountUpgradeOrder(tx, orderNo)
	if err != nil {
		return accountConversionResult{}, err
	}
	expectedCents, err := parseAmountToCents(order.AmountText)
	if err != nil || expectedCents != payment.PaidCents {
		return accountConversionResult{}, errAgentUpgradeAmountMismatch
	}
	openingBonusCents, err := parseAmountToCents(order.OpeningBonusText)
	if err != nil || openingBonusCents < 0 || openingBonusCents > maxAgentLevelMoneyCents {
		return accountConversionResult{}, errAgentUpgradeAmountMismatch
	}
	if order.PayChannel != "" && payment.PayChannel != "" && order.PayChannel != payment.PayChannel {
		return accountConversionResult{}, errAgentUpgradePaymentUnavailable
	}
	if order.PayMethod != "" && payment.PayMethod != "" && order.PayMethod != payment.PayMethod {
		return accountConversionResult{}, errAgentUpgradePaymentUnavailable
	}
	if order.GatewayTradeNo != "" && payment.GatewayTradeNo != "" && order.GatewayTradeNo != payment.GatewayTradeNo {
		return accountConversionResult{}, errAgentUpgradePaymentUnavailable
	}
	if order.Status == "completed" {
		return loadCompletedAccountConversion(tx, order)
	}
	if order.Status != "pending" && order.Status != "paid" {
		return accountConversionResult{}, errAgentUpgradeOrderState
	}

	user, balanceCents, err := lockAccountUpgradeUser(tx, order.UserID)
	if err != nil {
		return accountConversionResult{}, err
	}
	if user.AccountStatus == "converted" {
		return accountConversionResult{}, errAgentUpgradeUserConverted
	}
	if !user.Enabled {
		return accountConversionResult{}, errAgentUpgradeUserDisabled
	}
	if payment.DeductOpeningFee && balanceCents < expectedCents {
		return accountConversionResult{}, errAgentUpgradeInsufficientFunds
	}

	if payment.DeductOpeningFee {
		var levelEnabled, selfServiceEnabled bool
		var currentLevelCode, currentLevelName string
		var currentDiscount float64
		var currentPriceText string
		err = tx.QueryRow(`
			SELECT code, name, discount, upgrade_price, enabled, self_service_enabled
			FROM agent_levels WHERE id = ? FOR UPDATE
		`, order.LevelID).Scan(&currentLevelCode, &currentLevelName, &currentDiscount, &currentPriceText, &levelEnabled, &selfServiceEnabled)
		if err != nil || !levelEnabled || !selfServiceEnabled {
			return accountConversionResult{}, errAgentUpgradeLevelUnavailable
		}
		currentPriceCents, err := parseAmountToCents(currentPriceText)
		if err != nil || currentPriceCents != expectedCents || currentLevelCode != order.LevelCode {
			return accountConversionResult{}, errAgentUpgradeAmountMismatch
		}
	}

	var conflictCount int
	if err := tx.QueryRow(`
		SELECT COUNT(*) FROM agents
		WHERE email = ? OR (? <> '' AND contact = ?)
	`, user.Email, user.Phone.String, user.Phone.String).Scan(&conflictCount); err != nil {
		return accountConversionResult{}, err
	}
	if conflictCount > 0 {
		return accountConversionResult{}, errAgentUpgradeAccountConflict
	}

	var pendingOrderCount int
	if err := tx.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM license_purchase_orders
			 WHERE owner_type = 'user' AND owner_id = ? AND status = 'pending') +
			(SELECT COUNT(*) FROM recharge_orders
			 WHERE subject_type = 'user' AND subject_id = ? AND status = 'pending') +
			(SELECT COUNT(*) FROM agent_upgrade_orders
			 WHERE user_id = ? AND id <> ? AND status IN ('pending','paid','processing'))
	`, user.ID, user.ID, user.ID, order.ID).Scan(&pendingOrderCount); err != nil {
		return accountConversionResult{}, err
	}
	if pendingOrderCount > 0 {
		return accountConversionResult{}, errAgentUpgradePendingOrder
	}

	if _, err := tx.Exec(`UPDATE agent_upgrade_orders SET status = 'processing', error_message = NULL WHERE id = ?`, order.ID); err != nil {
		return accountConversionResult{}, err
	}

	remainingCents := balanceCents
	var balanceAfterFee any
	if payment.DeductOpeningFee {
		remainingCents -= expectedCents
		balanceAfterFee = formatCents(remainingCents)
	}
	if remainingCents > maxAgentLevelMoneyCents-openingBonusCents {
		return accountConversionResult{}, errors.New("赠送后代理余额超过账户上限")
	}
	finalAgentBalanceCents := remainingCents + openingBonusCents

	agentName := strings.TrimSpace(user.Nickname)
	if agentName == "" {
		agentName = user.Email
	}
	contact := strings.TrimSpace(user.Phone.String)
	if contact == "" {
		contact = user.Email
	}
	now := time.Now()
	agentResult, err := tx.Exec(`
		INSERT INTO agents (
			email, password_hash, name, contact, level, discount, balance,
			source, original_user_id, converted_at, remark, enabled,
			real_name, real_id_card, realname_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'user_upgrade', ?, ?, ?, 1, ?, ?, ?)
	`, user.Email, user.PasswordHash, agentName, contact, order.LevelCode, order.Discount,
		formatCents(finalAgentBalanceCents), user.ID, now, "用户自助开通代理", user.RealName, user.RealIDCard, user.RealnameAt)
	if err != nil {
		return accountConversionResult{}, fmt.Errorf("创建代理账户失败: %w", err)
	}
	agentID, err := agentResult.LastInsertId()
	if err != nil || agentID <= 0 {
		return accountConversionResult{}, fmt.Errorf("获取代理账户ID失败")
	}

	licenseResult, err := tx.Exec(`
		UPDATE licenses SET owner_type = 'agent', owner_id = ?
		WHERE owner_type = 'user' AND owner_id = ?
	`, agentID, user.ID)
	if err != nil {
		return accountConversionResult{}, fmt.Errorf("迁移授权归属失败: %w", err)
	}
	migratedLicenseCount, err := licenseResult.RowsAffected()
	if err != nil {
		return accountConversionResult{}, fmt.Errorf("读取授权迁移数量失败: %w", err)
	}
	if _, err := tx.Exec(`
		UPDATE license_purchase_orders SET owner_type = 'agent', owner_id = ?
		WHERE owner_type = 'user' AND owner_id = ? AND status = 'paid'
	`, agentID, user.ID); err != nil {
		return accountConversionResult{}, fmt.Errorf("迁移授权订单归属失败: %w", err)
	}

	if _, err := tx.Exec(`
		INSERT INTO transactions (tx_no, subject_type, subject_id, type, amount, balance_after, ref_type, ref_id, remark)
		VALUES (?, 'user', ?, 'consume', ?, ?, 'agent_upgrade_order', ?, ?)
	`, generateTransactionNo(), user.ID, formatCents(-expectedCents), balanceAfterFee, order.ID,
		fmt.Sprintf("%s支付开通%s", payMethodLabel(payment.PayMethod), order.LevelName)); err != nil {
		return accountConversionResult{}, fmt.Errorf("记录代理开通费流水失败: %w", err)
	}

	if remainingCents > 0 {
		transferNo := fmt.Sprintf("TR%d%06d", time.Now().Unix(), time.Now().Nanosecond()%1_000_000)
		if _, err := tx.Exec(`
			INSERT INTO transactions (tx_no, subject_type, subject_id, type, amount, balance_after, ref_type, ref_id, remark)
			VALUES (?, 'user', ?, 'transfer', ?, 0, 'account_conversion', ?, '用户余额转出至代理账户')
		`, transferNo+"U", user.ID, formatCents(-remainingCents), order.ID); err != nil {
			return accountConversionResult{}, fmt.Errorf("记录用户余额转出流水失败: %w", err)
		}
		if _, err := tx.Exec(`
			INSERT INTO transactions (tx_no, subject_type, subject_id, type, amount, balance_after, ref_type, ref_id, remark)
			VALUES (?, 'agent', ?, 'transfer', ?, ?, 'account_conversion', ?, '用户余额迁入代理账户')
		`, transferNo+"A", agentID, formatCents(remainingCents), formatCents(remainingCents), order.ID); err != nil {
			return accountConversionResult{}, fmt.Errorf("记录代理余额转入流水失败: %w", err)
		}
	}
	if openingBonusCents > 0 {
		if _, err := tx.Exec(`
			INSERT INTO transactions (tx_no, subject_type, subject_id, type, amount, balance_after, ref_type, ref_id, remark)
			VALUES (?, 'agent', ?, 'bonus', ?, ?, 'agent_upgrade_bonus', ?, ?)
		`, fmt.Sprintf("AB%d", order.ID), agentID, formatCents(openingBonusCents), formatCents(finalAgentBalanceCents), order.ID,
			fmt.Sprintf("开通%s赠送余额", order.LevelName)); err != nil {
			return accountConversionResult{}, fmt.Errorf("记录代理开通赠送流水失败: %w", err)
		}
	}

	if _, err := tx.Exec(`
		UPDATE user_password_resets SET used_at = COALESCE(used_at, NOW())
		WHERE user_id = ? AND used_at IS NULL
	`, user.ID); err != nil {
		return accountConversionResult{}, fmt.Errorf("作废密码重置令牌失败: %w", err)
	}
	if _, err := tx.Exec(`
		UPDATE user_email_codes SET used_at = COALESCE(used_at, NOW())
		WHERE email = ? AND used_at IS NULL
	`, user.Email); err != nil {
		return accountConversionResult{}, fmt.Errorf("作废用户验证码失败: %w", err)
	}

	userUpdate, err := tx.Exec(`
		UPDATE users
		SET balance = 0, account_status = 'converted', converted_agent_id = ?, converted_at = ?, enabled = 0
		WHERE id = ? AND account_status = 'active' AND enabled = 1
	`, agentID, now, user.ID)
	if err != nil {
		return accountConversionResult{}, fmt.Errorf("禁用原用户账户失败: %w", err)
	}
	if affected, _ := userUpdate.RowsAffected(); affected != 1 {
		return accountConversionResult{}, errAgentUpgradeUserConverted
	}

	sourceSnapshot, err := json.Marshal(map[string]any{
		"email":         user.Email,
		"phone":         user.Phone.String,
		"nickname":      user.Nickname,
		"realName":      user.RealName,
		"realnameAt":    user.RealnameAt.Time,
		"balanceBefore": formatCents(balanceCents),
	})
	if err != nil {
		return accountConversionResult{}, err
	}
	resultSnapshot, err := json.Marshal(map[string]any{
		"agentId":              agentID,
		"levelCode":            order.LevelCode,
		"openingFee":           formatCents(expectedCents),
		"transferredBalance":   formatCents(remainingCents),
		"openingBonus":         formatCents(openingBonusCents),
		"finalAgentBalance":    formatCents(finalAgentBalanceCents),
		"migratedLicenseCount": migratedLicenseCount,
	})
	if err != nil {
		return accountConversionResult{}, err
	}
	conversionNo := fmt.Sprintf("AC%d%06d", time.Now().Unix(), time.Now().Nanosecond()%1_000_000)
	if _, err := tx.Exec(`
		INSERT INTO account_conversions (
			conversion_no, upgrade_order_id, user_id, agent_id, level_id, status,
			opening_fee, transferred_balance, opening_bonus, migrated_license_count,
			source_snapshot, result_snapshot, completed_at
		) VALUES (?, ?, ?, ?, ?, 'completed', ?, ?, ?, ?, ?, ?, ?)
	`, conversionNo, order.ID, user.ID, agentID, order.LevelID, formatCents(expectedCents),
		formatCents(remainingCents), formatCents(openingBonusCents), migratedLicenseCount, sourceSnapshot, resultSnapshot, now); err != nil {
		return accountConversionResult{}, fmt.Errorf("记录账户转换审计失败: %w", err)
	}

	payChannel := strings.TrimSpace(payment.PayChannel)
	if payChannel == "" {
		payChannel = "easypay"
	}
	payMethod := strings.TrimSpace(payment.PayMethod)
	if payMethod == "" {
		payMethod = payChannel
	}
	if _, err := tx.Exec(`
		UPDATE agent_upgrade_orders
		SET status = 'completed', paid_amount = ?, pay_channel = ?, pay_method = ?,
			gateway_trade_no = ?, notify_payload = ?, agent_id = ?,
			paid_at = COALESCE(paid_at, ?), completed_at = ?, error_message = NULL
		WHERE id = ?
	`, formatCents(payment.PaidCents), payChannel, payMethod, payment.GatewayTradeNo,
		payment.NotifyPayload, agentID, now, now, order.ID); err != nil {
		return accountConversionResult{}, fmt.Errorf("完成代理升级订单失败: %w", err)
	}

	return accountConversionResult{
		OrderID:              order.ID,
		OrderNo:              order.OrderNo,
		UserID:               user.ID,
		AgentID:              uint64(agentID),
		AgentEmail:           user.Email,
		LevelID:              order.LevelID,
		LevelCode:            order.LevelCode,
		LevelName:            order.LevelName,
		OpeningFeeCents:      expectedCents,
		TransferredCents:     remainingCents,
		OpeningBonusCents:    openingBonusCents,
		FinalBalanceCents:    finalAgentBalanceCents,
		MigratedLicenseCount: migratedLicenseCount,
	}, nil
}

func lockAccountUpgradeOrder(tx *sql.Tx, orderNo string) (accountUpgradeOrder, error) {
	var order accountUpgradeOrder
	err := tx.QueryRow(`
		SELECT id, order_no, user_id, level_id, level_code_snapshot, level_name_snapshot,
		       discount_snapshot, opening_bonus_snapshot, amount, pay_channel, pay_method, COALESCE(gateway_trade_no, ''), status, agent_id
		FROM agent_upgrade_orders WHERE order_no = ? FOR UPDATE
	`, orderNo).Scan(&order.ID, &order.OrderNo, &order.UserID, &order.LevelID, &order.LevelCode,
		&order.LevelName, &order.Discount, &order.OpeningBonusText, &order.AmountText, &order.PayChannel, &order.PayMethod,
		&order.GatewayTradeNo, &order.Status, &order.AgentID)
	if errors.Is(err, sql.ErrNoRows) {
		return accountUpgradeOrder{}, errAgentUpgradeOrderNotFound
	}
	return order, err
}

func lockAccountUpgradeUser(tx *sql.Tx, userID uint64) (accountUpgradeUser, int64, error) {
	var user accountUpgradeUser
	err := tx.QueryRow(`
		SELECT id, email, phone, password_hash, COALESCE(nickname, ''), balance,
		       account_status, enabled, COALESCE(real_name, ''), COALESCE(real_id_card, ''), realname_at
		FROM users WHERE id = ? FOR UPDATE
	`, userID).Scan(&user.ID, &user.Email, &user.Phone, &user.PasswordHash, &user.Nickname,
		&user.BalanceText, &user.AccountStatus, &user.Enabled, &user.RealName, &user.RealIDCard, &user.RealnameAt)
	if err != nil {
		return accountUpgradeUser{}, 0, err
	}
	balanceCents, err := parseAmountToCents(user.BalanceText)
	if err != nil {
		return accountUpgradeUser{}, 0, fmt.Errorf("用户余额格式错误: %w", err)
	}
	return user, balanceCents, nil
}

func loadCompletedAccountConversion(tx *sql.Tx, order accountUpgradeOrder) (accountConversionResult, error) {
	var result accountConversionResult
	var openingFeeText, transferredText, openingBonusText string
	err := tx.QueryRow(`
		SELECT c.user_id, c.agent_id, a.email, c.level_id, o.level_code_snapshot,
		       o.level_name_snapshot, c.opening_fee, c.transferred_balance, c.opening_bonus,
		       c.migrated_license_count
		FROM account_conversions c
		JOIN agent_upgrade_orders o ON o.id = c.upgrade_order_id
		JOIN agents a ON a.id = c.agent_id
		WHERE c.upgrade_order_id = ? AND c.status = 'completed'
	`, order.ID).Scan(&result.UserID, &result.AgentID, &result.AgentEmail, &result.LevelID,
		&result.LevelCode, &result.LevelName, &openingFeeText, &transferredText, &openingBonusText,
		&result.MigratedLicenseCount)
	if err != nil {
		return accountConversionResult{}, err
	}
	result.OpeningFeeCents, err = parseAmountToCents(openingFeeText)
	if err != nil {
		return accountConversionResult{}, err
	}
	result.TransferredCents, err = parseAmountToCents(transferredText)
	if err != nil {
		return accountConversionResult{}, err
	}
	result.OpeningBonusCents, err = parseAmountToCents(openingBonusText)
	if err != nil {
		return accountConversionResult{}, err
	}
	if result.TransferredCents > maxAgentLevelMoneyCents-result.OpeningBonusCents {
		return accountConversionResult{}, errors.New("转换后代理余额数据异常")
	}
	result.FinalBalanceCents = result.TransferredCents + result.OpeningBonusCents
	result.OrderID = order.ID
	result.OrderNo = order.OrderNo
	return result, nil
}

func openAccountUpgradeDB() (*sql.DB, error) {
	cfg, err := config.LoadDBConfig()
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		return nil, err
	}
	if err := EnsureAccountUpgradeSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureLicensePurchaseOrderSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureRechargeOrderSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// UserAgentUpgradeLevels returns only levels explicitly opened for self-service.
func UserAgentUpgradeLevels(c *gin.Context) {
	userID, ok := getUserPanelID(c)
	if !ok {
		return
	}
	db, err := openAccountUpgradeDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "代理升级服务初始化失败"})
		return
	}
	defer db.Close()

	var balanceText, accountStatus string
	var enabled bool
	var convertedAgentID sql.NullInt64
	if err := db.QueryRow(`
		SELECT balance, account_status, enabled, converted_agent_id
		FROM users WHERE id = ?
	`, userID).Scan(&balanceText, &accountStatus, &enabled, &convertedAgentID); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "用户账户不存在"})
		return
	}
	balanceCents, err := parseAmountToCents(balanceText)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "账户余额数据异常"})
		return
	}
	if accountStatus == "converted" || convertedAgentID.Valid {
		c.JSON(http.StatusOK, gin.H{"code": 409, "msg": "当前账号已升级为代理，请前往代理端登录", "data": gin.H{"agentId": convertedAgentID.Int64}})
		return
	}
	if !enabled {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "当前账户已禁用"})
		return
	}

	rows, err := db.Query(`
		SELECT id, code, name, discount, upgrade_price, opening_bonus, COALESCE(benefits, ''), COALESCE(remark, '')
		FROM agent_levels
		WHERE enabled = 1 AND self_service_enabled = 1
		ORDER BY sort ASC, id ASC
	`)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询代理等级失败"})
		return
	}
	defer rows.Close()

	type levelItem struct {
		ID                  uint64  `json:"id"`
		Code                string  `json:"code"`
		Name                string  `json:"name"`
		Discount            float64 `json:"discount"`
		Price               float64 `json:"price"`
		OpeningBonus        float64 `json:"openingBonus"`
		Benefits            string  `json:"benefits"`
		Remark              string  `json:"remark"`
		CanAfford           bool    `json:"canAfford"`
		BalanceAfterUpgrade float64 `json:"balanceAfterUpgrade"`
		TransferredBalance  float64 `json:"transferredBalance"`
		FinalAgentBalance   float64 `json:"finalAgentBalance"`
	}
	list := make([]levelItem, 0)
	for rows.Next() {
		var item levelItem
		var priceText, openingBonusText string
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Discount, &priceText, &openingBonusText,
			&item.Benefits, &item.Remark); err != nil {
			continue
		}
		priceCents, err := parseAmountToCents(priceText)
		if err != nil {
			continue
		}
		openingBonusCents, err := parseAmountToCents(openingBonusText)
		if err != nil {
			continue
		}
		remainingCents := balanceCents - priceCents
		if remainingCents < 0 {
			remainingCents = 0
		}
		item.Price = float64(priceCents) / 100
		item.OpeningBonus = float64(openingBonusCents) / 100
		item.CanAfford = balanceCents >= priceCents
		item.BalanceAfterUpgrade = float64(remainingCents) / 100
		item.TransferredBalance = item.BalanceAfterUpgrade
		item.FinalAgentBalance = float64(remainingCents+openingBonusCents) / 100
		list = append(list, item)
	}

	payOptions := []payOption{{Code: "balance", Label: "余额支付", Icon: "ri:wallet-3-line", Color: "#2e7d32"}}
	payOptions = append(payOptions, configuredOnlinePayOptions(db)...)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"balance":      float64(balanceCents) / 100,
			"levels":       list,
			"payOptions":   dedupePayOptions(payOptions),
			"irreversible": true,
		},
	})
}

// UserAgentUpgradeCreate creates a balance order and converts the account atomically.
func UserAgentUpgradeCreate(c *gin.Context) {
	userID, ok := getUserPanelID(c)
	if !ok {
		return
	}
	var req struct {
		LevelID   uint64 `json:"levelId" binding:"required"`
		PayMethod string `json:"payMethod"`
		Confirm   bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.LevelID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请选择代理等级"})
		return
	}
	if !req.Confirm {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请确认账户升级为不可逆操作"})
		return
	}

	db, err := openAccountUpgradeDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "代理升级服务初始化失败"})
		return
	}
	defer db.Close()

	payMethod := strings.TrimSpace(req.PayMethod)
	if payMethod == "" {
		payMethod = "balance"
	}
	if payMethod == "balance" {
		result, err := createBalanceAgentUpgrade(db, uint64(userID), req.LevelID)
		if err != nil {
			writeAgentUpgradeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "代理账户开通成功，请使用原账号密码登录代理端",
			"data": accountConversionResponse(result),
		})
		return
	}

	result, err := createOnlineAgentUpgrade(c, db, uint64(userID), req.LevelID, payMethod)
	if err != nil {
		writeAgentUpgradeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "支付订单已创建，正在跳转收银台",
		"data": result,
	})
}

type agentUpgradeOnlineOrderResponse struct {
	OrderNo    string  `json:"orderNo"`
	Amount     float64 `json:"amount"`
	PayChannel string  `json:"payChannel"`
	PayMethod  string  `json:"payMethod"`
	PayURL     string  `json:"payUrl"`
	Status     string  `json:"status"`
}

func createOnlineAgentUpgrade(c *gin.Context, db *sql.DB, userID, levelID uint64, payMethod string) (agentUpgradeOnlineOrderResponse, error) {
	selection, ok := parseOnlinePaySelection(payMethod)
	if !ok {
		return agentUpgradeOnlineOrderResponse{}, errAgentUpgradePaymentUnavailable
	}
	if selection.Channel == "" {
		if cfg, err := loadEpayConfig(db); err == nil && cfg.validateForPay() == nil && cfg.isPayTypeEnabled(selection.PayType) {
			selection.Channel = payChannelEpayV1
		} else if cfg, err := loadEpayV2Config(db); err == nil && cfg.validateForPay() == nil && cfg.isPayTypeEnabled(selection.PayType) {
			selection.Channel = payChannelEpayV2
		} else {
			return agentUpgradeOnlineOrderResponse{}, errAgentUpgradePaymentUnavailable
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return agentUpgradeOnlineOrderResponse{}, err
	}
	defer tx.Rollback()

	var accountStatus string
	var enabled bool
	var convertedAgentID sql.NullInt64
	if err := tx.QueryRow(`
		SELECT account_status, enabled, converted_agent_id
		FROM users WHERE id = ? FOR UPDATE
	`, userID).Scan(&accountStatus, &enabled, &convertedAgentID); err != nil {
		return agentUpgradeOnlineOrderResponse{}, err
	}
	if accountStatus == "converted" || convertedAgentID.Valid {
		return agentUpgradeOnlineOrderResponse{}, errAgentUpgradeUserConverted
	}
	if !enabled {
		return agentUpgradeOnlineOrderResponse{}, errAgentUpgradeUserDisabled
	}

	var levelCode, levelName, priceText, openingBonusText string
	var discount float64
	var levelEnabled, selfServiceEnabled bool
	if err := tx.QueryRow(`
		SELECT code, name, discount, upgrade_price, opening_bonus, enabled, self_service_enabled
		FROM agent_levels WHERE id = ? FOR UPDATE
	`, levelID).Scan(&levelCode, &levelName, &discount, &priceText, &openingBonusText, &levelEnabled, &selfServiceEnabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return agentUpgradeOnlineOrderResponse{}, errAgentUpgradeLevelUnavailable
		}
		return agentUpgradeOnlineOrderResponse{}, err
	}
	if !levelEnabled || !selfServiceEnabled {
		return agentUpgradeOnlineOrderResponse{}, errAgentUpgradeLevelUnavailable
	}
	priceCents, err := parseAmountToCents(priceText)
	if err != nil || priceCents < minRechargeCents {
		return agentUpgradeOnlineOrderResponse{}, errAgentUpgradeAmountMismatch
	}
	openingBonusCents, err := parseAmountToCents(openingBonusText)
	if err != nil || openingBonusCents < 0 || openingBonusCents > maxAgentLevelMoneyCents {
		return agentUpgradeOnlineOrderResponse{}, errAgentUpgradeAmountMismatch
	}

	var activeOrderCount int
	if err := tx.QueryRow(`
		SELECT COUNT(*) FROM agent_upgrade_orders
		WHERE user_id = ? AND status IN ('pending','paid','processing')
	`, userID).Scan(&activeOrderCount); err != nil {
		return agentUpgradeOnlineOrderResponse{}, err
	}
	if activeOrderCount > 0 {
		return agentUpgradeOnlineOrderResponse{}, errAgentUpgradePendingOrder
	}

	orderNo := fmt.Sprintf("AU%d%06d", time.Now().Unix(), time.Now().Nanosecond()%1_000_000)
	returnPath := "/user/become-agent"
	frontendReturnURL := buildFrontendReturnURL(c, orderNo, returnPath)
	if err := insertAgentUpgradeOrder(tx, accountUpgradeOrder{
		OrderNo: orderNo, UserID: userID, LevelID: levelID, LevelCode: levelCode, LevelName: levelName,
		Discount: discount, OpeningBonusText: formatCents(openingBonusCents), AmountText: formatCents(priceCents),
		PayChannel: selection.Channel, PayMethod: selection.PayType, ReturnURL: frontendReturnURL,
	}); err != nil {
		return agentUpgradeOnlineOrderResponse{}, fmt.Errorf("%w: %v", errAgentUpgradePaymentCreate, err)
	}
	if err := tx.Commit(); err != nil {
		return agentUpgradeOnlineOrderResponse{}, fmt.Errorf("%w: %v", errAgentUpgradePaymentCreate, err)
	}

	orderName := fmt.Sprintf("开通代理 %s", levelName)
	payURL := ""
	switch selection.Channel {
	case payChannelEpayV1:
		cfg, err := loadEpayConfig(db)
		if err != nil || cfg.validateForPay() != nil || !cfg.isPayTypeEnabled(selection.PayType) {
			markAgentUpgradeOrderFailed(db, orderNo, "易支付 V1 通道不可用")
			return agentUpgradeOnlineOrderResponse{}, errAgentUpgradePaymentUnavailable
		}
		payURL, _, err = buildEpaySubmitURL(c, cfg, orderNo, priceCents, selection.PayType, orderName, returnPath)
		if err != nil {
			markAgentUpgradeOrderFailed(db, orderNo, err.Error())
			return agentUpgradeOnlineOrderResponse{}, fmt.Errorf("%w: %v", errAgentUpgradePaymentCreate, err)
		}
	case payChannelEpayV2:
		cfg, err := loadEpayV2Config(db)
		if err != nil || cfg.validateForPay() != nil || !cfg.isPayTypeEnabled(selection.PayType) {
			markAgentUpgradeOrderFailed(db, orderNo, "易支付 V2 通道不可用")
			return agentUpgradeOnlineOrderResponse{}, errAgentUpgradePaymentUnavailable
		}
		payURL, _, err = buildEpayV2Payment(c, cfg, orderNo, priceCents, selection.PayType, orderName, returnPath)
		if err != nil {
			markAgentUpgradeOrderFailed(db, orderNo, err.Error())
			return agentUpgradeOnlineOrderResponse{}, fmt.Errorf("%w: %v", errAgentUpgradePaymentCreate, err)
		}
	default:
		markAgentUpgradeOrderFailed(db, orderNo, "未知支付通道")
		return agentUpgradeOnlineOrderResponse{}, errAgentUpgradePaymentUnavailable
	}

	return agentUpgradeOnlineOrderResponse{
		OrderNo:    orderNo,
		Amount:     float64(priceCents) / 100,
		PayChannel: selection.Channel,
		PayMethod:  selection.PayType,
		PayURL:     payURL,
		Status:     "pending",
	}, nil
}

func markAgentUpgradeOrderFailed(db *sql.DB, orderNo, message string) {
	_, _ = db.Exec(`
		UPDATE agent_upgrade_orders
		SET status = 'failed', error_message = ?
		WHERE order_no = ? AND status = 'pending'
	`, "支付网关下单失败: "+strings.TrimSpace(message), orderNo)
}

// UserAgentUpgradeCancel cancels an unpaid online order owned by the current user.
func UserAgentUpgradeCancel(c *gin.Context) {
	userID, ok := getUserPanelID(c)
	if !ok {
		return
	}
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "订单号不能为空"})
		return
	}
	db, err := openAccountUpgradeDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "代理升级服务初始化失败"})
		return
	}
	defer db.Close()

	cancelled, err := cancelPendingAgentUpgradeOrder(db, uint64(userID), orderNo)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "取消支付订单失败"})
		return
	}
	if !cancelled {
		c.JSON(http.StatusOK, gin.H{"code": 409, "msg": "订单已支付、已取消或不可取消"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "支付订单已取消"})
}

func cancelPendingAgentUpgradeOrder(db *sql.DB, userID uint64, orderNo string) (bool, error) {
	result, err := db.Exec(`
		UPDATE agent_upgrade_orders
		SET status = 'cancelled', error_message = '用户取消支付'
		WHERE order_no = ? AND user_id = ? AND status = 'pending' AND pay_channel <> 'balance'
	`, orderNo, userID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

// settleAgentUpgradeOnlinePayment records a verified payment before attempting account conversion.
// A conversion failure leaves the order in paid state with an auditable error and can be retried safely.
func settleAgentUpgradeOnlinePayment(db *sql.DB, orderNo string, paidCents int64, payChannel, payMethod, gatewayTradeNo, notifyPayload string) error {
	payChannel = strings.TrimSpace(payChannel)
	payMethod = strings.TrimSpace(payMethod)
	gatewayTradeNo = strings.TrimSpace(gatewayTradeNo)
	if orderNo == "" || paidCents <= 0 || payChannel == "" || payMethod == "" || gatewayTradeNo == "" {
		return errors.New("支付回调参数不完整")
	}
	if err := recordAgentUpgradeOnlinePayment(db, orderNo, paidCents, payChannel, payMethod, gatewayTradeNo, notifyPayload); err != nil {
		return err
	}

	_, err := settleAgentUpgradeOrder(db, orderNo, accountUpgradePayment{
		PaidCents:        paidCents,
		PayChannel:       payChannel,
		PayMethod:        payMethod,
		GatewayTradeNo:   gatewayTradeNo,
		NotifyPayload:    notifyPayload,
		DeductOpeningFee: false,
	})
	if err != nil {
		_, _ = db.Exec(`
			UPDATE agent_upgrade_orders
			SET error_message = ?
			WHERE order_no = ? AND status = 'paid'
		`, "支付成功，账户转换失败: "+err.Error(), orderNo)
	}
	return err
}

func recordAgentUpgradeOnlinePayment(db *sql.DB, orderNo string, paidCents int64, payChannel, payMethod, gatewayTradeNo, notifyPayload string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var id uint64
	var amountText, expectedChannel, expectedMethod, currentTradeNo, status string
	err = tx.QueryRow(`
		SELECT id, amount, pay_channel, pay_method, COALESCE(gateway_trade_no, ''), status
		FROM agent_upgrade_orders
		WHERE order_no = ?
		FOR UPDATE
	`, orderNo).Scan(&id, &amountText, &expectedChannel, &expectedMethod, &currentTradeNo, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return errAgentUpgradeOrderNotFound
	}
	if err != nil {
		return err
	}

	expectedCents, err := parseAmountToCents(amountText)
	if err != nil || expectedCents != paidCents {
		return errAgentUpgradeAmountMismatch
	}
	if expectedChannel != payChannel || expectedMethod != payMethod {
		return errAgentUpgradePaymentUnavailable
	}
	if currentTradeNo != "" && currentTradeNo != gatewayTradeNo {
		return errAgentUpgradePaymentUnavailable
	}
	if status == "completed" || status == "paid" {
		return tx.Commit()
	}
	if status != "pending" {
		return errAgentUpgradeOrderState
	}

	var duplicateTradeCount int
	if err := tx.QueryRow(`
		SELECT COUNT(*) FROM agent_upgrade_orders
		WHERE gateway_trade_no = ? AND id <> ?
	`, gatewayTradeNo, id).Scan(&duplicateTradeCount); err != nil {
		return err
	}
	if duplicateTradeCount > 0 {
		return errAgentUpgradePaymentUnavailable
	}

	if _, err := tx.Exec(`
		UPDATE agent_upgrade_orders
		SET status = 'paid', paid_amount = ?, gateway_trade_no = ?,
			paid_at = COALESCE(paid_at, NOW()), notify_payload = ?, error_message = NULL
		WHERE id = ? AND status = 'pending'
	`, formatCents(paidCents), gatewayTradeNo, notifyPayload, id); err != nil {
		return err
	}
	return tx.Commit()
}

func createBalanceAgentUpgrade(db *sql.DB, userID, levelID uint64) (accountConversionResult, error) {
	tx, err := db.Begin()
	if err != nil {
		return accountConversionResult{}, err
	}
	defer tx.Rollback()

	var accountStatus string
	var enabled bool
	var convertedAgentID sql.NullInt64
	if err := tx.QueryRow(`
		SELECT account_status, enabled, converted_agent_id
		FROM users WHERE id = ? FOR UPDATE
	`, userID).Scan(&accountStatus, &enabled, &convertedAgentID); err != nil {
		return accountConversionResult{}, err
	}
	if accountStatus == "converted" || convertedAgentID.Valid {
		return loadCompletedAccountConversionByUser(tx, userID)
	}
	if !enabled {
		return accountConversionResult{}, errAgentUpgradeUserDisabled
	}

	var levelCode, levelName, priceText, openingBonusText string
	var discount float64
	var levelEnabled, selfServiceEnabled bool
	if err := tx.QueryRow(`
		SELECT code, name, discount, upgrade_price, opening_bonus, enabled, self_service_enabled
		FROM agent_levels WHERE id = ? FOR UPDATE
	`, levelID).Scan(&levelCode, &levelName, &discount, &priceText, &openingBonusText, &levelEnabled, &selfServiceEnabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return accountConversionResult{}, errAgentUpgradeLevelUnavailable
		}
		return accountConversionResult{}, err
	}
	if !levelEnabled || !selfServiceEnabled {
		return accountConversionResult{}, errAgentUpgradeLevelUnavailable
	}
	priceCents, err := parseAmountToCents(priceText)
	if err != nil || priceCents < 0 {
		return accountConversionResult{}, errAgentUpgradeAmountMismatch
	}
	openingBonusCents, err := parseAmountToCents(openingBonusText)
	if err != nil || openingBonusCents < 0 || openingBonusCents > maxAgentLevelMoneyCents {
		return accountConversionResult{}, errAgentUpgradeAmountMismatch
	}

	var activeOrderCount int
	if err := tx.QueryRow(`
		SELECT COUNT(*) FROM agent_upgrade_orders
		WHERE user_id = ? AND status IN ('pending','paid','processing')
	`, userID).Scan(&activeOrderCount); err != nil {
		return accountConversionResult{}, err
	}
	if activeOrderCount > 0 {
		return accountConversionResult{}, errAgentUpgradePendingOrder
	}

	orderNo := fmt.Sprintf("AU%d%06d", time.Now().Unix(), time.Now().Nanosecond()%1_000_000)
	if err := insertAgentUpgradeOrder(tx, accountUpgradeOrder{
		OrderNo: orderNo, UserID: userID, LevelID: levelID, LevelCode: levelCode, LevelName: levelName,
		Discount: discount, OpeningBonusText: formatCents(openingBonusCents), AmountText: formatCents(priceCents),
		PayChannel: "balance", PayMethod: "balance",
	}); err != nil {
		return accountConversionResult{}, fmt.Errorf("创建代理升级订单失败: %w", err)
	}

	result, err := settleAgentUpgradeOrderTx(tx, orderNo, accountUpgradePayment{
		PaidCents:        priceCents,
		PayChannel:       "balance",
		PayMethod:        "balance",
		DeductOpeningFee: true,
	})
	if err != nil {
		return accountConversionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return accountConversionResult{}, err
	}
	return result, nil
}

// UserAgentUpgradeOrderStatus returns an order owned by the current user.
func UserAgentUpgradeOrderStatus(c *gin.Context) {
	userID, ok := getUserPanelID(c)
	if !ok {
		return
	}
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "订单号不能为空"})
		return
	}
	db, err := openAccountUpgradeDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "代理升级服务初始化失败"})
		return
	}
	defer db.Close()

	var status, levelCode, levelName, amountText, openingBonusText, payChannel, payMethod, returnURL, errorMessage string
	var agentID sql.NullInt64
	var paidAt, completedAt sql.NullTime
	err = db.QueryRow(`
		SELECT status, level_code_snapshot, level_name_snapshot, amount, opening_bonus_snapshot,
		       pay_channel, pay_method, return_url, COALESCE(error_message, ''), agent_id, paid_at, completed_at
		FROM agent_upgrade_orders WHERE order_no = ? AND user_id = ?
	`, orderNo, userID).Scan(&status, &levelCode, &levelName, &amountText, &openingBonusText, &payChannel, &payMethod,
		&returnURL, &errorMessage, &agentID, &paidAt, &completedAt)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "升级订单不存在"})
		return
	}
	amountCents, _ := parseAmountToCents(amountText)
	openingBonusCents, _ := parseAmountToCents(openingBonusText)
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"orderNo":      orderNo,
			"status":       status,
			"levelCode":    levelCode,
			"levelName":    levelName,
			"amount":       float64(amountCents) / 100,
			"openingBonus": float64(openingBonusCents) / 100,
			"payChannel":   payChannel,
			"payMethod":    payMethod,
			"returnUrl":    returnURL,
			"agentId":      nullableInt64(agentID),
			"errorMessage": errorMessage,
			"paidAt":       nullableTimeText(paidAt),
			"completedAt":  nullableTimeText(completedAt),
		},
	})
}

func loadCompletedAccountConversionByUser(tx *sql.Tx, userID uint64) (accountConversionResult, error) {
	var order accountUpgradeOrder
	err := tx.QueryRow(`
		SELECT o.id, o.order_no, o.user_id, o.level_id, o.level_code_snapshot,
		       o.level_name_snapshot, o.discount_snapshot, o.amount, o.status, o.agent_id
		FROM agent_upgrade_orders o
		JOIN account_conversions c ON c.upgrade_order_id = o.id AND c.status = 'completed'
		WHERE c.user_id = ?
	`, userID).Scan(&order.ID, &order.OrderNo, &order.UserID, &order.LevelID, &order.LevelCode,
		&order.LevelName, &order.Discount, &order.AmountText, &order.Status, &order.AgentID)
	if err != nil {
		return accountConversionResult{}, errAgentUpgradeUserConverted
	}
	return loadCompletedAccountConversion(tx, order)
}

func accountConversionResponse(result accountConversionResult) gin.H {
	return gin.H{
		"orderNo":              result.OrderNo,
		"agentId":              result.AgentID,
		"agentEmail":           result.AgentEmail,
		"levelId":              result.LevelID,
		"levelCode":            result.LevelCode,
		"levelName":            result.LevelName,
		"openingFee":           float64(result.OpeningFeeCents) / 100,
		"transferredBalance":   float64(result.TransferredCents) / 100,
		"openingBonus":         float64(result.OpeningBonusCents) / 100,
		"finalAgentBalance":    float64(result.FinalBalanceCents) / 100,
		"migratedLicenseCount": result.MigratedLicenseCount,
		"loginPath":            "/agent-panel/login?upgraded=1",
	}
}

func writeAgentUpgradeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errAgentUpgradeInsufficientFunds):
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "用户余额不足，无法支付代理开通费"})
	case errors.Is(err, errAgentUpgradePendingOrder):
		c.JSON(http.StatusOK, gin.H{"code": 409, "msg": errAgentUpgradePendingOrder.Error()})
	case errors.Is(err, errAgentUpgradeAccountConflict):
		c.JSON(http.StatusOK, gin.H{"code": 409, "msg": errAgentUpgradeAccountConflict.Error()})
	case errors.Is(err, errAgentUpgradeUserConverted):
		c.JSON(http.StatusOK, gin.H{"code": 409, "msg": "当前账号已升级为代理，请前往代理端登录"})
	case errors.Is(err, errAgentUpgradeUserDisabled):
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": errAgentUpgradeUserDisabled.Error()})
	case errors.Is(err, errAgentUpgradeLevelUnavailable):
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": errAgentUpgradeLevelUnavailable.Error()})
	case errors.Is(err, errAgentUpgradePaymentUnavailable):
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": errAgentUpgradePaymentUnavailable.Error()})
	case errors.Is(err, errAgentUpgradePaymentCreate):
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
	case errors.Is(err, errAgentUpgradeAmountMismatch):
		c.JSON(http.StatusOK, gin.H{"code": 409, "msg": "代理等级价格已变化，请刷新后重试"})
	default:
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "代理账户开通失败，账户资产未发生变化"})
	}
}

func nullableInt64(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func nullableTimeText(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time.Format("2006-01-02 15:04:05")
}
