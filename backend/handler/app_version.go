package handler

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
)

const (
	appReleaseMaxBytes        int64 = 512 * 1024 * 1024
	appReleaseRequestMaxBytes       = appReleaseMaxBytes + 2*1024*1024
	appVersionCheckMaxSkew          = 10 * time.Minute
	appVersionDownloadTTL           = 10 * time.Minute
)

var (
	appVersionPattern = regexp.MustCompile(`^[vV]?[0-9]+(?:[._][0-9]+)*(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?(?:\+[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$`)
	md5Pattern        = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)
	versionTokenSplit = regexp.MustCompile(`[._-]`)
	appVersionTableMu sync.Mutex
	appVersionTableOK bool
)

type appVersionRecord struct {
	ID            int64
	AppID         int64
	Version       string
	Title         string
	Changelog     string
	UpdateSQL     string
	PackageName   string
	PackagePath   string
	DownloadURL   string
	FileSizeBytes int64
	FileMD5       string
	ForceUpdate   bool
	MinVersion    string
	Revision      int64
	PublishedAt   time.Time
	UpdatedAt     time.Time
}

type appVersionCheckRequest struct {
	AppKey         string `json:"appKey" binding:"required"`
	CurrentVersion string `json:"currentVersion" binding:"required"`
	Domain         string `json:"domain"`
	ServerIP       string `json:"serverIp"`
	LicenseKey     string `json:"licenseKey"`
	Timestamp      int64  `json:"timestamp" binding:"required"`
	SignVersion    string `json:"signVersion"`
	Sign           string `json:"sign" binding:"required"`
}

// EnsureAppVersionsTable 为已安装环境幂等补齐应用版本表。
func EnsureAppVersionsTable(db *sql.DB) error {
	appVersionTableMu.Lock()
	defer appVersionTableMu.Unlock()
	if appVersionTableOK {
		return nil
	}

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS app_versions (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
			app_id BIGINT UNSIGNED NOT NULL COMMENT '所属应用ID',
			from_version VARCHAR(50) NOT NULL DEFAULT '' COMMENT '兼容旧数据，已停用',
			from_version_norm VARCHAR(100) DEFAULT NULL COMMENT '兼容旧数据，已停用',
			version VARCHAR(50) NOT NULL COMMENT '版本号',
			version_norm VARCHAR(100) NOT NULL COMMENT '规范化版本号',
			title VARCHAR(200) NOT NULL COMMENT '更新标题',
			changelog MEDIUMTEXT NOT NULL COMMENT '更新日志',
			update_sql LONGTEXT NOT NULL COMMENT '客户端更新SQL',
			package_name VARCHAR(255) NOT NULL DEFAULT '' COMMENT '更新包原始文件名',
			package_path VARCHAR(512) NOT NULL DEFAULT '' COMMENT '本地更新包相对路径',
			download_url VARCHAR(2048) NOT NULL DEFAULT '' COMMENT '外部更新包下载URL',
			file_size_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '文件大小(字节)',
			file_md5 CHAR(32) NOT NULL DEFAULT '' COMMENT '文件MD5',
			force_update TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否强制更新',
			min_version VARCHAR(50) NOT NULL DEFAULT '' COMMENT '低于该版本时强制更新',
			revision BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '并发编辑修订号',
			published_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '发布时间',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
			PRIMARY KEY (id),
			UNIQUE KEY uk_app_version (app_id, version),
			UNIQUE KEY uk_app_version_norm (app_id, version_norm),
			KEY idx_app_published (app_id, published_at, id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='应用版本发布记录'
	`)
	if err != nil {
		return err
	}

	columns := []struct {
		name string
		ddl  string
	}{
		{name: "from_version", ddl: "ALTER TABLE app_versions ADD COLUMN from_version VARCHAR(50) NOT NULL DEFAULT '' COMMENT '兼容旧数据，已停用' AFTER app_id"},
		{name: "from_version_norm", ddl: "ALTER TABLE app_versions ADD COLUMN from_version_norm VARCHAR(100) DEFAULT NULL COMMENT '兼容旧数据，已停用' AFTER from_version"},
		{name: "version_norm", ddl: "ALTER TABLE app_versions ADD COLUMN version_norm VARCHAR(100) NOT NULL DEFAULT '' COMMENT '规范化版本号' AFTER version"},
		{name: "revision", ddl: "ALTER TABLE app_versions ADD COLUMN revision BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '并发编辑修订号' AFTER min_version"},
	}
	for _, column := range columns {
		exists, checkErr := appVersionColumnExists(db, column.name)
		if checkErr != nil {
			return checkErr
		}
		if !exists {
			if _, alterErr := db.Exec(column.ddl); alterErr != nil {
				return alterErr
			}
		}
	}

	if err := backfillAppVersionNorms(db); err != nil {
		return err
	}
	if err := ensureAppVersionUniqueIndex(db, "uk_app_version_norm", "version_norm"); err != nil {
		return err
	}
	if nullable, err := appVersionColumnNullable(db, "from_version_norm"); err != nil {
		return err
	} else if !nullable {
		if _, err := db.Exec(`
			ALTER TABLE app_versions
			MODIFY COLUMN from_version_norm VARCHAR(100) DEFAULT NULL COMMENT '兼容旧数据，已停用'
		`); err != nil {
			return err
		}
	}
	if exists, err := appVersionIndexExists(db, "uk_app_from_version_norm"); err != nil {
		return err
	} else if exists {
		if _, err := db.Exec(`ALTER TABLE app_versions DROP INDEX uk_app_from_version_norm`); err != nil {
			return err
		}
	}

	appVersionTableOK = true
	return nil
}

func appVersionColumnExists(db *sql.DB, column string) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'app_versions' AND COLUMN_NAME = ?
	`, column).Scan(&count)
	return count > 0, err
}

func appVersionColumnNullable(db *sql.DB, column string) (bool, error) {
	var isNullable string
	err := db.QueryRow(`
		SELECT IS_NULLABLE FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'app_versions' AND COLUMN_NAME = ?
	`, column).Scan(&isNullable)
	return strings.EqualFold(isNullable, "YES"), err
}

func appVersionIndexExists(db *sql.DB, index string) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'app_versions' AND INDEX_NAME = ?
	`, index).Scan(&count)
	return count > 0, err
}

func backfillAppVersionNorms(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, app_id, version FROM app_versions`)
	if err != nil {
		return err
	}
	items := make([]appVersionNormItem, 0)
	for rows.Next() {
		var id, appID int64
		var version string
		if err := rows.Scan(&id, &appID, &version); err != nil {
			rows.Close()
			return err
		}
		if !validVersion(version) {
			rows.Close()
			return fmt.Errorf("app_versions id=%d 的版本号格式不正确", id)
		}
		toNorm := normalizeVersion(version)
		items = append(items, appVersionNormItem{ID: id, AppID: appID, VersionNorm: toNorm})
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := normalizedVersionConflict(items); err != nil {
		return err
	}

	for _, item := range items {
		if _, err := db.Exec(`
			UPDATE app_versions SET from_version = '', from_version_norm = NULL, version_norm = ? WHERE id = ?
		`, item.VersionNorm, item.ID); err != nil {
			if isDuplicateEntry(err) {
				return errors.New("应用版本存在语义重复，请先修正历史版本")
			}
			return err
		}
	}
	return nil
}

