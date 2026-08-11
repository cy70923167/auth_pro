package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
)

type appVersionIntegrationEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type appVersionIntegrationItem struct {
	ID            int64   `json:"id"`
	Version       string  `json:"version"`
	Title         string  `json:"title"`
	UpdateSQL     string  `json:"updateSql"`
	PackageName   string  `json:"packageName"`
	SourceType    string  `json:"sourceType"`
	DownloadURL   string  `json:"downloadUrl"`
	FileSizeBytes int64   `json:"fileSizeBytes"`
	FileSizeMB    float64 `json:"fileSizeMb"`
	FileMD5       string  `json:"fileMd5"`
	ForceUpdate   bool    `json:"forceUpdate"`
	MinVersion    string  `json:"minVersion"`
	Revision      int64   `json:"revision"`
}

type appVersionIntegrationList struct {
	List          []appVersionIntegrationItem `json:"list"`
	Total         int                         `json:"total"`
	LatestVersion string                      `json:"latestVersion"`
}

type appVersionIntegrationUpdate struct {
	Version   string `json:"version"`
	Title     string `json:"title"`
	Changelog string `json:"changelog"`
	UpdateSQL string `json:"updateSql"`
}

type appVersionIntegrationCheck struct {
	HasUpdate     bool                          `json:"hasUpdate"`
	Current       string                        `json:"currentVersion"`
	Latest        string                        `json:"latestVersion"`
	Version       string                        `json:"version"`
	DownloadURL   string                        `json:"downloadUrl"`
	FileSizeBytes int64                         `json:"fileSizeBytes"`
	FileMD5       string                        `json:"fileMd5"`
	ForceUpdate   bool                          `json:"forceUpdate"`
	MinVersion    string                        `json:"minVersion"`
	Updates       []appVersionIntegrationUpdate `json:"updates"`
}

