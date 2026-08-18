package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"

	"auto_pro/config"

	"github.com/go-sql-driver/mysql"
)

const (
	purchaseLicenseTypeDomain uint8 = 1 << iota
	purchaseLicenseTypeWildcard
	purchaseLicenseTypeIP
	purchaseLicenseTypeKey
	purchaseLicenseTypeAll = purchaseLicenseTypeDomain | purchaseLicenseTypeWildcard | purchaseLicenseTypeIP | purchaseLicenseTypeKey
)

var (
	purchaseLicenseTypeOrder = []string{"domain", "wildcard", "ip", "key"}
	purchaseLicenseTypeBits  = map[string]uint8{
		"domain":   purchaseLicenseTypeDomain,
		"wildcard": purchaseLicenseTypeWildcard,
		"ip":       purchaseLicenseTypeIP,
		"key":      purchaseLicenseTypeKey,
	}
	appPurchaseLicenseTypesMu sync.Mutex
	appPurchaseLicenseTypesOK bool
	errPurchaseTypeNotAllowed = errors.New("应用不支持该购买授权类型")
	openAppPurchaseDB         = func() (*sql.DB, error) {
		cfg, _ := config.LoadDBConfig()
		return sql.Open("mysql", config.GetDSN(cfg))
	}
	openAdminLicenseDB = func() (*sql.DB, error) {
		cfg, err := config.LoadDBConfig()
		if err != nil {
			return nil, err
		}
		return sql.Open("mysql", config.GetDSN(cfg))
	}
	ensureAppPurchaseLicenseTypes    = EnsureAppPurchaseLicenseTypesColumn
	ensurePlanLicenseType            = ensurePlanLicenseTypeColumn
	ensurePurchaseOrderPricingSchema = ensureLicensePurchaseOrderSchema
	ensurePurchasePromotionSchema    = ensurePromotionCampaignSchema
	selfPurchaseEnabledForPurchase   = isSelfPurchaseEnabled
	queuePurchaseSuccessMail         = QueuePurchaseSuccessMail
	queueAdminLicenseOpenedMail      = QueueLicenseOpenedMail
	ensureAgentPurchaseSchemas       = func(db *sql.DB) error {
		if err := ensureAgentLevelSchema(db); err != nil {
			return fmt.Errorf("代理商等级表初始化失败: %w", err)
		}
		if err := ensureAgentQuotaSchema(db); err != nil {
			return fmt.Errorf("配额表初始化失败: %w", err)
		}
		if err := ensureLicensePurchaseOrderSchema(db); err != nil {
			return fmt.Errorf("授权购买订单价格快照初始化失败: %w", err)
		}
		if err := ensurePurchasePromotionSchema(db); err != nil {
			return fmt.Errorf("促销活动表初始化失败: %w", err)
		}
		return nil
	}
)

