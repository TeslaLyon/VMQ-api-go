package middleware

import (
	"strconv"
	"strings"

	"VMQ-api-go/pkg/response"

	"github.com/gin-gonic/gin"
)

// RequireRole 角色权限中间件
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists {
			response.Unauthorized(c, "User not authenticated")
			c.Abort()
			return
		}

		role := userRole.(string)

		// 超级管理员拥有所有权限
		if role == "super_admin" {
			c.Next()
			return
		}

		// 检查角色权限
		for _, requiredRole := range roles {
			if role == requiredRole {
				c.Next()
				return
			}
		}

		response.BadRequest(c, "Insufficient permissions")
		c.Abort()
	}
}

// RequireAdmin 需要管理员权限
func RequireAdmin() gin.HandlerFunc {
	return RequireRole("admin", "super_admin")
}

// RequireSuperAdmin 需要超级管理员权限
func RequireSuperAdmin() gin.HandlerFunc {
	return RequireRole("super_admin")
}

// isPublicOrderAccess 判断是否是公开的订单访问
func isPublicOrderAccess(c *gin.Context) bool {
	// 检查是否有 public=true 查询参数
	if c.Query("public") == "true" {
		return true
	}

	// 检查路径是否包含 /public 段
	if strings.Contains(c.Request.URL.Path, "/public") {
		return true
	}

	// 检查是否没有Authorization头（支付页面通常不会发送认证头）
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		// 进一步检查是否是订单相关的GET请求
		if c.Request.Method == "GET" &&
			(strings.Contains(c.Request.URL.Path, "/orders/") ||
				strings.Contains(c.Request.URL.Path, "/status")) {
			return true
		}
	}

	return false
}

// GetCurrentUserID 获取当前用户ID
func GetCurrentUserID(c *gin.Context) uint {
	userIDVal, exists := c.Get("userID")
	if !exists {
		return 0 // 或者根据你的业务逻辑返回错误
	}

	// 💡 使用类型推断 (Type Switch) 来安全地处理各种可能的类型
	switch v := userIDVal.(type) {
	case uint:
		return v
	case string:
		// 如果是字符串，将其转换为 uint
		id, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return 0 // 转换失败，按需处理
		}
		return uint(id)
	case float64:
		// ⚠️ 补充防护：如果你是用 jwt-go 等标准库，JSON 解析数字默认是 float64
		return uint(v)
	case int:
		return uint(v)
	default:
		// 类型不匹配时的默认处理
		return 0
	}
}

// GetCurrentUsername 获取当前用户名
func GetCurrentUsername(c *gin.Context) string {
	if username, exists := c.Get("username"); exists {
		return username.(string)
	}
	return ""
}

// GetCurrentUserRole 获取当前用户角色
func GetCurrentUserRole(c *gin.Context) string {
	if role, exists := c.Get("user_role"); exists {
		return role.(string)
	}
	return ""
}

// IsSuperAdmin 检查是否为超级管理员
func IsSuperAdmin(c *gin.Context) bool {
	return GetCurrentUserRole(c) == "super_admin"
}