func TestAppVersionHTTPIntegration(t *testing.T) {
	if os.Getenv("AUTO_PRO_RUN_DB_INTEGRATION") != "1" {
		t.Skip("set AUTO_PRO_RUN_DB_INTEGRATION=1 to run the real MySQL HTTP integration test")
	}
	gin.SetMode(gin.TestMode)

	backendDir, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUTO_PRO_DATA_DIR", backendDir)
	releaseDir := t.TempDir()
	t.Setenv("AUTO_PRO_APP_RELEASE_DIR", releaseDir)

	cfg, err := config.LoadDBConfig()
	if err != nil {
		t.Fatalf("load database config: %v", err)
	}
	db, err := sql.Open("mysql", config.GetDSN(cfg))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	ctxDeadline := 10 * time.Second
	db.SetConnMaxLifetime(ctxDeadline)
	if err := db.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	appVersionTableMu.Lock()
	appVersionTableOK = false
	appVersionTableMu.Unlock()
	if err := EnsureAppVersionsTable(db); err != nil {
		t.Fatalf("migrate app_versions: %v", err)
	}
	assertAppVersionMigration(t, db)

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	appKey := "version_it_" + suffix
	appSecret := "secret_" + suffix
	licenseKey := "license_" + suffix
	result, err := db.Exec(`
		INSERT INTO apps (app_name, app_key, app_secret, description, enabled)
		VALUES (?, ?, ?, 'app version integration test', 1)
	`, "Version Integration "+suffix, appKey, appSecret)
	if err != nil {
		t.Fatalf("create test app: %v", err)
	}
	appID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read test app id: %v", err)
	}
	var licenseID int64
	t.Cleanup(func() {
		rows, queryErr := db.Query(`SELECT package_path FROM app_versions WHERE app_id = ? AND package_path <> ''`, appID)
		if queryErr == nil {
			for rows.Next() {
				var packagePath string
				if rows.Scan(&packagePath) == nil {
					_ = removeReleasePackage(packagePath)
				}
			}
			rows.Close()
		}
		_, _ = db.Exec(`DELETE FROM app_versions WHERE app_id = ?`, appID)
		if licenseID > 0 {
			_, _ = db.Exec(`DELETE FROM license_domains WHERE license_id = ?`, licenseID)
		}
		_, _ = db.Exec(`DELETE FROM licenses WHERE app_id = ?`, appID)
		_, _ = db.Exec(`DELETE FROM apps WHERE id = ?`, appID)
	})

	result, err = db.Exec(`
		INSERT INTO licenses (
			license_no, app_id, type, status, source, owner_type, owner_id,
			duration_days, started_at, expired_at, license_key, max_domains, remark
		) VALUES (?, ?, 'key', 'active', 'admin', 'user', 0, 365, NOW(), DATE_ADD(NOW(), INTERVAL 365 DAY), ?, 1, ?)
	`, "LIT-"+suffix, appID, licenseKey, "app version integration test")
	if err != nil {
		t.Fatalf("create test license: %v", err)
	}
	licenseID, err = result.LastInsertId()
	if err != nil {
		t.Fatalf("read test license id: %v", err)
	}

	router := gin.New()
	router.GET("/api/app/:id/versions", AppVersionList)
	router.POST("/api/app/:id/versions", AppVersionCreate)
	router.PUT("/api/app/:id/versions/:versionId", AppVersionUpdate)
	router.DELETE("/api/app/:id/versions/:versionId", AppVersionDelete)
	router.POST("/api/app/:id/versions/:versionId/download-url", AppVersionAdminDownloadURL)
	router.POST("/api/app/version/check", AppVersionCheck)
	router.GET("/api/app/version/download", AppVersionDownload)
	server := httptest.NewServer(router)
	defer server.Close()
	client := server.Client()
	appURL := server.URL + "/api/app/" + strconv.FormatInt(appID, 10) + "/versions"

	firstID := createIntegrationVersion(t, client, appURL, map[string]string{
		"version": "1.1.0", "title": "first migration", "changelog": "prepare schema",
		"updateSql":  "ALTER TABLE integration_demo ADD COLUMN first_step INT;",
		"sourceType": "url", "downloadUrl": "https://example.com/releases/1.1.0.zip",
		"fileSizeMb": "1.25", "fileMd5": "11111111111111111111111111111111",
		"forceUpdate": "false", "minVersion": "1.1.0",
	}, "", nil)

	packageContent := []byte("real multipart application release package\n")
	packageHash := md5.Sum(packageContent)
	packageMD5 := hex.EncodeToString(packageHash[:])
	latestID := createIntegrationVersion(t, client, appURL, map[string]string{
		"version": "1.2.0", "title": "forced security release", "changelog": "security fixes",
		"updateSql":  "ALTER TABLE integration_demo ADD COLUMN second_step INT;",
		"sourceType": "upload", "forceUpdate": "true", "minVersion": "1.1.0",
	}, "release-1.2.0.zip", packageContent)

	concurrentBody, concurrentType := buildIntegrationMultipart(t, map[string]string{
		"version": "1.1.5", "title": "concurrent release", "changelog": "concurrent create",
		"updateSql": "", "sourceType": "url", "downloadUrl": "https://example.com/releases/1.1.5.zip",
		"fileSizeMb": "1", "fileMd5": "15151515151515151515151515151515",
		"forceUpdate": "false", "minVersion": "",
	}, "", nil)
	createCodes := concurrentIntegrationRequests(t, client, http.MethodPost, appURL, concurrentBody, concurrentType)
	assertIntegrationCodes(t, createCodes, 200, 400)

	list := listIntegrationVersions(t, client, appURL)
	if list.Total != 3 || list.LatestVersion != "1.2.0" {
		t.Fatalf("unexpected version list metadata: total=%d latest=%q", list.Total, list.LatestVersion)
	}
	latest := findIntegrationVersion(t, list.List, latestID)
	if latest.SourceType != "upload" || latest.DownloadURL != "" || latest.FileSizeBytes != int64(len(packageContent)) || latest.FileMD5 != packageMD5 || latest.Revision != 1 {
		t.Fatalf("unexpected uploaded release metadata: %#v", latest)
	}
	packagePath := queryIntegrationPackagePath(t, db, latestID)
	fullPackagePath, err := resolveReleasePackagePath(packagePath)
	if err != nil {
		t.Fatalf("resolve uploaded package: %v", err)
	}
	if saved, err := os.ReadFile(fullPackagePath); err != nil || !bytes.Equal(saved, packageContent) {
		t.Fatalf("uploaded package mismatch: error=%v content=%q", err, saved)
	}

	first := findIntegrationVersion(t, list.List, firstID)
	editBody, editType := buildIntegrationMultipart(t, map[string]string{
		"version": first.Version, "title": "concurrent edit A", "changelog": "edit conflict A",
		"updateSql": first.UpdateSQL, "sourceType": "url", "downloadUrl": first.DownloadURL,
		"fileSizeMb": strconv.FormatFloat(first.FileSizeMB, 'f', 3, 64), "fileMd5": first.FileMD5,
		"forceUpdate": "false", "minVersion": first.MinVersion, "revision": strconv.FormatInt(first.Revision, 10),
	}, "", nil)
	editURL := appURL + "/" + strconv.FormatInt(firstID, 10)
	editCodes := concurrentIntegrationRequests(t, client, http.MethodPut, editURL, editBody, editType)
	assertIntegrationCodes(t, editCodes, 200, 409)
	first = findIntegrationVersion(t, listIntegrationVersions(t, client, appURL).List, firstID)
	if first.Revision != 2 {
		t.Fatalf("revision after concurrent edit = %d, want 2", first.Revision)
	}

	check := checkIntegrationVersion(t, client, server.URL, appKey, appSecret, licenseKey, "1.0.0", true)
	if !check.HasUpdate || check.Latest != "1.2.0" || check.Version != "1.2.0" || !check.ForceUpdate {
		t.Fatalf("forced downstream update was not propagated: %#v", check)
	}
	if check.FileMD5 != packageMD5 || check.FileSizeBytes != int64(len(packageContent)) || check.DownloadURL == "" {
		t.Fatalf("unexpected version check package metadata: %#v", check)
	}
	if len(check.Updates) != 3 {
		t.Fatalf("updates length = %d, want 3: %#v", len(check.Updates), check.Updates)
	}
	versions := make([]string, 0, len(check.Updates))
	for _, update := range check.Updates {
		versions = append(versions, update.Version)
	}
	if strings.Join(versions, ",") != "1.1.0,1.1.5,1.2.0" {
		t.Fatalf("updates are not semantically sorted: %v", versions)
	}

	badCheck := checkIntegrationVersion(t, client, server.URL, appKey, appSecret+"-wrong", licenseKey, "1.0.0", false)
	if badCheck.HasUpdate {
		t.Fatalf("invalid signature unexpectedly returned update data: %#v", badCheck)
	}

	downloadResponse, err := client.Get(server.URL + check.DownloadURL)
	if err != nil {
		t.Fatalf("download licensed package: %v", err)
	}
	downloaded, readErr := io.ReadAll(downloadResponse.Body)
	downloadResponse.Body.Close()
	if readErr != nil || downloadResponse.StatusCode != http.StatusOK || !bytes.Equal(downloaded, packageContent) {
		t.Fatalf("licensed download failed: status=%d error=%v content=%q", downloadResponse.StatusCode, readErr, downloaded)
	}

	adminEnvelope := integrationJSONRequest(t, client, http.MethodPost, editURL+"/download-url", nil, "")
	if adminEnvelope.Code != 404 {
		t.Fatalf("external package admin download code = %d, want 404", adminEnvelope.Code)
	}
	adminEnvelope = integrationJSONRequest(t, client, http.MethodPost, appURL+"/"+strconv.FormatInt(latestID, 10)+"/download-url", nil, "")
	if adminEnvelope.Code != 200 {
		t.Fatalf("create admin download URL: code=%d msg=%q", adminEnvelope.Code, adminEnvelope.Msg)
	}
	var adminData struct {
		DownloadURL string `json:"downloadUrl"`
		ExpiresIn   int    `json:"expiresIn"`
	}
	decodeIntegrationData(t, adminEnvelope.Data, &adminData)
	if adminData.DownloadURL == "" || adminData.ExpiresIn != int(appVersionDownloadTTL.Seconds()) {
		t.Fatalf("unexpected admin download data: %#v", adminData)
	}

	anonymousResponse, err := client.Get(server.URL + "/api/app/version/download")
	if err != nil {
		t.Fatal(err)
	}
	anonymousResponse.Body.Close()
	if anonymousResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("anonymous download status = %d, want 403", anonymousResponse.StatusCode)
	}
	guessedResponse, err := client.Get(server.URL + "/api/app/version/" + strconv.FormatInt(latestID, 10) + "/download")
	if err != nil {
		t.Fatal(err)
	}
	guessedResponse.Body.Close()
	if guessedResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("legacy enumerable download status = %d, want 404", guessedResponse.StatusCode)
	}

	latest = findIntegrationVersion(t, listIntegrationVersions(t, client, appURL).List, latestID)
	lowerMinimumBody, lowerMinimumType := buildIntegrationMultipart(t, map[string]string{
		"version": latest.Version, "title": latest.Title, "changelog": "minimum policy lowered",
		"updateSql":  "ALTER TABLE integration_demo ADD COLUMN second_step INT;",
		"sourceType": "upload", "forceUpdate": "false", "minVersion": "1.0.0",
		"revision": strconv.FormatInt(latest.Revision, 10),
	}, "", nil)
	lowerEnvelope := integrationJSONRequest(t, client, http.MethodPut, appURL+"/"+strconv.FormatInt(latestID, 10), lowerMinimumBody, lowerMinimumType)
	if lowerEnvelope.Code != 200 {
		t.Fatalf("lower minimum version: code=%d msg=%q", lowerEnvelope.Code, lowerEnvelope.Msg)
	}
	lowered := checkIntegrationVersion(t, client, server.URL, appKey, appSecret, licenseKey, "1.0.5", true)
	if lowered.ForceUpdate {
		t.Fatalf("historical minimum version still forces update: %#v", lowered)
	}
	upToDate := checkIntegrationVersion(t, client, server.URL, appKey, appSecret, licenseKey, "1.2.0", true)
	if upToDate.HasUpdate || upToDate.Latest != "1.2.0" {
		t.Fatalf("up-to-date response is incorrect: %#v", upToDate)
	}
	if len(upToDate.Updates) != 1 || upToDate.Updates[0].Version != "1.2.0" {
		t.Fatalf("up-to-date response should include current version record: %#v", upToDate.Updates)
	}

	if _, err := db.Exec(`UPDATE licenses SET status = 'revoked' WHERE id = ?`, licenseID); err != nil {
		t.Fatalf("revoke test license: %v", err)
	}
	revokedResponse, err := client.Get(server.URL + check.DownloadURL)
	if err != nil {
		t.Fatal(err)
	}
	revokedResponse.Body.Close()
	if revokedResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("revoked license download status = %d, want 403", revokedResponse.StatusCode)
	}

	deleteEnvelope := integrationJSONRequest(t, client, http.MethodDelete, appURL+"/"+strconv.FormatInt(latestID, 10), nil, "")
	if deleteEnvelope.Code != 200 {
		t.Fatalf("delete uploaded release: code=%d msg=%q", deleteEnvelope.Code, deleteEnvelope.Msg)
	}
	if _, err := os.Stat(fullPackagePath); !os.IsNotExist(err) {
		t.Fatalf("package file still exists after delete: %v", err)
	}
}

