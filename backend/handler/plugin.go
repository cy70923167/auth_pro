package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

// ========== 应用商店（插件中心） ==========
//
// 插件化能力位：支付服务商、实名认证服务商等以后续可扩展的方式注册。
// 每个插件是一条 plugins 表记录（id 唯一），enabled 控制该能力位使用哪个实现。
// 同一 category 下同时只允许一个插件处于启用状态（启用一个会自动停用同类其他插件）。

type pluginInfo struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Homepage    string `json:"homepage"`
	Icon        string `json:"icon"`
	Version     string `json:"version"`
	Official    bool   `json:"official"`
	Enabled     bool   `json:"enabled"`
	Configured  bool   `json:"configured"`
	Local       bool   `json:"local"`       // 本地已有（内置或已下载）
	Source      string `json:"source"`      // 内置: builtin；远程插件: 来源仓库名
	Remote      bool   `json:"remote"`      // 仅存在于远程仓库、本地未下载
	DownloadURL string `json:"downloadUrl"` // 远程插件包地址（未下载时用于下载）
	Hidden      bool   `json:"-"`           // 暂时从应用商店隐藏，底层能力与历史状态保留
}

// pluginSourceRecord 软件源（远程插件仓库）。
type pluginSourceRecord struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
}

// remotePluginIndex 仓库清单（仓库 URL 指向的 JSON）。
type remotePluginIndex struct {
	Name    string `json:"name"`
	Plugins []struct {
		ID          string `json:"id"`
		Category    string `json:"category"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
		Version     string `json:"version"`
		DownloadURL string `json:"downloadUrl"`
	} `json:"plugins"`
}

// pluginCatalog 内置插件清单（代码注册，数据库只持久化启用状态）。
var pluginCatalog = []pluginInfo{
	{
		ID:          "epay",
		Category:    "payment",
		Name:        "易支付 V1",
		Description: "彩虹易支付聚合支付接口（MD5 页面跳转版），支持支付宝 / 微信 / QQ 钱包收单",
		Icon:        "ri:bank-card-line",
		Version:     "1.0.0",
		Official:    true,
	},
	{
		ID:          "epay-v2",
		Category:    "payment",
		Name:        "易支付 V2",
		Description: "彩虹易支付 V2 接口（RSA-SHA256 服务端下单），支持支付宝 / 微信 / QQ 钱包收单",
		Icon:        "ri:bank-card-2-line",
		Version:     "2.0.0",
		Official:    true,
	},
	{
		ID:          "alipay-realname",
		Category:    "realname",
		Name:        "支付宝实名认证",
		Description: "金融级实人认证，用户扫码刷脸完成核验，权威性强",
		Icon:        "ri:alipay-line",
		Version:     "1.0.0",
		Official:    true,
	},
	{
		ID:          "kuaitong-realname",
		Category:    "realname",
		Name:        "快瞳实名认证",
		Description: "支持姓名与身份证二要素核验，也可扫码拍照完成人脸认证",
		Icon:        "ri:id-card-line",
		Version:     "1.0.0",
		Official:    true,
	},
	{
		ID:          "tencent-realname",
		Category:    "realname",
		Name:        "靓仔聚合认证",
		Description: "靓仔聚合实名认证服务:接入地址为:http://real.4775.cn/",
		Homepage:    "http://real.4775.cn/",
		Icon:        "ri:id-card-line",
		Version:     "1.0.0",
		Official:    true,
	},
	{
		ID:          "xiaomu-realname",
		Category:    "realname",
		Name:        "小沐聚合实名",
		Description: "小沐聚合实名认证服务，支持三要素核验、人脸认证与微信实名，认证产品在系统配置中切换",
		Homepage:    "https://smapi.x1m1.cn/",
		Icon:        "ri:shield-user-line",
		Version:     "1.0.0",
		Official:    true,
	},
}

func findCatalogPlugin(id string) (pluginInfo, bool) {
	for _, p := range pluginCatalog {
		if p.ID == id {
			return p, true
		}
	}
	return pluginInfo{}, false
}

func listedCatalogPlugins() []pluginInfo {
	plugins := make([]pluginInfo, 0, len(pluginCatalog))
	for _, plugin := range pluginCatalog {
		if !plugin.Hidden {
			plugins = append(plugins, plugin)
		}
	}
	return plugins
}

// ensurePluginStorage 幂等建表。
func ensurePluginStorage(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS plugins (
			id VARCHAR(60) NOT NULL PRIMARY KEY COMMENT '插件标识',
			category VARCHAR(30) NOT NULL DEFAULT '' COMMENT '能力分类',
			enabled TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否启用',
			updated_at DATETIME DEFAULT NULL COMMENT '更新时间',
			KEY idx_category (category)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='应用商店插件'
	`); err != nil {
		return err
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS plugin_sources (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(60) NOT NULL DEFAULT '' COMMENT '软件源名称',
			url VARCHAR(500) NOT NULL COMMENT '仓库清单地址',
			created_at DATETIME DEFAULT NULL COMMENT '添加时间',
			UNIQUE KEY uk_url (url(191))
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='插件软件源'
	`); err != nil {
		return err
	}
	if err := ensureSystemConfigStorage(db); err != nil {
		return err
	}
	return ensureDefaultPluginState(db)
}

// ensureDefaultPluginState 只在腾讯实名插件尚无状态记录时执行一次默认迁移。
func ensureDefaultPluginState(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		INSERT INTO plugins (id, category, enabled, updated_at)
		VALUES ('tencent-realname', 'realname', 1, NOW())
		ON DUPLICATE KEY UPDATE id = VALUES(id)
	`)
	if err != nil {
		return err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if inserted == 0 {
		return tx.Commit()
	}

	if _, err := tx.Exec(`
		UPDATE plugins SET enabled = 0, updated_at = NOW()
		WHERE category = 'realname' AND id != 'tencent-realname'
	`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO system_configs (` + "`group`" + `, ` + "`key`" + `, value, description)
		VALUES ('realname', 'provider', 'tencent', '实名认证服务商(alipay/kuaitong/tencent)')
		ON DUPLICATE KEY UPDATE value = VALUES(value)
	`); err != nil {
		return err
	}
	return tx.Commit()
}

func loadPluginEnabledMap(db *sql.DB) (map[string]bool, error) {
	enabled := map[string]bool{}
	rows, err := db.Query("SELECT id, enabled FROM plugins")
	if err != nil {
		return enabled, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var en int
		if err := rows.Scan(&id, &en); err != nil {
			return enabled, err
		}
		enabled[id] = en == 1
	}
	return enabled, rows.Err()
}

// pluginConfigured 判断插件是否已填写可用配置。
func pluginConfigured(db *sql.DB, id string) bool {
	switch id {
	case "epay":
		cfg, err := loadEpayConfig(db)
		return err == nil && cfg.Gateway != "" && cfg.PID != "" && cfg.Key != ""
	case "epay-v2":
		cfg, err := loadEpayV2Config(db)
		return err == nil && cfg.Gateway != "" && cfg.PID != "" && cfg.MerchantKey != "" && cfg.PlatformKey != ""
	case "alipay-realname":
		cfg, err := loadRealnameConfig(db)
		return err == nil && cfg.AppID != "" && cfg.PrivateKey != "" && cfg.AlipayPublicKey != ""
	case "kuaitong-realname":
		cfg, err := loadRealnameConfig(db)
		return err == nil && cfg.KuaitongAccessKey != "" && cfg.KuaitongSecret != ""
	case "tencent-realname":
		cfg, err := loadRealnameConfig(db)
		return err == nil && cfg.TencentAPIKey != "" && cfg.TencentAPISecret != "" && cfg.TencentBaseURL != "" && cfg.TencentProductCode != ""
	case "xiaomu-realname":
		cfg, err := loadRealnameConfig(db)
		return err == nil && cfg.XiaomuAppKey != "" && cfg.XiaomuAppSecret != "" && cfg.XiaomuBaseURL != "" && cfg.XiaomuProductMode != ""
	}
	return false
}

// AdminPluginList 插件列表，按分类分组返回；合并本地插件与各软件源的远程插件。
// 支持 ?source=<id> 只看某个软件源、?source=local 只看本地、?q= 关键词过滤。
func AdminPluginList(c *gin.Context) {
	db, err := openSystemConfigDB()
	if err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	if err := ensurePluginStorage(db); err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 500, "msg": "初始化插件存储失败"})
		return
	}

	enabledMap, err := loadPluginEnabledMap(db)
	if err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 500, "msg": "读取插件状态失败"})
		return
	}
	localIDs, err := loadLocalPluginIDs()
	if err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 500, "msg": "读取本地插件失败"})
		return
	}
	sources, err := listPluginSources(db)
	if err != nil {
		writeSystemConfig(c, http.StatusOK, gin.H{"code": 500, "msg": "读取软件源失败"})
		return
	}

	sourceFilter := strings.TrimSpace(c.Query("source"))
	keyword := strings.ToLower(strings.TrimSpace(c.Query("q")))

	// 本地插件（内置 + 已下载）
	local := make([]pluginInfo, 0, len(pluginCatalog))
	if sourceFilter == "" || sourceFilter == "local" {
		for _, p := range listedCatalogPlugins() {
			p.Enabled = enabledMap[p.ID]
			p.Configured = pluginConfigured(db, p.ID)
			p.Local = true
			p.Source = "builtin"
			local = append(local, p)
		}
	}

	// 远程插件：本地已有的按本地处理，只补充本地没有的
	remote := make([]pluginInfo, 0)
	sourceOK := map[int64]bool{}
	if sourceFilter != "local" {
		for _, src := range sources {
			if sourceFilter != "" && sourceFilter != fmt.Sprintf("%d", src.ID) {
				continue
			}
			idx, err := fetchPluginSourceIndex(src.URL)
			if err != nil {
				sourceOK[src.ID] = false
				continue
			}
			sourceOK[src.ID] = true
			srcName := src.Name
			if srcName == "" {
				srcName = idx.Name
			}
			for _, rp := range idx.Plugins {
				if rp.ID == "" || localIDs[rp.ID] {
					continue // 本地已有，按本地插件展示
				}
				icon := rp.Icon
				if icon == "" {
					icon = "ri:puzzle-line"
				}
				remote = append(remote, pluginInfo{
					ID:          rp.ID,
					Category:    normalizePluginCategory(rp.Category),
					Name:        rp.Name,
					Description: rp.Description,
					Icon:        icon,
					Version:     rp.Version,
					Local:       false,
					Remote:      true,
					Source:      srcName,
					DownloadURL: rp.DownloadURL,
				})
			}
		}
	}

	matchKeyword := func(p pluginInfo) bool {
		if keyword == "" {
			return true
		}
		return strings.Contains(strings.ToLower(p.Name), keyword) ||
			strings.Contains(strings.ToLower(p.Description), keyword) ||
			strings.Contains(strings.ToLower(p.ID), keyword)
	}

	type categoryGroup struct {
		Category string       `json:"category"`
		Title    string       `json:"title"`
		Plugins  []pluginInfo `json:"plugins"`
	}
	categories := []struct {
		key   string
		title string
	}{
		{"payment", "支付插件"},
		{"realname", "实名认证服务商"},
		{"other", "其他插件"},
	}

	groups := make([]categoryGroup, 0, len(categories))
	for _, cat := range categories {
		group := categoryGroup{Category: cat.key, Title: cat.title, Plugins: []pluginInfo{}}
		for _, p := range local {
			if normalizePluginCategory(p.Category) == cat.key && matchKeyword(p) {
				group.Plugins = append(group.Plugins, p)
			}
		}
		for _, p := range remote {
			if normalizePluginCategory(p.Category) == cat.key && matchKeyword(p) {
				group.Plugins = append(group.Plugins, p)
			}
		}
		groups = append(groups, group)
	}

	// 源可用性输出给前端提示
	sourceStates := make([]gin.H, 0, len(sources))
	for _, src := range sources {
		ok, checked := sourceOK[src.ID]
		state := "unknown"
		if checked {
			if ok {
				state = "ok"
			} else {
				state = "error"
			}
		}
		sourceStates = append(sourceStates, gin.H{"id": src.ID, "name": src.Name, "url": src.URL, "state": state})
	}

	writeSystemConfig(c, http.StatusOK, gin.H{"code": 200, "msg": "", "data": gin.H{
		"categories": groups,
		"sources":    sourceStates,
	}})
}

func normalizePluginCategory(category string) string {
	switch category {
	case "payment", "realname":
		return category
	}
	return "other"
}

// loadLocalPluginIDs 返回本地已有的插件 id（内置 + plugins 目录下已下载）。
func loadLocalPluginIDs() (map[string]bool, error) {
	ids := map[string]bool{}
	for _, p := range pluginCatalog {
		ids[p.ID] = true
	}
	entries, err := os.ReadDir(config.GetPluginDir())
	if err != nil {
		return ids, nil // 目录不存在视为无已下载插件
	}
	for _, entry := range entries {
		if entry.IsDir() {
			ids[entry.Name()] = true
		}
	}
	return ids, nil
}

// ========== 软件源管理 ==========

func listPluginSources(db *sql.DB) ([]pluginSourceRecord, error) {
	rows, err := db.Query("SELECT id, name, url, created_at FROM plugin_sources ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sources := make([]pluginSourceRecord, 0)
	for rows.Next() {
		var src pluginSourceRecord
		var createdAt sql.NullTime
		if err := rows.Scan(&src.ID, &src.Name, &src.URL, &createdAt); err != nil {
			return nil, err
		}
		if createdAt.Valid {
			src.CreatedAt = createdAt.Time
		}
		sources = append(sources, src)
	}
	return sources, rows.Err()
}

var pluginSourceURLPattern = regexp.MustCompile(`^https?://`)

func validatePluginSourceURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !pluginSourceURLPattern.MatchString(raw) {
		return "", errors.New("仓库地址必须以 http:// 或 https:// 开头")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", errors.New("仓库地址格式不正确")
	}
	return raw, nil
}

// fetchPluginSourceIndex 拉取远程仓库清单。
func fetchPluginSourceIndex(rawURL string) (*remotePluginIndex, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("仓库连接失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("仓库返回状态码 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, errors.New("读取仓库清单失败")
	}
	var idx remotePluginIndex
	if err := json.Unmarshal(body, &idx); err != nil {
		return nil, errors.New("仓库清单不是有效的 JSON")
	}
	return &idx, nil
}

// AdminPluginSourceAdd 添加软件源；会立即拉取一次仓库清单做校验。
func AdminPluginSourceAdd(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	repoURL, err := validatePluginSourceURL(req.URL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	req.Name = strings.TrimSpace(req.Name)

	idx, err := fetchPluginSourceIndex(repoURL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "仓库校验失败：" + err.Error()})
		return
	}
	if req.Name == "" {
		req.Name = idx.Name
	}
	if req.Name == "" {
		req.Name = "未命名仓库"
	}
	if len([]rune(req.Name)) > 60 {
		req.Name = string([]rune(req.Name)[:60])
	}

	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensurePluginStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化插件存储失败"})
		return
	}

	if _, err := db.Exec("INSERT INTO plugin_sources (name, url, created_at) VALUES (?, ?, NOW()) ON DUPLICATE KEY UPDATE name = VALUES(name)", req.Name, repoURL); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存软件源失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": fmt.Sprintf("软件源已添加，发现 %d 个插件", len(idx.Plugins))})
}

// AdminPluginSourceDelete 删除软件源（不影响已下载到本地的插件）。
func AdminPluginSourceDelete(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensurePluginStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化插件存储失败"})
		return
	}
	if _, err := db.Exec("DELETE FROM plugin_sources WHERE id = ?", id); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "删除软件源失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "软件源已删除"})
}