func ensureAppVersionUniqueIndex(db *sql.DB, index, column string) error {
	exists, err := appVersionIndexExists(db, index)
	if err != nil || exists {
		return err
	}
	if _, err := db.Exec(fmt.Sprintf(
		"ALTER TABLE app_versions ADD UNIQUE KEY %s (app_id, %s)", index, column,
	)); err != nil {
		if isDuplicateEntry(err) {
			return errors.New("应用版本存在语义重复，请先修正历史版本")
		}
		return err
	}
	return nil
}

// AppVersionList 返回指定应用的版本发布记录。
func AppVersionList(c *gin.Context) {
	appID, err := positiveInt64(c.Param("id"))
	if err != nil {
		apiError(c, 400, "应用ID不正确")
		return
	}

	page := positiveIntDefault(c.Query("page"), 1)
	pageSize := positiveIntDefault(c.Query("pageSize"), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	db, err := openAppVersionDB()
	if err != nil {
		apiError(c, 500, "数据库连接失败")
		return
	}
	defer db.Close()
	if err := EnsureAppVersionsTable(db); err != nil {
		apiError(c, 500, "初始化版本数据失败")
		return
	}

	var appName, appKey string
	if err := db.QueryRow(`SELECT app_name, app_key FROM apps WHERE id = ?`, appID).Scan(&appName, &appKey); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apiError(c, 404, "应用不存在")
		} else {
			apiError(c, 500, "查询应用失败")
		}
		return
	}

	var total int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_versions WHERE app_id = ?`, appID).Scan(&total); err != nil {
		apiError(c, 500, "查询版本数量失败")
		return
	}

	rows, err := db.Query(`
		SELECT id, app_id, version, title, changelog, update_sql, package_name, package_path,
		       download_url, file_size_bytes, file_md5, force_update, min_version, revision, published_at, updated_at
		FROM app_versions
		WHERE app_id = ?
		ORDER BY published_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, appID, pageSize, (page-1)*pageSize)
	if err != nil {
		apiError(c, 500, "查询版本列表失败")
		return
	}
	defer rows.Close()

	list := make([]gin.H, 0)
	for rows.Next() {
		record, scanErr := scanAppVersion(rows)
		if scanErr != nil {
			apiError(c, 500, "读取版本数据失败")
			return
		}
		list = append(list, appVersionAdminJSON(record))
	}
	if err := rows.Err(); err != nil {
		apiError(c, 500, "读取版本数据失败")
		return
	}

	latestVersion, _ := findHighestAppVersion(db, appID)
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"app":           gin.H{"id": appID, "name": appName, "appKey": appKey},
			"list":          list,
			"total":         total,
			"latestVersion": latestVersion,
		},
	})
}

// AppVersionCreate 发布应用版本。
func AppVersionCreate(c *gin.Context) {
	saveAppVersion(c, false)
}

// AppVersionUpdate 编辑应用版本。
func AppVersionUpdate(c *gin.Context) {
	saveAppVersion(c, true)
}

