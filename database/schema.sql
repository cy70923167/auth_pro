-- ============================================================
-- 授权管理系统 数据库设计
-- MySQL 8.0+
-- 每个字段均含 COMMENT 备注
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- -----------------------------------------------------------
-- 1. admins 管理员表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `admins`;
CREATE TABLE `admins` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `username`      VARCHAR(50)  NOT NULL COMMENT '登录用户名',
  `password_hash` VARCHAR(255) NOT NULL COMMENT '密码哈希(bcrypt)',
  `nickname`      VARCHAR(50)  DEFAULT '' COMMENT '显示昵称',
  `avatar`        VARCHAR(255) DEFAULT '' COMMENT '头像URL',
  `email`         VARCHAR(100) DEFAULT '' COMMENT '邮箱',
  `role_id`       BIGINT UNSIGNED DEFAULT NULL COMMENT '关联角色ID',
  `enabled`       TINYINT(1)   DEFAULT 1 COMMENT '是否启用: 1启用 0禁用',
  `last_login_at` DATETIME     DEFAULT NULL COMMENT '最后登录时间',
  `last_login_ip` VARCHAR(45)  DEFAULT '' COMMENT '最后登录IP',
  `created_at`    DATETIME     DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`    DATETIME     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员表';

-- -----------------------------------------------------------
-- 2. roles 角色表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `roles`;
CREATE TABLE `roles` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `role_name`   VARCHAR(50)  NOT NULL COMMENT '角色名称',
  `role_code`   VARCHAR(50)  NOT NULL COMMENT '角色编码(自动生成 ROLE_XXXXXX)',
  `description` VARCHAR(255) DEFAULT '' COMMENT '角色描述',
  `enabled`     TINYINT(1)   DEFAULT 1 COMMENT '是否启用: 1启用 0禁用',
  `created_at`  DATETIME     DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`  DATETIME     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_code` (`role_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色表';

