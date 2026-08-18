package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestXiaomuProductCodeByModeWhitelist(t *testing.T) {
	cases := map[string]string{
		xiaomuModeThreeElement: xiaomuProductThreeElement,
		xiaomuModeFaceH5:       xiaomuProductFaceH5,
		xiaomuModeTencentH5:    xiaomuProductTencentH5,
	}
	for mode, expected := range cases {
		code, ok := xiaomuProductCodeByMode(mode)
		if !ok || code != expected {
			t.Fatalf("模式 %s 期望产品编码 %s，实际 %s(ok=%v)", mode, expected, code, ok)
		}
	}

	for _, invalid := range []string{"", "alipay_v3", "sm_three", "SM_THREE", "three_element ", "../etc/passwd"} {
		if _, ok := xiaomuProductCodeByMode(invalid); ok && strings.TrimSpace(invalid) != xiaomuModeThreeElement {
			t.Fatalf("非法模式 %q 不应通过白名单", invalid)
		}
	}
}

func TestBuildXiaomuInitializePayloadRejectsClientProductCode(t *testing.T) {
	// 客户端无法注入产品编码：入参只有管理端模式，输出固定为白名单映射结果。
	payload, err := buildXiaomuInitializePayload(xiaomuModeFaceH5, "张三", "11010519491231002X", "13800138000", "https://demo.local/return")
	if err != nil {
		t.Fatalf("组包失败：%v", err)
	}
	if payload["product_code"] != xiaomuProductFaceH5 {
		t.Fatalf("产品编码期望 %s，实际 %v", xiaomuProductFaceH5, payload["product_code"])
	}
	if _, exists := payload["mobile"]; exists {
		t.Fatal("人脸模式不应发送手机号")
	}
	if payload["return_url"] != "https://demo.local/return" {
		t.Fatalf("跳转模式应携带回跳地址，实际 %v", payload["return_url"])
	}

	if _, err := buildXiaomuInitializePayload("alipay_v3", "张三", "11010519491231002X", "", ""); err == nil {
		t.Fatal("未接入的产品应被拒绝")
	}
}

func TestBuildXiaomuInitializePayloadThreeElement(t *testing.T) {
	payload, err := buildXiaomuInitializePayload(xiaomuModeThreeElement, "张三", "11010519491231002X", "13800138000", "")
	if err != nil {
		t.Fatalf("三要素组包失败：%v", err)
	}
	if payload["product_code"] != xiaomuProductThreeElement {
		t.Fatalf("三要素产品编码错误：%v", payload["product_code"])
	}
	if payload["mobile"] != "13800138000" {
		t.Fatalf("三要素应携带手机号，实际 %v", payload["mobile"])
	}
	if payload["cert_name"] != "张三" || payload["cert_no"] != "11010519491231002X" {
		t.Fatalf("姓名或身份证未按上游参数名组包：%v", payload)
	}
	for _, legacy := range []string{"name", "idcard", "id_card"} {
		if _, exists := payload[legacy]; exists {
			t.Fatalf("不应发送非约定参数名 %s", legacy)
		}
	}
	if _, exists := payload["return_url"]; exists {
		t.Fatal("同步核验不应携带回跳地址")
	}

	if _, err := buildXiaomuInitializePayload(xiaomuModeThreeElement, "张三", "11010519491231002X", "", ""); err == nil {
		t.Fatal("三要素缺少手机号应报错")
	}
	if _, err := buildXiaomuInitializePayload(xiaomuModeThreeElement, "张三", "11010519491231002X", "1380013800", ""); err == nil {
		t.Fatal("三要素手机号位数不足应报错")
	}
}

func TestBuildXiaomuInitializePayloadUsesUpstreamParamNames(t *testing.T) {
	// 上游统一参数名：cert_name / cert_no，人脸 H5 另带 return_url。
	payload, err := buildXiaomuInitializePayload(xiaomuModeFaceH5, "李四", "11010519491231002X", "", "https://demo.local/back")
	if err != nil {
		t.Fatalf("组包失败：%v", err)
	}
	expected := map[string]interface{}{
		"product_code": xiaomuProductFaceH5,
		"cert_name":    "李四",
		"cert_no":      "11010519491231002X",
		"return_url":   "https://demo.local/back",
	}
	if len(payload) != len(expected) {
		t.Fatalf("请求体字段集合不符合约定：%v", payload)
	}
	for key, want := range expected {
		if payload[key] != want {
			t.Fatalf("参数 %s 期望 %v，实际 %v", key, want, payload[key])
		}
	}
}