func saveAppVersion(c *gin.Context, editing bool) {
	appID, err := positiveInt64(c.Param("id"))
	if err != nil {
		apiError(c, 400, "应用ID不正确")
		return
	}

	var versionID int64
	if editing {
		versionID, err = positiveInt64(c.Param("versionId"))
		if err != nil {
			apiError(c, 400, "版本ID不正确")
			return
		}
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, appReleaseRequestMaxBytes)
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		apiError(c, 400, "请求数据无效或更新包超过512MB")
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}

	version := strings.TrimSpace(c.PostForm("version"))
	title := strings.TrimSpace(c.PostForm("title"))
	changelog := strings.TrimSpace(c.PostForm("changelog"))
	updateSQL := strings.TrimSpace(c.PostForm("updateSql"))
	sourceType := strings.TrimSpace(c.PostForm("sourceType"))
	minVersion := strings.TrimSpace(c.PostForm("minVersion"))
	forceUpdate := formBool(c.PostForm("forceUpdate"))
	expectedRevision := int64(0)
	if editing {
		expectedRevision, err = positiveInt64(c.PostForm("revision"))
		if err != nil {
			apiError(c, 400, "版本修订号不正确")
			return
		}
	}

	if err := validateVersionFields(version, title, changelog, minVersion); err != nil {
		apiError(c, 400, err.Error())
		return
	}
	if sourceType != "upload" && sourceType != "url" {
		apiError(c, 400, "请选择更新包来源")
		return
	}

	var packageHeader *multipart.FileHeader
	if file, fileErr := c.FormFile("package"); fileErr == nil {
		packageHeader = file
	} else if !errors.Is(fileErr, http.ErrMissingFile) {
		apiError(c, 400, "读取更新包失败")
		return
	}
	if packageHeader != nil && packageHeader.Size > appReleaseMaxBytes {
		apiError(c, 400, "更新包超过512MB")
		return
	}

	db, err := openAppVersionDB()
	if err != nil {
		apiError(c, 500, "数据库连接失败")
		return
	}
	defer db.Close()
	if err := EnsureAppVersionsTable(db); err != nil {
		apiError(c, 500, "初始化版本数据失败")
		return
	}

	var appExists bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM apps WHERE id = ?)`, appID).Scan(&appExists); err != nil || !appExists {
		apiError(c, 404, "应用不存在")
		return
	}

	var old appVersionRecord
	if editing {
		row := db.QueryRow(`
			SELECT id, app_id, version, title, changelog, update_sql, package_name, package_path,
			       download_url, file_size_bytes, file_md5, force_update, min_version, revision, published_at, updated_at
			FROM app_versions WHERE id = ? AND app_id = ?
		`, versionID, appID)
		old, err = scanAppVersion(row)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				apiError(c, 404, "版本记录不存在")
			} else {
				apiError(c, 500, "查询版本记录失败")
			}
			return
		}
		if old.Revision != expectedRevision {
			apiError(c, 409, "版本记录已发生变化，请刷新后重试")
			return
		}
	}

	versionNorm := normalizeVersion(version)
	if err := validateVersionUniqueness(db, appID, versionID, versionNorm); err != nil {
		apiError(c, 400, err.Error())
		return
	}

	packageName := old.PackageName
	packagePath := old.PackagePath
	downloadURL := old.DownloadURL
	fileSizeBytes := old.FileSizeBytes
	fileMD5 := old.FileMD5
	newPackagePath := ""

	if sourceType == "upload" {
		downloadURL = ""
		if packageHeader != nil {
			packageName, newPackagePath, fileSizeBytes, fileMD5, err = saveReleasePackage(appID, packageHeader)
			if err != nil {
				apiError(c, 400, err.Error())
				return
			}
			packagePath = newPackagePath
		} else if packagePath == "" {
			apiError(c, 400, "请上传更新包")
			return
		}
	} else {
		downloadURL = strings.TrimSpace(c.PostForm("downloadUrl"))
		if err := validateDownloadURL(downloadURL); err != nil {
			apiError(c, 400, err.Error())
			return
		}
		fileSizeBytes, err = parseFileSizeMB(c.PostForm("fileSizeMb"))
		if err != nil {
			apiError(c, 400, err.Error())
			return
		}
		fileMD5 = strings.ToLower(strings.TrimSpace(c.PostForm("fileMd5")))
		if !md5Pattern.MatchString(fileMD5) {
			apiError(c, 400, "文件MD5必须是32位十六进制字符串")
			return
		}
		packageName = packageNameFromURL(downloadURL)
		packagePath = ""
	}

	if newPackagePath != "" {
		defer func() {
			if newPackagePath != "" {
				_ = removeReleasePackage(newPackagePath)
			}
		}()
	}

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		apiError(c, 500, "保存版本失败")
		return
	}
	defer tx.Rollback()
	var lockedAppID int64
	if err := tx.QueryRow(`SELECT id FROM apps WHERE id = ? FOR UPDATE`, appID).Scan(&lockedAppID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apiError(c, 404, "应用不存在")
		} else {
			apiError(c, 500, "锁定应用失败")
		}
		return
	}
	if editing {
		row := tx.QueryRow(`
				SELECT id, app_id, version, title, changelog, update_sql, package_name, package_path,
			       download_url, file_size_bytes, file_md5, force_update, min_version, revision, published_at, updated_at
			FROM app_versions WHERE id = ? AND app_id = ? FOR UPDATE
		`, versionID, appID)
		lockedOld, scanErr := scanAppVersion(row)
		if scanErr != nil {
			apiError(c, 409, "版本记录已发生变化，请刷新后重试")
			return
		}
		if lockedOld.Revision != expectedRevision || lockedOld != old {
			apiError(c, 409, "版本记录已发生变化，请刷新后重试")
			return
		}
	}
	if err := validateVersionUniqueness(tx, appID, versionID, versionNorm); err != nil {
		apiError(c, 400, err.Error())
		return
	}

	if editing {
		result, execErr := tx.Exec(`
			UPDATE app_versions
			SET from_version = '', from_version_norm = NULL, version = ?, version_norm = ?,
			    title = ?, changelog = ?, update_sql = ?, package_name = ?, package_path = ?,
			    download_url = ?, file_size_bytes = ?, file_md5 = ?, force_update = ?, min_version = ?,
			    revision = revision + 1
			WHERE id = ? AND app_id = ? AND revision = ?
		`, version, versionNorm, title, changelog, updateSQL, packageName, packagePath, downloadURL,
			fileSizeBytes, fileMD5, forceUpdate, minVersion, versionID, appID, expectedRevision)
		if execErr != nil {
			if isDuplicateEntry(execErr) {
				apiError(c, 400, "该版本已经发布")
				return
			}
			apiError(c, 500, "更新版本失败")
			return
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected != 1 {
			apiError(c, 409, "版本记录已发生变化，请刷新后重试")
			return
		}
		if err := tx.Commit(); err != nil {
			apiError(c, 500, "更新版本失败")
			return
		}
		if old.PackagePath != "" && old.PackagePath != packagePath {
			_ = removeReleasePackage(old.PackagePath)
		}
		newPackagePath = ""
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "版本更新成功"})
		return
	}

	result, err := tx.Exec(`
		INSERT INTO app_versions (
			app_id, from_version, from_version_norm, version, version_norm,
			title, changelog, update_sql, package_name, package_path,
			download_url, file_size_bytes, file_md5, force_update, min_version
		) VALUES (?, '', NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, appID, version, versionNorm,
		title, changelog, updateSQL, packageName, packagePath,
		downloadURL, fileSizeBytes, fileMD5, forceUpdate, minVersion)
	if err != nil {
		if isDuplicateEntry(err) {
			apiError(c, 400, "该版本已经发布")
			return
		}
		apiError(c, 500, "发布版本失败")
		return
	}
	versionID, _ = result.LastInsertId()
	if err := tx.Commit(); err != nil {
		apiError(c, 500, "发布版本失败")
		return
	}
	newPackagePath = ""
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "版本发布成功", "data": gin.H{"id": versionID}})
}