func assertAppVersionMigration(t *testing.T, db *sql.DB) {
	t.Helper()
	var revisionColumns int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'app_versions' AND COLUMN_NAME = 'revision'
	`).Scan(&revisionColumns); err != nil || revisionColumns != 1 {
		t.Fatalf("revision migration missing: count=%d error=%v", revisionColumns, err)
	}
	var nullable string
	if err := db.QueryRow(`
		SELECT IS_NULLABLE FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'app_versions' AND COLUMN_NAME = 'from_version_norm'
	`).Scan(&nullable); err != nil || nullable != "YES" {
		t.Fatalf("from_version_norm nullable migration missing: nullable=%q error=%v", nullable, err)
	}
	var currentIndex, retiredIndex int
	if err := db.QueryRow(`
		SELECT COUNT(DISTINCT INDEX_NAME) FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'app_versions' AND INDEX_NAME = 'uk_app_version_norm'
	`).Scan(&currentIndex); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`
		SELECT COUNT(DISTINCT INDEX_NAME) FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'app_versions' AND INDEX_NAME = 'uk_app_from_version_norm'
	`).Scan(&retiredIndex); err != nil {
		t.Fatal(err)
	}
	if currentIndex != 1 || retiredIndex != 0 {
		t.Fatalf("unexpected migration indexes: current=%d retired=%d", currentIndex, retiredIndex)
	}
}

func createIntegrationVersion(t *testing.T, client *http.Client, endpoint string, fields map[string]string, fileName string, file []byte) int64 {
	t.Helper()
	body, contentType := buildIntegrationMultipart(t, fields, fileName, file)
	envelope := integrationJSONRequest(t, client, http.MethodPost, endpoint, body, contentType)
	if envelope.Code != 200 {
		t.Fatalf("create version: code=%d msg=%q", envelope.Code, envelope.Msg)
	}
	var data struct {
		ID int64 `json:"id"`
	}
	decodeIntegrationData(t, envelope.Data, &data)
	if data.ID <= 0 {
		t.Fatalf("invalid created version id: %d", data.ID)
	}
	return data.ID
}

func listIntegrationVersions(t *testing.T, client *http.Client, endpoint string) appVersionIntegrationList {
	t.Helper()
	envelope := integrationJSONRequest(t, client, http.MethodGet, endpoint+"?page=1&pageSize=20", nil, "")
	if envelope.Code != 200 {
		t.Fatalf("list versions: code=%d msg=%q", envelope.Code, envelope.Msg)
	}
	var data appVersionIntegrationList
	decodeIntegrationData(t, envelope.Data, &data)
	return data
}

func findIntegrationVersion(t *testing.T, list []appVersionIntegrationItem, id int64) appVersionIntegrationItem {
	t.Helper()
	for _, item := range list {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("version id %d not found in %#v", id, list)
	return appVersionIntegrationItem{}
}

func queryIntegrationPackagePath(t *testing.T, db *sql.DB, versionID int64) string {
	t.Helper()
	var packagePath string
	if err := db.QueryRow(`SELECT package_path FROM app_versions WHERE id = ?`, versionID).Scan(&packagePath); err != nil {
		t.Fatal(err)
	}
	if packagePath == "" {
		t.Fatal("uploaded package path is empty")
	}
	return packagePath
}

func checkIntegrationVersion(t *testing.T, client *http.Client, baseURL, appKey, appSecret, licenseKey, currentVersion string, wantSuccess bool) appVersionIntegrationCheck {
	t.Helper()
	timestamp := time.Now().Unix()
	canonical := strings.Join([]string{appKey, currentVersion, licenseKey, strconv.FormatInt(timestamp, 10)}, "\n")
	mac := hmac.New(sha256.New, []byte(appSecret))
	_, _ = mac.Write([]byte(canonical))
	payload, err := json.Marshal(map[string]any{
		"appKey": appKey, "currentVersion": currentVersion, "licenseKey": licenseKey,
		"timestamp": timestamp, "sign": hex.EncodeToString(mac.Sum(nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	envelope := integrationJSONRequest(t, client, http.MethodPost, baseURL+"/api/app/version/check", payload, "application/json")
	if wantSuccess && envelope.Code != 200 {
		t.Fatalf("version check: code=%d msg=%q", envelope.Code, envelope.Msg)
	}
	if !wantSuccess {
		if envelope.Code != 403 {
			t.Fatalf("invalid version check code=%d, want 403", envelope.Code)
		}
		return appVersionIntegrationCheck{}
	}
	var raw map[string]json.RawMessage
	decodeIntegrationData(t, envelope.Data, &raw)
	if _, exists := raw["updateSql"]; exists {
		t.Fatal("top-level updateSql must not duplicate the updates migration list")
	}
	var data appVersionIntegrationCheck
	decodeIntegrationData(t, envelope.Data, &data)
	return data
}

func buildIntegrationMultipart(t *testing.T, fields map[string]string, fileName string, file []byte) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := writer.WriteField(key, fields[key]); err != nil {
			t.Fatal(err)
		}
	}
	if fileName != "" {
		part, err := writer.CreateFormFile("package", fileName)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(file); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

func concurrentIntegrationRequests(t *testing.T, client *http.Client, method, endpoint string, body []byte, contentType string) []int {
	t.Helper()
	start := make(chan struct{})
	codes := make(chan int, 2)
	errors := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			envelope, err := integrationJSONRequestResult(client, method, endpoint, body, contentType)
			if err != nil {
				errors <- err
				return
			}
			codes <- envelope.Code
		}()
	}
	close(start)
	wait.Wait()
	close(codes)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	result := make([]int, 0, 2)
	for code := range codes {
		result = append(result, code)
	}
	sort.Ints(result)
	return result
}

func assertIntegrationCodes(t *testing.T, got []int, want ...int) {
	t.Helper()
	sort.Ints(want)
	if len(got) != len(want) {
		t.Fatalf("response codes = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("response codes = %v, want %v", got, want)
		}
	}
}

func integrationJSONRequest(t *testing.T, client *http.Client, method, endpoint string, body []byte, contentType string) appVersionIntegrationEnvelope {
	t.Helper()
	envelope, err := integrationJSONRequestResult(client, method, endpoint, body, contentType)
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

func integrationJSONRequestResult(client *http.Client, method, endpoint string, body []byte, contentType string) (appVersionIntegrationEnvelope, error) {
	var envelope appVersionIntegrationEnvelope
	req, err := http.NewRequest(method, endpoint, bytes.NewReader(body))
	if err != nil {
		return envelope, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	response, err := client.Do(req)
	if err != nil {
		return envelope, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return envelope, err
	}
	if response.StatusCode != http.StatusOK {
		return envelope, fmt.Errorf("HTTP status %d: %s", response.StatusCode, payload)
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return envelope, fmt.Errorf("decode response %q: %w", payload, err)
	}
	return envelope, nil
}

func decodeIntegrationData(t *testing.T, raw json.RawMessage, target any) {
	t.Helper()
	if len(raw) == 0 {
		t.Fatal("response data is empty")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("decode response data %q: %v", raw, err)
	}
}