func TestXiaomuCodeIsSuccessAcceptsUpstream20000(t *testing.T) {
	// 上游成功码是 20000，早期只认 0 / 200 会把成功响应误判为请求失败。
	for _, code := range []int{0, 200, 20000} {
		if !xiaomuCodeIsSuccess(code, true) {
			t.Fatalf("响应码 %d 应视为成功", code)
		}
	}
	if !xiaomuCodeIsSuccess(0, false) {
		t.Fatal("缺失响应码应视为成功")
	}
	for _, code := range []int{1, 400, 1001, 40000, 50000} {
		if xiaomuCodeIsSuccess(code, true) {
			t.Fatalf("响应码 %d 不应视为成功", code)
		}
	}
}

func TestParseXiaomuRealnameResponseCode20000Initialize(t *testing.T) {
	// 线上实际形态：code=20000、message=success，认证信息全在 data 中。
	body := []byte(`{"code":20000,"message":"success","data":{
		"id":10245,
		"product_code":"shumai_face_h5",
		"outer_order_no":"XM20260314000456",
		"certify_id":"",
		"certify_page_url":"https://smapi.x1m1.cn/certify/xyz789",
		"status":"initialized",
		"fail_reason":""
	}}`)
	outcome, err := parseXiaomuRealnameResponse(http.StatusOK, body)
	if err != nil {
		t.Fatalf("20000 是成功码，不应返回请求级错误：%v", err)
	}
	if outcome.Status != realnameFaceStatusPending {
		t.Fatalf("状态期望 pending，实际 %s", outcome.Status)
	}
	if outcome.AuthURL != "https://smapi.x1m1.cn/certify/xyz789" {
		t.Fatalf("认证地址解析错误：%q", outcome.AuthURL)
	}
	if outcome.queryReference() != "10245" {
		t.Fatalf("查询标识解析错误：%q", outcome.queryReference())
	}
	if outcome.Message != "" {
		t.Fatalf("顶层 message=success 不应污染失败原因，实际 %q", outcome.Message)
	}
}

func TestParseXiaomuRealnameResponseErrorCarriesResponseBody(t *testing.T) {
	// 请求级失败必须带上响应正文，否则线上只能看到一句无信息量的提示。
	body := []byte(`{"code":40001,"message":"参数缺失","data":null}`)
	_, err := parseXiaomuRealnameResponse(http.StatusOK, body)
	if err == nil {
		t.Fatal("非成功码应返回错误")
	}
	for _, expected := range []string{"参数缺失", "40001", "HTTP 200", `"code":40001`} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("错误信息缺少 %s：%q", expected, err.Error())
		}
	}
}

func TestXiaomuQueryRequestUsesGetPathParameter(t *testing.T) {
	cfg := realnameConfig{
		XiaomuBaseURL:   "https://smapi.x1m1.cn/",
		XiaomuAppKey:    "test-key",
		XiaomuAppSecret: "test-secret",
	}
	path := xiaomuRealnameQueryPath("RN/2026 A")
	if path != "/api/realname/certifications/RN%2F2026%20A/query" {
		t.Fatalf("record 必须安全编码进路径，实际 %q", path)
	}
	req, err := xiaomuRealnameHTTPRequest(cfg, http.MethodGet, path, nil)
	if err != nil {
		t.Fatalf("构造查询请求失败：%v", err)
	}
	if req.Method != http.MethodGet {
		t.Fatalf("查询接口必须使用 GET，实际 %s", req.Method)
	}
	if req.URL.String() != "https://smapi.x1m1.cn/api/realname/certifications/RN%2F2026%20A/query" {
		t.Fatalf("查询地址错误：%s", req.URL.String())
	}
	if req.Body != nil {
		t.Fatal("GET 查询不应携带请求体")
	}
	if req.Header.Get("X-App-Key") != "test-key" || req.Header.Get("X-App-Secret") != "test-secret" {
		t.Fatalf("查询请求缺少鉴权头：%v", req.Header)
	}
	if req.Header.Get("Content-Type") != "" {
		t.Fatalf("GET 查询不应声明 JSON 请求体，实际 %q", req.Header.Get("Content-Type"))
	}
}