// AppVersionDelete 删除版本记录及其本地更新包。
func AppVersionDelete(c *gin.Context) {
	appID, appErr := positiveInt64(c.Param("id"))
	versionID, versionErr := positiveInt64(c.Param("versionId"))
	if appErr != nil || versionErr != nil {
		apiError(c, 400, "版本参数不正确")
		return
	}

	db, err := openAppVersionDB()
	if err != nil {
		apiError(c, 500, "数据库连接失败")
		return
	}
	defer db.Close()
	if err := EnsureAppVersionsTable(db); err != nil {
		apiError(c, 500, "初始化版本数据失败")
		return
	}

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		apiError(c, 500, "删除版本失败")
		return
	}
	defer tx.Rollback()
	var lockedAppID int64
	if err := tx.QueryRow(`SELECT id FROM apps WHERE id = ? FOR UPDATE`, appID).Scan(&lockedAppID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apiError(c, 404, "应用不存在")
		} else {
			apiError(c, 500, "锁定应用失败")
		}
		return
	}

	var packagePath string
	if err := tx.QueryRow(`
		SELECT package_path FROM app_versions
		WHERE id = ? AND app_id = ? FOR UPDATE
	`, versionID, appID).Scan(&packagePath); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apiError(c, 404, "版本记录不存在")
		} else {
			apiError(c, 500, "查询版本记录失败")
		}
		return
	}
	result, err := tx.Exec(`DELETE FROM app_versions WHERE id = ? AND app_id = ?`, versionID, appID)
	if err != nil {
		apiError(c, 500, "删除版本失败")
		return
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected != 1 {
		apiError(c, 409, "版本记录已发生变化，请刷新后重试")
		return
	}
	if err := tx.Commit(); err != nil {
		apiError(c, 500, "删除版本失败")
		return
	}
	if packagePath != "" {
		_ = removeReleasePackage(packagePath)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "版本删除成功"})
}

// AppVersionCheck 校验客户端授权并返回可用的最新版本。
func AppVersionCheck(c *gin.Context) {
	var req appVersionCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiError(c, 400, "参数错误")
		return
	}
	req.AppKey = strings.TrimSpace(req.AppKey)
	req.CurrentVersion = strings.TrimSpace(req.CurrentVersion)
	req.Domain = normalizeLicenseDomain(req.Domain)
	req.ServerIP = normalizeLicenseServerIP(req.ServerIP)
	req.LicenseKey = strings.TrimSpace(req.LicenseKey)
	req.Sign = strings.ToLower(strings.TrimSpace(req.Sign))
	if !validVersion(req.CurrentVersion) {
		apiError(c, 400, "当前版本号格式不正确")
		return
	}
	if req.Timestamp <= 0 || time.Since(time.Unix(req.Timestamp, 0)) > appVersionCheckMaxSkew || time.Until(time.Unix(req.Timestamp, 0)) > appVersionCheckMaxSkew {
		apiError(c, 403, "请求已过期")
		return
	}

	db, err := openAppVersionDB()
	if err != nil {
		apiError(c, 500, "数据库连接失败")
		return
	}
	defer db.Close()
	if err := EnsureAppVersionsTable(db); err != nil {
		apiError(c, 500, "初始化版本数据失败")
		return
	}
	if err := ensureAppLicenseRequiredColumn(db); err != nil {
		apiError(c, 500, "初始化应用授权开关失败")
		return
	}

	var appID int64
	var appSecret string
	var licenseRequired bool
	if err := db.QueryRow(`SELECT id, app_secret, license_required FROM apps WHERE app_key = ? AND enabled = 1`, req.AppKey).Scan(&appID, &appSecret, &licenseRequired); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apiError(c, 404, "应用不存在或已禁用")
		} else {
			apiError(c, 500, "查询应用失败")
		}
		return
	}

	target := appVersionCheckTarget(req)
	if target == "" {
		apiError(c, 403, "授权目标不能为空")
		return
	}
	if _, ok := appVersionCheckSignValid(req, appSecret); !ok {
		apiError(c, 403, "签名错误")
		return
	}
	verifyReq := licenseVerifyRequest{
		AppKey: req.AppKey, Domain: req.Domain, ServerIP: req.ServerIP, LicenseKey: req.LicenseKey,
	}
	if isLicenseTargetBlacklisted(db, appID, req.Domain, req.ServerIP) {
		apiError(c, 403, "授权目标已被拉黑")
		return
	}
	var license matchedLicense
	if licenseRequired {
		var ok bool
		license, ok, _ = findMatchedLicense(db, appID, verifyReq)
		if !ok || license.Status == "revoked" || license.Status == "expired" || (license.ExpiredAt.Valid && !license.ExpiredAt.Time.After(time.Now())) {
			apiError(c, 403, "授权无效或已过期")
			return
		}
		if license.Type == "key" {
			if siteErr := checkKeyLicenseSite(db, license.ID, req.Domain, req.ServerIP, false); siteErr != nil {
				_, message := licenseSiteFailure(siteErr)
				apiError(c, 403, message)
				return
			}
		}
		if required, verified, verifyErr := userRealnameRequired(db, appID, license.ID); verifyErr == nil && required && !verified {
			apiError(c, 403, "该应用要求实名认证")
			return
		}
	}

	latest, candidates, err := findAvailableAppVersions(db, appID, req.CurrentVersion)
	if err != nil {
		apiError(c, 500, "查询最新版本失败")
		return
	}
	updates := make([]gin.H, 0, len(candidates))
	for _, candidate := range candidates {
		updates = append(updates, gin.H{
			"version":   candidate.Version,
			"title":     candidate.Title,
			"changelog": candidate.Changelog,
			"updateSql": candidate.UpdateSQL,
		})
	}
	if latest.ID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{
			"hasUpdate": false, "currentVersion": req.CurrentVersion, "latestVersion": req.CurrentVersion,
			"reason": "up_to_date", "updates": updates,
		}})
		return
	}

	data := appVersionClientJSON(latest)
	data["currentVersion"] = req.CurrentVersion
	data["latestVersion"] = latest.Version
	data["hasUpdate"] = true
	data["forceUpdate"] = shouldForceUpdate(req.CurrentVersion, latest, candidates)
	data["updates"] = updates
	if latest.PackagePath != "" {
		downloadLicenseID := license.ID
		downloadScope := "license"
		if !licenseRequired {
			downloadLicenseID = 0
			downloadScope = "admin"
		}
		token, tokenErr := createAppVersionDownloadToken(latest.ID, appID, downloadLicenseID, downloadScope)
		if tokenErr != nil {
			apiError(c, 500, "生成下载地址失败")
			return
		}
		data["downloadUrl"] = "/api/app/version/download?token=" + url.QueryEscape(token)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": data})
}