-- -----------------------------------------------------------
-- 3. apps 应用表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `apps`;
CREATE TABLE `apps` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `app_name`    VARCHAR(100) NOT NULL COMMENT '应用名称',
  `app_key`     VARCHAR(64)  NOT NULL COMMENT '应用标识(对外公开)',
  `app_secret`  VARCHAR(128) NOT NULL COMMENT '应用密钥(验证签名用)',
  `description` VARCHAR(255) DEFAULT '' COMMENT '应用描述',
  `icon`        VARCHAR(100) DEFAULT '' COMMENT '图标标识(Iconify图标名)',
  `price`       DECIMAL(10,2) DEFAULT 0.00 COMMENT '年基础价格(元)',
  `enabled`     TINYINT(1)   DEFAULT 1 COMMENT '是否启用: 1启用 0禁用',
  `license_required` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否要求授权验证: 1要求 0免授权',
  `purchase_license_type_mask` TINYINT UNSIGNED NOT NULL DEFAULT 15 COMMENT '允许用户和代理购买的授权类型位掩码',
  `created_at`  DATETIME     DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`  DATETIME     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_app_key` (`app_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='应用表';

-- -----------------------------------------------------------
-- 3.1 app_versions 应用版本发布记录
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `app_versions`;
CREATE TABLE `app_versions` (
  `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `app_id`          BIGINT UNSIGNED NOT NULL COMMENT '所属应用ID',
  `from_version`    VARCHAR(50) NOT NULL DEFAULT '' COMMENT '兼容旧数据，已停用',
  `from_version_norm` VARCHAR(100) DEFAULT NULL COMMENT '兼容旧数据，已停用',
  `version`         VARCHAR(50)  NOT NULL COMMENT '版本号',
  `version_norm`    VARCHAR(100) NOT NULL COMMENT '规范化版本号',
  `title`           VARCHAR(200) NOT NULL COMMENT '更新标题',
  `changelog`       MEDIUMTEXT NOT NULL COMMENT '更新日志',
  `update_sql`      LONGTEXT NOT NULL COMMENT '客户端更新SQL',
  `package_name`    VARCHAR(255) NOT NULL DEFAULT '' COMMENT '更新包原始文件名',
  `package_path`    VARCHAR(512) NOT NULL DEFAULT '' COMMENT '本地更新包相对路径',
  `download_url`    VARCHAR(2048) NOT NULL DEFAULT '' COMMENT '外部更新包下载URL',
  `file_size_bytes` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '文件大小(字节)',
  `file_md5`        CHAR(32) NOT NULL DEFAULT '' COMMENT '文件MD5',
  `force_update`    TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否强制更新',
  `min_version`     VARCHAR(50) NOT NULL DEFAULT '' COMMENT '低于该版本时强制更新',
  `revision`        BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '并发编辑修订号',
  `published_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '发布时间',
  `created_at`      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_app_version` (`app_id`, `version`),
  UNIQUE KEY `uk_app_version_norm` (`app_id`, `version_norm`),
  KEY `idx_app_published` (`app_id`, `published_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='应用版本发布记录';

-- -----------------------------------------------------------
-- 4. role_apps 角色-应用权限关联表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `role_apps`;
CREATE TABLE `role_apps` (
  `id`      BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `role_id` BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
  `app_id`  BIGINT UNSIGNED NOT NULL COMMENT '应用ID',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_app` (`role_id`, `app_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色应用权限关联表';

-- -----------------------------------------------------------
-- 5. agent_levels 代理商等级表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `agent_levels`;
CREATE TABLE `agent_levels` (
  `id`                   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `code`                 VARCHAR(50) NOT NULL COMMENT '等级编码',
  `name`                 VARCHAR(50) NOT NULL COMMENT '等级名称',
  `discount`             DECIMAL(3,1) NOT NULL DEFAULT 9.0 COMMENT '折扣(1-10)',
  `self_service_enabled` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否允许用户自助开通',
  `upgrade_price`        DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '用户自助开通价格',
  `opening_bonus`        DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '自助开通赠送余额',
  `benefits`             TEXT COMMENT '等级权益说明',
  `sort`                 INT DEFAULT 0 COMMENT '排序',
  `enabled`              TINYINT(1) DEFAULT 1 COMMENT '是否启用',
  `remark`               VARCHAR(255) DEFAULT '' COMMENT '备注',
  `created_at`           DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`           DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_agent_level_code` (`code`),
  KEY `idx_agent_level_self_service` (`self_service_enabled`, `enabled`, `sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理商等级表';

INSERT INTO `agent_levels` (`code`, `name`, `discount`, `self_service_enabled`, `upgrade_price`, `sort`, `enabled`, `remark`) VALUES
('gold', '金牌代理', 7.0, 0, 0.00, 1, 1, '默认金牌等级'),
('silver', '银牌代理', 8.0, 0, 0.00, 2, 1, '默认银牌等级'),
('bronze', '铜牌代理', 9.0, 0, 0.00, 3, 1, '默认铜牌等级');

-- -----------------------------------------------------------
-- 6. agents 代理商表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `agents`;
CREATE TABLE `agents` (
  `id`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `email`            VARCHAR(100) NOT NULL COMMENT '登录邮箱',
  `password_hash`    VARCHAR(255) NOT NULL COMMENT '密码哈希(bcrypt)',
  `name`             VARCHAR(50)  DEFAULT '' COMMENT '代理商名称/公司名',
  `contact`          VARCHAR(100) DEFAULT '' COMMENT '联系方式(手机/微信)',
  `level`            VARCHAR(50) DEFAULT 'bronze' COMMENT '等级编码',
  `discount`         DECIMAL(3,1) DEFAULT 9.0 COMMENT '折扣(1-10)',
  `balance`          DECIMAL(12,2) DEFAULT 0.00 COMMENT '账户余额(元)',
  `source`           ENUM('admin','user_upgrade') NOT NULL DEFAULT 'admin' COMMENT '账户来源',
  `original_user_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '升级前用户ID',
  `converted_at`     DATETIME DEFAULT NULL COMMENT '账户转换完成时间',
  `remark`           VARCHAR(255) DEFAULT '' COMMENT '备注',
  `enabled`          TINYINT(1)   DEFAULT 1 COMMENT '是否启用: 1启用 0禁用',
  `last_login_at`    DATETIME     DEFAULT NULL COMMENT '最后登录时间',
  `last_login_ip`    VARCHAR(45)  DEFAULT '' COMMENT '最后登录IP',
  `created_at`       DATETIME     DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`       DATETIME     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_email` (`email`),
  UNIQUE KEY `uk_agent_original_user` (`original_user_id`),
  KEY `idx_agent_source` (`source`, `created_at`),
  KEY `idx_agent_level` (`level`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理商表';

-- -----------------------------------------------------------
-- 7. agent_quotas 代理商开码配额表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `agent_quotas`;
CREATE TABLE `agent_quotas` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `agent_id`   BIGINT UNSIGNED NOT NULL COMMENT '代理商ID',
  `app_id`     BIGINT UNSIGNED NOT NULL COMMENT '应用ID',
  `total`      INT UNSIGNED DEFAULT 0 COMMENT '总配额数',
  `used`       INT UNSIGNED DEFAULT 0 COMMENT '已使用配额数',
  `price`      DECIMAL(10,2) DEFAULT 0.00 COMMENT '单次开码价格(覆盖应用默认价)',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_agent_app` (`agent_id`, `app_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理商开码配额表';

-- -----------------------------------------------------------
-- 7. users 终端用户表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `users`;
CREATE TABLE `users` (
  `id`                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `email`              VARCHAR(100) NOT NULL COMMENT '登录邮箱',
  `phone`              VARCHAR(20)  DEFAULT NULL COMMENT '手机号',
  `password_hash`      VARCHAR(255) NOT NULL COMMENT '密码哈希(bcrypt)',
  `nickname`           VARCHAR(50)  DEFAULT '' COMMENT '显示昵称',
  `balance`            DECIMAL(12,2) DEFAULT 0.00 COMMENT '账户余额(元)',
  `account_status`     ENUM('active','converted') NOT NULL DEFAULT 'active' COMMENT '账户主体状态',
  `converted_agent_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '转换后的代理ID',
  `converted_at`       DATETIME DEFAULT NULL COMMENT '账户转换时间',
  `enabled`            TINYINT(1)   DEFAULT 1 COMMENT '是否启用: 1启用 0禁用',
  `last_login_at`      DATETIME     DEFAULT NULL COMMENT '最后登录时间',
  `last_login_ip`      VARCHAR(45)  DEFAULT '' COMMENT '最后登录IP',
  `created_at`         DATETIME     DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`         DATETIME     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_email` (`email`),
  UNIQUE KEY `uk_phone` (`phone`),
  UNIQUE KEY `uk_user_converted_agent` (`converted_agent_id`),
  KEY `idx_user_account_status` (`account_status`, `enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='终端用户表';

-- -----------------------------------------------------------
-- 7.1 user_password_resets 用户密码重置令牌表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `user_password_resets`;
CREATE TABLE `user_password_resets` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `user_id`    BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  `email`      VARCHAR(100) NOT NULL COMMENT '申请邮箱',
  `token_hash` CHAR(64) NOT NULL COMMENT '令牌SHA256',
  `expires_at` DATETIME NOT NULL COMMENT '过期时间',
  `used_at`    DATETIME DEFAULT NULL COMMENT '使用时间',
  `created_ip` VARCHAR(45) NOT NULL DEFAULT '' COMMENT '申请IP',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_token_hash` (`token_hash`),
  KEY `idx_email_created` (`email`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户密码重置令牌表';

-- -----------------------------------------------------------
-- 7.2 user_email_codes 用户邮箱验证码表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `user_email_codes`;
CREATE TABLE `user_email_codes` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `email`      VARCHAR(100) NOT NULL COMMENT '目标邮箱',
  `scene`      VARCHAR(20) NOT NULL DEFAULT 'register' COMMENT '场景(register)',
  `code_hash`  CHAR(64) NOT NULL COMMENT '验证码SHA256',
  `attempts`   INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '失败尝试次数',
  `expires_at` DATETIME NOT NULL COMMENT '过期时间',
  `used_at`    DATETIME DEFAULT NULL COMMENT '使用时间',
  `created_ip` VARCHAR(45) NOT NULL DEFAULT '' COMMENT '申请IP',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_email_scene` (`email`, `scene`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户邮箱验证码表';

-- -----------------------------------------------------------
-- 7.3 license_plans 授权套餐表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `license_plans`;
CREATE TABLE `license_plans` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `app_id`        BIGINT UNSIGNED NOT NULL COMMENT '应用ID',
  `name`          VARCHAR(100) NOT NULL COMMENT '套餐名称',
  `license_type`  VARCHAR(20) NOT NULL DEFAULT '' COMMENT '适用授权方式：空=全部, domain/wildcard/ip/key',
  `duration_days` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '授权天数，0表示永久',
  `price`         DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '套餐价格',
  `max_sites`     INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '密钥授权最大站点数, 0表示不限',
  `sort`          INT DEFAULT 0 COMMENT '排序',
  `enabled`       TINYINT(1) DEFAULT 1 COMMENT '是否启用',
  `remark`        VARCHAR(255) DEFAULT '' COMMENT '备注',
  `created_at`    DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`    DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_app_enabled_sort` (`app_id`, `enabled`, `sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='授权套餐表';

-- -----------------------------------------------------------
-- 7.4 promotion_campaigns 促销活动表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `promotion_campaigns`;
CREATE TABLE `promotion_campaigns` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `app_id`     BIGINT UNSIGNED NOT NULL COMMENT '应用ID',
  `name`       VARCHAR(100) NOT NULL COMMENT '活动名称',
  `audience`   ENUM('user','agent','all') NOT NULL COMMENT '适用对象: user用户/agent代理/all全部',
  `starts_at`  DATETIME NOT NULL COMMENT '开始时间（含）',
  `ends_at`    DATETIME NOT NULL COMMENT '结束时间（不含）',
  `enabled`    TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否启用',
  `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建管理员ID',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_promotion_app_time` (`app_id`, `enabled`, `starts_at`, `ends_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='促销活动表';

DROP TABLE IF EXISTS `promotion_campaign_plans`;
CREATE TABLE `promotion_campaign_plans` (
  `campaign_id`     BIGINT UNSIGNED NOT NULL COMMENT '活动ID',
  `plan_id`         BIGINT UNSIGNED NOT NULL COMMENT '套餐ID',
  `rule_type`       ENUM('discount','reduction','fixed_price') NOT NULL DEFAULT 'fixed_price' COMMENT '优惠方式',
  `rule_value`      DECIMAL(12,4) NOT NULL DEFAULT 0 COMMENT '折扣值或金额',
  `promotion_price` DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '兼容旧版固定活动价',
  `created_at`      DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`campaign_id`, `plan_id`),
  KEY `idx_promotion_plan` (`plan_id`, `campaign_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='活动套餐规则表';

-- -----------------------------------------------------------
-- 8. licenses 授权表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `licenses`;
CREATE TABLE `licenses` (
  `id`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `license_no`       VARCHAR(64)  NOT NULL COMMENT '授权编号(对外展示唯一标识)',
  `app_id`           BIGINT UNSIGNED NOT NULL COMMENT '所属应用ID',
  `plan_id`          BIGINT UNSIGNED DEFAULT NULL COMMENT '购买套餐ID',
  `original_price`   DECIMAL(12,2) DEFAULT NULL COMMENT '套餐原价快照',
  `type`             ENUM('domain','wildcard','ip','key') NOT NULL COMMENT '授权类型: domain单域名/wildcard泛域名/ip IP地址/key密钥',
  `status`           ENUM('active','expired','revoked') DEFAULT 'active' COMMENT '状态: active有效/expired已过期/revoked已吊销',
  `source`           ENUM('admin','agent','user_purchase','card') NOT NULL COMMENT '来源: admin后台开通/agent代理开码/user_purchase用户购买/card卡密兑换',
  `owner_type`       ENUM('user','agent') NOT NULL COMMENT '持有者类型: user终端用户/agent代理商',
  `owner_id`         BIGINT UNSIGNED NOT NULL COMMENT '持有者ID(对应user或agent的id)',
  `issued_by`        BIGINT UNSIGNED DEFAULT NULL COMMENT '开通操作者ID(admin_id或agent_id)',
  `duration_days`    INT UNSIGNED NOT NULL COMMENT '授权时长(天), 0表示永久',
  `started_at`       DATETIME NOT NULL COMMENT '生效时间',
  `expired_at`       DATETIME DEFAULT NULL COMMENT '到期时间(永久授权为NULL)',
  `license_key`      VARCHAR(255) DEFAULT '' COMMENT '密钥(type=key时生成的授权key)',
  `max_domains`      INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '密钥授权最大站点数快照, 0表示不限',
  `remark`           VARCHAR(255) DEFAULT '' COMMENT '备注',
  `created_at`       DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`       DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_license_no` (`license_no`),
  KEY `idx_app` (`app_id`),
  KEY `idx_owner` (`owner_type`, `owner_id`),
  KEY `idx_status_expired` (`status`, `expired_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='授权表';

-- -----------------------------------------------------------
-- 9. license_domains 授权绑定域名/IP表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `license_domains`;
CREATE TABLE `license_domains` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `license_id`    BIGINT UNSIGNED NOT NULL COMMENT '授权ID',
  `target_type`   VARCHAR(10) NOT NULL DEFAULT 'domain' COMMENT '绑定目标类型: domain/ip',
  `domain`        VARCHAR(255) NOT NULL COMMENT '规范化后的绑定域名或IP地址',
  `is_wildcard`   TINYINT(1) DEFAULT 0 COMMENT '是否泛域名: 1是(*.xxx.com) 0否',
  `server_ip`     VARCHAR(45) NOT NULL DEFAULT '' COMMENT '最近一次上报的服务器IP',
  `first_seen_at` DATETIME DEFAULT NULL COMMENT '首次绑定时间',
  `last_seen_at`  DATETIME DEFAULT NULL COMMENT '最近验证时间',
  `created_at`    DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_license_target` (`license_id`, `target_type`, `domain`),
  KEY `idx_license` (`license_id`),
  KEY `idx_domain` (`domain`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='授权绑定域名/IP表';

-- -----------------------------------------------------------
-- 9.1 license_card_batches 授权卡密批次表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `license_card_batches`;
CREATE TABLE `license_card_batches` (
  `id`                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `batch_no`           VARCHAR(64) NOT NULL COMMENT '卡密批次号',
  `app_id`             BIGINT UNSIGNED NOT NULL COMMENT '应用ID',
  `plan_id`            BIGINT UNSIGNED NOT NULL COMMENT '套餐ID',
  `app_name_snapshot`  VARCHAR(100) NOT NULL COMMENT '应用名称快照',
  `plan_name_snapshot` VARCHAR(100) NOT NULL COMMENT '套餐名称快照',
  `duration_days`      INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '套餐时长快照',
  `max_sites_snapshot` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '密钥授权最大站点数快照, 0表示不限',
  `price_snapshot`     DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '套餐价格快照',
  `license_type`       ENUM('domain','wildcard','ip','key') NOT NULL COMMENT '兑换授权类型',
  `total_count`        INT UNSIGNED NOT NULL COMMENT '生成数量',
  `status`             ENUM('active','disabled') NOT NULL DEFAULT 'active' COMMENT '批次状态',
  `remark`             VARCHAR(255) DEFAULT '' COMMENT '备注',
  `created_by`         BIGINT UNSIGNED DEFAULT NULL COMMENT '创建管理员ID',
  `created_at`         DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`         DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_card_batch_no` (`batch_no`),
  KEY `idx_card_batch_app` (`app_id`, `plan_id`),
  KEY `idx_card_batch_status` (`status`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='授权卡密批次表';

-- -----------------------------------------------------------
-- 9.2 license_cards 授权卡密库存表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `license_cards`;
CREATE TABLE `license_cards` (
  `id`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `batch_id`         BIGINT UNSIGNED NOT NULL COMMENT '批次ID',
  `card_code`        VARCHAR(64) NOT NULL COMMENT '完整卡密',
  `card_suffix`      VARCHAR(8) NOT NULL COMMENT '卡密尾号',
  `status`           ENUM('unused','redeemed','disabled') NOT NULL DEFAULT 'unused' COMMENT '卡密状态',
  `redeemed_by_type` ENUM('user','agent') DEFAULT NULL COMMENT '兑换主体类型',
  `redeemed_by_id`   BIGINT UNSIGNED DEFAULT NULL COMMENT '兑换主体ID',
  `license_id`       BIGINT UNSIGNED DEFAULT NULL COMMENT '生成授权ID',
  `redeemed_at`      DATETIME DEFAULT NULL COMMENT '兑换时间',
  `created_at`       DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`       DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_license_card_code` (`card_code`),
  UNIQUE KEY `uk_license_card_license` (`license_id`),
  KEY `idx_license_card_batch` (`batch_id`, `status`),
  KEY `idx_license_card_redeemer` (`redeemed_by_type`, `redeemed_by_id`, `redeemed_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='授权卡密库存表';

-- -----------------------------------------------------------
-- 10. verify_logs 验证日志表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `verify_logs`;
CREATE TABLE `verify_logs` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `license_id`  BIGINT UNSIGNED DEFAULT NULL COMMENT '匹配到的授权ID(未匹配为NULL)',
  `app_id`      BIGINT UNSIGNED NOT NULL COMMENT '请求验证的应用ID',
  `domain`      VARCHAR(255) DEFAULT '' COMMENT '请求验证的域名或IP',
  `server_ip`   VARCHAR(45)  DEFAULT '' COMMENT '被验证服务器的IP(本机IP)',
  `client_ip`   VARCHAR(45)  DEFAULT '' COMMENT '发起验证请求的客户端IP',
  `result`      ENUM('pass','fail','expired','blacklisted') NOT NULL COMMENT '验证结果: pass通过/fail失败/expired已过期/blacklisted黑名单拦截',
  `fail_reason` VARCHAR(255) DEFAULT '' COMMENT '失败原因描述',
  `user_agent`  VARCHAR(500) DEFAULT '' COMMENT '请求User-Agent',
  `created_at`  DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '验证时间',
  PRIMARY KEY (`id`),
  KEY `idx_license` (`license_id`),
  KEY `idx_app_time` (`app_id`, `created_at`),
  KEY `idx_domain` (`domain`),
  KEY `idx_server_ip` (`server_ip`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='验证日志表';

-- -----------------------------------------------------------
-- 11. piracy_records 盗版追踪记录表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `piracy_records`;
CREATE TABLE `piracy_records` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `app_id`     BIGINT UNSIGNED NOT NULL COMMENT '关联应用ID',
  `domain`     VARCHAR(255) NOT NULL COMMENT '盗版域名或IP',
  `server_ip`  VARCHAR(45)  DEFAULT '' COMMENT '盗版服务器IP',
  `status`     ENUM('discovered','blocked') DEFAULT 'discovered' COMMENT '状态: discovered已发现/blocked已拉黑',
  `hit_count`  INT UNSIGNED DEFAULT 1 COMMENT '累计命中次数',
  `first_seen` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '首次发现时间',
  `last_seen`  DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '最近一次命中时间',
  `evidence`   TEXT COMMENT '证据快照(JSON格式, 含请求信息等)',
  `remark`     VARCHAR(255) DEFAULT '' COMMENT '备注',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_app_domain` (`app_id`, `domain`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='盗版追踪记录表';

-- -----------------------------------------------------------
-- 12. piracy_alerts 盗版告警表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `piracy_alerts`;
CREATE TABLE `piracy_alerts` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `piracy_id`    BIGINT UNSIGNED DEFAULT NULL COMMENT '关联盗版追踪记录ID',
  `app_id`       BIGINT UNSIGNED NOT NULL COMMENT '关联应用ID',
  `alert_type`   VARCHAR(50) NOT NULL COMMENT '告警类型: freq_anomaly频率异常/multi_ip多IP访问/unauthorized未授权域名',
  `severity`     ENUM('low','medium','high') DEFAULT 'medium' COMMENT '严重程度: low低/medium中/high高',
  `status`       ENUM('pending','processed','ignored') DEFAULT 'pending' COMMENT '处理状态: pending待处理/processed已处理/ignored已忽略',
  `domain`       VARCHAR(255) DEFAULT '' COMMENT '触发告警的域名或IP',
  `detail`       TEXT COMMENT '告警详情(JSON格式)',
  `processed_at` DATETIME DEFAULT NULL COMMENT '处理时间',
  `processed_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '处理人(admin_id)',
  `created_at`   DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_status` (`status`, `created_at`),
  KEY `idx_app` (`app_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='盗版告警表';

-- -----------------------------------------------------------
-- 13. piracy_blacklist 黑名单表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `piracy_blacklist`;
CREATE TABLE `piracy_blacklist` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `app_id`     BIGINT UNSIGNED NOT NULL COMMENT '关联应用ID',
  `type`       ENUM('domain','ip') NOT NULL COMMENT '黑名单类型: domain域名/ip IP地址',
  `value`      VARCHAR(255) NOT NULL COMMENT '被拉黑的域名或IP',
  `reason`     VARCHAR(255) DEFAULT '' COMMENT '拉黑原因',
  `piracy_id`  BIGINT UNSIGNED DEFAULT NULL COMMENT '来源盗版追踪记录ID',
  `blocked_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '操作人(admin_id)',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '拉黑时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_app_type_value` (`app_id`, `type`, `value`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='黑名单表';

-- -----------------------------------------------------------
-- 14.1 agent_upgrade_orders 用户自助开通代理订单表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `agent_upgrade_orders`;
CREATE TABLE `agent_upgrade_orders` (
  `id`                     BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `order_no`               VARCHAR(64) NOT NULL COMMENT '升级订单号',
  `user_id`                BIGINT UNSIGNED NOT NULL COMMENT '发起升级的用户ID',
  `level_id`               BIGINT UNSIGNED NOT NULL COMMENT '目标代理等级ID',
  `level_code_snapshot`    VARCHAR(50) NOT NULL COMMENT '等级编码快照',
  `level_name_snapshot`    VARCHAR(50) NOT NULL COMMENT '等级名称快照',
  `discount_snapshot`      DECIMAL(3,1) NOT NULL COMMENT '代理折扣快照',
  `opening_bonus_snapshot` DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '开通赠送余额快照',
  `amount`                 DECIMAL(12,2) NOT NULL COMMENT '应付开通费',
  `paid_amount`            DECIMAL(12,2) DEFAULT NULL COMMENT '实际支付金额',
  `pay_channel`            VARCHAR(30) NOT NULL DEFAULT '' COMMENT '支付渠道(balance/easypay/easypay-v2)',
  `pay_method`             VARCHAR(30) NOT NULL DEFAULT '' COMMENT '支付方式',
  `gateway_trade_no`       VARCHAR(100) DEFAULT NULL COMMENT '支付网关交易号',
  `return_url`             VARCHAR(500) NOT NULL DEFAULT '' COMMENT '支付完成前端返回地址',
  `status`                 ENUM('pending','paid','processing','completed','failed','cancelled') NOT NULL DEFAULT 'pending' COMMENT '订单状态',
  `agent_id`               BIGINT UNSIGNED DEFAULT NULL COMMENT '转换后的代理ID',
  `paid_at`                DATETIME DEFAULT NULL COMMENT '支付完成时间',
  `completed_at`           DATETIME DEFAULT NULL COMMENT '转换完成时间',
  `error_message`          TEXT COMMENT '最近一次失败原因',
  `notify_payload`         TEXT COMMENT '支付回调原始参数',
  `created_at`             DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`             DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_agent_upgrade_order_no` (`order_no`),
  KEY `idx_agent_upgrade_user_status` (`user_id`, `status`, `created_at`),
  UNIQUE KEY `uk_agent_upgrade_gateway_trade` (`gateway_trade_no`),
  KEY `idx_agent_upgrade_status` (`status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户自助开通代理订单表';

-- -----------------------------------------------------------
-- 14.2 account_conversions 用户转代理审计表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `account_conversions`;
CREATE TABLE `account_conversions` (
  `id`                     BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `conversion_no`          VARCHAR(64) NOT NULL COMMENT '转换流水号',
  `upgrade_order_id`       BIGINT UNSIGNED NOT NULL COMMENT '升级订单ID',
  `user_id`                BIGINT UNSIGNED NOT NULL COMMENT '原用户ID',
  `agent_id`               BIGINT UNSIGNED DEFAULT NULL COMMENT '新代理ID',
  `level_id`               BIGINT UNSIGNED NOT NULL COMMENT '代理等级ID',
  `status`                 ENUM('processing','completed','failed') NOT NULL DEFAULT 'processing' COMMENT '转换状态',
  `opening_fee`            DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '开通费',
  `transferred_balance`    DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '迁移余额',
  `opening_bonus`          DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '开通赠送余额',
  `migrated_license_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '迁移授权数',
  `source_snapshot`        JSON DEFAULT NULL COMMENT '原用户关键资料快照',
  `result_snapshot`        JSON DEFAULT NULL COMMENT '转换结果快照',
  `error_message`          TEXT COMMENT '失败原因',
  `started_at`             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '开始时间',
  `completed_at`           DATETIME DEFAULT NULL COMMENT '完成时间',
  `created_at`             DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`             DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_account_conversion_no` (`conversion_no`),
  UNIQUE KEY `uk_account_conversion_order` (`upgrade_order_id`),
  UNIQUE KEY `uk_account_conversion_user` (`user_id`),
  UNIQUE KEY `uk_account_conversion_agent` (`agent_id`),
  KEY `idx_account_conversion_status` (`status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户转代理审计表';

-- -----------------------------------------------------------
-- 14. transactions 财务流水表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `transactions`;
CREATE TABLE `transactions` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `tx_no`         VARCHAR(64) NOT NULL COMMENT '流水号(唯一)',
  `subject_type`  ENUM('agent','user') NOT NULL COMMENT '主体类型: agent代理商/user终端用户',
  `subject_id`    BIGINT UNSIGNED NOT NULL COMMENT '主体ID(agent_id或user_id)',
  `type`          ENUM('recharge','consume','refund','purchase','transfer','bonus') NOT NULL COMMENT '类型: recharge充值/consume消费扣费/refund退款/purchase用户购买/transfer账户迁移/bonus开通赠送',
  `amount`        DECIMAL(12,2) NOT NULL COMMENT '金额(正数为入账, 负数为扣款)',
  `balance_after` DECIMAL(12,2) DEFAULT NULL COMMENT '交易后余额',
  `ref_type`      VARCHAR(50)  DEFAULT '' COMMENT '关联对象类型(license/recharge_order等)',
  `ref_id`        BIGINT UNSIGNED DEFAULT NULL COMMENT '关联对象ID',
  `remark`        VARCHAR(255) DEFAULT '' COMMENT '备注说明',
  `created_at`    DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tx_no` (`tx_no`),
  KEY `idx_subject` (`subject_type`, `subject_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='财务流水表';

-- -----------------------------------------------------------
-- 15. recharge_orders 充值订单表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `recharge_orders`;
CREATE TABLE `recharge_orders` (
  `id`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `order_no`         VARCHAR(64)  NOT NULL COMMENT '订单号(唯一)',
  `subject_type`     ENUM('agent','user','test') NOT NULL DEFAULT 'agent' COMMENT '充值主体类型: agent代理商/user终端用户/test支付测试',
  `subject_id`       BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '充值主体ID(agent_id或user_id)',
  `agent_id`         BIGINT UNSIGNED DEFAULT NULL COMMENT '充值代理商ID',
  `user_id`          BIGINT UNSIGNED DEFAULT NULL COMMENT '充值用户ID',
  `amount`           DECIMAL(12,2) NOT NULL COMMENT '充值金额(元)',
  `paid_amount`      DECIMAL(12,2) DEFAULT NULL COMMENT '实际支付金额(元)',
  `pay_channel`      VARCHAR(30)  DEFAULT '' COMMENT '支付渠道: easypay/manual',
  `pay_method`       VARCHAR(30)  DEFAULT '' COMMENT '支付方式: alipay/wxpay/qqpay/bank/manual',
  `gateway_trade_no` VARCHAR(100) DEFAULT '' COMMENT '支付网关交易号',
  `return_url`       VARCHAR(500) DEFAULT '' COMMENT '支付完成前端返回地址',
  `status`           ENUM('pending','paid','failed','cancelled') DEFAULT 'pending' COMMENT '状态: pending待支付/paid已支付/failed失败/cancelled已取消',
  `paid_at`          DATETIME     DEFAULT NULL COMMENT '支付完成时间',
  `approved_by`      BIGINT UNSIGNED DEFAULT NULL COMMENT '审核人(admin_id, 人工充值时)',
  `remark`           VARCHAR(255) DEFAULT '' COMMENT '备注',
  `notify_payload`   TEXT COMMENT '支付回调原始参数',
  `created_at`       DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`       DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_no` (`order_no`),
  KEY `idx_subject` (`subject_type`, `subject_id`, `status`),
  KEY `idx_agent` (`agent_id`, `status`),
  KEY `idx_user` (`user_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='充值订单表';

-- -----------------------------------------------------------
-- 15.1 license_purchase_orders 授权购买订单表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `license_purchase_orders`;
CREATE TABLE `license_purchase_orders` (
  `id`                       BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `order_no`                 VARCHAR(64) NOT NULL COMMENT '订单号',
  `agent_id`                 BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '付款代理商ID，用户自购为0',
  `user_id`                  BIGINT UNSIGNED DEFAULT NULL COMMENT '归属用户ID',
  `app_id`                   BIGINT UNSIGNED NOT NULL COMMENT '应用ID',
  `plan_id`                  BIGINT UNSIGNED NOT NULL COMMENT '套餐ID',
  `owner_type`               ENUM('agent','user') NOT NULL DEFAULT 'agent' COMMENT '授权归属类型',
  `owner_id`                 BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '授权归属ID',
  `type`                     VARCHAR(20) NOT NULL DEFAULT 'domain' COMMENT '授权类型',
  `target`                   VARCHAR(255) DEFAULT '' COMMENT '授权目标',
  `amount`                   DECIMAL(12,2) NOT NULL COMMENT '实际应付金额',
  `original_amount`          DECIMAL(12,2) DEFAULT NULL COMMENT '套餐原价快照',
  `base_amount`              DECIMAL(12,2) DEFAULT NULL COMMENT '促销前基础成交价快照',
  `discount_amount`          DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '活动优惠金额快照',
  `promotion_id`             BIGINT UNSIGNED DEFAULT NULL COMMENT '活动ID快照',
  `promotion_name`           VARCHAR(100) NOT NULL DEFAULT '' COMMENT '活动名称快照',
  `promotion_rule_snapshot`  JSON DEFAULT NULL COMMENT '活动规则快照',
  `pricing_snapshot`         JSON DEFAULT NULL COMMENT '完整定价快照',
  `app_name_snapshot`        VARCHAR(100) NOT NULL DEFAULT '' COMMENT '应用名称快照',
  `plan_name_snapshot`       VARCHAR(100) NOT NULL DEFAULT '' COMMENT '套餐名称快照',
  `duration_days_snapshot`   INT UNSIGNED DEFAULT NULL COMMENT '套餐时长快照',
  `max_sites_snapshot`       INT UNSIGNED DEFAULT NULL COMMENT '密钥授权最大站点数快照, 0表示不限',
  `paid_amount`              DECIMAL(12,2) DEFAULT NULL COMMENT '实际支付金额',
  `pay_channel`              VARCHAR(30) DEFAULT '' COMMENT '支付渠道',
  `pay_method`               VARCHAR(30) DEFAULT '' COMMENT '支付方式',
  `gateway_trade_no`         VARCHAR(100) DEFAULT '' COMMENT '支付网关交易号',
  `return_url`               VARCHAR(500) DEFAULT '' COMMENT '支付完成前端返回地址',
  `license_id`               BIGINT UNSIGNED DEFAULT NULL COMMENT '生成的授权ID',
  `license_no`               VARCHAR(64) DEFAULT '' COMMENT '生成的授权编号',
  `status`                   ENUM('pending','paid','failed','cancelled') DEFAULT 'pending' COMMENT '状态',
  `paid_at`                  DATETIME DEFAULT NULL COMMENT '支付完成时间',
  `remark`                   VARCHAR(255) DEFAULT '' COMMENT '备注',
  `notify_payload`           TEXT COMMENT '支付回调原始参数',
  `created_at`               DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`               DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_no` (`order_no`),
  KEY `idx_agent` (`agent_id`, `status`),
  KEY `idx_user` (`user_id`, `status`),
  KEY `idx_promotion` (`promotion_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='授权购买订单表';

-- -----------------------------------------------------------
-- 16. operation_logs 操作日志表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `operation_logs`;
CREATE TABLE `operation_logs` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `operator_type` ENUM('admin','agent','user','system') NOT NULL COMMENT '操作者类型: admin管理员/agent代理商/user用户/system系统自动',
  `operator_id`   BIGINT UNSIGNED DEFAULT NULL COMMENT '操作者ID(system时为NULL)',
  `action`        VARCHAR(100) NOT NULL COMMENT '操作动作(如: create_license/block_domain/login)',
  `target_type`   VARCHAR(50)  DEFAULT '' COMMENT '目标对象类型(license/agent/user等)',
  `target_id`     BIGINT UNSIGNED DEFAULT NULL COMMENT '目标对象ID',
  `detail`        JSON DEFAULT NULL COMMENT '变更详情(JSON, 含before/after)',
  `ip`            VARCHAR(45)  DEFAULT '' COMMENT '操作者IP',
  `created_at`    DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '操作时间',
  PRIMARY KEY (`id`),
  KEY `idx_operator` (`operator_type`, `operator_id`),
  KEY `idx_target` (`target_type`, `target_id`),
  KEY `idx_time` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='操作日志表';

-- -----------------------------------------------------------
-- 17. system_configs 系统配置表
-- -----------------------------------------------------------
DROP TABLE IF EXISTS `system_configs`;
CREATE TABLE `system_configs` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `group`       VARCHAR(50)  NOT NULL COMMENT '配置分组(如: site/payment/notify)',
  `key`         VARCHAR(100) NOT NULL COMMENT '配置键名',
  `value`       LONGTEXT NOT NULL COMMENT '配置值',
  `description` VARCHAR(255) DEFAULT '' COMMENT '配置说明',
  `updated_at`  DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_group_key` (`group`, `key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统配置表';

INSERT INTO `system_configs` (`group`, `key`, `value`, `description`) VALUES
('site', 'site_name', '授权管理系统', '网站名称'),
('site', 'site_subtitle', '专业的软件授权与服务平台', '网站副标题'),
('site', 'site_logo', '', '网站 Logo'),
('site', 'installed_at', DATE_FORMAT(NOW(), '%Y-%m-%d %H:%i:%s'), '系统安装时间'),
('site', 'station_qq', '', '站长 QQ'),
('site', 'icp_number', '', '网站备案号'),
('site', 'domain_license_notice', '', '域名授权网站公告'),
('site', 'registration_enabled', '1', '是否允许普通用户注册'),
('site', 'self_purchase_enabled', '1', '是否允许用户自助购买'),
('payment', 'easypay_enabled', '0', '是否启用易支付'),
('payment', 'easypay_gateway', '', '易支付网关地址'),
('payment', 'easypay_pid', '', '易支付商户 PID'),
('payment', 'easypay_key', '', '易支付商户 Key'),
('payment', 'easypay_default_type', 'alipay', '易支付默认支付方式'),
('payment', 'easypay_pay_types', 'alipay,wxpay,qqpay', '易支付已开启支付方式'),
('payment', 'easypay_notify_url', '', '易支付异步通知地址'),
('payment', 'easypay_return_url', '', '易支付同步跳转地址');

SET FOREIGN_KEY_CHECKS = 1;
