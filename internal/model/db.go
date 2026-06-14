package model

import (
	"fmt"
	"log"
	"time"

	"VMQ-api-go/internal/config" // 记得替换为你的实际模块名

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB 全局数据库连接对象
var DB *gorm.DB

// 1. 动态获取日志级别
var dbLogLevel logger.LogLevel

// TODO：业界标准做法是：本地用 AutoMigrate 快速迭代，线上配合 Atlas、Golang-migrate 或 GORM 官方的 GORM Atlas 迁移工具生成具体的 .sql 脚本进行精细化版本控制。

// InitDB 初始化 PostgreSQL 18 数据库连接
func InitDB() (*gorm.DB, error) {
	dbCfg := config.AppConfig.Database

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Asia/Shanghai",
		dbCfg.Host, dbCfg.Username, dbCfg.Password, dbCfg.Database, dbCfg.Port,
	)

	if config.AppConfig.Server.Mode == "release" {
		// 生产环境：只记录 Error 级别的日志，关闭普通的 SQL 打印
		dbLogLevel = logger.Error
	} else {
		// 开发/测试环境：记录所有 SQL（Info 级别），方便查问题
		dbLogLevel = logger.Info
	}

	// 假设你的配置存在 config.Mode 变量里
	log.Printf("!!! [DEBUG] 准备连接数据库，当前读取到的 Mode 是: [%s] !!!\n", config.AppConfig.Server.Mode)

	gormConfig := &gorm.Config{
		Logger:                                   logger.Default.LogMode(dbLogLevel),
		DisableForeignKeyConstraintWhenMigrating: true, // 禁用物理外键，推荐逻辑外键
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层 sql.DB 失败: %w", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour * 1)

	DB = db
	log.Println("✅ 成功连接至 PostgreSQL 18!")

	// ==========================================
	// 🆕 核心：在此处执行自动迁移
	// ==========================================
	log.Println("⏳ 正在检查并执行数据库结构自动迁移...")

	// 如果未来有新的模型（比如 Order, Product），直接在括号里用逗号追加即可：
	// err = DB.AutoMigrate(&User{}, &Order{}, &Product{})
	err = DB.AutoMigrate(
		&User{},
		&Setting{},
		&Order{},
		&PayQrcode{},
		&TmpPrice{},
	)
	if err != nil {
		return nil, fmt.Errorf("自动迁移数据表结构失败: %w", err)
	}

	// 2. 执行种子填充
	log.Println("⏳ 正在执行数据种子填充...")
	if err := SeedData(); err != nil {
		return nil, fmt.Errorf("种子填充失败: %w", err)
	}
	log.Println("🎉 数据库初始化完成!")
	return db, nil
}

// TableInfo 用于接收数据库返回的表名结构
type TableInfo struct {
	TableName string `gorm:"column:table_name"`
}

// ShowTables 查询当前 PostgreSQL 数据库中的所有表名
func ShowTables() ([]string, error) {
	var tables []TableInfo
	var tableNames []string

	// PostgreSQL 查所有表的标准原生 SQL
	// table_schema = 'public' 表示只查用户创建的公开表，排除系统内置表
	sqlStr := `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = 'public' 
		  AND table_type = 'BASE TABLE';
	`

	// 使用全局的 DB 对象执行原生 SQL，并将结果解析到结构体切片中
	err := DB.Raw(sqlStr).Scan(&tables).Error
	if err != nil {
		return nil, fmt.Errorf("query tables failed: %w", err)
	}

	// 提取出纯字符串切片
	for _, t := range tables {
		tableNames = append(tableNames, t.TableName)
	}

	return tableNames, nil
}
