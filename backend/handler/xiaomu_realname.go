package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ========== 小沐聚合实名（provider=xiaomu） ==========
//
// 第四家实名认证服务商，与支付宝 / 快瞳 / 靓仔聚合认证并列，由插件中心单选启用。
// 与前三家的区别：小沐插件内部还可由管理员切换认证产品，不同产品交互形态不同。
//
// 认证产品（管理端可选，后端按白名单固定映射为上游 product_code）：
//   - three_element -> sm_three        姓名 + 身份证 + 手机号三要素核验（同步返回结果）
//   - face_h5       -> shumai_face_h5  数脉活体 H5 + 人脸权威库比对（跳转上游认证页）
//   - tencent_h5    -> tencent_sm      微信 / 腾讯官方实名认证（跳转上游认证页）
//
// product_code 只能由上面的白名单产生，客户端传入的任何产品编码都会被忽略。
//
// 三要素需要手机号，但 users / agents 表没有手机号字段：手机号只在发起认证时
// 由用户临时填写并随认证单保存，不写入主体表。
//
// 认证单复用快瞳 / 靓仔的 kt_face_<token> 存储与 faceToken 轮询通道，跳转模式
// 额外在认证单上记录上游订单号与认证地址。

const (
	realnameProviderXiaomu = "xiaomu"

	xiaomuModeThreeElement = "three_element"
	xiaomuModeFaceH5       = "face_h5"
	xiaomuModeTencentH5    = "tencent_h5"

	xiaomuProductThreeElement = "sm_three"
	xiaomuProductFaceH5       = "shumai_face_h5"
	xiaomuProductTencentH5    = "tencent_sm"

	xiaomuRealnameBaseURL     = "https://smapi.x1m1.cn"
	xiaomuInitializePath      = "/api/realname/initialize"
	xiaomuQueryPathTemplate   = "/api/realname/certifications/%s/query"
	xiaomuRealnameHTTPTimeout = 20 * time.Second
	xiaomuRealnameSessionTTL  = 30 * time.Minute
)

// xiaomuProductCodeByMode 管理端模式到上游产品编码的唯一映射来源。
func xiaomuProductCodeByMode(mode string) (string, bool) {
	switch strings.TrimSpace(mode) {
	case xiaomuModeThreeElement:
		return xiaomuProductThreeElement, true
	case xiaomuModeFaceH5:
		return xiaomuProductFaceH5, true
	case xiaomuModeTencentH5:
		return xiaomuProductTencentH5, true
	}
	return "", false
}

func validXiaomuProductMode(mode string) bool {
	_, ok := xiaomuProductCodeByMode(mode)
	return ok
}

// xiaomuModeRequiresMobile 只有三要素需要手机号。
func xiaomuModeRequiresMobile(mode string) bool {
	return strings.TrimSpace(mode) == xiaomuModeThreeElement
}

// xiaomuModeIsRedirect 需要跳转上游认证页并轮询结果的模式。
func xiaomuModeIsRedirect(mode string) bool {
	switch strings.TrimSpace(mode) {
	case xiaomuModeFaceH5, xiaomuModeTencentH5:
		return true
	}
	return false
}

// isValidChinaMobile 大陆手机号：11 位，1 开头，第二位 3-9。
func isValidChinaMobile(mobile string) bool {
	if len(mobile) != 11 || mobile[0] != '1' || mobile[1] < '3' || mobile[1] > '9' {
		return false
	}
	for i := 0; i < 11; i++ {
		if mobile[i] < '0' || mobile[i] > '9' {
			return false
		}
	}
	return true
}

// ========== 上游协议 ==========

// xiaomuRealnameOutcome 归一化后的上游结果，屏蔽字段名差异。
//
// 上游发起响应的三个关键字段各有明确分工，不能互相替代：
//   - id               认证记录 ID，查询结果时作为 record 参数
//   - outer_order_no   认证订单号，用于对账与本站留档
//   - certify_page_url 认证访问地址，用户跳转 / 扫码完成认证
//
// certify_id 在发起阶段可能是空字符串，只能作为兜底标识使用。
type xiaomuRealnameOutcome struct {
	Status    string // passed / failed / pending
	RecordID  string
	OrderNo   string
	CertifyID string
	AuthURL   string
	ExpiresAt time.Time
	Message   string
	Score     string
}

