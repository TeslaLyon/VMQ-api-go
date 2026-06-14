package handler

import (
	"VMQ-api-go/internal/dto"
	"VMQ-api-go/internal/model"
	"VMQ-api-go/pkg/jwt"
	"VMQ-api-go/pkg/response" // 引入你的统一响应包
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// TODO:将报错信息写入日志，全局的
// 登录请求参数
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// 刷新请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Login 处理登录逻辑
func Login(c *gin.Context) {
	var req LoginRequest
	// 1. 参数校验
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "用户名或密码不能为空")
		return
	}

	// 2. 数据库查询用户
	var user model.User
	err := model.DB.Where("username = ?", req.Username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, response.CodeUserNotFound, "该账户不存在")
			return
		}
		response.InternalError(c, "数据库查询异常")
		return
	}

	// 3. 验证密码 (Bcrypt 对比)
	err = bcrypt.CompareHashAndPassword([]byte(user.Pass), []byte(req.Password))
	if err != nil {
		// 密码错误，使用你定义的业务错误码
		response.Error(c, response.CodeInvalidPassword, "密码错误")
		return
	}

	// 生成双 Token
	accessToken, refreshToken, err := jwt.GenerateAllTokens(strconv.Itoa(int(user.ID)), user.Username, user.Role)

	if err != nil {
		response.InternalError(c, "Token 生成失败")
		return
	}

	// 将长效 RefreshToken 存入 Valkey
	remaining, _ := jwt.GetTokenRemainingTime(refreshToken)
	model.VKB.Set(c, "refresh_token:"+strconv.Itoa(int(user.ID)), refreshToken, remaining)

	// 返回格式完美适配 Ant Design Pro 的 LoginResult 结构
	response.Success(c, gin.H{
		"status":           "ok",
		"currentAuthority": user.Role,
		"access_token":     accessToken,
		"refresh_token":    refreshToken,
	})
}

// CurrentUser 获取当前登录用户信息
func CurrentUser(c *gin.Context) {
	// 1. 从 JWT 中间件设置的上下文中获取用户 ID
	uidStr, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "请先登录")
		return
	}

	// 2. 从数据库读取最新数据
	var user model.User
	if err := model.DB.Select("id, username, role").First(&user, uidStr).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, response.CodeUserNotFound, "找不到该用户信息")
			return
		}
		response.InternalError(c, "数据库查询异常")
		return
	}

	// 3. 组装 Ant Design Pro 期待的 UserDetailedInfo
	// 注意：Data 字段会自动被 response.Success 包裹
	data := gin.H{
		"userid":   user.ID,
		"username": user.Username,
		"access":   user.Role, // 权限角色
	}

	response.Success(c, data)
}

// Logout 处理退出登录
func Logout(c *gin.Context) {
	// 1. 从 Header 中获取 Token
	authHeader := c.GetHeader("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		response.Success(c, gin.H{}) // 格式不对也当作成功，防止前端报错
		return
	}
	tokenString := parts[1]

	// 2. 计算该 Token 还有多久过期
	remaining, err := jwt.GetTokenRemainingTime(tokenString)
	if err != nil {
		response.Success(c, gin.H{})
		return
	}

	// 3. 将 Token 存入 Valkey 黑名单
	// Key 格式为 blacklist:token_字符串，Value 随意，过期时间设为 Token 剩余寿命
	blacklistKey := "blacklist:" + tokenString
	err = model.VKB.Set(c.Request.Context(), blacklistKey, "1", remaining).Err()
	if err != nil {
		response.InternalError(c, "注销失败")
		return
	}

	response.Success(c, gin.H{})
}

