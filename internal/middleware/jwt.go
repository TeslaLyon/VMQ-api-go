package middleware

import (
	"VMQ-api-go/internal/model"
	"VMQ-api-go/pkg/jwt"
	"VMQ-api-go/pkg/response"
	"strings"

	"github.com/gin-gonic/gin"
)

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "未登录")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			response.Unauthorized(c, "认证格式错误")
			c.Abort()
			return
		}

		tokenString := parts[1]

		// 检查 Valkey 黑名单
		exists, _ := model.VKB.Exists(c.Request.Context(), "blacklist:"+tokenString).Result()
		if exists > 0 {
			// ✅ 现在 CodeTokenInvalid 已经定义好了
			response.Error(c, response.CodeTokenInvalid, "登录已失效，请重新登录")
			c.Abort()
			return
		}

		// 解析 JWT
		claims, err := jwt.ParseToken(tokenString)
		if err != nil {
			response.Error(c, response.CodeTokenExpired, "身份验证已过期")
			c.Abort()
			return
		}

		// 安全加固：业务接口只允许 Access Token 访问
		if claims.TokenType != "access" {
			response.Error(c, response.CodeTokenInvalid, "非法令牌类型")
			c.Abort()
			return
		}

		c.Set("userID", claims.UserID)
		c.Next()
	}
}
