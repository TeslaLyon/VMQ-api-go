package model

// TmpPrice 临时价格表模型 - 用于避免金额冲突
// TODO：纠结是否添加 user_id字段，如果添加了，生成唯一价格时就需要考虑同一用户的订单不能有金额冲突，这样就更复杂了。暂时先不添加 user_id 字段，直接全局保证金额唯一。
type TmpPrice struct {
	ID    int64  `gorm:"primaryKey"`
	Price int64  `json:"price" gorm:"type:bigint;uniqueIndex:idx_price_type;not null;comment:价格-单位为分"`
	Type  int    `json:"type" gorm:"uniqueIndex:idx_price_type;not null;comment:1=微信,2=支付宝"`
	Oid   string `json:"oid" gorm:"size:255;not null;comment:订单ID"`
}

// TableName 指定表名
func (TmpPrice) TableName() string {
	return "tmp_price"
}