// 刷新 Token 接口
func RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误1")
		return
	}

	// 1. 解析并校验 Refresh Token
	claims, err := jwt.ParseToken(req.RefreshToken)
	if err != nil || claims.TokenType != "refresh" {
		response.Error(c, response.CodeTokenInvalid, "无效的刷新令牌")
		return
	}

	// 2. 核心安全检查：校验 Valkey 中是否存在该 Token (防止注销后仍能刷新)
	storedToken, err := model.VKB.Get(c, "refresh_token:"+claims.UserID).Result()
	if err != nil || storedToken != req.RefreshToken {
		response.Error(c, response.CodeTokenInvalid, "令牌已失效或已在别处登录")
		return
	}

	// 3. 查库获取用户信息确保用户未被禁用
	var user model.User
	if err := model.DB.Select("id").First(&user, claims.UserID).Error; err != nil {
		response.Error(c, response.CodeUserNotFound)
		return
	}

	// 4. 重新生成一对 Token (Rotation 机制：旧的刷新后立即作废)
	newAccess, newRefresh, _ := jwt.GenerateAllTokens(strconv.Itoa(int(user.ID)), user.Username, user.Role)

	// 5. 更新 Valkey 记录
	remaining, _ := jwt.GetTokenRemainingTime(newRefresh)
	model.VKB.Set(c, "refresh_token:"+claims.UserID, newRefresh, remaining)

	response.Success(c, gin.H{
		"access_token":  newAccess,
		"refresh_token": newRefresh,
	})
}
func SysConfig(c *gin.Context) {
	// 1. 从 JWT 中间件设置的上下文中获取用户 ID
	uidStr, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "请先登录")
		return
	}

	// 2. 从数据库读取最新数据
	var user model.User
	if err := model.DB.Select("id, username, role, pay_page_salt, monitor_app_salt, appid, notify_url, return_url, close, pay_qufen, wxpay, zfbpay").First(&user, uidStr).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, response.CodeUserNotFound, "找不到该用户信息")
			return
		}
		response.InternalError(c, "数据库查询异常")
		return
	}

	// 3. 组装 Ant Design Pro 期待的 UserDetailedInfo
	// 注意：Data 字段会自动被 response.Success 包裹
	data := gin.H{
		"username":         user.Username,
		"access":           user.Role, // 权限角色
		"pay_page_salt":    user.PayPageSalt,
		"monitor_app_salt": user.MonitorAppSalt,
		"appid":            user.AppId,
		"notifyUrl":        user.NotifyUrl,
		"returnUrl":        user.ReturnUrl,
		"close":            user.Close,
		"pay_qufen":        user.PayQf,
		"wxpay":            user.Wxpay,
		"zfbpay":           user.Zfbpay,
	}

	response.Success(c, data)
}

func UpdateSystemConfig(c *gin.Context) {

	userId, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "请先登录")
		return
	}

	var input dto.UpdateSysConfigInput

	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, response.CodeValidationFailed, "参数格式错误")
		return
	}

	// 核心：使用 map[string]interface{}，GORM 只会更新 map 中存在的 key
	updates := make(map[string]interface{})

	// 逻辑优化：手动映射字段名，确保与数据库 column 标签一致
	if input.AppId != nil {
		updates["appid"] = *input.AppId
	}
	if input.Close != nil {
		updates["close"] = *input.Close
	}
	if input.MonitorAppSalt != nil {
		updates["monitor_app_salt"] = *input.MonitorAppSalt
	}
	if input.NotifyUrl != nil {
		updates["notify_url"] = input.NotifyUrl
	}
	if input.PayPageSalt != nil {
		updates["pay_page_salt"] = *input.PayPageSalt
	}
	if input.PayQufen != nil {
		updates["pay_qufen"] = *input.PayQufen
	} // 映射 payQf
	if input.ReturnUrl != nil {
		updates["return_url"] = input.ReturnUrl
	}
	if input.Username != nil {
		updates["username"] = *input.Username
	}
	if input.Wxpay != nil {
		updates["wxpay"] = input.Wxpay
	}
	if input.Zfbpay != nil {
		updates["zfbpay"] = input.Zfbpay
	}

	// 密码特殊处理：如果传了密码且不为空字符串则加密
	if input.Password != nil && *input.Password != "" {
		hashed, _ := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
		updates["pass"] = string(hashed) // 映射数据库的 pass 字段
	}

	if len(updates) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "无内容变更"})
		return
	}

	// 执行更新（假设当前只维护一个管理员账号，根据 username="admin" 查找）
	// 注意：updated_at 会因为 GORM 的机制自动更新
	err := model.DB.Table("users").Where("id = ?", userId).Updates(updates).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "系统配置更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "保存成功"})
}

func MonitorSettings(c *gin.Context) {
	// 1. 从 JWT 中间件设置的上下文中获取用户 ID
	uidID, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "请先登录")
		return
	}

	// 2. 从数据库读取最新数据
	var user model.User
	if err := model.DB.Select("lastheart, lastpay, jkstate, appid, monitor_app_salt").First(&user, uidID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, response.CodeUserNotFound, "找不到该用户信息")
			return
		}
		response.InternalError(c, "数据库查询异常")
		return
	}

	// 3. 组装 Ant Design Pro 期待的 UserDetailedInfo
	// 注意：Data 字段会自动被 response.Success 包裹
	data := gin.H{
		"lastheart":        user.Lastheart,
		"lastpay":          user.Lastpay,
		"jkstate":          user.Jkstate,
		"appid":            user.AppId,
		"monitor_app_salt": user.MonitorAppSalt,
	}

	response.Success(c, data)
}
