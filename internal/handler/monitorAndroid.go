package handler

import (
	"VMQ-api-go/internal/model"
	"VMQ-api-go/internal/service"
	"log"

	"github.com/gin-gonic/gin"
)

type MonitorAndroidHandler struct {
	monitorAndroidService service.MonitorAndroidService
}

func NewMonitorAndroidHandler(monitorAndroidService service.MonitorAndroidService) *MonitorAndroidHandler {
	return &MonitorAndroidHandler{
		monitorAndroidService: monitorAndroidService,
	}
}

func (h *MonitorAndroidHandler) MonitorHeart(c *gin.Context) {
	var req model.MonitorHeartRequest

	// 兼容GET和POST请求，优先从查询参数获取
	req.T = c.Query("t")
	req.Sign = c.Query("sign")
	req.AppID = c.Query("appid")

	// 如果查询参数为空，尝试从表单数据获取
	if req.T == "" || req.Sign == "" {
		if err := c.ShouldBind(&req); err != nil {
			// response.ValidationFailed(c, err.Error())
			c.JSON(200, gin.H{
				"code": -1,
				"msg":  "Missing required parameters",
			})
			return
		}
	}

	// 验证必需参数
	if req.T == "" || req.Sign == "" {
		log.Printf("心跳参数验证失败: t=%s, sign=%s, appid=%s", req.T, req.Sign, req.AppID)
		// response.ValidationFailed(c, "Missing required parameters: t and sign")
		c.JSON(200, gin.H{
			"code": -1,
			"msg":  "Missing required parameters: t and sign",
		})
		return
	}

	log.Printf("收到心跳请求: t=%s, sign=%s, appid=%s", req.T, req.Sign, req.AppID)

	err := h.monitorAndroidService.ProcessMonitorHeart(&req)
	if err != nil {
		if err == service.ErrInvalidSign {
			// response.Error(c, response.CodeUnauthorized, "Invalid signature")
			c.JSON(200, gin.H{
				"code": -1,
				"msg":  "Invalid signature",
			})
			return
		}
		// response.InternalError(c, "Failed to process monitor heart")
		c.JSON(200, gin.H{
			"code": -1,
			"msg":  "Failed to process monitor heart",
		})
		return
	}

	// response.Success(c, nil)
	c.JSON(200, gin.H{
		"code": 1,
		"msg":  "success",
	})
}

func (h *MonitorAndroidHandler) MonitorPush(c *gin.Context) {
	var req model.MonitorPushRequest
	if err := c.ShouldBind(&req); err != nil {
		// response.ValidationFailed(c, err.Error())
		c.JSON(200, gin.H{
			"code": -1,
			"msg":  "Missing required parameters",
		})
		return
	}

	err := h.monitorAndroidService.ProcessMonitorPush(&req)
	if err != nil {
		if err == service.ErrInvalidSign {
			// response.Error(c, response.CodeUnauthorized, "Invalid signature")
			c.JSON(200, gin.H{
				"code": -1,
				"msg":  "Invalid signature",
			})
			return
		}
		// response.InternalError(c, "Failed to process monitor push")
		c.JSON(200, gin.H{
			"code": -1,
			"msg":  "Failed to process monitor push",
			// "msg": err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"code": 1,
		"msg":  "success",
	})
}
