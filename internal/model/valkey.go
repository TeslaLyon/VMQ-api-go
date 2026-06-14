package model

import (
	"context"
	"fmt"
	"log"
	"time"

	"VMQ-api-go/internal/config"

	"github.com/redis/go-redis/v9"
)

// VKB 全局 Valkey 客户端句柄
var VKB *redis.Client

// 全局上下文，常用于 Valkey/Redis 操作
var ctx = context.Background()

// InitValkey 初始化 Valkey 8.0 连接
func InitValkey() error {
	// 在实际生产中，建议将 Addr 和 Password 抽离到 config.yml 中
	// 这里为了呼应你 docker-compose.yml 中的配置，直接使用对应的账密
	VKB = redis.NewClient(&redis.Options{
		Addr:     config.AppConfig.Redis.Host + ":" + fmt.Sprintf("%d", config.AppConfig.Redis.Port), //
		Password: config.AppConfig.Redis.Password,
		DB:       0, // 默认数据库
	})

	// 🧪 通过 PING 命令测试连接是否正常
	_, err := VKB.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("无法连接到 Valkey 服务器: %w", err)
	}

	log.Println("✅ Valkey 8.0 连接成功!")
	return nil
}

// SetCache 写入数据 (包含过期时间)
func SetCache(key string, value string, expiration time.Duration) error {
	err := VKB.Set(ctx, key, value, expiration).Err()
	if err != nil {
		return fmt.Errorf("Valkey 写入失败 [%s]: %w", key, err)
	}
	return nil
}

// GetCache 读取数据
func GetCache(key string) (string, error) {
	val, err := VKB.Get(ctx, key).Result()
	if err == redis.Nil {
		// Key 不存在，不属于系统异常，返回空字符串和 nil
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("Valkey 读取失败 [%s]: %w", key, err)
	}
	return val, nil
}
