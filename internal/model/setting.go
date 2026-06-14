package model

type Setting struct {
	Vkey   string `json:"vkey" gorm:"primaryKey;size:255;column:vkey"`
	UserID uint   `json:"user_id" gorm:"primaryKey;default:1;column:user_id"`
	Vvalue string `json:"vvalue" gorm:"type:text;column:vvalue"`
}

// MonitorHeartRequest 监控心跳请求
type MonitorHeartRequest struct {
	T     string `form:"t" binding:"required"`
	Sign  string `form:"sign" binding:"required"`
	AppID string `form:"appid"` // 可选，用于多用户系统
}

// MonitorPushRequest 监控推送请求
type MonitorPushRequest struct {
	T     string `form:"t" binding:"required"`
	Sign  string `form:"sign" binding:"required"`
	Type  string `form:"type" binding:"required"`
	Price int64 `form:"price" binding:"required"`
	AppID string `form:"appid"` // 可选，用于多用户系统
}
