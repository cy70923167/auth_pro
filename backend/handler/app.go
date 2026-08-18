package handler

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

// AppList 应用列表（下拉选择用）
func AppList(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, app_name FROM apps WHERE enabled = 1 ORDER BY id ASC")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	defer rows.Close()

	type appItem struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}

	var list []appItem
	for rows.Next() {
		var item appItem
		if err := rows.Scan(&item.ID, &item.Name); err == nil {
			list = append(list, item)
		}
	}
	if list == nil {
		list = []appItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": list,
	})
}

// AppManageList 应用管理列表（含详细信息）
func AppManageList(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := EnsureAppPurchaseLicenseTypesColumn(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化应用授权方式失败"})
		return
	}
	if err := ensureAppLicenseRequiredColumn(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化应用授权开关失败"})
		return
	}
	if err := EnsureAppVersionsTable(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化版本数据失败"})
		return
	}

	rows, err := db.Query(`
		SELECT a.id, a.app_name, a.app_key, a.app_secret, a.description, a.enabled,
		       a.license_required, a.purchase_license_type_mask, a.created_at,
		       (SELECT COUNT(*) FROM licenses l WHERE l.app_id = a.id) AS license_count,
		       (SELECT COUNT(*) FROM app_versions v WHERE v.app_id = a.id) AS version_count,
		       COALESCE((
		           SELECT v.version FROM app_versions v WHERE v.app_id = a.id
		           ORDER BY v.published_at DESC, v.id DESC LIMIT 1
		       ), '') AS recent_version
		FROM apps a ORDER BY a.id ASC
	`)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	defer rows.Close()

	type appManageItem struct {
		ID                   int64    `json:"id"`
		Name                 string   `json:"name"`
		AppKey               string   `json:"appKey"`
		AppSecret            string   `json:"appSecret"`
		Remark               string   `json:"remark"`
		Enabled              bool     `json:"enabled"`
		LicenseRequired      bool     `json:"licenseRequired"`
		PurchaseLicenseTypes []string `json:"purchaseLicenseTypes"`
		CreatedAt            string   `json:"createdAt"`
		LicenseCount         int64    `json:"licenseCount"`
		VersionCount         int64    `json:"versionCount"`
		RecentVersion        string   `json:"recentVersion"`
	}

	var list []appManageItem
	for rows.Next() {
		var item appManageItem
		var createdAt time.Time
		var desc string
		var purchaseLicenseTypeMask uint8
		if err := rows.Scan(&item.ID, &item.Name, &item.AppKey, &item.AppSecret,
			&desc, &item.Enabled, &item.LicenseRequired, &purchaseLicenseTypeMask, &createdAt, &item.LicenseCount, &item.VersionCount,
			&item.RecentVersion); err == nil {
			item.Remark = desc
			item.PurchaseLicenseTypes = purchaseLicenseTypesFromMask(purchaseLicenseTypeMask)
			item.CreatedAt = createdAt.Format("2006-01-02 15:04")
			list = append(list, item)
		}
	}
	if list == nil {
		list = []appManageItem{}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": list})
}

// AppCreate 新增应用
func AppCreate(c *gin.Context) {
	var req struct {
		Name                 string   `json:"name" binding:"required"`
		Enabled              bool     `json:"enabled"`
		Remark               string   `json:"remark"`
		PurchaseLicenseTypes []string `json:"purchaseLicenseTypes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "应用名称不能为空"})
		return
	}
	purchaseLicenseTypeMask, err := purchaseLicenseTypeMaskForCreate(req.PurchaseLicenseTypes)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := EnsureAppPurchaseLicenseTypesColumn(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化应用授权方式失败"})
		return
	}

	appKey := fmt.Sprintf("app_%s_%d", randomHex(6), time.Now().Unix()%10000)
	appSecret := "sk_live_" + randomHex(16)

	result, err := db.Exec(`
		INSERT INTO apps (app_name, app_key, app_secret, description, enabled, purchase_license_type_mask)
		VALUES (?, ?, ?, ?, ?, ?)
	`, req.Name, appKey, appSecret, req.Remark, req.Enabled, purchaseLicenseTypeMask)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建失败: " + err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "创建成功", "data": gin.H{"id": id}})
}

// AppUpdate 编辑应用
func AppUpdate(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name                 string   `json:"name" binding:"required"`
		Enabled              bool     `json:"enabled"`
		Remark               string   `json:"remark"`
		PurchaseLicenseTypes []string `json:"purchaseLicenseTypes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	var purchaseLicenseTypeMask uint8
	if req.PurchaseLicenseTypes != nil {
		var err error
		purchaseLicenseTypeMask, err = parsePurchaseLicenseTypes(req.PurchaseLicenseTypes)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
			return
		}
	}

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := EnsureAppPurchaseLicenseTypesColumn(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化应用授权方式失败"})
		return
	}

	var result sql.Result
	if req.PurchaseLicenseTypes == nil {
		result, err = db.Exec("UPDATE apps SET app_name = ?, description = ?, enabled = ? WHERE id = ?",
			req.Name, req.Remark, req.Enabled, id)
	} else {
		result, err = db.Exec(`
			UPDATE apps
			SET app_name = ?, description = ?, enabled = ?, purchase_license_type_mask = ?
			WHERE id = ?
		`, req.Name, req.Remark, req.Enabled, purchaseLicenseTypeMask, id)
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "更新失败"})
		return
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected != 1 {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "应用不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "更新成功"})
}

// AppLicenseRequiredUpdate 更新应用是否要求许可证校验。
func AppLicenseRequiredUpdate(c *gin.Context) {
	id, err := positiveInt64(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "应用ID不正确"})
		return
	}

	var req struct {
		LicenseRequired *bool `json:"licenseRequired"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.LicenseRequired == nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "授权校验状态不正确"})
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
	if err := ensureAppLicenseRequiredColumn(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化应用授权开关失败"})
		return
	}

	result, err := db.Exec("UPDATE apps SET license_required = ? WHERE id = ?", *req.LicenseRequired, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "更新授权校验状态失败"})
		return
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		var exists int
		if err := db.QueryRow("SELECT COUNT(*) FROM apps WHERE id = ?", id).Scan(&exists); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "检查应用状态失败"})
			return
		}
		if exists == 0 {
			c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "应用不存在"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "授权校验状态已更新",
		"data": gin.H{"licenseRequired": *req.LicenseRequired},
	})
}

// AppResetSecret 重置应用密钥
func AppResetSecret(c *gin.Context) {
	id := c.Param("id")

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	newSecret := "sk_live_" + randomHex(16)
	_, err = db.Exec("UPDATE apps SET app_secret = ? WHERE id = ?", newSecret, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "重置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "密钥已重置", "data": gin.H{"appSecret": newSecret}})
}

// AppDelete 删除应用
func AppDelete(c *gin.Context) {
	id := c.Param("id")

	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := EnsureAppVersionsTable(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化版本数据失败"})
		return
	}

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "删除应用失败"})
		return
	}
	defer tx.Rollback()
	var lockedAppID int64
	if err := tx.QueryRow("SELECT id FROM apps WHERE id = ? FOR UPDATE", id).Scan(&lockedAppID); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "应用不存在"})
		} else {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "锁定应用失败"})
		}
		return
	}

	rows, err := tx.Query("SELECT package_path FROM app_versions WHERE app_id = ? AND package_path <> '' FOR UPDATE", id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询应用版本失败"})
		return
	}
	var packagePaths []string
	for rows.Next() {
		var packagePath string
		if scanErr := rows.Scan(&packagePath); scanErr != nil {
			rows.Close()
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取应用版本失败"})
			return
		}
		packagePaths = append(packagePaths, packagePath)
	}
	if err := rows.Close(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取应用版本失败"})
		return
	}

	if _, err = tx.Exec("DELETE FROM app_versions WHERE app_id = ?", id); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "删除应用版本失败"})
		return
	}
	result, err := tx.Exec("DELETE FROM apps WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "删除失败"})
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "应用不存在"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "删除应用失败"})
		return
	}
	for _, packagePath := range packagePaths {
		_ = removeReleasePackage(packagePath)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "删除成功"})
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
