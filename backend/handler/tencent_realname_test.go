package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

type tencentRealnameRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn tencentRealnameRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestBuildTencentRealnameInitPayload(t *testing.T) {
	cfg := realnameConfig{TencentProductCode: "face-product", TencentUsePackage: true}
	payload := buildTencentRealnameInitPayload(
		cfg,
		"Test Name",
		"11010519491231002X",
		"https://example.com/realname-return",
	)
	want := map[string]interface{}{
		"product_code": "face-product",
		"use_package":  true,
		"name":         "Test Name",
		"idcard":       "11010519491231002X",
		"return_url":   "https://example.com/realname-return",
	}
	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("payload = %#v, want %#v", payload, want)
	}
	for _, field := range []string{"base64_image", "image_base64"} {
		if _, exists := payload[field]; exists {
			t.Fatalf("init payload must not include %s", field)
		}
	}
}

func TestBuildTencentRealnameQueryPayload(t *testing.T) {
	cfg := realnameConfig{TencentProductCode: "face-product"}
	payload := buildTencentRealnameQueryPayload(cfg, "cert-123")
	want := map[string]interface{}{
		"certify_id":   "cert-123",
		"product_code": "face-product",
	}
	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("payload = %#v, want %#v", payload, want)
	}
}

func TestParseTencentRealnameInitResponse(t *testing.T) {
	body := []byte(`{"code":0,"message":"success","data":{"certify_id":"cert-123","certify_url":"https://example.com/certify/123","cost_time":186}}`)
	result, err := parseTencentRealnameInitResponse(http.StatusOK, body)
	if err != nil {
		t.Fatalf("parse init response: %v", err)
	}
	if result.CertifyID != "cert-123" || result.AuthURL != "https://example.com/certify/123" {
		t.Fatalf("result = %#v", result)
	}
}

func TestParseTencentRealnameInitResponseRequiresSessionData(t *testing.T) {
	for _, body := range []string{
		`{"code":0,"message":"success","data":{"certify_url":"https://example.com/certify/123"}}`,
		`{"code":0,"message":"success","data":{"certify_id":"cert-123"}}`,
	} {
		_, err := parseTencentRealnameInitResponse(http.StatusOK, []byte(body))
		if err == nil || !isTencentRealnameResultUnknown(err) {
			t.Fatalf("body %s: expected unknown init result, got %v", body, err)
		}
	}
}

func TestParseTencentRealnameQueryResponse(t *testing.T) {
	body := []byte(`{"code":0,"message":"success","data":{"success":true,"message":"认证通过","cost_time":1245,"status_code":"200","raw_data":"{\"code\":\"0\",\"result\":{\"result\":\"0\"}}","is_charged":true,"charge_amount":0,"package_used":true,"duplicate_query":false,"package_id_used":10,"use_package":true}}`)
	result, pending, err := parseTencentRealnameQueryResponse(http.StatusOK, body)
	if err != nil {
		t.Fatalf("parse query response: %v", err)
	}
	if pending || !result.Passed {
		t.Fatalf("result = %#v, pending = %v", result, pending)
	}
	for _, want := range []string{"data.success=true", "已扣费=true", "套餐ID=10"} {
		if !strings.Contains(result.Detail, want) {
			t.Fatalf("detail = %q, want %q", result.Detail, want)
		}
	}
}

func TestParseTencentRealnameQueryPending(t *testing.T) {
	body := []byte(`{"code":0,"message":"success","data":{"status":"processing","message":"认证中"}}`)
	result, pending, err := parseTencentRealnameQueryResponse(http.StatusOK, body)
	if err != nil || !pending || result.Passed {
		t.Fatalf("result = %#v, pending = %v, err = %v", result, pending, err)
	}
}

