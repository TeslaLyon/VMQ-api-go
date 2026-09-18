package service

import (
	"VMQ-api-go/internal/config"
	"VMQ-api-go/internal/model"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type mockTransport struct {
	roundTrip func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func TestSendBarkNotification(t *testing.T) {
	var receivedMsg BarkMessage
	var receivedHeader http.Header

	oldClient := barkHTTPClient
	defer func() { barkHTTPClient = oldClient }()

	barkHTTPClient = &http.Client{
		Transport: &mockTransport{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				receivedHeader = req.Header
				bodyBytes, _ := io.ReadAll(req.Body)
				_ = json.Unmarshal(bodyBytes, &receivedMsg)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(`{"code":200,"message":"success"}`)),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	config.AppConfig = &config.Config{
		Bark: config.BarkConfig{
			Server: "https://api.day.app",
			Key:    "test-device-key",
		},
		Server: config.ServerConfig{
			FrontendURL: "https://admin.example.com",
		},
	}

	testMsg := &BarkMessage{
		Title: "⚠️ VMQ 异步回调失败告警",
		Body:  "【金额】¥88.00 (微信支付)\n【商户单号】ORD20260916001\n【失败原因】HTTP 504 Gateway Timeout (重试 3 次均失败)",
		Group: "支付告警",
		Sound: "alarm",
		Level: "timeSensitive",
		Badge: 1,
		Copy:  "ORD20260916001",
		URL:   "https://admin.example.com/order/ORD20260916001",
		Icon:  "https://img.icons8.com/color/96/warning-shield.png",
	}

	err := SendBarkNotification(testMsg)
	if err != nil {
		t.Fatalf("SendBarkNotification failed: %v", err)
	}

	if !strings.Contains(receivedHeader.Get("Content-Type"), "application/json") {
		t.Errorf("expected Content-Type application/json, got: %s", receivedHeader.Get("Content-Type"))
	}
	if receivedMsg.Title != testMsg.Title {
		t.Errorf("expected title %s, got %s", testMsg.Title, receivedMsg.Title)
	}
	if receivedMsg.Group != "支付告警" {
		t.Errorf("expected group 支付告警, got %s", receivedMsg.Group)
	}
	if receivedMsg.Sound != "alarm" {
		t.Errorf("expected sound alarm, got %s", receivedMsg.Sound)
	}
	if receivedMsg.Level != "timeSensitive" {
		t.Errorf("expected level timeSensitive, got %s", receivedMsg.Level)
	}
	if receivedMsg.Badge != 1 {
		t.Errorf("expected badge 1, got %d", receivedMsg.Badge)
	}
	if receivedMsg.Copy != "ORD20260916001" {
		t.Errorf("expected copy ORD20260916001, got %s", receivedMsg.Copy)
	}
	if receivedMsg.URL != "https://admin.example.com/order/ORD20260916001" {
		t.Errorf("expected URL https://admin.example.com/order/ORD20260916001, got %s", receivedMsg.URL)
	}
}

func TestSendOrderCallbackFailedAlert(t *testing.T) {
	var receivedMsg BarkMessage

	oldClient := barkHTTPClient
	defer func() { barkHTTPClient = oldClient }()

	barkHTTPClient = &http.Client{
		Transport: &mockTransport{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				_ = json.Unmarshal(bodyBytes, &receivedMsg)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(`{"code":200,"message":"success"}`)),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	config.AppConfig = &config.Config{
		Bark: config.BarkConfig{
			Server: "https://api.day.app",
			Key:    "test-key",
		},
		Server: config.ServerConfig{
			FrontendURL: "https://admin.test.com",
		},
	}

	order := &model.Order{
		Order_id:     "ORD20260916001",
		Pay_id:       "PAY998877",
		Type:         model.OrderTypeWechat,
		Price:        8800, // 88.00 元
		Really_price: 8800,
	}

	err := SendOrderCallbackFailedAlert(order, "HTTP 504 Gateway Timeout (重试 3 次均失败)", "Gateway Timeout")
	if err != nil {
		t.Fatalf("SendOrderCallbackFailedAlert failed: %v", err)
	}

	if !strings.Contains(receivedMsg.Body, "【金额】¥88.00 (微信支付)") {
		t.Errorf("body does not contain expected amount and payment type, got: %s", receivedMsg.Body)
	}
	if !strings.Contains(receivedMsg.Body, "【商户单号】ORD20260916001") {
		t.Errorf("body does not contain order id, got: %s", receivedMsg.Body)
	}
	if !strings.Contains(receivedMsg.Body, "【失败原因】HTTP 504 Gateway Timeout (重试 3 次均失败)") {
		t.Errorf("body does not contain failure reason, got: %s", receivedMsg.Body)
	}
	if !strings.Contains(receivedMsg.Body, "【返回内容】Gateway Timeout") {
		t.Errorf("body does not contain return body, got: %s", receivedMsg.Body)
	}
	if receivedMsg.Copy != "ORD20260916001" {
		t.Errorf("expected copy ORD20260916001, got %s", receivedMsg.Copy)
	}
	if receivedMsg.URL != "https://admin.test.com/order/ORD20260916001" {
		t.Errorf("expected URL https://admin.test.com/order/ORD20260916001, got %s", receivedMsg.URL)
	}
}

func TestSendMerchantCallbackWithRetry_Success(t *testing.T) {
	var attempts int32
	var barkCalls int32

	oldBarkClient := barkHTTPClient
	oldCallbackClient := callbackHTTPClient
	oldInterval := callbackRetryInterval
	defer func() {
		barkHTTPClient = oldBarkClient
		callbackHTTPClient = oldCallbackClient
		callbackRetryInterval = oldInterval
	}()

	callbackHTTPClient = &http.Client{
		Transport: &mockTransport{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				atomic.AddInt32(&attempts, 1)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("success")),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	barkHTTPClient = &http.Client{
		Transport: &mockTransport{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				atomic.AddInt32(&barkCalls, 1)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(`{"code":200}`)),
					Header:     make(http.Header),
				}, nil
			},
		},
	}
	callbackRetryInterval = 1 * time.Millisecond

	config.AppConfig = &config.Config{
		Bark: config.BarkConfig{
			Server: "https://api.day.app",
			Key:    "test-key",
		},
	}

	order := &model.Order{
		Order_id:     "ORD_SUCCESS_001",
		State:        model.OrderStatusPaid,
		Type:         model.OrderTypeAlipay,
		Really_price: 1000,
	}

	sendMerchantCallbackWithRetry(order, "https://merchant.example.com/callback", "testKey", "testSecret", nil)

	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 merchant attempt, got %d", atomic.LoadInt32(&attempts))
	}
	if atomic.LoadInt32(&barkCalls) != 0 {
		t.Errorf("expected 0 bark alerts on success, got %d", atomic.LoadInt32(&barkCalls))
	}
	if order.State != model.OrderStatusPaid {
		t.Errorf("expected order state to remain OrderStatusPaid, got: %d", order.State)
	}
}

func TestSendMerchantCallbackWithRetry_FailureTriggersBark(t *testing.T) {
	var attempts int32
	var barkCalls int32
	var receivedBarkMsg BarkMessage

	oldBarkClient := barkHTTPClient
	oldCallbackClient := callbackHTTPClient
	oldInterval := callbackRetryInterval
	defer func() {
		barkHTTPClient = oldBarkClient
		callbackHTTPClient = oldCallbackClient
		callbackRetryInterval = oldInterval
	}()

	callbackHTTPClient = &http.Client{
		Transport: &mockTransport{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				atomic.AddInt32(&attempts, 1)
				return &http.Response{
					StatusCode: http.StatusGatewayTimeout,
					Body:       io.NopCloser(bytes.NewBufferString("Gateway Timeout")),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	barkHTTPClient = &http.Client{
		Transport: &mockTransport{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				atomic.AddInt32(&barkCalls, 1)
				bodyBytes, _ := io.ReadAll(req.Body)
				_ = json.Unmarshal(bodyBytes, &receivedBarkMsg)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(`{"code":200}`)),
					Header:     make(http.Header),
				}, nil
			},
		},
	}
	callbackRetryInterval = 1 * time.Millisecond

	config.AppConfig = &config.Config{
		Bark: config.BarkConfig{
			Server: "https://api.day.app",
			Key:    "test-key",
		},
		Server: config.ServerConfig{
			FrontendURL: "https://admin.example.com",
		},
	}

	order := &model.Order{
		Order_id:     "ORD20260916001",
		State:        model.OrderStatusPaid,
		Type:         model.OrderTypeWechat,
		Price:        8800,
		Really_price: 8800,
	}

	sendMerchantCallbackWithRetry(order, "https://merchant.example.com/callback", "testKey", "testSecret", nil)

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 retry attempts, got %d", atomic.LoadInt32(&attempts))
	}
	if atomic.LoadInt32(&barkCalls) != 1 {
		t.Errorf("expected 1 bark alert on failure, got %d", atomic.LoadInt32(&barkCalls))
	}
	if !strings.Contains(receivedBarkMsg.Body, "HTTP 504 Gateway Timeout (重试 3 次均失败)") {
		t.Errorf("bark alert body should specify 504 retry failure, got: %s", receivedBarkMsg.Body)
	}
	if !strings.Contains(receivedBarkMsg.Body, "【金额】¥88.00 (微信支付)") {
		t.Errorf("bark alert body should contain amount and method, got: %s", receivedBarkMsg.Body)
	}
	if !strings.Contains(receivedBarkMsg.Body, "【返回内容】Gateway Timeout") {
		t.Errorf("bark alert body should contain return body, got: %s", receivedBarkMsg.Body)
	}
	if order.State != model.OrderStatusNotifyFailed {
		t.Errorf("expected order state to be OrderStatusNotifyFailed (2), got: %d", order.State)
	}
}