func TestParseXiaomuQueryResponsePassed(t *testing.T) {
	body := []byte(`{"code":20000,"msg":"success","status":200,"data":{
		"id":123,
		"product_code":"sm_face",
		"outer_order_no":"RN20260703120000ABCDEF",
		"certify_id":"CERTIFY_ID_OR_TOKEN",
		"certify_page_url":"https://smapi.x1m1.cn/api/realname/certifications/123/certify",
		"expires_at":"2026-07-03 14:00:00",
		"status":"passed",
		"fail_reason":"",
		"passed_at":"2026-07-03T12:03:00.000000Z",
		"created_at":"2026-07-03T12:00:00.000000Z"
	}}`)
	outcome, err := parseXiaomuRealnameResponse(http.StatusOK, body)
	if err != nil {
		t.Fatalf("查询响应解析失败：%v", err)
	}
	if outcome.Status != realnameFaceStatusPassed {
		t.Fatalf("查询通过状态应归一化为 passed，实际 %s", outcome.Status)
	}
	if outcome.RecordID != "123" || outcome.OrderNo != "RN20260703120000ABCDEF" || outcome.CertifyID != "CERTIFY_ID_OR_TOKEN" {
		t.Fatalf("查询响应标识解析错误：%+v", outcome)
	}
}

func TestParseXiaomuQueryResponseUpdatedIsTerminal(t *testing.T) {
	body := []byte(`{"code":20000,"msg":"success","status":200,"data":{"id":123,"status":"updated","fail_reason":""}}`)
	outcome, err := parseXiaomuRealnameResponse(http.StatusOK, body)
	if err != nil {
		t.Fatalf("查询响应解析失败：%v", err)
	}
	if outcome.Status != realnameFaceStatusFailed {
		t.Fatalf("updated 对旧认证单必须是终态，实际 %s", outcome.Status)
	}
	if !strings.Contains(outcome.Message, "新的认证取代") {
		t.Fatalf("updated 应给出可操作原因，实际 %q", outcome.Message)
	}
}

func TestIsValidChinaMobile(t *testing.T) {
	valid := []string{"13800138000", "19900001111", "15012345678"}
	for _, mobile := range valid {
		if !isValidChinaMobile(mobile) {
			t.Fatalf("%s 应判定为合法手机号", mobile)
		}
	}
	invalid := []string{"", "1380013800", "138001380001", "12800138000", "1380013800a", "+8613800138000"}
	for _, mobile := range invalid {
		if isValidChinaMobile(mobile) {
			t.Fatalf("%s 应判定为非法手机号", mobile)
		}
	}
}

func TestParseXiaomuRealnameResponsePassed(t *testing.T) {
	body := []byte(`{"code":0,"message":"ok","data":{"status":"SUCCESS","order_no":"XM202603140001","score":"97.5"}}`)
	outcome, err := parseXiaomuRealnameResponse(http.StatusOK, body)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if outcome.Status != realnameFaceStatusPassed {
		t.Fatalf("状态期望 passed，实际 %s", outcome.Status)
	}
	if outcome.OrderNo != "XM202603140001" || outcome.Score != "97.5" {
		t.Fatalf("订单号或分数解析错误：%+v", outcome)
	}
}

func TestParseXiaomuRealnameResponseFailedKeepsReason(t *testing.T) {
	body := []byte(`{"code":0,"data":{"status":"failed","fail_reason":"三要素不一致","error_code":"NAME_MISMATCH"}}`)
	outcome, err := parseXiaomuRealnameResponse(http.StatusOK, body)
	if err != nil {
		t.Fatalf("业务失败不应返回请求级错误：%v", err)
	}
	if outcome.Status != realnameFaceStatusFailed {
		t.Fatalf("状态期望 failed，实际 %s", outcome.Status)
	}
	if outcome.Message != "三要素不一致" {
		t.Fatalf("应保留上游失败原因，实际 %q", outcome.Message)
	}
}

