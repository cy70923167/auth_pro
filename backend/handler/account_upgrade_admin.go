package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type adminAgentUpgradeOrderItem struct {
	ID             uint64   `json:"id"`
	OrderNo        string   `json:"orderNo"`
	UserID         uint64   `json:"userId"`
	UserEmail      string   `json:"userEmail"`
	UserName       string   `json:"userName"`
	LevelID        uint64   `json:"levelId"`
	LevelCode      string   `json:"levelCode"`
	LevelName      string   `json:"levelName"`
	Discount       float64  `json:"discount"`
	Amount         float64  `json:"amount"`
	OpeningBonus   float64  `json:"openingBonus"`
	PaidAmount     *float64 `json:"paidAmount"`
	PayChannel     string   `json:"payChannel"`
	PayMethod      string   `json:"payMethod"`
	Status         string   `json:"status"`
	AgentID        *int64   `json:"agentId"`
	AgentName      string   `json:"agentName"`
	GatewayTradeNo string   `json:"gatewayTradeNo"`
	ErrorMessage   string   `json:"errorMessage"`
	CreatedAt      string   `json:"createdAt"`
	PaidAt         string   `json:"paidAt"`
	CompletedAt    string   `json:"completedAt"`
	UpdatedAt      string   `json:"updatedAt"`
}

type adminAccountConversionItem struct {
	ID                   uint64  `json:"id"`
	ConversionNo         string  `json:"conversionNo"`
	OrderNo              string  `json:"orderNo"`
	UserID               uint64  `json:"userId"`
	UserEmail            string  `json:"userEmail"`
	UserName             string  `json:"userName"`
	AgentID              *int64  `json:"agentId"`
	AgentEmail           string  `json:"agentEmail"`
	AgentName            string  `json:"agentName"`
	LevelID              uint64  `json:"levelId"`
	LevelName            string  `json:"levelName"`
	Status               string  `json:"status"`
	OpeningFee           float64 `json:"openingFee"`
	TransferredBalance   float64 `json:"transferredBalance"`
	OpeningBonus         float64 `json:"openingBonus"`
	FinalBalance         float64 `json:"finalBalance"`
	MigratedLicenseCount uint64  `json:"migratedLicenseCount"`
	ErrorMessage         string  `json:"errorMessage"`
	StartedAt            string  `json:"startedAt"`
	CompletedAt          string  `json:"completedAt"`
	CreatedAt            string  `json:"createdAt"`
	UpdatedAt            string  `json:"updatedAt"`
}

func adminUpgradePagination(c *gin.Context) (int, int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize, (page - 1) * pageSize
}

func openAdminAccountUpgradeDB(c *gin.Context) (*sql.DB, bool) {
	db, err := openAccountUpgradeDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "代理升级审计服务初始化失败"})
		return nil, false
	}
	return db, true
}

func adminAmountValue(text string) float64 {
	cents, err := parseAmountToCents(text)
	if err != nil {
		return 0
	}
	return float64(cents) / 100
}

func adminNullableAmount(value sql.NullString) *float64 {
	if !value.Valid {
		return nil
	}
	amount := adminAmountValue(value.String)
	return &amount
}

func adminNullableInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func adminNullableTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02 15:04:05")
}

// AdminAgentUpgradeStats returns aggregate values for balance upgrade auditing.
func AdminAgentUpgradeStats(c *gin.Context) {
	db, ok := openAdminAccountUpgradeDB(c)
	if !ok {
		return
	}
	defer db.Close()

	var totalOrders, pendingOrders, completedOrders, failedOrders int64
	var completedAmount, transferredBalance, openingBonus sql.NullString
	var completedConversions, migratedLicenses int64
	if err := db.QueryRow(`
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN status IN ('pending','paid','processing') THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0),
		       CAST(COALESCE(SUM(CASE WHEN status = 'completed' THEN COALESCE(paid_amount, amount) ELSE 0 END), 0) AS CHAR)
		FROM agent_upgrade_orders
	`).Scan(&totalOrders, &pendingOrders, &completedOrders, &failedOrders, &completedAmount); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询升级订单统计失败"})
		return
	}
	if err := db.QueryRow(`
		SELECT COUNT(*), CAST(COALESCE(SUM(transferred_balance), 0) AS CHAR),
		       CAST(COALESCE(SUM(opening_bonus), 0) AS CHAR),
		       COALESCE(SUM(migrated_license_count), 0)
		FROM account_conversions WHERE status = 'completed'
	`).Scan(&completedConversions, &transferredBalance, &openingBonus, &migratedLicenses); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询转换统计失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{
		"totalOrders":          totalOrders,
		"pendingOrders":        pendingOrders,
		"completedOrders":      completedOrders,
		"failedOrders":         failedOrders,
		"completedAmount":      adminAmountValue(completedAmount.String),
		"completedConversions": completedConversions,
		"transferredBalance":   adminAmountValue(transferredBalance.String),
		"openingBonus":         adminAmountValue(openingBonus.String),
		"migratedLicenses":     migratedLicenses,
	}})
}

