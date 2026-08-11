package main

import (
	"database/sql"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"auto_pro/config"
	"auto_pro/handler"
	"auto_pro/middleware"

	"github.com/gin-gonic/gin"
)

//go:embed static/*
var staticFS embed.FS

func main() {
	r := gin.Default()

	// CORS
	r.Use(middleware.Cors())

	// API 路由
	api := r.Group("/api")
	{
		// 安装相关（无需鉴权）
		install := api.Group("/install")
		{
			install.GET("/status", handler.InstallStatus)
			install.POST("/test-db", handler.InstallTestDB)
			install.POST("/init-tables", handler.InstallInitTables)
			install.POST("/create-admin", handler.InstallCreateAdmin)
		}

		// 公开系统配置（登录前可读取品牌信息）
		api.GET("/system-config/public", handler.PublicSystemConfig)
		api.GET("/system/version", handler.SystemVersion)

		// 公开授权校验（SDK 调用，无需后台登录）
		api.POST("/license/verify", handler.LicenseVerify)
		// 应用客户端使用签名授权检查版本，并通过短期令牌下载本地更新包。
		api.POST("/app/version/check", handler.AppVersionCheck)
		api.GET("/app/version/download", handler.AppVersionDownload)

		// 易支付回调（支付网关调用，无需登录）
		api.Any("/payment/easypay/notify", handler.EpayNotify)
		api.Any("/payment/easypay/return", handler.EpayReturn)

		// 易支付 V2 回调（支付网关调用，无需登录）
		api.Any("/payment/easypay-v2/notify", handler.EpayV2Notify)
		api.Any("/payment/easypay-v2/return", handler.EpayV2Return)

		// 快瞳 / 腾讯云增强人脸拍照提交（扫码手机端调用，token 即凭证，无需登录）
		api.POST("/realname/face/submit", handler.RealnameFaceSubmit)

		// 认证相关（无需鉴权）
		auth := api.Group("/auth")
		{
			auth.POST("/login", handler.Login)
		}

		// 代理端（无需管理员鉴权）
		agentAuth := api.Group("/agent-panel")
		{
			agentAuth.POST("/login", handler.AgentPanelLogin)
		}

		// 代理端（需鉴权）
		agentSecured := api.Group("/agent-panel")
		agentSecured.Use(middleware.JWTAuth())
		{
			agentSecured.GET("/apps", handler.AgentPanelAppList)
			agentSecured.GET("/apps/purchase", handler.AgentPanelPurchaseApps)
			agentSecured.GET("/users/options", handler.AgentPanelUserOptions)
			agentSecured.GET("/licenses", handler.AgentPanelLicenseList)
			agentSecured.POST("/cards/redeem", handler.AgentLicenseCardRedeem)
			agentSecured.PUT("/licenses/:id", handler.AgentPanelLicenseUpdate)
			agentSecured.POST("/licenses/:id/refresh-key", handler.AgentPanelLicenseRefreshKey)
			agentSecured.GET("/balance", handler.AgentPanelBalance)
			agentSecured.GET("/profile", handler.AgentPanelProfile)
			agentSecured.PUT("/profile", handler.AgentPanelUpdateProfile)
			agentSecured.POST("/change-password", handler.AgentPanelChangePassword)
			agentSecured.POST("/realname/init", handler.AgentRealnameInit)
			agentSecured.GET("/realname/query", handler.AgentRealnameQuery)
			agentSecured.GET("/dashboard/stats", handler.AgentPanelStats)
			agentSecured.GET("/dashboard/info", handler.AgentPanelInfo)
			agentSecured.GET("/dashboard/trend", handler.AgentPanelTrend)
			agentSecured.GET("/dashboard/app-dist", handler.AgentPanelAppDist)
			agentSecured.GET("/dashboard/recent-licenses", handler.AgentPanelRecentLicenses)
			agentSecured.GET("/finance/overview", handler.AgentPanelFinanceOverview)
			agentSecured.GET("/finance/quotas", handler.AgentPanelFinanceQuotas)
			agentSecured.GET("/finance/transactions", handler.AgentPanelFinanceTransactions)
			agentSecured.GET("/recharge/options", handler.AgentPanelRechargeOptions)
			agentSecured.POST("/recharge/orders", handler.AgentPanelRechargeCreate)
			agentSecured.GET("/recharge/orders/:orderNo", handler.AgentPanelRechargeStatus)
			agentSecured.GET("/purchase/pay-options", handler.AgentPanelPurchasePayOptions)
			agentSecured.POST("/purchase", handler.AgentPanelPurchase)
			agentSecured.GET("/purchase/orders/:orderNo", handler.AgentPanelPurchaseOrderStatus)
		}

		// 用户端（无需管理员鉴权）
		userAuth := api.Group("/user-panel")
		{
			userAuth.POST("/login", handler.UserLogin)
			userAuth.GET("/license-query", handler.PublicUserLicenseQuery)
			userAuth.GET("/agent-query", handler.PublicAgentQuery)
			userAuth.POST("/register/email-code", handler.UserSendRegisterEmailCode)
			userAuth.POST("/register", handler.UserRegister)
			userAuth.POST("/forgot-password", handler.UserForgotPassword)
			userAuth.POST("/reset-password", handler.UserResetPassword)
		}

		// 用户端（需鉴权）
		userSecured := api.Group("/user-panel")
		userSecured.Use(middleware.JWTAuth())
		{
			userSecured.GET("/dashboard", handler.UserDashboard)
			userSecured.GET("/licenses", handler.UserLicenseList)
			userSecured.POST("/cards/redeem", handler.UserLicenseCardRedeem)
			userSecured.PUT("/licenses/:id/target", handler.UserLicenseUpdateTarget)
			userSecured.POST("/licenses/:id/refresh-key", handler.UserLicenseRefreshKey)
			userSecured.GET("/apps", handler.UserAppList)
			userSecured.GET("/apps/purchase", handler.UserAppListForPurchase)
			userSecured.GET("/balance", handler.UserGetBalance)
			userSecured.GET("/recharge/options", handler.UserRechargeOptions)
			userSecured.POST("/recharge/orders", handler.UserRechargeCreate)
			userSecured.GET("/recharge/orders/:orderNo", handler.UserRechargeStatus)
			userSecured.GET("/recharge-v2/options", handler.UserRechargeV2Options)
			userSecured.POST("/recharge-v2/orders", handler.UserRechargeV2Create)
			userSecured.GET("/purchase/pay-options", handler.UserPurchasePayOptions)
			userSecured.POST("/purchase", handler.UserPurchase)
			userSecured.GET("/purchase/orders/:orderNo", handler.UserPurchaseOrderStatus)
			userSecured.GET("/profile", handler.UserProfile)
			userSecured.PUT("/profile", handler.UserUpdateProfile)
			userSecured.POST("/change-password", handler.UserChangePassword)
			userSecured.POST("/realname/init", handler.UserRealnameInit)
			userSecured.GET("/realname/query", handler.UserRealnameQuery)
		}

		// 需要鉴权的路由（仅管理员角色）
		secured := api.Group("/")
		secured.Use(middleware.JWTAuth(), middleware.RequireAdmin())
		{
			secured.GET("/user/info", handler.GetUserInfo)
			secured.POST("/user/change-password", handler.ChangePassword)
			secured.GET("/user/list", handler.AdminUserList)
			secured.POST("/user/create", handler.AdminUserCreate)
			secured.PUT("/user/:id", handler.AdminUserUpdate)
			secured.PUT("/user/:id/toggle", handler.AdminUserToggle)
			secured.DELETE("/user/:id", handler.AdminUserDelete)
			secured.POST("/user/:id/impersonate", handler.AdminImpersonateUser)
			secured.GET("/system/menus", handler.GetMenuList)
			secured.GET("/system/config", handler.AdminSystemConfig)
			secured.PUT("/system/config", handler.AdminSystemConfigUpdate)
			secured.PUT("/system/config/switch/:key", handler.AdminSystemFeatureSwitchUpdate)
			secured.GET("/system/payment-config", handler.AdminPaymentConfig)
			secured.PUT("/system/payment-config", handler.AdminPaymentConfigUpdate)
			secured.POST("/system/payment-config/test", handler.AdminPaymentTestCreate)
			secured.GET("/system/payment-config/test/:orderNo", handler.AdminPaymentTestStatus)
			secured.GET("/system/payment-v2-config", handler.AdminPaymentV2Config)
			secured.PUT("/system/payment-v2-config", handler.AdminPaymentV2ConfigUpdate)
			secured.POST("/system/payment-v2-config/test", handler.AdminPaymentV2TestCreate)
			secured.GET("/system/payment-v2-config/test/:orderNo", handler.AdminPaymentV2TestStatus)
			secured.GET("/system/payment-orders", handler.AdminPaymentOrderList)
			secured.GET("/system/plugins", handler.AdminPluginList)
			secured.POST("/system/plugins/:id/toggle", handler.AdminPluginToggle)
			secured.POST("/system/plugins/:id/download", handler.AdminPluginDownload)
			secured.POST("/system/plugin-sources", handler.AdminPluginSourceAdd)
			secured.DELETE("/system/plugin-sources/:id", handler.AdminPluginSourceDelete)
			secured.GET("/system/realname-config", handler.AdminRealnameConfig)
			secured.PUT("/system/realname-config", handler.AdminRealnameConfigUpdate)
			secured.POST("/system/realname-products", handler.AdminRealnameProducts)
			secured.GET("/system/realname-records", handler.AdminRealnameRecordList)
			secured.GET("/system/mail-config", handler.AdminMailConfig)
			secured.PUT("/system/mail-config", handler.AdminMailConfigUpdate)
			secured.PUT("/system/mail-config/content-type", handler.AdminMailContentTypeUpdate)
			secured.POST("/system/mail-config/test", handler.AdminMailConfigTest)
			secured.GET("/system/mail-logs", handler.AdminMailLogList)
			secured.GET("/system/mail-logs/:id", handler.AdminMailLogDetail)
			secured.GET("/system/update/status", handler.AdminOnlineUpdateStatus)
			secured.GET("/system/update/history", handler.AdminOnlineUpdateHistory)
			secured.POST("/system/update/check", handler.AdminOnlineUpdateCheck)
			secured.POST("/system/update/apply", handler.AdminOnlineUpdateApply)
			secured.GET("/system/update/jobs/:id", handler.AdminOnlineUpdateJob)
			secured.GET("/license/dashboard", handler.LicenseDashboard)
			secured.GET("/dashboard/overview", handler.AdminDashboardOverview)
			secured.GET("/dashboard/cards", handler.AdminDashboardCards)
			secured.GET("/dashboard/trend", handler.AdminDashboardTrend)
			secured.GET("/dashboard/license-status", handler.AdminDashboardLicenseStatus)
			secured.GET("/dashboard/payment-methods", handler.AdminDashboardPaymentMethods)
			secured.GET("/dashboard/agent-metrics", handler.AdminDashboardAgentMetrics)
			secured.GET("/dashboard/user-metrics", handler.AdminDashboardUserMetrics)
			secured.GET("/dashboard/app-metrics", handler.AdminDashboardAppMetrics)
			secured.GET("/dashboard/app-ranking", handler.AdminDashboardAppRanking)
			secured.GET("/dashboard/agent-ranking", handler.AdminDashboardAgentRanking)
			secured.GET("/dashboard/activities", handler.AdminDashboardActivities)
			secured.GET("/dashboard/quick-entries", handler.AdminDashboardQuickEntries)
			secured.GET("/license/list", handler.LicenseList)
			secured.GET("/license/query-by-user", handler.UserLicenseQuery)
			secured.GET("/license/cards/batches", handler.AdminLicenseCardBatchList)
			secured.POST("/license/cards/batches", handler.AdminLicenseCardBatchCreate)
			secured.PUT("/license/cards/batches/:id/status", handler.AdminLicenseCardBatchToggle)
			secured.DELETE("/license/cards/batches/:id", handler.AdminLicenseCardBatchDelete)
			secured.GET("/license/cards/batches/:id/cards", handler.AdminLicenseCardList)
			secured.GET("/license/cards/batches/:id/export", handler.AdminLicenseCardExport)
			secured.PUT("/license/cards/:id/status", handler.AdminLicenseCardToggle)
			secured.GET("/license/apps", handler.AppList)
			secured.GET("/license/owners", handler.LicenseOwnerOptions)
			secured.GET("/app/list", handler.AppManageList)
			secured.POST("/app/create", handler.AppCreate)
			secured.PUT("/app/:id", handler.AppUpdate)
			secured.PUT("/app/:id/license-required", handler.AppLicenseRequiredUpdate)
			secured.PUT("/app/:id/reset-secret", handler.AppResetSecret)
			secured.DELETE("/app/:id", handler.AppDelete)
			secured.GET("/app/:id/versions", handler.AppVersionList)
			secured.POST("/app/:id/versions", handler.AppVersionCreate)
			secured.PUT("/app/:id/versions/:versionId", handler.AppVersionUpdate)
			secured.DELETE("/app/:id/versions/:versionId", handler.AppVersionDelete)
			secured.POST("/app/:id/versions/:versionId/download-url", handler.AppVersionAdminDownloadURL)
			secured.GET("/plan/list", handler.PlanList)
			secured.POST("/plan/create", handler.PlanCreate)
			secured.PUT("/plan/:id", handler.PlanUpdate)
			secured.PUT("/plan/:id/toggle", handler.PlanToggle)
			secured.DELETE("/plan/:id", handler.PlanDelete)
			secured.GET("/verify-log/list", handler.VerifyLogList)
			secured.DELETE("/verify-log/clear", handler.VerifyLogClear)
			secured.GET("/agent/list", handler.AgentList)
			secured.POST("/agent/create", handler.AgentCreate)
			secured.PUT("/agent/:id", handler.AgentUpdate)
			secured.PUT("/agent/:id/toggle", handler.AgentToggle)
			secured.POST("/agent/:id/recharge", handler.AgentRecharge)
			secured.DELETE("/agent/:id", handler.AgentDelete)
			secured.POST("/agent/:id/impersonate", handler.AdminImpersonateAgent)
			secured.GET("/agent/select-list", handler.AgentSelectList)
			secured.GET("/agent-level/list", handler.AgentLevelList)
			secured.GET("/agent-level/select-list", handler.AgentLevelSelectList)
			secured.POST("/agent-level/create", handler.AgentLevelCreate)
			secured.PUT("/agent-level/:id", handler.AgentLevelUpdate)
			secured.DELETE("/agent-level/:id", handler.AgentLevelDelete)
			secured.GET("/transaction/list", handler.TransactionList)
			secured.GET("/transaction/stats", handler.TransactionStats)
			secured.GET("/quota/list", handler.QuotaList)
			secured.POST("/quota/create", handler.QuotaCreate)
			secured.PUT("/quota/:id", handler.QuotaUpdate)
			secured.DELETE("/quota/:id", handler.QuotaDelete)
			secured.POST("/license/create", handler.LicenseCreate)
			secured.PUT("/license/:id", handler.LicenseUpdate)
			secured.PUT("/license/:id/toggle", handler.LicenseToggle)
			secured.DELETE("/license/:id", handler.LicenseDelete)

			// 反盗版 - 追踪
			secured.GET("/piracy/tracking/stats", handler.PiracyTrackingStats)
			secured.GET("/piracy/tracking/list", handler.PiracyTrackingList)
			secured.GET("/piracy/tracking/:id", handler.PiracyTrackingDetail)
			secured.POST("/piracy/tracking/create", handler.PiracyTrackingCreate)
			secured.PUT("/piracy/tracking/:id/block", handler.PiracyTrackingBlock)
			secured.PUT("/piracy/tracking/:id/unblock", handler.PiracyTrackingUnblock)
			secured.POST("/piracy/tracking/batch-block", handler.PiracyTrackingBatchBlock)
			// 反盗版 - 告警
			secured.GET("/piracy/alert/stats", handler.PiracyAlertStats)
			secured.GET("/piracy/alert/list", handler.PiracyAlertList)
			secured.PUT("/piracy/alert/:id/mark", handler.PiracyAlertMark)
			secured.POST("/piracy/alert/batch-mark", handler.PiracyAlertBatchMark)
			// 反盗版 - 黑名单
			secured.GET("/piracy/blacklist/list", handler.PiracyBlacklistList)
			secured.POST("/piracy/blacklist/create", handler.PiracyBlacklistCreate)
			secured.PUT("/piracy/blacklist/:id", handler.PiracyBlacklistUpdate)
			secured.DELETE("/piracy/blacklist/:id", handler.PiracyBlacklistDelete)
			secured.POST("/piracy/blacklist/batch-delete", handler.PiracyBlacklistBatchDelete)
			// 反盗版 - 数据报表
			secured.GET("/piracy/report/overview", handler.ReportOverview)
			// 角色管理
			secured.GET("/role/list", handler.RoleList)
			secured.POST("/role/create", handler.RoleCreate)
			secured.PUT("/role/:id", handler.RoleUpdate)
			secured.DELETE("/role/:id", handler.RoleDelete)
			secured.GET("/role/:id/menus", handler.RoleMenus)
			secured.PUT("/role/:id/menus", handler.RoleUpdateMenus)
			// 菜单管理
			secured.GET("/menu/list", handler.MenuManageList)
			secured.POST("/menu/create", handler.MenuManageCreate)
			secured.PUT("/menu/:id", handler.MenuManageUpdate)
			secured.DELETE("/menu/:id", handler.MenuManageDelete)
		}
	}

	// 快瞳 / 腾讯云增强人脸扫码拍照落地页（无需登录，token 即凭证）
	r.GET("/realname-face", handler.RealnameFacePage)

	// 静态文件服务：优先使用部署目录下的真实前端产物（AUTO_PRO_FRONTEND_DIR
	// 或数据目录里的 frontend/current），不存在时退回内嵌产物。
	// 这样即使支付回跳意外落到后端地址，也能拿到最新页面而不是旧 embed。
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal("failed to load static files:", err)
	}
	embedServer := http.FileServer(http.FS(staticSub))
	frontendDir := config.GetFrontendDir()
	var diskServer http.Handler
	if info, err := os.Stat(filepath.Join(frontendDir, "index.html")); err == nil && !info.IsDir() {
		diskServer = http.FileServer(http.Dir(frontendDir))
		log.Printf("Serving frontend from disk: %s", frontendDir)
	}
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "请求的资源不存在"})
			return
		}

		cleanPath := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")

		if diskServer != nil {
			if cleanPath != "." {
				if info, err := os.Stat(filepath.Join(frontendDir, cleanPath)); err == nil && !info.IsDir() {
					diskServer.ServeHTTP(c.Writer, c.Request)
					return
				}
			}
			c.File(filepath.Join(frontendDir, "index.html"))
			return
		}

		if cleanPath != "." {
			if _, err := fs.Stat(staticSub, cleanPath); err == nil {
				embedServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}

		indexHTML, err := fs.ReadFile(staticSub, "index.html")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "前端入口文件不存在"})
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})

	// 启动后台邮件到期提醒任务
	handler.StartMailReminderWorker()

	// 兜底迁移：补齐购买订单字段，修正历史线上购买流水与价格快照
	go func() {
		cfg, err := config.LoadDBConfig()
		if err != nil {
			return
		}
		db, err := sql.Open("mysql", config.GetDSN(cfg))
		if err != nil {
			return
		}
		defer db.Close()
		if err := handler.EnsureAppVersionsTable(db); err != nil {
			log.Printf("ensure app_versions table failed: %v", err)
		}
		if err := handler.EnsureAppPurchaseLicenseTypesColumn(db); err != nil {
			log.Printf("ensure app purchase license types failed: %v", err)
		}
		if err := handler.EnsurePurchaseOrderUserIDColumn(db); err != nil {
			log.Printf("ensure user_id column failed: %v", err)
		}
		if err := handler.EnsureLicensePurchasePriceSnapshotSchema(db); err != nil {
			log.Printf("ensure license purchase price snapshots failed: %v", err)
		}
		handler.BackfillLicensePurchaseTransactions(db)
	}()

	// 启动
	host := config.GetHost()
	port := config.GetPort()
	log.Printf("Server starting on %s:%s", host, port)
	if err := r.Run(host + ":" + port); err != nil {
		log.Fatal("Server failed:", err)
	}
}
