package middleware

import (
	"VMQ-api-go/internal/config"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// 允许的时间戳误差范围（秒），防止重放攻击
	// 例如：5分钟内有效
	MaxTimeOffset = 300
)

// SignAuthMiddleware 签名验证中间件
func SignAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 解析请求参数 (支持 GET Query 和 POST Form)
		if err := c.Request.ParseForm(); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "解析请求参数失败"})
			return
		}

		// 将所有参数存入 map
		params := make(map[string]string)
		for k, v := range c.Request.Form {
			if len(v) > 0 {
				params[k] = v[0]
			}
		}

		// 2. 提取签名 s 和时间戳 timestamp
		clientSign, hasSign := params["s"]
		timestampStr, hasTime := params["timestamp"]

		if !hasSign || !hasTime {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "缺少签名或时间戳参数"})
			return
		}

		// 3. 校验时间戳 (防重放攻击)
		clientTime, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "非法的时间戳格式"})
			return
		}

		now := time.Now().Unix()
		// 如果请求的时间戳距离服务器当前时间超过了允许的误差，直接拒绝
		if now-clientTime > MaxTimeOffset || clientTime-now > MaxTimeOffset {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "请求已过期 (Timestamp Error)"})
			return
		}

		// 4. 从参数中剔除签名本身，准备进行排序计算
		delete(params, "s")

		// 5. 将参数按 Key 的字典序进行升序排序
		var keys []string
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// 6. 拼接字符串 (格式: key1=value1&key2=value2...&salt=AppSalt)
		var buffer bytes.Buffer
		for _, k := range keys {
			// 如果你的业务逻辑要求空值不参与签名，可以在这里加个判断：if params[k] == "" { continue }
			buffer.WriteString(k)
			buffer.WriteString("=")
			buffer.WriteString(params[k])
			buffer.WriteString("&")
		}
		// 追加自定义 Salt
		buffer.WriteString("salt=")
		buffer.WriteString(config.AppConfig.Server.Appsalt)

		signStr := buffer.String()

		// 7. 生成 MD5
		hash := md5.Sum([]byte(signStr))
		serverSign := hex.EncodeToString(hash[:]) // 转换为小写 32 位 MD5 字符串

		// 8. 比对签名
		if serverSign != clientSign {
			// 为了安全，建议不要把正确的签名返回给客户端，打印在控制台用于调试即可
			fmt.Printf("[SignAuth] 签名校验失败. 客户端串: %s, 计算用串: %s, 期望签名: %s\n", clientSign, signStr, serverSign)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "签名验证失败"})
			return
		}

		// 校验通过，交由下一个 Handler 处理
		c.Next()
	}
}
