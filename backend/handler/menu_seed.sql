-- 菜单种子数据：与前端 asyncRoutes 对应
-- 一级菜单
INSERT INTO `menus` (`id`, `parent_id`, `name`, `path`, `component`, `title`, `icon`, `sort`, `keep_alive`) VALUES
(1,   0, 'Dashboard',  '/dashboard',    '/index/index', 'menus.dashboard.title',  'ri:pie-chart-line',       1, 0),
(3,   0, 'License',    '/license',      '/index/index', '授权管理',               'ri:shield-keyhole-line',  2, 0),
(4,   0, 'Agent',      '/agent',        '/index/index', '代理商管理',             'ri:team-line',            3, 0),
(5,   0, 'Piracy',     '/piracy',       '/index/index', '反盗版',                 'ri:shield-cross-line',    4, 0),
(201, 0, 'User',       '/user-manage',  '/system/user', 'menus.system.user',      'ri:user-line',            5, 1),
(209, 0, 'OrderList',  '/order-list',   '/system/payment-orders', 'menus.system.paymentOrders', 'ri:file-list-3-line', 6, 1),
(2,   0, 'System',     '/system',       '/index/index', 'menus.system.title',     'ri:user-3-line',          7, 0),
(6,   0, 'Result',     '/result',       '/index/index', 'menus.result.title',     'ri:checkbox-circle-line', 8, 0),
(7,   0, 'Exception',  '/exception',    '/index/index', 'menus.exception.title',  'ri:error-warning-line',   9, 0),
(8,   0, 'Sdk',        '/sdk',          '/index/index', 'SDK 接入',               'ri:code-box-line',        99, 0),
(212, 0, 'PromotionCampaigns', '/promotion-campaigns', '/promotion-campaigns/index', 'menus.promotionCampaigns', 'ri:discount-percent-line', 7, 1);

-- 在线更新
INSERT INTO `menus` (`id`, `parent_id`, `name`, `path`, `component`, `title`, `icon`, `sort`, `keep_alive`) VALUES
(211, 0, 'OnlineUpdate', '/online-update', '/online-update/index', 'menus.onlineUpdate', 'ri:download-cloud-2-line', 7, 1);

-- Dashboard 子菜单
INSERT INTO `menus` (`id`, `parent_id`, `name`, `path`, `component`, `title`, `icon`, `sort`, `keep_alive`, `fixed_tab`) VALUES
(101, 1, 'Console', 'console', '/dashboard/console', 'menus.dashboard.console', '', 1, 0, 1);

-- System 子菜单
INSERT INTO `menus` (`id`, `parent_id`, `name`, `path`, `component`, `title`, `icon`, `sort`, `keep_alive`, `is_hide`, `is_hide_tab`) VALUES
(202, 2, 'Role',       'role',        '/system/role',        'menus.system.role',       '', 2, 1, 0, 0),
(203, 2, 'UserCenter', 'user-center', '/system/user-center', 'menus.system.userCenter', '', 3, 1, 1, 1),
(204, 2, 'Menus',      'menu',        '/system/menu',        'menus.system.menu',       '', 4, 1, 0, 0),
(205, 2, 'SystemConfig', 'config',    '/system/config',      '系统配置',                  'ri:settings-3-line', 5, 1, 0, 0),
(208, 2, 'EpayConfig', 'epay-config', '/system/epay-config', '易支付配置',                'ri:bank-card-line', 6, 1, 0, 0),
(206, 2, 'MailConfig', 'mail-config', '/system/mail-config', '邮件配置',                  'ri:mail-settings-line', 8, 1, 0, 0),
(207, 2, 'MailLogs',   'mail-logs',   '/system/mail-logs',   '邮件日志',                  'ri:mail-check-line',    9, 1, 0, 0);

