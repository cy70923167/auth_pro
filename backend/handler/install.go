package handler

import (
	"database/sql"
	_ "embed"
	"net/http"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

//go:embed schema.sql
var schemaSQL string

//go:embed menu_seed.sql
var menuSeedSQL string

type dbRequest struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type createAdminRequest struct {
	dbRequest
	AdminUsername string `json:"adminUsername"`
	AdminPassword string `json:"adminPassword"`
}

// InstallStatus 仅根据 install.lock 判断安装状态。
func InstallStatus(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"installed": config.IsInstalled(),
	})
}

// InstallTestDB 测试数据库连接
func InstallTestDB(c *gin.Context) {
	var req dbRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	cfg := &config.DBConfig{
		Host:     req.Host,
		Port:     req.Port,
		Database: req.Database,
		Username: req.Username,
		Password: req.Password,
	}

	dsn := cfg.Username + ":" + cfg.Password + "@tcp(" + cfg.Host + ":" + cfg.Port + ")/" + cfg.Database + "?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "连接失败: " + err.Error()})
		return
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "连接失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "连接成功"})
}

// InstallInitTables 初始化数据库表
func InstallInitTables(c *gin.Context) {
	var req dbRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	cfg := &config.DBConfig{
		Host:     req.Host,
		Port:     req.Port,
		Database: req.Database,
		Username: req.Username,
		Password: req.Password,
	}

	dsn := config.GetDSN(cfg)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "连接失败: " + err.Error()})
		return
	}
	defer db.Close()

	if _, err := db.Exec(schemaSQL); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "建表失败: " + err.Error()})
		return
	}

	// 插入菜单种子数据
	if _, err := db.Exec(menuSeedSQL); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "初始化菜单失败: " + err.Error()})
		return
	}

	// 保存数据库配置
	if err := config.SaveDBConfig(cfg); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "保存配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "数据表安装完成"})
}

// InstallCreateAdmin 创建管理员并完成安装
func InstallCreateAdmin(c *gin.Context) {
	var req createAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	cfg, err := config.LoadDBConfig()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取配置失败: " + err.Error()})
		return
	}

	dsn := config.GetDSN(cfg)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "连接失败: " + err.Error()})
		return
	}
	defer db.Close()

	// 插入默认角色（先建角色，管理员需要引用 role_id=1）
	_, _ = db.Exec(`INSERT IGNORE INTO roles (id, role_name, role_code, description, discount, enabled) VALUES
		(1, '超级管理员', 'R_SUPER', '系统超级管理员，拥有所有权限', 10.0, 1),
		(2, '代理商', 'R_AGENT', '代理商角色，可管理下级授权', 8.0, 1),
		(3, '服务商', 'R_SERVICE', '服务商角色，提供技术服务', 7.0, 1),
		(4, '合作商', 'R_PARTNER', '合作商角色，合作推广', 6.5, 1)`)

	// 哈希密码
	hash, err := bcrypt.GenerateFromPassword([]byte(req.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "密码加密失败"})
		return
	}

	// 插入管理员
	_, err = db.Exec(
		"INSERT INTO admins (username, password_hash, nickname, role_id, enabled) VALUES (?, ?, '超级管理员', 1, 1)",
		req.AdminUsername, string(hash),
	)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "创建管理员失败: " + err.Error()})
		return
	}

	// 将全部菜单关联到超级管理员角色
	_, _ = db.Exec("INSERT IGNORE INTO role_menus (role_id, menu_id) SELECT 1, id FROM menus")
	// 给其他角色分配基础菜单（排除系统管理）
	_, _ = db.Exec("INSERT IGNORE INTO role_menus (role_id, menu_id) SELECT 2, id FROM menus WHERE name NOT LIKE 'System%' AND name NOT LIKE 'Menu%'")
	_, _ = db.Exec("INSERT IGNORE INTO role_menus (role_id, menu_id) SELECT 3, id FROM menus WHERE name NOT LIKE 'System%' AND name NOT LIKE 'Menu%'")
	_, _ = db.Exec("INSERT IGNORE INTO role_menus (role_id, menu_id) SELECT 4, id FROM menus WHERE name NOT LIKE 'System%' AND name NOT LIKE 'Menu%'")

	// 生成并持久化 JWT 签名密钥（已存在则复用，避免轮换导致旧 token 失效）
	if _, err := config.LoadOrCreateJWTSecret(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "生成 JWT 密钥失败: " + err.Error()})
		return
	}

	// 创建锁文件
	if err := config.CreateLockFile(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "创建锁文件失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "安装完成"})
}