func TestParseXiaomuRealnameResponseRedirectPending(t *testing.T) {
	// 字段名差异：h5Url / certifyId / 状态缺失，均应归一化为待认证。
	body := []byte(`{"code":200,"data":{"h5Url":"https://smapi.x1m1.cn/h5/abc","certifyId":"CID-9527"}}`)
	outcome, err := parseXiaomuRealnameResponse(http.StatusOK, body)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if outcome.Status != realnameFaceStatusPending {
		t.Fatalf("状态期望 pending，实际 %s", outcome.Status)
	}
	if outcome.AuthURL != "https://smapi.x1m1.cn/h5/abc" {
		t.Fatalf("认证地址解析错误：%q", outcome.AuthURL)
	}
	if outcome.OrderNo != "CID-9527" {
		t.Fatalf("订单号解析错误：%q", outcome.OrderNo)
	}
}

func TestParseXiaomuRealnameResponseNestedResult(t *testing.T) {
	body := []byte(`{"code":0,"data":{"result":{"state":"pass","orderId":"NESTED-1"}}}`)
	outcome, err := parseXiaomuRealnameResponse(http.StatusOK, body)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if outcome.Status != realnameFaceStatusPassed || outcome.OrderNo != "NESTED-1" {
		t.Fatalf("嵌套结构解析错误：%+v", outcome)
	}
}

func TestParseXiaomuRealnameResponseBusinessCodeError(t *testing.T) {
	body := []byte(`{"code":1001,"message":"余额不足","data":{"error_code":"NO_BALANCE"}}`)
	_, err := parseXiaomuRealnameResponse(http.StatusOK, body)
	if err == nil {
		t.Fatal("业务码非成功应返回错误")
	}
	if isXiaomuRealnameResultUnknown(err) {
		t.Fatal("明确的业务错误不应标记为结果未知")
	}
	httpStatus, responseCode, businessCode := xiaomuRealnameErrorMetadata(err)
	if httpStatus != http.StatusOK || responseCode != 1001 || businessCode != "NO_BALANCE" {
		t.Fatalf("错误元数据丢失：http=%d code=%d business=%s", httpStatus, responseCode, businessCode)
	}
	if !strings.Contains(err.Error(), "余额不足") || !strings.Contains(err.Error(), "NO_BALANCE") {
		t.Fatalf("错误信息应同时保留上游消息与业务码，实际 %q", err.Error())
	}
}

func TestParseXiaomuRealnameResponseHTTPError(t *testing.T) {
	body := []byte(`{"message":"鉴权失败"}`)
	_, err := parseXiaomuRealnameResponse(http.StatusUnauthorized, body)
	if err == nil {
		t.Fatal("非 2xx 应返回错误")
	}
	httpStatus, _, _ := xiaomuRealnameErrorMetadata(err)
	if httpStatus != http.StatusUnauthorized {
		t.Fatalf("应保留 HTTP 状态，实际 %d", httpStatus)
	}
	if !strings.Contains(err.Error(), "鉴权失败") {
		t.Fatalf("应保留上游消息，实际 %q", err.Error())
	}
}

func TestParseXiaomuRealnameResponseMalformedJSON(t *testing.T) {
	body := []byte(`<html><body>502 Bad Gateway</body></html>`)
	_, err := parseXiaomuRealnameResponse(http.StatusOK, body)
	if err == nil {
		t.Fatal("畸形响应应返回错误")
	}
	if !isXiaomuRealnameResultUnknown(err) {
		t.Fatal("HTTP 成功但响应不可解析时结果无法确认，应禁止自动重试")
	}
	if !strings.Contains(err.Error(), "502 Bad Gateway") || !strings.Contains(err.Error(), "HTTP 200") {
		t.Fatalf("错误信息应包含 HTTP 状态与正文摘要，实际 %q", err.Error())
	}
}

func TestParseXiaomuRealnameResponseNonObjectData(t *testing.T) {
	// 上游把 data 返回为布尔值时，不能当作失败，也不能丢失诊断信息。
	body := []byte(`{"code":0,"message":"ok","data":false}`)
	_, err := parseXiaomuRealnameResponse(http.StatusOK, body)
	if err == nil {
		t.Fatal("缺少结果结构应返回错误")
	}
	if !isXiaomuRealnameResultUnknown(err) {
		t.Fatal("成功响应但无结果结构时应标记结果未知")
	}
	if !strings.Contains(err.Error(), "data=false") {
		t.Fatalf("应记录异常 data 形态，实际 %q", err.Error())
	}
}