-- License 子菜单
INSERT INTO `menus` (`id`, `parent_id`, `name`, `path`, `component`, `title`, `icon`, `sort`, `keep_alive`) VALUES
(301, 3, 'LicenseDashboard', 'dashboard', '/license/dashboard', '授权概览', 'ri:dashboard-line',    1, 0),
(302, 3, 'LicenseList',      'list',      '/license/list',      '授权列表', 'ri:file-list-3-line',  2, 1),
(303, 3, 'LicenseApps',      'apps',      '/license/apps',      '应用管理', 'ri:apps-line',         3, 1),
(304, 3, 'LicenseLogs',      'logs',      '/license/logs',      '验证日志', 'ri:file-text-line',    4, 1),
(306, 3, 'LicenseCards',     'cards',     '/license/cards',     '卡密管理', 'ri:coupon-3-line',     5, 1);

INSERT INTO `menus` (`id`, `parent_id`, `name`, `path`, `component`, `title`, `icon`, `sort`, `keep_alive`, `is_hide`) VALUES
(305, 3, 'AppVersions', 'apps/:id/versions', '/license/app-versions', '版本管理', 'ri:git-branch-line', 99, 0, 1);

-- Agent 子菜单
INSERT INTO `menus` (`id`, `parent_id`, `name`, `path`, `component`, `title`, `icon`, `sort`, `keep_alive`) VALUES
(401, 4, 'AgentList',     'list',     '/agent/list',     '代理商列表', 'ri:user-star-line',         1, 1),
(402, 4, 'AgentLevel',    'level',    '/agent/level',    '等级管理',   'ri:vip-crown-line',        2, 1),
(405, 4, 'AgentUpgrade',  'upgrade',  '/agent/upgrade',  '升级审计',   'ri:user-shared-line',       3, 1),
(403, 4, 'AgentRecharge', 'recharge', '/agent/recharge', '财务流水',   'ri:money-cny-circle-line',  4, 1),
(404, 4, 'AgentQuota',    'quota',    '/agent/quota',    '开码配额',   'ri:key-2-line',             5, 1);

-- Piracy 子菜单
INSERT INTO `menus` (`id`, `parent_id`, `name`, `path`, `component`, `title`, `icon`, `sort`, `keep_alive`) VALUES
(501, 5, 'PiracyTracking',  'tracking',  '/piracy/tracking',  '盗版追踪', 'ri:spy-line',            1, 1),
(502, 5, 'PiracyBlacklist', 'blacklist', '/piracy/blacklist', '黑名单管理', 'ri:forbid-line',       2, 1),
(503, 5, 'PiracyAlerts',    'alerts',    '/piracy/alerts',    '告警中心', 'ri:alarm-warning-line',   3, 0),
(504, 5, 'PiracyReports',   'reports',   '/piracy/reports',   '数据报表', 'ri:bar-chart-box-line',   4, 0);

-- Result 子菜单
INSERT INTO `menus` (`id`, `parent_id`, `name`, `path`, `component`, `title`, `icon`, `sort`, `keep_alive`) VALUES
(601, 6, 'ResultSuccess', 'success', '/result/success', 'menus.result.success', 'ri:checkbox-circle-line', 1, 1),
(602, 6, 'ResultFail',    'fail',    '/result/fail',    'menus.result.fail',    'ri:close-circle-line',    2, 1);

-- Exception 子菜单
INSERT INTO `menus` (`id`, `parent_id`, `name`, `path`, `component`, `title`, `icon`, `sort`, `keep_alive`, `is_hide_tab`, `is_full_page`) VALUES
(701, 7, 'Exception403', '403', '/exception/403', 'menus.exception.forbidden',   '', 1, 1, 1, 1),
(702, 7, 'Exception404', '404', '/exception/404', 'menus.exception.notFound',    '', 2, 1, 1, 1),
(703, 7, 'Exception500', '500', '/exception/500', 'menus.exception.serverError', '', 3, 1, 1, 1);

-- SDK 子菜单
INSERT INTO `menus` (`id`, `parent_id`, `name`, `path`, `component`, `title`, `icon`, `sort`, `keep_alive`) VALUES
(801, 8, 'SdkIndex',     'index',         '/sdk/index',         'SDK 示例',                 'ri:code-s-slash-line', 1, 1),
(802, 8, 'DeveloperDoc', 'developer-doc', '/sdk/developer-doc', 'menus.system.developerDoc', 'ri:file-code-line',    2, 1);
