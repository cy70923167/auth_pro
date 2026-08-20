package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net"
	"strings"
)

const (
	licenseSignVersionV1 = "v1"
	licenseSignVersionV2 = "v2"
)

var (
	errLicenseSiteEmpty         = errors.New("empty_target")
	errLicenseSiteInvalidDomain = errors.New("invalid_domain")
	errLicenseSiteInvalidIP     = errors.New("invalid_server_ip")
	errLicenseSiteNotBound      = errors.New("site_not_bound")
	errLicenseSiteLimitReached  = errors.New("site_limit_exceeded")
	errLicenseSignatureUpgrade  = errors.New("signature_upgrade_required")
)

type licenseSiteTarget struct {
	Type     string
	Value    string
	ServerIP string
}

func normalizeLicenseSignVersion(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "2", licenseSignVersionV2:
		return licenseSignVersionV2
	default:
		return licenseSignVersionV1
	}
}

func normalizeLicenseDomain(value string) string {
	value = strings.TrimSuffix(normalizeLicenseTarget(value), ".")
	value = strings.TrimPrefix(strings.TrimSuffix(value, "]"), "[")
	if value == "" {
		return ""
	}
	if parsed := net.ParseIP(value); parsed != nil {
		return parsed.String()
	}
	if len(value) > 253 || strings.ContainsAny(value, " \\@?#%") {
		return ""
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return ""
		}
		for _, char := range label {
			if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
				continue
			}
			return ""
		}
	}
	return value
}

func normalizeLicenseServerIP(value string) string {
	value = strings.TrimSpace(value)
	if zoneIndex := strings.LastIndex(value, "%"); zoneIndex > 0 {
		value = value[:zoneIndex]
	}
	value = strings.TrimPrefix(strings.TrimSuffix(value, "]"), "[")
	if parsed := net.ParseIP(value); parsed != nil {
		return parsed.String()
	}
	return ""
}

func resolveLicenseSiteTarget(domain, serverIP string) (licenseSiteTarget, error) {
	normalizedDomain := normalizeLicenseDomain(domain)
	normalizedIP := normalizeLicenseServerIP(serverIP)
	if strings.TrimSpace(domain) != "" && normalizedDomain == "" {
		return licenseSiteTarget{}, errLicenseSiteInvalidDomain
	}
	if strings.TrimSpace(serverIP) != "" && normalizedIP == "" {
		return licenseSiteTarget{}, errLicenseSiteInvalidIP
	}
	if normalizedDomain != "" {
		targetType := "domain"
		if net.ParseIP(normalizedDomain) != nil {
			targetType = "ip"
		}
		return licenseSiteTarget{Type: targetType, Value: normalizedDomain, ServerIP: normalizedIP}, nil
	}
	if normalizedIP != "" {
		return licenseSiteTarget{Type: "ip", Value: normalizedIP, ServerIP: normalizedIP}, nil
	}
	return licenseSiteTarget{}, errLicenseSiteEmpty
}

func licenseVerifyV2Canonical(req licenseVerifyRequest) string {
	return strings.Join([]string{
		licenseSignVersionV2,
		req.AppKey,
		req.LicenseKey,
		normalizeLicenseDomain(req.Domain),
		normalizeLicenseServerIP(req.ServerIP),
		int64ToString(req.Timestamp),
	}, "\n")
}

func licenseVerifyV2Sign(req licenseVerifyRequest, appSecret string) string {
	mac := hmac.New(sha256.New, []byte(appSecret))
	_, _ = mac.Write([]byte(licenseVerifyV2Canonical(req)))
	return hex.EncodeToString(mac.Sum(nil))
}

func checkKeyLicenseSite(db *sql.DB, licenseID int64, domain, serverIP string, allowCreate bool) error {
	target, err := resolveLicenseSiteTarget(domain, serverIP)
	if err != nil {
		return err
	}

	for attempt := 0; ; attempt++ {
		err := checkKeyLicenseSiteOnce(db, licenseID, target, allowCreate)
		if err != nil && attempt < 2 && isDuplicateEntry(err) {
			continue
		}
		return err
	}
}

func checkKeyLicenseSiteOnce(db *sql.DB, licenseID int64, target licenseSiteTarget, allowCreate bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var licenseType string
	var maxSites int
	if err := tx.QueryRow(`
		SELECT type, COALESCE(max_domains, 0)
		FROM licenses
		WHERE id = ?
		FOR UPDATE
	`, licenseID).Scan(&licenseType, &maxSites); err != nil {
		return err
	}
	if licenseType != "key" {
		return nil
	}

	var bindingID int64
	err = tx.QueryRow(`
		SELECT id FROM license_domains
		WHERE license_id = ? AND target_type = ? AND domain = ?
		FOR UPDATE
	`, licenseID, target.Type, target.Value).Scan(&bindingID)
	if err == nil {
		if _, err := tx.Exec(`
			UPDATE license_domains
			SET server_ip = ?, last_seen_at = NOW(), first_seen_at = COALESCE(first_seen_at, created_at, NOW())
			WHERE id = ?
		`, target.ServerIP, bindingID); err != nil {
			return err
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if !allowCreate {
		return errLicenseSiteNotBound
	}

	if maxSites > 0 {
		var boundSites int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM license_domains WHERE license_id = ?`, licenseID).Scan(&boundSites); err != nil {
			return err
		}
		if boundSites >= maxSites {
			return errLicenseSiteLimitReached
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO license_domains
		(license_id, target_type, domain, is_wildcard, server_ip, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, 0, ?, NOW(), NOW())
	`, licenseID, target.Type, target.Value, target.ServerIP); err != nil {
		return err
	}
	return tx.Commit()
}

func requireKeyLicenseSite(db *sql.DB, licenseID int64, domain, serverIP, signVersion string) error {
	allowCreate := normalizeLicenseSignVersion(signVersion) == licenseSignVersionV2
	err := checkKeyLicenseSite(db, licenseID, domain, serverIP, allowCreate)
	if errors.Is(err, errLicenseSiteNotBound) && !allowCreate {
		return errLicenseSignatureUpgrade
	}
	return err
}