// AdminAgentUpgradeOrderList lists balance upgrade orders without changing their state.
func AdminAgentUpgradeOrderList(c *gin.Context) {
	db, ok := openAdminAccountUpgradeDB(c)
	if !ok {
		return
	}
	defer db.Close()

	page, pageSize, offset := adminUpgradePagination(c)
	where := []string{"1=1"}
	args := make([]any, 0, 8)
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		where = append(where, `(o.order_no LIKE ? OR u.email LIKE ? OR COALESCE(u.nickname, '') LIKE ? OR COALESCE(a.email, '') LIKE ? OR COALESCE(a.name, '') LIKE ?)`)
		like := "%" + keyword + "%"
		args = append(args, like, like, like, like, like)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		where = append(where, "o.status = ?")
		args = append(args, status)
	}
	if channel := strings.TrimSpace(c.Query("payChannel")); channel != "" {
		where = append(where, "o.pay_channel = ?")
		args = append(args, channel)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	countSQL := fmt.Sprintf(`
		SELECT COUNT(*) FROM agent_upgrade_orders o
		JOIN users u ON u.id = o.user_id
		LEFT JOIN agents a ON a.id = o.agent_id
		WHERE %s
	`, whereSQL)
	if err := db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询升级订单总数失败"})
		return
	}

	listSQL := fmt.Sprintf(`
		SELECT o.id, o.order_no, o.user_id, u.email, COALESCE(u.nickname, ''),
		       o.level_id, o.level_code_snapshot, o.level_name_snapshot, o.discount_snapshot,
		       CAST(o.amount AS CHAR), CAST(o.opening_bonus_snapshot AS CHAR), CAST(o.paid_amount AS CHAR),
		       o.pay_channel, o.pay_method,
		       o.status, o.agent_id, COALESCE(a.name, ''), o.gateway_trade_no,
		       COALESCE(o.error_message, ''), o.created_at, o.paid_at, o.completed_at, o.updated_at
		FROM agent_upgrade_orders o
		JOIN users u ON u.id = o.user_id
		LEFT JOIN agents a ON a.id = o.agent_id
		WHERE %s
		ORDER BY o.created_at DESC, o.id DESC
		LIMIT ? OFFSET ?
	`, whereSQL)
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := db.Query(listSQL, queryArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询升级订单失败: " + err.Error()})
		return
	}
	defer rows.Close()

	list := make([]adminAgentUpgradeOrderItem, 0)
	for rows.Next() {
		var item adminAgentUpgradeOrderItem
		var amount, openingBonus string
		var paidAmount sql.NullString
		var agentID sql.NullInt64
		var createdAt, updatedAt time.Time
		var paidAt, completedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrderNo, &item.UserID, &item.UserEmail, &item.UserName,
			&item.LevelID, &item.LevelCode, &item.LevelName, &item.Discount, &amount, &openingBonus, &paidAmount,
			&item.PayChannel, &item.PayMethod, &item.Status, &agentID, &item.AgentName,
			&item.GatewayTradeNo, &item.ErrorMessage, &createdAt, &paidAt, &completedAt, &updatedAt); err != nil {
			continue
		}
		item.Amount = adminAmountValue(amount)
		item.OpeningBonus = adminAmountValue(openingBonus)
		item.PaidAmount = adminNullableAmount(paidAmount)
		item.AgentID = adminNullableInt64(agentID)
		item.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		item.PaidAt = adminNullableTime(paidAt)
		item.CompletedAt = adminNullableTime(completedAt)
		item.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取升级订单失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{
		"list": list, "total": total, "page": page, "pageSize": pageSize,
	}})
}

