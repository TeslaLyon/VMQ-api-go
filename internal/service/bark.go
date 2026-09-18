package service

import (
	"VMQ-api-go/internal/config"
	"VMQ-api-go/internal/model"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// BarkMessage Bark 推送消息体定义
type BarkMessage struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Group string `json:"group,omitempty"`
	Sound string `json:"sound,omitempty"`
	Level string `json:"level,omitempty"`
	Badge int    `json:"badge,omitempty"`
	Copy  string `json:"copy,omitempty"`
	URL   string `json:"url,omitempty"`
	Icon  string `json:"icon,omitempty"`
}

// HTTPClient 用于发送请求的客户端，可在单测中替换
var barkHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
}

// SendBarkNotification 发送 Bark 推送通知
func SendBarkNotification(msg *BarkMessage) error {
	if msg == nil {
		return errors.New("bark message cannot be nil")
	}

	barkServer := "https://api.day.app"
	barkKey := "GTw9Cu67wExYB9TsujRTeP"

	if config.AppConfig != nil {
		if config.AppConfig.Bark.Server != "" {
			barkServer = strings.TrimRight(config.AppConfig.Bark.Server, "/")
		}
		if config.AppConfig.Bark.Key != "" {
			barkKey = strings.TrimSpace(config.AppConfig.Bark.Key)
		}
	}

	if barkKey == "" {
		log.Printf("[Bark通知] 未配置 Bark Key，跳过发送")
		return nil
	}

	var targetURL string
	if strings.HasPrefix(barkKey, "http://") || strings.HasPrefix(barkKey, "https://") {
		targetURL = barkKey
	} else {
		targetURL = fmt.Sprintf("%s/%s", barkServer, barkKey)
	}

	payloadBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化 Bark 消息失败: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("构建 Bark 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := barkHTTPClient.Do(req)
	if err != nil {
		log.Printf("[Bark通知失败] 目标=%s, 错误: %v", targetURL, err)
		return fmt.Errorf("发送 Bark 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	trimmedResp := strings.TrimSpace(string(respBody))

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Bark通知异常] HTTP=%d, 响应: %s", resp.StatusCode, trimmedResp)
		return fmt.Errorf("bark API 响应状态码异常: %d, body: %s", resp.StatusCode, trimmedResp)
	}

	log.Printf("[Bark通知成功] 标题=%s, 响应=%s", msg.Title, trimmedResp)
	return nil
}

// SendOrderCallbackFailedAlert 当异步回调失败时构造告警并发送 Bark 通知
func SendOrderCallbackFailedAlert(order *model.Order, failureReason string) error {
	if order == nil {
		return errors.New("order cannot be nil")
	}

	// 支付方式文案
	payTypeStr := "其他支付"
	switch order.Type {
	case model.OrderTypeWechat:
		payTypeStr = "微信支付"
	case model.OrderTypeAlipay:
		payTypeStr = "支付宝"
	}

	// 金额转换：优先使用实际支付金额 (单位: 分)
	priceVal := order.Really_price
	if priceVal <= 0 {
		priceVal = order.Price
	}
	amountYuan := float64(priceVal) / 100.0

	// 构建告警内容
	bodyLines := []string{
		fmt.Sprintf("【金额】¥%.2f (%s)", amountYuan, payTypeStr),
		fmt.Sprintf("【商户单号】%s", order.Order_id),
	}

	if order.Pay_id != "" && order.Pay_id != order.Order_id {
		bodyLines = append(bodyLines, fmt.Sprintf("【支付流水】%s", order.Pay_id))
	}

	bodyLines = append(bodyLines,
		fmt.Sprintf("【失败原因】%s", failureReason),
		fmt.Sprintf("【通知时间】%s", time.Now().Format("2006-01-02 15:04:05")),
	)

	// 管理端跳转链接
	adminURL := "https://admin.example.com"
	if config.AppConfig != nil && config.AppConfig.Server.FrontendURL != "" {
		adminURL = strings.TrimRight(config.AppConfig.Server.FrontendURL, "/")
	}
	orderURL := fmt.Sprintf("%s/order/%s", adminURL, order.Order_id)

	msg := &BarkMessage{
		Title: "⚠️ VMQ 异步回调失败告警",
		Body:  strings.Join(bodyLines, "\n"),
		Group: "支付告警",
		Sound: "alarm",
		Level: "timeSensitive",
		Badge: 1,
		Copy:  order.Order_id,
		URL:   orderURL,
		Icon:  "https://img.icons8.com/color/96/warning-shield.png",
	}

	return SendBarkNotification(msg)
}