// queryReference 查询认证结果时使用的 record 值，订单号仅作兜底。
func (o xiaomuRealnameOutcome) queryReference() string {
	if o.RecordID != "" {
		return o.RecordID
	}
	return o.OrderNo
}

// xiaomuRealnameError 保留上游业务码、接口响应码与 HTTP 状态，便于线上无抓包定位。
// ResultUnknown 表示请求可能已受理但结果无法确认，不能自动重试（可能计费）。
type xiaomuRealnameError struct {
	ResultUnknown bool
	Message       string
	HTTPStatus    int
	ResponseCode  int
	BusinessCode  string
}

func (e *xiaomuRealnameError) Error() string {
	return e.Message
}

func newXiaomuRealnameError(message string, resultUnknown bool, httpStatus, responseCode int, businessCode string) error {
	return &xiaomuRealnameError{
		ResultUnknown: resultUnknown,
		Message:       message,
		HTTPStatus:    httpStatus,
		ResponseCode:  responseCode,
		BusinessCode:  strings.TrimSpace(businessCode),
	}
}

func xiaomuRealnameErrorMetadata(err error) (httpStatus, responseCode int, businessCode string) {
	var target *xiaomuRealnameError
	if !errors.As(err, &target) {
		return 0, 0, ""
	}
	return target.HTTPStatus, target.ResponseCode, target.BusinessCode
}

func isXiaomuRealnameResultUnknown(err error) bool {
	var target *xiaomuRealnameError
	return errors.As(err, &target) && target.ResultUnknown
}

// xiaomuRealnameEnvelope 顶层信封：data 用原始字节承载，避免上游把 data 返回为
// false / 字符串等非对象形态时，真实错误信息被解析错误覆盖。
type xiaomuRealnameEnvelope struct {
	Code    json.RawMessage `json:"code"`
	Message string          `json:"message"`
	Msg     string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

// xiaomuFlexText 统一读取字符串 / 数字 / 布尔形态的标量。
func xiaomuFlexText(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	if strings.HasPrefix(trimmed, `"`) {
		var text string
		if json.Unmarshal(raw, &text) == nil {
			return strings.TrimSpace(text)
		}
	}
	return trimmed
}

// xiaomuResponseCode 顶层业务码归一化。
func xiaomuResponseCode(raw json.RawMessage) (code int, present bool) {
	text := xiaomuFlexText(raw)
	if text == "" {
		return 0, false
	}
	var value int
	if _, err := fmt.Sscanf(text, "%d", &value); err != nil {
		return 0, false
	}
	return value, true
}

// xiaomuCodeIsSuccess 顶层成功码：缺失 / 0 / 200 / 20000。
// 上游正式返回的成功码是 20000，早期按 0 / 200 判定会把成功响应误判为请求失败。
func xiaomuCodeIsSuccess(code int, present bool) bool {
	if !present {
		return true
	}
	switch code {
	case 0, 200, 20000:
		return true
	}
	return false
}

func summarizeXiaomuResponseBody(body []byte) string {
	const maxRunes = 96

	text := strings.TrimSpace(string(body))
	if text == "" {
		return "<empty>"
	}
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return text
}

// xiaomuPickString 在扁平化后的 data 中按候选键取第一个非空标量。
func xiaomuPickString(data map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		if raw, ok := data[key]; ok {
			if text := xiaomuFlexText(raw); text != "" && text != "null" {
				return text
			}
		}
	}
	return ""
}

// flattenXiaomuData 把 data 及其一层嵌套对象合并成扁平表，容忍 data.result / data.data 包裹。
func flattenXiaomuData(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || !strings.HasPrefix(trimmed, "{") {
		return nil, false
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return nil, false
	}
	flat := make(map[string]json.RawMessage, len(top)*2)
	for key, value := range top {
		flat[key] = value
	}
	for _, key := range []string{"result", "data", "detail", "order", "info"} {
		nested, ok := top[key]
		if !ok {
			continue
		}
		var child map[string]json.RawMessage
		if json.Unmarshal(nested, &child) != nil {
			continue
		}
		for childKey, childValue := range child {
			if _, exists := flat[childKey]; !exists {
				flat[childKey] = childValue
			}
		}
	}
	return flat, true
}