// AppVersionAdminDownloadURL 为后台管理员生成短期下载地址。
func AppVersionAdminDownloadURL(c *gin.Context) {
	appID, appErr := positiveInt64(c.Param("id"))
	versionID, versionErr := positiveInt64(c.Param("versionId"))
	if appErr != nil || versionErr != nil {
		apiError(c, 400, "版本参数不正确")
		return
	}
	db, err := openAppVersionDB()
	if err != nil {
		apiError(c, 500, "数据库连接失败")
		return
	}
	defer db.Close()
	if err := EnsureAppVersionsTable(db); err != nil {
		apiError(c, 500, "初始化版本数据失败")
		return
	}
	var packagePath string
	if err := db.QueryRow(`
		SELECT package_path FROM app_versions WHERE id = ? AND app_id = ?
	`, versionID, appID).Scan(&packagePath); err != nil || packagePath == "" {
		apiError(c, 404, "本地更新包不存在")
		return
	}
	token, err := createAppVersionDownloadToken(versionID, appID, 0, "admin")
	if err != nil {
		apiError(c, 500, "生成下载地址失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{
		"downloadUrl": "/api/app/version/download?token=" + url.QueryEscape(token),
		"expiresIn":   int(appVersionDownloadTTL.Seconds()),
	}})
}

