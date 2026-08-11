package handler

import (
	"database/sql"
	"net/http"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

type menuRow struct {
	ID         int64
	ParentID   int64
	Name       string
	Path       string
	Component  string
	Redirect   string
	Title      string
	Icon       string
	Sort       int
	IsHide     bool
	IsHideTab  bool
	IsFullPage bool
	KeepAlive  bool
	FixedTab   bool
}

type menuResponse struct {
	Name      string          `json:"name"`
	Path      string          `json:"path"`
	Component string          `json:"component,omitempty"`
	Redirect  string          `json:"redirect,omitempty"`
	Meta      menuMeta        `json:"meta"`
	Children  []*menuResponse `json:"children,omitempty"`
}

type menuMeta struct {
	Title      string `json:"title"`
	Icon       string `json:"icon,omitempty"`
	IsHide     bool   `json:"isHide,omitempty"`
	IsHideTab  bool   `json:"isHideTab,omitempty"`
	IsFullPage bool   `json:"isFullPage,omitempty"`
	KeepAlive  bool   `json:"keepAlive,omitempty"`
	FixedTab   bool   `json:"fixedTab,omitempty"`
}

// GetMenuList 获取当前用户的菜单树
func GetMenuList(c *gin.Context) {
	userID, _ := c.Get("user_id")

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
	ensureUserManageMenu(db)
	ensureAgentLevelMenu(db)
	ensureSystemConfigMenu(db)
	ensureEpayConfigMenu(db)
	ensurePaymentOrdersMenu(db)
	ensurePluginStoreMenu(db)
	ensureOnlineUpdateMenu(db)
	ensureMailConfigMenu(db)
	ensureMailLogMenu(db)
	ensureDeveloperDocMenu(db)
	ensureAppVersionMenu(db)
	ensureLicenseCardMenu(db)

	// 查询用户的 role_id
	var roleID sql.NullInt64
	err = db.QueryRow("SELECT role_id FROM admins WHERE id = ?", userID).Scan(&roleID)
	if err != nil || !roleID.Valid {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": []any{}})
		return
	}

	// 查询该角色关联的菜单
	rows, err := db.Query(`
		SELECT m.id, m.parent_id, m.name, m.path, m.component, m.redirect,
		       m.title, m.icon, m.sort, m.is_hide, m.is_hide_tab, m.is_full_page,
		       m.keep_alive, m.fixed_tab
		FROM menus m
		INNER JOIN role_menus rm ON rm.menu_id = m.id
		WHERE rm.role_id = ? AND m.enabled = 1
		ORDER BY m.sort ASC, m.id ASC
	`, roleID.Int64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询菜单失败"})
		return
	}
	defer rows.Close()

	var allMenus []menuRow
	for rows.Next() {
		var m menuRow
		err := rows.Scan(&m.ID, &m.ParentID, &m.Name, &m.Path, &m.Component, &m.Redirect,
			&m.Title, &m.Icon, &m.Sort, &m.IsHide, &m.IsHideTab, &m.IsFullPage,
			&m.KeepAlive, &m.FixedTab)
		if err != nil {
			continue
		}
		allMenus = append(allMenus, m)
	}

	// 组装树形结构
	tree := buildMenuTree(allMenus, 0)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": tree,
	})
}

func buildMenuTree(menus []menuRow, parentID int64) []*menuResponse {
	var result []*menuResponse
	for _, m := range menus {
		if m.ParentID != parentID {
			continue
		}
		node := &menuResponse{
			Name:      m.Name,
			Path:      m.Path,
			Component: m.Component,
			Redirect:  m.Redirect,
			Meta: menuMeta{
				Title:      m.Title,
				Icon:       m.Icon,
				IsHide:     m.IsHide,
				IsHideTab:  m.IsHideTab,
				IsFullPage: m.IsFullPage,
				KeepAlive:  m.KeepAlive,
				FixedTab:   m.FixedTab,
			},
		}
		children := buildMenuTree(menus, m.ID)
		if len(children) > 0 {
			node.Children = children
		}
		result = append(result, node)
	}
	return result
}

// ========== 菜单管理 CRUD ==========

type menuManageItem struct {
	ID         int64             `json:"id"`
	ParentID   int64             `json:"parentId"`
	Name       string            `json:"name"`
	Path       string            `json:"path"`
	Component  string            `json:"component"`
	Redirect   string            `json:"redirect"`
	Title      string            `json:"title"`
	Icon       string            `json:"icon"`
	Sort       int               `json:"sort"`
	IsHide     bool              `json:"isHide"`
	IsHideTab  bool              `json:"isHideTab"`
	IsFullPage bool              `json:"isFullPage"`
	KeepAlive  bool              `json:"keepAlive"`
	FixedTab   bool              `json:"fixedTab"`
	Enabled    bool              `json:"enabled"`
	Children   []*menuManageItem `json:"children,omitempty"`
}