// xiaomuNormalizeStatus 把上游状态文本归一化为本站三态。
// initialized / processing 是发起成功后的正常中间态，必须落到 pending 继续轮询。
func xiaomuNormalizeStatus(text string) string {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "passed", "pass", "success", "succeed", "succeeded", "ok", "true", "complete", "completed", "done", "verified", "1":
		return realnameFaceStatusPassed
	case "failed", "fail", "reject", "rejected", "error", "false", "invalid", "mismatch", "not_match", "0", "-1":
		return realnameFaceStatusFailed
	case "updated":
		// 同一身份发起了新认证，旧记录被取代：对本认证单是终态失败。
		return realnameFaceStatusFailed
	case "initialized", "initializing", "init", "created", "processing", "pending", "waiting", "in_progress":
		return realnameFaceStatusPending
	}
	return realnameFaceStatusPending
}

// parseXiaomuExpiresAt 解析认证链接有效期。
//
// 上游未给出固定时间格式，这里宽松匹配常见形态；解析结果只有落在
// [now+1min, now+24h] 区间内才采用，避免时区误差把认证单立刻判死或无限延长。
func parseXiaomuExpiresAt(text string, now time.Time) (time.Time, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || trimmed == "null" {
		return time.Time{}, false
	}

	var parsed time.Time
	if ts, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		switch {
		case ts > 1e12:
			parsed = time.UnixMilli(ts)
		case ts > 1e9:
			parsed = time.Unix(ts, 0)
		default:
			return time.Time{}, false
		}
	} else {
		layouts := []string{
			time.RFC3339,
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
			"2006/01/02 15:04:05",
			"2006-01-02 15:04",
		}
		for _, layout := range layouts {
			if value, err := time.ParseInLocation(layout, trimmed, time.Local); err == nil {
				parsed = value
				break
			}
		}
		if parsed.IsZero() {
			return time.Time{}, false
		}
	}

	if !parsed.After(now.Add(time.Minute)) || parsed.After(now.Add(24*time.Hour)) {
		return time.Time{}, false
	}
	return parsed, true
}

