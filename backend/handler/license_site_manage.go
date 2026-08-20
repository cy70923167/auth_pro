package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
)

type licenseSiteActor struct {
	Type string
	ID   int64
}

func AdminLicenseSiteList(c *gin.Context) {
	licenseSiteList(c, licenseSiteActor{Type: "admin", ID: contextUserID(c)})
}

func UserLicenseSiteList(c *gin.Context) {
	licenseSiteList(c, licenseSiteActor{Type: "user", ID: contextUserID(c)})
}

func AgentLicenseSiteList(c *gin.Context) {
	licenseSiteList(c, licenseSiteActor{Type: "agent", ID: contextUserID(c)})
}

func AdminLicenseSiteUnbind(c *gin.Context) {
	licenseSiteUnbind(c, licenseSiteActor{Type: "admin", ID: contextUserID(c)})
}

func UserLicenseSiteUnbind(c *gin.Context) {
	licenseSiteUnbind(c, licenseSiteActor{Type: "user", ID: contextUserID(c)})
}

func AgentLicenseSiteUnbind(c *gin.Context) {
	licenseSiteUnbind(c, licenseSiteActor{Type: "agent", ID: contextUserID(c)})
}

func contextUserID(c *gin.Context) int64 {
	value, exists := c.Get("user_id")
	if !exists {
		return 0
	}
	switch id := value.(type) {
	case uint:
		return int64(id)
	case uint64:
		return int64(id)
	case int64:
		return id
	case int:
		return int64(id)
	default:
		return 0
	}
}

func openLicenseSiteDB() (*sql.DB, error) {
	cfg, err := config.LoadDBConfig()
	if err != nil {
		return nil, err
	}
	return sql.Open("mysql", config.GetDSN(cfg))
}

func parseLicenseSiteIDs(c *gin.Context, includeSite bool) (int64, int64, bool) {
	licenseID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || licenseID <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "授权ID不正确"})
		return 0, 0, false
	}
	if !includeSite {
		return licenseID, 0, true
	}
	siteID, err := strconv.ParseInt(strings.TrimSpace(c.Param("siteId")), 10, 64)
	if err != nil || siteID <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "站点ID不正确"})
		return 0, 0, false
	}
	return licenseID, siteID, true
}

func licenseSiteOwnerCondition(actor licenseSiteActor) (string, []any) {
	if actor.Type == "admin" {
		return "l.id = ? AND l.type = 'key'", nil
	}
	return "l.id = ? AND l.type = 'key' AND l.owner_type = ? AND l.owner_id = ?", []any{actor.Type, actor.ID}
}

func licenseSiteList(c *gin.Context, actor licenseSiteActor) {
	licenseID, _, ok := parseLicenseSiteIDs(c, false)
	if !ok {
		return
	}
	db, err := openLicenseSiteDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	condition, ownerArgs := licenseSiteOwnerCondition(actor)
	args := append([]any{licenseID}, ownerArgs...)
	var maxSites int
	var licenseNo string
	if err := db.QueryRow(`SELECT l.license_no, COALESCE(l.max_domains, 0) FROM licenses l WHERE `+condition, args...).Scan(&licenseNo, &maxSites); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "密钥授权不存在或无权访问"})
		} else {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询授权失败"})
		}
		return
	}

	rows, err := db.Query(`
		SELECT id, target_type, domain, server_ip, first_seen_at, last_seen_at, created_at
		FROM license_domains
		WHERE license_id = ?
		ORDER BY COALESCE(first_seen_at, created_at) ASC, id ASC
	`, licenseID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询绑定站点失败"})
		return
	}
	defer rows.Close()

	list := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var targetType, target, serverIP string
		var firstSeen, lastSeen sql.NullTime
		var createdAt time.Time
		if err := rows.Scan(&id, &targetType, &target, &serverIP, &firstSeen, &lastSeen, &createdAt); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取绑定站点失败"})
			return
		}
		item := gin.H{"id": id, "targetType": targetType, "target": target, "serverIp": serverIP}
		item["firstSeenAt"] = formatLicenseSiteTime(firstSeen, createdAt)
		item["lastSeenAt"] = formatLicenseSiteTime(lastSeen, createdAt)
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取绑定站点失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{
		"licenseId": licenseID, "licenseNo": licenseNo, "boundSites": len(list), "maxSites": maxSites, "list": list,
	}})
}

func licenseSiteUnbind(c *gin.Context, actor licenseSiteActor) {
	licenseID, siteID, ok := parseLicenseSiteIDs(c, true)
	if !ok {
		return
	}
	db, err := openLicenseSiteDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "解绑失败"})
		return
	}
	defer tx.Rollback()

	condition, ownerArgs := licenseSiteOwnerCondition(actor)
	args := append([]any{licenseID}, ownerArgs...)
	var lockedID int64
	if err := tx.QueryRow(`SELECT l.id FROM licenses l WHERE `+condition+` FOR UPDATE`, args...).Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "密钥授权不存在或无权访问"})
		} else {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询授权失败"})
		}
		return
	}

	var targetType, target, serverIP string
	if err := tx.QueryRow(`
		SELECT target_type, domain, server_ip FROM license_domains
		WHERE id = ? AND license_id = ?
		FOR UPDATE
	`, siteID, licenseID).Scan(&targetType, &target, &serverIP); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "绑定站点不存在"})
		} else {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询绑定站点失败"})
		}
		return
	}
	if _, err := tx.Exec(`DELETE FROM license_domains WHERE id = ? AND license_id = ?`, siteID, licenseID); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "解绑失败"})
		return
	}
	if _, err := tx.Exec(`
		INSERT INTO operation_logs
		(operator_type, operator_id, action, target_type, target_id, detail, ip)
		VALUES (?, ?, 'license_site_unbind', 'license', ?,
		        JSON_OBJECT('siteId', ?, 'targetType', ?, 'target', ?, 'serverIp', ?), ?)
	`, actor.Type, actor.ID, licenseID, siteID, targetType, target, serverIP, c.ClientIP()); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "记录解绑审计失败"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "解绑失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "站点已解绑，名额已立即释放"})
}

func formatLicenseSiteTime(value sql.NullTime, fallback time.Time) string {
	if value.Valid {
		return value.Time.Format("2006-01-02 15:04:05")
	}
	return fallback.Format("2006-01-02 15:04:05")
}