// MenuManageList 菜单管理列表（全量树形）
func MenuManageList(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id, parent_id, name, path, component, redirect, title, icon, sort,
		is_hide, is_hide_tab, is_full_page, keep_alive, fixed_tab, enabled
		FROM menus ORDER BY sort ASC, id ASC`)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	defer rows.Close()

	var all []menuManageItem
	for rows.Next() {
		var m menuManageItem
		rows.Scan(&m.ID, &m.ParentID, &m.Name, &m.Path, &m.Component, &m.Redirect,
			&m.Title, &m.Icon, &m.Sort, &m.IsHide, &m.IsHideTab, &m.IsFullPage,
			&m.KeepAlive, &m.FixedTab, &m.Enabled)
		all = append(all, m)
	}

	tree := buildManageTree(all, 0)

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": tree})
}

func buildManageTree(menus []menuManageItem, parentID int64) []*menuManageItem {
	var result []*menuManageItem
	for i := range menus {
		if menus[i].ParentID != parentID {
			continue
		}
		node := &menuManageItem{}
		*node = menus[i]
		children := buildManageTree(menus, menus[i].ID)
		if len(children) > 0 {
			node.Children = children
		}
		result = append(result, node)
	}
	return result
}

// MenuManageCreate 创建菜单
func MenuManageCreate(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	var req struct {
		ParentID   int64  `json:"parentId"`
		Name       string `json:"name"`
		Path       string `json:"path"`
		Component  string `json:"component"`
		Redirect   string `json:"redirect"`
		Title      string `json:"title"`
		Icon       string `json:"icon"`
		Sort       int    `json:"sort"`
		IsHide     bool   `json:"isHide"`
		IsHideTab  bool   `json:"isHideTab"`
		IsFullPage bool   `json:"isFullPage"`
		KeepAlive  bool   `json:"keepAlive"`
		FixedTab   bool   `json:"fixedTab"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if req.Name == "" || req.Path == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "名称和路径不能为空"})
		return
	}

	result, err := db.Exec(`INSERT INTO menus (parent_id, name, path, component, redirect, title, icon, sort, is_hide, is_hide_tab, is_full_page, keep_alive, fixed_tab)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ParentID, req.Name, req.Path, req.Component, req.Redirect, req.Title, req.Icon, req.Sort,
		req.IsHide, req.IsHideTab, req.IsFullPage, req.KeepAlive, req.FixedTab)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建失败: " + err.Error()})
		return
	}

	id, _ := result.LastInsertId()

	// 自动关联到超级管理员角色
	db.Exec("INSERT IGNORE INTO role_menus (role_id, menu_id) VALUES (1, ?)", id)

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "创建成功", "data": gin.H{"id": id}})
}

// MenuManageUpdate 更新菜单
func MenuManageUpdate(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	id := c.Param("id")
	var req struct {
		ParentID   int64  `json:"parentId"`
		Name       string `json:"name"`
		Path       string `json:"path"`
		Component  string `json:"component"`
		Redirect   string `json:"redirect"`
		Title      string `json:"title"`
		Icon       string `json:"icon"`
		Sort       int    `json:"sort"`
		IsHide     bool   `json:"isHide"`
		IsHideTab  bool   `json:"isHideTab"`
		IsFullPage bool   `json:"isFullPage"`
		KeepAlive  bool   `json:"keepAlive"`
		FixedTab   bool   `json:"fixedTab"`
		Enabled    bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	enabledInt := 0
	if req.Enabled {
		enabledInt = 1
	}

	_, err = db.Exec(`UPDATE menus SET parent_id=?, name=?, path=?, component=?, redirect=?, title=?, icon=?, sort=?,
		is_hide=?, is_hide_tab=?, is_full_page=?, keep_alive=?, fixed_tab=?, enabled=? WHERE id=?`,
		req.ParentID, req.Name, req.Path, req.Component, req.Redirect, req.Title, req.Icon, req.Sort,
		req.IsHide, req.IsHideTab, req.IsFullPage, req.KeepAlive, req.FixedTab, enabledInt, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "更新成功"})
}

// MenuManageDelete 删除菜单
func MenuManageDelete(c *gin.Context) {
	cfg, _ := config.LoadDBConfig()
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "数据库连接失败"})
		return
	}
	defer db.Close()

	id := c.Param("id")

	// 检查是否有子菜单
	var childCount int
	db.QueryRow("SELECT COUNT(*) FROM menus WHERE parent_id=?", id).Scan(&childCount)
	if childCount > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "该菜单下有子菜单，请先删除子菜单"})
		return
	}

	// 删除角色关联
	db.Exec("DELETE FROM role_menus WHERE menu_id=?", id)
	// 删除菜单
	db.Exec("DELETE FROM menus WHERE id=?", id)

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "删除成功"})
}

func ensureUserManageMenu(db *sql.DB) {
	_, _ = db.Exec(`
		INSERT INTO menus (id, parent_id, name, path, component, title, icon, sort, keep_alive, enabled)
		VALUES (201, 0, 'User', '/user-manage', '/system/user', 'menus.system.user', 'ri:user-line', 5, 1, 1)
		ON DUPLICATE KEY UPDATE parent_id = VALUES(parent_id), path = VALUES(path), component = VALUES(component),
			title = VALUES(title), icon = VALUES(icon), sort = VALUES(sort), keep_alive = VALUES(keep_alive), enabled = 1
	`)

	var menuID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'User' LIMIT 1").Scan(&menuID); err != nil || menuID == 0 {
		return
	}

	_, _ = db.Exec(`
		INSERT IGNORE INTO role_menus (role_id, menu_id)
		SELECT id, ? FROM roles WHERE role_code IN ('R_SUPER', 'R_ADMIN') AND enabled = 1
	`, menuID)
}

func ensureAppVersionMenu(db *sql.DB) {
	var parentID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'License' LIMIT 1").Scan(&parentID); err != nil || parentID == 0 {
		return
	}

	_, _ = db.Exec(`
		INSERT INTO menus (parent_id, name, path, component, title, icon, sort, is_hide, keep_alive, enabled)
		VALUES (?, 'AppVersions', 'apps/:id/versions', '/license/app-versions', '版本管理', 'ri:git-branch-line', 99, 1, 0, 1)
		ON DUPLICATE KEY UPDATE parent_id = VALUES(parent_id), path = VALUES(path), component = VALUES(component),
			title = VALUES(title), icon = VALUES(icon), sort = VALUES(sort), is_hide = 1, keep_alive = 0, enabled = 1
	`, parentID)

	var menuID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'AppVersions' LIMIT 1").Scan(&menuID); err != nil || menuID == 0 {
		return
	}

	_, _ = db.Exec(`
		INSERT IGNORE INTO role_menus (role_id, menu_id)
		SELECT DISTINCT rm.role_id, ?
		FROM role_menus rm
		INNER JOIN menus m ON m.id = rm.menu_id
		WHERE m.name = 'LicenseApps'
	`, menuID)
}

func ensureAgentLevelMenu(db *sql.DB) {
	var parentID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'Agent' LIMIT 1").Scan(&parentID); err != nil || parentID == 0 {
		return
	}

	_, _ = db.Exec(`
		INSERT INTO menus (parent_id, name, path, component, title, icon, sort, keep_alive, enabled)
		VALUES (?, 'AgentLevel', 'level', '/agent/level', '等级管理', 'ri:vip-crown-line', 2, 1, 1)
		ON DUPLICATE KEY UPDATE parent_id = VALUES(parent_id), path = VALUES(path), component = VALUES(component),
			title = VALUES(title), icon = VALUES(icon), sort = VALUES(sort), keep_alive = VALUES(keep_alive), enabled = 1
	`, parentID)

	var menuID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'AgentLevel' LIMIT 1").Scan(&menuID); err != nil || menuID == 0 {
		return
	}

	_, _ = db.Exec(`
		INSERT IGNORE INTO role_menus (role_id, menu_id)
		SELECT id, ? FROM roles WHERE enabled = 1
	`, menuID)
}

func ensureSystemConfigMenu(db *sql.DB) {
	var parentID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'System' LIMIT 1").Scan(&parentID); err != nil || parentID == 0 {
		return
	}

	_, _ = db.Exec(`
		INSERT INTO menus (parent_id, name, path, component, title, icon, sort, keep_alive, enabled)
		VALUES (?, 'SystemConfig', 'config', '/system/config', '系统配置', 'ri:settings-3-line', 5, 1, 1)
		ON DUPLICATE KEY UPDATE parent_id = VALUES(parent_id), path = VALUES(path), component = VALUES(component),
			title = VALUES(title), icon = VALUES(icon), sort = VALUES(sort), keep_alive = VALUES(keep_alive), enabled = 1
	`, parentID)

	var menuID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'SystemConfig' LIMIT 1").Scan(&menuID); err != nil || menuID == 0 {
		return
	}

	_, _ = db.Exec("INSERT IGNORE INTO role_menus (role_id, menu_id) VALUES (1, ?)", menuID)
}

func ensureEpayConfigMenu(db *sql.DB) {
	var parentID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'System' LIMIT 1").Scan(&parentID); err != nil || parentID == 0 {
		return
	}

	_, _ = db.Exec(`
		INSERT INTO menus (parent_id, name, path, component, title, icon, sort, keep_alive, enabled)
		VALUES (?, 'EpayConfig', 'epay-config', '/system/epay-config', '支付配置', 'ri:bank-card-line', 6, 1, 1)
		ON DUPLICATE KEY UPDATE parent_id = VALUES(parent_id), path = VALUES(path), component = VALUES(component),
			title = VALUES(title), icon = VALUES(icon), sort = VALUES(sort), keep_alive = VALUES(keep_alive), enabled = 1
	`, parentID)

	var menuID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'EpayConfig' LIMIT 1").Scan(&menuID); err != nil || menuID == 0 {
		return
	}

	_, _ = db.Exec("INSERT IGNORE INTO role_menus (role_id, menu_id) VALUES (1, ?)", menuID)
}

func ensurePluginStoreMenu(db *sql.DB) {
	// 应用商店为一级菜单（parent_id = 0），与系统管理同级。
	_, _ = db.Exec(`
		INSERT INTO menus (id, parent_id, name, path, component, title, icon, sort, keep_alive, enabled)
		VALUES (210, 0, 'PluginStore', '/plugin-store', '/plugin-store/index', 'menus.pluginStore', 'ri:store-2-line', 7, 1, 1)
		ON DUPLICATE KEY UPDATE parent_id = VALUES(parent_id), name = VALUES(name), path = VALUES(path), component = VALUES(component),
			title = VALUES(title), icon = VALUES(icon), sort = VALUES(sort), keep_alive = VALUES(keep_alive), enabled = 1
	`)

	var menuID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'PluginStore' LIMIT 1").Scan(&menuID); err != nil || menuID == 0 {
		return
	}
	_, _ = db.Exec("INSERT IGNORE INTO role_menus (role_id, menu_id) VALUES (1, ?)", menuID)
}

func ensureOnlineUpdateMenu(db *sql.DB) {
	// 在线更新紧跟应用商店，同一 sort 下按 id 排序。
	_, _ = db.Exec(`
		INSERT INTO menus (id, parent_id, name, path, component, title, icon, sort, keep_alive, enabled)
		VALUES (211, 0, 'OnlineUpdate', '/online-update', '/online-update/index', 'menus.onlineUpdate', 'ri:download-cloud-2-line', 7, 1, 1)
		ON DUPLICATE KEY UPDATE parent_id = VALUES(parent_id), name = VALUES(name), path = VALUES(path), component = VALUES(component),
			title = VALUES(title), icon = VALUES(icon), sort = VALUES(sort), keep_alive = VALUES(keep_alive), enabled = 1
	`)

	var menuID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'OnlineUpdate' LIMIT 1").Scan(&menuID); err != nil || menuID == 0 {
		return
	}
	_, _ = db.Exec("INSERT IGNORE INTO role_menus (role_id, menu_id) VALUES (1, ?)", menuID)
}

func ensurePaymentOrdersMenu(db *sql.DB) {
	// 订单列表放在与用户管理同级的一级菜单（parent_id = 0）。
	// 兼容旧数据：此前曾作为 System 子菜单（name=PaymentOrders），一并清理后重建为一级菜单。
	_, _ = db.Exec("DELETE FROM menus WHERE name = 'PaymentOrders'")

	_, _ = db.Exec(`
		INSERT INTO menus (id, parent_id, name, path, component, title, icon, sort, keep_alive, enabled)
		VALUES (209, 0, 'OrderList', '/order-list', '/system/payment-orders', 'menus.system.paymentOrders', 'ri:file-list-3-line', 6, 1, 1)
		ON DUPLICATE KEY UPDATE parent_id = VALUES(parent_id), name = VALUES(name), path = VALUES(path), component = VALUES(component),
			title = VALUES(title), icon = VALUES(icon), sort = VALUES(sort), keep_alive = VALUES(keep_alive), enabled = 1
	`)

	var menuID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'OrderList' LIMIT 1").Scan(&menuID); err != nil || menuID == 0 {
		return
	}

	_, _ = db.Exec(`
		INSERT IGNORE INTO role_menus (role_id, menu_id)
		SELECT id, ? FROM roles WHERE role_code IN ('R_SUPER', 'R_ADMIN') AND enabled = 1
	`, menuID)
}

func ensureMailConfigMenu(db *sql.DB) {
	var parentID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'System' LIMIT 1").Scan(&parentID); err != nil || parentID == 0 {
		return
	}

	_, _ = db.Exec(`
		INSERT INTO menus (parent_id, name, path, component, title, icon, sort, keep_alive, enabled)
		VALUES (?, 'MailConfig', 'mail-config', '/system/mail-config', '邮件配置', 'ri:mail-settings-line', 7, 1, 1)
		ON DUPLICATE KEY UPDATE parent_id = VALUES(parent_id), path = VALUES(path), component = VALUES(component),
			title = VALUES(title), icon = VALUES(icon), sort = VALUES(sort), keep_alive = VALUES(keep_alive), enabled = 1
	`, parentID)

	var menuID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'MailConfig' LIMIT 1").Scan(&menuID); err != nil || menuID == 0 {
		return
	}

	_, _ = db.Exec("INSERT IGNORE INTO role_menus (role_id, menu_id) VALUES (1, ?)", menuID)
}

func ensureLicenseCardMenu(db *sql.DB) {
	var parentID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'License' LIMIT 1").Scan(&parentID); err != nil || parentID == 0 {
		return
	}
	_, _ = db.Exec(`
		INSERT INTO menus (id, parent_id, name, path, component, title, icon, sort, keep_alive, enabled)
		VALUES (306, ?, 'LicenseCards', 'cards', '/license/cards', '卡密管理', 'ri:coupon-3-line', 5, 1, 1)
		ON DUPLICATE KEY UPDATE parent_id = VALUES(parent_id), path = VALUES(path), component = VALUES(component),
			title = VALUES(title), icon = VALUES(icon), sort = VALUES(sort), keep_alive = VALUES(keep_alive), enabled = 1
	`, parentID)
	var menuID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'LicenseCards' LIMIT 1").Scan(&menuID); err != nil || menuID == 0 {
		return
	}
	_, _ = db.Exec(`
		INSERT IGNORE INTO role_menus (role_id, menu_id)
		SELECT id, ? FROM roles WHERE role_code IN ('R_SUPER', 'R_ADMIN') AND enabled = 1
	`, menuID)
}

func ensureDeveloperDocMenu(db *sql.DB) {
	var parentID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'Sdk' LIMIT 1").Scan(&parentID); err != nil || parentID == 0 {
		return
	}

	_, _ = db.Exec(`
		INSERT INTO menus (id, parent_id, name, path, component, title, icon, sort, keep_alive, enabled)
		VALUES (802, ?, 'DeveloperDoc', 'developer-doc', '/sdk/developer-doc', 'menus.system.developerDoc', 'ri:file-code-line', 2, 1, 1)
		ON DUPLICATE KEY UPDATE parent_id = VALUES(parent_id), path = VALUES(path), component = VALUES(component),
			title = VALUES(title), icon = VALUES(icon), sort = VALUES(sort), keep_alive = VALUES(keep_alive), enabled = 1
	`, parentID)

	var menuID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'DeveloperDoc' LIMIT 1").Scan(&menuID); err != nil || menuID == 0 {
		return
	}

	_, _ = db.Exec(`
		INSERT IGNORE INTO role_menus (role_id, menu_id)
		SELECT id, ? FROM roles WHERE role_code IN ('R_SUPER', 'R_ADMIN') AND enabled = 1
	`, menuID)
}

func ensureMailLogMenu(db *sql.DB) {
	var parentID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'System' LIMIT 1").Scan(&parentID); err != nil || parentID == 0 {
		return
	}

	_, _ = db.Exec(`
		INSERT INTO menus (parent_id, name, path, component, title, icon, sort, keep_alive, enabled)
		VALUES (?, 'MailLogs', 'mail-logs', '/system/mail-logs', '邮件日志', 'ri:mail-check-line', 8, 1, 1)
		ON DUPLICATE KEY UPDATE parent_id = VALUES(parent_id), path = VALUES(path), component = VALUES(component),
			title = VALUES(title), icon = VALUES(icon), sort = VALUES(sort), keep_alive = VALUES(keep_alive), enabled = 1
	`, parentID)

	var menuID int64
	if err := db.QueryRow("SELECT id FROM menus WHERE name = 'MailLogs' LIMIT 1").Scan(&menuID); err != nil || menuID == 0 {
		return
	}

	_, _ = db.Exec("INSERT IGNORE INTO role_menus (role_id, menu_id) VALUES (1, ?)", menuID)
}