func EnsureAppPurchaseLicenseTypesColumn(db *sql.DB) error {
	appPurchaseLicenseTypesMu.Lock()
	defer appPurchaseLicenseTypesMu.Unlock()
	if appPurchaseLicenseTypesOK {
		return nil
	}

	var exists int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'apps'
		  AND COLUMN_NAME = 'purchase_license_type_mask'
	`).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		_, err := db.Exec(`
			ALTER TABLE apps
			ADD COLUMN purchase_license_type_mask TINYINT UNSIGNED NOT NULL DEFAULT 15
			COMMENT '允许用户和代理购买的授权类型位掩码' AFTER enabled
		`)
		if err != nil {
			var mysqlErr *mysql.MySQLError
			if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1060 {
				return err
			}
		}
	}
	appPurchaseLicenseTypesOK = true
	return nil
}

func parsePurchaseLicenseTypes(types []string) (uint8, error) {
	var mask uint8
	for _, value := range types {
		licenseType := strings.ToLower(strings.TrimSpace(value))
		bit, ok := purchaseLicenseTypeBits[licenseType]
		if !ok {
			return 0, fmt.Errorf("无效的购买授权类型: %s", value)
		}
		mask |= bit
	}
	return mask, nil
}

func purchaseLicenseTypeMaskForCreate(types []string) (uint8, error) {
	if types == nil {
		return purchaseLicenseTypeAll, nil
	}
	return parsePurchaseLicenseTypes(types)
}

func purchaseLicenseTypesFromMask(mask uint8) []string {
	types := make([]string, 0, len(purchaseLicenseTypeOrder))
	for _, licenseType := range purchaseLicenseTypeOrder {
		if mask&purchaseLicenseTypeBits[licenseType] != 0 {
			types = append(types, licenseType)
		}
	}
	return types
}

func purchaseLicenseTypeAllowed(mask uint8, licenseType string) bool {
	bit, ok := purchaseLicenseTypeBits[strings.ToLower(strings.TrimSpace(licenseType))]
	return ok && mask&bit != 0
}

func purchaseLicenseTypeLabel(licenseType string) string {
	labels := map[string]string{
		"domain":   "单域名",
		"wildcard": "泛域名",
		"ip":       "IP",
		"key":      "密钥",
	}
	if label := labels[licenseType]; label != "" {
		return label
	}
	return licenseType
}

func requireAppPurchaseLicenseType(tx *sql.Tx, appID int64, licenseType string) error {
	var mask uint8
	if err := tx.QueryRow(`
		SELECT purchase_license_type_mask
		FROM apps
		WHERE id = ? AND enabled = 1
		LOCK IN SHARE MODE
	`, appID).Scan(&mask); err != nil {
		return err
	}
	if !purchaseLicenseTypeAllowed(mask, licenseType) {
		return errPurchaseTypeNotAllowed
	}
	return nil
}

func purchaseLicenseTypeNotAllowedMessage(licenseType string) string {
	return fmt.Sprintf("该应用暂不支持购买%s授权", purchaseLicenseTypeLabel(licenseType))
}

// validPlanLicenseType 校验套餐授权方式，空值表示通用（不限方式）
func validPlanLicenseType(licenseType string) bool {
	if licenseType == "" {
		return true
	}
	_, ok := purchaseLicenseTypeBits[licenseType]
	return ok
}

var (
	planLicenseTypeMu sync.Mutex
	planLicenseTypeOK bool
)

// ensurePlanLicenseTypeColumn 给 license_plans 加 license_type 列并调整唯一键
func ensurePlanLicenseTypeColumn(db *sql.DB) error {
	planLicenseTypeMu.Lock()
	defer planLicenseTypeMu.Unlock()
	if planLicenseTypeOK {
		return nil
	}

	if err := ensureColumn(db, "license_plans", "license_type",
		"ALTER TABLE license_plans ADD COLUMN license_type VARCHAR(20) NOT NULL DEFAULT '' COMMENT '适用授权方式：空=通用,domain,wildcard,ip,key' AFTER name"); err != nil {
		return err
	}

	// 唯一键从 (app_id, name) 调整为 (app_id, name, license_type)，允许同名不同方式共存
	var oldKeyCount int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'license_plans'
		  AND INDEX_NAME = 'uk_app_plan_name' AND COLUMN_NAME = 'license_type'
	`).Scan(&oldKeyCount); err != nil {
		return err
	}
	if oldKeyCount == 0 {
		if _, err := db.Exec("ALTER TABLE license_plans DROP INDEX uk_app_plan_name"); err != nil {
			var mysqlErr *mysql.MySQLError
			if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1091 {
				return err
			}
		}
		if _, err := db.Exec("ALTER TABLE license_plans ADD UNIQUE KEY uk_app_plan_name (app_id, name, license_type)"); err != nil {
			var mysqlErr *mysql.MySQLError
			if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1061 {
				return err
			}
		}
	}
	planLicenseTypeOK = true
	return nil
}