// AppVersionDownload 使用短期签名令牌返回本机保存的更新包。
func AppVersionDownload(c *gin.Context) {
	claims, err := parseAppVersionDownloadToken(strings.TrimSpace(c.Query("token")))
	if err != nil {
		c.Status(http.StatusForbidden)
		return
	}

	db, err := openAppVersionDB()
	if err != nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	defer db.Close()
	if err := EnsureAppVersionsTable(db); err != nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}

	var packagePath, packageName, fileMD5 string
	err = db.QueryRow(`
		SELECT v.package_path, v.package_name, v.file_md5
		FROM app_versions v
		INNER JOIN apps a ON a.id = v.app_id
		WHERE v.id = ? AND v.app_id = ? AND a.enabled = 1 AND v.package_path <> ''
	`, claims.VersionID, claims.AppID).Scan(&packagePath, &packageName, &fileMD5)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if claims.Scope == "license" {
		var valid bool
		if err := db.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM licenses
				WHERE id = ? AND app_id = ? AND status = 'active'
				  AND (expired_at IS NULL OR expired_at > NOW())
			)
		`, claims.LicenseID, claims.AppID).Scan(&valid); err != nil || !valid {
			c.Status(http.StatusForbidden)
			return
		}
	}

	fullPath, err := resolveReleasePackagePath(packagePath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if _, err := os.Stat(fullPath); err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, no-store")
	c.Header("ETag", `"`+fileMD5+`"`)
	c.FileAttachment(fullPath, packageName)
}

type appVersionScanner interface {
	Scan(dest ...any) error
}

func scanAppVersion(scanner appVersionScanner) (appVersionRecord, error) {
	var record appVersionRecord
	err := scanner.Scan(
		&record.ID, &record.AppID, &record.Version, &record.Title, &record.Changelog,
		&record.UpdateSQL, &record.PackageName, &record.PackagePath, &record.DownloadURL,
		&record.FileSizeBytes, &record.FileMD5, &record.ForceUpdate, &record.MinVersion, &record.Revision,
		&record.PublishedAt, &record.UpdatedAt,
	)
	return record, err
}

func appVersionAdminJSON(record appVersionRecord) gin.H {
	sourceType := "url"
	downloadURL := record.DownloadURL
	if record.PackagePath != "" {
		sourceType = "upload"
		downloadURL = ""
	}
	return gin.H{
		"id":            record.ID,
		"appId":         record.AppID,
		"version":       record.Version,
		"title":         record.Title,
		"changelog":     record.Changelog,
		"updateSql":     record.UpdateSQL,
		"packageName":   record.PackageName,
		"sourceType":    sourceType,
		"downloadUrl":   downloadURL,
		"fileSizeBytes": record.FileSizeBytes,
		"fileSizeMb":    bytesToMB(record.FileSizeBytes),
		"fileMd5":       record.FileMD5,
		"forceUpdate":   record.ForceUpdate,
		"minVersion":    record.MinVersion,
		"revision":      record.Revision,
		"publishedAt":   record.PublishedAt.Format("2006-01-02 15:04:05"),
		"updatedAt":     record.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func appVersionClientJSON(record appVersionRecord) gin.H {
	downloadURL := record.DownloadURL
	if record.PackagePath != "" {
		downloadURL = ""
	}
	return gin.H{
		"version":       record.Version,
		"title":         record.Title,
		"changelog":     record.Changelog,
		"packageName":   record.PackageName,
		"downloadUrl":   downloadURL,
		"fileSizeBytes": record.FileSizeBytes,
		"fileSizeMb":    bytesToMB(record.FileSizeBytes),
		"fileMd5":       record.FileMD5,
		"minVersion":    record.MinVersion,
		"publishedAt":   record.PublishedAt.Format("2006-01-02 15:04:05"),
	}
}

func openAppVersionDB() (*sql.DB, error) {
	cfg, err := config.LoadDBConfig()
	if err != nil {
		return nil, err
	}
	return sql.Open("mysql", config.GetDSN(cfg))
}

func apiError(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, gin.H{"code": code, "msg": message})
}

func validateVersionFields(version, title, changelog, minVersion string) error {
	if !validVersion(version) {
		return errors.New("版本号格式不正确")
	}
	if len(title) == 0 || len([]rune(title)) > 200 {
		return errors.New("更新标题不能为空且不能超过200个字符")
	}
	if changelog == "" {
		return errors.New("更新日志不能为空")
	}
	if minVersion != "" && !validVersion(minVersion) {
		return errors.New("最低版本格式不正确")
	}
	if minVersion != "" && compareVersions(minVersion, version) > 0 {
		return errors.New("最低版本不能高于发布版本")
	}
	return nil
}

func validVersion(version string) bool {
	version = strings.TrimSpace(version)
	return len(version) <= 50 && appVersionPattern.MatchString(version)
}

func normalizeVersion(version string) string {
	main, prerelease := splitVersion(version)
	parts := strings.FieldsFunc(main, func(r rune) bool { return r == '.' || r == '_' })
	for len(parts) > 1 && normalizeNumericToken(parts[len(parts)-1]) == "0" {
		parts = parts[:len(parts)-1]
	}
	for index, part := range parts {
		if isDigits(part) {
			parts[index] = normalizeNumericToken(part)
		} else {
			parts[index] = strings.ToLower(part)
		}
	}
	normalized := strings.Join(parts, ".")
	if prerelease == "" {
		return normalized
	}
	preParts := versionTokenSplit.Split(prerelease, -1)
	for index, part := range preParts {
		if isDigits(part) {
			preParts[index] = normalizeNumericToken(part)
		} else {
			preParts[index] = strings.ToLower(part)
		}
	}
	return normalized + "-" + strings.Join(preParts, ".")
}

func normalizedVersionConflict(items []appVersionNormItem) error {
	targetOwners := make(map[string]int64)
	for _, item := range items {
		targetKey := fmt.Sprintf("%d\x00%s", item.AppID, item.VersionNorm)
		if ownerID, exists := targetOwners[targetKey]; exists {
			return fmt.Errorf("app_versions id=%d 与 id=%d 的目标版本语义重复", ownerID, item.ID)
		}
		targetOwners[targetKey] = item.ID
	}
	return nil
}

type appVersionNormItem struct {
	ID          int64
	AppID       int64
	VersionNorm string
}

func normalizeNumericToken(value string) string {
	value = strings.TrimLeft(value, "0")
	if value == "" {
		return "0"
	}
	return value
}

func validateVersionUniqueness(db appVersionQueryer, appID, versionID int64, versionNorm string) error {
	var count int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM app_versions
		WHERE app_id = ? AND version_norm = ? AND id <> ?
	`, appID, versionNorm, versionID).Scan(&count); err != nil {
		return errors.New("校验目标版本失败")
	}
	if count > 0 {
		return errors.New("该版本已经发布")
	}
	return nil
}

type appVersionQueryer interface {
	QueryRow(query string, args ...any) *sql.Row
}

func findHighestAppVersion(db *sql.DB, appID int64) (string, error) {
	rows, err := db.Query(`SELECT version FROM app_versions WHERE app_id = ?`, appID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	latest := ""
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return "", err
		}
		if latest == "" || compareVersions(version, latest) > 0 {
			latest = version
		}
	}
	return latest, rows.Err()
}

func findAvailableAppVersions(db *sql.DB, appID int64, currentVersion string) (appVersionRecord, []appVersionRecord, error) {
	rows, err := db.Query(`
		SELECT id, app_id, version, title, changelog, update_sql, package_name, package_path,
		       download_url, file_size_bytes, file_md5, force_update, min_version, revision, published_at, updated_at
		FROM app_versions WHERE app_id = ?
	`, appID)
	if err != nil {
		return appVersionRecord{}, nil, err
	}
	defer rows.Close()

	var latest appVersionRecord
	candidates := make([]appVersionRecord, 0)
	for rows.Next() {
		record, scanErr := scanAppVersion(rows)
		if scanErr != nil {
			return appVersionRecord{}, nil, scanErr
		}
		comparison := compareVersions(record.Version, currentVersion)
		if comparison < 0 {
			continue
		}
		candidates = append(candidates, record)
		if comparison > 0 && (latest.ID == 0 || compareVersions(record.Version, latest.Version) > 0) {
			latest = record
		}
	}
	if err := rows.Err(); err != nil {
		return appVersionRecord{}, nil, err
	}
	sort.Slice(candidates, func(i, j int) bool {
		return compareVersions(candidates[i].Version, candidates[j].Version) < 0
	})
	return latest, candidates, nil
}

func appVersionCheckTarget(req appVersionCheckRequest) string {
	if req.LicenseKey != "" {
		return req.LicenseKey
	}
	if req.Domain != "" {
		return req.Domain
	}
	return req.ServerIP
}

