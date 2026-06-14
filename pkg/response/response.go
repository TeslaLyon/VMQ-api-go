package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应格式 (增加 Success 字段以完美适配 Ant Design Pro)
type Response[T any] struct {
	Success      bool   `json:"success"`         // Ant Design Pro 核心识别字段
	ErrorCode    int    `json:"code"`            // 业务错误码
	ErrorMessage string `json:"msg"`             // 错误消息
	Data         T      `json:"data,omitempty"`  // 数据部分
	Total        int64  `json:"total,omitempty"` // 专门为 Ant Design Pro Table 优化的总数统计
}

// PagedResponse 分页响应格式
type PagedResponse struct {
	ErrorCode    int         `json:"code"`
	ErrorMessage string      `json:"msg"`
	Data         interface{} `json:"data"`
	Meta         PageMeta    `json:"meta"`
}

// PageMeta 分页元数据
type PageMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// 错误码常量 (保持你原有的逻辑)
const (
	CodeSuccess          = 200
	CodeBadRequest       = 400
	CodeUnauthorized     = 401
	CodeForbidden        = 403
	CodeNotFound         = 404
	CodeConflict         = 409
	CodeValidationFailed = 422
	CodeInternalError    = 500

	// 业务错误码
	CodeUserNotFound    = 40001
	CodeInvalidPassword = 40003
	CodeTokenExpired    = 40006
	CodeTokenInvalid    = 40007
)

// Success 成功响应 (支持泛型)
func Success[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, Response[T]{
		Success:      true,
		ErrorCode:    CodeSuccess,
		ErrorMessage: "Success",
		Data:         data,
	})
}

// SuccessTable 专门为 Ant Design Pro Table 设计的分页响应
// ProTable 默认结构是 { data: [], success: true, total: 100 }
func SuccessTable[T any](c *gin.Context, data T, total int64) {
	c.JSON(http.StatusOK, Response[T]{
		Success:      true,
		ErrorCode:    CodeSuccess,
		ErrorMessage: "Success",
		Data:         data,
		Total:        total,
	})
}

// Error 错误响应
func Error(c *gin.Context, code int, message ...string) {
	msg := getErrorMessage(code)
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}

	c.JSON(http.StatusOK, Response[any]{
		Success:      false,
		ErrorCode:    code,
		ErrorMessage: msg,
	})
}

// --- 快捷错误函数 ---

func BadRequest(c *gin.Context, message ...string) {
	Error(c, CodeBadRequest, message...)
}

func Unauthorized(c *gin.Context, message ...string) {
	Error(c, CodeUnauthorized, message...)
}

func Forbidden(c *gin.Context, message ...string) {
	Error(c, CodeForbidden, message...)
}

// ValidationFailed 422验证失败
func ValidationFailed(c *gin.Context, message ...string) {
	Error(c, CodeValidationFailed, message...)
}

func InternalError(c *gin.Context, message ...string) {
	Error(c, CodeInternalError, message...)
}

// NotFound 404错误
func NotFound(c *gin.Context, message ...string) {
	Error(c, CodeNotFound, message...)
}

// getHTTPStatus 获取 HTTP 状态码
func getHTTPStatus(code int) int {
	switch {
	case code == CodeSuccess:
		return http.StatusOK
	case code == CodeUnauthorized || code == CodeTokenExpired:
		return http.StatusUnauthorized
	case code == CodeForbidden:
		return http.StatusForbidden
	case code == CodeNotFound:
		return http.StatusNotFound
	case code >= 500:
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}

// getErrorMessage 获取错误消息
var errorMessages = map[int]string{
	CodeSuccess:         "Success",
	CodeBadRequest:      "Bad Request",
	CodeUnauthorized:    "Unauthorized",
	CodeInternalError:   "Internal Server Error",
	CodeUserNotFound:    "用户不存在",
	CodeInvalidPassword: "Wrong password",
	CodeTokenExpired:    "Token has expired",
	CodeTokenInvalid:    "Invalid Token or logged out",
	CodeConflict:        "Conflict",
}

func getErrorMessage(code int) string {
	if msg, exists := errorMessages[code]; exists {
		return msg
	}
	return "Unknown error"
}

// SuccessPaged 分页成功响应
func SuccessPaged(c *gin.Context, data interface{}, meta PageMeta) {
	c.JSON(http.StatusOK, PagedResponse{
		ErrorCode:    CodeSuccess,
		ErrorMessage: "Success",
		Data:         data,
		Meta:         meta,
	})
}

// SuccessWithMessage 带消息的成功响应
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response[any]{
		Success:      true,
		ErrorCode:    CodeSuccess,
		ErrorMessage: message,
		Data:         data,
	})
}