func TestSummarizeXiaomuResponseBodyIsSingleLineAndBounded(t *testing.T) {
	summary := summarizeXiaomuResponseBody([]byte(strings.Repeat("响应正文\n", 100)))
	if strings.ContainsAny(summary, "\n\r") {
		t.Fatal("摘要必须是单行")
	}
	if len([]rune(summary)) > 99 {
		t.Fatalf("摘要长度应受限，实际 %d", len([]rune(summary)))
	}
	if summarizeXiaomuResponseBody(nil) != "<empty>" {
		t.Fatal("空正文应返回占位标记")
	}
}

func TestXiaomuFlexTextAcceptsScalarShapes(t *testing.T) {
	cases := map[string]string{
		`"abc"`: "abc",
		`123`:   "123",
		`true`:  "true",
		`null`:  "",
	}
	for raw, expected := range cases {
		if got := xiaomuFlexText(json.RawMessage(raw)); got != expected {
			t.Fatalf("%s 期望 %q，实际 %q", raw, expected, got)
		}
	}
}

func TestParseXiaomuRealnameResponseOfficialInitializeContract(t *testing.T) {
	// 官方发起响应契约：认证地址在 certify_page_url，查询标识在 id，订单号在 outer_order_no。
	body := []byte(`{"code":0,"message":"ok","data":{
		"id":10231,
		"product_code":"shumai_face_h5",
		"outer_order_no":"XM20260314000123",
		"certify_id":"",
		"certify_page_url":"https://smapi.x1m1.cn/certify/abc123",
		"expires_at":"2099-01-01 12:00:00",
		"status":"initialized",
		"fail_reason":"",
		"passed_at":null,
		"created_at":"2026-03-14 10:00:00"
	}}`)
	outcome, err := parseXiaomuRealnameResponse(http.StatusOK, body)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if outcome.Status != realnameFaceStatusPending {
		t.Fatalf("initialized 应归一化为 pending，实际 %s", outcome.Status)
	}
	if outcome.AuthURL != "https://smapi.x1m1.cn/certify/abc123" {
		t.Fatalf("认证地址必须取 certify_page_url，实际 %q", outcome.AuthURL)
	}
	if outcome.RecordID != "10231" {
		t.Fatalf("查询标识必须取 id，实际 %q", outcome.RecordID)
	}
	if outcome.OrderNo != "XM20260314000123" {
		t.Fatalf("订单号必须取 outer_order_no，实际 %q", outcome.OrderNo)
	}
	if outcome.queryReference() != "10231" {
		t.Fatalf("查询参数 record 应优先使用记录 ID，实际 %q", outcome.queryReference())
	}
	if outcome.Message != "" {
		t.Fatalf("发起成功时空 fail_reason 不应产生失败原因，实际 %q", outcome.Message)
	}
	if !outcome.ExpiresAt.IsZero() {
		t.Fatal("超过 24 小时的有效期不应覆盖本地认证单期限")
	}
}

func TestParseXiaomuRealnameResponseProcessingKeepsPolling(t *testing.T) {
	body := []byte(`{"code":0,"data":{"id":88,"status":"processing","fail_reason":"","passed_at":null}}`)
	outcome, err := parseXiaomuRealnameResponse(http.StatusOK, body)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if outcome.Status != realnameFaceStatusPending {
		t.Fatalf("processing 应继续轮询，实际 %s", outcome.Status)
	}
	if outcome.queryReference() != "88" {
		t.Fatalf("查询标识丢失：%q", outcome.queryReference())
	}
}

func TestParseXiaomuRealnameResponseTimeoutFailure(t *testing.T) {
	// 认证链接过期后上游返回 failed + fail_reason=认证超时，必须原样落到失败原因。
	body := []byte(`{"code":0,"data":{"id":77,"status":"failed","fail_reason":"认证超时"}}`)
	outcome, err := parseXiaomuRealnameResponse(http.StatusOK, body)
	if err != nil {
		t.Fatalf("业务失败不应返回请求级错误：%v", err)
	}
	if outcome.Status != realnameFaceStatusFailed || outcome.Message != "认证超时" {
		t.Fatalf("超时失败解析错误：%+v", outcome)
	}
}

