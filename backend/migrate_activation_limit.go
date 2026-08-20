package main

import (
	"database/sql"
	"fmt"
)

const (
	licenseSiteLimitSnapshotMigration = "license_site_limit_snapshots_v1"
	licenseSiteBindingMigration       = "license_site_bindings_v1"
)

func ensureLicenseSiteLimitSchema(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name VARCHAR(100) NOT NULL,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (name)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='运行时结构迁移记录'`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	shouldBackfillSiteLimits, err := siteLimitMigrationPending(db, licenseSiteLimitSnapshotMigration)
	if err != nil {
		return err
	}
	shouldBackfillBindings, err := siteLimitMigrationPending(db, licenseSiteBindingMigration)
	if err != nil {
		return err
	}

	maxSitesExists, err := siteLimitColumnExists(db, "license_plans", "max_sites")
	if err != nil {
		return err
	}
	legacyLimitExists, err := siteLimitColumnExists(db, "license_plans", "max_activations")
	if err != nil {
		return err
	}

	switch {
	case !maxSitesExists && legacyLimitExists:
		if _, err := db.Exec(`ALTER TABLE license_plans
			CHANGE COLUMN max_activations max_sites INT UNSIGNED NOT NULL DEFAULT 0
			COMMENT '密钥授权最大站点数，0表示不限'`); err != nil {
			return fmt.Errorf("rename license_plans.max_activations: %w", err)
		}
	case !maxSitesExists:
		if _, err := db.Exec(`ALTER TABLE license_plans
			ADD COLUMN max_sites INT UNSIGNED NOT NULL DEFAULT 0
			COMMENT '密钥授权最大站点数，0表示不限' AFTER duration_days`); err != nil {
			return fmt.Errorf("add license_plans.max_sites: %w", err)
		}
	case legacyLimitExists:
		if _, err := db.Exec(`UPDATE license_plans
			SET max_sites = max_activations
			WHERE max_sites = 0 AND max_activations > 0`); err != nil {
			return fmt.Errorf("backfill license_plans.max_sites: %w", err)
		}
	}
	if legacyLimitExists && maxSitesExists {
		if _, err := db.Exec(`ALTER TABLE license_plans DROP COLUMN max_activations`); err != nil {
			return fmt.Errorf("drop license_plans.max_activations: %w", err)
		}
	}

	if err := ensureSiteLimitColumn(db, "licenses", "max_domains", `ALTER TABLE licenses
		ADD COLUMN max_domains INT UNSIGNED NOT NULL DEFAULT 0
		COMMENT '密钥授权最大站点数，0表示不限' AFTER license_key`); err != nil {
		return err
	}
	if _, err := db.Exec(`ALTER TABLE licenses
		MODIFY COLUMN max_domains INT UNSIGNED NOT NULL DEFAULT 0
		COMMENT '密钥授权最大站点数快照，0表示不限'`); err != nil {
		return fmt.Errorf("normalize licenses.max_domains: %w", err)
	}
	if exists, err := siteLimitTableExists(db, "license_purchase_orders"); err != nil {
		return err
	} else if exists {
		if err := ensureSiteLimitColumn(db, "license_purchase_orders", "max_sites_snapshot", `ALTER TABLE license_purchase_orders
			ADD COLUMN max_sites_snapshot INT UNSIGNED DEFAULT NULL
			COMMENT '密钥授权最大站点数快照，0表示不限' AFTER duration_days_snapshot`); err != nil {
			return err
		}
	}
	if exists, err := siteLimitTableExists(db, "license_card_batches"); err != nil {
		return err
	} else if exists {
		if err := ensureSiteLimitColumn(db, "license_card_batches", "max_sites_snapshot", `ALTER TABLE license_card_batches
			ADD COLUMN max_sites_snapshot INT UNSIGNED NOT NULL DEFAULT 0
			COMMENT '密钥授权最大站点数快照，0表示不限' AFTER duration_days`); err != nil {
			return err
		}
	}
	if err := ensureSiteLimitColumn(db, "license_domains", "target_type", `ALTER TABLE license_domains
		ADD COLUMN target_type VARCHAR(10) NOT NULL DEFAULT 'domain'
		COMMENT '绑定目标类型: domain/ip' AFTER license_id`); err != nil {
		return err
	}
	if err := ensureSiteLimitColumn(db, "license_domains", "server_ip", `ALTER TABLE license_domains
		ADD COLUMN server_ip VARCHAR(45) NOT NULL DEFAULT ''
		COMMENT '最近一次上报的服务器IP' AFTER is_wildcard`); err != nil {
		return err
	}
	if err := ensureSiteLimitColumn(db, "license_domains", "first_seen_at", `ALTER TABLE license_domains
		ADD COLUMN first_seen_at DATETIME DEFAULT NULL
		COMMENT '首次绑定时间' AFTER server_ip`); err != nil {
		return err
	}
	if err := ensureSiteLimitColumn(db, "license_domains", "last_seen_at", `ALTER TABLE license_domains
		ADD COLUMN last_seen_at DATETIME DEFAULT NULL
		COMMENT '最近验证时间' AFTER first_seen_at`); err != nil {
		return err
	}

	if _, err := db.Exec(`UPDATE license_domains ld
		JOIN licenses l ON l.id = ld.license_id
		SET ld.domain = LOWER(TRIM(TRAILING '.' FROM TRIM(ld.domain))),
		    ld.target_type = CASE
		      WHEN l.type = 'ip' OR (l.type = 'key' AND INET6_ATON(TRIM(ld.domain)) IS NOT NULL) THEN 'ip'
		      ELSE 'domain'
		    END,
		    ld.first_seen_at = COALESCE(ld.first_seen_at, ld.created_at),
		    ld.last_seen_at = COALESCE(ld.last_seen_at, ld.created_at)`); err != nil {
		return fmt.Errorf("normalize existing license bindings: %w", err)
	}

	if _, err := db.Exec(`DELETE newer FROM license_domains newer
		JOIN license_domains older
		  ON older.license_id = newer.license_id
		 AND older.target_type = newer.target_type
		 AND older.domain = newer.domain
		 AND older.id < newer.id`); err != nil {
		return fmt.Errorf("deduplicate license bindings: %w", err)
	}

	uniqueExists, err := siteLimitIndexExists(db, "license_domains", "uk_license_target")
	if err != nil {
		return err
	}
	if !uniqueExists {
		if _, err := db.Exec(`CREATE UNIQUE INDEX uk_license_target
			ON license_domains (license_id, target_type, domain)`); err != nil {
			return fmt.Errorf("create license binding unique index: %w", err)
		}
	}

	if shouldBackfillSiteLimits {
		if _, err := db.Exec(`UPDATE licenses l
			JOIN license_plans p ON p.id = l.plan_id
			SET l.max_domains = p.max_sites
			WHERE l.type = 'key'`); err != nil {
			return fmt.Errorf("snapshot existing key site limits: %w", err)
		}
		if exists, err := siteLimitTableExists(db, "license_purchase_orders"); err != nil {
			return err
		} else if exists {
			if _, err := db.Exec(`UPDATE license_purchase_orders o
				JOIN license_plans p ON p.id = o.plan_id
				SET o.max_sites_snapshot = p.max_sites
				WHERE o.max_sites_snapshot IS NULL`); err != nil {
				return fmt.Errorf("snapshot pending purchase site limits: %w", err)
			}
		}
		if exists, err := siteLimitTableExists(db, "license_card_batches"); err != nil {
			return err
		} else if exists {
			if _, err := db.Exec(`UPDATE license_card_batches b
				JOIN license_plans p ON p.id = b.plan_id
				SET b.max_sites_snapshot = p.max_sites`); err != nil {
				return fmt.Errorf("snapshot license card site limits: %w", err)
			}
		}
		if err := markSiteLimitMigration(db, licenseSiteLimitSnapshotMigration); err != nil {
			return err
		}
	}

	if shouldBackfillBindings {
		if _, err := db.Exec(`INSERT IGNORE INTO license_domains
			(license_id, target_type, domain, is_wildcard, server_ip, first_seen_at, last_seen_at, created_at)
			SELECT history.license_id, history.target_type, history.target, 0,
			       SUBSTRING_INDEX(GROUP_CONCAT(history.server_ip ORDER BY history.created_at DESC), ',', 1),
			       MIN(history.created_at), MAX(history.created_at), MIN(history.created_at)
			FROM (
				SELECT v.license_id,
				       CASE WHEN TRIM(v.domain) <> '' THEN 'domain' ELSE 'ip' END AS target_type,
				       CASE
				         WHEN TRIM(v.domain) <> '' THEN LOWER(TRIM(TRAILING '.' FROM TRIM(v.domain)))
				         ELSE TRIM(v.server_ip)
				       END AS target,
				       TRIM(v.server_ip) AS server_ip, v.created_at
				FROM verify_logs v
				JOIN licenses l ON l.id = v.license_id AND l.type = 'key'
				WHERE v.result = 'pass'
				  AND (TRIM(v.domain) <> '' OR TRIM(v.server_ip) <> '')
			) history
			WHERE history.target <> ''
			  AND NOT EXISTS (
			    SELECT 1
			    FROM operation_logs o
			    WHERE o.action = 'license_site_unbind'
			      AND o.target_type = 'license'
			      AND o.target_id = history.license_id
			      AND JSON_UNQUOTE(JSON_EXTRACT(o.detail, '$.targetType')) = history.target_type
			      AND JSON_UNQUOTE(JSON_EXTRACT(o.detail, '$.target')) = history.target
			  )
			GROUP BY history.license_id, history.target_type, history.target`); err != nil {
			return fmt.Errorf("backfill key site bindings: %w", err)
		}
		if err := markSiteLimitMigration(db, licenseSiteBindingMigration); err != nil {
			return err
		}
	}

	return nil
}

func siteLimitMigrationPending(db *sql.DB, name string) (bool, error) {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE name = ?`, name).Scan(&count); err != nil {
		return false, fmt.Errorf("check migration %s: %w", name, err)
	}
	return count == 0, nil
}

func markSiteLimitMigration(db *sql.DB, name string) error {
	if _, err := db.Exec(`INSERT IGNORE INTO schema_migrations (name) VALUES (?)`, name); err != nil {
		return fmt.Errorf("mark migration %s: %w", name, err)
	}
	return nil
}

func ensureSiteLimitColumn(db *sql.DB, table, column, statement string) error {
	exists, err := siteLimitColumnExists(db, table, column)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if _, err := db.Exec(statement); err != nil {
		return fmt.Errorf("add %s.%s: %w", table, column, err)
	}
	return nil
}

func siteLimitTableExists(db *sql.DB, table string) (bool, error) {
	var count int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
	`, table).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func siteLimitColumnExists(db *sql.DB, table, column string) (bool, error) {
	var count int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?
	`, table, column).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func siteLimitIndexExists(db *sql.DB, table, index string) (bool, error) {
	var count int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?
	`, table, index).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}
