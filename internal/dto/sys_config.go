package dto

// UpdateSysConfigInput 定义了更新接口允许接收的参数
type UpdateSysConfigInput struct {
	AppId          *string `json:"appid"`
	Close          *int    `json:"close"`
	MonitorAppSalt *string `json:"monitor_app_salt"`
	NotifyUrl      *string `json:"notifyUrl"`
	PayPageSalt    *string `json:"pay_page_salt"`
	PayQufen       *int16  `json:"pay_qufen"` // 对应前端的 pay_qufen
	ReturnUrl      *string `json:"returnUrl"`
	Username       *string `json:"username"`
	Password       *string `json:"password"`
	Wxpay          *string `json:"wxpay"`
	Zfbpay         *string `json:"zfbpay"`
}