// parseXiaomuRealnameResponse 解析发起 / 查询响应，返回归一化结果。
func parseXiaomuRealnameResponse(statusCode int, body []byte) (xiaomuRealnameOutcome, error) {
	var envelope xiaomuRealnameEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		httpOK := statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
		detail := fmt.Sprintf(
			"小沐聚合实名响应解析失败：HTTP %d，JSON错误：%v，响应正文：%s",
			statusCode,
			err,
			summarizeXiaomuResponseBody(body),
		)
		return xiaomuRealnameOutcome{}, newXiaomuRealnameError(detail, httpOK, statusCode, 0, "")
	}

	topMessage := strings.TrimSpace(envelope.Message)
	if topMessage == "" {
		topMessage = strings.TrimSpace(envelope.Msg)
	}
	code, codePresent := xiaomuResponseCode(envelope.Code)
	data, dataIsObject := flattenXiaomuData(envelope.Data)
	httpOK := statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices

	if !httpOK || !xiaomuCodeIsSuccess(code, codePresent) {
		message := topMessage
		businessCode := ""
		if dataIsObject {
			if detail := xiaomuPickString(data, "message", "msg", "reason", "fail_reason", "failReason", "error_msg", "errorMsg"); detail != "" && !strings.Contains(message, detail) {
				if message == "" {
					message = detail
				} else {
					message += "：" + detail
				}
			}
			businessCode = xiaomuPickString(data, "code", "error_code", "errorCode", "sub_code", "subCode")
			if businessCode != "" && !strings.Contains(message, businessCode) {
				if message == "" {
					message = businessCode
				} else {
					message += "（" + businessCode + "）"
				}
			}
		} else if anomaly := summarizeXiaomuResponseBody(envelope.Data); message == "" && anomaly != "<empty>" {
			message = "data=" + anomaly
		}
		if message == "" {
			message = fmt.Sprintf("HTTP %d", statusCode)
		}
		return xiaomuRealnameOutcome{}, newXiaomuRealnameError(
			fmt.Sprintf(
				"小沐聚合实名请求失败：%s（HTTP %d，接口响应码 %d，响应正文：%s）",
				message, statusCode, code, summarizeXiaomuResponseBody(body),
			),
			false,
			statusCode,
			code,
			businessCode,
		)
	}

	if !dataIsObject {
		// HTTP 与业务码都成功但没有结果结构：结果无法确认，不能当作失败重试。
		detail := fmt.Sprintf(
			"小沐聚合实名响应未包含认证结果：HTTP %d，data=%s，响应正文：%s",
			statusCode,
			summarizeXiaomuResponseBody(envelope.Data),
			summarizeXiaomuResponseBody(body),
		)
		return xiaomuRealnameOutcome{}, newXiaomuRealnameError(detail, true, statusCode, code, "")
	}

	outcome := xiaomuRealnameOutcome{
		RecordID: xiaomuPickString(data,
			"record", "record_id", "recordId", "id",
		),
		OrderNo: xiaomuPickString(data,
			"outer_order_no", "outerOrderNo", "order_no", "orderNo", "order_id", "orderId", "trade_no", "tradeNo",
			"task_id", "taskId", "serial_no", "serialNo", "biz_id", "bizId",
		),
		CertifyID: xiaomuPickString(data, "certify_id", "certifyId"),
		AuthURL: xiaomuPickString(data,
			"certify_page_url", "certifyPageUrl", "certify_url", "certifyUrl",
			"url", "auth_url", "authUrl", "h5_url", "h5Url",
			"verify_url", "verifyUrl", "redirect_url", "redirectUrl", "short_url", "shortUrl", "link",
		),
		Message: xiaomuPickString(data, "message", "msg", "reason", "fail_reason", "failReason", "error_msg", "errorMsg"),
		Score:   xiaomuPickString(data, "score", "similarity", "confidence"),
	}
	if expiresAt, ok := parseXiaomuExpiresAt(
		xiaomuPickString(data, "expires_at", "expiresAt", "expire_at", "expireAt", "expire_time", "expireTime"),
		time.Now(),
	); ok {
		outcome.ExpiresAt = expiresAt
	}
	if outcome.OrderNo == "" {
		outcome.OrderNo = outcome.CertifyID
	}

	statusText := xiaomuPickString(data, "status", "state", "verify_status", "verifyStatus", "result_status", "resultStatus", "result")
	successText := xiaomuPickString(data, "success", "passed", "is_passed", "isPassed")
	switch {
	case statusText != "":
		outcome.Status = xiaomuNormalizeStatus(statusText)
	case successText != "":
		outcome.Status = xiaomuNormalizeStatus(successText)
	default:
		outcome.Status = realnameFaceStatusPending
	}
	if strings.EqualFold(strings.TrimSpace(statusText), "updated") && outcome.Message == "" {
		outcome.Message = "该认证记录已被新的认证取代，请重新发起认证"
	}
	if outcome.Status == realnameFaceStatusFailed && outcome.Message == "" {
		outcome.Message = "实名核验未通过"
	}
	return outcome, nil
}

// xiaomuRealnameRequest 统一发起请求：凭据走请求头，产品编码由服务端注入。
func xiaomuRealnameRequest(cfg realnameConfig, path string, payload map[string]interface{}) (xiaomuRealnameOutcome, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return xiaomuRealnameOutcome{}, newXiaomuRealnameError(fmt.Sprintf("构造小沐聚合实名请求失败：%v", err), false, 0, 0, "")
	}
	return xiaomuRealnameDo(cfg, http.MethodPost, path, strings.NewReader(string(body)))
}

