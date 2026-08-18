package handler

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
)

const (
	maxOnlineUpdatePackageSize = int64(512 << 20)
	githubUpdateRepositoryPath = "/cy70923167/auth_pro/releases/"
)

var onlineUpdateVersionPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

type onlineUpdatePackage struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	FileName  string `json:"fileName"`
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
	Signature string `json:"signature"`
}

type onlineUpdateActions struct {
	UpdateFrontend bool `json:"updateFrontend"`
	UpdateBackend  bool `json:"updateBackend"`
	RestartBackend bool `json:"restartBackend"`
	BackupDatabase bool `json:"backupDatabase"`
}

type onlineUpdateManifest struct {
	Version     string              `json:"version"`
	Channel     string              `json:"channel"`
	MinVersion  string              `json:"minVersion"`
	Force       bool                `json:"force"`
	ReleasedAt  string              `json:"releasedAt"`
	ReleasesURL string              `json:"releasesUrl"`
	Package     onlineUpdatePackage `json:"package"`
	Actions     onlineUpdateActions `json:"actions"`
	Notes       []string            `json:"notes"`

	// 兼容极简版 latest.json。
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type onlineUpdateRelease struct {
	Version    string   `json:"version"`
	Channel    string   `json:"channel"`
	ReleasedAt string   `json:"releasedAt"`
	Notes      []string `json:"notes"`
}

type onlineUpdateReleases struct {
	Releases []onlineUpdateRelease `json:"releases"`
}

type cachedOnlineUpdateReleases struct {
	URL       string
	Releases  []onlineUpdateRelease
	ExpiresAt time.Time
}

type onlineUpdateJob struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Progress  int       `json:"progress"`
	Version   string    `json:"version"`
	Logs      []string  `json:"logs"`
	Error     string    `json:"error"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type onlineUpdateStore struct {
	mu        sync.Mutex
	jobs      map[string]*onlineUpdateJob
	runningID string
	latest    *onlineUpdateManifest
}

var updateStore = &onlineUpdateStore{jobs: make(map[string]*onlineUpdateJob)}

var updateReleasesCache struct {
	mu    sync.Mutex
	value cachedOnlineUpdateReleases
}

// SystemVersion 返回当前系统整体版本。健康检查脚本也会访问它。
func SystemVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"version":   config.AppVersion,
			"buildTime": config.BuildTime,
			"os":        runtime.GOOS,
			"arch":      runtime.GOARCH,
		},
	})
}

// AdminOnlineUpdateStatus 返回当前版本、更新地址、最近一次清单和任务状态。
func AdminOnlineUpdateStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"currentVersion": config.AppVersion,
			"buildTime":      config.BuildTime,
			"updateUrl":      config.GetUpdateManifestURL(),
			"frontendDir":    config.GetFrontendDir(),
			"serviceName":    config.GetServiceName(),
			"latest":         cachedOnlineUpdateManifest(),
			"runningJob":     runningOnlineUpdateJob(),
		},
	})
}

// AdminOnlineUpdateCheck 拉取 latest.json 并检查版本和更新包元数据。
func AdminOnlineUpdateCheck(c *gin.Context) {
	manifest, err := fetchOnlineUpdateManifest()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "检查更新失败：" + err.Error()})
		return
	}

	setCachedOnlineUpdateManifest(manifest)
	packageErr := validateOnlineUpdateManifest(manifest)
	available, versionErr := onlineUpdateAvailable(config.AppVersion, manifest)

	data := gin.H{
		"currentVersion": config.AppVersion,
		"latest":         manifest,
		"updateUrl":      config.GetUpdateManifestURL(),
		"canApply":       packageErr == nil && available && versionErr == "",
		"packageValid":   packageErr == nil,
		"packageError":   errorText(packageErr),
		"versionError":   versionErr,
	}
	if available {
		data["updateAvailable"] = true
	} else {
		data["updateAvailable"] = false
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": data})
}

// AdminOnlineUpdateHistory 返回只读历史版本和每版更新日志。
func AdminOnlineUpdateHistory(c *gin.Context) {
	manifest := cachedOnlineUpdateManifest()
	if manifest == nil {
		var err error
		manifest, err = fetchOnlineUpdateManifest()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取历史版本失败：" + err.Error()})
			return
		}
		setCachedOnlineUpdateManifest(manifest)
	}

	releases, releasesURL, err := fetchOnlineUpdateReleases(manifest, c.Query("refresh") == "1")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取历史版本失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "",
		"data": gin.H{
			"currentVersion": config.AppVersion,
			"releasesUrl":    releasesURL,
			"releases":       releases,
		},
	})
}

// AdminOnlineUpdateApply 创建一键整包更新任务。
func AdminOnlineUpdateApply(c *gin.Context) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "一键整包更新仅支持 Linux amd64 宝塔部署环境"})
		return
	}

	manifest, err := fetchOnlineUpdateManifest()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "读取更新清单失败：" + err.Error()})
		return
	}
	if err := validateOnlineUpdateManifest(manifest); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "更新包信息不完整：" + err.Error()})
		return
	}
	available, versionErr := onlineUpdateAvailable(config.AppVersion, manifest)
	if versionErr != "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": versionErr})
		return
	}
	if !available {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "当前已经是最新版本"})
		return
	}
	if !reserveOnlineUpdateJob() {
		c.JSON(http.StatusOK, gin.H{"code": 409, "msg": "已有更新任务正在执行"})
		return
	}

	job := createOnlineUpdateJob(manifest.Version)
	appendOnlineUpdateLog(job.ID, "更新任务已创建")
	go runOnlineUpdateJob(job.ID, *manifest)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "更新任务已启动，服务即将重启，请稍后刷新页面",
		"data": snapshotOnlineUpdateJob(job.ID),
	})
}

// AdminOnlineUpdateJob 查询更新任务状态。
func AdminOnlineUpdateJob(c *gin.Context) {
	job := snapshotOnlineUpdateJob(c.Param("id"))
	if job == nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "更新任务不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "", "data": job})
}

func parseOnlineUpdateURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || parsed.Scheme != "https" || parsed.User != nil {
		return nil, errors.New("更新地址必须是有效的 HTTPS 地址")
	}
	if strings.EqualFold(parsed.Hostname(), "github.com") {
		if parsed.Port() != "" && parsed.Port() != "443" {
			return nil, errors.New("GitHub 更新地址只能使用 HTTPS 默认端口")
		}
		if !isGitHubRepositoryReleaseURL(parsed) {
			return nil, errors.New("GitHub 更新地址不属于受信任的发布仓库")
		}
	}
	return parsed, nil
}

func isGitHubRepositoryReleaseURL(parsed *url.URL) bool {
	return parsed != nil && strings.EqualFold(parsed.Hostname(), "github.com") &&
		strings.HasPrefix(strings.ToLower(parsed.EscapedPath()), githubUpdateRepositoryPath)
}

func isGitHubReleaseAssetHost(hostname string) bool {
	return strings.EqualFold(hostname, "release-assets.githubusercontent.com")
}

func newOnlineUpdateHTTPClient(rawURL string, timeout time.Duration) (*http.Client, error) {
	initialURL, err := parseOnlineUpdateURL(rawURL)
	if err != nil {
		return nil, err
	}
	githubRelease := isGitHubRepositoryReleaseURL(initialURL)
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("更新地址重定向次数过多")
			}
			if req.URL.Scheme != "https" || req.URL.User != nil {
				return errors.New("更新地址重定向到非 HTTPS 地址")
			}
			if githubRelease {
				if req.URL.Port() != "" && req.URL.Port() != "443" {
					return errors.New("GitHub 更新地址重定向到非 HTTPS 默认端口")
				}
				if isGitHubRepositoryReleaseURL(req.URL) || isGitHubReleaseAssetHost(req.URL.Hostname()) {
					return nil
				}
				return errors.New("GitHub 更新地址重定向到非受信任域名")
			}
			if !strings.EqualFold(req.URL.Host, initialURL.Host) {
				return errors.New("更新地址重定向到非同源地址")
			}
			return nil
		},
	}
	return client, nil
}

func fetchOnlineUpdateManifest() (*onlineUpdateManifest, error) {
	manifestURL := strings.TrimSpace(config.GetUpdateManifestURL())
	if _, err := parseOnlineUpdateURL(manifestURL); err != nil {
		return nil, errors.New("更新清单地址格式不正确：" + err.Error())
	}

	client, err := newOnlineUpdateHTTPClient(manifestURL, 15*time.Second)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "auth_pro-updater/"+config.AppVersion)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接更新服务器失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("更新服务器返回状态码 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, errors.New("读取更新清单失败")
	}
	var manifest onlineUpdateManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, errors.New("更新清单不是有效的 JSON")
	}
	normalizeOnlineUpdateManifest(&manifest)
	return &manifest, nil
}

func fetchOnlineUpdateReleases(manifest *onlineUpdateManifest, forceRefresh bool) ([]onlineUpdateRelease, string, error) {
	releasesURL, err := resolveOnlineUpdateReleasesURL(manifest)
	if err != nil {
		return nil, "", err
	}

	updateReleasesCache.mu.Lock()
	cached := updateReleasesCache.value
	if !forceRefresh && cached.URL == releasesURL && time.Now().Before(cached.ExpiresAt) {
		releases := cloneOnlineUpdateReleases(cached.Releases)
		updateReleasesCache.mu.Unlock()
		return releases, releasesURL, nil
	}
	updateReleasesCache.mu.Unlock()

	client, err := newOnlineUpdateHTTPClient(releasesURL, 10*time.Second)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequest(http.MethodGet, releasesURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "auth_pro-updater/"+config.AppVersion)

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("连接更新服务器失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("更新服务器返回状态码 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return nil, "", errors.New("读取历史版本清单失败")
	}
	if len(body) > 1<<20 {
		return nil, "", errors.New("历史版本清单超过 1MB 限制")
	}
	var payload onlineUpdateReleases
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, "", errors.New("历史版本清单不是有效的 JSON")
	}
	if err := normalizeOnlineUpdateReleases(&payload); err != nil {
		return nil, "", err
	}

	updateReleasesCache.mu.Lock()
	updateReleasesCache.value = cachedOnlineUpdateReleases{
		URL:       releasesURL,
		Releases:  cloneOnlineUpdateReleases(payload.Releases),
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	updateReleasesCache.mu.Unlock()
	return payload.Releases, releasesURL, nil
}

func resolveOnlineUpdateReleasesURL(manifest *onlineUpdateManifest) (string, error) {
	manifestURL, err := parseOnlineUpdateURL(config.GetUpdateManifestURL())
	if err != nil {
		return "", errors.New("更新清单地址格式不正确：" + err.Error())
	}

	releasesRef := strings.TrimSpace(manifest.ReleasesURL)
	if releasesRef == "" {
		releasesRef = "releases.json"
	}
	parsedRef, err := url.Parse(releasesRef)
	if err != nil {
		return "", errors.New("历史版本清单地址格式不正确")
	}
	releasesURL := manifestURL.ResolveReference(parsedRef)
	if _, err := parseOnlineUpdateURL(releasesURL.String()); err != nil {
		return "", errors.New("历史版本清单地址格式不正确：" + err.Error())
	}
	if releasesURL.Scheme != manifestURL.Scheme || !strings.EqualFold(releasesURL.Host, manifestURL.Host) {
		return "", errors.New("历史版本清单必须与更新清单同源")
	}
	return releasesURL.String(), nil
}

func normalizeOnlineUpdateReleases(payload *onlineUpdateReleases) error {
	if len(payload.Releases) > 200 {
		return errors.New("历史版本数量超过 200 条限制")
	}

	seen := make(map[string]struct{}, len(payload.Releases))
	normalized := make([]onlineUpdateRelease, 0, len(payload.Releases))
	for index := range payload.Releases {
		release := payload.Releases[index]
		release.Version = strings.TrimSpace(release.Version)
		release.Channel = strings.TrimSpace(release.Channel)
		release.ReleasedAt = strings.TrimSpace(release.ReleasedAt)
		if _, ok := parseOnlineUpdateVersion(release.Version); !ok {
			return fmt.Errorf("历史版本第 %d 条版本号格式不正确", index+1)
		}
		if release.ReleasedAt != "" {
			if _, err := time.Parse(time.RFC3339, release.ReleasedAt); err != nil {
				return fmt.Errorf("历史版本 %s 发布时间格式不正确", release.Version)
			}
		}
		if release.Channel == "" {
			release.Channel = "stable"
		}
		if _, exists := seen[release.Version]; exists {
			continue
		}
		seen[release.Version] = struct{}{}
		notes := make([]string, 0, len(release.Notes))
		for _, note := range release.Notes {
			note = strings.TrimSpace(note)
			if note != "" {
				notes = append(notes, note)
			}
		}
		release.Notes = notes
		normalized = append(normalized, release)
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		comparison, ok := compareOnlineUpdateVersions(normalized[i].Version, normalized[j].Version)
		return ok && comparison > 0
	})
	payload.Releases = normalized
	return nil
}

func cloneOnlineUpdateReleases(releases []onlineUpdateRelease) []onlineUpdateRelease {
	cloned := make([]onlineUpdateRelease, len(releases))
	for index, release := range releases {
		cloned[index] = release
		cloned[index].Notes = append([]string(nil), release.Notes...)
	}
	return cloned
}

func normalizeOnlineUpdateManifest(manifest *onlineUpdateManifest) {
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Channel = strings.TrimSpace(manifest.Channel)
	manifest.MinVersion = strings.TrimSpace(manifest.MinVersion)
	manifest.ReleasesURL = strings.TrimSpace(manifest.ReleasesURL)
	manifest.Package.OS = strings.ToLower(strings.TrimSpace(manifest.Package.OS))
	manifest.Package.Arch = strings.ToLower(strings.TrimSpace(manifest.Package.Arch))
	manifest.Package.FileName = strings.TrimSpace(manifest.Package.FileName)
	manifest.Package.URL = strings.TrimSpace(manifest.Package.URL)
	manifest.Package.SHA256 = strings.ToLower(strings.TrimSpace(manifest.Package.SHA256))
	manifest.Package.Signature = strings.TrimSpace(manifest.Package.Signature)

	if manifest.Package.URL == "" {
		manifest.Package.URL = strings.TrimSpace(manifest.URL)
	}
	if manifest.Package.SHA256 == "" {
		manifest.Package.SHA256 = strings.ToLower(strings.TrimSpace(manifest.SHA256))
	}
	if manifest.Package.Size == 0 {
		manifest.Package.Size = manifest.Size
	}
	if manifest.Channel == "" {
		manifest.Channel = "stable"
	}
	if !manifest.Actions.UpdateFrontend && !manifest.Actions.UpdateBackend &&
		!manifest.Actions.RestartBackend && !manifest.Actions.BackupDatabase {
		manifest.Actions = onlineUpdateActions{
			UpdateFrontend: true,
			UpdateBackend:  true,
			RestartBackend: true,
			BackupDatabase: true,
		}
	}
}

func validateOnlineUpdateManifest(manifest *onlineUpdateManifest) error {
	if _, ok := parseOnlineUpdateVersion(manifest.Version); !ok {
		return errors.New("版本号格式不正确")
	}
	if manifest.Package.OS != "" && manifest.Package.OS != runtime.GOOS {
		return fmt.Errorf("更新包系统 %s 与当前系统 %s 不兼容", manifest.Package.OS, runtime.GOOS)
	}
	if manifest.Package.Arch != "" && manifest.Package.Arch != runtime.GOARCH {
		return fmt.Errorf("更新包架构 %s 与当前架构 %s 不兼容", manifest.Package.Arch, runtime.GOARCH)
	}
	parsed, err := parseOnlineUpdateURL(manifest.Package.URL)
	if err != nil {
		return errors.New("更新包下载地址不正确：" + err.Error())
	}
	manifestSource, sourceErr := parseOnlineUpdateURL(config.GetUpdateManifestURL())
	if sourceErr == nil && isGitHubRepositoryReleaseURL(manifestSource) && !isGitHubRepositoryReleaseURL(parsed) {
		return errors.New("GitHub 更新包地址不属于受信任的发布仓库")
	}
	if strings.Contains(strings.ToLower(parsed.Host), "your-domain.com") {
		return errors.New("更新包下载地址仍是示例地址")
	}
	if !isHexSHA256(manifest.Package.SHA256) {
		return errors.New("更新包 SHA256 未配置或格式不正确")
	}
	if manifest.Package.Size <= 0 {
		return errors.New("更新包大小未配置")
	}
	if manifest.Package.Size > maxOnlineUpdatePackageSize {
		return errors.New("更新包超过 512MB 限制")
	}
	return nil
}

func onlineUpdateAvailable(currentVersion string, manifest *onlineUpdateManifest) (bool, string) {
	comparison, ok := compareOnlineUpdateVersions(currentVersion, manifest.Version)
	if !ok {
		return false, "当前版本或最新版本格式不正确"
	}
	if manifest.MinVersion != "" {
		minComparison, minOK := compareOnlineUpdateVersions(currentVersion, manifest.MinVersion)
		if minOK && minComparison < 0 {
			return false, "当前版本过低，不能直接升级到该版本"
		}
	}
	return comparison < 0, ""
}

func compareOnlineUpdateVersions(left, right string) (int, bool) {
	leftParts, leftOK := parseOnlineUpdateVersion(left)
	rightParts, rightOK := parseOnlineUpdateVersion(right)
	if !leftOK || !rightOK {
		return 0, false
	}
	length := len(leftParts)
	if len(rightParts) > length {
		length = len(rightParts)
	}
	for i := 0; i < length; i++ {
		var l, r int
		if i < len(leftParts) {
			l = leftParts[i]
		}
		if i < len(rightParts) {
			r = rightParts[i]
		}
		if l < r {
			return -1, true
		}
		if l > r {
			return 1, true
		}
	}
	return 0, true
}

func parseOnlineUpdateVersion(value string) ([]int, bool) {
	matches := onlineUpdateVersionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 4 {
		return nil, false
	}
	parts := make([]int, 0, 3)
	for _, item := range matches[1:] {
		part, err := strconv.Atoi(item)
		if err != nil {
			return nil, false
		}
		parts = append(parts, part)
	}
	return parts, true
}

func isHexSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func reserveOnlineUpdateJob() bool {
	updateStore.mu.Lock()
	defer updateStore.mu.Unlock()
	if updateStore.runningID != "" {
		return false
	}
	updateStore.runningID = "reserved"
	return true
}

func createOnlineUpdateJob(version string) *onlineUpdateJob {
	now := time.Now()
	job := &onlineUpdateJob{
		ID:        fmt.Sprintf("U%d", now.UnixNano()),
		Status:    "running",
		Message:   "更新任务已启动",
		Progress:  0,
		Version:   version,
		Logs:      []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	updateStore.mu.Lock()
	updateStore.jobs[job.ID] = job
	updateStore.runningID = job.ID
	updateStore.mu.Unlock()
	persistOnlineUpdateJob(job)
	return job
}

func finishOnlineUpdateJob(id string, status string, message string, err error) {
	updateStore.mu.Lock()
	job, ok := updateStore.jobs[id]
	if !ok {
		updateStore.mu.Unlock()
		return
	}
	job.Status = status
	job.Message = message
	job.UpdatedAt = time.Now()
	if status == "restarting" && job.Progress < 95 {
		job.Progress = 95
	}
	if status == "success" {
		job.Progress = 100
	}
	if err != nil {
		job.Error = err.Error()
	}
	if status != "running" && status != "restarting" && updateStore.runningID == id {
		updateStore.runningID = ""
	}
	copyJob := cloneOnlineUpdateJob(job)
	updateStore.mu.Unlock()
	persistOnlineUpdateJob(copyJob)
}

func appendOnlineUpdateLog(id string, message string) {
	updateStore.mu.Lock()
	job, ok := updateStore.jobs[id]
	if !ok {
		updateStore.mu.Unlock()
		return
	}
	job.Logs = append(job.Logs, fmt.Sprintf("%s %s", time.Now().Format("15:04:05"), message))
	job.Message = message
	job.UpdatedAt = time.Now()
	if len(job.Logs) > 200 {
		job.Logs = job.Logs[len(job.Logs)-200:]
	}
	copyJob := cloneOnlineUpdateJob(job)
	updateStore.mu.Unlock()
	persistOnlineUpdateJob(copyJob)
}

func updateOnlineUpdateProgress(id string, progress int, message string) {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	updateStore.mu.Lock()
	job, ok := updateStore.jobs[id]
	if !ok || progress < job.Progress {
		updateStore.mu.Unlock()
		return
	}
	if progress == job.Progress && (message == "" || message == job.Message) {
		updateStore.mu.Unlock()
		return
	}
	job.Progress = progress
	if message != "" {
		job.Message = message
	}
	job.UpdatedAt = time.Now()
	copyJob := cloneOnlineUpdateJob(job)
	updateStore.mu.Unlock()
	persistOnlineUpdateJob(copyJob)
}

func snapshotOnlineUpdateJob(id string) *onlineUpdateJob {
	updateStore.mu.Lock()
	job, ok := updateStore.jobs[id]
	if ok {
		job = cloneOnlineUpdateJob(job)
	}
	updateStore.mu.Unlock()
	if ok {
		return job
	}
	return loadOnlineUpdateJob(id)
}

func cloneOnlineUpdateJob(job *onlineUpdateJob) *onlineUpdateJob {
	if job == nil {
		return nil
	}
	copyJob := *job
	copyJob.Logs = append([]string{}, job.Logs...)
	return &copyJob
}

func onlineUpdateJobStatePath(id string) string {
	return filepath.Join(config.GetUpdateDir(), id+".json")
}

func persistOnlineUpdateJob(job *onlineUpdateJob) {
	if job == nil {
		return
	}
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return
	}
	path := onlineUpdateJobStatePath(job.ID)
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return
	}
	_ = os.Rename(tempPath, path)
}

func loadOnlineUpdateJob(id string) *onlineUpdateJob {
	if id == "" || filepath.Base(id) != id {
		return nil
	}
	data, err := os.ReadFile(onlineUpdateJobStatePath(id))
	if err != nil {
		return nil
	}
	var job onlineUpdateJob
	if err := json.Unmarshal(data, &job); err != nil || job.ID != id {
		return nil
	}
	return reconcileOnlineUpdateJobResult(&job)
}

func reconcileOnlineUpdateJobResult(job *onlineUpdateJob) *onlineUpdateJob {
	data, err := os.ReadFile(onlineUpdateJobStatePath(job.ID) + ".result")
	if err != nil {
		return job
	}
	result := strings.TrimSpace(string(data))
	if result != "success" && result != "failed" {
		return job
	}
	if job.Status == result {
		return job
	}
	job.Status = result
	job.Error = ""
	if result == "success" {
		job.Message = "更新完成"
		job.Progress = 100
		job.Logs = append(job.Logs, fmt.Sprintf("%s 前端和后端已切换到 v%s", time.Now().Format("15:04:05"), job.Version))
	} else {
		job.Message = "更新失败，已尝试回滚"
		job.Error = "新版本健康检查失败或更新脚本执行异常"
		job.Logs = append(job.Logs, fmt.Sprintf("%s 更新失败，已尝试回滚", time.Now().Format("15:04:05")))
	}
	job.UpdatedAt = time.Now()
	persistOnlineUpdateJob(job)
	return job
}

func runningOnlineUpdateJob() *onlineUpdateJob {
	updateStore.mu.Lock()
	id := updateStore.runningID
	updateStore.mu.Unlock()
	if id == "" || id == "reserved" {
		return nil
	}
	return snapshotOnlineUpdateJob(id)
}

func setCachedOnlineUpdateManifest(manifest *onlineUpdateManifest) {
	updateStore.mu.Lock()
	copyManifest := *manifest
	copyManifest.Notes = append([]string{}, manifest.Notes...)
	updateStore.latest = &copyManifest
	updateStore.mu.Unlock()
}

func cachedOnlineUpdateManifest() *onlineUpdateManifest {
	updateStore.mu.Lock()
	defer updateStore.mu.Unlock()
	if updateStore.latest == nil {
		return nil
	}
	copyManifest := *updateStore.latest
	copyManifest.Notes = append([]string{}, updateStore.latest.Notes...)
	return &copyManifest
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func runOnlineUpdateJob(jobID string, manifest onlineUpdateManifest) {
	if err := executeOnlineUpdate(jobID, &manifest); err != nil {
		appendOnlineUpdateLog(jobID, "更新失败："+err.Error())
		finishOnlineUpdateJob(jobID, "failed", "更新失败", err)
	}
}

func executeOnlineUpdate(jobID string, manifest *onlineUpdateManifest) error {
	if !manifest.Actions.UpdateFrontend || !manifest.Actions.UpdateBackend {
		return errors.New("方案二要求更新包同时包含前端和后端")
	}
	if manifest.Package.Signature != "" {
		appendOnlineUpdateLog(jobID, "检测到签名字段，当前版本使用 SHA256 校验")
	}

	updateOnlineUpdateProgress(jobID, 5, "正在准备更新")
	appendOnlineUpdateLog(jobID, "开始下载更新包")
	updateOnlineUpdateProgress(jobID, 10, "开始下载更新包")
	packagePath, err := downloadOnlineUpdatePackage(jobID, manifest)
	if err != nil {
		return err
	}
	appendOnlineUpdateLog(jobID, "更新包下载完成")
	updateOnlineUpdateProgress(jobID, 50, "更新包下载完成")

	appendOnlineUpdateLog(jobID, "开始解压更新包")
	updateOnlineUpdateProgress(jobID, 55, "开始解压更新包")
	stagingDir, err := extractOnlineUpdatePackage(jobID, packagePath)
	if err != nil {
		return err
	}
	appendOnlineUpdateLog(jobID, "更新包解压完成")
	updateOnlineUpdateProgress(jobID, 65, "更新包解压完成")

	pkg, err := validateExtractedOnlineUpdatePackage(stagingDir, manifest.Version)
	if err != nil {
		return err
	}
	appendOnlineUpdateLog(jobID, "更新包结构校验通过")
	updateOnlineUpdateProgress(jobID, 72, "更新包结构校验通过")

	if manifest.Actions.BackupDatabase {
		updateOnlineUpdateProgress(jobID, 76, "正在备份数据库")
		if err := backupDatabaseForOnlineUpdate(jobID); err != nil {
			return err
		}
		updateOnlineUpdateProgress(jobID, 82, "数据库备份完成")
	}

	frontendDir := config.GetFrontendDir()
	if info, err := os.Stat(frontendDir); err != nil || !info.IsDir() {
		return fmt.Errorf("未找到前端目录 %s，请设置 AUTO_PRO_FRONTEND_DIR", frontendDir)
	}

	updateOnlineUpdateProgress(jobID, 86, "正在准备新版本文件")
	frontendSource, err := prepareOnlineUpdateFrontendSource(stagingDir, pkg)
	if err != nil {
		return err
	}

	scriptPath, err := writeOnlineUpdateScript(jobID, stagingDir, pkg, frontendSource, manifest.Version, frontendDir)
	if err != nil {
		return err
	}
	appendOnlineUpdateLog(jobID, "更新脚本已生成，准备重启服务")
	updateOnlineUpdateProgress(jobID, 92, "更新脚本已生成，准备重启服务")

	logPath := filepath.Join(config.GetUpdateDir(), jobID+".log")
	logFile, _ := os.Create(logPath)
	if logFile != nil {
		defer logFile.Close()
	}
	cmd := exec.Command("/bin/sh", scriptPath)
	cmd.Dir = config.GetDataDir()
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动更新脚本失败：%w", err)
	}

	finishOnlineUpdateJob(jobID, "restarting", "服务正在切换并重启", nil)
	return nil
}

func downloadOnlineUpdatePackage(jobID string, manifest *onlineUpdateManifest) (string, error) {
	client, err := newOnlineUpdateHTTPClient(manifest.Package.URL, 10*time.Minute)
	if err != nil {
		return "", err
	}
	return downloadOnlineUpdatePackageWithClient(jobID, manifest, client)
}

type onlineUpdateDownloadWriter struct {
	jobID       string
	destination io.Writer
	total       int64
	written     int64
	lastPercent int
}

func (writer *onlineUpdateDownloadWriter) Write(data []byte) (int, error) {
	count, err := writer.destination.Write(data)
	writer.written += int64(count)
	if writer.total <= 0 {
		return count, err
	}

	downloadPercent := int(writer.written * 100 / writer.total)
	if downloadPercent > 100 {
		downloadPercent = 100
	}
	if downloadPercent != writer.lastPercent {
		writer.lastPercent = downloadPercent
		overallProgress := 10 + downloadPercent*40/100
		updateOnlineUpdateProgress(
			writer.jobID,
			overallProgress,
			fmt.Sprintf("正在下载更新包 %d%%", downloadPercent),
		)
	}
	return count, err
}

func downloadOnlineUpdatePackageWithClient(jobID string, manifest *onlineUpdateManifest, client *http.Client) (string, error) {
	fileName := manifest.Package.FileName
	if fileName == "" {
		fileName = fmt.Sprintf("auth_pro-full-v%s.tar.gz", manifest.Version)
	}
	fileName = filepath.Base(fileName)
	target := filepath.Join(config.GetUpdateDir(), jobID+"-"+fileName)

	req, err := http.NewRequest(http.MethodGet, manifest.Package.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "auth_pro-updater/"+config.AppVersion)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载更新包失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("更新包下载地址返回状态码 %d", resp.StatusCode)
	}

	out, err := os.Create(target)
	if err != nil {
		return "", fmt.Errorf("创建更新包文件失败：%w", err)
	}
	defer out.Close()

	hash := sha256.New()
	progressWriter := &onlineUpdateDownloadWriter{
		jobID:       jobID,
		destination: out,
		total:       manifest.Package.Size,
		lastPercent: -1,
	}
	written, err := io.Copy(
		progressWriter,
		io.TeeReader(io.LimitReader(resp.Body, maxOnlineUpdatePackageSize+1), hash),
	)
	if err != nil {
		return "", fmt.Errorf("保存更新包失败：%w", err)
	}
	if written > maxOnlineUpdatePackageSize {
		return "", errors.New("更新包超过 512MB 限制")
	}
	if written != manifest.Package.Size {
		return "", fmt.Errorf("更新包大小不一致，期望 %d 字节，实际 %d 字节", manifest.Package.Size, written)
	}
	actualSHA256 := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actualSHA256, manifest.Package.SHA256) {
		return "", fmt.Errorf("更新包 SHA256 不一致，期望 %s，实际 %s", manifest.Package.SHA256, actualSHA256)
	}
	return target, nil
}

func extractOnlineUpdatePackage(jobID string, packagePath string) (string, error) {
	stagingDir := filepath.Join(config.GetUpdateDir(), jobID, "staging")
	if err := os.RemoveAll(filepath.Dir(stagingDir)); err != nil {
		return "", err
	}
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return "", err
	}

	file, err := os.Open(packagePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", errors.New("更新包不是有效的 tar.gz 文件")
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", errors.New("读取更新包失败")
		}
		name := path.Clean(strings.TrimPrefix(header.Name, "./"))
		if name == "." {
			if header.Typeflag == tar.TypeDir {
				continue
			}
			return "", fmt.Errorf("更新包包含非法路径 %s", header.Name)
		}
		if path.IsAbs(name) || strings.HasPrefix(name, "../") {
			return "", fmt.Errorf("更新包包含非法路径 %s", header.Name)
		}
		target := filepath.Join(stagingDir, filepath.FromSlash(name))
		cleanStaging := filepath.Clean(stagingDir)
		if target != cleanStaging && !strings.HasPrefix(target, cleanStaging+string(os.PathSeparator)) {
			return "", fmt.Errorf("更新包包含越界路径 %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return "", err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return "", err
			}
			_, copyErr := io.Copy(out, tarReader)
			closeErr := out.Close()
			if copyErr != nil {
				return "", copyErr
			}
			if closeErr != nil {
				return "", closeErr
			}
		default:
			return "", fmt.Errorf("更新包包含不支持的文件类型 %s", header.Name)
		}
	}
	return stagingDir, nil
}

type extractedOnlineUpdateManifest struct {
	Version       string   `json:"version"`
	FrontendDir   string   `json:"frontendDir"`
	BackendFile   string   `json:"backendFile"`
	RequiredFiles []string `json:"requiredFiles"`
}

func validateExtractedOnlineUpdatePackage(stagingDir string, expectedVersion string) (*extractedOnlineUpdateManifest, error) {
	manifestPath := filepath.Join(stagingDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, errors.New("更新包缺少 manifest.json")
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	var pkg extractedOnlineUpdateManifest
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, errors.New("更新包 manifest.json 不是有效 JSON")
	}
	if pkg.Version != "" && pkg.Version != expectedVersion {
		return nil, fmt.Errorf("更新包内部版本 %s 与清单版本 %s 不一致", pkg.Version, expectedVersion)
	}
	if pkg.FrontendDir == "" {
		pkg.FrontendDir = "frontend"
	}
	if pkg.BackendFile == "" {
		pkg.BackendFile = "backend/auth_pro"
	}
	if !safeOnlineUpdateRelativePath(pkg.FrontendDir) || !safeOnlineUpdateRelativePath(pkg.BackendFile) {
		return nil, errors.New("更新包 manifest 包含非法路径")
	}
	required := append([]string{
		path.Join(pkg.FrontendDir, "index.html"),
		path.Join(pkg.FrontendDir, "version.json"),
		pkg.BackendFile,
	}, pkg.RequiredFiles...)
	for _, item := range required {
		if !safeOnlineUpdateRelativePath(item) {
			return nil, fmt.Errorf("更新包 manifest 包含非法路径 %s", item)
		}
		if info, err := os.Stat(filepath.Join(stagingDir, filepath.FromSlash(item))); err != nil || info.IsDir() {
			return nil, fmt.Errorf("更新包缺少文件 %s", item)
		}
	}
	return &pkg, nil
}

func safeOnlineUpdateRelativePath(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !path.IsAbs(value) && path.Clean(value) == value && !strings.HasPrefix(value, "../")
}

func backupDatabaseForOnlineUpdate(jobID string) error {
	mysqldump, err := exec.LookPath("mysqldump")
	if err != nil {
		return errors.New("未找到 mysqldump，无法执行更新前数据库备份")
	}
	cfg, err := config.LoadDBConfig()
	if err != nil {
		return fmt.Errorf("读取数据库配置失败，无法执行更新前备份：%w", err)
	}
	backupDir := filepath.Join(config.GetUpdateDir(), "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("创建数据库备份目录失败：%w", err)
	}
	backupPath := filepath.Join(backupDir, fmt.Sprintf("db-%s.sql", time.Now().Format("20060102150405")))
	out, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("创建数据库备份文件失败：%w", err)
	}

	args := []string{"--single-transaction", "--quick"}
	if strings.HasPrefix(cfg.Host, "unix:") {
		args = append(args, "--socket", strings.TrimPrefix(cfg.Host, "unix:"))
	} else {
		args = append(args, "-h", cfg.Host)
		if cfg.Port != "" {
			args = append(args, "-P", cfg.Port)
		}
	}
	args = append(args, "-u", cfg.Username, cfg.Database)

	var stderr bytes.Buffer
	cmd := exec.Command(mysqldump, args...)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+cfg.Password)
	cmd.Stdout = out
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	closeErr := out.Close()
	if runErr != nil {
		_ = os.Remove(backupPath)
		return fmt.Errorf("数据库备份失败：%s", strings.TrimSpace(stderr.String()))
	}
	if closeErr != nil {
		_ = os.Remove(backupPath)
		return fmt.Errorf("保存数据库备份失败：%w", closeErr)
	}
	appendOnlineUpdateLog(jobID, "数据库已备份到 "+backupPath)
	return nil
}

func prepareOnlineUpdateFrontendSource(stagingDir string, pkg *extractedOnlineUpdateManifest) (string, error) {
	if pkg.FrontendDir != "." {
		return filepath.Join(stagingDir, filepath.FromSlash(pkg.FrontendDir)), nil
	}

	// 宝塔手工部署包的前端文件直接放在包根目录。
	// 在线更新时过滤掉后端和 manifest，只把 Web 文件复制到前端 releases 目录。
	filteredDir := filepath.Join(filepath.Dir(stagingDir), "frontend-filtered")
	if err := os.RemoveAll(filteredDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filteredDir, 0755); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(stagingDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "backend" || name == "manifest.json" {
			continue
		}
		if err := copyOnlineUpdatePath(
			filepath.Join(stagingDir, name),
			filepath.Join(filteredDir, name),
		); err != nil {
			return "", err
		}
	}
	return filteredDir, nil
}

func copyOnlineUpdatePath(source string, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := os.Open(source)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}

	if err := os.MkdirAll(target, info.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := copyOnlineUpdatePath(
			filepath.Join(source, entry.Name()),
			filepath.Join(target, entry.Name()),
		); err != nil {
			return err
		}
	}
	return nil
}

func writeOnlineUpdateScript(jobID string, stagingDir string, pkg *extractedOnlineUpdateManifest, frontendSource string, version string, frontendDir string) (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	dataDir := config.GetDataDir()
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", err
	}
	scriptPath := filepath.Join(config.GetUpdateDir(), jobID+".sh")
	script := fmt.Sprintf(`#!/bin/sh
set -u

APP_PID=%d
APP_BIN=%s
NEW_BIN=%s
FRONTEND_SOURCE=%s
FRONTEND_CURRENT=%s
DATA_DIR=%s
SERVICE_NAME=%s
PORT=%s
VERSION=%s
LOG_FILE=%s
JOB_RESULT=%s

log() {
  printf '%%s %%s\n' "$(date '+%%Y-%%m-%%d %%H:%%M:%%S')" "$1" >> "$LOG_FILE"
}

finish_job() {
  printf '%%s\n' "$1" > "$JOB_RESULT.tmp"
  mv -f "$JOB_RESULT.tmp" "$JOB_RESULT"
}

sleep 2
TS=$(date '+%%Y%%m%%d%%H%%M%%S')
USE_SYSTEMD=0
if command -v systemctl >/dev/null 2>&1 && systemctl cat "$SERVICE_NAME" >/dev/null 2>&1; then
  USE_SYSTEMD=1
fi

FRONTEND_MODE="inplace"
PREV_FRONTEND_TARGET=""
FRONTEND_BACKUP=""
FRONTEND_ROOT=""
TARGET_RELEASE=""
if [ ! -d "$FRONTEND_CURRENT" ] && [ ! -L "$FRONTEND_CURRENT" ]; then
  log "frontend directory not found: $FRONTEND_CURRENT"
  finish_job failed
  exit 1
fi

if [ -L "$FRONTEND_CURRENT" ] || [ "$(basename "$FRONTEND_CURRENT")" = "current" ]; then
  FRONTEND_MODE="release"
  FRONT_ROOT=$(dirname "$FRONTEND_CURRENT")
  RELEASES="$FRONT_ROOT/releases"
  TARGET_RELEASE="$RELEASES/$VERSION"
  mkdir -p "$RELEASES" || { finish_job failed; exit 1; }
  rm -rf "$TARGET_RELEASE"
  cp -a "$FRONTEND_SOURCE" "$TARGET_RELEASE" || { finish_job failed; exit 1; }
  if [ -L "$FRONTEND_CURRENT" ]; then
    PREV_FRONTEND_TARGET=$(readlink "$FRONTEND_CURRENT" || true)
    rm -f "$FRONTEND_CURRENT"
  else
    FRONTEND_BACKUP="$FRONT_ROOT/current.backup.$TS"
    mv "$FRONTEND_CURRENT" "$FRONTEND_BACKUP" || { finish_job failed; exit 1; }
  fi
  ln -s "$TARGET_RELEASE" "$FRONTEND_CURRENT" || { finish_job failed; exit 1; }
  log "frontend release switched to $TARGET_RELEASE"
else
  FRONTEND_ROOT="$FRONTEND_CURRENT"
  FRONTEND_BACKUP="${FRONTEND_ROOT}.backup.$TS"
  mkdir -p "$FRONTEND_BACKUP" || { finish_job failed; exit 1; }
  for item in index.html version.json favicon.ico manifest.json; do
    if [ -e "$FRONTEND_ROOT/$item" ] || [ -L "$FRONTEND_ROOT/$item" ]; then
      cp -a "$FRONTEND_ROOT/$item" "$FRONTEND_BACKUP/" || { finish_job failed; exit 1; }
    fi
  done
  if [ -d "$FRONTEND_SOURCE/assets" ]; then
    mkdir -p "$FRONTEND_ROOT/assets" || { finish_job failed; exit 1; }
    cp -a "$FRONTEND_SOURCE/assets/." "$FRONTEND_ROOT/assets/" || { finish_job failed; exit 1; }
  fi
  for item in favicon.ico manifest.json version.json; do
    if [ -f "$FRONTEND_SOURCE/$item" ]; then
      cp "$FRONTEND_SOURCE/$item" "$FRONTEND_ROOT/$item.next.$TS" || { finish_job failed; exit 1; }
      mv -f "$FRONTEND_ROOT/$item.next.$TS" "$FRONTEND_ROOT/$item" || { finish_job failed; exit 1; }
    fi
  done
  cp "$FRONTEND_SOURCE/index.html" "$FRONTEND_ROOT/index.html.next.$TS" || { finish_job failed; exit 1; }
  mv -f "$FRONTEND_ROOT/index.html.next.$TS" "$FRONTEND_ROOT/index.html" || { finish_job failed; exit 1; }
  log "frontend files switched in place at $FRONTEND_ROOT"
fi

BACKUP_BIN="$APP_BIN.backup.$TS"
APP_STAGE="$APP_BIN.next.$TS"
STARTED_PID=""

rollback_frontend() {
  if [ "$FRONTEND_MODE" = "release" ]; then
    if [ -n "$PREV_FRONTEND_TARGET" ]; then
      rm -f "$FRONTEND_CURRENT"
      ln -s "$PREV_FRONTEND_TARGET" "$FRONTEND_CURRENT"
    elif [ -n "$FRONTEND_BACKUP" ] && [ -d "$FRONTEND_BACKUP" ]; then
      rm -f "$FRONTEND_CURRENT"
      mv "$FRONTEND_BACKUP" "$FRONTEND_CURRENT"
    fi
    return
  fi
  for item in index.html version.json favicon.ico manifest.json; do
    if [ -e "$FRONTEND_BACKUP/$item" ] || [ -L "$FRONTEND_BACKUP/$item" ]; then
      cp -a "$FRONTEND_BACKUP/$item" "$FRONTEND_ROOT/$item.rollback.$TS" || true
      mv -f "$FRONTEND_ROOT/$item.rollback.$TS" "$FRONTEND_ROOT/$item" || true
    else
      rm -f "$FRONTEND_ROOT/$item"
    fi
  done
}

if [ ! -f "$NEW_BIN" ]; then
  log "new backend binary not found: $NEW_BIN"
  rollback_frontend
  finish_job failed
  exit 1
fi
if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
  log "curl or wget is required for health check"
  rollback_frontend
  finish_job failed
  exit 1
fi
cp "$APP_BIN" "$BACKUP_BIN" || { rollback_frontend; finish_job failed; exit 1; }
cp "$NEW_BIN" "$APP_STAGE" || { rollback_frontend; finish_job failed; exit 1; }
chmod 755 "$APP_STAGE" || { rm -f "$APP_STAGE"; rollback_frontend; finish_job failed; exit 1; }

if [ "$USE_SYSTEMD" = "1" ]; then
  if ! systemctl stop "$SERVICE_NAME"; then
    rm -f "$APP_STAGE"
    rollback_frontend
    finish_job failed
    exit 1
  fi
else
  kill "$APP_PID" >/dev/null 2>&1 || true
  for i in $(seq 1 20); do
    if ! kill -0 "$APP_PID" >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
  if kill -0 "$APP_PID" >/dev/null 2>&1; then
    kill -9 "$APP_PID" >/dev/null 2>&1 || true
    sleep 1
  fi
fi

if ! mv -f "$APP_STAGE" "$APP_BIN"; then
  if [ "$USE_SYSTEMD" = "1" ]; then
    systemctl start "$SERVICE_NAME" >/dev/null 2>&1 || true
  else
    cd "$DATA_DIR" || exit 1
    AUTO_PRO_DATA_DIR="$DATA_DIR" PORT="$PORT" nohup "$APP_BIN" >> "$DATA_DIR/logs/auto_pro.log" 2>&1 &
  fi
  rollback_frontend
  finish_job failed
  exit 1
fi
chmod 755 "$APP_BIN"

if [ "$USE_SYSTEMD" = "1" ]; then
  systemctl start "$SERVICE_NAME" >/dev/null 2>&1 || true
else
  mkdir -p "$DATA_DIR/logs"
  cd "$DATA_DIR" || exit 1
  AUTO_PRO_DATA_DIR="$DATA_DIR" PORT="$PORT" nohup "$APP_BIN" >> "$DATA_DIR/logs/auto_pro.log" 2>&1 &
  STARTED_PID=$!
fi

HEALTH=""
for i in $(seq 1 30); do
  sleep 1
  if command -v curl >/dev/null 2>&1; then
    HEALTH=$(curl -fs "http://127.0.0.1:$PORT/api/system/version" 2>/dev/null || true)
  else
    HEALTH=$(wget -qO- "http://127.0.0.1:$PORT/api/system/version" 2>/dev/null || true)
  fi
  if printf '%%s' "$HEALTH" | grep -Fq "\"version\":\"$VERSION\""; then
    log "update to $VERSION succeeded"
    finish_job success
    exit 0
  fi
done

log "health check failed, rolling back"
if [ "$USE_SYSTEMD" = "1" ]; then
  systemctl stop "$SERVICE_NAME" >/dev/null 2>&1 || true
elif [ -n "$STARTED_PID" ]; then
  kill "$STARTED_PID" >/dev/null 2>&1 || true
fi
mv -f "$BACKUP_BIN" "$APP_BIN"
chmod 755 "$APP_BIN"
rollback_frontend

if [ "$USE_SYSTEMD" = "1" ]; then
  systemctl start "$SERVICE_NAME" >/dev/null 2>&1 || true
else
  cd "$DATA_DIR" || exit 1
  AUTO_PRO_DATA_DIR="$DATA_DIR" PORT="$PORT" nohup "$APP_BIN" >> "$DATA_DIR/logs/auto_pro.log" 2>&1 &
fi

log "update failed and rollback completed"
finish_job failed
exit 1
`,
		os.Getpid(),
		shellQuoteOnlineUpdate(executable),
		shellQuoteOnlineUpdate(filepath.Join(stagingDir, filepath.FromSlash(pkg.BackendFile))),
		shellQuoteOnlineUpdate(frontendSource),
		shellQuoteOnlineUpdate(frontendDir),
		shellQuoteOnlineUpdate(dataDir),
		shellQuoteOnlineUpdate(config.GetServiceName()),
		shellQuoteOnlineUpdate(config.GetPort()),
		shellQuoteOnlineUpdate(version),
		shellQuoteOnlineUpdate(filepath.Join(logDir, jobID+".log")),
		shellQuoteOnlineUpdate(onlineUpdateJobStatePath(jobID)+".result"),
	)
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return "", err
	}
	return scriptPath, nil
}

func shellQuoteOnlineUpdate(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