func TestSendMerchantCallbackWithRetry_RetrySucceeds(t *testing.T) {
	var attempts int32
	var barkCalls int32

	oldBarkClient := barkHTTPClient
	oldCallbackClient := callbackHTTPClient
	oldInterval := callbackRetryInterval
	defer func() {
		barkHTTPClient = oldBarkClient
		callbackHTTPClient = oldCallbackClient
		callbackRetryInterval = oldInterval
	}()

	callbackHTTPClient = &http.Client{
		Transport: &mockTransport{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				n := atomic.AddInt32(&attempts, 1)
				if n < 2 {
					return &http.Response{
						StatusCode: http.StatusBadGateway,
						Body:       io.NopCloser(bytes.NewBufferString("bad gateway")),
						Header:     make(http.Header),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("SUCCESS")),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	barkHTTPClient = &http.Client{
		Transport: &mockTransport{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				atomic.AddInt32(&barkCalls, 1)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(`{"code":200}`)),
					Header:     make(http.Header),
				}, nil
			},
		},
	}
	callbackRetryInterval = 1 * time.Millisecond

	config.AppConfig = &config.Config{
		Bark: config.BarkConfig{
			Server: "https://api.day.app",
			Key:    "test-key",
		},
	}

	order := &model.Order{
		Order_id:     "ORD_RETRY_OK_001",
		State:        model.OrderStatusPaid,
		Type:         model.OrderTypeAlipay,
		Really_price: 2000,
	}

	sendMerchantCallbackWithRetry(order, "https://merchant.example.com/callback", "testKey", "testSecret", nil)

	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts before success, got %d", atomic.LoadInt32(&attempts))
	}
	if atomic.LoadInt32(&barkCalls) != 0 {
		t.Errorf("expected 0 bark alerts when retry succeeds, got %d", atomic.LoadInt32(&barkCalls))
	}
	if order.State != model.OrderStatusPaid {
		t.Errorf("expected order state to remain OrderStatusPaid, got: %d", order.State)
	}
}

