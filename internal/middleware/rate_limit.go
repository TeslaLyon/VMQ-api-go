package middleware

import (
	"context"
	"net/http"
	"time"
	"VMQ-api-go/internal/model"

	"github.com/gin-gonic/gin"
)

func LoginRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		ip := c.ClientIP()
		limitKey := "rate_limit:login:" + ip

		// 在 Valkey 中对该 IP 的计数器加 1
		count, err := model.VKB.Incr(ctx, limitKey).Result()
		if err != nil {
			c.Next() // 如果缓存挂了，放行，不影响核心业务
			return
		}

		// 如果是第一次访问，设置 1 分钟的过期时间
		if count == 1 {
			model.VKB.Expire(ctx, limitKey, time.Minute)
		}

		// 限制一分钟最多 5 次
		if count > 5 {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"status":  "error",
				"message": "登录过于频繁，请一分钟后再试",
			})
			c.Abort() // 拦截请求，不再往下执行 Handler
			return
		}

		c.Next()
	}
}