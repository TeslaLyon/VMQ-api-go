package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"net/http"
	"strconv"
	"time"

	"VMQ-api-go/internal/config"

	"github.com/gin-gonic/gin"
)

// Mock 数据库查询：根据 AppKey 获取对应的 AppSecret
// 实际开发中，你应该从数据库（如 developer_apps 表）中查询
func getAppSecretByKey(appKey string) string {
	if appKey == config.AppConfig.Server.OpenapiKey {
		// 匹配成功，返回配置中的 openapi_value
		return config.AppConfig.Server.OpenapiValue
	}
	return ""
}

// OpenAPIAuthMiddleware 开放接口的签名校验中间件
func OpenAPIAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 从 Header 中获取签名相关参数
		appKey := c.GetHeader("X-App-Key")
		timestampStr := c.GetHeader("X-Timestamp")
		nonce := c.GetHeader("X-Nonce")
		clientSignature := c.GetHeader("X-Signature")

		// 2. 检查参数是否完整
		if appKey == "" || timestampStr == "" || nonce == "" || clientSignature == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "缺少必要的签名参数"})
			return
		}

		// 3. 校验时间戳，防止“重放攻击”（Replay Attack）
		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "非法的时间戳格式"})
			return
		}

		now := time.Now().Unix()
		// 允许服务器之间有 5 分钟（300秒）的时钟误差
		if math.Abs(float64(now-timestamp)) > 300 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "请求已过期或时间戳异常"})
			return
		}

		// 4. 获取对应的 AppSecret
		appSecret := getAppSecretByKey(appKey)
		if appSecret == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "无效的 AppKey"})
			return
		}

		// 5. 服务端重新计算签名
		// 签名规则（你需要写进给第三方的 API 文档里）：
		// 拼接规则：AppKey + Timestamp + Nonce
		// 加密算法：HMAC-SHA256
		signPayload := appKey + timestampStr + nonce
		expectedSignature := GenerateHMACSHA256(appSecret, signPayload)

		// 6. 比对签名
		if clientSignature != expectedSignature {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "签名验证失败，请检查 Secret 或加密算法"})
			return
		}

		// 7. 校验通过，可以将第三方标识存入上下文，供后续 Controller 使用
		c.Set("OpenAPI_AppKey", appKey)
		c.Next()
	}
}

// GenerateHMACSHA256 使用 HMAC-SHA256 算法生成签名
func GenerateHMACSHA256(secret, payload string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	// 转换为小写的十六进制字符串
	return hex.EncodeToString(h.Sum(nil))
}