// TestRealBarkNotify_WithRetry 直连真实 Bark 服务器，用于手动测试重试机制并让手机真实收到通知
func TestRealBarkNotify_WithRetry(t *testing.T) {
	// 加载真实配置
	_ = config.LoadConfig("../../")

	// 模拟一个持续返回 504 的商户服务
	var attempts int32
	mockMerchantClient := &http.Client{
		Transport: &mockTransport{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				atomic.AddInt32(&attempts, 1)
				return &http.Response{
					StatusCode: http.StatusGatewayTimeout,
					Body:       io.NopCloser(bytes.NewBufferString("Gateway Timeout")),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	// 替换回调客户端与重试间隔（每次重试间隔 1 秒）
	oldCallbackClient := callbackHTTPClient
	oldInterval := callbackRetryInterval
	defer func() {
		callbackHTTPClient = oldCallbackClient
		callbackRetryInterval = oldInterval
	}()

	callbackHTTPClient = mockMerchantClient
	callbackRetryInterval = 1 * time.Second

	// 保证 Bark 请求走系统真实网络（不 Mock）
	barkHTTPClient = &http.Client{Timeout: 10 * time.Second}

	order := &model.Order{
		Order_id:     "ORD20260918001",
		Pay_id:       "PAY20260918999",
		Type:         model.OrderTypeWechat,
		Price:        8800,
		Really_price: 8800,
	}

	t.Logf(">>> 开始测试商户回调重试机制...")
	sendMerchantCallbackWithRetry(order, "https://mock.merchant.com/notify", "testKey", "testSecret", nil)
	t.Logf(">>> 回调重试结束，已向真实 Bark 发送告警，请查看手机！")
}


