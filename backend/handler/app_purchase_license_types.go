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
	ensureAppPurchaseLicenseTypes  = EnsureAppPurchaseLicenseTypesColumn
	selfPurchaseEnabledForPurchase = isSelfPurchaseEnabled
	queuePurchaseSuccessMail       = QueuePurchaseSuccessMail
	queueAdminLicenseOpenedMail    = QueueLicenseOpenedMail
	ensureAgentPurchaseSchemas     = func(db *sql.DB) error {
		if err := ensureAgentLevelSchema(db); err != nil {
			return fmt.Errorf("代理商等级表初始化失败: %w", err)
		}
		if err := ensureAgentQuotaSchema(db); err != nil {
			return fmt.Errorf("配额表初始化失败: %w", err)
		}
		if err := ensureLicensePriceSnapshotSchema(db); err != nil {
			return fmt.Errorf("授权价格快照字段初始化失败: %w", err)
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
