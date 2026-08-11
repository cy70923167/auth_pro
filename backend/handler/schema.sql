SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

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

DROP TABLE IF EXISTS `roles`;
CREATE TABLE `roles` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `role_name`   VARCHAR(50)  NOT NULL COMMENT '角色名称',
  `role_code`   VARCHAR(50)  NOT NULL COMMENT '角色编码',
  `description` VARCHAR(255) DEFAULT '' COMMENT '角色描述',
  `discount`    DECIMAL(3,1) DEFAULT 10.0 COMMENT '折扣(1-10, 10=无折扣)',
  `enabled`     TINYINT(1)   DEFAULT 1 COMMENT '是否启用',
  `created_at`  DATETIME     DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`  DATETIME     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_code` (`role_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色表';

DROP TABLE IF EXISTS `apps`;
CREATE TABLE `apps` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `app_name`    VARCHAR(100) NOT NULL COMMENT '应用名称',
  `app_key`     VARCHAR(64)  NOT NULL COMMENT '应用标识',
  `app_secret`  VARCHAR(128) NOT NULL COMMENT '应用密钥',
  `description` VARCHAR(255) DEFAULT '' COMMENT '应用描述',
  `icon`        VARCHAR(100) DEFAULT '' COMMENT '图标标识',
  `price`       DECIMAL(10,2) DEFAULT 0.00 COMMENT '年基础价格',
  `enabled`     TINYINT(1)   DEFAULT 1 COMMENT '是否启用',
  `license_required` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否要求授权验证: 1要求 0免授权',
  `purchase_license_type_mask` TINYINT UNSIGNED NOT NULL DEFAULT 15 COMMENT '允许用户和代理购买的授权类型位掩码',
  `created_at`  DATETIME     DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`  DATETIME     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_app_key` (`app_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='应用表';

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

DROP TABLE IF EXISTS `role_apps`;
CREATE TABLE `role_apps` (
  `id`      BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `role_id` BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
  `app_id`  BIGINT UNSIGNED NOT NULL COMMENT '应用ID',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_app` (`role_id`, `app_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色应用权限关联表';

DROP TABLE IF EXISTS `agent_levels`;
CREATE TABLE `agent_levels` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `code`       VARCHAR(50) NOT NULL COMMENT '等级编码',
  `name`       VARCHAR(50) NOT NULL COMMENT '等级名称',
  `discount`   DECIMAL(3,1) NOT NULL DEFAULT 9.0 COMMENT '折扣(1-10)',
  `sort`       INT DEFAULT 0 COMMENT '排序',
  `enabled`    TINYINT(1) DEFAULT 1 COMMENT '是否启用',
  `remark`     VARCHAR(255) DEFAULT '' COMMENT '备注',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_agent_level_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理商等级表';

INSERT INTO `agent_levels` (`code`, `name`, `discount`, `sort`, `enabled`, `remark`) VALUES
('gold', '金牌代理', 7.0, 1, 1, '默认金牌等级'),
('silver', '银牌代理', 8.0, 2, 1, '默认银牌等级'),
('bronze', '铜牌代理', 9.0, 3, 1, '默认铜牌等级');

DROP TABLE IF EXISTS `agents`;
CREATE TABLE `agents` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `email`         VARCHAR(100) NOT NULL COMMENT '登录邮箱',
  `password_hash` VARCHAR(255) NOT NULL COMMENT '密码哈希(bcrypt)',
  `name`          VARCHAR(50)  DEFAULT '' COMMENT '代理商名称',
  `contact`       VARCHAR(100) DEFAULT '' COMMENT '联系方式',
  `level`         VARCHAR(50) DEFAULT 'bronze' COMMENT '等级编码',
  `discount`      DECIMAL(3,1) DEFAULT 9.0 COMMENT '折扣(1-10)',
  `balance`       DECIMAL(12,2) DEFAULT 0.00 COMMENT '账户余额',
  `remark`        VARCHAR(255) DEFAULT '' COMMENT '备注',
  `enabled`       TINYINT(1)   DEFAULT 1 COMMENT '是否启用',
  `last_login_at` DATETIME     DEFAULT NULL COMMENT '最后登录时间',
  `last_login_ip` VARCHAR(45)  DEFAULT '' COMMENT '最后登录IP',
  `created_at`    DATETIME     DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`    DATETIME     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理商表';

DROP TABLE IF EXISTS `agent_quotas`;
CREATE TABLE `agent_quotas` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `agent_id`   BIGINT UNSIGNED NOT NULL COMMENT '代理商ID',
  `app_id`     BIGINT UNSIGNED NOT NULL COMMENT '应用ID',
  `total`      INT UNSIGNED DEFAULT 0 COMMENT '总配额数',
  `used`       INT UNSIGNED DEFAULT 0 COMMENT '已使用配额数',
  `price`      DECIMAL(10,2) DEFAULT 0.00 COMMENT '单次开码价格',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_agent_app` (`agent_id`, `app_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理商开码配额表';

DROP TABLE IF EXISTS `users`;
CREATE TABLE `users` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `email`         VARCHAR(100) NOT NULL COMMENT '登录邮箱',
  `phone`         VARCHAR(20)  DEFAULT NULL COMMENT '手机号',
  `password_hash` VARCHAR(255) NOT NULL COMMENT '密码哈希(bcrypt)',
  `nickname`      VARCHAR(50)  DEFAULT '' COMMENT '显示昵称',
  `balance`       DECIMAL(12,2) DEFAULT 0.00 COMMENT '账户余额',
  `enabled`       TINYINT(1)   DEFAULT 1 COMMENT '是否启用',
  `last_login_at` DATETIME     DEFAULT NULL COMMENT '最后登录时间',
  `last_login_ip` VARCHAR(45)  DEFAULT '' COMMENT '最后登录IP',
  `created_at`    DATETIME     DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`    DATETIME     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_email` (`email`),
  UNIQUE KEY `uk_phone` (`phone`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='终端用户表';

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

DROP TABLE IF EXISTS `license_plans`;
CREATE TABLE `license_plans` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `app_id`        BIGINT UNSIGNED NOT NULL COMMENT '应用ID',
  `name`          VARCHAR(100) NOT NULL COMMENT '套餐名称',
  `duration_days` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '授权天数，0表示永久',
  `price`         DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '套餐价格',
  `sort`          INT DEFAULT 0 COMMENT '排序',
  `enabled`       TINYINT(1) DEFAULT 1 COMMENT '是否启用',
  `remark`        VARCHAR(255) DEFAULT '' COMMENT '备注',
  `created_at`    DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`    DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_app_plan_name` (`app_id`, `name`),
  KEY `idx_app_enabled_sort` (`app_id`, `enabled`, `sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='授权套餐表';

DROP TABLE IF EXISTS `licenses`;
CREATE TABLE `licenses` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `license_no`     VARCHAR(64)  NOT NULL COMMENT '授权编号',
  `app_id`         BIGINT UNSIGNED NOT NULL COMMENT '所属应用ID',
  `plan_id`        BIGINT UNSIGNED DEFAULT NULL COMMENT '购买套餐ID',
  `original_price` DECIMAL(12,2) DEFAULT NULL COMMENT '套餐原价快照',
  `type`           ENUM('domain','wildcard','ip','key') NOT NULL COMMENT '授权类型',
  `status`         ENUM('active','expired','revoked') DEFAULT 'active' COMMENT '状态',
  `source`         ENUM('admin','agent','user_purchase','card') NOT NULL COMMENT '来源',
  `owner_type`     ENUM('user','agent') NOT NULL COMMENT '持有者类型',
  `owner_id`       BIGINT UNSIGNED NOT NULL COMMENT '持有者ID',
  `issued_by`      BIGINT UNSIGNED DEFAULT NULL COMMENT '开通操作者ID',
  `duration_days`  INT UNSIGNED NOT NULL COMMENT '授权时长(天)',
  `started_at`     DATETIME NOT NULL COMMENT '生效时间',
  `expired_at`     DATETIME DEFAULT NULL COMMENT '到期时间',
  `license_key`    VARCHAR(255) DEFAULT '' COMMENT '密钥',
  `max_domains`    INT UNSIGNED DEFAULT 1 COMMENT '最大绑定数',
  `remark`         VARCHAR(255) DEFAULT '' COMMENT '备注',
  `created_at`     DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`     DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_license_no` (`license_no`),
  KEY `idx_app` (`app_id`),
  KEY `idx_owner` (`owner_type`, `owner_id`),
  KEY `idx_status_expired` (`status`, `expired_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='授权表';

DROP TABLE IF EXISTS `license_domains`;
CREATE TABLE `license_domains` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `license_id`  BIGINT UNSIGNED NOT NULL COMMENT '授权ID',
  `domain`      VARCHAR(255) NOT NULL COMMENT '绑定域名或IP',
  `is_wildcard` TINYINT(1) DEFAULT 0 COMMENT '是否泛域名',
  `created_at`  DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_license` (`license_id`),
  KEY `idx_domain` (`domain`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='授权绑定域名/IP表';

DROP TABLE IF EXISTS `license_card_batches`;
CREATE TABLE `license_card_batches` (
  `id`                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `batch_no`           VARCHAR(64) NOT NULL COMMENT '卡密批次号',
  `app_id`             BIGINT UNSIGNED NOT NULL COMMENT '应用ID',
  `plan_id`            BIGINT UNSIGNED NOT NULL COMMENT '套餐ID',
  `app_name_snapshot`  VARCHAR(100) NOT NULL COMMENT '应用名称快照',
  `plan_name_snapshot` VARCHAR(100) NOT NULL COMMENT '套餐名称快照',
  `duration_days`      INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '套餐时长快照',
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

DROP TABLE IF EXISTS `verify_logs`;
CREATE TABLE `verify_logs` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `license_id`  BIGINT UNSIGNED DEFAULT NULL COMMENT '匹配到的授权ID',
  `app_id`      BIGINT UNSIGNED NOT NULL COMMENT '请求验证的应用ID',
  `domain`      VARCHAR(255) DEFAULT '' COMMENT '请求验证的域名或IP',
  `server_ip`   VARCHAR(45)  DEFAULT '' COMMENT '服务器IP',
  `client_ip`   VARCHAR(45)  DEFAULT '' COMMENT '客户端IP',
  `result`      ENUM('pass','fail','expired','blacklisted') NOT NULL COMMENT '验证结果',
  `fail_reason` VARCHAR(255) DEFAULT '' COMMENT '失败原因',
  `user_agent`  VARCHAR(500) DEFAULT '' COMMENT 'User-Agent',
  `created_at`  DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '验证时间',
  PRIMARY KEY (`id`),
  KEY `idx_license` (`license_id`),
  KEY `idx_app_time` (`app_id`, `created_at`),
  KEY `idx_domain` (`domain`),
  KEY `idx_server_ip` (`server_ip`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='验证日志表';

DROP TABLE IF EXISTS `piracy_records`;
CREATE TABLE `piracy_records` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `app_id`     BIGINT UNSIGNED NOT NULL COMMENT '关联应用ID',
  `domain`     VARCHAR(255) NOT NULL COMMENT '盗版域名或IP',
  `server_ip`  VARCHAR(45)  DEFAULT '' COMMENT '盗版服务器IP',
  `status`     ENUM('discovered','blocked') DEFAULT 'discovered' COMMENT '状态',
  `hit_count`  INT UNSIGNED DEFAULT 1 COMMENT '命中次数',
  `first_seen` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '首次发现时间',
  `last_seen`  DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '最近命中时间',
  `evidence`   TEXT COMMENT '证据快照(JSON)',
  `remark`     VARCHAR(255) DEFAULT '' COMMENT '备注',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_app_domain` (`app_id`, `domain`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='盗版追踪记录表';

DROP TABLE IF EXISTS `piracy_alerts`;
CREATE TABLE `piracy_alerts` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `piracy_id`    BIGINT UNSIGNED DEFAULT NULL COMMENT '关联追踪记录ID',
  `app_id`       BIGINT UNSIGNED NOT NULL COMMENT '关联应用ID',
  `alert_type`   VARCHAR(50) NOT NULL COMMENT '告警类型',
  `severity`     ENUM('low','medium','high') DEFAULT 'medium' COMMENT '严重程度',
  `status`       ENUM('pending','processed','ignored') DEFAULT 'pending' COMMENT '处理状态',
  `domain`       VARCHAR(255) DEFAULT '' COMMENT '触发告警的域名或IP',
  `detail`       TEXT COMMENT '告警详情(JSON)',
  `processed_at` DATETIME DEFAULT NULL COMMENT '处理时间',
  `processed_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '处理人',
  `created_at`   DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_status` (`status`, `created_at`),
  KEY `idx_app` (`app_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='盗版告警表';

DROP TABLE IF EXISTS `piracy_blacklist`;
CREATE TABLE `piracy_blacklist` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `app_id`     BIGINT UNSIGNED NOT NULL COMMENT '关联应用ID',
  `type`       ENUM('domain','ip') NOT NULL COMMENT '黑名单类型',
  `value`      VARCHAR(255) NOT NULL COMMENT '被拉黑的域名或IP',
  `reason`     VARCHAR(255) DEFAULT '' COMMENT '拉黑原因',
  `piracy_id`  BIGINT UNSIGNED DEFAULT NULL COMMENT '来源追踪记录ID',
  `blocked_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '操作人',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '拉黑时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_app_type_value` (`app_id`, `type`, `value`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='黑名单表';

DROP TABLE IF EXISTS `transactions`;
CREATE TABLE `transactions` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `tx_no`         VARCHAR(64) NOT NULL COMMENT '流水号',
  `subject_type`  ENUM('agent','user') NOT NULL COMMENT '主体类型',
  `subject_id`    BIGINT UNSIGNED NOT NULL COMMENT '主体ID',
  `type`          ENUM('recharge','consume','refund','purchase') NOT NULL COMMENT '类型',
  `amount`        DECIMAL(12,2) NOT NULL COMMENT '金额',
  `balance_after` DECIMAL(12,2) DEFAULT NULL COMMENT '交易后余额',
  `ref_type`      VARCHAR(50)  DEFAULT '' COMMENT '关联对象类型',
  `ref_id`        BIGINT UNSIGNED DEFAULT NULL COMMENT '关联对象ID',
  `remark`        VARCHAR(255) DEFAULT '' COMMENT '备注',
  `created_at`    DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tx_no` (`tx_no`),
  KEY `idx_subject` (`subject_type`, `subject_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='财务流水表';

DROP TABLE IF EXISTS `recharge_orders`;
CREATE TABLE `recharge_orders` (
  `id`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `order_no`         VARCHAR(64)  NOT NULL COMMENT '订单号',
  `subject_type`     ENUM('agent','user','test') NOT NULL DEFAULT 'agent' COMMENT '充值主体类型',
  `subject_id`       BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '充值主体ID',
  `agent_id`         BIGINT UNSIGNED DEFAULT NULL COMMENT '充值代理商ID',
  `user_id`          BIGINT UNSIGNED DEFAULT NULL COMMENT '充值用户ID',
  `amount`           DECIMAL(12,2) NOT NULL COMMENT '充值金额',
  `paid_amount`      DECIMAL(12,2) DEFAULT NULL COMMENT '实际支付金额',
  `pay_channel`      VARCHAR(30)  DEFAULT '' COMMENT '支付渠道',
  `pay_method`       VARCHAR(30)  DEFAULT '' COMMENT '支付方式',
  `gateway_trade_no` VARCHAR(100) DEFAULT '' COMMENT '支付网关交易号',
  `return_url`       VARCHAR(500) DEFAULT '' COMMENT '支付完成前端返回地址',
  `status`           ENUM('pending','paid','failed','cancelled') DEFAULT 'pending' COMMENT '状态',
  `paid_at`          DATETIME     DEFAULT NULL COMMENT '支付完成时间',
  `approved_by`      BIGINT UNSIGNED DEFAULT NULL COMMENT '审核人',
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

DROP TABLE IF EXISTS `operation_logs`;
CREATE TABLE `operation_logs` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `operator_type` ENUM('admin','agent','user','system') NOT NULL COMMENT '操作者类型',
  `operator_id`   BIGINT UNSIGNED DEFAULT NULL COMMENT '操作者ID',
  `action`        VARCHAR(100) NOT NULL COMMENT '操作动作',
  `target_type`   VARCHAR(50)  DEFAULT '' COMMENT '目标对象类型',
  `target_id`     BIGINT UNSIGNED DEFAULT NULL COMMENT '目标对象ID',
  `detail`        JSON DEFAULT NULL COMMENT '变更详情',
  `ip`            VARCHAR(45)  DEFAULT '' COMMENT '操作者IP',
  `created_at`    DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '操作时间',
  PRIMARY KEY (`id`),
  KEY `idx_operator` (`operator_type`, `operator_id`),
  KEY `idx_target` (`target_type`, `target_id`),
  KEY `idx_time` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='操作日志表';

DROP TABLE IF EXISTS `mail_send_logs`;
CREATE TABLE `mail_send_logs` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `event_type`  VARCHAR(50) NOT NULL COMMENT '事件类型',
  `target_type` VARCHAR(30) DEFAULT '' COMMENT '收件主体类型',
  `target_id`   BIGINT UNSIGNED DEFAULT NULL COMMENT '收件主体ID',
  `license_id`  BIGINT UNSIGNED DEFAULT NULL COMMENT '授权ID',
  `recipient`   VARCHAR(255) DEFAULT '' COMMENT '收件邮箱',
  `subject`     VARCHAR(255) DEFAULT '' COMMENT '邮件标题',
  `content`     LONGTEXT COMMENT '邮件内容',
  `status`      VARCHAR(20) NOT NULL DEFAULT 'pending' COMMENT '发送状态',
  `error`       TEXT COMMENT '错误信息',
  `remind_days` INT DEFAULT NULL COMMENT '提醒天数',
  `event_key`   VARCHAR(120) DEFAULT NULL COMMENT '幂等键',
  `created_at`  DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `sent_at`     DATETIME DEFAULT NULL COMMENT '发送时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_event_key` (`event_key`),
  KEY `idx_event_status` (`event_type`, `status`, `created_at`),
  KEY `idx_license_event` (`license_id`, `event_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='邮件发送日志表';

DROP TABLE IF EXISTS `system_configs`;
CREATE TABLE `system_configs` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `group`       VARCHAR(50)  NOT NULL COMMENT '配置分组',
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

DROP TABLE IF EXISTS `menus`;
CREATE TABLE `menus` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `parent_id`   BIGINT UNSIGNED DEFAULT 0 COMMENT '父菜单ID，0为顶级',
  `name`        VARCHAR(50)  NOT NULL COMMENT '路由name',
  `path`        VARCHAR(200) NOT NULL COMMENT '路由path',
  `component`   VARCHAR(200) DEFAULT '' COMMENT '组件路径',
  `redirect`    VARCHAR(200) DEFAULT '' COMMENT '重定向',
  `title`       VARCHAR(100) DEFAULT '' COMMENT '菜单标题',
  `icon`        VARCHAR(100) DEFAULT '' COMMENT '图标',
  `sort`        INT DEFAULT 0 COMMENT '排序',
  `is_hide`     TINYINT(1) DEFAULT 0 COMMENT '是否隐藏',
  `is_hide_tab` TINYINT(1) DEFAULT 0 COMMENT '是否隐藏标签',
  `is_full_page` TINYINT(1) DEFAULT 0 COMMENT '是否全屏',
  `keep_alive`  TINYINT(1) DEFAULT 0 COMMENT '是否缓存',
  `fixed_tab`   TINYINT(1) DEFAULT 0 COMMENT '是否固定标签',
  `enabled`     TINYINT(1) DEFAULT 1,
  `created_at`  DATETIME DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='菜单表';

DROP TABLE IF EXISTS `role_menus`;
CREATE TABLE `role_menus` (
  `role_id` BIGINT UNSIGNED NOT NULL,
  `menu_id` BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (`role_id`, `menu_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色菜单关联表';

SET FOREIGN_KEY_CHECKS = 1;
