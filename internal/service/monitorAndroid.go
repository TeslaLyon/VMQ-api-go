package service

import (
	"VMQ-api-go/internal/model"
	"VMQ-api-go/internal/repository"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"
)

type MonitorAndroidService interface {
	ProcessMonitorHeart(req *model.MonitorHeartRequest) error
	ProcessMonitorPush(req *model.MonitorPushRequest) error
	// 监控端状态检查
	CheckAndUpdateMonitorStatus() error
}

type monitorAndroidService struct {
	userRepo  repository.UserRepository //TODO 研究这样写的目的 userRepo是否可以替换为UserService接口，解耦服务层和仓储层
	orderRepo repository.OrderRepository
	startTime time.Time
}

var (
	ErrSettingNotFound = errors.New("setting not found")
	ErrInvalidSign     = errors.New("invalid signature")
)

func NewMonitorAndroidService(userRepo repository.UserRepository, orderRepo repository.OrderRepository) MonitorAndroidService {
	return &monitorAndroidService{
		userRepo:  userRepo,
		orderRepo: orderRepo,
		startTime: time.Now(),
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

	price := int64(req.Price)

	strType := strconv.FormatInt(req.Type, 10)
	strPrice := fmt.Sprintf("%.2f", req.Price)

	// 验证签名 - 适配Android端格式：md5(type + price + timestamp + key)
	signStr := strType + strPrice + req.T + user.GetKey()
	log.Printf("type: %s", strType)
	log.Printf("strPrice: %s", strPrice)
	log.Printf("price: %d", price)
	log.Printf("timestamp: %s", req.T)
	log.Printf("key: %s", user.GetKey())
	log.Printf("签名字符串: %s", signStr)
	// expectedSign := fmt.Sprintf("%x", md5.Sum([]byte(signStr)))
	hash := md5.Sum([]byte(signStr))
	expectedSign := hex.EncodeToString(hash[:])
	log.Printf("expectedSign: %s", expectedSign)
	if req.Sign != expectedSign {
		return ErrInvalidSign
	}

	// 根据价格和类型查找对应的待支付订单

	// if err != nil {
	// 	return fmt.Errorf("invalid price: %s", req.Price)
	// }

	orderType, err := strconv.Atoi(strType)
	if err != nil {
		return fmt.Errorf("invalid type: %s", req.Type)
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

		err = s.orderRepo.Update(order)
		if err != nil {
			log.Printf("更新订单状态失败: 订单ID=%s, 错误=%v", order.Order_id, err)
		} else {
			log.Printf("订单支付成功: 订单ID=%s, 用户ID=%d, 价格=%d", order.Order_id, user.ID, price)
		}
	}

	// 更新最后支付时间
	now := time.Now().Unix()
	user.Lastpay = &now
	return s.userRepo.Update(user)
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
