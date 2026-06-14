package model

import (
	"time"

	"gorm.io/gorm"
)

// LoginRequest 对应 Ant Design Pro 登录表单提交的字段
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 对应 Ant Design Pro 期待的登录返回格式
type LoginResponse struct {
	Status string `json:"status"` // "ok" 或 "error"
	Token  string `json:"token"`  // JWT Token
}

// UserInfo 对应 Ant Design Pro /api/currentUser 期待的用户信息格式
type User struct {
	// 基本信息
	ID       uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Username string `json:"username" gorm:"uniqueIndex;size:50;not null;column:username"`
	Email    string `json:"email" gorm:"uniqueIndex;size:100;not null"`
	Pass     string `json:"-" gorm:"size:191;not null;column:pass"`
	Role     string `json:"role" gorm:"type:varchar(50);not null;default:'admin'"`
	Status   int16  `json:"status" gorm:"type:smallint;not null;default:1;check:status IN (0,1);column:status"`

	// API认证配置
	PayPageSalt    *string `json:"pay_page_salt" gorm:"size:32;column:pay_page_salt"`
	MonitorAppSalt *string `json:"monitor_app_salt" gorm:"size:32;column:monitor_app_salt"`
	AppId          *string `json:"appid" gorm:"uniqueIndex;size:32;column:appid"`

	// 支付回调配置
	NotifyUrl *string `json:"notify_url" gorm:"size:255;column:notify_url"`
	ReturnUrl *string `json:"return_url" gorm:"size:255;column:return_url"`

	// 订单配置
	Close *int  `json:"close" gorm:"default:5;column:close"`
	PayQf *int8 `json:"pay_qufen" gorm:"type:smallint;default:1;column:pay_qufen;comment:'区分方式 0:减少 1:增加'"`

	// 收款码配置
	Wxpay  *string `json:"wxpay" gorm:"type:text;column:wxpay"`
	Zfbpay *string `json:"zfbpay" gorm:"type:text;column:zfbpay"`

	// 监控状态
	Lastheart *int64 `json:"lastheart" gorm:"column:lastheart"`
	Lastpay   *int64 `json:"lastpay" gorm:"column:lastpay"`
	Jkstate   *int16 `json:"jkstate" gorm:"type:smallint;default:0;column:jkstate"`

	// 登录信息
	Last_login_time *int64  `json:"last_login_time"`
	Last_login_ip   *string `json:"last_login_ip" gorm:"size:45"`

	// 时间戳
	Created_at int64 `json:"created_at" gorm:"not null"`
	Updated_at int64 `json:"updated_at" gorm:"not null"`
}

// CreateUserRequest 创建用户请求（管理员使用）
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email,max=100"`
	Password string `json:"password" binding:"required,min=6,max=50"`
	Role     string `json:"role" binding:"required,oneof=admin super_admin"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Username string `json:"username" binding:"omitempty,min=3,max=50"`
	Email    string `json:"email" binding:"omitempty,email,max=100"`
	Role     string `json:"role" binding:"omitempty,oneof=admin super_admin"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	u.Created_at = now
	u.Updated_at = now

	return nil
}

// BeforeUpdate 是一个方法，用于在记录更新前执行
// 它是 GORM 框架提供的一个钩子方法，会在数据库记录更新前被自动调用
// 参数 tx 是一个 GORM 数据库事务对象，用于操作数据库
// 返回值 error，如果返回错误则更新操作将被中止
func (u *User) BeforeUpdate(tx *gorm.DB) error {
	// 将 Updated_at 字段更新为当前时间戳
	// 使用 Unix() 方法获取当前时间的 Unix 时间戳（秒级）
	u.Updated_at = time.Now().Unix()
	// 返回 nil 表示方法执行成功，更新操作可以继续进行
	return nil
}

func (u *User) GetKey() string {
	if u.MonitorAppSalt != nil {
		return *u.MonitorAppSalt
	}
	return ""
}

// GetAppId 获取应用ID
func (u *User) GetAppId() string {
	if u.AppId != nil {
		return *u.AppId
	}
	return ""
}

// GetNotifyUrl 获取异步回调地址
func (u *User) GetNotifyUrl() string {
	if u.NotifyUrl != nil {
		return *u.NotifyUrl
	}
	return ""
}

// GetReturnUrl 获取同步返回地址
func (u *User) GetReturnUrl() string {
	if u.ReturnUrl != nil {
		return *u.ReturnUrl
	}
	return ""
}

// GetClose 获取订单超时时间
func (u *User) GetClose() int {
	if u.Close != nil {
		return *u.Close
	}
	return 5 // 默认5分钟
}

// GetPayQf 获取支付区分方式
func (u *User) GetPayQf() int8 {
	if u.PayQf != nil {
		return *u.PayQf
	}
	return 1 // 默认金额递增
}

// GetWxpay 获取微信收款码
func (u *User) GetWxpay() string {
	if u.Wxpay != nil {
		return *u.Wxpay
	}
	return ""
}

// GetZfbpay 获取支付宝收款码
func (u *User) GetZfbpay() string {
	if u.Zfbpay != nil {
		return *u.Zfbpay
	}
	return ""
}

// GetLastheart 获取最后心跳时间
func (u *User) GetLastheart() int64 {
	if u.Lastheart != nil {
		return *u.Lastheart
	}
	return 0
}

// GetLastpay 获取最后支付时间
func (u *User) GetLastpay() int64 {
	if u.Lastpay != nil {
		return *u.Lastpay
	}
	return 0
}

// GetJkstate 获取监控状态
func (u *User) GetJkstate() int16 {
	if u.Jkstate != nil {
		return *u.Jkstate
	}
	return 0 // 默认离线
}

// 用户角色常量
const (
	RoleAdmin      = "admin"
	RoleSuperAdmin = "super_admin"
)

// 用户状态常量
const (
	StatusDisabled = 0
	StatusEnabled  = 1
)