func TestTencentRealnameRequestUsesVerifyAndFaceQueryContracts(t *testing.T) {
	oldClient := tencentRealnameHTTPClient
	defer func() { tencentRealnameHTTPClient = oldClient }()
	tencentRealnameHTTPClient = &http.Client{Transport: tencentRealnameRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		for _, header := range []string{"X-Api-Key", "X-Timestamp", "X-Nonce", "X-Signature"} {
			if r.Header.Get(header) == "" {
				t.Errorf("missing header %s", header)
			}
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			return nil, err
		}
		responseBody := ""
		switch r.URL.Path {
		case "/common/openapi/verify":
			if payload["product_code"] != "face-product" || payload["name"] != "Test Name" || payload["idcard"] != "11010519491231002X" {
				t.Errorf("verify payload = %#v", payload)
			}
			if _, exists := payload["base64_image"]; exists {
				t.Error("verify payload must not include base64_image")
			}
			responseBody = `{"code":0,"message":"success","data":{"certify_id":"cert-123","certify_url":"https://example.com/certify/123"}}`
		case "/common/openapi/faceQuery":
			if payload["certify_id"] != "cert-123" || payload["product_code"] != "face-product" {
				t.Errorf("faceQuery payload = %#v", payload)
			}
			responseBody = `{"code":0,"message":"success","data":{"success":true,"message":"认证通过"}}`
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			responseBody = `{"code":1,"message":"not found"}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    r,
		}, nil
	})}

	cfg := realnameConfig{
		TencentAPIKey:      "api-key",
		TencentAPISecret:   "api-secret",
		TencentBaseURL:     "https://example.test/common/openapi",
		TencentProductCode: "face-product",
		TencentUsePackage:  true,
	}
	initResult, err := tencentRealnameInitialize(cfg, "Test Name", "11010519491231002X", "https://example.com/return")
	if err != nil || initResult.CertifyID != "cert-123" {
		t.Fatalf("init result = %#v, err = %v", initResult, err)
	}
	result, pending, err := tencentRealnameQuery(cfg, initResult.CertifyID)
	if err != nil || pending || !result.Passed {
		t.Fatalf("query result = %#v, pending = %v, err = %v", result, pending, err)
	}
}

func TestParseTencentRealnameProducts(t *testing.T) {
	body := []byte(`{"code":0,"message":"success","data":{"products":[{"product_code":"cloud_face","product_name":"增强人脸","price":0.35},{"code":"idcard","name":"身份证二要素","unit_price":"0.10"}]}}`)
	products, err := parseTencentRealnameProducts(200, body)
	if err != nil {
		t.Fatalf("parse products: %v", err)
	}
	want := []realnameProductItem{
		{ProductCode: "cloud_face", Name: "增强人脸", Price: "0.35"},
		{ProductCode: "idcard", Name: "身份证二要素", Price: "0.10"},
	}
	if !reflect.DeepEqual(products, want) {
		t.Fatalf("products = %#v, want %#v", products, want)
	}
}

func TestParseTencentRealnameProductsNestedList(t *testing.T) {
	body := []byte(`{"code":0,"data":{"productList":[{"productCode":"cloud_face","productName":"增强人脸","unitPrice":0.35}]}}`)
	products, err := parseTencentRealnameProducts(200, body)
	if err != nil {
		t.Fatalf("parse nested products: %v", err)
	}
	want := []realnameProductItem{{ProductCode: "cloud_face", Name: "增强人脸", Price: "0.35"}}
	if !reflect.DeepEqual(products, want) {
		t.Fatalf("products = %#v, want %#v", products, want)
	}
}

func TestParseTencentRealnameProductsBusinessError(t *testing.T) {
	body := []byte(`{"code":1,"message":"缺少必要的签名参数","data":{"code":"AUTH_FAILED"}}`)
	_, err := parseTencentRealnameProducts(200, body)
	if err == nil {
		t.Fatal("expected product list business error")
	}
	const want = "获取产品列表失败：缺少必要的签名参数（AUTH_FAILED）"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestParseTencentRealnameVerifyResponseSuccess(t *testing.T) {
	body := []byte(`{"code":0,"message":"success","data":{"success":true,"message":"一致","certify_id":"cert-123","raw_data":{"score":96.8}}}`)
	result, err := parseTencentRealnameVerifyResponse(200, body)
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if !result.Passed || result.SerialNo != "cert-123" || result.Score != "96.8" {
		t.Fatalf("result = (%v, %q, %q), want (true, \"cert-123\", \"96.8\")", result.Passed, result.SerialNo, result.Score)
	}
	if !strings.Contains(result.Detail, "data.success=true") {
		t.Fatalf("detail = %q, want outcome basis", result.Detail)
	}
	if !strings.Contains(result.Detail, `"certify_id":"cert-123"`) {
		t.Fatalf("detail = %q, want full upstream body", result.Detail)
	}
}

func TestParseTencentRealnameVerifyResponseBusinessError(t *testing.T) {
	body := []byte(`{"code":1,"message":"无可用套餐","data":{"code":"INSUFFICIENT_PACKAGE"}}`)
	_, err := parseTencentRealnameVerifyResponse(200, body)
	if err == nil {
		t.Fatal("expected business error")
	}
	if isTencentRealnameResultUnknown(err) {
		t.Fatal("business error must not be classified as unknown")
	}
	const want = "腾讯增强人脸请求失败：无可用套餐（INSUFFICIENT_PACKAGE）；处置建议：无可用套餐或套餐不含该产品，请购买套餐或关闭套餐扣费改用余额"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
	httpStatus, responseCode, businessCode := tencentRealnameErrorMetadata(err)
	if httpStatus != http.StatusOK || responseCode != 1 || businessCode != "INSUFFICIENT_PACKAGE" {
		t.Fatalf("metadata = (%d, %d, %q), want (200, 1, %q)", httpStatus, responseCode, businessCode, "INSUFFICIENT_PACKAGE")
	}
}

func TestParseTencentRealnameVerifyResponseMalformedSuccess(t *testing.T) {
	_, err := parseTencentRealnameVerifyResponse(200, []byte(`not-json`))
	if err == nil || !isTencentRealnameResultUnknown(err) {
		t.Fatalf("malformed accepted response must be unknown, got %v", err)
	}
	const want = "腾讯增强人脸响应解析失败：HTTP 200，JSON错误：invalid character 'o' in literal null (expecting 'u')，响应正文：not-json"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
	httpStatus, responseCode, businessCode := tencentRealnameErrorMetadata(err)
	if httpStatus != http.StatusOK || responseCode != 0 || businessCode != "" {
		t.Fatalf("metadata = (%d, %d, %q), want (200, 0, empty)", httpStatus, responseCode, businessCode)
	}
}

func TestParseTencentRealnameVerifyResponseMalformedError(t *testing.T) {
	body := []byte("  <html>\n  upstream timeout  </html>  ")
	_, err := parseTencentRealnameVerifyResponse(http.StatusBadGateway, body)
	if err == nil {
		t.Fatal("expected malformed gateway response error")
	}
	if isTencentRealnameResultUnknown(err) {
		t.Fatal("non-2xx malformed response must not be classified as unknown")
	}
	const want = "腾讯增强人脸响应解析失败：HTTP 502，JSON错误：invalid character '<' looking for beginning of value，响应正文：<html> upstream timeout </html>"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
	httpStatus, responseCode, businessCode := tencentRealnameErrorMetadata(err)
	if httpStatus != http.StatusBadGateway || responseCode != 0 || businessCode != "" {
		t.Fatalf("metadata = (%d, %d, %q), want (502, 0, empty)", httpStatus, responseCode, businessCode)
	}
}

func TestParseTencentRealnameVerifyResponseIncludesDataMessage(t *testing.T) {
	body := []byte(`{"code":1,"message":"核验失败","data":{"code":"FACE_INVALID","message":"未检测到完整人脸"}}`)
	_, err := parseTencentRealnameVerifyResponse(http.StatusOK, body)
	if err == nil {
		t.Fatal("expected business error")
	}
	const want = "腾讯增强人脸请求失败：核验失败：未检测到完整人脸（FACE_INVALID）"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestParseTencentRealnameVerifyResponseNonObjectData(t *testing.T) {
	body := []byte(`{"code":1,"data":false,"exdata":null,"message":"人脸认证初始化失败: 增强人脸版请上传人脸图片","time":1785228361606}`)
	_, err := parseTencentRealnameVerifyResponse(http.StatusOK, body)
	if err == nil {
		t.Fatal("expected business error")
	}
	if isTencentRealnameResultUnknown(err) {
		t.Fatal("upstream parameter rejection must not be classified as unknown")
	}
	const want = "腾讯增强人脸请求失败：人脸认证初始化失败: 增强人脸版请上传人脸图片"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
	httpStatus, responseCode, businessCode := tencentRealnameErrorMetadata(err)
	if httpStatus != http.StatusOK || responseCode != 1 || businessCode != "" {
		t.Fatalf("metadata = (%d, %d, %q), want (200, 1, empty)", httpStatus, responseCode, businessCode)
	}
}

func TestParseTencentRealnameVerifyResponseNumericBusinessCode(t *testing.T) {
	body := []byte(`{"code":1,"message":"核验失败","data":{"code":40004,"message":"图片无效"}}`)
	_, err := parseTencentRealnameVerifyResponse(http.StatusOK, body)
	if err == nil {
		t.Fatal("expected business error")
	}
	const want = "腾讯增强人脸请求失败：核验失败：图片无效（40004）"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
	_, _, businessCode := tencentRealnameErrorMetadata(err)
	if businessCode != "40004" {
		t.Fatalf("businessCode = %q, want %q", businessCode, "40004")
	}
}

func TestParseTencentRealnameVerifyResponseSuccessCodeWithoutResultObject(t *testing.T) {
	for _, body := range []string{
		`{"code":0,"message":"success","data":false}`,
		`{"code":0,"message":"success","data":"ok"}`,
	} {
		_, err := parseTencentRealnameVerifyResponse(http.StatusOK, []byte(body))
		if err == nil || !isTencentRealnameResultUnknown(err) {
			t.Fatalf("body %s: expected unknown result, got %v", body, err)
		}
		if !strings.Contains(err.Error(), "未包含认证结果") {
			t.Fatalf("body %s: error = %q, want missing-result detail", body, err.Error())
		}
	}
}

func TestParseTencentRealnameVerifyResponseStringSuccessFlag(t *testing.T) {
	body := []byte(`{"code":0,"data":{"success":"true","certify_id":"cert-9","raw_data":{"similarity":88}}}`)
	result, err := parseTencentRealnameVerifyResponse(http.StatusOK, body)
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if !result.Passed || result.SerialNo != "cert-9" || result.Score != "88" {
		t.Fatalf("result = (%v, %q, %q), want (true, \"cert-9\", \"88\")", result.Passed, result.SerialNo, result.Score)
	}
}

// 官方通用响应契约：data.success 为权威结论，data 内同时带扣费与套餐元数据。
func TestParseTencentRealnameVerifyResponseOfficialContract(t *testing.T) {
	body := `{"code":0,"message":"success","data":{"success":true,"message":"一致","cost_time":245,"charge_amount":0,"is_charged":true,"package_used":true,"package_id_used":10,"use_package":true}}`
	result, err := parseTencentRealnameVerifyResponse(http.StatusOK, []byte(body))
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if !result.Passed {
		t.Fatalf("official success contract must pass, reason = %q", result.Reason)
	}
	for _, want := range []string{
		"data.success=true",
		"已扣费=true",
		"扣费金额=0",
		"使用套餐=true",
		"套餐ID=10",
		"请求套餐模式=true",
		"上游耗时(ms)=245",
		body,
	} {
		if !strings.Contains(result.Detail, want) {
			t.Fatalf("detail = %q, want %q", result.Detail, want)
		}
	}
}

// 公开错误码需要翻译成可执行的处置建议。
func TestTencentRealnameErrorCodeHints(t *testing.T) {
	cases := map[string]string{
		"INSUFFICIENT_BALANCE": "上游余额不足",
		"AUTH_FAILED":          "签名验证失败",
		"INVALID_PARAM":        "未传 package_id",
		"PRODUCT_NOT_FOUND":    "产品不存在或已被禁用",
	}
	for code, want := range cases {
		body := fmt.Sprintf(`{"code":1,"message":"调用失败","data":{"code":%q}}`, code)
		_, err := parseTencentRealnameVerifyResponse(http.StatusOK, []byte(body))
		if err == nil {
			t.Fatalf("code %s: expected business error", code)
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("code %s: error = %q, want hint %q", code, err.Error(), want)
		}
	}
}

func TestParseTencentRealnameVerifyResponseAlternateSuccessKeys(t *testing.T) {
	cases := []struct {
		body  string
		basis string
	}{
		{`{"code":0,"data":{"result":true,"certify_id":"c1"}}`, "data.result=true"},
		{`{"code":0,"data":{"status":"SUCCESS","certify_id":"c2"}}`, "data.status=success"},
		{`{"code":0,"data":{"pass":1,"certify_id":"c3"}}`, "data.pass=1"},
		{`{"code":0,"data":{"raw_data":{"is_match":"Y"}}}`, "data.raw_data.is_match=y"},
		{`{"code":0,"data":{"raw_data":{"Text":{"ErrCode":0}}}}`, "data.raw_data.Text.errcode=0"},
		{`{"code":0,"data":{"code":"0","certify_id":"c4"}}`, "接口返回成功码，响应未包含结论字段"},
		{`{"code":0,"data":{"certify_id":"c5"}}`, "接口返回成功码，响应未包含结论字段"},
	}
	for _, item := range cases {
		result, err := parseTencentRealnameVerifyResponse(http.StatusOK, []byte(item.body))
		if err != nil {
			t.Fatalf("body %s: parse response: %v", item.body, err)
		}
		if !result.Passed {
			t.Fatalf("body %s: want passed, got reason %q", item.body, result.Reason)
		}
		if !strings.Contains(result.Detail, item.basis) {
			t.Fatalf("body %s: detail = %q, want basis %q", item.body, result.Detail, item.basis)
		}
	}
}

func TestParseTencentRealnameVerifyResponseRejectedKeepsFullBody(t *testing.T) {
	body := `{"code":0,"data":{"result":false,"code":"FACE_MISMATCH","message":"人脸比对不通过","certify_id":"c9","raw_data":{"score":12.5}}}`
	result, err := parseTencentRealnameVerifyResponse(http.StatusOK, []byte(body))
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if result.Passed {
		t.Fatal("explicit rejection must not pass")
	}
	const wantReason = "人脸比对不通过（FACE_MISMATCH）"
	if result.Reason != wantReason {
		t.Fatalf("reason = %q, want %q", result.Reason, wantReason)
	}
	for _, want := range []string{wantReason, "data.result=false", "接口响应码：0；HTTP状态：200", body} {
		if !strings.Contains(result.Detail, want) {
			t.Fatalf("detail = %q, want %q", result.Detail, want)
		}
	}
}

func TestParseTencentRealnameVerifyResponseRejectedByBusinessCode(t *testing.T) {
	result, err := parseTencentRealnameVerifyResponse(http.StatusOK, []byte(`{"code":0,"data":{"code":"40010","message":"活体检测失败"}}`))
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if result.Passed {
		t.Fatal("non-success business code must not pass")
	}
	if !strings.Contains(result.Detail, "data.code=40010") {
		t.Fatalf("detail = %q, want business code basis", result.Detail)
	}
}

// 请求级错误也要把完整上游正文带进认证记录，而不是只留 96 字符摘要。
func TestTencentRealnameRecordFailureReasonCarriesFullBody(t *testing.T) {
	body := []byte(`{"code":1,"data":false,"message":"人脸认证初始化失败: 增强人脸版请上传人脸图片","trace":"0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789"}`)
	_, err := parseTencentRealnameVerifyResponse(http.StatusOK, body)
	if err == nil {
		t.Fatal("expected business error")
	}
	session := applyTencentRealnameError(realnameFaceSession{}, err)
	reason := tencentRealnameRecordFailureReason(session, err)
	if !strings.Contains(reason, "上游响应正文：") {
		t.Fatalf("reason = %q, want upstream body section", reason)
	}
	if !strings.Contains(reason, "0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789") {
		t.Fatalf("reason = %q, want untruncated body", reason)
	}
	if !strings.Contains(reason, "接口响应码：1") || !strings.Contains(reason, "HTTP状态：200") {
		t.Fatalf("reason = %q, want response codes", reason)
	}
}

func TestTencentRealnameSignature(t *testing.T) {
	got := tencentRealnameSignature("api-key", "1721635200", "nonce", "secret")
	const want = "ddfe32dfdebc6e90254d889a9df28e693861b43d4c367e5903abdbe8cb9efbce"
	if got != want {
		t.Fatalf("signature = %q, want %q", got, want)
	}
}

func TestTencentRealnameFaceSessionRoundTrip(t *testing.T) {
	want := realnameFaceSession{
		Provider:     realnameProviderTencent,
		OwnerType:    "user",
		UserID:       7,
		RealName:     "Test Name",
		IDCard:       "11010519491231002X",
		ReturnURL:    "https://example.com/realname-face?t=token",
		Status:       realnameFaceStatusFailed,
		FailMsg:      "腾讯增强人脸请求失败：无可用套餐（INSUFFICIENT_PACKAGE）",
		FailCode:     "INSUFFICIENT_PACKAGE",
		HTTPStatus:   http.StatusOK,
		ResponseCode: 1,
		ExpireAt:     time.Unix(1721635200, 0),
	}
	got, err := parseRealnameFaceSession(want.encode())
	if err != nil {
		t.Fatalf("parse session: %v", err)
	}
	if !got.ExpireAt.Equal(want.ExpireAt) {
		t.Fatalf("expireAt = %v, want %v", got.ExpireAt, want.ExpireAt)
	}
	got.ExpireAt = want.ExpireAt
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("session = %#v, want %#v", got, want)
	}
}

func TestApplyTencentRealnameBusinessError(t *testing.T) {
	_, err := parseTencentRealnameVerifyResponse(
		http.StatusOK,
		[]byte(`{"code":1,"message":"无可用套餐","data":{"code":"INSUFFICIENT_PACKAGE"}}`),
	)
	if err == nil {
		t.Fatal("expected business error")
	}

	session := applyTencentRealnameError(realnameFaceSession{}, err)
	if session.Status != realnameFaceStatusFailed || session.FailCode != "INSUFFICIENT_PACKAGE" || session.HTTPStatus != http.StatusOK || session.ResponseCode != 1 {
		t.Fatalf("session metadata = %#v", session)
	}
	data := realnameFaceFailureData(session)
	if data["reason"] != err.Error() || data["errorCode"] != "INSUFFICIENT_PACKAGE" || data["upstreamHttpStatus"] != http.StatusOK || data["upstreamResponseCode"] != 1 {
		t.Fatalf("failure data = %#v", data)
	}
	const wantRecordReason = "腾讯增强人脸请求失败：无可用套餐（INSUFFICIENT_PACKAGE）；处置建议：无可用套餐或套餐不含该产品，请购买套餐或关闭套餐扣费改用余额\n错误代码：INSUFFICIENT_PACKAGE；接口响应码：1；HTTP状态：200\n上游响应正文：{\"code\":1,\"message\":\"无可用套餐\",\"data\":{\"code\":\"INSUFFICIENT_PACKAGE\"}}"
	if got := tencentRealnameRecordFailureReason(session, err); got != wantRecordReason {
		t.Fatalf("record failure reason = %q, want %q", got, wantRecordReason)
	}
}

func TestApplyTencentRealnameUnknownResponse(t *testing.T) {
	_, err := parseTencentRealnameVerifyResponse(http.StatusOK, []byte(`not-json`))
	if err == nil {
		t.Fatal("expected malformed response error")
	}

	session := applyTencentRealnameError(realnameFaceSession{}, err)
	const wantReason = "腾讯增强人脸服务响应中断，认证结果无法确认，请返回电脑端重新发起认证；详细错误：腾讯增强人脸响应解析失败：HTTP 200，JSON错误：invalid character 'o' in literal null (expecting 'u')，响应正文：not-json"
	if session.Status != realnameFaceStatusUnknown || session.FailMsg != wantReason || session.HTTPStatus != http.StatusOK {
		t.Fatalf("unknown session = %#v", session)
	}
	data := realnameFaceFailureData(session)
	if data["status"] != "failed" || data["reason"] != wantReason || data["upstreamHttpStatus"] != http.StatusOK {
		t.Fatalf("unknown failure data = %#v", data)
	}
	if _, exists := data["errorCode"]; exists {
		t.Fatalf("unexpected business error code in malformed response: %#v", data)
	}
	if got := tencentRealnameRecordFailureReason(session, err); got != wantReason+"\nHTTP状态：200\n上游响应正文：not-json" {
		t.Fatalf("record failure reason = %q", got)
	}
}

func TestRealnameFacePassedDataHasNoErrorMetadata(t *testing.T) {
	data := realnameFacePassedData(realnameFaceSession{RealName: "测试", IDCard: "11010519491231002X"})
	for _, key := range []string{"reason", "errorCode", "upstreamHttpStatus", "upstreamResponseCode"} {
		if _, exists := data[key]; exists {
			t.Fatalf("passed data contains %q: %#v", key, data)
		}
	}
}

func TestParseLegacyRealnameFaceSessionWithoutErrorMetadata(t *testing.T) {
	raw := `{"provider":"tencent","ownerType":"user","ownerId":7,"realName":"Test Name","idCard":"11010519491231002X","status":"failed","failMsg":"旧版失败信息","expireAt":"2030-01-01T00:00:00Z"}`
	session, err := parseRealnameFaceSession(raw)
	if err != nil {
		t.Fatalf("parse legacy session: %v", err)
	}
	if session.FailMsg != "旧版失败信息" || session.FailCode != "" || session.HTTPStatus != 0 || session.ResponseCode != 0 {
		t.Fatalf("legacy session = %#v", session)
	}
}

func TestKuaitongPostIDCard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart form: %v", err)
		}
		want := map[string]string{
			"token": "access-token", "idCard": "11010519491231002X", "realName": "Test Name",
		}
		for key, value := range want {
			if got := r.FormValue(key); got != value {
				t.Errorf("form[%q] = %q, want %q", key, got, value)
			}
		}
		if _, exists := r.MultipartForm.Value["imgBase64"]; exists {
			t.Error("two-element request must not include imgBase64")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":200,"serialNo":"serial-1","message":"success","code":10000}`))
	}))
	defer server.Close()

	message, serialNo, retry, err := kuaitongPostIDCardWithClient(
		server.Client(), server.URL, "access-token", "Test Name", "11010519491231002X",
	)
	if err != nil {
		t.Fatalf("post ID card: %v", err)
	}
	if message != "" || serialNo != "serial-1" || retry {
		t.Fatalf("result = (%q, %q, %v), want (\"\", \"serial-1\", false)", message, serialNo, retry)
	}
}