// AdminAccountConversionList lists immutable account conversion audit records.
func AdminAccountConversionList(c *gin.Context) {
	db, ok := openAdminAccountUpgradeDB(c)
	if !ok {
		return
	}
	defer db.Close()

	page, pageSize, offset := adminUpgradePagination(c)
	where := []string{"1=1"}
	args := make([]any, 0, 6)
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		where = append(where, `(c.conversion_no LIKE ? OR o.order_no LIKE ? OR u.email LIKE ? OR COALESCE(a.email, '') LIKE ? OR COALESCE(a.name, '') LIKE ?)`)
		like := "%" + keyword + "%"
		args = append(args, like, like, like, like, like)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		where = append(where, "c.status = ?")
		args = append(args, status)
	}
	whereSQL := strings.Join(where, " AND ")

	joinSQL := `
		FROM account_conversions c
		JOIN agent_upgrade_orders o ON o.id = c.upgrade_order_id
		JOIN users u ON u.id = c.user_id
		LEFT JOIN agents a ON a.id = c.agent_id
	`
	var total int64
	if err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) %s WHERE %s", joinSQL, whereSQL), args...).Scan(&total); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询转换记录总数失败"})
		return
	}

	listSQL := fmt.Sprintf(`
		SELECT c.id, c.conversion_no, o.order_no, c.user_id, u.email, COALESCE(u.nickname, ''),
		       c.agent_id, COALESCE(a.email, ''), COALESCE(a.name, ''), c.level_id,
		       o.level_name_snapshot, c.status, CAST(c.opening_fee AS CHAR),
		       CAST(c.transferred_balance AS CHAR), CAST(c.opening_bonus AS CHAR), c.migrated_license_count,
		       COALESCE(c.error_message, ''), c.started_at, c.completed_at, c.created_at, c.updated_at
		%s WHERE %s
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT ? OFFSET ?
	`, joinSQL, whereSQL)
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := db.Query(listSQL, queryArgs...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询转换记录失败: " + err.Error()})
		return
	}
	defer rows.Close()

	list := make([]adminAccountConversionItem, 0)
	for rows.Next() {
		var item adminAccountConversionItem
		var agentID sql.NullInt64
		var openingFee, transferredBalance, openingBonus string
		var startedAt, createdAt, updatedAt time.Time
		var completedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.ConversionNo, &item.OrderNo, &item.UserID, &item.UserEmail,
			&item.UserName, &agentID, &item.AgentEmail, &item.AgentName, &item.LevelID, &item.LevelName,
			&item.Status, &openingFee, &transferredBalance, &openingBonus, &item.MigratedLicenseCount,
			&item.ErrorMessage, &startedAt, &completedAt, &createdAt, &updatedAt); err != nil {
			continue
		}
		item.AgentID = adminNullableInt64(agentID)
		item.OpeningFee = adminAmountValue(openingFee)
		item.TransferredBalance = adminAmountValue(transferredBalance)
		item.OpeningBonus = adminAmountValue(openingBonus)
		item.FinalBalance = item.TransferredBalance + item.OpeningBonus
		item.StartedAt = startedAt.Format("2006-01-02 15:04:05")
		item.CompletedAt = adminNullableTime(completedAt)
		item.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		item.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取转换记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{
		"list": list, "total": total, "page": page, "pageSize": pageSize,
	}})
}

// AdminAccountConversionDetail returns stored snapshots for audit inspection.
func AdminAccountConversionDetail(c *gin.Context) {
	conversionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || conversionID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "转换记录ID无效"})
		return
	}
	db, ok := openAdminAccountUpgradeDB(c)
	if !ok {
		return
	}
	defer db.Close()

	var conversionNo, orderNo, status, errorMessage string
	var sourceSnapshot, resultSnapshot sql.NullString
	err = db.QueryRow(`
		SELECT c.conversion_no, o.order_no, c.status, COALESCE(c.error_message, ''),
		       CAST(c.source_snapshot AS CHAR), CAST(c.result_snapshot AS CHAR)
		FROM account_conversions c
		JOIN agent_upgrade_orders o ON o.id = c.upgrade_order_id
		WHERE c.id = ?
	`, conversionID).Scan(&conversionNo, &orderNo, &status, &errorMessage, &sourceSnapshot, &resultSnapshot)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "转换记录不存在"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询转换详情失败"})
		return
	}

	decodeSnapshot := func(value sql.NullString) any {
		if !value.Valid || strings.TrimSpace(value.String) == "" {
			return nil
		}
		var decoded any
		if json.Unmarshal([]byte(value.String), &decoded) != nil {
			return value.String
		}
		return decoded
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{
		"id": conversionID, "conversionNo": conversionNo, "orderNo": orderNo,
		"status": status, "errorMessage": errorMessage,
		"sourceSnapshot": decodeSnapshot(sourceSnapshot), "resultSnapshot": decodeSnapshot(resultSnapshot),
	}})
}
