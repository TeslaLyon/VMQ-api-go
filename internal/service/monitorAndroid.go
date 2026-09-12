package service

import (
	"VMQ-api-go/internal/config"
	"VMQ-api-go/internal/model"
	"VMQ-api-go/internal/repository"
	"crypto/hmac" // 🌟 用于 HMAC 运算
	"crypto/md5"
	cryptoRand "crypto/rand" // 🌟 用于生成安全的随机 Nonce
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type MonitorAndroidService interface {
	ProcessMonitorHeart(req *model.MonitorHeartRequest) error
	ProcessMonitorPush(req *model.MonitorPushRequest) error
	// 监控端状态检查
	CheckAndUpdateMonitorStatus() error
}

type monitorAndroidService struct {
	userRepo     repository.UserRepository //TODO 研究这样写的目的 userRepo是否可以替换为UserService接口，解耦服务层和仓储层
	orderRepo    repository.OrderRepository
	tmpPriceRepo repository.TmpPriceRepository
	startTime    time.Time
}

var (
	ErrSettingNotFound = errors.New("setting not found")
	ErrInvalidSign     = errors.New("invalid signature")
)

func NewMonitorAndroidService(userRepo repository.UserRepository, orderRepo repository.OrderRepository, tmpPriceRepo repository.TmpPriceRepository) MonitorAndroidService {
	return &monitorAndroidService{
		userRepo:     userRepo,
		orderRepo:    orderRepo,
		tmpPriceRepo: tmpPriceRepo,
		startTime:    time.Now(),
	}
}

func (s *monitorAndroidService) ProcessMonitorHeart(req *model.MonitorHeartRequest) error {
	// 确定用户
	var user *model.User
	var err error

	if req.AppID != "" {
		log.Printf("心跳请求包含AppID: %s，尝试查找对应用户", req.AppID)
		user, err = s.userRepo.GetByAppID(req.AppID)
		if err != nil {
			log.Printf("AppID查找失败: %s, 错误: %v", req.AppID, err)
			return fmt.Errorf("invalid appid: %s", req.AppID)
		}
		log.Printf("AppID %s 对应用户ID: %d", req.AppID, user.ID)
	} else {
		log.Printf("心跳请求未包含AppID，使用默认用户ID: 1")
		user, err = s.userRepo.GetByID(1)
		if err != nil {
			return err
		}
	}

	// 验证签名 - 适配Android端格式：md5(timestamp + key)
	expectedSign := fmt.Sprintf("%x", md5.Sum([]byte(req.T+user.GetKey())))
	if req.Sign != expectedSign {
		return ErrInvalidSign
	}

	// 更新心跳时间和监控状态
	now := time.Now().Unix()
	jkstate := int16(1) // 假设1表示在线状态
	user.Lastheart = &now
	user.Jkstate = &jkstate

	return s.userRepo.Update(user)
}

// ProcessMonitorPush 处理监控推送
func (s *monitorAndroidService) ProcessMonitorPush(req *model.MonitorPushRequest) error {
	// 确定用户
	var user *model.User
	var err error

	if req.AppID != "" {
		user, err = s.userRepo.GetByAppID(req.AppID)
		if err != nil {
			return fmt.Errorf("invalid appid: %s", req.AppID)
		}
	} else {
		user, err = s.userRepo.GetByID(1)
		if err != nil {
			return err
		}
	}

	price := int64(req.Price * 100)

	strType := strconv.FormatInt(req.Type, 10)
	strPrice := fmt.Sprintf("%.2f", req.Price)

	// 验证签名 - 适配Android端格式：md5(type + price + timestamp + key)
	signStr := strType + strPrice + req.T + user.GetKey()
	// log.Printf("type: %s", strType)
	// log.Printf("strPrice: %s", strPrice)
	// log.Printf("price: %d", price)
	// log.Printf("timestamp: %s", req.T)
	// log.Printf("key: %s", user.GetKey())
	// log.Printf("签名字符串: %s", signStr)
	// expectedSign := fmt.Sprintf("%x", md5.Sum([]byte(signStr)))
	hash := md5.Sum([]byte(signStr))
	expectedSign := hex.EncodeToString(hash[:])
	// log.Printf("expectedSign: %s", expectedSign)
	if req.Sign != expectedSign {
		return ErrInvalidSign
	}

	// 根据价格和类型查找对应的待支付订单

	// if err != nil {
	// 	return fmt.Errorf("invalid price: %s", req.Price)
	// }

	orderType, err := strconv.Atoi(strType)
	if err != nil {
		return fmt.Errorf("invalid type: %s", strType)
	}

	// 查找该用户最近创建的匹配订单
	order, err := s.orderRepo.GetRecentPendingOrderByPriceAndType(user.ID, price, orderType)
	if err != nil {
		log.Printf("未找到匹配的订单: 用户ID=%d, 价格=%d, 类型=%d, 错误=%v", user.ID, price, orderType, err)
		// 即使没找到订单，也更新lastpay时间
	} else {
		// 更新订单状态为已支付
		order.State = model.OrderStatusPaid
		order.Pay_date = time.Now().Unix()
		// TODO:即使没有对应金额、state 为 0 的订单也返回success 是否是正确的？
		err = s.orderRepo.Update(order)
		if err != nil {
			log.Printf("更新订单状态失败: 订单ID=%s, 错误=%v", order.Order_id, err)
		} else {
			delete_err := s.tmpPriceRepo.DeleteWithOID(order.Order_id)
			if delete_err != nil {
				log.Printf("ProcessMonitorPush删除临时价格数据失败: 订单ID=%s, 错误=%v", order.Order_id, delete_err)
			}
			notifyUrl := order.Notify_url
			parsedURL, _ := url.Parse(notifyUrl)
			query := parsedURL.Query()
			query.Set("order_id", fmt.Sprint(order.Order_id))
			query.Set("type", fmt.Sprint(order.Type))
			query.Set("price", fmt.Sprint(order.Price))
			query.Set("reallyPrice", fmt.Sprint(order.Really_price))
			parsedURL.RawQuery = query.Encode()
			finalNotifyURL := parsedURL.String()
			log.Printf("订单支付成功: 订单ID=%s, 用户ID=%d, 价格=%d，准备回调商户: %s", order.Order_id, user.ID, price, finalNotifyURL)

			// 🌟 核心：使用独立 Goroutine 异步向商户发送回调通知，避免阻塞监控端请求
			// 异步发起通知
			go func(targetURL string, orderID string, appKey string, appSecret string) {
				client := &http.Client{Timeout: 5 * time.Second}
				req, reqErr := http.NewRequest(http.MethodGet, targetURL, nil)
				if reqErr != nil {
					log.Printf("[异步回调异常] 订单ID=%s, 创建请求失败: %v", orderID, reqErr)
					return
				}

				// 基础环境标识
				req.Header.Set("User-Agent", "VMQ-Monitor-Notifier/1.0")

				// 🌟 核心：计算并批量装配认证 Header
				headers := buildSignedHeaders(appKey, appSecret)
				for k, v := range headers {
					req.Header.Set(k, v)
				}

				// 发起请求
				resp, doErr := client.Do(req)
				if doErr != nil {
					log.Printf("[异步回调失败] 订单ID=%s, 请求错误: %v", orderID, doErr)
					return
				}
				defer resp.Body.Close()

				bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
				responseContent := strings.TrimSpace(string(bodyBytes))

				if resp.StatusCode == http.StatusOK && strings.EqualFold(responseContent, "success") {
					log.Printf("[异步回调成功] 订单ID=%s, 商户返回 success", orderID)
				} else {
					log.Printf("[异步回调未确认] 订单ID=%s, HTTP=%d, Body=%s", orderID, resp.StatusCode, responseContent)
				}
			}(finalNotifyURL, order.Order_id, config.AppConfig.Server.OpenapiKey, config.AppConfig.Server.OpenapiValue) // 🌟 传入凭据
			log.Printf("订单支付成功: 订单ID=%s, 用户ID=%d, 价格=%d", order.Order_id, user.ID, price)
		}
	}

	// 更新最后支付时间
	now := time.Now().Unix()
	user.Lastpay = &now
	return s.userRepo.Update(user)
}

func getAppSecretByKey(appKey string) string {
	if appKey == config.AppConfig.Server.OpenapiKey {
		// 匹配成功，返回配置中的 openapi_value
		return config.AppConfig.Server.OpenapiValue
	}
	return ""
}

// CheckAndUpdateMonitorStatus 检查并更新所有用户的监控端状态
func (s *monitorAndroidService) CheckAndUpdateMonitorStatus() error {
	// 心跳超时时间：180秒（3分钟）
	const heartbeatTimeout = 180
	currentTime := time.Now().Unix()

	// 获取所有用户（分页获取，避免一次性加载过多数据）
	page := 1
	limit := 100

	for {
		users, total, err := s.userRepo.GetUsers(page, limit, "")
		if err != nil {
			return err
		}

		for _, user := range users {
			if user.Lastheart == nil || *user.Lastheart == 0 {
				// 没有心跳记录，设置为掉线状态
				jkstate := int16(0)
				user.Jkstate = &jkstate
				s.userRepo.Update(user)
				continue
			}

			// 检查心跳是否超时
			if currentTime-*user.Lastheart >= heartbeatTimeout {
				// 心跳超时，设置为掉线状态
				jkstate := int16(0)
				user.Jkstate = &jkstate
				s.userRepo.Update(user)
			}
			// 如果心跳正常，不需要更新，因为心跳接口会自动设置为1
		}

		// 检查是否还有更多用户
		if int64(page*limit) >= total {
			break
		}
		page++
	}

	return nil
}

const nonceChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// generateRandomNonce 生成指定长度的高熵随机字符串 (对标 Laravel 的 Str::random)
func generateRandomNonce(length int) string {
	bytes := make([]byte, length)
	if _, err := cryptoRand.Read(bytes); err != nil {
		// 极端保底：降级为时间戳随机
		return fmt.Sprintf("%016x", time.Now().UnixNano())[:length]
	}
	for i, b := range bytes {
		bytes[i] = nonceChars[int(b)%len(nonceChars)]
	}
	return string(bytes)
}

// buildSignedHeaders 构造带有 HMAC-SHA256 签名的 Header 映射
func buildSignedHeaders(appKey, appSecret string) map[string]string {
	// 1. 10 位秒级时间戳与 16 位随机 Nonce
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := generateRandomNonce(16)

	// 2. 拼接载荷：AppKey + Timestamp + Nonce
	signPayload := appKey + timestamp + nonce

	// 3. HMAC-SHA256 签名计算并转为十六进制小写
	h := hmac.New(sha256.New, []byte(appSecret))
	h.Write([]byte(signPayload))
	signature := hex.EncodeToString(h.Sum(nil))

	return map[string]string{
		"X-App-Key":   appKey,
		"X-Timestamp": timestamp,
		"X-Nonce":     nonce,
		"X-Signature": signature,
	}
}