// ========== 插件下载 ==========

var pluginIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,58}$`)

// AdminPluginDownload 从软件源下载插件包到本地 plugins 目录。
func AdminPluginDownload(c *gin.Context) {
	pluginID := strings.TrimSpace(c.Param("id"))
	if !pluginIDPattern.MatchString(pluginID) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "插件标识不合法"})
		return
	}

	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()
	if err := ensurePluginStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化插件存储失败"})
		return
	}

	localIDs, _ := loadLocalPluginIDs()
	if localIDs[pluginID] {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "插件已在本地，无需下载"})
		return
	}

	// 在所有软件源中定位该插件
	sources, err := listPluginSources(db)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取软件源失败"})
		return
	}
	downloadURL := ""
	for _, src := range sources {
		idx, err := fetchPluginSourceIndex(src.URL)
		if err != nil {
			continue
		}
		for _, rp := range idx.Plugins {
			if rp.ID == pluginID {
				downloadURL = strings.TrimSpace(rp.DownloadURL)
				break
			}
		}
		if downloadURL != "" {
			break
		}
	}
	if downloadURL == "" {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "未在任何软件源中找到该插件或插件未提供下载地址"})
		return
	}
	if _, err := validatePluginSourceURL(downloadURL); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "插件下载地址不合法"})
		return
	}

	payload, err := downloadPluginPackage(downloadURL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "下载失败：" + err.Error()})
		return
	}

	pluginDir := filepath.Join(config.GetPluginDir(), pluginID)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建插件目录失败"})
		return
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.pkg"), payload, 0644); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存插件包失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "插件已下载到本地"})
}

func downloadPluginPackage(rawURL string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("连接失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载地址返回状态码 %d", resp.StatusCode)
	}
	// 单插件包限制 20MB
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, errors.New("读取插件包失败")
	}
	if len(payload) == 0 {
		return nil, errors.New("插件包为空")
	}
	return payload, nil
}

// AdminPluginToggle 启用/停用插件；启用时自动停用同分类其他插件。
func AdminPluginToggle(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	plugin, ok := findCatalogPlugin(id)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "插件不存在"})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	db, err := openSystemConfigDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	if err := ensurePluginStorage(db); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "初始化插件存储失败"})
		return
	}

	if req.Enabled {
		// 同分类互斥：先停用同类，再启用目标
		if _, err := db.Exec("UPDATE plugins SET enabled = 0, updated_at = NOW() WHERE category = ? AND id != ?", plugin.Category, id); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "更新插件状态失败"})
			return
		}
	}

	enabledVal := 0
	if req.Enabled {
		enabledVal = 1
	}
	if _, err := db.Exec(`
		INSERT INTO plugins (id, category, enabled, updated_at)
		VALUES (?, ?, ?, NOW())
		ON DUPLICATE KEY UPDATE enabled = VALUES(enabled), updated_at = NOW()
	`, id, plugin.Category, enabledVal); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "保存插件状态失败"})
		return
	}

	// 实名/支付的启用插件变更后，同步对应模块配置中的 provider，保持一处生效
	switch plugin.Category {
	case "realname":
		provider := realnameProviderAlipay
		if req.Enabled && id == "kuaitong-realname" {
			provider = realnameProviderKuaitong
		} else if req.Enabled && id == "tencent-realname" {
			provider = realnameProviderTencent
		} else if req.Enabled && id == "xiaomu-realname" {
			provider = realnameProviderXiaomu
		}
		_, _ = db.Exec(`
			INSERT INTO system_configs (`+"`group`"+`, `+"`key`"+`, value, description)
			VALUES ('realname', 'provider', ?, '实名认证服务商(alipay/kuaitong/tencent/xiaomu)')
			ON DUPLICATE KEY UPDATE value = VALUES(value)
		`, provider)
	}

	msg := "插件已停用"
	if req.Enabled {
		msg = "插件已启用"
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": msg})
}

// isPluginEnabled 供其他模块查询插件是否启用；存储异常时按未启用处理。
func isPluginEnabled(db *sql.DB, id string) bool {
	if err := ensurePluginStorage(db); err != nil {
		return false
	}
	var enabled int
	if err := db.QueryRow("SELECT enabled FROM plugins WHERE id = ?", id).Scan(&enabled); err != nil {
		return false
	}
	return enabled == 1
}

// realnamePluginIDByProvider 反查 provider 对应的插件 id。
func realnamePluginIDByProvider(provider string) string {
	if provider == realnameProviderKuaitong {
		return "kuaitong-realname"
	}
	if provider == realnameProviderTencent {
		return "tencent-realname"
	}
	if provider == realnameProviderXiaomu {
		return "xiaomu-realname"
	}
	return "alipay-realname"
}

var _ = json.Marshal // 保留 encoding/json 引用，后续插件自定义元数据会用到
