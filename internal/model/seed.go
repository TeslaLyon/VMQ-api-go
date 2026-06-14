package model

import (
	"VMQ-api-go/internal/config"
	"VMQ-api-go/internal/utils"
	"log"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SeedData 统一入口
func SeedData() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		// 1. 初始化用户
		if err := seedUsers(tx); err != nil {
			return err
		}
		// 2. 初始化系统设置
		if err := seedSettings(tx); err != nil {
			return err
		}
		return nil
	})
}

// 填充默认用户
func seedUsers(tx *gorm.DB) error {
	defaultAdmin := User{
		Username: config.AppConfig.Server.AdminUsername,
		Role:     "super_admin",
	}

	// 2. 检查 admin 是否已存在
	var count int64
	tx.Model(&User{}).Where("username = ?", config.AppConfig.Server.AdminUsername).Count(&count)

	if count == 0 {
		// 3. 如果用户不存在，进行创建
		log.Println("⏳ 检测到系统尚未初始化管理员，正在创建...")

		// 使用 bcrypt 加密密码 (默认密码设为 admin123)
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(config.AppConfig.Server.AdminPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		defaultAdmin.Pass = string(hashedPassword)
		// 用户支付端salt
		randomPayPageSalt := utils.GenerateRandomString16Fast()
		defaultAdmin.PayPageSalt = &randomPayPageSalt
		// 监控 app 端通行秘钥
		randomMonitorAppSalt := utils.GenerateRandomString16Fast()
		defaultAdmin.MonitorAppSalt = &randomMonitorAppSalt
		defaultAdmin.AppId = &config.AppConfig.Server.DefaultAppID
		defaultClose := 5
		defaultPayQf := int8(1)
		defaultJkstate := int16(0)

		defaultAdmin.Close = &defaultClose
		defaultAdmin.PayQf = &defaultPayQf
		defaultAdmin.Jkstate = &defaultJkstate

		// 插入数据
		if err := tx.Create(&defaultAdmin).Error; err != nil {
			return err
		}
		log.Println("🎉 默认管理员用户 'admin' 创建成功 (密码: " + config.AppConfig.Server.AdminPassword + ")")
	} else {
		log.Println("✅ 管理员用户已存在，跳过初始化。")
	}
	return nil
}

// 填充默认配置
func seedSettings(tx *gorm.DB) error {
	defaults := []Setting{
		{Vkey: "site_title", UserID: 1, Vvalue: "我的管理后台"},
	}

	for _, setting := range defaults {
		var count int64
		tx.Model(&Setting{}).Where("vkey = ? AND user_id = ?", setting.Vkey, setting.UserID).Count(&count)
		if count == 0 {
			if err := tx.Create(&setting).Error; err != nil {
				return err
			}
		}
	}
	log.Println("✅ 默认设置填充完成")
	return nil
}