func TestKuaitongPostIDCardBusinessAndAuthErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/expired" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"status":403,"message":"token expired","code":40301}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":400,"serialNo":"serial-2","message":"身份信息不匹配","code":40001}`))
	}))
	defer server.Close()

	message, serialNo, retry, err := kuaitongPostIDCardWithClient(
		server.Client(), server.URL+"/business", "token", "Test Name", "11010519491231002X",
	)
	if err != nil || message != "身份信息不匹配" || serialNo != "serial-2" || retry {
		t.Fatalf("business result = (%q, %q, %v, %v)", message, serialNo, retry, err)
	}

	message, serialNo, retry, err = kuaitongPostIDCardWithClient(
		server.Client(), server.URL+"/expired", "token", "Test Name", "11010519491231002X",
	)
	if err != nil || message != "" || serialNo != "" || !retry {
		t.Fatalf("auth result = (%q, %q, %v, %v)", message, serialNo, retry, err)
	}
}

func TestValidKuaitongAuthType(t *testing.T) {
	if !validKuaitongAuthType(kuaitongAuthTypeFace) || !validKuaitongAuthType(kuaitongAuthTypeTwoElement) {
		t.Fatal("supported Kuaitong auth type was rejected")
	}
	if validKuaitongAuthType("unknown") {
		t.Fatal("unsupported Kuaitong auth type was accepted")
	}
}