// xiaomuRealnameHTTPRequest 构造上游 HTTP 请求；凭据只放请求头。
func xiaomuRealnameHTTPRequest(cfg realnameConfig, method, path string, reader io.Reader) (*http.Request, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.XiaomuBaseURL), "/")
	if baseURL == "" {
		baseURL = xiaomuRealnameBaseURL
	}
	req, err := http.NewRequest(method, baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-App-Key", cfg.XiaomuAppKey)
	req.Header.Set("X-App-Secret", cfg.XiaomuAppSecret)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// xiaomuRealnameDo 发起一次上游调用并解析响应。
func xiaomuRealnameDo(cfg realnameConfig, method, path string, reader io.Reader) (xiaomuRealnameOutcome, error) {
	req, err := xiaomuRealnameHTTPRequest(cfg, method, path, reader)
	if err != nil {
		return xiaomuRealnameOutcome{}, newXiaomuRealnameError(fmt.Sprintf("创建小沐聚合实名请求失败：%v", err), false, 0, 0, "")
	}

	client := &http.Client{Timeout: xiaomuRealnameHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return xiaomuRealnameOutcome{}, newXiaomuRealnameError(fmt.Sprintf("请求小沐聚合实名接口失败：%v", err), true, 0, 0, "")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return xiaomuRealnameOutcome{}, newXiaomuRealnameError(fmt.Sprintf("读取小沐聚合实名响应失败：%v", err), true, 0, 0, "")
	}
	return parseXiaomuRealnameResponse(resp.StatusCode, respBody)
}

// buildXiaomuInitializePayload 组装发起认证请求体；product_code 只来自后端白名单。
// 参数名固定使用上游统一约定：cert_name / cert_no / mobile / return_url。
func buildXiaomuInitializePayload(mode, realName, idCard, mobile, returnURL string) (map[string]interface{}, error) {
	productCode, ok := xiaomuProductCodeByMode(mode)
	if !ok {
		return nil, errors.New("小沐聚合实名认证产品不合法")
	}
	payload := map[string]interface{}{
		"product_code": productCode,
		"cert_name":    realName,
		"cert_no":      idCard,
	}
	if xiaomuModeRequiresMobile(mode) {
		if !isValidChinaMobile(mobile) {
			return nil, errors.New("三要素认证需要有效的手机号")
		}
		payload["mobile"] = mobile
	}
	if returnURL != "" {
		payload["return_url"] = returnURL
	}
	return payload, nil
}

func xiaomuRealnameInitialize(cfg realnameConfig, realName, idCard, mobile, returnURL string) (xiaomuRealnameOutcome, error) {
	payload, err := buildXiaomuInitializePayload(cfg.XiaomuProductMode, realName, idCard, mobile, returnURL)
	if err != nil {
		return xiaomuRealnameOutcome{}, newXiaomuRealnameError(err.Error(), false, 0, 0, "")
	}
	return xiaomuRealnameRequest(cfg, xiaomuInitializePath, payload)
}

// xiaomuRealnameQueryPath 查询接口路径：record 为路径参数，
// 支持发起响应的 id / outer_order_no / certify_id 三种取值。
func xiaomuRealnameQueryPath(record string) string {
	return fmt.Sprintf(xiaomuQueryPathTemplate, url.PathEscape(strings.TrimSpace(record)))
}

// xiaomuRealnameQueryRecord GET 查询认证结果，无请求体，凭据只走请求头。
func xiaomuRealnameQueryRecord(cfg realnameConfig, record string) (xiaomuRealnameOutcome, error) {
	if strings.TrimSpace(record) == "" {
		return xiaomuRealnameOutcome{}, newXiaomuRealnameError("缺少小沐聚合实名认证记录标识", false, 0, 0, "")
	}
	return xiaomuRealnameDo(cfg, http.MethodGet, xiaomuRealnameQueryPath(record), nil)
}

// ========== 认证单 ==========

func insertXiaomuRealnameSession(db *sql.DB, token string, session realnameFaceSession) error {
	_, err := db.Exec(`
		INSERT INTO system_configs (`+"`group`"+`, `+"`key`"+`, value, description)
		VALUES (?, ?, ?, '小沐聚合实名认证单')
		ON DUPLICATE KEY UPDATE value = VALUES(value)
	`, realnameGroup, "kt_face_"+token, session.encode())
	return err
}

func applyXiaomuRealnameError(session realnameFaceSession, err error) realnameFaceSession {
	httpStatus, responseCode, businessCode := xiaomuRealnameErrorMetadata(err)
	session.FailCode = businessCode
	session.HTTPStatus = httpStatus
	session.ResponseCode = responseCode
	if isXiaomuRealnameResultUnknown(err) {
		session.Status = realnameFaceStatusUnknown
		session.FailMsg = "小沐聚合实名服务响应中断，认证结果无法确认，请稍后重新发起认证；详细错误：" + err.Error()
		return session
	}
	session.Status = realnameFaceStatusFailed
	session.FailMsg = err.Error()
	return session
}

// ========== 发起认证 ==========

// startXiaomuRealname 三要素同步核验；人脸 / 微信模式生成上游认证页并落认证单。
func startXiaomuRealname(c *gin.Context, db *sql.DB, cfg realnameConfig, ownerType string, ownerID uint, req userRealnameInitRequest) {
	if cfg.XiaomuAppKey == "" || cfg.XiaomuAppSecret == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "管理员尚未完成小沐聚合实名配置"})
		return
	}
	if !validXiaomuProductMode(cfg.XiaomuProductMode) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "管理员尚未选择小沐聚合实名认证产品"})
		return
	}

	mobile := strings.TrimSpace(req.Mobile)
	if xiaomuModeRequiresMobile(cfg.XiaomuProductMode) {
		if mobile == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "三要素认证需要填写本人手机号"})
			return
		}
		if !isValidChinaMobile(mobile) {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "手机号格式不正确"})
			return
		}
	} else {
		mobile = ""
	}

	token, err := newRealnameFaceToken()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建认证单失败"})
		return
	}
	returnURL := ""
	if xiaomuModeIsRedirect(cfg.XiaomuProductMode) {
		returnURL = buildFrontendReturnURL(c, "", "/")
	}

	session := realnameFaceSession{
		Provider:  realnameProviderXiaomu,
		OwnerType: ownerType,
		UserID:    int64(ownerID),
		RealName:  req.RealName,
		IDCard:    req.IDCard,
		ReturnURL: returnURL,
		Status:    realnameFaceStatusPending,
		AuthMode:  cfg.XiaomuProductMode,
		Mobile:    mobile,
		ExpireAt:  time.Now().Add(xiaomuRealnameSessionTTL),
	}

	outcome, verr := xiaomuRealnameInitialize(cfg, req.RealName, req.IDCard, mobile, returnURL)
	if verr != nil {
		session = applyXiaomuRealnameError(session, verr)
		_ = insertXiaomuRealnameSession(db, token, session)
		writeRealnameRecord(db, ownerType, int64(ownerID), realnameProviderXiaomu,
			req.RealName, req.IDCard, realnameFaceStatusFailed, tencentRealnameRecordFailureReason(session, verr), "", "")
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": session.FailMsg, "data": realnameFaceFailureData(session)})
		return
	}

	session.OrderNo = outcome.OrderNo
	session.RecordID = outcome.RecordID
	session.AuthURL = outcome.AuthURL
	if !outcome.ExpiresAt.IsZero() {
		session.ExpireAt = outcome.ExpiresAt
	}

	switch outcome.Status {
	case realnameFaceStatusPassed:
		session.Status = realnameFaceStatusPassed
		_ = insertXiaomuRealnameSession(db, token, session)
		_, _ = db.Exec(
			"UPDATE "+realnameOwnerTable(ownerType)+" SET real_name = ?, real_id_card = ?, realname_at = NOW() WHERE id = ?",
			session.RealName, session.IDCard, session.UserID,
		)
		writeRealnameRecord(db, ownerType, int64(ownerID), realnameProviderXiaomu,
			req.RealName, req.IDCard, realnameFaceStatusPassed, "", outcome.OrderNo, outcome.Score)
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "实名认证成功", "data": realnameFacePassedData(session)})
		return
	case realnameFaceStatusFailed:
		session.Status = realnameFaceStatusFailed
		session.FailMsg = outcome.Message
		_ = insertXiaomuRealnameSession(db, token, session)
		writeRealnameRecord(db, ownerType, int64(ownerID), realnameProviderXiaomu,
			req.RealName, req.IDCard, realnameFaceStatusFailed, outcome.Message, outcome.OrderNo, outcome.Score)
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "实名核验未通过：" + outcome.Message, "data": realnameFaceFailureData(session)})
		return
	}

	// 待认证：跳转模式必须同时拿到认证地址与可查询标识，否则用户认证完也无法回收结果。
	if xiaomuModeIsRedirect(cfg.XiaomuProductMode) {
		if outcome.AuthURL == "" {
			session.Status = realnameFaceStatusFailed
			session.FailMsg = "小沐聚合实名未返回认证地址（certify_page_url），请联系管理员核对认证产品配置"
			_ = insertXiaomuRealnameSession(db, token, session)
			writeRealnameRecord(db, ownerType, int64(ownerID), realnameProviderXiaomu,
				req.RealName, req.IDCard, realnameFaceStatusFailed, session.FailMsg, outcome.OrderNo, "")
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": session.FailMsg})
			return
		}
		if outcome.queryReference() == "" {
			session.Status = realnameFaceStatusUnknown
			session.FailMsg = "小沐聚合实名未返回认证记录 ID（id），无法查询认证结果，请稍后重新发起认证"
			_ = insertXiaomuRealnameSession(db, token, session)
			writeRealnameRecord(db, ownerType, int64(ownerID), realnameProviderXiaomu,
				req.RealName, req.IDCard, realnameFaceStatusFailed, session.FailMsg, outcome.OrderNo, "")
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": session.FailMsg, "data": realnameFaceFailureData(session)})
			return
		}
	} else if outcome.queryReference() == "" {
		session.Status = realnameFaceStatusUnknown
		session.FailMsg = "小沐聚合实名未返回认证结果与认证记录 ID，结果无法确认，请稍后重新发起认证"
		_ = insertXiaomuRealnameSession(db, token, session)
		writeRealnameRecord(db, ownerType, int64(ownerID), realnameProviderXiaomu,
			req.RealName, req.IDCard, realnameFaceStatusFailed, session.FailMsg, "", "")
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": session.FailMsg, "data": realnameFaceFailureData(session)})
		return
	}

	if err := insertXiaomuRealnameSession(db, token, session); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "创建认证单失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "请点击链接或扫码完成实名认证",
		"data": gin.H{
			"status":     realnameFaceStatusPending,
			"provider":   realnameProviderXiaomu,
			"authMode":   cfg.XiaomuProductMode,
			"faceToken":  token,
			"certifyUrl": outcome.AuthURL,
			"orderNo":    outcome.OrderNo,
		},
	})
}