func TestParseXiaomuExpiresAtGuardsBounds(t *testing.T) {
	now := time.Date(2026, 3, 14, 10, 0, 0, 0, time.Local)

	valid := now.Add(30 * time.Minute)
	got, ok := parseXiaomuExpiresAt(valid.Format("2006-01-02 15:04:05"), now)
	if !ok || !got.Equal(valid) {
		t.Fatalf("正常有效期应被采用：got=%v ok=%v", got, ok)
	}
	if _, ok := parseXiaomuExpiresAt(valid.Format(time.RFC3339), now); !ok {
		t.Fatal("RFC3339 形态应被识别")
	}
	if _, ok := parseXiaomuExpiresAt(strconv.FormatInt(valid.Unix(), 10), now); !ok {
		t.Fatal("秒级时间戳应被识别")
	}

	for _, rejected := range []string{
		"",
		"null",
		"not-a-time",
		now.Add(-time.Hour).Format("2006-01-02 15:04:05"),
		now.Add(48 * time.Hour).Format("2006-01-02 15:04:05"),
	} {
		if _, ok := parseXiaomuExpiresAt(rejected, now); ok {
			t.Fatalf("越界或非法有效期不应被采用：%q", rejected)
		}
	}
}

func TestXiaomuOutcomeQueryReferenceFallsBackToOrderNo(t *testing.T) {
	// 历史认证单只有订单号时仍要能查询，不能因缺少 id 直接卡死在 pending。
	outcome := xiaomuRealnameOutcome{OrderNo: "XM-OLD-1"}
	if outcome.queryReference() != "XM-OLD-1" {
		t.Fatalf("缺少记录 ID 时应退回订单号，实际 %q", outcome.queryReference())
	}
	if (xiaomuRealnameOutcome{}).queryReference() != "" {
		t.Fatal("两者都缺失时必须返回空，交由调用方判定结果未知")
	}
}

func TestXiaomuModeTraits(t *testing.T) {
	if !xiaomuModeRequiresMobile(xiaomuModeThreeElement) {
		t.Fatal("三要素需要手机号")
	}
	if xiaomuModeRequiresMobile(xiaomuModeFaceH5) || xiaomuModeRequiresMobile(xiaomuModeTencentH5) {
		t.Fatal("跳转模式不应索取手机号")
	}
	if !xiaomuModeIsRedirect(xiaomuModeFaceH5) || !xiaomuModeIsRedirect(xiaomuModeTencentH5) {
		t.Fatal("人脸与微信模式需要跳转上游认证页")
	}
	if xiaomuModeIsRedirect(xiaomuModeThreeElement) {
		t.Fatal("三要素为同步核验，不需要跳转")
	}
}

func TestRealnamePluginIDByProviderIncludesXiaomu(t *testing.T) {
	if id := realnamePluginIDByProvider(realnameProviderXiaomu); id != "xiaomu-realname" {
		t.Fatalf("小沐 provider 应映射到 xiaomu-realname，实际 %s", id)
	}
	// 现有三家服务商映射不受影响
	if realnamePluginIDByProvider(realnameProviderAlipay) != "alipay-realname" ||
		realnamePluginIDByProvider(realnameProviderKuaitong) != "kuaitong-realname" ||
		realnamePluginIDByProvider(realnameProviderTencent) != "tencent-realname" {
		t.Fatal("原有实名插件映射被破坏")
	}
}

func TestXiaomuRealnameSessionFailureReasonKeepsUpstreamCodes(t *testing.T) {
	xerr := newXiaomuRealnameError("核验失败", false, 400, 1001, "NO_BALANCE")
	session := applyXiaomuRealnameError(realnameFaceSession{}, xerr)
	if session.Status != realnameFaceStatusFailed {
		t.Fatalf("明确错误应落为 failed，实际 %s", session.Status)
	}
	reason := tencentRealnameRecordFailureReason(session, xerr)
	for _, expected := range []string{"核验失败", "NO_BALANCE", "1001", "400"} {
		if !strings.Contains(reason, expected) {
			t.Fatalf("留档失败原因缺少 %s：%q", expected, reason)
		}
	}

	unknown := applyXiaomuRealnameError(realnameFaceSession{}, newXiaomuRealnameError("连接中断", true, 0, 0, ""))
	if unknown.Status != realnameFaceStatusUnknown {
		t.Fatalf("结果未知应落为 unknown，实际 %s", unknown.Status)
	}
}