func appVersionCheckV2Canonical(req appVersionCheckRequest) string {
	return strings.Join([]string{
		licenseSignVersionV2,
		req.AppKey,
		req.CurrentVersion,
		req.LicenseKey,
		normalizeLicenseDomain(req.Domain),
		normalizeLicenseServerIP(req.ServerIP),
		strconv.FormatInt(req.Timestamp, 10),
	}, "\n")
}

func appVersionCheckV2Sign(canonical, appSecret string) string {
	mac := hmac.New(sha256.New, []byte(appSecret))
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

func appVersionCheckSignValid(req appVersionCheckRequest, appSecret string) (string, bool) {
	requestedVersion := strings.ToLower(strings.TrimSpace(req.SignVersion))
	var canonical string
	switch requestedVersion {
	case "2", licenseSignVersionV2:
		canonical = appVersionCheckV2Canonical(req)
		requestedVersion = licenseSignVersionV2
	case "", "1", licenseSignVersionV1:
		canonical = strings.Join([]string{
			req.AppKey,
			req.CurrentVersion,
			appVersionCheckTarget(req),
			strconv.FormatInt(req.Timestamp, 10),
		}, "\n")
		requestedVersion = licenseSignVersionV1
	default:
		return "", false
	}
	mac := hmac.New(sha256.New, []byte(appSecret))
	_, _ = mac.Write([]byte(canonical))
	want := hex.EncodeToString(mac.Sum(nil))
	return requestedVersion, hmac.Equal([]byte(req.Sign), []byte(want))
}

type appVersionDownloadClaims struct {
	VersionID int64
	AppID     int64
	LicenseID int64
	Scope     string
	ExpiresAt int64
}

func createAppVersionDownloadToken(versionID, appID, licenseID int64, scope string) (string, error) {
	if versionID <= 0 || appID <= 0 || (scope != "admin" && scope != "license") {
		return "", errors.New("无效的下载令牌参数")
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	payload := strings.Join([]string{
		strconv.FormatInt(versionID, 10),
		strconv.FormatInt(appID, 10),
		strconv.FormatInt(licenseID, 10),
		scope,
		strconv.FormatInt(time.Now().Add(appVersionDownloadTTL).Unix(), 10),
		base64.RawURLEncoding.EncodeToString(nonce),
	}, ".")
	secret, err := config.LoadOrCreateJWTSecret()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + signature, nil
}

func parseAppVersionDownloadToken(token string) (appVersionDownloadClaims, error) {
	var claims appVersionDownloadClaims
	tokenParts := strings.Split(token, ".")
	if len(tokenParts) != 2 {
		return claims, errors.New("无效的下载令牌")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(tokenParts[0])
	if err != nil {
		return claims, errors.New("无效的下载令牌")
	}
	providedSignature, err := base64.RawURLEncoding.DecodeString(tokenParts[1])
	if err != nil {
		return claims, errors.New("无效的下载令牌")
	}
	secret, err := config.LoadOrCreateJWTSecret()
	if err != nil {
		return claims, err
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payloadBytes)
	if !hmac.Equal(providedSignature, mac.Sum(nil)) {
		return claims, errors.New("下载令牌签名错误")
	}

	parts := strings.Split(string(payloadBytes), ".")
	if len(parts) != 6 {
		return claims, errors.New("无效的下载令牌")
	}
	claims.VersionID, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil || claims.VersionID <= 0 {
		return appVersionDownloadClaims{}, errors.New("无效的版本参数")
	}
	claims.AppID, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil || claims.AppID <= 0 {
		return appVersionDownloadClaims{}, errors.New("无效的应用参数")
	}
	claims.LicenseID, err = strconv.ParseInt(parts[2], 10, 64)
	if err != nil || claims.LicenseID < 0 {
		return appVersionDownloadClaims{}, errors.New("无效的授权参数")
	}
	claims.Scope = parts[3]
	if claims.Scope != "admin" && claims.Scope != "license" {
		return appVersionDownloadClaims{}, errors.New("无效的下载范围")
	}
	if claims.Scope == "license" && claims.LicenseID == 0 {
		return appVersionDownloadClaims{}, errors.New("下载令牌缺少授权")
	}
	claims.ExpiresAt, err = strconv.ParseInt(parts[4], 10, 64)
	if err != nil || time.Now().Unix() >= claims.ExpiresAt {
		return appVersionDownloadClaims{}, errors.New("下载令牌已过期")
	}
	return claims, nil
}

func validateDownloadURL(rawURL string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("下载地址必须是有效的HTTP或HTTPS URL")
	}
	if parsed.User != nil {
		return errors.New("下载地址不能包含账号凭证")
	}
	if len(rawURL) > 2048 {
		return errors.New("下载地址不能超过2048个字符")
	}
	return nil
}

func parseFileSizeMB(value string) (int64, error) {
	sizeMB, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(sizeMB) || math.IsInf(sizeMB, 0) || sizeMB <= 0 {
		return 0, errors.New("文件大小必须大于0MB")
	}
	bytes := int64(math.Round(sizeMB * 1024 * 1024))
	if bytes <= 0 || bytes > appReleaseMaxBytes {
		return 0, errors.New("文件大小不能超过512MB")
	}
	return bytes, nil
}

func bytesToMB(bytes int64) float64 {
	return math.Round(float64(bytes)/1024/1024*1000) / 1000
}

func packageNameFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	name := filepath.Base(parsed.Path)
	if name == "." || name == "/" {
		return ""
	}
	return sanitizePackageName(name)
}

func saveReleasePackage(appID int64, header *multipart.FileHeader) (string, string, int64, string, error) {
	if header == nil {
		return "", "", 0, "", errors.New("请上传更新包")
	}
	if header.Size <= 0 {
		return "", "", 0, "", errors.New("更新包不能为空")
	}
	if header.Size > appReleaseMaxBytes {
		return "", "", 0, "", errors.New("更新包超过512MB")
	}

	source, err := header.Open()
	if err != nil {
		return "", "", 0, "", errors.New("打开更新包失败")
	}
	defer source.Close()

	packageName := sanitizePackageName(header.Filename)
	appDirName := fmt.Sprintf("app-%d", appID)
	appDir := filepath.Join(config.GetAppReleaseDir(), appDirName)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", "", 0, "", errors.New("创建更新包目录失败")
	}

	target, err := os.CreateTemp(appDir, "release-*")
	if err != nil {
		return "", "", 0, "", errors.New("创建更新包文件失败")
	}
	targetPath := target.Name()
	succeeded := false
	defer func() {
		_ = target.Close()
		if !succeeded {
			_ = os.Remove(targetPath)
		}
	}()

	hash := md5.New()
	written, err := io.Copy(io.MultiWriter(target, hash), io.LimitReader(source, appReleaseMaxBytes+1))
	if err != nil {
		return "", "", 0, "", errors.New("保存更新包失败")
	}
	if written > appReleaseMaxBytes {
		return "", "", 0, "", errors.New("更新包超过512MB")
	}
	if written == 0 {
		return "", "", 0, "", errors.New("更新包不能为空")
	}
	if err := target.Sync(); err != nil {
		return "", "", 0, "", errors.New("写入更新包失败")
	}
	if err := target.Close(); err != nil {
		return "", "", 0, "", errors.New("关闭更新包失败")
	}

	relativePath, err := filepath.Rel(config.GetAppReleaseDir(), targetPath)
	if err != nil {
		return "", "", 0, "", errors.New("生成更新包路径失败")
	}
	succeeded = true
	return packageName, filepath.ToSlash(relativePath), written, hex.EncodeToString(hash.Sum(nil)), nil
}