// ========== 结果轮询 ==========

// xiaomuRealnameQuery 按主体轮询小沐认证单；未出结论时向上游查询订单状态。
func xiaomuRealnameQuery(c *gin.Context, db *sql.DB, cfg realnameConfig, ownerType string, ownerID uint) {
	token := strings.TrimSpace(c.Query("faceToken"))
	if token == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "缺少 faceToken"})
		return
	}

	session, err := loadRealnameFaceSession(db, token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if session.OwnerType != ownerType || session.UserID != int64(ownerID) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "认证单号不匹配"})
		return
	}

	switch session.Status {
	case realnameFaceStatusPassed:
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "实名认证成功", "data": realnameFacePassedData(session)})
		return
	case realnameFaceStatusFailed, realnameFaceStatusUnknown:
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "认证未通过", "data": realnameFaceFailureData(session)})
		return
	}

	queryRef := session.RecordID
	if queryRef == "" {
		queryRef = session.OrderNo
	}
	if queryRef == "" {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "认证未完成", "data": gin.H{"status": realnameFaceStatusPending}})
		return
	}

	outcome, verr := xiaomuRealnameQueryRecord(cfg, queryRef)
	if verr != nil {
		// 查询失败不改变认证单状态：网络抖动或上游临时不可用时允许继续轮询，
		// 但必须把真实错误返回给前端，不能伪装成普通“认证未完成”。
		httpStatus, responseCode, businessCode := xiaomuRealnameErrorMetadata(verr)
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "查询认证结果失败：" + verr.Error(),
			"data": gin.H{
				"status":               realnameFaceStatusPending,
				"reason":               verr.Error(),
				"errorCode":            businessCode,
				"upstreamHttpStatus":   httpStatus,
				"upstreamResponseCode": responseCode,
				"queryRecord":          queryRef,
			},
		})
		return
	}

	if session.OrderNo == "" && outcome.OrderNo != "" {
		session.OrderNo = outcome.OrderNo
	}

	switch outcome.Status {
	case realnameFaceStatusPassed:
		session.Status = realnameFaceStatusPassed
		session.FailMsg = ""
		saveRealnameFaceSession(db, token, session)
		_, _ = db.Exec(
			"UPDATE "+realnameOwnerTable(ownerType)+" SET real_name = ?, real_id_card = ?, realname_at = NOW() WHERE id = ?",
			session.RealName, session.IDCard, session.UserID,
		)
		writeRealnameRecord(db, ownerType, session.UserID, realnameProviderXiaomu,
			session.RealName, session.IDCard, realnameFaceStatusPassed, "", session.OrderNo, outcome.Score)
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "实名认证成功", "data": realnameFacePassedData(session)})
	case realnameFaceStatusFailed:
		session.Status = realnameFaceStatusFailed
		session.FailMsg = outcome.Message
		saveRealnameFaceSession(db, token, session)
		writeRealnameRecord(db, ownerType, session.UserID, realnameProviderXiaomu,
			session.RealName, session.IDCard, realnameFaceStatusFailed, outcome.Message, session.OrderNo, outcome.Score)
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "认证未通过", "data": realnameFaceFailureData(session)})
	default:
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "认证未完成", "data": gin.H{"status": realnameFaceStatusPending}})
	}
}