func sanitizePackageName(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	if name == "" || name == "." {
		name = "update-package"
	}
	if len(name) > 255 {
		ext := filepath.Ext(name)
		nameRunes := []rune(strings.TrimSuffix(name, ext))
		extRunes := []rune(ext)
		baseLimit := 255 - len(extRunes)
		if baseLimit < 1 {
			return string([]rune(name)[:255])
		}
		if len(nameRunes) > baseLimit {
			nameRunes = nameRunes[:baseLimit]
		}
		name = string(nameRunes) + string(extRunes)
	}
	return name
}

func resolveReleasePackagePath(relativePath string) (string, error) {
	if strings.TrimSpace(relativePath) == "" || filepath.IsAbs(relativePath) {
		return "", errors.New("无效的更新包路径")
	}
	root := filepath.Clean(config.GetAppReleaseDir())
	fullPath := filepath.Clean(filepath.Join(root, filepath.FromSlash(relativePath)))
	rel, err := filepath.Rel(root, fullPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("无效的更新包路径")
	}
	return fullPath, nil
}

func removeReleasePackage(relativePath string) error {
	fullPath, err := resolveReleasePackagePath(relativePath)
	if err != nil {
		return err
	}
	if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func positiveInt64(value string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("必须是正整数")
	}
	return id, nil
}

func positiveIntDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func formBool(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

func isDuplicateEntry(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func shouldForceUpdate(currentVersion string, latest appVersionRecord, candidates []appVersionRecord) bool {
	for _, candidate := range candidates {
		if compareVersions(candidate.Version, currentVersion) > 0 && candidate.ForceUpdate {
			return true
		}
	}
	return latest.MinVersion != "" && compareVersions(currentVersion, latest.MinVersion) < 0
}

func compareVersions(left, right string) int {
	leftMain, leftPre := splitVersion(left)
	rightMain, rightPre := splitVersion(right)
	leftParts := strings.FieldsFunc(leftMain, func(r rune) bool { return r == '.' || r == '_' })
	rightParts := strings.FieldsFunc(rightMain, func(r rune) bool { return r == '.' || r == '_' })
	if compared := compareTokenLists(leftParts, rightParts, false); compared != 0 {
		return compared
	}
	if leftPre == "" && rightPre != "" {
		return 1
	}
	if leftPre != "" && rightPre == "" {
		return -1
	}
	return compareTokenLists(versionTokenSplit.Split(leftPre, -1), versionTokenSplit.Split(rightPre, -1), true)
}

func splitVersion(version string) (string, string) {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(strings.TrimPrefix(version, "v"), "V")
	if index := strings.IndexByte(version, '+'); index >= 0 {
		version = version[:index]
	}
	if index := strings.IndexByte(version, '-'); index >= 0 {
		return version[:index], version[index+1:]
	}
	return version, ""
}

func compareTokenLists(left, right []string, prerelease bool) int {
	maxLength := len(left)
	if len(right) > maxLength {
		maxLength = len(right)
	}
	for i := 0; i < maxLength; i++ {
		leftToken, rightToken := "0", "0"
		leftMissing, rightMissing := i >= len(left), i >= len(right)
		if !leftMissing {
			leftToken = left[i]
		}
		if !rightMissing {
			rightToken = right[i]
		}
		if prerelease && (leftMissing || rightMissing) {
			if leftMissing && !rightMissing {
				return -1
			}
			if !leftMissing && rightMissing {
				return 1
			}
		}
		if compared := compareVersionToken(leftToken, rightToken, prerelease); compared != 0 {
			return compared
		}
	}
	return 0
}

func compareVersionToken(left, right string, prerelease bool) int {
	leftNumeric := isDigits(left)
	rightNumeric := isDigits(right)
	if leftNumeric && rightNumeric {
		left = strings.TrimLeft(left, "0")
		right = strings.TrimLeft(right, "0")
		if left == "" {
			left = "0"
		}
		if right == "" {
			right = "0"
		}
		if len(left) < len(right) {
			return -1
		}
		if len(left) > len(right) {
			return 1
		}
	} else if prerelease && leftNumeric != rightNumeric {
		if leftNumeric {
			return -1
		}
		return 1
	}
	left = strings.ToLower(left)
	right = strings.ToLower(right)
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
